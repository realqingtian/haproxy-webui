package dataplane

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// sslCertPath 是 dataplaneapi 3.x 的证书存储前缀。
// 注意:节点侧必须显式配置 --ssl-certs-dir 才会注册该路由,否则返回
// {"code":404,"message":"path ... was not found"}(2026-09-18 实测,3.4.3)。
const sslCertPath = "/v3/services/haproxy/storage/ssl_certificates"

// SSLCertificate 是 dataplaneapi 存储证书的元数据视图。
// dataplaneapi 的 storage 接口不返回证书内容,只有元数据。
type SSLCertificate struct {
	StorageName string    `json:"storage_name"`
	File        string    `json:"file"`
	Description string    `json:"description"`
	Subject     string    `json:"subject"`
	Issuers     string    `json:"issuers"`
	Serial      string    `json:"serial"`
	NotBefore   time.Time `json:"not_before"`
	NotAfter    time.Time `json:"not_after"`
	Size        int64     `json:"size"`
}

// ListSSLCertificates 列出节点证书目录(--ssl-certs-dir)下的证书。
// 注意:3.4.3 的列表接口只做文件列举,不解析证书(subject/有效期为零值),
// 需要元数据时用 GetSSLCertificate 逐个查询(见 handler 层补全逻辑)。
func (c *Client) ListSSLCertificates(ctx context.Context) ([]SSLCertificate, error) {
	var out []SSLCertificate
	if err := c.getJSON(ctx, sslCertPath, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetSSLCertificate 查询单个证书的元数据(含解析出的 subject / 有效期 / 序列号)。
func (c *Client) GetSSLCertificate(ctx context.Context, name string) (*SSLCertificate, error) {
	var out SSLCertificate
	path := sslCertPath + "/" + url.PathEscape(name)
	if err := c.getJSON(ctx, path, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateSSLCertificate 上传 PEM 文件(文件名 name,如 demo.pem)。
// dataplaneapi 校验证书内容,无效 PEM 返回 500;默认只写文件不触发 reload(201)。
func (c *Client) CreateSSLCertificate(ctx context.Context, name string, pem []byte) (*SSLCertificate, error) {
	var out SSLCertificate
	if err := c.postMultipart(ctx, sslCertPath, name, pem, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteSSLCertificate 删除证书文件。dataplaneapi 默认删除后触发一次 reload
// (202 + Reload-ID 头):引用该证书的配置会让 reload 失败并被 v0.6 告警捕获,
// 运行中的旧进程不受影响——让错误立即暴露比静默删文件更安全。
func (c *Client) DeleteSSLCertificate(ctx context.Context, name string) (reloadID string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete,
		c.baseURL+sslCertPath+"/"+url.PathEscape(name), nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(c.username, c.password)

	resp, err := c.hc.Do(req)
	if err != nil {
		return "", fmt.Errorf("connect dataplaneapi: %w", err)
	}
	defer resp.Body.Close()
	// 204 = 立即删除且未触发 reload;202 = 删除已受理,reload 异步进行
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("delete ssl certificate returned %d: %s", resp.StatusCode, string(body))
	}
	return resp.Header.Get("Reload-Id"), nil
}
