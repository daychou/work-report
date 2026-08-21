package service

import (
	"time"

	"gorm.io/gorm"

	"work-report/server/internal/model"
)

// StatsOverview 统计页数据
type StatsOverview struct {
	ByUser        []UserStat `json:"by_user"`
	ByProject     []ProjStat `json:"by_project"`
	DailyTrend    []DayStat  `json:"daily_trend"`
	TotalWork     int64      `json:"total_work"`
	TotalProjects int64      `json:"total_projects"`
	TotalUsers    int64      `json:"total_users"`
}

type UserStat struct {
	UserID   uint   `json:"user_id"`
	UserName string `json:"user_name"`
	WorkCnt  int64  `json:"work_cnt"`
	DoneCnt  int64  `json:"done_cnt"`
}

type ProjStat struct {
	ProjectID   uint   `json:"project_id"`
	ProjectName string `json:"project_name"`
	Cnt         int64  `json:"cnt"`
}

type DayStat struct {
	Day string `json:"day"`
	Cnt int64  `json:"cnt"`
}

// BuildStats 统计近 days 天的数据
func BuildStats(db *gorm.DB, days int) (*StatsOverview, error) {
	if days <= 0 {
		days = 30
	}
	from := time.Now().AddDate(0, 0, -days+1)
	from = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.Local)

	s := &StatsOverview{}

	// 按人统计（任务已不区分工作/计划类型，统一统计条目数）
	var userStats []UserStat
	if err := db.Model(&model.WorkItem{}).
		Select("work_items.author_id AS user_id, users.name AS user_name, "+
			"COUNT(*) AS work_cnt, "+
			"SUM(CASE WHEN work_items.status = 'done' THEN 1 ELSE 0 END) AS done_cnt").
		Joins("JOIN users ON users.id = work_items.author_id").
		Where("work_items.work_date >= ?", from).
		Where("work_items.status != ?", "cancelled").
		Group("work_items.author_id").
		Order("COUNT(*) DESC").
		Scan(&userStats).Error; err != nil {
		return nil, err
	}
	s.ByUser = userStats

	// 按项目分布
	var projStats []ProjStat
	if err := db.Model(&model.WorkItem{}).
		Select("work_items.project_id, projects.name AS project_name, COUNT(*) AS cnt").
		Joins("JOIN projects ON projects.id = work_items.project_id").
		Where("work_items.work_date >= ?", from).
		Where("work_items.status != ?", "cancelled").
		Group("work_items.project_id").
		Order("cnt DESC").
		Scan(&projStats).Error; err != nil {
		return nil, err
	}
	s.ByProject = projStats

	// 每日提交趋势（补零）
	var trend []DayStat
	if err := db.Model(&model.WorkItem{}).
		Select("DATE_FORMAT(work_date, '%Y-%m-%d') AS day, COUNT(*) AS cnt").
		Where("work_date >= ?", from).
		Where("status != ?", "cancelled").
		Group("day").
		Order("day").
		Scan(&trend).Error; err != nil {
		return nil, err
	}
	trendMap := map[string]int64{}
	for _, t := range trend {
		trendMap[t.Day] = t.Cnt
	}
	for i := 0; i < days; i++ {
		d := from.AddDate(0, 0, i).Format("2006-01-02")
		s.DailyTrend = append(s.DailyTrend, DayStat{Day: d, Cnt: trendMap[d]})
	}

	db.Model(&model.WorkItem{}).Where("work_date >= ? AND status != 'cancelled'", from).Count(&s.TotalWork)
	db.Model(&model.Project{}).Where("status = 'active'").Count(&s.TotalProjects)
	db.Model(&model.User{}).Count(&s.TotalUsers)
	return s, nil
}
