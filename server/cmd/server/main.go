package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"

	"work-report/server/internal/config"
	"work-report/server/internal/database"
	"work-report/server/internal/handler"
	"work-report/server/internal/job"
	"work-report/server/internal/middleware"
	"work-report/server/internal/notify"
)

func main() {
	confPath := flag.String("conf", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*confPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := database.Init(cfg.Database.DSN)
	if err != nil {
		log.Fatalf("init database: %v", err)
	}
	if err := database.Seed(db); err != nil {
		log.Fatalf("seed database: %v", err)
	}
	log.Println("database ready")

	feishu := notify.NewFeishuClient(cfg.Feishu.AppID, cfg.Feishu.AppSecret, cfg.Feishu.Enabled)

	// 定时任务：任务按日期粒度管理——每天 18:00 扫描到期/逾期，12:00 提醒当天开始的任务，
	// 00:10 将到达开始日期的待办任务自动转入进行中；
	// 启动后 10 秒补跑一次（18 点前/12 点前分别跳过对应扫描，迁移幂等，去重表保证提醒不重复）。
	// 若后续任务精确到分钟，需改为分钟级扫描频率
	c := cron.New()
	remindJob := job.NewPlanRemindJob(db, feishu)
	if _, err := c.AddFunc("1 18 * * *", remindJob.Run); err != nil {
		log.Fatalf("add cron job: %v", err)
	}
	if _, err := c.AddFunc("1 12 * * *", remindJob.RunStartRemind); err != nil {
		log.Fatalf("add cron job: %v", err)
	}
	if _, err := c.AddFunc("10 0 * * *", remindJob.MigrateTodoToDoing); err != nil {
		log.Fatalf("add cron job: %v", err)
	}
	c.Start()
	defer c.Stop()
	go func() {
		time.Sleep(10 * time.Second)
		remindJob.Run()
		remindJob.RunStartRemind()
		remindJob.MigrateTodoToDoing()
	}()

	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	r.Use(corsMiddleware())

	authH := handler.NewAuthHandler(db, cfg)
	projectH := handler.NewProjectHandler(db)
	workItemH := handler.NewWorkItemHandler(db)
	commentH := handler.NewCommentHandler(db, feishu, cfg.App.URL)
	userH := handler.NewUserHandler(db, cfg)
	roleH := handler.NewRoleHandler(db)
	reportH := handler.NewReportHandler(db)
	notifyH := handler.NewNotificationHandler(db)
	aiModelH := handler.NewAIModelHandler(db)
	aiReportH := handler.NewAIReportHandler(db)
	aiPromptH := handler.NewAIPromptHandler(db)
	ossH := handler.NewOSSHandler(db)
	apiKeyH := handler.NewAPIKeyHandler(db)

	api := r.Group("/api")
	{
		api.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
		api.GET("/auth/config", authH.GetConfig)
		api.POST("/auth/callback", authH.Callback)
		api.POST("/auth/dev-login", authH.DevLogin)
		api.POST("/auth/login", authH.Login)

		authed := api.Group("")
		authed.Use(middleware.AuthRequired(db, cfg.JWT.Secret))
		{
			authed.GET("/auth/me", authH.Me)
			authed.PUT("/auth/me", authH.UpdateMe)
			authed.POST("/auth/change-password", authH.ChangePassword)
			authed.GET("/users", handler.ListUsers(db))
			authed.POST("/users", userH.Create)
			authed.PUT("/users/:id", userH.Update)
			authed.POST("/users/:id/impersonate", middleware.SessionOnly(), userH.Impersonate)

			authed.GET("/api-keys", middleware.SessionOnly(), apiKeyH.List)
			authed.POST("/api-keys", middleware.SessionOnly(), apiKeyH.Create)
			authed.DELETE("/api-keys/:id", middleware.SessionOnly(), apiKeyH.Delete)

			authed.GET("/roles", roleH.List)
			authed.POST("/roles", roleH.Create)
			authed.PUT("/roles/:id", roleH.Update)
			authed.DELETE("/roles/:id", roleH.Delete)

			// AI 模型：启用列表所有登录用户可查（选模型用）；管理操作仅管理员（handler 内校验）
			authed.GET("/ai-models/enabled", aiModelH.EnabledList)
			authed.GET("/ai-models", aiModelH.List)
			authed.POST("/ai-models", aiModelH.Create)
			authed.PUT("/ai-models/:id", aiModelH.Update)
			authed.DELETE("/ai-models/:id", aiModelH.Delete)

			// AI 提示词：列表登录用户可查（AI 分析页按报告类型加载默认提示词）；管理操作仅管理员
			authed.GET("/ai-prompts", aiPromptH.List)
			authed.POST("/ai-prompts", aiPromptH.Create)
			authed.PUT("/ai-prompts/:id", aiPromptH.Update)
			authed.DELETE("/ai-prompts/:id", aiPromptH.Delete)

			// AI 分析报告：创建后后端异步生成，前端轮询状态
			authed.GET("/ai-reports", aiReportH.List)
			authed.POST("/ai-reports", aiReportH.Create)
			// 数据预览：按执行人+时间范围查看将提交给 AI 的工作数据（与生成取数逻辑一致）
			authed.GET("/ai-reports/preview", aiReportH.Preview)
			authed.GET("/ai-reports/:id", aiReportH.GetByID)
			// 删除：发起人或管理员（handler 内校验）
			authed.DELETE("/ai-reports/:id", aiReportH.Delete)

			// OSS：配置仅管理员可读写；上传为登录用户（服务端中继，AK 不下发）
			authed.GET("/oss-config", ossH.GetConfig)
			authed.PUT("/oss-config", ossH.SaveConfig)
			authed.POST("/uploads", ossH.Upload)

			authed.GET("/projects", projectH.List)
			authed.POST("/projects", projectH.Create)
			authed.PUT("/projects/:id", projectH.Update)
			authed.DELETE("/projects/:id", projectH.Delete)

			authed.GET("/work-items", workItemH.List)
			authed.GET("/work-items/:id", workItemH.GetByID)
			authed.POST("/work-items", workItemH.Create)
			authed.PUT("/work-items/:id", workItemH.Update)
			authed.PATCH("/work-items/:id/status", workItemH.UpdateStatus)
			authed.DELETE("/work-items/:id", workItemH.Delete)
			authed.POST("/work-items/:id/restore", workItemH.Restore)

			authed.GET("/work-items/:id/comments", commentH.List)
			authed.POST("/work-items/:id/comments", commentH.Create)
			authed.DELETE("/comments/:id", commentH.Delete)

			authed.GET("/reports", reportH.Get)
			authed.GET("/stats", reportH.Stats)

			authed.GET("/notifications", notifyH.List)
			authed.GET("/notifications/unread-count", notifyH.UnreadCount)
			authed.PATCH("/notifications/:id/read", notifyH.MarkRead)
		}
	}

	// 托管前端构建产物（SPA fallback 到 index.html）；目录不存在时纯 API 模式，
	// 前端可独立部署在 Nginx（反代 /api 到本服务）
	serveSPA(r, cfg.Server.StaticDir)

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("listening on %s (casdoor=%v feishu=%v static=%s)", addr, cfg.Casdoor.Enabled, feishu.Enabled(), cfg.Server.StaticDir)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}

// serveSPA 从磁盘目录托管前端静态资源 + 前端路由 fallback
func serveSPA(r *gin.Engine, staticDir string) {
	info, err := os.Stat(staticDir)
	if err != nil || !info.IsDir() {
		log.Printf("static dir %q not found, api only (frontend can be served by nginx)", staticDir)
		return
	}
	root := filepath.Clean(staticDir)

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		// API 未匹配路由返回 JSON 404，不 fallback 到前端页面
		if strings.HasPrefix(path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		// 防路径穿越
		clean := filepath.Clean(strings.TrimPrefix(path, "/"))
		full := filepath.Join(root, clean)
		if !strings.HasPrefix(full, root+string(os.PathSeparator)) && full != root {
			c.Status(http.StatusForbidden)
			return
		}
		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			c.File(full)
			return
		}
		// SPA 路由 fallback
		c.File(filepath.Join(root, "index.html"))
	})
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
