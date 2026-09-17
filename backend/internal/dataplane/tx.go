package dataplane

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// Transaction 对应 dataplaneapi 事务。
type Transaction struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Version int64  `json:"_version"`
}

// Reload 对应 reload 状态。
type Reload struct {
	ID     string `json:"id"`
	Status string `json:"status"` // in_progress / succeeded / failed
}

// ACL 是一条 ACL 规则项(按 acl_name 分组)。
type ACL struct {
	AclName   string `json:"acl_name"`
	Criterion string `json:"criterion"`
	Value     string `json:"value"`
}

// Version 返回当前配置版本号。
func (c *Client) Version(ctx context.Context) (int64, error) {
	var v int64
	if err := c.getJSON(ctx, "/v3/services/haproxy/configuration/version", &v); err != nil {
		return 0, err
	}
	return v, nil
}

// StartTransaction 以指定版本开启事务(版本不匹配会失败,即乐观锁)。
func (c *Client) StartTransaction(ctx context.Context, version int64) (*Transaction, error) {
	var tx Transaction
	if err := c.postJSON(ctx, "/v3/services/haproxy/transactions?version="+strconv.FormatInt(version, 10), nil, &tx); err != nil {
		return nil, err
	}
	return &tx, nil
}

// CommitTransaction 提交事务,返回 reload-id(服务端通过响应头下发)。
func (c *Client) CommitTransaction(ctx context.Context, id string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut,
		c.baseURL+"/v3/services/haproxy/transactions/"+id, nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(c.username, c.password)

	resp, err := c.hc.Do(req)
	if err != nil {
		return "", fmt.Errorf("connect dataplaneapi: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("commit transaction returned %d: %s", resp.StatusCode, string(body))
	}
	return resp.Header.Get("Reload-Id"), nil
}

// AbortTransaction 放弃事务。
func (c *Client) AbortTransaction(ctx context.Context, id string) error {
	return c.deleteJSON(ctx, "/v3/services/haproxy/transactions/"+id)
}

// ReloadStatus 查询 reload 执行状态。
func (c *Client) ReloadStatus(ctx context.Context, id string) (*Reload, error) {
	var r Reload
	if err := c.getJSON(ctx, "/v3/services/haproxy/reloads/"+id, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// PushRawConfig 整体替换配置(带版本校验),用于回滚。reload 异步进行,返回 201/202。
func (c *Client) PushRawConfig(ctx context.Context, version int64, raw string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/v3/services/haproxy/configuration/raw?version="+strconv.FormatInt(version, 10),
		strings.NewReader(raw))
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Content-Type", "text/plain")

	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("connect dataplaneapi: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("push config returned %d", resp.StatusCode)
	}
	return nil
}

// ---- 事务内配置操作 ----

// CreateBackend 在事务内创建 backend。
func (c *Client) CreateBackend(ctx context.Context, txID, name string) error {
	return c.postJSON(ctx,
		"/v3/services/haproxy/configuration/backends?transaction_id="+txID,
		map[string]any{"name": name}, nil)
}

// DeleteBackend 在事务内删除 backend。
func (c *Client) DeleteBackend(ctx context.Context, txID, name string) error {
	return c.deleteJSON(ctx,
		"/v3/services/haproxy/configuration/backends/"+name+"?transaction_id="+txID)
}

// ServerPayload 是事务内创建/更新服务器的字段。
type ServerPayload struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	Port    *int   `json:"port,omitempty"`
	Check   string `json:"check,omitempty"` // enabled / disabled
}

func (c *Client) CreateServer(ctx context.Context, txID, backend string, s ServerPayload) error {
	return c.postJSON(ctx,
		"/v3/services/haproxy/configuration/backends/"+backend+"/servers?transaction_id="+txID,
		s, nil)
}

func (c *Client) UpdateServer(ctx context.Context, txID, backend, name string, s ServerPayload) error {
	return c.putJSON(ctx,
		"/v3/services/haproxy/configuration/backends/"+backend+"/servers/"+name+"?transaction_id="+txID,
		s, nil)
}

func (c *Client) DeleteServer(ctx context.Context, txID, backend, name string) error {
	return c.deleteJSON(ctx,
		"/v3/services/haproxy/configuration/backends/"+backend+"/servers/"+name+"?transaction_id="+txID)
}

// CreateFrontend 在事务内创建 frontend,可指定模式(http/tcp)与默认后端。
// 注:v3 的 GET 不返回 mode,但 POST 接受并会写入配置(实测确认)。
func (c *Client) CreateFrontend(ctx context.Context, txID, name, mode, defaultBackend string) error {
	body := map[string]any{"name": name}
	if mode != "" {
		body["mode"] = mode
	}
	if defaultBackend != "" {
		body["default_backend"] = defaultBackend
	}
	return c.postJSON(ctx,
		"/v3/services/haproxy/configuration/frontends?transaction_id="+txID,
		body, nil)
}

// UpdateFrontendDefaultBackend 更新 frontend 的默认后端。
func (c *Client) UpdateFrontendDefaultBackend(ctx context.Context, txID, name, defaultBackend string) error {
	return c.putJSON(ctx,
		"/v3/services/haproxy/configuration/frontends/"+name+"?transaction_id="+txID,
		map[string]any{"default_backend": defaultBackend}, nil)
}

func (c *Client) DeleteFrontend(ctx context.Context, txID, name string) error {
	return c.deleteJSON(ctx,
		"/v3/services/haproxy/configuration/frontends/"+name+"?transaction_id="+txID)
}

// CreateBind 在事务内为 frontend 添加监听(bind 需要 name)。
func (c *Client) CreateBind(ctx context.Context, txID, frontend, name, address string, port *int) error {
	body := map[string]any{"name": name, "address": address}
	if port != nil {
		body["port"] = *port
	}
	return c.postJSON(ctx,
		"/v3/services/haproxy/configuration/frontends/"+frontend+"/binds?transaction_id="+txID,
		body, nil)
}

func (c *Client) DeleteBind(ctx context.Context, txID, frontend, name string) error {
	return c.deleteJSON(ctx,
		"/v3/services/haproxy/configuration/frontends/"+frontend+"/binds/"+name+"?transaction_id="+txID)
}

// CreateACL 在事务内为 frontend/backend 添加一条 ACL 规则项。
// parentType 为 "frontends" 或 "backends"。
func (c *Client) CreateACL(ctx context.Context, txID, parentType, parent, aclName, criterion, value string) error {
	return c.postJSON(ctx,
		"/v3/services/haproxy/configuration/"+parentType+"/"+parent+"/acls?transaction_id="+txID,
		map[string]any{"acl_name": aclName, "criterion": criterion, "value": value}, nil)
}

// DeleteACL 删除整组同名 ACL。
func (c *Client) DeleteACL(ctx context.Context, txID, parentType, parent, aclName string) error {
	return c.deleteJSON(ctx,
		"/v3/services/haproxy/configuration/"+parentType+"/"+parent+"/acls/"+aclName+"?transaction_id="+txID)
}

// ListACLs 读取某 frontend/backend 的全部 ACL 规则项。
func (c *Client) ListACLs(ctx context.Context, parentType, parent string) ([]ACL, error) {
	var list []ACL
	if err := c.getJSON(ctx, "/v3/services/haproxy/configuration/"+parentType+"/"+parent+"/acls", &list); err != nil {
		return nil, err
	}
	return list, nil
}
