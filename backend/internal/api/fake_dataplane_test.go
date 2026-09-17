package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// fakeDataplane 模拟 dataplaneapi v3 的最小行为面,支撑 BFF 进程内集成测试:
// 事务开启(版本乐观锁)→ 事务内写操作(202,提交才生效)→ 提交(Reload-Id 头、版本 +1)→
// 放弃(204);raw 读写与配置列表读取;Basic Auth 错误返回 401。
type fakeDataplane struct {
	mu       sync.Mutex
	user, pw string
	url      string // httptest server 地址,由 newTestEnv 注入

	version   int64
	raw       string
	backends  []string
	servers   map[string][]string // backend -> server 名
	frontends []string
	staged    map[string][]stagedOp // txID -> 待生效操作
	seq       int                   // 事务序号
	starts    int
	commits   int
	aborts    int
	lastPush  string             // 最近一次 raw 整体推送(回滚路径)
	nextReload string             // 下一次 reload 查询返回的状态,默认 succeeded
	reloadPolls int              // reloads/:id 被查询次数(验证监视器确实在轮询)
}

type stagedOp struct {
	kind     string // backend / server / frontend / bind / acl / delete
	name     string
	backend  string
	frontend string
	line     string
}

func (f *fakeDataplane) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u, p, ok := r.BasicAuth()
	if !ok || u != f.user || p != f.pw {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	writeJSON := func(status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}

	path := r.URL.Path
	switch {
	case r.Method == http.MethodGet && path == "/v3/info":
		writeJSON(http.StatusOK, map[string]any{"api": map[string]any{"version": "v3.4.3"}})

	case r.Method == http.MethodGet && path == "/v3/services/haproxy/configuration/version":
		fmt.Fprint(w, strconv.FormatInt(f.version, 10))

	case r.Method == http.MethodPost && path == "/v3/services/haproxy/transactions":
		v, err := strconv.ParseInt(r.URL.Query().Get("version"), 10, 64)
		if err != nil || v != f.version {
			writeJSON(http.StatusBadRequest, map[string]any{"error": "version mismatch"})
			return
		}
		f.seq++
		f.starts++
		id := fmt.Sprintf("tx-%d", f.seq)
		f.staged[id] = nil
		writeJSON(http.StatusCreated, map[string]any{"id": id, "status": "in_progress", "_version": f.version})

	case r.Method == http.MethodPut && strings.HasPrefix(path, "/v3/services/haproxy/transactions/"):
		id := strings.TrimPrefix(path, "/v3/services/haproxy/transactions/")
		ops, ok := f.staged[id]
		if !ok {
			writeJSON(http.StatusNotFound, map[string]any{"error": "no such transaction"})
			return
		}
		for _, op := range ops {
			f.raw += op.line
			switch op.kind {
			case "backend":
				f.backends = append(f.backends, op.name)
			case "server":
				f.servers[op.backend] = append(f.servers[op.backend], op.name)
			case "frontend":
				f.frontends = append(f.frontends, op.name)
			}
		}
		delete(f.staged, id)
		f.commits++
		f.version++
		w.Header().Set("Reload-Id", fmt.Sprintf("reload-%d", f.commits))
		writeJSON(http.StatusOK, map[string]any{"id": id, "status": "success"})

	case r.Method == http.MethodDelete && strings.HasPrefix(path, "/v3/services/haproxy/transactions/"):
		id := strings.TrimPrefix(path, "/v3/services/haproxy/transactions/")
		if _, ok := f.staged[id]; !ok {
			writeJSON(http.StatusNotFound, map[string]any{"error": "no such transaction"})
			return
		}
		delete(f.staged, id)
		f.aborts++
		w.WriteHeader(http.StatusNoContent)

	case r.Method == http.MethodGet && path == "/v3/services/haproxy/configuration/raw":
		w.Header().Set("Content-Type", "text/plain")
		if txID := r.URL.Query().Get("transaction_id"); txID != "" {
			// 事务内读取:当前 raw + 该事务已暂存的变更(未提交不落盘)
			var b strings.Builder
			b.WriteString(f.raw)
			for _, op := range f.staged[txID] {
				b.WriteString(op.line)
			}
			fmt.Fprint(w, b.String())
			return
		}
		fmt.Fprint(w, f.raw)

	case r.Method == http.MethodPost && path == "/v3/services/haproxy/configuration/raw":
		v, err := strconv.ParseInt(r.URL.Query().Get("version"), 10, 64)
		if err != nil || v != f.version {
			writeJSON(http.StatusConflict, map[string]any{"error": "version mismatch"})
			return
		}
		body, _ := io.ReadAll(r.Body)
		f.lastPush = string(body)
		f.raw = string(body)
		f.version++
		w.WriteHeader(http.StatusCreated)

	case r.Method == http.MethodGet && path == "/v3/services/haproxy/configuration/backends":
		list := make([]map[string]any, 0, len(f.backends))
		for _, b := range f.backends {
			list = append(list, map[string]any{"name": b})
		}
		writeJSON(http.StatusOK, list)

	case r.Method == http.MethodGet && path == "/v3/services/haproxy/configuration/frontends":
		list := make([]map[string]any, 0, len(f.frontends))
		for _, name := range f.frontends {
			list = append(list, map[string]any{"name": name, "default_backend": ""})
		}
		writeJSON(http.StatusOK, list)

	case r.Method == http.MethodGet && strings.HasPrefix(path, "/v3/services/haproxy/reloads/"):
		f.reloadPolls++
		status := f.nextReload
		if status == "" {
			status = "succeeded"
		}
		writeJSON(http.StatusOK, map[string]any{"id": path[strings.LastIndex(path, "/")+1:], "status": status})

	case r.Method == http.MethodGet && strings.HasSuffix(path, "/binds"):
		writeJSON(http.StatusOK, []map[string]any{})

	case r.Method == http.MethodGet && strings.Contains(path, "/configuration/backends/") && strings.HasSuffix(path, "/servers"):
		backend := path[strings.Index(path, "/backends/")+len("/backends/") : strings.LastIndex(path, "/servers")]
		list := make([]map[string]any, 0)
		for _, s := range f.servers[backend] {
			list = append(list, map[string]any{"name": s, "address": "10.0.0.1", "port": 80, "check": "enabled"})
		}
		writeJSON(http.StatusOK, list)

	case r.Method == http.MethodGet && strings.Contains(path, "/runtime/backends/") && strings.HasSuffix(path, "/servers"):
		// 运行时 server 列表:返回空即可(Config 视图按名合并,无则跳过)
		writeJSON(http.StatusOK, []map[string]any{})

	// ---- 事务内写操作:记录到 staged,提交时才生效(202) ----
	case r.URL.Query().Get("transaction_id") == "":
		writeJSON(http.StatusNotFound, map[string]any{"error": "unknown endpoint " + path})

	case r.Method == http.MethodPost && strings.HasSuffix(path, "/configuration/backends"):
		var body struct{ Name string `json:"name"` }
		_ = json.NewDecoder(r.Body).Decode(&body)
		if f.backendExists(body.Name, r.URL.Query().Get("transaction_id")) {
			writeJSON(http.StatusBadRequest, map[string]any{"error": "backend already exists: " + body.Name})
			return
		}
		f.stage(r, stagedOp{kind: "backend", name: body.Name, line: "backend " + body.Name + "\n"})
		w.WriteHeader(http.StatusAccepted)

	case r.Method == http.MethodPost && strings.Contains(path, "/backends/") && strings.HasSuffix(path, "/servers"):
		var body struct {
			Name    string `json:"name"`
			Address string `json:"address"`
			Port    int    `json:"port"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		backend := path[strings.Index(path, "/backends/")+len("/backends/") : strings.LastIndex(path, "/servers")]
		f.stage(r, stagedOp{kind: "server", name: body.Name, backend: backend,
			line: fmt.Sprintf("\tserver %s %s:%d\n", body.Name, body.Address, body.Port)})
		w.WriteHeader(http.StatusAccepted)

	case r.Method == http.MethodPost && strings.HasSuffix(path, "/configuration/frontends"):
		var body struct {
			Name           string `json:"name"`
			Mode           string `json:"mode"`
			DefaultBackend string `json:"default_backend"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.stage(r, stagedOp{kind: "frontend", name: body.Name, line: "frontend " + body.Name + "\n"})
		w.WriteHeader(http.StatusAccepted)

	case r.Method == http.MethodPost && strings.HasSuffix(path, "/binds"):
		var body struct {
			Name    string `json:"name"`
			Address string `json:"address"`
			Port    int    `json:"port"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.stage(r, stagedOp{kind: "bind", line: fmt.Sprintf("\tbind %s %s:%d\n", body.Name, body.Address, body.Port)})
		w.WriteHeader(http.StatusAccepted)

	case r.Method == http.MethodPost && strings.HasSuffix(path, "/acls"):
		var body struct {
			AclName   string `json:"acl_name"`
			Criterion string `json:"criterion"`
			Value     string `json:"value"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.stage(r, stagedOp{kind: "acl", line: fmt.Sprintf("\tacl %s %s %s\n", body.AclName, body.Criterion, body.Value)})
		w.WriteHeader(http.StatusAccepted)

	case r.Method == http.MethodPut && r.URL.Query().Get("transaction_id") != "":
		f.stage(r, stagedOp{kind: "update", line: "\t# update\n"})
		w.WriteHeader(http.StatusAccepted)

	case r.Method == http.MethodDelete && r.URL.Query().Get("transaction_id") != "":
		f.stage(r, stagedOp{kind: "delete", line: "\t# delete\n"})
		w.WriteHeader(http.StatusAccepted)

	default:
		writeJSON(http.StatusNotFound, map[string]any{"error": "unhandled " + r.Method + " " + path})
	}
}

func (f *fakeDataplane) stage(r *http.Request, op stagedOp) {
	txID := r.URL.Query().Get("transaction_id")
	f.staged[txID] = append(f.staged[txID], op)
}

// backendExists 判断 backend 是否已提交或已在同一事务中暂存(重复创建应失败,驱动回滚路径)。
func (f *fakeDataplane) backendExists(name, txID string) bool {
	for _, b := range f.backends {
		if b == name {
			return true
		}
	}
	for _, op := range f.staged[txID] {
		if op.kind == "backend" && op.name == name {
			return true
		}
	}
	return false
}

func (f *fakeDataplane) stats() (starts, commits, aborts int, version int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.starts, f.commits, f.aborts, f.version
}
