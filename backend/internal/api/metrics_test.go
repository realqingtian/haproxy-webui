package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"haproxy-webui/backend/internal/model"
)

func TestMetricsEndpointDerivation(t *testing.T) {
	cases := []struct {
		name       string
		baseURL    string
		metricsURL string
		want       string
	}{
		{"默认按 8404 推导", "http://10.0.0.1:5555", "", "http://10.0.0.1:8404/metrics"},
		{"显式基地址补 /metrics", "http://10.0.0.1:5555", "http://10.0.0.1:19004", "http://10.0.0.1:19004/metrics"},
		{"显式地址已含 /metrics", "http://10.0.0.1:5555", "http://10.0.0.1:19004/metrics/", "http://10.0.0.1:19004/metrics"},
		{"BaseURL 无 host 返回空", "::::", "", ""},
	}
	for _, tc := range cases {
		inst := &model.Instance{BaseURL: tc.baseURL, MetricsURL: tc.metricsURL}
		if got := metricsEndpoint(inst); got != tc.want {
			t.Errorf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
}

func TestMetricsProbeAPI(t *testing.T) {
	r, fake := newTestEnv(t)
	admin := loginToken(t, r, "admin", "admin123")

	// httptest 伪装 Prometheus 导出端点
	metricsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/metrics" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write([]byte("# HELP haproxy_up\nhaproxy_up 1\n"))
	}))
	defer metricsSrv.Close()

	// 实例 A:显式 metrics 地址指向伪装端点 → ok
	w := doJSON(t, r, http.MethodPost, "/api/instances", admin, map[string]string{
		"name": "with-metrics", "baseUrl": fake.url, "username": itDPUser, "password": itDPPass,
		"metricsUrl": metricsSrv.URL,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create instance A: %d %s", w.Code, w.Body.String())
	}
	w = doJSON(t, r, http.MethodGet, "/api/instances/1/metrics-probe", admin, nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"ok":true`) ||
		!strings.Contains(w.Body.String(), "haproxy") {
		t.Fatalf("probe A: %d %s", w.Code, w.Body.String())
	}

	// 实例 B:未配置 metrics 地址,8404 推导落空 → ok=false 且带指引
	if w = doJSON(t, r, http.MethodPost, "/api/instances", admin, map[string]string{
		"name": "no-metrics", "baseUrl": fake.url, "username": itDPUser, "password": itDPPass,
	}); w.Code != http.StatusCreated {
		t.Fatalf("create instance B: %d %s", w.Code, w.Body.String())
	}
	w = doJSON(t, r, http.MethodGet, "/api/instances/2/metrics-probe", admin, nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"ok":false`) ||
		!strings.Contains(w.Body.String(), "hint") {
		t.Fatalf("probe B: %d %s", w.Code, w.Body.String())
	}

	// 不存在的实例 404
	if w = doJSON(t, r, http.MethodGet, "/api/instances/999/metrics-probe", admin, nil); w.Code != http.StatusNotFound {
		t.Fatalf("probe missing instance: %d", w.Code)
	}
}
