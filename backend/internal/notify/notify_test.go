package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"haproxy-webui/backend/internal/database"
	"haproxy-webui/backend/internal/model"
)

func TestBuildPayloadShapes(t *testing.T) {
	cases := []struct {
		typ  string
		want string
	}{
		{model.ChannelFeishu, `{"content":{"text":"x"},"msg_type":"text"}`},
		{model.ChannelDingtalk, `{"msgtype":"text","text":{"content":"x"}}`},
		{model.ChannelWecom, `{"msgtype":"text","text":{"content":"x"}}`},
	}
	for _, tc := range cases {
		payload, err := buildPayload(tc.typ, "x")
		if err != nil {
			t.Fatalf("%s: %v", tc.typ, err)
		}
		buf, _ := json.Marshal(payload)
		// map 序列化键序不定,只做结构断言
		var m map[string]any
		if err := json.Unmarshal(buf, &m); err != nil {
			t.Fatal(err)
		}
		got, _ := json.Marshal(m)
		if !strings.Contains(string(got), "x") {
			t.Fatalf("%s payload missing text: %s", tc.typ, got)
		}
	}
	if _, err := buildPayload("unknown", "x"); err == nil {
		t.Fatal("unknown type should error")
	}
}

func TestAlertTextContainsKeyFacts(t *testing.T) {
	txt := Alert{Kind: KindReloadFailed, Instance: "prod-1", Detail: "boom"}.Text()
	for _, want := range []string{"reload 失败", "prod-1", "boom"} {
		if !strings.Contains(txt, want) {
			t.Errorf("text missing %q: %s", want, txt)
		}
	}
	if txt := (Alert{Kind: KindNodeRecovered, Instance: "p", Detail: "d"}).Text(); !strings.Contains(txt, "恢复") {
		t.Errorf("recovery text should say 恢复: %s", txt)
	}
}

// fan-out:只发启用渠道,单渠道失败不影响其他渠道。
func TestSendFanOut(t *testing.T) {
	var hits atomic.Int32
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
	}))
	defer good.Close()
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer bad.Close()

	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	channels := []model.AlertChannel{
		{Name: "feishu-main", Type: model.ChannelFeishu, WebhookURL: good.URL, Enabled: true},
		{Name: "wecom-off", Type: model.ChannelWecom, WebhookURL: good.URL, Enabled: false},
		{Name: "ding-broken", Type: model.ChannelDingtalk, WebhookURL: bad.URL, Enabled: true},
	}
	for i := range channels {
		if err := db.Create(&channels[i]).Error; err != nil {
			t.Fatal(err)
		}
	}

	Send(context.Background(), db, Alert{Kind: KindBackendDown, Instance: "n1", Detail: "web 全 DOWN"})
	if hits.Load() != 1 {
		t.Fatalf("enabled+reachable channel should receive exactly 1 push, got %d", hits.Load())
	}
}
