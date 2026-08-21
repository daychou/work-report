package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"work-report/server/internal/middleware"
	"work-report/server/internal/model"
)

type AIModelHandler struct {
	db *gorm.DB
}

func NewAIModelHandler(db *gorm.DB) *AIModelHandler {
	return &AIModelHandler{db: db}
}

// adminOnly 仅管理员可管理 AI 模型
func adminOnly(c *gin.Context) bool {
	if !middleware.CurrentUser(c).IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅管理员可以管理 AI 模型"})
		return false
	}
	return true
}

// EnabledList 已启用模型列表（登录用户可用，AI 分析页选模型；不返回 api_key）
func (h *AIModelHandler) EnabledList(c *gin.Context) {
	var models []model.AIModel
	if err := h.db.Select("id", "name", "provider", "model_id", "base_url", "enabled").
		Where("enabled = ?", true).Order("id asc").Find(&models).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, models)
}

// List 全部模型（仅管理员，含 api_key 用于编辑回显）
func (h *AIModelHandler) List(c *gin.Context) {
	if !adminOnly(c) {
		return
	}
	var models []model.AIModel
	if err := h.db.Order("id asc").Find(&models).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, models)
}

type aiModelBody struct {
	Name     string `json:"name" binding:"required"`
	Provider string `json:"provider" binding:"required"`
	ModelID  string `json:"model_id" binding:"required"`
	APIKey   string `json:"api_key"`
	BaseURL  string `json:"base_url"`
	Enabled  bool   `json:"enabled"`
}

// Create 新建模型（仅管理员）
func (h *AIModelHandler) Create(c *gin.Context) {
	if !adminOnly(c) {
		return
	}
	var body aiModelBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	m := model.AIModel{
		Name:     body.Name,
		Provider: body.Provider,
		ModelID:  body.ModelID,
		APIKey:   body.APIKey,
		BaseURL:  body.BaseURL,
		Enabled:  body.Enabled,
	}
	if err := h.db.Create(&m).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, m)
}

// Update 编辑模型（仅管理员）
func (h *AIModelHandler) Update(c *gin.Context) {
	if !adminOnly(c) {
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	var m model.AIModel
	if err := h.db.First(&m, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "模型不存在"})
		return
	}
	var body aiModelBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updates := map[string]any{
		"name":     body.Name,
		"provider": body.Provider,
		"model_id": body.ModelID,
		"api_key":  body.APIKey,
		"base_url": body.BaseURL,
		"enabled":  body.Enabled,
	}
	if err := h.db.Model(&m).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.db.First(&m, id)
	c.JSON(http.StatusOK, m)
}

// Delete 删除模型（仅管理员；已被报告引用的模型不可删）
func (h *AIModelHandler) Delete(c *gin.Context) {
	if !adminOnly(c) {
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	var m model.AIModel
	if err := h.db.First(&m, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "模型不存在"})
		return
	}
	var cnt int64
	if err := h.db.Model(&model.AIReport{}).Where("ai_model_id = ?", id).Count(&cnt).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if cnt > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该模型已被生成的报告引用，不可删除（可改为停用）"})
		return
	}
	if err := h.db.Delete(&m).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
