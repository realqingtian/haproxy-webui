package api

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestContainerMapsFlow 面向真实 dataplaneapi 的 runtime maps 链路(local-e2e 镜像
// 已内置被 demo 前端引用的 hosts.map):列表 / 条目增删改(force_sync 持久化)/ 上传。
func TestContainerMapsFlow(t *testing.T) {
	base := containerEndpoint(t)
	r, _ := newRouterDB(t)
	token := loginToken(t, r, "admin", "admin123")

	sfx := fmt.Sprintf("%d", time.Now().UnixNano()%1_000_000)
	id := registerInstanceNamed(t, r, token, base, "dataplaneapi", itDPPass, "it-maps-node-"+sfx)
	mapsBase := fmt.Sprintf("/api/instances/%d/maps", id)

	// 列表:镜像内置的 hosts.map 生效中
	w := doJSON(t, r, http.MethodGet, mapsBase, token, nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"hosts.map"`) {
		t.Fatalf("list maps: %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"active":true`) {
		t.Fatalf("hosts.map should be active: %s", w.Body.String())
	}

	// 条目:镜像内置 2 条(app.local / test.local)
	w = doJSON(t, r, http.MethodGet, mapsBase+"/hosts.map/entries", token, nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "app.local") {
		t.Fatalf("entries: %d %s", w.Code, w.Body.String())
	}

	// 增 → 文件持久化(force_sync)→ 改 → 删
	key := "it-maps-" + sfx + ".local"
	w = doJSON(t, r, http.MethodPost, mapsBase+"/hosts.map/entries", token, map[string]string{"key": key, "value": "demo_app"})
	if w.Code != http.StatusCreated {
		t.Fatalf("add entry: %d %s", w.Code, w.Body.String())
	}
	w = doJSON(t, r, http.MethodGet, mapsBase+"/hosts.map/content", token, nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), key+" demo_app") {
		t.Fatalf("force_sync should persist to node file: %d %s", w.Code, w.Body.String())
	}
	w = doJSON(t, r, http.MethodPut, mapsBase+"/hosts.map/entries/"+key, token, map[string]string{"value": "demo_app2"})
	if w.Code != http.StatusOK {
		t.Fatalf("set entry: %d %s", w.Code, w.Body.String())
	}
	w = doJSON(t, r, http.MethodGet, mapsBase+"/hosts.map/content", token, nil)
	if !strings.Contains(w.Body.String(), key+" demo_app2") {
		t.Fatalf("updated value should persist: %s", w.Body.String())
	}
	w = doJSON(t, r, http.MethodDelete, mapsBase+"/hosts.map/entries/"+key, token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete entry: %d %s", w.Code, w.Body.String())
	}
	w = doJSON(t, r, http.MethodGet, mapsBase+"/hosts.map/content", token, nil)
	if strings.Contains(w.Body.String(), key) {
		t.Fatalf("deleted key should be gone from node file")
	}

	// 上传新 map 文件
	w = doJSON(t, r, http.MethodPost, mapsBase, token, map[string]string{
		"name": "it-maps-" + sfx + ".map", "content": "a.key a.value\n",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("upload map: %d %s", w.Code, w.Body.String())
	}
	w = doJSON(t, r, http.MethodGet, mapsBase, token, nil)
	if !strings.Contains(w.Body.String(), "it-maps-"+sfx+".map") {
		t.Fatalf("uploaded map should be listed: %s", w.Body.String())
	}
}
