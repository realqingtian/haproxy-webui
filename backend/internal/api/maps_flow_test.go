package api

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// Runtime maps(v0.10)进程内集成测试:列表合并视图 / 条目增删改 / RBAC / 未生效 hint。

func TestMapsFlow(t *testing.T) {
	r, fake := newTestEnv(t)
	admin := loginToken(t, r, "admin", "admin123")
	id := registerInstance(t, r, admin, fake.url, itDPPass)
	base := fmt.Sprintf("/api/instances/%d/maps", id)

	// 合并列表:hosts.map 同时存在 runtime 与 storage,应只出现一次且 active
	w := doJSON(t, r, http.MethodGet, base, admin, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list maps: %d %s", w.Code, w.Body.String())
	}
	if got := strings.Count(w.Body.String(), `"hosts.map"`); got != 1 {
		t.Fatalf("hosts.map should appear once (merged view): %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"active":true`) {
		t.Fatalf("hosts.map should be active: %s", w.Body.String())
	}

	// 条目列表
	w = doJSON(t, r, http.MethodGet, base+"/hosts.map/entries", admin, nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "app.local") {
		t.Fatalf("entries: %d %s", w.Code, w.Body.String())
	}

	// 新增(key 含空白被本地校验拒绝)
	w = doJSON(t, r, http.MethodPost, base+"/hosts.map/entries", admin, map[string]string{"key": "a b", "value": "v"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid key should 400: %d %s", w.Code, w.Body.String())
	}
	w = doJSON(t, r, http.MethodPost, base+"/hosts.map/entries", admin, map[string]string{"key": "grey.local", "value": "demo_app"})
	if w.Code != http.StatusCreated {
		t.Fatalf("add entry: %d %s", w.Code, w.Body.String())
	}

	// force_sync:文件内容应包含新条目
	if fake.mapsStorage["hosts.map"] == "" || !strings.Contains(fake.mapsStorage["hosts.map"], "grey.local demo_app") {
		t.Fatalf("force_sync should persist entry to file: %q", fake.mapsStorage["hosts.map"])
	}

	// 更新(upsert)
	w = doJSON(t, r, http.MethodPut, base+"/hosts.map/entries/grey.local", admin, map[string]string{"value": "app_pool"})
	if w.Code != http.StatusOK {
		t.Fatalf("set entry: %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(fake.mapsStorage["hosts.map"], "grey.local app_pool") {
		t.Fatalf("updated value should persist: %q", fake.mapsStorage["hosts.map"])
	}

	// viewer:读 200,写 403
	if w := doJSON(t, r, http.MethodPost, "/api/users", admin, map[string]string{"username": "v9", "password": "pass1234", "role": "viewer"}); w.Code != http.StatusCreated {
		t.Fatalf("create viewer: %d", w.Code)
	}
	viewer := loginToken(t, r, "v9", "pass1234")
	if w := doJSON(t, r, http.MethodGet, base, viewer, nil); w.Code != http.StatusOK {
		t.Fatalf("viewer list maps: %d", w.Code)
	}
	if w := doJSON(t, r, http.MethodPost, base+"/hosts.map/entries", viewer, map[string]string{"key": "x", "value": "y"}); w.Code != http.StatusForbidden {
		t.Fatalf("viewer add should 403: %d", w.Code)
	}

	// 删除
	w = doJSON(t, r, http.MethodDelete, base+"/hosts.map/entries/grey.local", admin, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete entry: %d %s", w.Code, w.Body.String())
	}
	if strings.Contains(fake.mapsStorage["hosts.map"], "grey.local") {
		t.Fatalf("deleted key should be gone from file: %q", fake.mapsStorage["hosts.map"])
	}

	// 上传新 map 文件(列表出现,active=false)
	w = doJSON(t, r, http.MethodPost, base, admin, map[string]string{"name": "uploaded.map", "content": "# comment\na.key a.value\n"})
	if w.Code != http.StatusCreated {
		t.Fatalf("upload map: %d %s", w.Code, w.Body.String())
	}
	w = doJSON(t, r, http.MethodGet, base, admin, nil)
	if !strings.Contains(w.Body.String(), `"uploaded.map"`) || !strings.Contains(w.Body.String(), `"active":false`) {
		t.Fatalf("uploaded map should list as inactive: %s", w.Body.String())
	}
	// 未生效 map 的条目操作:400 + hint
	w = doJSON(t, r, http.MethodGet, base+"/uploaded.map/entries", admin, nil)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "未被 haproxy.cfg 引用") {
		t.Fatalf("inactive map entries should 400 with hint: %d %s", w.Code, w.Body.String())
	}

	// 审计
	if w := doJSON(t, r, http.MethodGet, "/api/audit-logs?action=map.entry.add", admin, nil); !strings.Contains(w.Body.String(), "map.entry.add") {
		t.Fatalf("audit missing map.entry.add: %s", w.Body.String())
	}
}
