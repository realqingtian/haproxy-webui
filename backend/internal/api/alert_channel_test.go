package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 渠道管理与测试发送接口(admin)。回归:测试发送曾因 map 字面量对 nil err 调
// err.Error() 而 panic(GIN recover 成 500),此处锁住修复。
func TestAlertChannelTestSend(t *testing.T) {
	r, _, _ := newTestEnvWithDB(t)
	admin := loginToken(t, r, "admin", "admin123")

	var got map[string]any
	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_ = json.NewDecoder(req.Body).Decode(&got)
		w.Write([]byte(`{"code":0}`))
	}))
	defer webhook.Close()

	w := doJSON(t, r, http.MethodPost, "/api/alert-channels", admin, map[string]any{
		"name": "accept-ch", "type": "feishu", "webhookUrl": webhook.URL, "enabled": true,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create channel: %d %s", w.Code, w.Body.String())
	}

	// 测试发送:此前版本此处 panic → 500;修复后应 200 且 payload 为飞书格式
	w = doJSON(t, r, http.MethodPost, "/api/alert-channels/1/test", admin, nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"ok":true`) {
		t.Fatalf("test send: %d %s", w.Code, w.Body.String())
	}
	if got["msg_type"] != "text" {
		t.Fatalf("payload should be feishu text message, got %v", got)
	}

	// 渠道类型校验
	w = doJSON(t, r, http.MethodPost, "/api/alert-channels", admin, map[string]any{
		"name": "bad", "type": "slack", "webhookUrl": webhook.URL,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid type should 400: %d", w.Code)
	}

	// viewer 禁止管理渠道
	createNamedUser(t, r, admin, "chv", "pass1234", "viewer")
	viewer := loginToken(t, r, "chv", "pass1234")
	if w = doJSON(t, r, http.MethodGet, "/api/alert-channels", viewer, nil); w.Code != http.StatusForbidden {
		t.Fatalf("viewer list channels should 403: %d", w.Code)
	}
	if w = doJSON(t, r, http.MethodPost, "/api/alert-channels", viewer, map[string]any{
		"name": "v-ch", "type": "feishu", "webhookUrl": webhook.URL,
	}); w.Code != http.StatusForbidden {
		t.Fatalf("viewer create channel should 403: %d", w.Code)
	}
}
