package dataplane

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// 本文件回归业务方法的请求形状与 v3 字段差异(M2 实测):
//   - runtime server 名字字段为 name(规范示例写 server_name)
//   - runtime weight 为 JSON 数字(stats/native 不带参数即返回全部对象)
//   - 事务提交的 reload-id 经响应头 Reload-Id 下发

func TestInfo(t *testing.T) {
	client, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v3/info" {
			t.Errorf("path = %s", r.URL.Path)
		}
		fmt.Fprint(w, `{"api":{"version":"v3.4.3"}}`)
	}))
	info, err := client.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.API.Version != "v3.4.3" {
		t.Fatalf("version = %q", info.API.Version)
	}
}

func TestVersion(t *testing.T) {
	client, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "7")
	}))
	v, err := client.Version(context.Background())
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if v != 7 {
		t.Fatalf("version = %d", v)
	}
}

func TestStringOrNumber(t *testing.T) {
	cases := []struct {
		in   string
		want StringOrNumber
	}{
		{`128`, "128"},         // v3 实际返回 JSON 数字
		{`"128"`, "128"},       // 兼容字符串
		{`"abc"`, "abc"},       // 非数字字符串
		{`null`, ""},           // 空
		{` 42 `, "42"},         // 前导空白
	}
	for _, tc := range cases {
		var s StringOrNumber
		if err := json.Unmarshal([]byte(tc.in), &s); err != nil {
			t.Errorf("unmarshal %s: %v", tc.in, err)
			continue
		}
		if s != tc.want {
			t.Errorf("unmarshal %s = %q, want %q", tc.in, s, tc.want)
		}
	}
}

// captureDataplane 记录请求序列,按 method+path 模板应答,用于断言请求形状。
type requestRecord struct {
	Method string
	Path   string // 不含 query
	Query  string
	Body   string
}

func TestTransactionLifecycle(t *testing.T) {
	var mu sync.Mutex
	var reqs []requestRecord

	client, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		reqs = append(reqs, requestRecord{r.Method, r.URL.Path, r.URL.RawQuery, string(body)})
		mu.Unlock()

		switch {
		case r.Method == "GET" && r.URL.Path == "/v3/services/haproxy/configuration/version":
			fmt.Fprint(w, "5")
		case r.Method == "POST" && r.URL.Path == "/v3/services/haproxy/transactions":
			if r.URL.Query().Get("version") != "5" {
				t.Errorf("start tx version = %q", r.URL.Query().Get("version"))
			}
			fmt.Fprint(w, `{"id":"tx-1","status":"in_progress","_version":5}`)
		case r.Method == "PUT" && r.URL.Path == "/v3/services/haproxy/transactions/tx-1":
			w.Header().Set("Reload-Id", "reload-9")
			fmt.Fprint(w, `{}`)
		case r.Method == "DELETE" && r.URL.Path == "/v3/services/haproxy/transactions/tx-2":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/configuration/backends") && r.URL.Query().Get("transaction_id") != "":
			w.WriteHeader(http.StatusAccepted) // 事务内写操作返回 202
		default:
			t.Errorf("unexpected request: %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
		}
	}))
	ctx := context.Background()

	// 完整事务:取版本 → 开事务(乐观锁)→ 事务内写 → 提交拿 reload-id
	v, err := client.Version(ctx)
	if err != nil || v != 5 {
		t.Fatalf("Version = %d, %v", v, err)
	}
	tx, err := client.StartTransaction(ctx, v)
	if err != nil {
		t.Fatalf("StartTransaction: %v", err)
	}
	if tx.ID != "tx-1" || tx.Version != 5 {
		t.Fatalf("tx = %+v", tx)
	}
	if err := client.CreateBackend(ctx, tx.ID, "b1"); err != nil {
		t.Fatalf("CreateBackend: %v", err)
	}
	reloadID, err := client.CommitTransaction(ctx, tx.ID)
	if err != nil {
		t.Fatalf("CommitTransaction: %v", err)
	}
	if reloadID != "reload-9" {
		t.Fatalf("reloadID = %q", reloadID)
	}

	// 放弃事务
	if err := client.AbortTransaction(ctx, "tx-2"); err != nil {
		t.Fatalf("AbortTransaction: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(reqs) != 5 {
		t.Fatalf("request count = %d: %+v", len(reqs), reqs)
	}
	if q := reqs[1].Query; q != "version=5" {
		t.Errorf("start tx query = %q", q)
	}
	if q := reqs[2].Query; q != "transaction_id=tx-1" {
		t.Errorf("create backend query = %q", q)
	}
}

func TestConfigOpPayloadShapes(t *testing.T) {
	var mu sync.Mutex
	var bodies = map[string]string{}

	client, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies[r.Method+" "+r.URL.Path] = string(body)
		mu.Unlock()
		switch r.Method {
		case http.MethodPost:
			w.WriteHeader(http.StatusAccepted)
		case http.MethodPut:
			fmt.Fprint(w, `{}`)
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	ctx := context.Background()
	port := 8080

	if err := client.CreateServer(ctx, "t1", "b1", ServerPayload{Name: "s1", Address: "10.0.0.1", Port: &port, Check: "enabled"}); err != nil {
		t.Fatalf("CreateServer: %v", err)
	}
	if err := client.UpdateServer(ctx, "t1", "b1", "s1", ServerPayload{Name: "s1", Address: "10.0.0.2", Check: "disabled"}); err != nil {
		t.Fatalf("UpdateServer: %v", err)
	}
	if err := client.CreateFrontend(ctx, "t1", "f1", "http", "b1"); err != nil {
		t.Fatalf("CreateFrontend: %v", err)
	}
	if err := client.CreateBind(ctx, "t1", "f1", "b0", "*", &port); err != nil {
		t.Fatalf("CreateBind: %v", err)
	}
	if err := client.CreateACL(ctx, "t1", "frontends", "f1", "host_a", "req.hdr(host)", "a.com"); err != nil {
		t.Fatalf("CreateACL: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	want := map[string]string{
		"POST /v3/services/haproxy/configuration/backends/b1/servers":       `{"name":"s1","address":"10.0.0.1","port":8080,"check":"enabled"}`,
		"PUT /v3/services/haproxy/configuration/backends/b1/servers/s1":     `{"name":"s1","address":"10.0.0.2","check":"disabled"}`, // Port 为 nil 时 omitempty 省略
		"POST /v3/services/haproxy/configuration/frontends":                 `{"name":"f1","mode":"http","default_backend":"b1"}`,
		"POST /v3/services/haproxy/configuration/frontends/f1/binds":        `{"name":"b0","address":"*","port":8080}`,
		"POST /v3/services/haproxy/configuration/frontends/f1/acls":         `{"acl_name":"host_a","criterion":"req.hdr(host)","value":"a.com"}`,
	}
	for key, wantBody := range want {
		got := bodies[key]
		var gotJSON, wantJSON map[string]any
		if err := json.Unmarshal([]byte(got), &gotJSON); err != nil {
			t.Errorf("%s body not json: %s", key, got)
			continue
		}
		if err := json.Unmarshal([]byte(wantBody), &wantJSON); err != nil {
			t.Fatalf("want body not json: %v", err)
		}
		if !reflect.DeepEqual(gotJSON, wantJSON) {
			t.Errorf("%s body:\n got  %s\n want %s", key, got, wantBody)
		}
	}
}

func TestRuntimeServersFieldDifferences(t *testing.T) {
	// v3 实测:名字字段是 name;weight 是 JSON 数字(与规范示例的 server_name / 字符串不同)
	client, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/runtime/backends/b1/servers") {
			t.Errorf("path = %s", r.URL.Path)
		}
		fmt.Fprint(w, `[
			{"name":"s1","address":"10.0.0.1","port":80,"admin_state":"ready","operational_state":"UP","weight":128},
			{"name":"s2","address":"10.0.0.2","port":80,"admin_state":"maint","operational_state":"DOWN","weight":"64"}
		]`)
	}))
	list, err := client.RuntimeServers(context.Background(), "b1")
	if err != nil {
		t.Fatalf("RuntimeServers: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("len = %d", len(list))
	}
	if list[0].Name != "s1" || list[0].Weight != "128" {
		t.Errorf("s1 = %+v", list[0])
	}
	if list[1].Weight != "64" {
		t.Errorf("s2 weight = %q", list[1].Weight)
	}
}

func TestNativeStats(t *testing.T) {
	client, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v3/services/haproxy/stats/native" || r.URL.RawQuery != "" {
			t.Errorf("path = %s?%s(不带参数即返回全部对象)", r.URL.Path, r.URL.RawQuery)
		}
		fmt.Fprint(w, `{"stats":[
			{"name":"f1","type":"frontend","backend_name":"-","stats":{"req_rate":5,"stot":100}},
			{"name":"b1/s1","type":"server","backend_name":"b1","stats":{"status":"UP"}}
		]}`)
	}))
	list, err := client.NativeStats(context.Background(), "")
	if err != nil {
		t.Fatalf("NativeStats: %v", err)
	}
	if len(list) != 2 || list[0].Type != "frontend" || list[1].BackendName != "b1" {
		t.Fatalf("list = %+v", list)
	}

	client, _ = newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"stats":[],"error":"stats module not enabled"}`)
	}))
	if _, err := client.NativeStats(context.Background(), ""); err == nil ||
		!strings.Contains(err.Error(), "stats error") {
		t.Fatalf("got %v", err)
	}
}

func TestPushRawConfig(t *testing.T) {
	var mu sync.Mutex
	var gotBody, gotCT, gotQuery string
	client, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		gotBody, gotCT, gotQuery = string(body), r.Header.Get("Content-Type"), r.URL.RawQuery
		mu.Unlock()
		w.WriteHeader(http.StatusCreated)
	}))
	raw := "global\n\tmaxconn 100\n"
	if err := client.PushRawConfig(context.Background(), 3, raw); err != nil {
		t.Fatalf("PushRawConfig: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if gotBody != raw || gotCT != "text/plain" || gotQuery != "version=3" {
		t.Fatalf("body=%q ct=%q query=%q", gotBody, gotCT, gotQuery)
	}

	client, _ = newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	if err := client.PushRawConfig(context.Background(), 3, raw); err == nil ||
		!strings.Contains(err.Error(), "push config returned 409") {
		t.Fatalf("got %v", err)
	}
}

func TestReloadStatus(t *testing.T) {
	client, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/reloads/r-1") {
			t.Errorf("path = %s", r.URL.Path)
		}
		fmt.Fprint(w, `{"id":"r-1","status":"succeeded"}`)
	}))
	r, err := client.ReloadStatus(context.Background(), "r-1")
	if err != nil {
		t.Fatalf("ReloadStatus: %v", err)
	}
	if r.Status != "succeeded" {
		t.Fatalf("status = %q", r.Status)
	}
}
