package handler

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"work-report/server/internal/database"
	"work-report/server/internal/model"
)

// 构造一个假的 Casdoor JWT（ParseUnverified 不验签，仅测试用）
// sub 固定为 ou_test_open_id，模拟飞书登录用户的飞书 Open ID
func fakeCasdoorJWT(t *testing.T, owner, name, displayName string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"name":        name,
		"displayName": displayName,
		"owner":       owner,
		"sub":         "ou_test_open_id",
		"exp":         time.Now().Add(time.Hour).Unix(),
	}
	head, _ := json.Marshal(map[string]string{"alg": "none", "typ": "JWT"})
	body, _ := json.Marshal(claims)
	b64 := base64.RawURLEncoding
	return fmt.Sprintf("%s.%s.%s", b64.EncodeToString(head), b64.EncodeToString(body), b64.EncodeToString([]byte("sig")))
}

// 验证：Casdoor 登录时若存在同名手动创建的用户，自动关联而非新建
func TestCasdoorAutoLinkByName(t *testing.T) {
	db, err := database.Init("root:workreport123@tcp(127.0.0.1:3307)/work_report?charset=utf8mb4&parseTime=True&loc=Local")
	if err != nil {
		t.Skipf("mysql not available: %v", err)
	}
	h := &AuthHandler{db: db}

	name := "关联测试用户"
	// 清理残留
	db.Unscoped().Where("name = ?", name).Delete(&model.User{})
	t.Cleanup(func() {
		db.Unscoped().Where("name = ?", name).Delete(&model.User{})
	})

	// 1. 手动创建（模拟管理员预注册）
	manual := model.User{CasdoorID: ManualUserPrefix + name, Name: name}
	if err := db.Create(&manual).Error; err != nil {
		t.Fatalf("create manual user: %v", err)
	}

	// 2. 模拟 Casdoor 登录（displayName 相同）
	token := fakeCasdoorJWT(t, "myorg", "casdoor_login_name", name)
	user, err := h.upsertUserFromCasdoorToken(token)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// 3. 应关联到同一个用户 ID，casdoor_id 被替换为真实值
	if user.ID != manual.ID {
		t.Fatalf("expected link to existing user id=%d, got id=%d", manual.ID, user.ID)
	}
	if user.CasdoorID != "myorg/casdoor_login_name" {
		t.Fatalf("expected casdoor_id updated, got %s", user.CasdoorID)
	}
	// sub 自动绑定为飞书 Open ID
	if user.FeishuOpenID != "ou_test_open_id" {
		t.Fatalf("expected feishu_open_id auto bound, got %s", user.FeishuOpenID)
	}

	// 4. 再次登录仍命中同一账号
	user2, err := h.upsertUserFromCasdoorToken(token)
	if err != nil {
		t.Fatalf("upsert again: %v", err)
	}
	if user2.ID != manual.ID {
		t.Fatalf("expected same user on relogin, got %d vs %d", user2.ID, manual.ID)
	}

	// 5. 不同名则新建，新建用户同样自动绑定飞书 Open ID
	token3 := fakeCasdoorJWT(t, "myorg", "someone_else", "关联测试用户-不存在")
	user3, err := h.upsertUserFromCasdoorToken(token3)
	if err != nil {
		t.Fatalf("upsert new: %v", err)
	}
	if user3.ID == manual.ID {
		t.Fatalf("expected new user for different name")
	}
	if user3.FeishuOpenID != "ou_test_open_id" {
		t.Fatalf("expected feishu_open_id set on new user, got %s", user3.FeishuOpenID)
	}
	db.Unscoped().Delete(user3)
}
