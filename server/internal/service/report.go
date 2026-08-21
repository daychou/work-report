package service

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"work-report/server/internal/model"
)

// ReportPeriod 日/周/月
func ResolvePeriod(period, date string) (from, to time.Time, label string, err error) {
	d, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return from, to, label, fmt.Errorf("date 格式应为 YYYY-MM-DD")
	}
	switch period {
	case "day":
		return d, d, d.Format("2006-01-02"), nil
	case "week":
		// 以周一为一周开始
		offset := (int(d.Weekday()) + 6) % 7
		monday := d.AddDate(0, 0, -offset)
		sunday := monday.AddDate(0, 0, 6)
		return monday, sunday, monday.Format("2006-01-02") + " ~ " + sunday.Format("2006-01-02"), nil
	case "month":
		first := time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, time.Local)
		last := first.AddDate(0, 1, -1)
		return first, last, d.Format("2006-01"), nil
	default:
		return from, to, label, fmt.Errorf("period 应为 day/week/month")
	}
}

type ReportData struct {
	Period    string               `json:"period"`
	Label     string               `json:"label"`
	From      string               `json:"from"`
	To        string               `json:"to"`
	ByUser    []UserReportGroup    `json:"by_user"`
	ByProject []ProjectReportGroup `json:"by_project"`
	Summary   ReportSummary        `json:"summary"`
}

type UserReportGroup struct {
	User  model.User       `json:"user"`
	Works []model.WorkItem `json:"works"` // kind=work
	Plans []model.WorkItem `json:"plans"` // kind=plan
}

type ProjectReportGroup struct {
	Project model.Project    `json:"project"`
	Works   []model.WorkItem `json:"works"`
	Plans   []model.WorkItem `json:"plans"`
}

type ReportSummary struct {
	WorkCount      int     `json:"work_count"`
	PlanCount      int     `json:"plan_count"`
	PlanDoneCount  int     `json:"plan_done_count"`
	PlanDoneRate   float64 `json:"plan_done_rate"` // 0~100
	ActiveUsers    int     `json:"active_users"`
	ActiveProjects int     `json:"active_projects"`
}

// BuildReport 聚合 [from,to] 区间内的工作内容
func BuildReport(db *gorm.DB, period string, from, to time.Time, label string) (*ReportData, error) {
	var items []model.WorkItem
	if err := db.Preload("Author").Preload("Project").
		Where("work_date >= ? AND work_date <= ?", from, to).
		Where("status != ?", "cancelled").
		Order("author_id, project_id, work_date").
		Find(&items).Error; err != nil {
		return nil, err
	}

	data := &ReportData{
		Period: period,
		Label:  label,
		From:   from.Format("2006-01-02"),
		To:     to.Format("2006-01-02"),
	}

	userGroups := map[uint]*UserReportGroup{}
	projectGroups := map[uint]*ProjectReportGroup{}
	planDone := 0

	for _, it := range items {
		// 按人分组
		ug, ok := userGroups[it.AuthorID]
		if !ok {
			ug = &UserReportGroup{User: it.Author}
			userGroups[it.AuthorID] = ug
		}
		// 按项目分组
		pg, ok := projectGroups[it.ProjectID]
		if !ok {
			pg = &ProjectReportGroup{Project: it.Project}
			projectGroups[it.ProjectID] = pg
		}

		if it.Kind == "work" {
			ug.Works = append(ug.Works, it)
			pg.Works = append(pg.Works, it)
			data.Summary.WorkCount++
		} else {
			ug.Plans = append(ug.Plans, it)
			pg.Plans = append(pg.Plans, it)
			data.Summary.PlanCount++
			if it.Status == "done" {
				planDone++
			}
		}
	}

	for _, g := range userGroups {
		data.ByUser = append(data.ByUser, *g)
	}
	for _, g := range projectGroups {
		data.ByProject = append(data.ByProject, *g)
	}

	data.Summary.PlanDoneCount = planDone
	if data.Summary.PlanCount > 0 {
		data.Summary.PlanDoneRate = float64(planDone) / float64(data.Summary.PlanCount) * 100
	}
	data.Summary.ActiveUsers = len(data.ByUser)
	data.Summary.ActiveProjects = len(data.ByProject)
	return data, nil
}

var statusCN = map[string]string{
	"todo":      "待办",
	"doing":     "进行中",
	"done":      "已完成",
	"cancelled": "已取消",
}

var priorityCN = map[string]string{
	"high":   "高",
	"medium": "中",
	"low":    "低",
}

// RenderMarkdown 把报表渲染成 Markdown，方便粘贴到 IM/邮件/文档
func RenderMarkdown(data *ReportData) string {
	var b strings.Builder
	periodCN := map[string]string{"day": "日报", "week": "周报", "month": "月报"}[data.Period]
	fmt.Fprintf(&b, "# 工作%s（%s）\n\n", periodCN, data.Label)
	fmt.Fprintf(&b, "> 工作内容 %d 条｜计划 %d 条（完成 %d，完成率 %.0f%%）｜参与 %d 人 / %d 个项目\n\n",
		data.Summary.WorkCount, data.Summary.PlanCount, data.Summary.PlanDoneCount,
		data.Summary.PlanDoneRate, data.Summary.ActiveUsers, data.Summary.ActiveProjects)

	for _, g := range data.ByUser {
		fmt.Fprintf(&b, "## %s\n\n", g.User.Name)
		if len(g.Works) > 0 {
			b.WriteString("### 工作内容\n\n")
			for _, w := range g.Works {
				fmt.Fprintf(&b, "- 【%s】%s（%s）\n", w.Project.Name, w.Title, w.WorkDate.Format("01-02"))
				if detail := StripHTML(w.Content); detail != "" {
					fmt.Fprintf(&b, "  - %s\n", detail)
				}
			}
			b.WriteString("\n")
		}
		if len(g.Plans) > 0 {
			b.WriteString("### 计划\n\n")
			for _, p := range g.Plans {
				due := ""
				if p.DueDate != nil {
					due = "，截止 " + p.DueDate.Format("01-02")
				}
				fmt.Fprintf(&b, "- 【%s】%s（%s，优先级 %s%s）\n",
					p.Project.Name, p.Title, statusCN[p.Status], priorityCN[p.Priority], due)
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}
