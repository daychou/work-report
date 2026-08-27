package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestAuthorizationToken(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{name: "bearer jwt", header: "Bearer jwt.token", want: "jwt.token"},
		{name: "case insensitive bearer", header: "bearer jwt.token", want: "jwt.token"},
		{name: "bearer api key", header: "Bearer wrk_secret", want: "wrk_secret"},
		{name: "raw api key", header: "wrk_secret", want: "wrk_secret"},
		{name: "raw jwt rejected", header: "jwt.token", want: ""},
		{name: "empty bearer rejected", header: "Bearer", want: ""},
		{name: "extra fields rejected", header: "Bearer one two", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := authorizationToken(tt.header); got != tt.want {
				t.Fatalf("authorizationToken(%q) = %q, want %q", tt.header, got, tt.want)
			}
		})
	}
}

func TestSessionOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		authType   string
		wantStatus int
	}{
		{name: "session accepted", authType: AuthTypeSession, wantStatus: http.StatusNoContent},
		{name: "api key rejected", authType: AuthTypeAPIKey, wantStatus: http.StatusForbidden},
		{name: "missing auth type rejected", authType: "", wantStatus: http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.Use(func(c *gin.Context) {
				if tt.authType != "" {
					c.Set(CtxAuthTypeKey, tt.authType)
				}
				c.Next()
			})
			r.GET("/", SessionOnly(), func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			})

			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

func TestShouldTouchAPIKeyLastUsed(t *testing.T) {
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	recent := now.Add(-time.Minute)
	stale := now.Add(-apiKeyLastUsedUpdateInterval - time.Second)
	atCutoff := now.Add(-apiKeyLastUsedUpdateInterval)

	if !shouldTouchAPIKeyLastUsed(nil, now) {
		t.Fatal("never-used key must be touched")
	}
	if shouldTouchAPIKeyLastUsed(&recent, now) {
		t.Fatal("recently used key must be throttled")
	}
	if !shouldTouchAPIKeyLastUsed(&stale, now) {
		t.Fatal("stale last_used_at must be refreshed")
	}
	if shouldTouchAPIKeyLastUsed(&atCutoff, now) {
		t.Fatal("timestamp exactly at the cutoff must match the SQL throttling condition")
	}
}
