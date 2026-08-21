package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"work-report/server/internal/middleware"
	"work-report/server/internal/model"
)

type WorkItemHandler struct {
	db *gorm.DB
}

func NewWorkItemHandler(db *gorm.DB) *WorkItemHandler {
	return &WorkItemHandler{db: db}
}

func parseDate(s string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", s, time.Local)
}

// List 工作内容列表
// 过滤参数：project_id / author_id / kind / status / date_from / date_to / due_before / visible
// visible=1 时按可见性过滤（提交人/负责人/参与人可见；管理员不受限）
// limit 兜底防数据暴涨拖垮接口：默认 500，上限 1000
func (h *WorkItemHandler) List(c *gin.Context) {
	user := middleware.CurrentUser(c)
	q := h.db.Preload("Project").Preload("Author").Preload("Assignee").Preload("Participants").
		Order("work_date desc, created_at desc")

	limit := 500
	if v, err := strconv.Atoi(c.Query("limit")); err == nil && v > 0 {
		limit = min(v, 1000)
	}
	q = q.Limit(limit)

	if c.Query("visible") == "1" && !user.IsAdmin {
		q = q.Where(
			"work_items.author_id = ? OR work_items.assignee_id = ? OR work_items.id IN "+
				"(SELECT work_item_id FROM work_item_participants WHERE user_id = ?)",
			user.ID, user.ID, user.ID,
		)
	}
	if v := c.Query("project_id"); v != "" {
		q = q.Where("project_id = ?", v)
	}
	if v := c.Query("author_id"); v != "" {
		q = q.Where("author_id = ?", v)
	}
	if v := c.Query("kind"); v != "" {
		q = q.Where("kind = ?", v)
	}
	if v := c.Query("status"); v != "" {
		q = q.Where("status = ?", v)
	} else {
		q = q.Where("status != ?", "cancelled")
	}
	if v := c.Query("date_from"); v != "" {
		if d, err := parseDate(v); err == nil {
			q = q.Where("work_date >= ?", d)
		}
	}
	if v := c.Query("date_to"); v != "" {
		if d, err := parseDate(v); err == nil {
			q = q.Where("work_date <= ?", d)
		}
	}
	// done_date_from/done_date_to：只按时间过滤已完成任务（看板用），
	// 待办/进行中的任务不受时间限制、始终展示
	var doneFrom, doneTo *time.Time
	if v := c.Query("done_date_from"); v != "" {
		if d, err := parseDate(v); err == nil {
			doneFrom = &d
		}
	}
	if v := c.Query("done_date_to"); v != "" {
		if d, err := parseDate(v); err == nil {
			doneTo = &d
		}
	}
	if doneFrom != nil || doneTo != nil {
		doneCond := h.db.Where("status = ?", "done")
		if doneFrom != nil {
			doneCond = doneCond.Where("work_date >= ?", *doneFrom)
		}
		if doneTo != nil {
			doneCond = doneCond.Where("work_date <= ?", *doneTo)
		}
		q = q.Where(h.db.Where("status != ?", "done").Or(doneCond))
	}
	if v := c.Query("due_before"); v != "" {
		if d, err := parseDate(v); err == nil {
			q = q.Where("due_date <= ?", d)
		}
	}

	var items []model.WorkItem
	if err := q.Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.fillCommentCounts(items)
	c.JSON(http.StatusOK, items)
}

// fillCommentCounts 一次性 GROUP BY 统计各条目评论数，避免逐条查询的 N+1
func (h *WorkItemHandler) fillCommentCounts(items []model.WorkItem) {
	if len(items) == 0 {
		return
	}
	ids := make([]uint, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	type row struct {
		WorkItemID uint
		Cnt        int64
	}
	var rows []row
	if err := h.db.Model(&model.Comment{}).
		Select("work_item_id, COUNT(*) AS cnt").
		Where("work_item_id IN ?", ids).
		Group("work_item_id").
		Scan(&rows).Error; err != nil {
		return // 统计失败不阻塞列表返回，评论数按 0 处理
	}
	byID := make(map[uint]int64, len(rows))
	for _, r := range rows {
		byID[r.WorkItemID] = r.Cnt
	}
	for i := range items {
		items[i].CommentCount = byID[items[i].ID]
	}
}

// GetByID 单条查询（任务详情链接跳转用），任何登录用户可查
func (h *WorkItemHandler) GetByID(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var item model.WorkItem
	if err := h.db.Preload("Project").Preload("Author").Preload("Assignee").Preload("Participants").
		First(&item, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "条目不存在"})
		return
	}
	c.JSON(http.StatusOK, item)
}

// Create 提交工作内容/计划
// status 可由前端指定（done/doing/todo），缺省：work→done，plan→todo
// assignee_id 缺省为提交人自己；participant_ids 为参与人列表
func (h *WorkItemHandler) Create(c *gin.Context) {
	user := middleware.CurrentUser(c)
	var body struct {
		Title          string `json:"title" binding:"required"`
		Content        string `json:"content"`
		Detail         string `json:"detail"` // 详细内容（第二层，可选）
		ProjectID      uint   `json:"project_id" binding:"required"`
		Kind           string `json:"kind"` // 可选，缺省 work（前端已不再区分类型，保留兼容旧调用）
		Priority       string `json:"priority"`
		Status         string `json:"status"`    // done / doing / todo
		WorkDate       string `json:"work_date"` // YYYY-MM-DD，默认今天
		DueDate        string `json:"due_date"`  // 可选
		DueRemind      bool   `json:"due_remind"`   // 勾选后截止日当天 18:00 提醒
		StartRemind    bool   `json:"start_remind"` // 勾选后开始日当天 12:00 提醒（仅未来开始日期生效）
		AssigneeID     *uint  `json:"assignee_id"`
		ParticipantIDs []uint `json:"participant_ids"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var project model.Project
	if err := h.db.First(&project, body.ProjectID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "项目不存在，请先创建项目"})
		return
	}

	// work_date 可空：空表示待办任务尚未排期（「待办」勾选框清空日期）
	var workDate *time.Time
	if body.WorkDate != "" {
		if d, err := parseDate(body.WorkDate); err == nil {
			workDate = &d
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "work_date 格式应为 YYYY-MM-DD"})
			return
		}
	}

	var dueDate *time.Time
	if body.DueDate != "" {
		d, err := parseDate(body.DueDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "due_date 格式应为 YYYY-MM-DD"})
			return
		}
		dueDate = &d
	}

	priority := body.Priority
	if priority == "" {
		priority = "medium"
	}

	// 类型：缺省按 work 处理（前端已统一为任务，不再区分）
	kind := body.Kind
	if kind != "plan" && kind != "work" {
		kind = "work"
	}

	// 状态：优先前端指定；缺省当日工作已完成、计划待办
	status := body.Status
	if status != "done" && status != "doing" && status != "todo" {
		if kind == "work" {
			status = "done"
		} else {
			status = "todo"
		}
	}
	// 未排期（无开始日期）或未来开始的任务一律进入待办，开始当天由定时任务自动转入进行中
	today := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local)
	if workDate == nil || workDate.After(today) {
		status = "todo"
	}

	assigneeID := body.AssigneeID
	if assigneeID == nil {
		assigneeID = &user.ID
	} else {
		var u model.User
		if err := h.db.First(&u, *assigneeID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "负责人不存在"})
			return
		}
	}

	item := model.WorkItem{
		Title:      body.Title,
		Content:    body.Content,
		Detail:     body.Detail,
		ProjectID:  body.ProjectID,
		AuthorID:   user.ID,
		AssigneeID: assigneeID,
		Kind:       kind,
		Status:     status,
		Priority:   priority,
		WorkDate:   workDate,
		DueDate:    dueDate,
		DueRemind:  body.DueRemind,
		// 开始提醒仅对「未来开始日期」有意义，其他情况强制关闭
		StartRemind: body.StartRemind && workDate != nil && workDate.After(today),
	}
	if len(body.ParticipantIDs) > 0 {
		var participants []model.User
		h.db.Where("id IN ?", body.ParticipantIDs).Find(&participants)
		item.Participants = participants
	}
	if status == "done" {
		now := time.Now()
		item.DoneAt = &now
	}

	if err := h.db.Create(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.db.Preload("Project").Preload("Author").Preload("Assignee").Preload("Participants").First(&item, item.ID)
	c.JSON(http.StatusCreated, item)
}

// Update 编辑工作内容（标题/内容/优先级/日期）
func (h *WorkItemHandler) Update(c *gin.Context) {
	user := middleware.CurrentUser(c)
	id, _ := strconv.Atoi(c.Param("id"))
	var item model.WorkItem
	if err := h.db.First(&item, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "条目不存在"})
		return
	}
	if item.AuthorID != user.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "只能编辑本人提交的条目"})
		return
	}

	var body struct {
		Title          string  `json:"title"`
		Content        string  `json:"content"`
		Detail         *string `json:"detail"` // 指针区分未传与显式清空（取消勾选详细内容）
		Priority       string  `json:"priority"`
		WorkDate       string  `json:"work_date"`
		DueDate        string  `json:"due_date"`
		DueRemind      *bool   `json:"due_remind"`   // 指针区分未传与显式 false
		StartRemind    *bool   `json:"start_remind"` // 同上
		AssigneeID     *uint   `json:"assignee_id"`
		ParticipantIDs []uint  `json:"participant_ids"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	today := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local)
	updates := map[string]any{}
	if body.Title != "" {
		updates["title"] = body.Title
	}
	if body.Content != "" {
		updates["content"] = body.Content
	}
	if body.Detail != nil {
		updates["detail"] = *body.Detail
	}
	if body.Priority != "" {
		updates["priority"] = body.Priority
	}
	if body.WorkDate != "" {
		if d, err := parseDate(body.WorkDate); err == nil {
			updates["work_date"] = d
			// 待办任务在编辑中把开始日期排到「今天或之前」→ 保存后自动进入进行中
			if item.Status == "todo" && !d.After(today) {
				updates["status"] = "doing"
			}
		}
	}
	if body.DueDate != "" {
		if d, err := parseDate(body.DueDate); err == nil {
			updates["due_date"] = d
		}
	}
	if body.DueRemind != nil {
		updates["due_remind"] = *body.DueRemind
	}
	if body.StartRemind != nil {
		updates["start_remind"] = *body.StartRemind
	}
	if body.AssigneeID != nil {
		updates["assignee_id"] = *body.AssigneeID
	}
	if err := h.db.Model(&item).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if body.ParticipantIDs != nil {
		var participants []model.User
		h.db.Where("id IN ?", body.ParticipantIDs).Find(&participants)
		h.db.Model(&item).Association("Participants").Replace(participants)
	}
	h.db.Preload("Project").Preload("Author").Preload("Assignee").Preload("Participants").First(&item, item.ID)
	c.JSON(http.StatusOK, item)
}

// UpdateStatus 状态流转（看板拖拽/卡片菜单用）：todo / doing / done / cancelled
// 权限：仅发布者、负责人或管理员可变更状态
func (h *WorkItemHandler) UpdateStatus(c *gin.Context) {
	user := middleware.CurrentUser(c)
	id, _ := strconv.Atoi(c.Param("id"))
	var item model.WorkItem
	if err := h.db.First(&item, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "条目不存在"})
		return
	}
	var assigneeID uint
	if item.AssigneeID != nil {
		assigneeID = *item.AssigneeID
	}
	if item.AuthorID != user.ID && assigneeID != user.ID && !user.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "只有发布者、负责人或管理员可以变更状态"})
		return
	}

	var body struct {
		Status string `json:"status" binding:"required,oneof=todo doing done cancelled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	today := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local)
	updates := map[string]any{"status": body.Status}
	switch body.Status {
	case "done":
		now := time.Now()
		updates["done_at"] = &now
		// 未排期的待办任务直接完成：开始日期补为今天，保证报表/统计可聚合
		if item.WorkDate == nil {
			updates["work_date"] = today
		}
		// 完成时未填截止日期：自动补为当天
		if item.DueDate == nil {
			updates["due_date"] = today
		}
	case "todo":
		// 拖回待办 = 尚未排期：清空开始/截止日期与关联提醒
		updates["done_at"] = nil
		updates["work_date"] = nil
		updates["due_date"] = nil
		updates["due_remind"] = false
		updates["start_remind"] = false
	default: // doing / cancelled
		updates["done_at"] = nil
		// 待办任务开始推进：开始日期补为今天
		if body.Status == "doing" && item.WorkDate == nil {
			updates["work_date"] = today
		}
		// 已完成 → 进行中：截止日期已到期（≤今天）则清空（重开后旧截止日无意义）；未来截止日期保留
		if body.Status == "doing" && item.Status == "done" && item.DueDate != nil && !item.DueDate.After(today) {
			updates["due_date"] = nil
		}
	}
	if err := h.db.Model(&item).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.db.Preload("Project").Preload("Author").Preload("Assignee").Preload("Participants").First(&item, item.ID)
	c.JSON(http.StatusOK, item)
}

// Delete 删除（软删除，仅本人或管理员）
func (h *WorkItemHandler) Delete(c *gin.Context) {
	user := middleware.CurrentUser(c)
	id, _ := strconv.Atoi(c.Param("id"))
	var item model.WorkItem
	if err := h.db.First(&item, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "条目不存在"})
		return
	}
	if item.AuthorID != user.ID && !user.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "只有创建者或管理员可以删除"})
		return
	}
	if err := h.db.Delete(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// Restore 恢复软删除的条目（看板删除撤销用），仅本人或管理员
func (h *WorkItemHandler) Restore(c *gin.Context) {
	user := middleware.CurrentUser(c)
	id, _ := strconv.Atoi(c.Param("id"))
	var item model.WorkItem
	// Unscoped 才能查到已被软删除的记录
	if err := h.db.Unscoped().First(&item, id).Error; err != nil || !item.DeletedAt.Valid {
		c.JSON(http.StatusNotFound, gin.H{"error": "条目不存在或未被删除"})
		return
	}
	if item.AuthorID != user.ID && !user.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "只有创建者或管理员可以恢复"})
		return
	}
	if err := h.db.Unscoped().Model(&item).Update("deleted_at", nil).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.db.Preload("Project").Preload("Author").Preload("Assignee").Preload("Participants").First(&item, item.ID)
	c.JSON(http.StatusOK, item)
}
