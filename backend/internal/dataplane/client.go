package dataplane

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client 是 dataplaneapi 的最小访问封装(M2/M3 会扩展配置与运行时接口)。
// 文档: https://github.com/haproxytech/dataplaneapi
type Client struct {
	baseURL  string
	username string
	password string
	hc       *http.Client
}

func NewClient(baseURL, username, password string) *Client {
	return &Client{
		baseURL:  baseURL,
		username: username,
		password: password,
		hc:       &http.Client{Timeout: 5 * time.Second},
	}
}

// Info 对应 GET /v3/info,用作连通性与版本探测。
// 注意:dataplaneapi 3.x 的 API 前缀是 /v3(2.x 为 /v2),且 info 不再包含 haproxy 版本。
type Info struct {
	API struct {
		Version string `json:"version"`
	} `json:"api"`
}

func (c *Client) Info(ctx context.Context) (*Info, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v3/info", nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.username, c.password)

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connect dataplaneapi: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("dataplaneapi auth failed (401)")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("dataplaneapi returned %d", resp.StatusCode)
	}

	var info Info
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decode info: %w", err)
	}
	return &info, nil
}
