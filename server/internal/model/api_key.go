package model

import (
	"time"

	"gorm.io/gorm"
)

// UserAPIKey 用户创建的长期访问凭据。KeyHash 只保存完整 key 的 SHA-256 摘要，
// 任何 API 响应都不得返回该字段。
type UserAPIKey struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	UserID     uint           `gorm:"not null;index" json:"user_id"`
	Name       string         `gorm:"size:128;not null" json:"name"`
	KeyHash    string         `gorm:"size:64;not null;uniqueIndex" json:"-"`
	KeyPrefix  string         `gorm:"size:16;not null" json:"key_prefix"`
	ExpiresAt  *time.Time     `gorm:"index" json:"expires_at"`
	LastUsedAt *time.Time     `json:"last_used_at"`
	CreatedAt  time.Time      `json:"created_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}
