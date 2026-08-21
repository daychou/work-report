package handler

import (
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"work-report/server/internal/middleware"
	"work-report/server/internal/model"
	"work-report/server/internal/notify"
)

type CommentHandler struct {
	db     *gorm.DB
	feishu *notify.FeishuClient
	appURL string

	// 用户列表缓存：@提及 解析每次全量查用户表，评论高频时是浪费；60s TTL 足够新鲜
	usersMu    sync.Mutex
	usersCache []model.User
	usersAt    time.Time
}

func NewCommentHandler(db *gorm.DB, feishu *notify.FeishuClient, appURL string) *CommentHandler {
	return &CommentHandler{db: db, feishu: feishu, appURL: strings.TrimSuffix(appURL, "/")}
}

// cachedUsers 60 秒内复用用户列表
func (h *CommentHandler) cachedUsers() ([]model.User, error) {
	h.usersMu.Lock()
	defer h.usersMu.Unlock()
	if h.usersCache != nil && time.Since(h.usersAt) < 60*time.Second {
		return h.usersCache, nil
	}
	var users []model.User
	if err := h.db.Find(&users).Error; err != nil {
		return nil, err
	}
	h.usersCache = users
	h.usersAt = time.Now()
	return users, nil
}

// List 某工作内容下的评论（按时间正序，最多 500 条兜底）
func (h *CommentHandler) List(c *gin.Context) {
	workItemID, _ := strconv.Atoi(c.Param("id"))
	var comments []model.Comment
	if err := h.db.Preload("Author").
		Where("work_item_id = ?", workItemID).
		Order("created_at asc").
		Limit(500).
		Find(&comments).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, comments)
}

// Create 发表评论（任何登录用户）；内容中 @某人 会给对方发站内通知，绑定飞书则同步推送。
// 带 parent_id 时为回复：回复的回复归到顶级评论下（两层结构），并通知被回复者
func (h *CommentHandler) Create(c *gin.Context) {
	user := middleware.CurrentUser(c)
	workItemID, _ := strconv.Atoi(c.Param("id"))

	var item model.WorkItem
	if err := h.db.First(&item, workItemID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "工作内容不存在"})
		return
	}

	var body struct {
		Content  string `json:"content" binding:"required"`
		ParentID *uint  `json:"parent_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "评论内容不能为空"})
		return
	}
	content := strings.TrimSpace(body.Content)
	if content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "评论内容不能为空"})
		return
	}

	// 回复：被回复的评论必须属于同一工作内容；回复的回复归到其顶级评论下
	var replyTarget *model.Comment
	if body.ParentID != nil {
		var p model.Comment
		if err := h.db.Preload("Author").First(&p, *body.ParentID).Error; err != nil || p.WorkItemID != item.ID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "回复的评论不存在"})
			return
		}
		replyTarget = &p
		if p.ParentID != nil {
			body.ParentID = p.ParentID
		}
	}

	comment := model.Comment{
		WorkItemID: item.ID,
		ParentID:   body.ParentID,
		AuthorID:   user.ID,
		Content:    content,
	}
	if err := h.db.Create(&comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.db.Preload("Author").First(&comment, comment.ID)

	mentioned := h.parseMentions(content, user.ID)
	go h.notifyMentions(user, &item, content, comment.ID, mentioned)

	// 回复通知：回复他人评论且对方未被 @ 命中时发（避免与提及通知重复）
	if replyTarget != nil && replyTarget.AuthorID != user.ID && !hasUser(mentioned, replyTarget.AuthorID) {
		go h.notifyReply(user, &item, &replyTarget.Author, content, comment.ID)
	}

	// 评论通知：无论是否 @，都通知任务的负责人与参与人（排除作者自己与已覆盖的人）
	go h.notifyStakeholders(user, &item, content, comment.ID, mentioned, replyTarget)

	c.JSON(http.StatusCreated, comment)
}

func hasUser(users []model.User, id uint) bool {
	for _, u := range users {
		if u.ID == id {
			return true
		}
	}
	return false
}

// notifyReply 回复评论时通知被回复者：站内通知 + 飞书消息（含任务链接，可定位到该回复）
func (h *CommentHandler) notifyReply(author *model.User, item *model.WorkItem, target *model.User, content string, commentID uint) {
	link := fmt.Sprintf("%s/board?item=%d&comment=%d", h.appURL, item.ID, commentID)
	n := model.Notification{
		UserID:     target.ID,
		WorkItemID: &item.ID,
		CommentID:  &commentID,
		Type:       "reply",
		Title:      fmt.Sprintf("%s 在「%s」中回复了你的评论", author.Name, item.Title),
		Content:    content,
	}
	if err := h.db.Create(&n).Error; err != nil {
		log.Printf("reply notification: %v", err)
		return
	}
	if target.FeishuOpenID == "" || h.feishu == nil || !h.feishu.Enabled() {
		return
	}
	text := fmt.Sprintf("%s 在「%s」中回复了你的评论：\n%s\n\n查看任务：%s", author.Name, item.Title, content, link)
	if err := h.feishu.SendText(target.FeishuOpenID, text); err != nil {
		log.Printf("feishu reply to %s: %v", target.Name, err)
	}
}

// notifyStakeholders 评论时通知任务负责人与参与人：站内通知 + 飞书消息。
// 排除：评论作者自己、已被 @ 提及覆盖的、已被回复通知覆盖的（避免重复打扰）
func (h *CommentHandler) notifyStakeholders(author *model.User, item *model.WorkItem, content string, commentID uint, mentioned []model.User, replyTarget *model.Comment) {
	targets := map[uint]model.User{}
	if item.AssigneeID != nil {
		var u model.User
		if err := h.db.First(&u, *item.AssigneeID).Error; err == nil {
			targets[u.ID] = u
		}
	}
	var participants []model.User
	if err := h.db.Model(item).Association("Participants").Find(&participants); err == nil {
		for _, p := range participants {
			targets[p.ID] = p
		}
	}

	link := fmt.Sprintf("%s/board?item=%d&comment=%d", h.appURL, item.ID, commentID)
	for id, target := range targets {
		if id == author.ID || hasUser(mentioned, id) {
			continue
		}
		if replyTarget != nil && replyTarget.AuthorID == id {
			continue
		}
		n := model.Notification{
			UserID:     id,
			WorkItemID: &item.ID,
			CommentID:  &commentID,
			Type:       "comment",
			Title:      fmt.Sprintf("%s 评论了「%s」", author.Name, item.Title),
			Content:    content,
		}
		if err := h.db.Create(&n).Error; err != nil {
			log.Printf("comment notification: %v", err)
			continue
		}
		if target.FeishuOpenID == "" || h.feishu == nil || !h.feishu.Enabled() {
			continue
		}
		text := fmt.Sprintf("%s 评论了「%s」：\n%s\n\n查看任务：%s", author.Name, item.Title, content, link)
		if err := h.feishu.SendText(target.FeishuOpenID, text); err != nil {
			log.Printf("feishu comment to %s: %v", target.Name, err)
		}
	}
}

// notifyMentions 给被 @ 提及者发站内通知 + 飞书消息（含任务链接，可定位到评论）
func (h *CommentHandler) notifyMentions(author *model.User, item *model.WorkItem, content string, commentID uint, mentioned []model.User) {
	if len(mentioned) == 0 {
		return
	}
	link := fmt.Sprintf("%s/board?item=%d&comment=%d", h.appURL, item.ID, commentID)
	for _, target := range mentioned {
		n := model.Notification{
			UserID:     target.ID,
			WorkItemID: &item.ID,
			CommentID:  &commentID,
			Type:       "mention",
			Title:      fmt.Sprintf("%s 在「%s」的评论中提到了你", author.Name, item.Title),
			Content:    content,
		}
		if err := h.db.Create(&n).Error; err != nil {
			log.Printf("mention notification: %v", err)
			continue
		}
		if target.FeishuOpenID == "" || h.feishu == nil || !h.feishu.Enabled() {
			continue
		}
		text := fmt.Sprintf("%s 在「%s」的评论中提到了你：\n%s\n\n查看任务：%s", author.Name, item.Title, content, link)
		if err := h.feishu.SendText(target.FeishuOpenID, text); err != nil {
			log.Printf("feishu mention to %s: %v", target.Name, err)
		}
	}
}

// parseMentions 找出内容中 @ 到的用户。按用户名长度降序做带边界的子串匹配，
// 避免「张三」误命中「张三丰」。@自己 不通知。
func (h *CommentHandler) parseMentions(content string, authorID uint) []model.User {
	users, err := h.cachedUsers()
	if err != nil {
		return nil
	}
	sort.Slice(users, func(i, j int) bool { return len(users[i].Name) > len(users[j].Name) })

	hit := map[uint]bool{}
	var out []model.User
	for _, u := range users {
		if u.ID == authorID || u.Name == "" || hit[u.ID] {
			continue
		}
		needle := "@" + u.Name
		idx := 0
		for {
			i := strings.Index(content[idx:], needle)
			if i < 0 {
				break
			}
			i += idx
			after := content[i+len(needle):]
			// 名字后面必须是结束或非命名字符（空格/标点/换行等）
			r, _ := utf8.DecodeRuneInString(after)
			if after == "" || !isNameChar(r) {
				hit[u.ID] = true
				out = append(out, u)
				break
			}
			idx = i + len(needle)
		}
	}
	return out
}

// isNameChar 判断字符是否可能属于用户名（汉字/字母/数字/下划线/连字符）；
// 中文标点（，。！等）不在汉字区间，视为边界
func isNameChar(r rune) bool {
	if r >= 0x4E00 && r <= 0x9FFF { // CJK 统一汉字
		return true
	}
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-'
}

// Delete 删除评论：仅评论作者或管理员
func (h *CommentHandler) Delete(c *gin.Context) {
	user := middleware.CurrentUser(c)
	id, _ := strconv.Atoi(c.Param("id"))

	var comment model.Comment
	if err := h.db.First(&comment, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "评论不存在"})
		return
	}
	if comment.AuthorID != user.ID && !user.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "只有评论发布者或管理员可以删除"})
		return
	}
	// 删除顶级评论时级联删除其下所有回复，避免产生孤儿回复
	if err := h.db.Where("parent_id = ?", comment.ID).Delete(&model.Comment{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := h.db.Delete(&comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
