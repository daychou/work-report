package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"work-report/server/internal/config"
	"work-report/server/internal/middleware"
	"work-report/server/internal/model"
)

type AuthHandler struct {
	db  *gorm.DB
	cfg *config.Config
}

func NewAuthHandler(db *gorm.DB, cfg *config.Config) *AuthHandler {
	return &AuthHandler{db: db, cfg: cfg}
}

// GetConfig 返回前端需要的认证配置（是否启用 Casdoor、登录跳转地址）
func (h *AuthHandler) GetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"casdoor_enabled": h.cfg.Casdoor.Enabled,
		"authorize_url":   h.authorizeURL(),
	})
}

func (h *AuthHandler) authorizeURL() string {
	if !h.cfg.Casdoor.Enabled {
		return ""
	}
	q := url.Values{}
	q.Set("client_id", h.cfg.Casdoor.ClientID)
	q.Set("response_type", "code")
	q.Set("redirect_uri", h.cfg.Casdoor.RedirectURI)
	q.Set("scope", "read")
	q.Set("state", h.cfg.Casdoor.Application)
	return strings.TrimSuffix(h.cfg.Casdoor.Endpoint, "/") + "/login/oauth/authorize?" + q.Encode()
}

// Callback 前端把 Casdoor 回调拿到的 code 发到这里，换取平台会话 token
func (h *AuthHandler) Callback(c *gin.Context) {
	var body struct {
		Code  string `json:"code" binding:"required"`
		State string `json:"state"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !h.cfg.Casdoor.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "casdoor is not enabled"})
		return
	}

	casdoorToken, err := h.exchangeCode(body.Code)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "exchange code failed: " + err.Error()})
		return
	}

	user, err := h.upsertUserFromCasdoorToken(casdoorToken)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "parse casdoor token failed: " + err.Error()})
		return
	}

	h.respondSession(c, user)
}

// Login 本地账号密码登录（与 Casdoor 认证并行，Casdoor 仅作统一认证入口）
func (h *AuthHandler) Login(c *gin.Context) {
	var body struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名和密码必填"})
		return
	}
	username := strings.TrimSpace(body.Username)

	var user model.User
	err := h.db.Preload("Role").Where("casdoor_id = ?", model.LocalUserPrefix+username).First(&user).Error
	if err != nil || user.PasswordHash == "" ||
		bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(body.Password)) != nil {
		// 统一报错，不区分用户不存在/密码错误，避免枚举用户名
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}
	h.respondSession(c, &user)
}

// ChangePassword 修改本人密码（本地账号）。首次登录（初始密码）用户改密后才能访问其他接口
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	user := middleware.CurrentUser(c)
	var body struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "原密码和新密码必填"})
		return
	}
	if user.PasswordHash == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "当前账号不支持修改密码（统一认证用户）"})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(body.OldPassword)) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "原密码不正确"})
		return
	}
	if len(body.NewPassword) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "新密码长度至少 6 位"})
		return
	}
	if body.NewPassword == body.OldPassword {
		c.JSON(http.StatusBadRequest, gin.H{"error": "新密码不能与原密码相同"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(body.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	updates := map[string]any{"password_hash": string(hash), "must_change_password": false}
	if err := h.db.Model(user).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	user.MustChangePassword = false
	c.JSON(http.StatusOK, user)
}

// DevLogin 开发模式登录（casdoor.enabled=false 时可用），任意用户名直接登录
func (h *AuthHandler) DevLogin(c *gin.Context) {
	if h.cfg.Casdoor.Enabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "dev login disabled when casdoor enabled"})
		return
	}
	var body struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 开发模式：名字以 "admin:" 开头则创建为管理员，方便本地测试权限
	isAdmin := false
	name := body.Name
	if rest, ok := strings.CutPrefix(name, "admin:"); ok {
		isAdmin = true
		name = rest
	}

	var user model.User
	err := h.db.Where("casdoor_id = ?", "dev:"+name).First(&user).Error
	if err == gorm.ErrRecordNotFound {
		user = model.User{CasdoorID: "dev:" + name, Name: name, IsAdmin: isAdmin}
		user.RoleID = h.defaultRoleID(isAdmin)
		if err := h.db.Create(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	} else if isAdmin && !user.IsAdmin {
		updates := map[string]any{"is_admin": true}
		if rid := h.defaultRoleID(true); rid != nil {
			updates["role_id"] = *rid
		}
		h.db.Model(&user).Updates(updates)
		user.IsAdmin = true
	}
	h.db.Preload("Role").First(&user, user.ID)
	h.respondSession(c, &user)
}

// defaultRoleID 按是否管理员取内置角色 ID（角色不存在时返回 nil，由 seed 保证存在）
func (h *AuthHandler) defaultRoleID(isAdmin bool) *uint {
	name := model.RoleUserName
	if isAdmin {
		name = model.RoleAdminName
	}
	var role model.Role
	if err := h.db.Where("name = ?", name).First(&role).Error; err != nil {
		return nil
	}
	return &role.ID
}

// Me 当前登录用户信息
func (h *AuthHandler) Me(c *gin.Context) {
	c.JSON(http.StatusOK, middleware.CurrentUser(c))
}

// UpdateMe 更新本人资料（目前支持绑定飞书 open_id）
func (h *AuthHandler) UpdateMe(c *gin.Context) {
	user := middleware.CurrentUser(c)
	var body struct {
		FeishuOpenID string `json:"feishu_open_id"`
		Avatar       string `json:"avatar"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updates := map[string]any{}
	if body.FeishuOpenID != "" || c.Request.Method == http.MethodPut {
		updates["feishu_open_id"] = body.FeishuOpenID
	}
	if body.Avatar != "" {
		updates["avatar"] = body.Avatar
	}
	if err := h.db.Model(user).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.db.First(user, user.ID)
	c.JSON(http.StatusOK, user)
}

func (h *AuthHandler) respondSession(c *gin.Context, user *model.User) {
	token, err := middleware.SignSession(user.ID, h.cfg.JWT.Secret, h.cfg.JWT.ExpireHours)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "user": user})
}

// exchangeCode 用授权码向 Casdoor 换 access_token（Casdoor 返回的是 JWT）
func (h *AuthHandler) exchangeCode(code string) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", h.cfg.Casdoor.ClientID)
	form.Set("client_secret", h.cfg.Casdoor.ClientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", h.cfg.Casdoor.RedirectURI)

	endpoint := strings.TrimSuffix(h.cfg.Casdoor.Endpoint, "/") + "/api/login/oauth/access_token"
	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("casdoor token endpoint %d: %s", resp.StatusCode, string(data))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		IDToken     string `json:"id_token"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(data, &tokenResp); err != nil {
		return "", err
	}
	if tokenResp.Error != "" {
		return "", fmt.Errorf("casdoor error: %s", tokenResp.Error)
	}
	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("empty access_token from casdoor")
	}
	return tokenResp.AccessToken, nil
}

// casdoorClaims Casdoor access_token(JWT) 中我们关心的字段
// 注意：权限不走 Casdoor（Casdoor 仅作认证），roles 字段不再读取
type casdoorClaims struct {
	Name        string `json:"name"`        // Casdoor 用户名（唯一）
	DisplayName string `json:"displayName"` // 显示名
	Avatar      string `json:"avatar"`
	Email       string `json:"email"`
	Owner       string `json:"owner"` // 组织名
	// RegisteredClaims.Subject 即 sub 字段：飞书登录的用户 sub 就是飞书 Open ID
	jwt.RegisteredClaims
}

// upsertUserFromCasdoorToken 解析 Casdoor JWT（签名由 Casdoor 侧保证，
// token 通过 TLS 直接从 Casdoor 服务端换取），同步用户到本地库。
// 权限由平台内部角色决定：新用户默认「普通用户」角色，已有用户角色保持不变
func (h *AuthHandler) upsertUserFromCasdoorToken(tokenStr string) (*model.User, error) {
	var claims casdoorClaims
	if _, _, err := jwt.NewParser().ParseUnverified(tokenStr, &claims); err != nil {
		return nil, fmt.Errorf("parse casdoor jwt: %w", err)
	}
	if claims.Name == "" {
		return nil, fmt.Errorf("casdoor jwt missing name")
	}

	casdoorID := claims.Owner + "/" + claims.Name
	displayName := claims.DisplayName
	if displayName == "" {
		displayName = claims.Name
	}
	// sub 即飞书 Open ID（飞书登录方式），自动绑定用于飞书提醒；为空时不覆盖已有绑定
	feishuOpenID := claims.Subject

	var user model.User
	err := h.db.Where("casdoor_id = ?", casdoorID).First(&user).Error
	if err == gorm.ErrRecordNotFound {
		// 未绑定过 Casdoor：按显示名查找手动创建/开发模式的同名账号，自动关联
		if err2 := h.db.Where("name = ? AND casdoor_id NOT LIKE ?", displayName, "%/%").
			First(&user).Error; err2 == nil {
			updates := map[string]any{
				"casdoor_id": casdoorID,
				"avatar":     claims.Avatar,
				"email":      claims.Email,
			}
			if feishuOpenID != "" {
				updates["feishu_open_id"] = feishuOpenID
			}
			// 历史遗留账号若尚未分配角色，归位到普通用户
			if user.RoleID == nil {
				if rid := h.defaultRoleID(false); rid != nil {
					updates["role_id"] = *rid
				}
			}
			h.db.Model(&user).Updates(updates)
			h.db.Preload("Role").First(&user, user.ID)
			return &user, nil
		}
		user = model.User{
			CasdoorID:    casdoorID,
			Name:         displayName,
			Avatar:       claims.Avatar,
			Email:        claims.Email,
			FeishuOpenID: feishuOpenID,
			RoleID:       h.defaultRoleID(false),
		}
		if err := h.db.Create(&user).Error; err != nil {
			return nil, err
		}
		h.db.Preload("Role").First(&user, user.ID)
		return &user, nil
	}
	if err != nil {
		return nil, err
	}

	// 仅同步资料，角色（权限）保持平台内部设置不变
	updates := map[string]any{
		"name":   displayName,
		"avatar": claims.Avatar,
		"email":  claims.Email,
	}
	if feishuOpenID != "" {
		updates["feishu_open_id"] = feishuOpenID
	}
	h.db.Model(&user).Updates(updates)
	h.db.Preload("Role").First(&user, user.ID)
	return &user, nil
}
