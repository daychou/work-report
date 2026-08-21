package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"work-report/server/internal/middleware"
	"work-report/server/internal/model"
	"work-report/server/internal/service"
)

type AIReportHandler struct {
	db *gorm.DB
}

func NewAIReportHandler(db *gorm.DB) *AIReportHandler {
	return &AIReportHandler{db: db}
}

// DefaultAIPrompt 额外提示词的默认值（前端表单也内置同一份，用户可改）
const DefaultAIPrompt = "你是一名擅长帮助技术人员撰写年度工作总结的职业总结顾问。\n\n我会提供我一段时间的工作日报，请你不要简单地按照日期罗列工作，而是从全年日报中提炼我的工作成果、核心贡献、解决的问题、能力成长和明年的工作方向等。"

// Create 创建 AI 报告生成任务（异步执行，前端轮询状态；刷新页面不影响生成）
func (h *AIReportHandler) Create(c *gin.Context) {
	user := middleware.CurrentUser(c)
	var body struct {
		UserID      uint   `json:"user_id" binding:"required"`      // 执行人（被分析的同事）
		AIModelID   uint   `json:"ai_model_id" binding:"required"`
		ReportType  string `json:"report_type" binding:"required,oneof=week year"`
		DateFrom    string `json:"date_from" binding:"required"`
		DateTo      string `json:"date_to" binding:"required"`
		ExtraPrompt string `json:"extra_prompt"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	from, err := parseDate(body.DateFrom)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_from 格式应为 YYYY-MM-DD"})
		return
	}
	to, err := parseDate(body.DateTo)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_to 格式应为 YYYY-MM-DD"})
		return
	}
	if to.Before(from) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "结束日期不能早于开始日期"})
		return
	}

	var target model.User
	if err := h.db.First(&target, body.UserID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "执行人不存在"})
		return
	}

	var m model.AIModel
	if err := h.db.First(&m, body.AIModelID).Error; err != nil || !m.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "模型不存在或未启用"})
		return
	}
	if m.APIKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该模型未配置 API Key，请先在系统设置中完善"})
		return
	}

	// 数据量门槛：与预览同一套取数逻辑，不足 2 条直接拒绝，不进入异步生成
	items, err := h.queryReportItems(target.ID, from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(items) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "数据过少无法生成报告（该时间范围内已完成任务不足 2 条）"})
		return
	}

	report := model.AIReport{
		RequesterID: user.ID,
		UserID:      target.ID,
		AIModelID:   m.ID,
		ReportType:  body.ReportType,
		DateFrom:    from,
		DateTo:      to,
		ExtraPrompt: body.ExtraPrompt,
		Status:      "running",
	}
	if err := h.db.Create(&report).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	go h.generate(report.ID)

	h.db.Preload("Requester").Preload("User").Preload("AIModel").First(&report, report.ID)
	c.JSON(http.StatusCreated, report)
}

// queryReportItems 拉取执行人在时间范围内已完成（未删除）的任务：提交的或负责的。
// 生成报告与数据预览共用同一套取数逻辑，保证「所见即所喂给 AI」
func (h *AIReportHandler) queryReportItems(userID uint, from, to time.Time) ([]model.WorkItem, error) {
	var items []model.WorkItem
	err := h.db.Preload("Project").Preload("Author").Preload("Assignee").
		Where("status = ?", "done").
		Where("work_date >= ? AND work_date <= ?", from, to).
		Where("author_id = ? OR assignee_id = ?", userID, userID).
		Order("work_date asc").Limit(500).
		Find(&items).Error
	return items, err
}

// Preview 按执行人+时间范围预览将提交给 AI 的工作数据（生成报告前可先看喂了哪些数据）
func (h *AIReportHandler) Preview(c *gin.Context) {
	userID, _ := strconv.Atoi(c.Query("user_id"))
	from, errFrom := parseDate(c.Query("date_from"))
	to, errTo := parseDate(c.Query("date_to"))
	if userID <= 0 || errFrom != nil || errTo != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误：需要 user_id、date_from、date_to（YYYY-MM-DD）"})
		return
	}
	if to.Before(from) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "结束日期不能早于开始日期"})
		return
	}
	items, err := h.queryReportItems(uint(userID), from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

// List 报告列表：我发起的 + 以我为执行人的（管理员可见全部）
func (h *AIReportHandler) List(c *gin.Context) {
	user := middleware.CurrentUser(c)
	q := h.db.Preload("Requester").Preload("User").Preload("AIModel").
		Order("created_at desc").Limit(100)
	if !user.IsAdmin {
		q = q.Where("requester_id = ? OR user_id = ?", user.ID, user.ID)
	}
	var reports []model.AIReport
	if err := q.Find(&reports).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, reports)
}

// GetByID 单条查询（前端轮询生成状态用）
func (h *AIReportHandler) GetByID(c *gin.Context) {
	user := middleware.CurrentUser(c)
	id, _ := strconv.Atoi(c.Param("id"))
	var report model.AIReport
	if err := h.db.Preload("Requester").Preload("User").Preload("AIModel").First(&report, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "报告不存在"})
		return
	}
	if !user.IsAdmin && report.RequesterID != user.ID && report.UserID != user.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权查看该报告"})
		return
	}
	c.JSON(http.StatusOK, report)
}

// Delete 删除报告（发起人或管理员；生成中不可删除）。
// 仅删除报告记录：生成成功后写入执行人任务的【AI生成】条目属于执行人的工作数据，不随报告删除
func (h *AIReportHandler) Delete(c *gin.Context) {
	user := middleware.CurrentUser(c)
	id, _ := strconv.Atoi(c.Param("id"))
	var report model.AIReport
	if err := h.db.First(&report, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "报告不存在"})
		return
	}
	if report.RequesterID != user.ID && !user.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅报告发起人或管理员可删除"})
		return
	}
	if report.Status == "running" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "生成中的报告不可删除，请等待完成"})
		return
	}
	if err := h.db.Delete(&report).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// generate 异步生成：拉取执行人时间范围内已完成任务 → 调 AI → 存结果并写入执行人任务
func (h *AIReportHandler) generate(reportID uint) {
	var report model.AIReport
	if err := h.db.Preload("User").Preload("AIModel").First(&report, reportID).Error; err != nil {
		return
	}
	fail := func(err error) {
		h.db.Model(&report).Updates(map[string]any{"status": "failed", "error": err.Error()})
	}

	// 执行人在时间范围内已完成（且未删除）的任务：提交的或负责的
	items, err := h.queryReportItems(report.UserID, report.DateFrom, report.DateTo)
	if err != nil {
		fail(fmt.Errorf("查询工作数据失败: %w", err))
		return
	}
	if len(items) == 0 {
		fail(fmt.Errorf("该时间范围内 %s 没有已完成的工作数据", report.User.Name))
		return
	}

	result, err := callAI(&report, items)
	if err != nil {
		fail(err)
		return
	}

	// 把报告写进执行人的任务：标题带「AI生成」标签，负责人即执行人
	now := time.Now()
	typeName := "周报"
	if report.ReportType == "year" {
		typeName = "年度报告"
	}
	item := model.WorkItem{
		Title: fmt.Sprintf("【AI生成】%s（%s ~ %s）", typeName,
			report.DateFrom.Format("2006-01-02"), report.DateTo.Format("2006-01-02")),
		Content:    result,
		ProjectID:  items[0].ProjectID, // 挂靠到该周期内第一条任务所属项目
		AuthorID:   report.UserID,
		AssigneeID: &report.UserID,
		Kind:       "work",
		Status:     "done",
		Priority:   "medium",
		WorkDate:   now,
		DoneAt:     &now,
	}
	if err := h.db.Create(&item).Error; err != nil {
		fail(fmt.Errorf("报告生成成功但写入任务失败: %w", err))
		return
	}

	h.db.Model(&report).Updates(map[string]any{
		"status":       "done",
		"result":       result,
		"work_item_id": item.ID,
	})
}

// callAI 调用 OpenAI 兼容的 chat completions 接口（DeepSeek 等）
func callAI(report *model.AIReport, items []model.WorkItem) (string, error) {
	systemPrompt := strings.TrimSpace(report.ExtraPrompt)
	if systemPrompt == "" {
		systemPrompt = DefaultAIPrompt
	}

	var sb strings.Builder
	typeName := "周报"
	extraReq := "最后请给出下周的工作计划建议。"
	if report.ReportType == "year" {
		typeName = "年度总结报告"
		extraReq = "最后请给出明年的工作方向建议。"
	}
	fmt.Fprintf(&sb, "请根据以下工作数据生成一份%s。\n\n", typeName)
	fmt.Fprintf(&sb, "统计周期：%s 至 %s\n执行人：%s\n已完成任务数：%d\n\n工作数据（按日期排序）：\n",
		report.DateFrom.Format("2006-01-02"), report.DateTo.Format("2006-01-02"),
		report.User.Name, len(items))
	for _, it := range items {
		fmt.Fprintf(&sb, "- [%s]（项目：%s）%s\n", it.WorkDate.Format("2006-01-02"), it.Project.Name, it.Title)
		// 只取标题+正文（任务总结），详细内容不提交给 AI，控制数据量；
		// 正文为富文本 HTML，喂给 AI 前转纯文本
		if summary := service.StripHTML(strings.TrimSpace(it.Content)); summary != "" {
			fmt.Fprintf(&sb, "  正文：%s\n", summary)
		}
	}
	fmt.Fprintf(&sb, "\n要求：\n1. 不要简单按日期罗列，要提炼工作成果、核心贡献、解决的问题与能力成长\n2. %s\n3. 使用 Markdown 格式输出\n", extraReq)

	baseURL := strings.TrimRight(report.AIModel.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.deepseek.com/v1"
	}
	reqBody, _ := json.Marshal(map[string]any{
		"model": report.AIModel.ModelID,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": sb.String()},
		},
		"stream": false,
	})

	req, err := http.NewRequest(http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+report.AIModel.APIKey)

	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("调用 AI 接口失败: %w", err)
	}
	defer resp.Body.Close()
	respData, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("AI 接口返回 %d: %s", resp.StatusCode, string(respData))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respData, &parsed); err != nil || len(parsed.Choices) == 0 {
		return "", fmt.Errorf("AI 接口响应解析失败")
	}
	content := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if content == "" {
		return "", fmt.Errorf("AI 返回内容为空")
	}
	return content, nil
}
