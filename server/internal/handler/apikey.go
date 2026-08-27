package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	apikeyutil "work-report/server/internal/apikey"
	"work-report/server/internal/middleware"
	"work-report/server/internal/model"
)

type APIKeyHandler struct {
	db *gorm.DB
}

func NewAPIKeyHandler(db *gorm.DB) *APIKeyHandler {
	return &APIKeyHandler{db: db}
}

// List 仅返回当前用户尚未吊销的 API key 元数据。
func (h *APIKeyHandler) List(c *gin.Context) {
	user := middleware.CurrentUser(c)
	var keys []model.UserAPIKey
	if err := h.db.Where("user_id = ?", user.ID).Order("created_at DESC").Find(&keys).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, keys)
}

type createAPIKeyResponse struct {
	model.UserAPIKey
	Key string `json:"key"`
}

// Create 创建 API key。明文 key 只在本次响应中返回，数据库仅保存其摘要。
func (h *APIKeyHandler) Create(c *gin.Context) {
	user := middleware.CurrentUser(c)
	var body struct {
		Name      string     `json:"name" binding:"required"`
		ExpiresAt *time.Time `json:"expires_at"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name 必填，expires_at 必须为 RFC3339 时间"})
		return
	}

	name := strings.TrimSpace(body.Name)
	if name == "" || utf8.RuneCountInString(name) > 128 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name 长度必须为 1-128 个字符"})
		return
	}
	if body.ExpiresAt != nil && !body.ExpiresAt.After(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "expires_at 必须晚于当前时间"})
		return
	}

	plainKey, err := apikeyutil.Generate()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成 API key 失败"})
		return
	}
	key := model.UserAPIKey{
		UserID:    user.ID,
		Name:      name,
		KeyHash:   apikeyutil.Hash(plainKey),
		KeyPrefix: apikeyutil.VisiblePrefix(plainKey),
		ExpiresAt: body.ExpiresAt,
	}
	if err := h.db.Create(&key).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, createAPIKeyResponse{UserAPIKey: key, Key: plainKey})
}

// Delete 软删除当前用户自己的 API key。
func (h *APIKeyHandler) Delete(c *gin.Context) {
	user := middleware.CurrentUser(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 API key ID"})
		return
	}

	result := h.db.Where("id = ? AND user_id = ?", id, user.ID).Delete(&model.UserAPIKey{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key 不存在"})
		return
	}
	c.Status(http.StatusNoContent)
}
