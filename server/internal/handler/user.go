package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"work-report/server/internal/config"
	"work-report/server/internal/middleware"
	"work-report/server/internal/model"
)

// ManualUserPrefix 手动创建（尚未通过 Casdoor 登录）用户的 casdoor_id 前缀；
// Casdoor 登录时若名称相同会自动关联绑定，此前缀被真实 casdoor_id 替换
const ManualUserPrefix = "manual:"

type UserHandler struct {
	db  *gorm.DB
	cfg *config.Config
}

func NewUserHandler(db *gorm.DB, cfg *config.Config) *UserHandler {
	return &UserHandler{db: db, cfg: cfg}
}

// Create 管理员手动创建成员（预注册）。成员之后用 Casdoor 登录且名称相同，会自动关联到该账号
func (h *UserHandler) Create(c *gin.Context) {
	user := middleware.CurrentUser(c)
	if !user.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅管理员可以创建成员"})
		return
	}
	var body struct {
		Name  string `json:"name" binding:"required"`
		Email string `json:"email"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "成员名称必填"})
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "成员名称不能为空"})
		return
	}
	var count int64
	h.db.Model(&model.User{}).Where("name = ?", name).Count(&count)
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "已存在同名成员"})
		return
	}
	u := model.User{CasdoorID: ManualUserPrefix + name, Name: name, Email: body.Email, RoleID: defaultUserRoleID(h.db)}
	if err := h.db.Create(&u).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.db.Preload("Role").First(&u, u.ID)
	c.JSON(http.StatusCreated, u)
}

// defaultUserRoleID 内置「普通用户」角色 ID（不存在时返回 nil，由 seed 保证存在）
func defaultUserRoleID(db *gorm.DB) *uint {
	var role model.Role
	if err := db.Where("name = ?", model.RoleUserName).First(&role).Error; err != nil {
		return nil
	}
	return &role.ID
}

// Impersonate 管理员模拟指定成员身份：签发 uid=目标用户、imp_by=管理员 的会话 token，
// 之后所有 API 均以目标用户视角执行，便于排查问题
func (h *UserHandler) Impersonate(c *gin.Context) {
	admin := middleware.CurrentUser(c)
	if !admin.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅管理员可以模拟身份"})
		return
	}
	// 模拟会话中不允许再次模拟（避免嵌套导致无法回到真实管理员）
	if admin.ImpersonatedBy != 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请先退出当前模拟身份"})
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	if uint(id) == admin.ID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能模拟自己"})
		return
	}
	var target model.User
	if err := h.db.First(&target, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "成员不存在"})
		return
	}
	token, err := middleware.SignImpersonatedSession(target.ID, admin.ID, h.cfg.JWT.Secret, h.cfg.JWT.ExpireHours)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	target.ImpersonatedBy = admin.ID
	c.JSON(http.StatusOK, gin.H{"token": token, "user": target})
}

// Update 修改成员资料：管理员可改任何人（含角色），本人可改自己的名称/邮箱
func (h *UserHandler) Update(c *gin.Context) {
	user := middleware.CurrentUser(c)
	id, _ := strconv.Atoi(c.Param("id"))

	var target model.User
	if err := h.db.First(&target, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "成员不存在"})
		return
	}
	if !user.IsAdmin && user.ID != target.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "只能修改自己的名称"})
		return
	}

	var body struct {
		Name   string `json:"name" binding:"required"`
		Email  string `json:"email"`
		RoleID *uint  `json:"role_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "成员名称必填"})
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "成员名称不能为空"})
		return
	}
	var count int64
	h.db.Model(&model.User{}).Where("name = ? AND id != ?", name, target.ID).Count(&count)
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "已存在同名成员"})
		return
	}

	updates := map[string]any{"name": name}
	if body.Email != "" {
		updates["email"] = body.Email
	}
	// 手动创建的用户改名时同步 casdoor_id 占位符，保持可关联性
	if strings.HasPrefix(target.CasdoorID, ManualUserPrefix) {
		updates["casdoor_id"] = ManualUserPrefix + name
	}
	// 角色调整仅管理员可操作；同步 is_admin 冗余字段
	if body.RoleID != nil {
		if !user.IsAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "仅管理员可以调整成员角色"})
			return
		}
		var role model.Role
		if err := h.db.First(&role, *body.RoleID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "角色不存在"})
			return
		}
		// 防止管理员把自己降级，导致平台失去管理入口
		if target.ID == user.ID && !role.IsAdmin {
			c.JSON(http.StatusBadRequest, gin.H{"error": "不能取消自己的管理员角色"})
			return
		}
		updates["role_id"] = role.ID
		updates["is_admin"] = role.IsAdmin
	}
	if err := h.db.Model(&target).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.db.Preload("Role").First(&target, target.ID)
	c.JSON(http.StatusOK, target)
}
