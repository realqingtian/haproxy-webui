package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// TestContainerRealDataplaneFlow 面向真实 dataplaneapi(deploy/dataplaneapi/local-e2e 容器)的容器级集成:
// 注册 → 连通性 → 事务写入(读回校验)→ 事务内 raw 预览 → 运行时切换 → 回滚。
// 容器不可达时自动跳过(保持 `go test ./...` 无 Docker 也能全绿);`make test-integration` 起 compose 后运行。
func TestContainerRealDataplaneFlow(t *testing.T) {
	base := containerEndpoint(t)
	r := newRouterDB(t)
	token := loginToken(t, r, "admin", "admin123")

	// 随机后缀:容器配置在多次运行间持久,避免重名冲突
	sfx := fmt.Sprintf("%d", time.Now().UnixNano()%1_000_000)
	backend := "it_b_" + sfx
	frontend := "it_f_" + sfx
	name := "it-container-" + sfx

	id := registerInstanceNamed(t, r, token, base, "dataplaneapi", itDPPass, name)

	// 连通性
	w := doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/instances/%d/test", id), token, nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "v3.4") {
		t.Fatalf("test instance: %d %s", w.Code, w.Body.String())
	}

	// 事务写入:backend + server + frontend + bind 一次提交
	port := 47651
	w = applyOps(t, r, token, id, []map[string]any{
		{"kind": "create_backend", "name": backend},
		{"kind": "create_server", "backend": backend, "name": "s1", "address": "127.0.0.1", "port": 8080, "check": "disabled"},
		{"kind": "create_frontend", "name": frontend, "mode": "http", "defaultBackend": backend},
		{"kind": "create_bind", "frontend": frontend, "name": "b0", "address": "*", "port": port},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("apply: %d %s", w.Code, w.Body.String())
	}

	// 读回:配置视图包含新对象
	w = doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/instances/%d/config", id), token, nil)
	for _, want := range []string{backend, frontend, "s1"} {
		if !strings.Contains(w.Body.String(), want) {
			t.Errorf("config readback missing %q", want)
		}
	}

	// 事务内 raw 预览:再加一个 backend,预览可见但当前 raw 不可见
	extra := "it_x_" + sfx
	w = doJSON(t, r, http.MethodPost, fmt.Sprintf("/api/instances/%d/config/preview", id), token, map[string]any{
		"ops": []map[string]any{{"kind": "create_backend", "name": extra}},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("preview: %d %s", w.Code, w.Body.String())
	}
	var pv struct {
		Current string `json:"current"`
		Preview string `json:"preview"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &pv)
	if !strings.Contains(pv.Preview, "backend "+extra) || strings.Contains(pv.Current, "backend "+extra) {
		t.Errorf("preview mismatch: preview-has-extra=%v current-has-extra=%v",
			strings.Contains(pv.Preview, "backend "+extra), strings.Contains(pv.Current, "backend "+extra))
	}

	// 运行时切换:maint 即时生效(不写配置)。用初始配置就存在的 demo_app/s1——
	// 容器经多次 USR2 reload 后会有多代进程残留,dataplaneapi 的 runtime 连接可能打到旧代,
	// 新写入的对象在其 runtime 视图中不可见(仅容器环境坑,真机 systemd reload 无此问题)。
	w = doJSON(t, r, http.MethodPut, fmt.Sprintf("/api/instances/%d/backends/demo_app/servers/s1/state", id),
		token, map[string]string{"state": "maint"})
	if w.Code != http.StatusOK {
		t.Fatalf("runtime state: %d %s", w.Code, w.Body.String())
	}
	w = doJSON(t, r, http.MethodPut, fmt.Sprintf("/api/instances/%d/backends/demo_app/servers/s1/state", id),
		token, map[string]string{"state": "ready"})
	if w.Code != http.StatusOK {
		t.Fatalf("runtime state restore: %d %s", w.Code, w.Body.String())
	}

	// 快照语义:每次提交后记录。再提交一个对象产生新快照,然后回滚到首个快照
	// (= 第一次提交后的状态):第二个对象消失,第一个仍在。
	extra2 := "it_y_" + sfx
	if w := applyOps(t, r, token, id, []map[string]any{{"kind": "create_backend", "name": extra2}}); w.Code != http.StatusOK {
		t.Fatalf("apply extra: %d %s", w.Code, w.Body.String())
	}
	w = doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/instances/%d/config/revisions", id), token, nil)
	var revs []map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &revs)
	if len(revs) < 2 {
		t.Fatalf("revisions = %d, want >= 2", len(revs))
	}
	firstRev := fmt.Sprintf("%v", revs[len(revs)-1]["id"]) // 列表按 id 倒序,最后一个是首个快照
	w = doJSON(t, r, http.MethodPost, fmt.Sprintf("/api/instances/%d/config/revisions/%s/rollback", id, firstRev), token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("rollback: %d %s", w.Code, w.Body.String())
	}
	w = doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/instances/%d/config", id), token, nil)
	if !strings.Contains(w.Body.String(), backend) {
		t.Errorf("after rollback missing %q (first snapshot should keep it)", backend)
	}
	if strings.Contains(w.Body.String(), extra2) {
		t.Errorf("after rollback still contains %q", extra2)
	}

	// 清理:删除本次测试创建的 frontend(引用 default_backend)与 backend,容器恢复初始配置。
	// 只删 backend 会因 frontend 引用缺失被 haproxy 校验拒绝——顺带验证了提交前校验路径。
	if w := applyOps(t, r, token, id, []map[string]any{
		{"kind": "delete_frontend", "name": frontend},
		{"kind": "delete_backend", "name": backend},
	}); w.Code != http.StatusOK {
		t.Fatalf("cleanup apply: %d %s", w.Code, w.Body.String())
	}
	w = doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/instances/%d/config", id), token, nil)
	if strings.Contains(w.Body.String(), backend) {
		t.Errorf("cleanup failed, %q still present", backend)
	}
}

// containerEndpoint 返回测试用 dataplaneapi 地址(可用 HAPROXY_WEBUI_IT_DPAPI 覆盖),不可达时跳过。
func containerEndpoint(t *testing.T) string {
	t.Helper()
	base := os.Getenv("HAPROXY_WEBUI_IT_DPAPI")
	if base == "" {
		base = "http://localhost:5555"
	}
	req, err := http.NewRequest(http.MethodGet, base+"/v3/info", nil)
	if err != nil {
		t.Skipf("bad HAPROXY_WEBUI_IT_DPAPI %q: %v", base, err)
	}
	req.SetBasicAuth("dataplaneapi", itDPPass)
	hc := &http.Client{Timeout: 2 * time.Second}
	resp, err := hc.Do(req)
	if err != nil {
		t.Skipf("dataplaneapi at %s unreachable (run `make test-integration` to start it): %v", base, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Skipf("dataplaneapi at %s returned %d", base, resp.StatusCode)
	}
	return base
}
