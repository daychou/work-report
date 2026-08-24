package model

import (
	"time"

	"gorm.io/gorm"
)

// 内置角色名（seed 时创建，不可删除）
const (
	RoleAdminName = "admin" // 管理员
	RoleUserName  = "user"  // 普通用户
)

// LocalUserPrefix 本地账号（账号密码登录）用户的 casdoor_id 前缀
const LocalUserPrefix = "local:"

// Role 平台内部角色。权限不再走 Casdoor：Casdoor 仅作认证登录，
// 是否管理员由用户关联角色的 IsAdmin 标识决定（同步冗余到 User.IsAdmin）
type Role struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Name        string         `gorm:"size:64;not null;uniqueIndex" json:"name"`
	Description string         `gorm:"size:256" json:"description"`
	IsAdmin     bool           `gorm:"default:false" json:"is_admin"` // 管理员角色：拥有系统设置与全部数据权限
	BuiltIn     bool           `gorm:"default:false" json:"built_in"` // 内置角色：不可删除、不可变更权限标识
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// User 平台用户，首次 Casdoor 登录时自动创建
type User struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	CasdoorID    string `gorm:"uniqueIndex;size:128" json:"casdoor_id"` // Casdoor 侧唯一标识；本地账号为 local:用户名；开发模式为用户名
	Name         string `gorm:"size:64;not null" json:"name"`
	Avatar       string `gorm:"size:512" json:"avatar"`
	Email        string `gorm:"size:128" json:"email"`
	FeishuOpenID string `gorm:"size:128" json:"feishu_open_id"` // 绑定飞书后用于接收提醒
	// IsAdmin 由关联角色的 IsAdmin 派生（冗余字段，角色变更时同步，权限判断统一用它）
	IsAdmin bool `gorm:"default:false" json:"is_admin"`
	// PasswordHash 本地账号密码（bcrypt），Casdoor 登录用户为空
	PasswordHash string `gorm:"size:128" json:"-"`
	// MustChangePassword 为 true 时登录后强制跳转到修改密码页（初始密码场景）
	MustChangePassword bool  `gorm:"default:false" json:"must_change_password"`
	RoleID             *uint `gorm:"index" json:"role_id"`
	Role               *Role `gorm:"foreignKey:RoleID" json:"role"`
	// ImpersonatedBy 非 0 表示当前会话是管理员模拟身份（不入库，由中间件从 JWT 注入）
	ImpersonatedBy uint           `gorm:"-" json:"impersonated_by,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

// Project 项目，需提前创建，必须指定名称与负责人
type Project struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Name        string         `gorm:"size:128;not null;uniqueIndex" json:"name"`
	Description string         `gorm:"size:1024" json:"description"`
	OwnerID     uint           `gorm:"not null;index" json:"owner_id"` // 负责人
	Owner       User           `gorm:"foreignKey:OwnerID" json:"owner"`
	Status      string         `gorm:"size:16;default:active;index" json:"status"` // active / archived
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// WorkItem 工作内容/计划 统一模型
// kind = plan（计划，面向未来，带 due_date）/ work（当日工作内容）
// status = todo / doing / done / cancelled
type WorkItem struct {
	ID      uint   `gorm:"primaryKey" json:"id"`
	Title   string `gorm:"size:256;not null" json:"title"`
	Content string `gorm:"type:text" json:"content"` // 正文：任务总结（AI 分析与报表导出只取标题+正文，控制数据量）
	// Detail 详细内容：可选的第二层内容（细节/截图/日志等），仅详情页展示，不提交给 AI
	Detail    string  `gorm:"type:text" json:"detail"`
	ProjectID uint    `gorm:"not null;index" json:"project_id"`
	Project   Project `gorm:"foreignKey:ProjectID" json:"project"`
	AuthorID  uint    `gorm:"not null;index" json:"author_id"` // 提交人
	Author    User    `gorm:"foreignKey:AuthorID" json:"author"`
	// 负责人（默认提交人）与参与人：看板可见性 = 提交人/负责人/参与人，管理员全见
	AssigneeID   *uint          `gorm:"index" json:"assignee_id"`
	Assignee     *User          `gorm:"foreignKey:AssigneeID" json:"assignee"`
	Participants []User         `gorm:"many2many:work_item_participants" json:"participants"`
	Kind         string         `gorm:"size:16;not null;index" json:"kind"`                // plan / work
	Status       string         `gorm:"size:16;not null;default:todo;index" json:"status"` // todo / doing / done / cancelled
	Priority     string         `gorm:"size:8;default:medium" json:"priority"`             // high / medium / low
	// WorkDate 开始日期（工作发生日，日报/周报聚合依据）；待办任务可为 NULL，表示尚未排期
	WorkDate     *time.Time     `gorm:"type:date;index" json:"work_date"`
	DueDate      *time.Time     `gorm:"type:date;index" json:"due_date"`                   // 截止日期（到期提醒依据）
	// DueRemind 勾选后：截止日期当天 18:00 平台内 + 飞书提醒作者与负责人任务快到期
	DueRemind bool `gorm:"default:false" json:"due_remind"`
	// StartRemind 勾选后：开始日期当天 12:00 平台内 + 飞书提醒作者与负责人任务开始（仅开始日期为未来时可勾选）
	StartRemind bool           `gorm:"default:false" json:"start_remind"`
	DoneAt      *time.Time     `json:"done_at"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
	// CommentCount 评论数（不入库，List 接口统计填充，看板卡片展示用）
	CommentCount int64 `gorm:"-" json:"comment_count"`
}

// Comment 工作内容下的评论；任何登录用户可发表，仅评论作者或管理员可删除
// ParentID 非空表示回复：统一挂在顶级评论下（回复的回复也指向顶级评论，保持两层结构）
type Comment struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	WorkItemID uint           `gorm:"not null;index" json:"work_item_id"`
	WorkItem   WorkItem       `gorm:"foreignKey:WorkItemID" json:"-"`
	ParentID   *uint          `gorm:"index" json:"parent_id"`
	AuthorID   uint           `gorm:"not null;index" json:"author_id"`
	Author     User           `gorm:"foreignKey:AuthorID" json:"author"`
	Content    string         `gorm:"type:text;not null" json:"content"`
	CreatedAt  time.Time      `json:"created_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

// Notification 平台内通知（到期提醒等）
type Notification struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `gorm:"not null;index" json:"user_id"` // 接收人
	User       User      `gorm:"foreignKey:UserID" json:"user"`
	WorkItemID *uint     `gorm:"index" json:"work_item_id"`
	WorkItem   *WorkItem `gorm:"foreignKey:WorkItemID" json:"work_item,omitempty"`
	// CommentID 提及类通知对应的评论，用于跳转后直接定位到该评论
	CommentID *uint          `gorm:"index" json:"comment_id,omitempty"`
	Type      string         `gorm:"size:32;not null" json:"type"` // plan_due / plan_overdue
	Title     string         `gorm:"size:256;not null" json:"title"`
	Content   string         `gorm:"size:1024" json:"content"`
	Read      bool           `gorm:"default:false;index" json:"read"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// PlanRemindLog 记录某计划在某天已发过提醒，避免 cron 重复推送
type PlanRemindLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	WorkItemID uint      `gorm:"uniqueIndex:uk_item_kind_day" json:"work_item_id"`
	Kind       string    `gorm:"size:32;uniqueIndex:uk_item_kind_day" json:"kind"` // due_today / overdue / due_remind_18h / start_remind_12h
	Day        string    `gorm:"size:10;uniqueIndex:uk_item_kind_day" json:"day"`  // YYYY-MM-DD
	CreatedAt  time.Time `json:"created_at"`
}

// WorkItemParticipant 显式声明多对多中间表，仅为 user_id 加二级索引：
// 可见性过滤的子查询 WHERE user_id = ? 用不上 (work_item_id, user_id) 复合主键，
// 数据量大时会全表扫描
type WorkItemParticipant struct {
	WorkItemID uint `gorm:"primaryKey" json:"work_item_id"`
	UserID     uint `gorm:"primaryKey;index" json:"user_id"`
}

func (WorkItemParticipant) TableName() string { return "work_item_participants" }

// AIModel 第三方 AI 大模型配置（系统设置中管理，仅管理员可写）。
// 仅 Enabled 的模型可在「AI 分析」页被选用；APIKey 仅管理员接口返回。
type AIModel struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Name      string         `gorm:"size:64;not null" json:"name"`           // 显示名，如 DeepSeek V4 Flash
	Provider  string         `gorm:"size:32;not null" json:"provider"`       // deepseek / ...（预留扩展）
	ModelID   string         `gorm:"size:64;not null" json:"model_id"`       // 调 API 时的 model 参数
	APIKey    string         `gorm:"size:256" json:"api_key,omitempty"`      // 仅管理员接口返回
	BaseURL   string         `gorm:"size:256" json:"base_url"`               // 可空，缺省按 provider 默认
	Enabled   bool           `gorm:"default:false" json:"enabled"`           // 启用后才可被选用
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// AIPrompt AI 分析提示词（系统设置中管理，写操作仅管理员；列表登录用户可查）。
// 内置周报（week）/ 年度报告（year）两条：AI 分析页按报告类型加载对应默认提示词；
// 内置提示词可编辑名称与内容，但不可删除、不可变更关联类型，保证类型联动始终有默认值。
// ReportType 为空表示自定义主题，不联动报告类型，可在 AI 分析页手动选用。
type AIPrompt struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	Name       string         `gorm:"size:64;not null" json:"name"`     // 主题名，如 周报
	ReportType string         `gorm:"size:16;index" json:"report_type"` // week / year；空 = 自定义主题
	Content    string         `gorm:"type:text;not null" json:"content"`
	BuiltIn    bool           `gorm:"default:false" json:"built_in"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 显式指定表名：GORM 命名转换会把 "AIPrompt" 中的 "IP" 当作常见缩写词处理，
// 默认会生成 a_iprompts，这里固定为 ai_prompts
func (AIPrompt) TableName() string { return "ai_prompts" }

// OSSConfig 阿里云 OSS 配置（系统设置中管理，仅管理员可见/可写；全表仅一行）。
// 任务富文本中的图片/附件经服务端中继上传到该 Bucket，
// Bucket 需公共读（或配置自定义域名），否则前端无法展示/下载
type OSSConfig struct {
	ID              uint   `gorm:"primaryKey" json:"id"`
	Endpoint        string `gorm:"size:128;not null" json:"endpoint"` // 如 oss-cn-hangzhou.aliyuncs.com（不含协议）
	Bucket          string `gorm:"size:64;not null" json:"bucket"`
	AccessKeyID     string `gorm:"size:128;not null" json:"access_key_id"`
	AccessKeySecret string `gorm:"size:128" json:"access_key_secret"` // 仅管理员接口返回
	Dir             string `gorm:"size:128" json:"dir"`               // 对象 key 前缀目录，如 work-report
	Domain          string `gorm:"size:256" json:"domain"`            // 自定义访问域名（含协议），空则用 https://bucket.endpoint
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}

// AIReport AI 生成的总结报告（周报/年度报告）。
// 生成是后端异步任务：创建后 status=running，前端轮询；刷新页面不影响生成。
type AIReport struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	RequesterID uint           `gorm:"not null;index" json:"requester_id"` // 发起人（谁点的生成）
	Requester   User           `gorm:"foreignKey:RequesterID" json:"requester"`
	UserID      uint           `gorm:"not null;index" json:"user_id"` // 执行人（被分析的同事，报告写入其任务）
	User        User           `gorm:"foreignKey:UserID" json:"user"`
	AIModelID   uint           `gorm:"not null" json:"ai_model_id"`
	AIModel     AIModel        `gorm:"foreignKey:AIModelID" json:"ai_model"`
	ReportType  string         `gorm:"size:16;not null" json:"report_type"` // week / year
	DateFrom    time.Time      `gorm:"type:date;not null" json:"date_from"`
	DateTo      time.Time      `gorm:"type:date;not null" json:"date_to"`
	ExtraPrompt string         `gorm:"type:text" json:"extra_prompt"` // 用户自定义提示词
	Status      string         `gorm:"size:16;not null;default:running;index" json:"status"` // running / done / failed
	Result      string         `gorm:"type:longtext" json:"result"`
	Error       string         `gorm:"size:1024" json:"error"`
	WorkItemID  *uint          `gorm:"index" json:"work_item_id"` // 生成成功后写入执行人任务的条目 ID
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func All() []any {
	return []any{
		&Role{},
		&User{},
		&Project{},
		&WorkItem{},
		&Comment{},
		&Notification{},
		&PlanRemindLog{},
		&WorkItemParticipant{},
		&AIModel{},
		&AIReport{},
		&AIPrompt{},
		&OSSConfig{},
	}
}
