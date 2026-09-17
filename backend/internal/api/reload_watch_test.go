package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"haproxy-webui/backend/internal/model"
)

// 验收链路:人为制造 reload 失败(fake 端点返回 failed)→ 后台监视器轮询发现
// → 审计记录 reload.failed → webhook 渠道收到告警推送。
func TestReloadFailureAlertsWebhook(t *testing.T) {
	r, fake, db := newTestEnvWithDB(t)

	var hits atomic.Int32
	var payload atomic.Value // string
	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		buf := make([]byte, 2048)
		n, _ := req.Body.Read(buf)
		payload.Store(string(buf[:n]))
		hits.Add(1)
	}))
	defer webhook.Close()
	if err := db.Create(&model.AlertChannel{
		Name: "it-webhook", Type: model.ChannelWecom, WebhookURL: webhook.URL, Enabled: true,
	}).Error; err != nil {
		t.Fatal(err)
	}

	admin := loginToken(t, r, "admin", "admin123")
	id := registerInstance(t, r, admin, fake.url, itDPPass)

	// 制造 reload 失败
	fake.mu.Lock()
	fake.nextReload = "failed"
	fake.mu.Unlock()

	if w := applyOps(t, r, admin, id, []map[string]any{{"kind": "create_backend", "name": "will-fail"}}); w.Code != http.StatusOK {
		t.Fatalf("apply: %d %s", w.Code, w.Body.String())
	}

	// 等待后台监视器轮询并推送(轮询间隔 1s,上限 8s)
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) && hits.Load() == 0 {
		time.Sleep(50 * time.Millisecond)
	}
	if hits.Load() != 1 {
		t.Fatalf("webhook hits = %d, want 1 (reload failure alert missing)", hits.Load())
	}
	if body, _ := payload.Load().(string); !strings.Contains(body, "reload 失败") {
		t.Fatalf("alert payload missing kind label: %s", body)
	}
	fake.mu.Lock()
	polls := fake.reloadPolls
	fake.mu.Unlock()
	if polls == 0 {
		t.Fatal("fake reload endpoint was never polled")
	}

	// 审计里应有 reload.failed
	deadline = time.Now().Add(3 * time.Second)
	var audited bool
	for time.Now().Before(deadline) && !audited {
		w := doJSON(t, r, http.MethodGet, "/api/audit-logs?action=reload.failed", admin, nil)
		audited = strings.Contains(w.Body.String(), "reload.failed")
		if !audited {
			time.Sleep(50 * time.Millisecond)
		}
	}
	if !audited {
		t.Fatal("audit log missing reload.failed")
	}
}
