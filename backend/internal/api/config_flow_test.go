package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"haproxy-webui/backend/internal/config"
	"haproxy-webui/backend/internal/cryptoutil"
	"haproxy-webui/backend/internal/database"
)

// 进程内集成测试:真实 BFF 路由 + 临时 SQLite + fake dataplaneapi,
// 覆盖 login → 实例注册(凭据加密落库)→ apply 单事务批量 → 快照/回滚 → RBAC → 审计。

const (
	itJWTSecret = "integration-test-secret"
	itDPUser    = "dpapi"
	itDPPass    = "demosecret"
)

func newRouterDB(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	// 与 main.go 一致的派生方式(JWTSecret:EncryptionKey),测试内保持同一 material
	cryptoutil.Init(itJWTSecret + ":")

	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := database.SeedAdmin(db, "admin", "admin123"); err != nil {
		t.Fatalf("seed admin: %v", err)
	}
	cfg := &config.Config{JWTSecret: itJWTSecret, AdminUsername: "admin", AdminPassword: "admin123"}
	return NewRouter(cfg, db)
}

func newTestEnv(t *testing.T) (*gin.Engine, *fakeDataplane) {
	t.Helper()
	fake := &fakeDataplane{
		user: itDPUser, pw: itDPPass, version: 1,
		raw:     "# baseline\n",
		servers: map[string][]string{},
		staged:  map[string][]stagedOp{},
	}
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	fake.url = srv.URL
	return newRouterDB(t), fake
}

func doJSON(t *testing.T, r *gin.Engine, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func loginToken(t *testing.T, r *gin.Engine, username, password string) string {
	t.Helper()
	w := doJSON(t, r, http.MethodPost, "/api/auth/login", "", map[string]string{"username": username, "password": password})
	if w.Code != http.StatusOK {
		t.Fatalf("login %s: %d %s", username, w.Code, w.Body.String())
	}
	var resp struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || resp.Token == "" {
		t.Fatalf("login response: %v %s", err, w.Body.String())
	}
	return resp.Token
}

func registerInstance(t *testing.T, r *gin.Engine, token, fakeURL, password string) uint {
	return registerInstanceNamed(t, r, token, fakeURL, itDPUser, password, "it-node")
}

func registerInstanceNamed(t *testing.T, r *gin.Engine, token, baseURL, username, password, name string) uint {
	t.Helper()
	w := doJSON(t, r, http.MethodPost, "/api/instances", token, map[string]string{
		"name": name, "baseUrl": baseURL, "username": username, "password": password,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create instance: %d %s", w.Code, w.Body.String())
	}
	var inst struct {
		ID uint `json:"id"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &inst)
	return inst.ID
}

func applyOps(t *testing.T, r *gin.Engine, token string, id uint, ops []map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	return doJSON(t, r, http.MethodPost, fmt.Sprintf("/api/instances/%d/config/apply", id), token, map[string]any{"ops": ops})
}

func TestConfigApplySingleTransaction(t *testing.T) {
	r, fake := newTestEnv(t)
	token := loginToken(t, r, "admin", "admin123")
	id := registerInstance(t, r, token, fake.url, itDPPass)

	// 连通性测试(fake 凭据正确)
	w := doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/instances/%d/test", id), token, nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "v3.4.3") {
		t.Fatalf("test instance: %d %s", w.Code, w.Body.String())
	}

	// 三条不同类型操作一次提交:只应开一个事务、commit 一次、版本 +1、reload 一次
	w = applyOps(t, r, token, id, []map[string]any{
		{"kind": "create_backend", "name": "web"},
		{"kind": "create_server", "backend": "web", "name": "app1", "address": "10.0.0.1", "port": 8080, "check": "enabled"},
		{"kind": "create_frontend", "name": "fe_http", "mode": "http", "defaultBackend": "web"},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("apply: %d %s", w.Code, w.Body.String())
	}
	var applyResp struct {
		OK       bool   `json:"ok"`
		ReloadID string `json:"reloadId"`
		Note     string `json:"note"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &applyResp)
	if !applyResp.OK || applyResp.ReloadID != "reload-1" {
		t.Fatalf("apply resp: %+v", applyResp)
	}
	for _, want := range []string{"创建 backend web", "添加服务器 app1", "创建 frontend fe_http"} {
		if !strings.Contains(applyResp.Note, want) {
			t.Errorf("note %q missing %q", applyResp.Note, want)
		}
	}

	starts, commits, aborts, version := fake.stats()
	if starts != 1 || commits != 1 || aborts != 0 || version != 2 {
		t.Fatalf("fake tx stats: starts=%d commits=%d aborts=%d version=%d", starts, commits, aborts, version)
	}

	// 配置视图能读到新内容
	w = doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/instances/%d/config", id), token, nil)
	for _, want := range []string{"web", "app1", "fe_http"} {
		if !strings.Contains(w.Body.String(), want) {
			t.Errorf("config view missing %q: %s", want, w.Body.String())
		}
	}

	// 自动快照:一条 revision,raw 含新增内容,操作人为 admin
	w = doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/instances/%d/config/revisions", id), token, nil)
	var revisions []map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &revisions)
	if len(revisions) != 1 {
		t.Fatalf("revisions = %d: %s", len(revisions), w.Body.String())
	}
	if revisions[0]["createdBy"] != "admin" {
		t.Errorf("createdBy = %v", revisions[0]["createdBy"])
	}
	w = doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/instances/%d/config/revisions/1/raw", id), token, nil)
	for _, want := range []string{"backend web", "server app1", "frontend fe_http"} {
		if !strings.Contains(w.Body.String(), want) {
			t.Errorf("revision raw missing %q: %s", want, w.Body.String())
		}
	}

	// 审计:config.apply 已记录
	w = doJSON(t, r, http.MethodGet, "/api/audit-logs?action=config.apply", token, nil)
	if !strings.Contains(w.Body.String(), "config.apply") || !strings.Contains(w.Body.String(), applyResp.Note[:20]) {
		t.Errorf("audit missing config.apply: %s", w.Body.String())
	}
}

func TestConfigApplyFailureAbortsTransaction(t *testing.T) {
	r, fake := newTestEnv(t)
	token := loginToken(t, r, "admin", "admin123")
	id := registerInstance(t, r, token, fake.url, itDPPass)

	if w := applyOps(t, r, token, id, []map[string]any{{"kind": "create_backend", "name": "b1"}}); w.Code != http.StatusOK {
		t.Fatalf("first apply: %d %s", w.Code, w.Body.String())
	}

	// 重复创建同名 backend:fake 返回 400 → BFF 应放弃事务、返回 502 + aborted
	w := applyOps(t, r, token, id, []map[string]any{
		{"kind": "create_backend", "name": "b2"},
		{"kind": "create_backend", "name": "b1"}, // 与已提交的重复
	})
	if w.Code != http.StatusBadGateway {
		t.Fatalf("apply should fail: %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"aborted":true`) {
		t.Errorf("resp missing aborted: %s", w.Body.String())
	}

	starts, commits, aborts, version := fake.stats()
	if starts != 2 || commits != 1 || aborts != 1 || version != 2 {
		t.Fatalf("fake tx stats: starts=%d commits=%d aborts=%d version=%d", starts, commits, aborts, version)
	}

	// 失败的批次不产生新快照,配置保持第一次提交后的状态
	w = doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/instances/%d/config/revisions", id), token, nil)
	var revisions []map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &revisions)
	if len(revisions) != 1 {
		t.Fatalf("revisions after failed apply = %d", len(revisions))
	}
}

func TestRollbackRestoresPreviousConfig(t *testing.T) {
	r, fake := newTestEnv(t)
	token := loginToken(t, r, "admin", "admin123")
	id := registerInstance(t, r, token, fake.url, itDPPass)

	if w := applyOps(t, r, token, id, []map[string]any{{"kind": "create_backend", "name": "b1"}}); w.Code != http.StatusOK {
		t.Fatalf("apply 1: %d %s", w.Code, w.Body.String())
	}
	if w := applyOps(t, r, token, id, []map[string]any{{"kind": "create_backend", "name": "b2"}}); w.Code != http.StatusOK {
		t.Fatalf("apply 2: %d %s", w.Code, w.Body.String())
	}

	// 回滚到快照 #1:应整体推送快照 #1 的 raw,并生成一条回滚快照
	w := doJSON(t, r, http.MethodPost, fmt.Sprintf("/api/instances/%d/config/revisions/1/rollback", id), token, nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "回滚到快照 #1") {
		t.Fatalf("rollback: %d %s", w.Code, w.Body.String())
	}
	fake.mu.Lock()
	pushed := fake.lastPush
	fake.mu.Unlock()
	if !strings.Contains(pushed, "backend b1") || strings.Contains(pushed, "backend b2") {
		t.Fatalf("pushed raw = %q", pushed)
	}

	w = doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/instances/%d/config/revisions", id), token, nil)
	var revisions []map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &revisions)
	if len(revisions) != 3 {
		t.Fatalf("revisions after rollback = %d", len(revisions))
	}
	if note, _ := revisions[0]["note"].(string); !strings.Contains(note, "回滚到快照 #1") {
		t.Errorf("latest note = %v", revisions[0]["note"])
	}
}

func TestPreviewOpsAbortsTransaction(t *testing.T) {
	r, fake := newTestEnv(t)
	token := loginToken(t, r, "admin", "admin123")
	id := registerInstance(t, r, token, fake.url, itDPPass)
	if w := applyOps(t, r, token, id, []map[string]any{{"kind": "create_backend", "name": "b1"}}); w.Code != http.StatusOK {
		t.Fatalf("baseline apply: %d %s", w.Code, w.Body.String())
	}

	// 预览两条新增:返回前后 raw,但不 commit、不 reload、不产生快照
	w := doJSON(t, r, http.MethodPost, fmt.Sprintf("/api/instances/%d/config/preview", id), token, map[string]any{
		"ops": []map[string]any{
			{"kind": "create_backend", "name": "b2"},
			{"kind": "create_backend", "name": "b3"},
		},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("preview: %d %s", w.Code, w.Body.String())
	}
	var pv struct {
		Current string `json:"current"`
		Preview string `json:"preview"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &pv)
	if !strings.Contains(pv.Preview, "backend b2") || !strings.Contains(pv.Preview, "backend b3") {
		t.Errorf("preview raw missing staged ops: %q", pv.Preview)
	}
	if strings.Contains(pv.Current, "backend b2") || !strings.Contains(pv.Current, "backend b1") {
		t.Errorf("current raw unexpected: %q", pv.Current)
	}
	_, commits, aborts, version := fake.stats()
	if commits != 1 || aborts != 1 || version != 2 {
		t.Fatalf("preview must not commit: commits=%d aborts=%d version=%d", commits, aborts, version)
	}

	// 预览失败路径:非法操作 → 502 + aborted,事务同样被放弃
	w = doJSON(t, r, http.MethodPost, fmt.Sprintf("/api/instances/%d/config/preview", id), token, map[string]any{
		"ops": []map[string]any{{"kind": "create_backend", "name": "b1"}}, // 与已提交重名
	})
	if w.Code != http.StatusBadGateway || !strings.Contains(w.Body.String(), `"aborted":true`) {
		t.Fatalf("preview failure: %d %s", w.Code, w.Body.String())
	}
	_, commits2, aborts2, version2 := fake.stats()
	if commits2 != 1 || aborts2 != 2 || version2 != 2 {
		t.Fatalf("failed preview must abort only: commits=%d aborts=%d version=%d", commits2, aborts2, version2)
	}
}

func TestViewerCannotApplyConfig(t *testing.T) {
	r, fake := newTestEnv(t)
	admin := loginToken(t, r, "admin", "admin123")

	// admin 建 viewer 账号
	w := doJSON(t, r, http.MethodPost, "/api/users", admin, map[string]string{"username": "viewer1", "password": "pass1234", "role": "viewer"})
	if w.Code != http.StatusCreated {
		t.Fatalf("create viewer: %d %s", w.Code, w.Body.String())
	}
	viewer := loginToken(t, r, "viewer1", "pass1234")

	id := registerInstance(t, r, admin, fake.url, itDPPass)

	// viewer:写操作 403,读操作 200(RBAC 兜底)
	if w := applyOps(t, r, viewer, id, []map[string]any{{"kind": "create_backend", "name": "x"}}); w.Code != http.StatusForbidden {
		t.Fatalf("viewer apply should 403: %d %s", w.Code, w.Body.String())
	}
	if w := doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/instances/%d/config", id), viewer, nil); w.Code != http.StatusOK {
		t.Fatalf("viewer read config: %d", w.Code)
	}
	// viewer 提交被拒,不应产生任何事务
	if _, commits, _, _ := fake.stats(); commits != 0 {
		t.Fatalf("commits = %d, want 0", commits)
	}
}

func TestTruncateNote(t *testing.T) {
	// 短 note 原样保留
	if got := truncateNote("创建 backend web"); got != "创建 backend web" {
		t.Fatalf("short note = %q", got)
	}
	// 超长(多字节中文)在 rune 边界截断且总长受控,尾部为省略号
	long := strings.Repeat("创建backend名称很长的操作", 60) // 13 rune × 60 = 780 rune,远超 500 字节
	got := truncateNote(long)
	if len(got) > 503 {
		t.Fatalf("truncated length = %d", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("truncated tail = %q", got[len(got)-10:])
	}
	if !utf8.ValidString(got) {
		t.Fatalf("truncated note is not valid utf-8")
	}
}

func TestInstanceWrongCredentialsSurfaceAuthFailure(t *testing.T) {
	r, fake := newTestEnv(t)
	token := loginToken(t, r, "admin", "admin123")
	// 凭据加密落库后,用错误密码注册:fake 返回 401,BFF 应透出鉴权失败语义
	id := registerInstance(t, r, token, fake.url, "wrong-password")

	w := doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/instances/%d/test", id), token, nil)
	if w.Code != http.StatusBadGateway || !strings.Contains(w.Body.String(), "dataplaneapi auth failed (401)") {
		t.Fatalf("test with wrong creds: %d %s", w.Code, w.Body.String())
	}
}
