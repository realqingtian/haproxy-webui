package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// v0.6 审计日志增强:action/username 之外的 from/to 时间过滤与 CSV 导出。

func TestAuditLogFilters(t *testing.T) {
	r, _ := newTestEnv(t)
	admin := loginToken(t, r, "admin", "admin123") // 产生 login.ok 审计
	createNamedUser(t, r, admin, "aud1", "pass1234", "viewer")
	loginToken(t, r, "aud1", "pass1234")

	// action 过滤
	w := doJSON(t, r, http.MethodGet, "/api/audit-logs?action=user.create", admin, nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "user.create") ||
		strings.Contains(w.Body.String(), "login.ok") {
		t.Fatalf("action filter: %d %s", w.Code, w.Body.String())
	}

	// 时间范围:一小时前至今应包含全部;未来/过去边界之外应为空
	now := time.Now()
	from := fmt.Sprintf("/api/audit-logs?from=%s", url.QueryEscape(now.Add(-time.Hour).Format(time.RFC3339)))
	if w = doJSON(t, r, http.MethodGet, from, admin, nil); w.Code != http.StatusOK {
		t.Fatalf("from filter: %d %s", w.Code, w.Body.String())
	}
	var logs []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &logs); err != nil || len(logs) < 3 {
		t.Fatalf("from=1h-ago should include all: %d rows (%v)", len(logs), err)
	}

	if w = doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/audit-logs?to=%s", url.QueryEscape(now.Add(-time.Hour).Format(time.RFC3339))), admin, nil); w.Code != http.StatusOK {
		t.Fatalf("to filter: %d", w.Code)
	}
	if err := json.Unmarshal(w.Body.Bytes(), &logs); err != nil || len(logs) != 0 {
		t.Fatalf("to=1h-ago should be empty, got %d rows", len(logs))
	}

	// datetime-local(分钟精度)格式也要能解析
	dl := fmt.Sprintf("/api/audit-logs?from=%s", now.Add(-time.Hour).Format("2006-01-02T15:04"))
	if w = doJSON(t, r, http.MethodGet, dl, admin, nil); w.Code != http.StatusOK {
		t.Fatalf("datetime-local from: %d", w.Code)
	}
	// 非法时间 400
	if w = doJSON(t, r, http.MethodGet, "/api/audit-logs?from=not-a-time", admin, nil); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid from should 400: %d", w.Code)
	}
}

func TestAuditLogExportCSV(t *testing.T) {
	r, _ := newTestEnv(t)
	admin := loginToken(t, r, "admin", "admin123")

	w := doJSON(t, r, http.MethodGet, "/api/audit-logs/export", admin, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("export: %d %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/csv") {
		t.Fatalf("content type = %q", ct)
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, "attachment") ||
		!strings.Contains(cd, ".csv") {
		t.Fatalf("content disposition = %q", cd)
	}
	body := w.Body.String()
	if !strings.HasPrefix(body, "\xEF\xBB\xBF") {
		t.Fatal("csv missing UTF-8 BOM")
	}
	if !strings.Contains(body, "时间,用户,操作,对象,详情,来源 IP") || !strings.Contains(body, "login.ok") {
		t.Fatalf("csv content unexpected:\n%s", body)
	}

	// 导出同样支持过滤:限定不存在的用户应为仅表头
	w = doJSON(t, r, http.MethodGet, "/api/audit-logs/export?username=nobody", admin, nil)
	if lines := strings.Count(w.Body.String(), "\n"); lines != 1 { // 仅表头一行
		t.Fatalf("filtered export should be header-only, got %d lines", lines)
	}
}
