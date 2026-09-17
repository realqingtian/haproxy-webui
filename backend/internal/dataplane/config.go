package dataplane

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// getJSON 请求 dataplaneapi 并把响应解码到 out。
func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.username, c.password)

	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("connect dataplaneapi: %w", err)
	}
	defer resp.Body.Close()

	return decodeResponse(resp, http.StatusOK, path, out)
}

// putJSON 提交部分字段并解码响应(运行时接口接受部分字段)。
func (c *Client) putJSON(ctx context.Context, path string, body any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("connect dataplaneapi: %w", err)
	}
	defer resp.Body.Close()

	if out == nil {
		return decodeResponse(resp, http.StatusOK, path, &struct{}{})
	}
	return decodeResponse(resp, http.StatusOK, path, out)
}

// decodeResponse 统一处理非 2xx 与解码错误。
func decodeResponse(resp *http.Response, wantStatus int, path string, out any) error {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != wantStatus {
		msg := string(body)
		if len(msg) > 300 {
			msg = msg[:300]
		}
		if resp.StatusCode == http.StatusUnauthorized {
			return fmt.Errorf("dataplaneapi auth failed (401)")
		}
		return fmt.Errorf("%s returned %d: %s", path, resp.StatusCode, msg)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

// ---- 配置读取 ----

type Bind struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	Port    *int   `json:"port"`
}

type Frontend struct {
	Name           string `json:"name"`
	DefaultBackend string `json:"default_backend"`
	Binds          []Bind `json:"-"`
}

type Backend struct {
	Name string `json:"name"`
}

type ConfigServer struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	Port    *int   `json:"port"`
	Check   string `json:"check"` // "enabled" / "disabled"
}

func (c *Client) Frontends(ctx context.Context) ([]Frontend, error) {
	var list []Frontend
	if err := c.getJSON(ctx, "/v3/services/haproxy/configuration/frontends", &list); err != nil {
		return nil, err
	}
	return list, nil
}

func (c *Client) Binds(ctx context.Context, frontend string) ([]Bind, error) {
	var list []Bind
	if err := c.getJSON(ctx, "/v3/services/haproxy/configuration/frontends/"+frontend+"/binds", &list); err != nil {
		return nil, err
	}
	return list, nil
}

func (c *Client) Backends(ctx context.Context) ([]Backend, error) {
	var list []Backend
	if err := c.getJSON(ctx, "/v3/services/haproxy/configuration/backends", &list); err != nil {
		return nil, err
	}
	return list, nil
}

func (c *Client) ConfigServers(ctx context.Context, backend string) ([]ConfigServer, error) {
	var list []ConfigServer
	if err := c.getJSON(ctx, "/v3/services/haproxy/configuration/backends/"+backend+"/servers", &list); err != nil {
		return nil, err
	}
	return list, nil
}

// RawConfig 返回 haproxy.cfg 原文(configuration/raw 为 text/plain 响应)。
func (c *Client) RawConfig(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v3/services/haproxy/configuration/raw", nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(c.username, c.password)

	resp, err := c.hc.Do(req)
	if err != nil {
		return "", fmt.Errorf("connect dataplaneapi: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("configuration/raw returned %d", resp.StatusCode)
	}
	return string(body), nil
}

// ---- 运行时 ----

// StringOrNumber 兼容 dataplaneapi 返回中字符串/数字不一致的字段(如 weight)。
type StringOrNumber string

func (s *StringOrNumber) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		*s = ""
		return nil
	}
	if b[0] == '"' {
		var v string
		if err := json.Unmarshal(b, &v); err != nil {
			return err
		}
		*s = StringOrNumber(v)
		return nil
	}
	*s = StringOrNumber(b)
	return nil
}

// RuntimeServer 是运行时服务器状态;Weight 为字符串(空串表示未知/未设置)。
// 注:实际 API 返回的名字字段是 name(与规范示例的 server_name 不一致)。
type RuntimeServer struct {
	Name             string         `json:"name"`
	Address          string         `json:"address"`
	Port             *int           `json:"port"`
	AdminState       string         `json:"admin_state"` // ready / maint / drain
	OperationalState string         `json:"operational_state"`
	Weight           StringOrNumber `json:"weight"`
}

func (c *Client) RuntimeServers(ctx context.Context, backend string) ([]RuntimeServer, error) {
	var list []RuntimeServer
	if err := c.getJSON(ctx, "/v3/services/haproxy/runtime/backends/"+backend+"/servers", &list); err != nil {
		return nil, err
	}
	return list, nil
}

// SetRuntimeServer 以部分字段更新运行时服务器(admin_state / weight / address / port)。
func (c *Client) SetRuntimeServer(ctx context.Context, backend, name string, patch map[string]any) (*RuntimeServer, error) {
	var out RuntimeServer
	path := "/v3/services/haproxy/runtime/backends/" + backend + "/servers/" + name
	if err := c.putJSON(ctx, path, patch, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ---- stats ----

// StatObject 对应 native_stat:一个 frontend/backend/server 对象的指标集合。
// 指标字段随对象类型差异较大,保留原始 map 由前端按需渲染。
type StatObject struct {
	Name        string         `json:"name"`
	Type        string         `json:"type"` // frontend / backend / server
	BackendName string         `json:"backend_name"`
	Stats       map[string]any `json:"stats"`
}

// NativeStats 拉取运行指标;typ 为空时返回全部对象(frontend/backend/server 一次拿全)。
func (c *Client) NativeStats(ctx context.Context, typ string) ([]StatObject, error) {
	path := "/v3/services/haproxy/stats/native"
	if typ != "" {
		path += "?type=" + typ
	}
	var resp struct {
		Stats []StatObject `json:"stats"`
		Error string       `json:"error"`
	}
	if err := c.getJSON(ctx, path, &resp); err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("stats error: %s", resp.Error)
	}
	return resp.Stats, nil
}
