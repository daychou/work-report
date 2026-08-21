package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"

	"work-report/server/internal/model"
)

const CtxUserKey = "currentUser"

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

// AuthRequired 校验 Authorization: Bearer <session-jwt>，把当前用户注入 gin.Context
func AuthRequired(db *gorm.DB, secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		tokenStr := strings.TrimPrefix(header, "Bearer ")
		if tokenStr == "" || tokenStr == header {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}

		var claims SessionClaims
		token, err := jwt.ParseWithClaims(tokenStr, &claims, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
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
		// 首次登录（初始密码）必须先改密：仅放行改密与查询本人信息接口
		if user.MustChangePassword {
			p := c.Request.URL.Path
			if p != "/api/auth/me" && p != "/api/auth/change-password" {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "首次登录请先修改初始密码", "code": "must_change_password"})
				return
			}
		}
		user.ImpersonatedBy = claims.ImpersonatedBy
		c.Set(CtxUserKey, &user)
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
