package api

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestContainerLogsStream 面向真实容器的日志尾部链路(local-e2e 镜像内置
// sshd + syslogd,/var/log/haproxy.log 含就绪标记行):SSE 流应包含该标记。
func TestContainerLogsStream(t *testing.T) {
	base := containerEndpoint(t)
	r, _ := newRouterDB(t)
	token := loginToken(t, r, "admin", "admin123")

	sfx := fmt.Sprintf("%d", time.Now().UnixNano()%1_000_000)
	id := registerInstanceNamed(t, r, token, base, "dataplaneapi", itDPPass, "it-logs-node-"+sfx)
	if w := doJSON(t, r, http.MethodPut, fmt.Sprintf("/api/instances/%d", id), token, map[string]any{
		"name": "it-logs-node-" + sfx, "baseUrl": base, "username": itDPUser,
		"sshHost": "127.0.0.1", "sshPort": 2222, "sshUser": "root", "sshPassword": "devroot",
	}); w.Code != http.StatusOK {
		t.Fatalf("update instance with ssh: %d %s", w.Code, w.Body.String())
	}

	ts := httptest.NewServer(r)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/api/instances/%d/logs/stream", ts.URL, id), nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("stream request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stream status: %d", resp.StatusCode)
	}

	// 逐行读到就绪标记为止
	found := false
	scanner := bufio.NewScanner(resp.Body)
	for ctx.Err() == nil && scanner.Scan() && !found {
		line := scanner.Text()
		if strings.Contains(line, "data: ") && strings.Contains(line, "local-e2e syslog ready") {
			found = true
		}
	}
	if !found {
		t.Fatal("log stream missing ready marker within timeout")
	}
}
