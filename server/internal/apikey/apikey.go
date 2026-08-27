package apikey

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
)

const (
	Prefix              = "wrk_"
	randomBytes         = 32
	visiblePrefixLength = 12
)

// Generate 创建一个高熵 API key。完整明文只能在创建时返回给调用方。
func Generate() (string, error) {
	random := make([]byte, randomBytes)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return Prefix + base64.RawURLEncoding.EncodeToString(random), nil
}

// Hash 返回用于持久化和查询的 SHA-256 十六进制摘要。
func Hash(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// VisiblePrefix 返回用于列表展示和人工辨识的非敏感 key 前缀。
func VisiblePrefix(key string) string {
	return key[:min(len(key), visiblePrefixLength)]
}

func IsAPIKey(token string) bool {
	return strings.HasPrefix(token, Prefix)
}
