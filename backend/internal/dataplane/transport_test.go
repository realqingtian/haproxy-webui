package dataplane

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 本文件回归 transport.go 的状态码约定与错误包装(2026-09-17 实测定论):
//   - 配置读取 200;资源创建 201;事务内写/删 202;立即删除 204;raw 推送 201/202
//   - 401 统一映射为 dataplaneapi auth failed
//   - 非 2xx 错误格式 "%s returned %d: %s";连接错误包装 connect dataplaneapi

func newTestClient(t *testing.T, handler http.Handler) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, "user", "pass"), srv
}

func TestGetJSONDecodesResult(t *testing.T) {
	client, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"api":{"version":"v3.4.3"}}`)
	}))
	var out struct{ API struct{ Version string `json:"version"` } `json:"api"` }
	if err := client.getJSON(context.Background(), "/v3/info", &out); err != nil {
		t.Fatalf("getJSON: %v", err)
	}
	if out.API.Version != "v3.4.3" {
		t.Fatalf("decoded version = %q", out.API.Version)
	}
}

func TestGetJSONSendsBasicAuth(t *testing.T) {
	client, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != "user" || p != "pass" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		fmt.Fprint(w, `1`)
	}))
	var v int
	if err := client.getJSON(context.Background(), "/v3/x", &v); err != nil {
		t.Fatalf("getJSON with correct basic auth: %v", err)
	}
}

func TestGetJSONErrorFormat(t *testing.T) {
	cases := []struct {
		status int
		want   string
	}{
		{http.StatusUnauthorized, "dataplaneapi auth failed (401)"},
		{http.StatusInternalServerError, "/v3/x returned 500: boom"},
	}
	for _, tc := range cases {
		client, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
			fmt.Fprint(w, "boom")
		}))
		err := client.getJSON(context.Background(), "/v3/x", &struct{}{})
		if err == nil || err.Error() != tc.want {
			t.Errorf("status %d: got %v, want %q", tc.status, err, tc.want)
		}
	}
}

func TestGetJSONDecodeError(t *testing.T) {
	client, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "not-json")
	}))
	err := client.getJSON(context.Background(), "/v3/x", &struct{}{})
	if err == nil || !strings.HasPrefix(err.Error(), "decode /v3/x:") {
		t.Fatalf("want decode error, got %v", err)
	}
}

func TestPostJSONStatusSemantics(t *testing.T) {
	// 201(已创建)/ 202(事务内接受)/ 200 均为成功;其余报错
	for _, status := range []int{http.StatusOK, http.StatusCreated, http.StatusAccepted} {
		client, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			fmt.Fprint(w, `{}`)
		}))
		if err := client.postJSON(context.Background(), "/v3/x", map[string]any{"k": 1}, nil); err != nil {
			t.Errorf("status %d should succeed: %v", status, err)
		}
	}
	client, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "bad op")
	}))
	err := client.postJSON(context.Background(), "/v3/x", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "/v3/x returned 400: bad op") {
		t.Fatalf("got %v", err)
	}
}

func TestPostJSONMarshalsBodyAndDecodesOut(t *testing.T) {
	var gotBody map[string]any
	client, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		fmt.Fprint(w, `{"id":"tx-1"}`)
	}))
	var out struct{ ID string `json:"id"` }
	if err := client.postJSON(context.Background(), "/v3/x", map[string]any{"name": "b1"}, &out); err != nil {
		t.Fatalf("postJSON: %v", err)
	}
	if gotBody["name"] != "b1" {
		t.Errorf("request body = %v", gotBody)
	}
	if out.ID != "tx-1" {
		t.Errorf("decoded id = %q", out.ID)
	}
}

func TestPutJSONRequiresOK(t *testing.T) {
	client, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{}`)
	}))
	if err := client.putJSON(context.Background(), "/v3/x", map[string]any{"a": 1}, nil); err != nil {
		t.Fatalf("200 should succeed: %v", err)
	}

	client, _ = newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		fmt.Fprint(w, "stale version")
	}))
	err := client.putJSON(context.Background(), "/v3/x", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "/v3/x returned 409") {
		t.Fatalf("got %v", err)
	}
}

func TestDeleteJSONStatusSemantics(t *testing.T) {
	// 204(立即删除)/ 202(事务内删除)/ 200 均为成功
	for _, status := range []int{http.StatusOK, http.StatusAccepted, http.StatusNoContent} {
		client, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
		}))
		if err := client.deleteJSON(context.Background(), "/v3/x"); err != nil {
			t.Errorf("status %d should succeed: %v", status, err)
		}
	}
	client, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "no such backend")
	}))
	err := client.deleteJSON(context.Background(), "/v3/x")
	if err == nil || !strings.Contains(err.Error(), "/v3/x returned 404") {
		t.Fatalf("got %v", err)
	}
}

func TestReadTextReturnsBody(t *testing.T) {
	client, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "global\n\tmaxconn 100\n")
	}))
	got, err := client.readText(context.Background(), "/v3/services/haproxy/configuration/raw")
	if err != nil {
		t.Fatalf("readText: %v", err)
	}
	if !strings.Contains(got, "maxconn 100") {
		t.Fatalf("body = %q", got)
	}

	client, _ = newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "boom")
	}))
	if _, err := client.readText(context.Background(), "/v3/x"); err == nil ||
		err.Error() != "/v3/x returned 500" {
		t.Fatalf("got %v", err)
	}
}

func TestConnectErrorWrapped(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close() // 立即关闭,制造连接失败

	client := NewClient(url, "u", "p")
	err := client.getJSON(context.Background(), "/v3/x", &struct{}{})
	if err == nil || !strings.HasPrefix(err.Error(), "connect dataplaneapi:") {
		t.Fatalf("got %v", err)
	}
}
