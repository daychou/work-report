package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"work-report/server/internal/middleware"
	"work-report/server/internal/model"
)

type RoleHandler struct {
	db *gorm.DB
}

func NewRoleHandler(db *gorm.DB) *RoleHandler {
	return &RoleHandler{db: db}
}

// List 角色列表（含成员数）。所有登录用户可读（成员编辑下拉等场景用），写操作仅管理员
func (h *RoleHandler) List(c *gin.Context) {
	var roles []model.Role
	if err := h.db.Order("built_in desc, id asc").Find(&roles).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	type row struct {
		RoleID uint
		Cnt    int64
	}
	var rows []row
	if err := h.db.Model(&model.User{}).
		Select("role_id, COUNT(*) AS cnt").
		Where("role_id IS NOT NULL").
		Group("role_id").
		Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	byID := make(map[uint]int64, len(rows))
	for _, r := range rows {
		byID[r.RoleID] = r.Cnt
	}
	type roleWithCount struct {
		model.Role
		MemberCount int64 `json:"member_count"`
	}
	out := make([]roleWithCount, len(roles))
	for i, r := range roles {
		out[i] = roleWithCount{Role: r, MemberCount: byID[r.ID]}
	}
	c.JSON(http.StatusOK, out)
}

// Create 新建角色（仅管理员）
func (h *RoleHandler) Create(c *gin.Context) {
	if !middleware.CurrentUser(c).IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅管理员可以管理角色"})
		return
	}
	var body struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		IsAdmin     bool   `json:"is_admin"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "角色名称必填"})
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "角色名称不能为空"})
		return
	}
	var count int64
	h.db.Model(&model.Role{}).Where("name = ?", name).Count(&count)
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "已存在同名角色"})
		return
	}
	role := model.Role{Name: name, Description: strings.TrimSpace(body.Description), IsAdmin: body.IsAdmin}
	if err := h.db.Create(&role).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, role)
}

// Update 编辑角色（仅管理员）。内置角色仅允许修改描述，权限标识不可变更
func (h *RoleHandler) Update(c *gin.Context) {
	if !middleware.CurrentUser(c).IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅管理员可以管理角色"})
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	var role model.Role
	if err := h.db.First(&role, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "角色不存在"})
		return
	}
	var body struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		IsAdmin     bool   `json:"is_admin"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "角色名称必填"})
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "角色名称不能为空"})
		return
	}

	updates := map[string]any{"description": strings.TrimSpace(body.Description)}
	if role.BuiltIn {
		if name != role.Name || body.IsAdmin != role.IsAdmin {
			c.JSON(http.StatusBadRequest, gin.H{"error": "内置角色仅允许修改描述"})
			return
		}
	} else {
		var count int64
		h.db.Model(&model.Role{}).Where("name = ? AND id != ?", name, role.ID).Count(&count)
		if count > 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "已存在同名角色"})
			return
		}
		updates["name"] = name
		updates["is_admin"] = body.IsAdmin
	}
	if err := h.db.Model(&role).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// 权限标识变更时，同步该角色下所有成员的 is_admin 冗余字段
	if v, ok := updates["is_admin"]; ok && v != role.IsAdmin {
		if err := h.db.Model(&model.User{}).Where("role_id = ?", role.ID).
			Update("is_admin", v).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	h.db.First(&role, role.ID)
	c.JSON(http.StatusOK, role)
}

// Delete 删除角色（仅管理员）。内置角色不可删；仍有成员的角色不可删
func (h *RoleHandler) Delete(c *gin.Context) {
	if !middleware.CurrentUser(c).IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅管理员可以管理角色"})
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	var role model.Role
	if err := h.db.First(&role, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "角色不存在"})
		return
	}
	if role.BuiltIn {
		c.JSON(http.StatusBadRequest, gin.H{"error": "内置角色不可删除"})
		return
	}
	var count int64
	h.db.Model(&model.User{}).Where("role_id = ?", role.ID).Count(&count)
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该角色下仍有成员，请先调整成员角色"})
		return
	}
	if err := h.db.Delete(&role).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
