package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestUserAPIKeyJSONNeverExposesHash(t *testing.T) {
	key := UserAPIKey{
		ID:        1,
		UserID:    2,
		Name:      "automation",
		KeyHash:   "must-never-leak",
		KeyPrefix: "wrk_example",
	}
	data, err := json.Marshal(key)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}
	body := string(data)
	if strings.Contains(body, "must-never-leak") || strings.Contains(body, "key_hash") {
		t.Fatalf("serialized API key exposed KeyHash: %s", body)
	}
	if !strings.Contains(body, `"key_prefix":"wrk_example"`) {
		t.Fatalf("serialized API key omitted display prefix: %s", body)
	}
}

func TestAllIncludesUserAPIKey(t *testing.T) {
	for _, candidate := range All() {
		if _, ok := candidate.(*UserAPIKey); ok {
			return
		}
	}
	t.Fatal("model.All() must include UserAPIKey")
}
