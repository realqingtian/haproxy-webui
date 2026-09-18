package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"haproxy-webui/backend/internal/systemd/systemdtest"
)

// v0.11 SSE 实时推送测试:stats 快照流与 SSH 日志尾部流。

func TestStatsStreamSSE(t *testing.T) {
	r, fake := newTestEnv(t)
	admin := loginToken(t, r, "admin", "admin123")
	id := registerInstance(t, r, admin, fake.url, itDPPass)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/instances/%d/stats/stream", id), nil).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+admin)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if !strings.Contains(w.Body.String(), "data: [") {
		t.Fatalf("stats stream should push json arrays, got: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "demo_app") {
		t.Fatalf("stats payload missing data: %s", w.Body.String())
	}
}

// 日志尾部:fake SSH 模拟 tail -f 输出三行,断言 SSE 逐行 JSON 推送;
// 未配置 SSH 的实例返回 400 + 设置指引。
func TestLogsStreamViaSSH(t *testing.T) {
	r, fake := newTestEnv(t)
	admin := loginToken(t, r, "admin", "admin123")

	// 未配置 SSH:400 + hint
	idNoSSH := registerInstanceNamed(t, r, admin, fake.url, itDPUser, itDPPass, "no-ssh")
	w := doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/instances/%d/logs/stream", idNoSSH), admin, nil)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "未配置 SSH") {
		t.Fatalf("no-ssh should 400: %d %s", w.Code, w.Body.String())
	}

	tailExec := func(cmd string) (string, int) {
		if strings.HasPrefix(cmd, "tail -n 200 -f '/var/log/haproxy.log'") {
			time.Sleep(200 * time.Millisecond) // 让 handler 完成写出后再收尾
			return "line-1\nline-2\nline-3\n", 0
		}
		return "unknown\n", 127
	}
	sshAddr, _ := systemdtest.NewServer(t, "sshu", "sshp", tailExec)
	host, port, _ := net.SplitHostPort(sshAddr)

	id := registerInstance(t, r, admin, fake.url, itDPPass)
	if w := doJSON(t, r, http.MethodPut, fmt.Sprintf("/api/instances/%d", id), admin, map[string]any{
		"name": "it-node", "baseUrl": fake.url, "username": itDPUser,
		"sshHost": host, "sshPort": mustPort(t, port), "sshUser": "sshu", "sshPassword": "sshp",
	}); w.Code != http.StatusOK {
		t.Fatalf("update instance: %d %s", w.Code, w.Body.String())
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel2()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/instances/%d/logs/stream", id), nil).WithContext(ctx2)
	req.Header.Set("Authorization", "Bearer "+admin)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	for _, line := range []string{`data: "line-1"`, `data: "line-2"`, `data: "line-3"`} {
		if !strings.Contains(w.Body.String(), line) {
			t.Fatalf("stream missing %s, got: %s", line, w.Body.String())
		}
	}
}

// 非法日志路径:本地校验拒绝。
func TestLogsStreamInvalidPath(t *testing.T) {
	r, fake := newTestEnv(t)
	admin := loginToken(t, r, "admin", "admin123")
	sshAddr, _ := systemdtest.NewServer(t, "sshu", "sshp", func(string) (string, int) { return "", 0 })
	host, port, _ := net.SplitHostPort(sshAddr)

	id := registerInstance(t, r, admin, fake.url, itDPPass)
	if w := doJSON(t, r, http.MethodPut, fmt.Sprintf("/api/instances/%d", id), admin, map[string]any{
		"name": "it-node", "baseUrl": fake.url, "username": itDPUser,
		"sshHost": host, "sshPort": mustPort(t, port), "sshUser": "sshu", "sshPassword": "sshp",
	}); w.Code != http.StatusOK {
		t.Fatalf("update instance: %d", w.Code)
	}
	w := doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/instances/%d/logs/stream?path=relative/x", id), admin, nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid path should 400: %d %s", w.Code, w.Body.String())
	}
}
