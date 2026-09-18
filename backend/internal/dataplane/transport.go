package dataplane

// 本文件收口 dataplaneapi 的全部 HTTP 请求助手:状态码约定与错误包装只在这里维护。
//
// 状态码约定(2026-09-17 实测定论):
//   - 配置读取:200
//   - 资源创建(立即可用):201
//   - 事务内的写/删操作:202(提交事务时才生效)
//   - 删除(立即):204
//   - raw 配置推送:201/202(reload 异步进行)

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

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

func (c *Client) postJSON(ctx context.Context, path string, body any, out any) error {
	var payload []byte
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
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

	// 202 = 配置变更在事务内被接受(提交时生效);201 = 资源已创建
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("%s returned %d: %s", path, resp.StatusCode, string(body))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

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

func (c *Client) deleteJSON(ctx context.Context, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.username, c.password)

	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("connect dataplaneapi: %w", err)
	}
	defer resp.Body.Close()

	// 事务内的删除返回 202(提交时生效),立即删除返回 204
	if resp.StatusCode != http.StatusNoContent &&
		resp.StatusCode != http.StatusAccepted &&
		resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("%s returned %d: %s", path, resp.StatusCode, string(body))
	}
	return nil
}

// postMultipart 以 multipart/form-data 上传 file_upload 字段并解码 JSON 响应
// (storage 证书 / map 文件上传共用)。
func (c *Client) postMultipart(ctx context.Context, path, filename string, content []byte, out any) error {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("file_upload", filename)
	if err != nil {
		return err
	}
	if _, err := fw.Write(content); err != nil {
		return err
	}
	if err := mw.Close(); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, &body)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("connect dataplaneapi: %w", err)
	}
	defer resp.Body.Close()
	return decodeResponse(resp, http.StatusCreated, path, out)
}

// readText GET 并以文本形式返回响应体(如 configuration/raw 的 text/plain)。
func (c *Client) readText(ctx context.Context, path string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
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
		return "", fmt.Errorf("%s returned %d", path, resp.StatusCode)
	}
	return string(body), nil
}

// decodeResponse 统一处理非 2xx 与 JSON 解码。
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
