package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"work-report/server/internal/middleware"
	"work-report/server/internal/model"
)

type ProjectHandler struct {
	db *gorm.DB
}

func NewProjectHandler(db *gorm.DB) *ProjectHandler {
	return &ProjectHandler{db: db}
}

// List 项目列表（默认不含已归档，?all=1 含归档）
func (h *ProjectHandler) List(c *gin.Context) {
	q := h.db.Preload("Owner")
	if c.Query("all") != "1" {
		q = q.Where("status = ?", "active")
	}
	var projects []model.Project
	if err := q.Order("created_at desc").Find(&projects).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, projects)
}

// Create 创建项目：名称 + 负责人必填
func (h *ProjectHandler) Create(c *gin.Context) {
	var body struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		OwnerID     uint   `json:"owner_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "项目名称与负责人必填: " + err.Error()})
		return
	}

	var owner model.User
	if err := h.db.First(&owner, body.OwnerID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "负责人不存在"})
		return
	}

	project := model.Project{
		Name:        body.Name,
		Description: body.Description,
		OwnerID:     body.OwnerID,
		Status:      "active",
	}
	if err := h.db.Create(&project).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "创建失败（项目名可能重复）: " + err.Error()})
		return
	}
	h.db.Preload("Owner").First(&project, project.ID)
	c.JSON(http.StatusCreated, project)
}

// Update 更新项目（名称/描述/负责人/归档状态），仅管理员
func (h *ProjectHandler) Update(c *gin.Context) {
	user := middleware.CurrentUser(c)
	if !user.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅管理员可以修改项目信息"})
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	var project model.Project
	if err := h.db.First(&project, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}

	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		OwnerID     uint   `json:"owner_id"`
		Status      string `json:"status"` // active / archived
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]any{}
	if body.Name != "" {
		updates["name"] = body.Name
	}
	if body.Description != "" {
		updates["description"] = body.Description
	}
	if body.OwnerID != 0 {
		updates["owner_id"] = body.OwnerID
	}
	if body.Status == "active" || body.Status == "archived" {
		updates["status"] = body.Status
	}
	if err := h.db.Model(&project).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.db.Preload("Owner").First(&project, project.ID)
	c.JSON(http.StatusOK, project)
}

// Delete 删除项目（软删除；已有工作内容的会被阻止），仅管理员
func (h *ProjectHandler) Delete(c *gin.Context) {
	user := middleware.CurrentUser(c)
	if !user.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅管理员可以删除项目"})
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	var count int64
	h.db.Model(&model.WorkItem{}).Where("project_id = ?", id).Count(&count)
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "项目下已有工作内容，请改为归档"})
		return
	}
	if err := h.db.Delete(&model.Project{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// Members 平台全部用户（选负责人/成员用），顺便给前端头像列表
func ListUsers(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var users []model.User
		if err := db.Preload("Role").Order("created_at asc").Find(&users).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, users)
	}
}
