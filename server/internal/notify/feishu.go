package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// FeishuClient 飞书自建应用客户端（tenant_access_token 自动缓存与刷新）
type FeishuClient struct {
	appID     string
	appSecret string
	enabled   bool

	mu        sync.Mutex
	token     string
	tokenExpr time.Time

	httpClient *http.Client
}

func NewFeishuClient(appID, appSecret string, enabled bool) *FeishuClient {
	return &FeishuClient{
		appID:      appID,
		appSecret:  appSecret,
		enabled:    enabled,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (f *FeishuClient) Enabled() bool {
	return f.enabled && f.appID != "" && f.appSecret != ""
}

func (f *FeishuClient) tenantAccessToken() (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.token != "" && time.Now().Before(f.tokenExpr) {
		return f.token, nil
	}

	body, _ := json.Marshal(map[string]string{
		"app_id":     f.appID,
		"app_secret": f.appSecret,
	})
	resp, err := f.httpClient.Post(
		"https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal",
		"application/json; charset=utf-8",
		bytes.NewReader(body),
	)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)

	var r struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return "", err
	}
	if r.Code != 0 {
		return "", fmt.Errorf("feishu token error %d: %s", r.Code, r.Msg)
	}
	f.token = r.TenantAccessToken
	// 提前 5 分钟过期
	f.tokenExpr = time.Now().Add(time.Duration(r.Expire-300) * time.Second)
	return f.token, nil
}

// SendText 给指定 open_id 用户发送文本消息
func (f *FeishuClient) SendText(openID, text string) error {
	if !f.Enabled() {
		return fmt.Errorf("feishu not configured")
	}
	token, err := f.tenantAccessToken()
	if err != nil {
		return err
	}

	content, _ := json.Marshal(map[string]string{"text": text})
	body, _ := json.Marshal(map[string]any{
		"receive_id": openID,
		"msg_type":   "text",
		"content":    string(content),
	})

	req, err := http.NewRequest(http.MethodPost,
		"https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=open_id",
		bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)

	var r struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return err
	}
	if r.Code != 0 {
		return fmt.Errorf("feishu send error %d: %s", r.Code, r.Msg)
	}
	return nil
}

var _ = strings.TrimSpace
