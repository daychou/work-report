package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"work-report/server/internal/service"
)

type ReportHandler struct {
	db *gorm.DB
}

func NewReportHandler(db *gorm.DB) *ReportHandler {
	return &ReportHandler{db: db}
}

// Get 报表聚合：?period=day|week|month&date=YYYY-MM-DD&format=json|markdown
func (h *ReportHandler) Get(c *gin.Context) {
	period := c.DefaultQuery("period", "day")
	date := c.Query("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	from, to, label, err := service.ResolvePeriod(period, date)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	data, err := service.BuildReport(h.db, period, from, to, label)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if c.Query("format") == "markdown" {
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.String(http.StatusOK, service.RenderMarkdown(data))
		return
	}
	c.JSON(http.StatusOK, data)
}

// Stats 统计：?days=30
func (h *ReportHandler) Stats(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	data, err := service.BuildStats(h.db, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}
