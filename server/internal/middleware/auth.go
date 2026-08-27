package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"

	apikeyutil "work-report/server/internal/apikey"
	"work-report/server/internal/model"
)

const (
	CtxUserKey     = "currentUser"
	CtxAuthTypeKey = "authType"

	AuthTypeSession = "session"
	AuthTypeAPIKey  = "api_key"

	apiKeyLastUsedUpdateInterval = 5 * time.Minute
)

// SessionClaims 平台会话 JWT（Casdoor 登录成功后由后端签发）
type SessionClaims struct {
	UserID uint `json:"uid"`
	// ImpersonatedBy 非 0 表示这是管理员模拟身份的会话，值为管理员用户 ID
	ImpersonatedBy uint `json:"imp_by,omitempty"`
	jwt.RegisteredClaims
}

func SignSession(userID uint, secret string, expireHours int) (string, error) {
	return sign(userID, 0, secret, expireHours)
}

// SignImpersonatedSession 签发管理员模拟目标用户身份的会话 token
func SignImpersonatedSession(targetUserID, adminID uint, secret string, expireHours int) (string, error) {
	return sign(targetUserID, adminID, secret, expireHours)
}

func sign(userID, impBy uint, secret string, expireHours int) (string, error) {
	claims := SessionClaims{
		UserID:         userID,
		ImpersonatedBy: impBy,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expireHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// AuthRequired 校验会话 JWT 或 API key，并统一把当前用户与认证类型注入 gin.Context。
// 支持 Authorization: Bearer <jwt|wrk_key>，也兼容直接传 Authorization: wrk_key。
func AuthRequired(db *gorm.DB, secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := authorizationToken(c.GetHeader("Authorization"))
		if tokenStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}

		if apikeyutil.IsAPIKey(tokenStr) {
			authenticateAPIKey(c, db, tokenStr)
			return
		}
		authenticateSession(c, db, secret, tokenStr)
	}
}

func authorizationToken(header string) string {
	parts := strings.Fields(header)
	if len(parts) == 1 && apikeyutil.IsAPIKey(parts[0]) {
		return parts[0]
	}
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return parts[1]
	}
	return ""
}

func authenticateSession(c *gin.Context, db *gorm.DB, secret, tokenStr string) {
	var claims SessionClaims
	token, err := jwt.ParseWithClaims(tokenStr, &claims, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
		return
	}

	var user model.User
	if err := db.Preload("Role").First(&user, claims.UserID).Error; err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}
	// 首次登录（初始密码）必须先改密：仅会话可访问改密与查询本人信息接口。
	if user.MustChangePassword {
		p := c.Request.URL.Path
		if p != "/api/auth/me" && p != "/api/auth/change-password" {
			abortMustChangePassword(c)
			return
		}
	}
	user.ImpersonatedBy = claims.ImpersonatedBy
	setAuthenticatedUser(c, &user, AuthTypeSession)
}

func authenticateAPIKey(c *gin.Context, db *gorm.DB, plainKey string) {
	var key model.UserAPIKey
	if err := db.Where("key_hash = ?", apikeyutil.Hash(plainKey)).First(&key).Error; err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired API key"})
		return
	}

	now := time.Now()
	if key.ExpiresAt != nil && !key.ExpiresAt.After(now) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired API key"})
		return
	}

	var user model.User
	if err := db.Preload("Role").First(&user, key.UserID).Error; err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "API key user not found"})
		return
	}
	// API key 不能访问改密接口，因此首次改密用户一律拒绝，避免绕过初始化流程。
	if user.MustChangePassword {
		abortMustChangePassword(c)
		return
	}

	touchAPIKeyLastUsed(db, &key, now)
	setAuthenticatedUser(c, &user, AuthTypeAPIKey)
}

func touchAPIKeyLastUsed(db *gorm.DB, key *model.UserAPIKey, now time.Time) {
	cutoff := now.Add(-apiKeyLastUsedUpdateInterval)
	if !shouldTouchAPIKeyLastUsed(key.LastUsedAt, now) {
		return
	}
	// 条件更新同时负责并发节流；审计时间写入失败不应让一次已通过的认证失败。
	db.Model(&model.UserAPIKey{}).
		Where("id = ? AND (last_used_at IS NULL OR last_used_at < ?)", key.ID, cutoff).
		Update("last_used_at", now)
}

func shouldTouchAPIKeyLastUsed(lastUsedAt *time.Time, now time.Time) bool {
	return lastUsedAt == nil || lastUsedAt.Before(now.Add(-apiKeyLastUsedUpdateInterval))
}

func setAuthenticatedUser(c *gin.Context, user *model.User, authType string) {
	c.Set(CtxUserKey, user)
	c.Set(CtxAuthTypeKey, authType)
	c.Next()
}

func abortMustChangePassword(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
		"error": "首次登录请先修改初始密码",
		"code":  "must_change_password",
	})
}

// SessionOnly 仅允许平台会话认证，用于管理凭据及模拟身份等敏感操作。
func SessionOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		if CurrentAuthType(c) != AuthTypeSession {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "该操作仅允许会话认证",
				"code":  "session_required",
			})
			return
		}
		c.Next()
	}
}

// CurrentUser 从上下文取当前用户（需在 AuthRequired 之后调用）
func CurrentUser(c *gin.Context) *model.User {
	v, ok := c.Get(CtxUserKey)
	if !ok {
		return nil
	}
	u, _ := v.(*model.User)
	return u
}

// CurrentAuthType 返回当前请求使用的认证方式。
func CurrentAuthType(c *gin.Context) string {
	v, _ := c.Get(CtxAuthTypeKey)
	authType, _ := v.(string)
	return authType
}
