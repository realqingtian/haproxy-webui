package scheduler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"haproxy-webui/backend/internal/database"
	"haproxy-webui/backend/internal/model"
	"haproxy-webui/backend/internal/settings"
)

// 监控循环:边沿触发 —— 首轮只记录;翻转才告警;恢复再告警。
func TestMonitorEdgeTriggeredAlerts(t *testing.T) {
	var infoUp atomic.Bool
	infoUp.Store(true)
	var serversDown atomic.Bool

	mux := http.NewServeMux()
	mux.HandleFunc("/v3/info", func(w http.ResponseWriter, r *http.Request) {
		if !infoUp.Load() {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Write([]byte(`{"api":{"version":"v3.4.3"}}`))
	})
	mux.HandleFunc("/v3/services/haproxy/stats/native", func(w http.ResponseWriter, r *http.Request) {
		status := "UP"
		if serversDown.Load() {
			status = "DOWN"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"stats": []map[string]any{
			{"name": "web", "type": "backend", "stats": map[string]any{}},
			{"name": "app1", "type": "server", "backend_name": "web", "stats": map[string]any{"status": status}},
		}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	var alerts atomic.Int32
	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		alerts.Add(1)
	}))
	defer webhook.Close()

	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	db.Create(&model.AlertChannel{Name: "ch", Type: model.ChannelWecom, WebhookURL: webhook.URL, Enabled: true})
	inst := model.Instance{Name: "mon-node", BaseURL: srv.URL, Username: "u", Password: "p", Enabled: true}
	db.Create(&inst)

	s := New(db)
	ctx := context.Background()
	cfg := settings.Config{MonitorIntervalSeconds: 60, AlertCooldownMinutes: 1}
	now := time.Now()

	run := func() { // 每次推进一个探测周期,确保 due 判定放行
		now = now.Add(61 * time.Second)
		runMonitorChecks(ctx, s, cfg, []model.Instance{inst}, now)
	}

	run() // 首轮:节点 UP、web 不全 DOWN → 只记录不告警
	if alerts.Load() != 0 {
		t.Fatalf("first round must not alert, got %d", alerts.Load())
	}

	serversDown.Store(true)
	run() // UP → 全 DOWN:告警
	run() // 状态未变:不重复告警
	if alerts.Load() != 1 {
		t.Fatalf("down transition should alert exactly once, got %d", alerts.Load())
	}

	serversDown.Store(false)
	run() // 恢复:告警恢复
	if alerts.Load() != 2 {
		t.Fatalf("recovery should alert once more, got %d", alerts.Load())
	}

	// 节点失联 → 告警;恢复 → 告警恢复
	infoUp.Store(false)
	run()
	infoUp.Store(true)
	run()
	if alerts.Load() != 4 {
		t.Fatalf("node down+recovered should add 2 alerts, got %d", alerts.Load())
	}
}
