package dataplane

import (
	"context"
	"net/http"
	"time"
)

// Client 是 dataplaneapi 的最小访问封装。
// HTTP 请求助手统一在 transport.go;各类业务方法在 client.go / config.go / tx.go。
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

// Info 是 GET /v3/info 的响应结构,用作连通性与版本探测。
// 注意:dataplaneapi 3.x 的 API 前缀是 /v3(2.x 为 /v2),且 info 不再包含 haproxy 版本。
type Info struct {
	API struct {
		Version string `json:"version"`
	} `json:"api"`
}

func (c *Client) Info(ctx context.Context) (*Info, error) {
	var info Info
	if err := c.getJSON(ctx, "/v3/info", &info); err != nil {
		return nil, err
	}
	return &info, nil
}
