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

type AIPromptHandler struct {
	db *gorm.DB
}

func NewAIPromptHandler(db *gorm.DB) *AIPromptHandler {
	return &AIPromptHandler{db: db}
}

func promptAdminOnly(c *gin.Context) bool {
	if !middleware.CurrentUser(c).IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅管理员可以管理提示词"})
		return false
	}
	return true
}

// List 提示词列表（登录用户可查：AI 分析页按报告类型加载默认提示词；内置的排前面）
func (h *AIPromptHandler) List(c *gin.Context) {
	var prompts []model.AIPrompt
	if err := h.db.Order("built_in desc, id asc").Find(&prompts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, prompts)
}

type aiPromptBody struct {
	Name       string `json:"name" binding:"required"`
	ReportType string `json:"report_type"` // week / year / 空（自定义主题，不联动报告类型）
	Content    string `json:"content" binding:"required"`
}

func validPromptReportType(t string) bool {
	return t == "" || t == "week" || t == "year"
}

// Create 新建提示词（仅管理员）
func (h *AIPromptHandler) Create(c *gin.Context) {
	if !promptAdminOnly(c) {
		return
	}
	var body aiPromptBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !validPromptReportType(body.ReportType) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "report_type 仅支持 week / year / 空"})
		return
	}
	p := model.AIPrompt{
		Name:       strings.TrimSpace(body.Name),
		ReportType: body.ReportType,
		Content:    body.Content,
	}
	if err := h.db.Create(&p).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, p)
}

// Update 编辑提示词（仅管理员）。
// 内置提示词的关联类型不可变更，保证周报/年度报告始终有默认提示词可联动
func (h *AIPromptHandler) Update(c *gin.Context) {
	if !promptAdminOnly(c) {
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	var p model.AIPrompt
	if err := h.db.First(&p, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "提示词不存在"})
		return
	}
	var body aiPromptBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !validPromptReportType(body.ReportType) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "report_type 仅支持 week / year / 空"})
		return
	}
	reportType := body.ReportType
	if p.BuiltIn {
		reportType = p.ReportType
	}
	if err := h.db.Model(&p).Updates(map[string]any{
		"name":        strings.TrimSpace(body.Name),
		"report_type": reportType,
		"content":     body.Content,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.db.First(&p, id)
	c.JSON(http.StatusOK, p)
}

// Delete 删除提示词（仅管理员；内置提示词不可删除，可编辑内容）
func (h *AIPromptHandler) Delete(c *gin.Context) {
	if !promptAdminOnly(c) {
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	var p model.AIPrompt
	if err := h.db.First(&p, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "提示词不存在"})
		return
	}
	if p.BuiltIn {
		c.JSON(http.StatusBadRequest, gin.H{"error": "内置提示词不可删除（可编辑内容）"})
		return
	}
	if err := h.db.Delete(&p).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
