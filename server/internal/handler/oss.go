package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"work-report/server/internal/middleware"
	"work-report/server/internal/model"
)

// OSSHandler OSS 配置管理（仅管理员）与文件上传中继（登录用户）
type OSSHandler struct {
	db *gorm.DB
}

func NewOSSHandler(db *gorm.DB) *OSSHandler {
	return &OSSHandler{db: db}
}

// MaxUploadSize 单个附件大小上限 500M
const MaxUploadSize = 500 << 20

func (h *OSSHandler) loadConfig() (*model.OSSConfig, error) {
	var cfg model.OSSConfig
	err := h.db.First(&cfg).Error
	return &cfg, err
}

func ossAdminOnly(c *gin.Context) bool {
	if !middleware.CurrentUser(c).IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅管理员可管理 OSS 配置"})
		return false
	}
	return true
}

// GetConfig 读取 OSS 配置（仅管理员；未配置时返回空对象）
func (h *OSSHandler) GetConfig(c *gin.Context) {
	if !ossAdminOnly(c) {
		return
	}
	cfg, err := h.loadConfig()
	if err == gorm.ErrRecordNotFound {
		c.JSON(http.StatusOK, gin.H{})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

// SaveConfig 保存 OSS 配置（仅管理员；单行 upsert）。
// AccessKey Secret 留空表示保持不变，避免每次编辑强制重填
func (h *OSSHandler) SaveConfig(c *gin.Context) {
	if !ossAdminOnly(c) {
		return
	}
	var body struct {
		Endpoint        string `json:"endpoint" binding:"required"`
		Bucket          string `json:"bucket" binding:"required"`
		AccessKeyID     string `json:"access_key_id" binding:"required"`
		AccessKeySecret string `json:"access_key_secret"`
		Dir             string `json:"dir"`
		Domain          string `json:"domain"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Endpoint、Bucket、AccessKey ID 必填"})
		return
	}
	// 规范化：endpoint 去协议与末尾斜杠；dir 去首尾斜杠；domain 去末尾斜杠
	endpoint := strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(
		strings.TrimSpace(body.Endpoint), "https://"), "http://"), "/")
	dir := strings.Trim(strings.TrimSpace(body.Dir), "/")
	domain := strings.TrimSuffix(strings.TrimSpace(body.Domain), "/")

	cfg, err := h.loadConfig()
	if err == gorm.ErrRecordNotFound {
		if strings.TrimSpace(body.AccessKeySecret) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "首次配置必须填写 AccessKey Secret"})
			return
		}
		cfg = &model.OSSConfig{
			Endpoint:        endpoint,
			Bucket:          strings.TrimSpace(body.Bucket),
			AccessKeyID:     strings.TrimSpace(body.AccessKeyID),
			AccessKeySecret: strings.TrimSpace(body.AccessKeySecret),
			Dir:             dir,
			Domain:          domain,
		}
		if err := h.db.Create(cfg).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, cfg)
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	updates := map[string]any{
		"endpoint":      endpoint,
		"bucket":        strings.TrimSpace(body.Bucket),
		"access_key_id": strings.TrimSpace(body.AccessKeyID),
		"dir":           dir,
		"domain":        domain,
	}
	if strings.TrimSpace(body.AccessKeySecret) != "" {
		updates["access_key_secret"] = strings.TrimSpace(body.AccessKeySecret)
	}
	if err := h.db.Model(cfg).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.db.First(cfg, cfg.ID)
	c.JSON(http.StatusOK, cfg)
}

// Upload 富文本图片/附件上传：服务端中继到 OSS（AK 不下发浏览器），单文件上限 500M。
// 注意：经 Nginx 反代时需设置 client_max_body_size 500m，否则大文件会被网关拦截
func (h *OSSHandler) Upload(c *gin.Context) {
	cfg, err := h.loadConfig()
	if err == gorm.ErrRecordNotFound {
		c.JSON(http.StatusBadRequest, gin.H{"error": "附件存储未配置，请管理员先在「系统设置 → 附件存储」中配置 OSS"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 请求体硬限制 500M（+1M 表单开销），超限直接拒绝
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxUploadSize+(1<<20))
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "读取上传文件失败：附件大小不能超过 500M"})
		return
	}
	defer file.Close()
	if header.Size > MaxUploadSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "附件大小不能超过 500M"})
		return
	}

	// 对象 key：dir/YYYYMM/随机串.扩展名（随机串避免文件名冲突与中文编码问题）
	ext := strings.ToLower(filepath.Ext(header.Filename))
	key := path.Join(cfg.Dir, time.Now().Format("200601"), randHex()+ext)

	client, err := oss.New("https://"+cfg.Endpoint, cfg.AccessKeyID, cfg.AccessKeySecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "OSS 客户端初始化失败: " + err.Error()})
		return
	}
	bucket, err := client.Bucket(cfg.Bucket)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "OSS Bucket 打开失败: " + err.Error()})
		return
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	opts := []oss.Option{oss.ContentType(contentType)}
	// 非图片附件下载时保留原始文件名（图片内联展示，不设 disposition）
	if !strings.HasPrefix(contentType, "image/") {
		opts = append(opts, oss.ContentDisposition("attachment; filename*=UTF-8''"+url.PathEscape(header.Filename)))
	}
	if err := bucket.PutObject(key, file, opts...); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "上传到 OSS 失败: " + err.Error()})
		return
	}

	fileURL := "https://" + cfg.Bucket + "." + cfg.Endpoint + "/" + key
	if cfg.Domain != "" {
		fileURL = cfg.Domain + "/" + key
	}
	c.JSON(http.StatusOK, gin.H{"url": fileURL, "name": header.Filename, "size": header.Size})
}

func randHex() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
