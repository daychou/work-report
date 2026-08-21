package job

import (
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"

	"work-report/server/internal/model"
	"work-report/server/internal/notify"
)

// PlanRemindJob 计划到期提醒：扫描 kind=plan 且未完成且 due_date 临期/逾期的条目
// - 到期当天（due_today）提醒一次
// - 逾期后每天（overdue）提醒一次
// 平台内通知接收人：任务作者 + 项目负责人；飞书消息发给绑定了 open_id 的用户
type PlanRemindJob struct {
	db     *gorm.DB
	feishu *notify.FeishuClient
}

func NewPlanRemindJob(db *gorm.DB, feishu *notify.FeishuClient) *PlanRemindJob {
	return &PlanRemindJob{db: db, feishu: feishu}
}

// Run 每天 18:00 由 cron 触发一次（任务按日期粒度管理，无需高频扫描）；
// 服务启动后的补跑在 18 点前直接跳过，18 点后用于弥补重启错过的当天提醒
func (j *PlanRemindJob) Run() {
	if time.Now().Hour() < 18 {
		return
	}
	today := time.Now().Format("2006-01-02")
	todayDate, _ := time.ParseInLocation("2006-01-02", today, time.Local)

	var items []model.WorkItem
	if err := j.db.Preload("Author").Preload("Project").Preload("Project.Owner").
		Where("kind = 'plan'").
		Where("status NOT IN ('done', 'cancelled')").
		Where("due_date IS NOT NULL AND due_date <= ?", todayDate).
		Find(&items).Error; err != nil {
		log.Printf("[remind] scan failed: %v", err)
		return
	}

	for _, item := range items {
		kind := "due_today"
		if item.DueDate != nil && item.DueDate.Before(todayDate) {
			kind = "overdue"
		}

		// 去重：同一计划同一类型同一天只提醒一次
		logEntry := model.PlanRemindLog{WorkItemID: item.ID, Kind: kind, Day: today}
		res := j.db.Where(logEntry).FirstOrCreate(&logEntry)
		if res.RowsAffected == 0 {
			continue // 今天已提醒过
		}

		j.notify(item, kind)
	}

	j.runDueRemind(today, todayDate)
	// 开始提醒的补跑：正常由 12:00 的 cron 触发；若当时服务未运行，18:00 这班补发（去重表保证不重复）
	j.runStartRemind(today, todayDate)
	// 待办迁移的补跑：正常由 00:10 的 cron 触发
	j.MigrateTodoToDoing()
}

// RunStartRemind 每天 12:00 由 cron 触发：提醒今天开始的任务（启动补跑在 12 点前直接跳过）
func (j *PlanRemindJob) RunStartRemind() {
	if time.Now().Hour() < 12 {
		return
	}
	today := time.Now().Format("2006-01-02")
	todayDate, _ := time.ParseInLocation("2006-01-02", today, time.Local)
	j.MigrateTodoToDoing() // 12:00 班次顺带补跑待办迁移（幂等）
	j.runStartRemind(today, todayDate)
}

// MigrateTodoToDoing 待办任务到达开始日期后自动转入「进行中」。
// 幂等（只更新 todo 且 work_date 已到期的行），每天 00:10 由 cron 触发，
// 并在 12:00 提醒班次与服务启动时补跑，覆盖停机错过的场景
func (j *PlanRemindJob) MigrateTodoToDoing() {
	today := time.Now().Format("2006-01-02")
	todayDate, _ := time.ParseInLocation("2006-01-02", today, time.Local)
	res := j.db.Model(&model.WorkItem{}).
		Where("status = 'todo'").
		Where("work_date IS NOT NULL AND work_date <= ?", todayDate).
		Update("status", "doing")
	if res.Error != nil {
		log.Printf("[remind] todo->doing migration failed: %v", res.Error)
		return
	}
	if res.RowsAffected > 0 {
		log.Printf("[remind] todo->doing migration: %d task(s) started", res.RowsAffected)
	}
}

// runStartRemind 勾选「开始提醒」的任务：开始日期当天提醒作者与负责人（当天一次）
func (j *PlanRemindJob) runStartRemind(today string, todayDate time.Time) {
	var items []model.WorkItem
	if err := j.db.Preload("Author").Preload("Assignee").Preload("Project").
		Where("start_remind = ?", true).
		Where("status NOT IN ('done', 'cancelled')").
		Where("work_date = ?", todayDate).
		Find(&items).Error; err != nil {
		log.Printf("[remind] start_remind scan failed: %v", err)
		return
	}

	for _, item := range items {
		logEntry := model.PlanRemindLog{WorkItemID: item.ID, Kind: "start_remind_12h", Day: today}
		res := j.db.Where(logEntry).FirstOrCreate(&logEntry)
		if res.RowsAffected == 0 {
			continue // 今天已提醒过
		}

		title := fmt.Sprintf("任务开始提醒：%s", item.Title)
		content := fmt.Sprintf("项目【%s】的任务「%s」今天开始，请按计划推进。",
			item.Project.Name, item.Title)
		j.notifyAuthorAndAssignee(&item, "start_remind", title, content)
	}
}

// runDueRemind 勾选「到期提醒」的任务：截止日期当天提醒作者与负责人（当天一次）
func (j *PlanRemindJob) runDueRemind(today string, todayDate time.Time) {
	var items []model.WorkItem
	if err := j.db.Preload("Author").Preload("Assignee").Preload("Project").
		Where("due_remind = ?", true).
		Where("status NOT IN ('done', 'cancelled')").
		Where("due_date = ?", todayDate).
		Find(&items).Error; err != nil {
		log.Printf("[remind] due_remind scan failed: %v", err)
		return
	}

	for _, item := range items {
		logEntry := model.PlanRemindLog{WorkItemID: item.ID, Kind: "due_remind_18h", Day: today}
		res := j.db.Where(logEntry).FirstOrCreate(&logEntry)
		if res.RowsAffected == 0 {
			continue // 今天已提醒过
		}

		title := fmt.Sprintf("任务临期提醒：%s", item.Title)
		content := fmt.Sprintf("项目【%s】的任务「%s」今天截止，请及时跟进完成。",
			item.Project.Name, item.Title)
		j.notifyAuthorAndAssignee(&item, "due_remind", title, content)
	}
}

// notifyAuthorAndAssignee 站内通知 + 飞书推送给任务作者与负责人（需 Preload Author/Assignee/Project）
func (j *PlanRemindJob) notifyAuthorAndAssignee(item *model.WorkItem, notifType, title, content string) {
	// 接收人：作者 + 负责人（去重）
	receivers := map[uint]model.User{}
	receivers[item.AuthorID] = item.Author
	if item.AssigneeID != nil && *item.AssigneeID != item.AuthorID && item.Assignee != nil {
		receivers[*item.AssigneeID] = *item.Assignee
	}

	for _, u := range receivers {
		n := model.Notification{
			UserID:     u.ID,
			WorkItemID: &item.ID,
			Type:       notifType,
			Title:      title,
			Content:    content,
		}
		if err := j.db.Create(&n).Error; err != nil {
			log.Printf("[remind] create %s notification failed: %v", notifType, err)
		}
		if j.feishu.Enabled() && u.FeishuOpenID != "" {
			text := fmt.Sprintf("%s\n%s", title, content)
			if err := j.feishu.SendText(u.FeishuOpenID, text); err != nil {
				log.Printf("[remind] feishu send to %s failed: %v", u.Name, err)
			}
		}
	}
}

func (j *PlanRemindJob) notify(item model.WorkItem, kind string) {
	var title, content string
	if kind == "due_today" {
		title = fmt.Sprintf("计划今日到期：%s", item.Title)
		content = fmt.Sprintf("项目【%s】的计划「%s」今天到期，请及时跟进。",
			item.Project.Name, item.Title)
	} else {
		days := int(time.Since(*item.DueDate).Hours() / 24)
		title = fmt.Sprintf("计划已逾期 %d 天：%s", days, item.Title)
		content = fmt.Sprintf("项目【%s】的计划「%s」截止于 %s，已逾期 %d 天。",
			item.Project.Name, item.Title, item.DueDate.Format("2006-01-02"), days)
	}

	// 接收人：作者 + 项目负责人（去重）
	receivers := map[uint]model.User{}
	receivers[item.AuthorID] = item.Author
	if item.Project.OwnerID != item.AuthorID {
		receivers[item.Project.OwnerID] = item.Project.Owner
	}

	for _, u := range receivers {
		n := model.Notification{
			UserID:     u.ID,
			WorkItemID: &item.ID,
			Type:       "plan_" + kind,
			Title:      title,
			Content:    content,
		}
		if err := j.db.Create(&n).Error; err != nil {
			log.Printf("[remind] create notification failed: %v", err)
		}

		if j.feishu.Enabled() && u.FeishuOpenID != "" {
			text := fmt.Sprintf("%s\n%s", title, content)
			if err := j.feishu.SendText(u.FeishuOpenID, text); err != nil {
				log.Printf("[remind] feishu send to %s failed: %v", u.Name, err)
			}
		}
	}
}
