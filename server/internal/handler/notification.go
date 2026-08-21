package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"work-report/server/internal/middleware"
	"work-report/server/internal/model"
)

type NotificationHandler struct {
	db *gorm.DB
}

func NewNotificationHandler(db *gorm.DB) *NotificationHandler {
	return &NotificationHandler{db: db}
}

// List 本人的平台内通知（新→旧）
func (h *NotificationHandler) List(c *gin.Context) {
	user := middleware.CurrentUser(c)
	var list []model.Notification
	if err := h.db.Preload("WorkItem").
		Where("user_id = ?", user.ID).
		Order("created_at desc").
		Limit(100).
		Find(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

// UnreadCount 未读数（顶栏红点）
func (h *NotificationHandler) UnreadCount(c *gin.Context) {
	user := middleware.CurrentUser(c)
	var count int64
	h.db.Model(&model.Notification{}).
		Where("user_id = ? AND `read` = 0", user.ID).
		Count(&count)
	c.JSON(http.StatusOK, gin.H{"count": count})
}

// MarkRead 标记已读（单条或全部）
func (h *NotificationHandler) MarkRead(c *gin.Context) {
	user := middleware.CurrentUser(c)
	id := c.Param("id")
	q := h.db.Model(&model.Notification{}).Where("user_id = ?", user.ID)
	if id != "all" {
		n, _ := strconv.Atoi(id)
		q = q.Where("id = ?", n)
	}
	if err := q.Update("read", true).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
