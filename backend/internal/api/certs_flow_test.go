package api

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"haproxy-webui/backend/internal/model"
)

// SSL 证书管理(v0.7)进程内集成测试:fake dataplaneapi 的 storage ssl_certificates。

// mustTestPEM 现场生成一次性自签测试证书(EC P-256,含私钥,CN=test.local)。
// 不入库 .pem 文件:遵守「任何 .pem 私钥禁止入库」约定。
func mustTestPEM(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatalf("generate serial: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "test.local"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	var b strings.Builder
	b.Write(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	b.Write(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))
	return b.String()
}

func TestSSLCertFlow(t *testing.T) {
	r, fake := newTestEnv(t)
	admin := loginToken(t, r, "admin", "admin123")
	id := registerInstance(t, r, admin, fake.url, itDPPass)
	base := fmt.Sprintf("/api/instances/%d/certs", id)

	// 初始为空列表
	w := doJSON(t, r, http.MethodGet, base, admin, nil)
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("initial list: %d %s", w.Code, w.Body.String())
	}

	// 非 PEM 内容被拒(本地校验,不打到节点)
	w = doJSON(t, r, http.MethodPost, base, admin, map[string]string{"name": "bad.pem", "content": "hello"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("non-PEM upload should 400: %d %s", w.Code, w.Body.String())
	}
	// 路径穿越文件名被拒
	w = doJSON(t, r, http.MethodPost, base, admin, map[string]string{"name": "../evil.pem", "content": mustTestPEM(t)})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("path traversal name should 400: %d %s", w.Code, w.Body.String())
	}

	// 正常上传 → 列表含该证书
	w = doJSON(t, r, http.MethodPost, base, admin, map[string]string{"name": "test.pem", "content": mustTestPEM(t)})
	if w.Code != http.StatusCreated {
		t.Fatalf("upload: %d %s", w.Code, w.Body.String())
	}
	if _, commits, _, _ := fake.stats(); commits != 0 {
		t.Fatalf("upload must not open config transactions, commits = %d", commits)
	}
	w = doJSON(t, r, http.MethodGet, base, admin, nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"test.pem"`) {
		t.Fatalf("list after upload: %d %s", w.Code, w.Body.String())
	}

	// viewer 只读:GET 200,上传 403
	if w := doJSON(t, r, http.MethodPost, "/api/users", admin, map[string]string{"username": "v1", "password": "pass1234", "role": "viewer"}); w.Code != http.StatusCreated {
		t.Fatalf("create viewer: %d %s", w.Code, w.Body.String())
	}
	viewer := loginToken(t, r, "v1", "pass1234")
	if w := doJSON(t, r, http.MethodGet, base, viewer, nil); w.Code != http.StatusOK {
		t.Fatalf("viewer list certs: %d", w.Code)
	}
	if w := doJSON(t, r, http.MethodPost, base, viewer, map[string]string{"name": "x.pem", "content": mustTestPEM(t)}); w.Code != http.StatusForbidden {
		t.Fatalf("viewer upload should 403: %d", w.Code)
	}

	// 删除:200 + reloadId(fake 返回 202 + Reload-Id)
	w = doJSON(t, r, http.MethodDelete, base+"/test.pem", admin, nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "reload-cert-del") {
		t.Fatalf("delete: %d %s", w.Code, w.Body.String())
	}
	w = doJSON(t, r, http.MethodGet, base, admin, nil)
	if strings.Contains(w.Body.String(), "test.pem") {
		t.Fatalf("cert still listed after delete: %s", w.Body.String())
	}

	// 审计:cert.upload 与 cert.delete 已记录
	w = doJSON(t, r, http.MethodGet, "/api/audit-logs?action=cert.upload", admin, nil)
	if !strings.Contains(w.Body.String(), "cert.upload") {
		t.Fatalf("audit missing cert.upload: %s", w.Body.String())
	}
	w = doJSON(t, r, http.MethodGet, "/api/audit-logs?action=cert.delete", admin, nil)
	if !strings.Contains(w.Body.String(), "cert.delete") {
		t.Fatalf("audit missing cert.delete: %s", w.Body.String())
	}
}

// 节点未启用 --ssl-certs-dir 时:所有证书接口 502 + 可操作 hint。
func TestSSLCertDisabledHint(t *testing.T) {
	r, fake := newTestEnv(t)
	fake.certsDisabled = true
	admin := loginToken(t, r, "admin", "admin123")
	id := registerInstance(t, r, admin, fake.url, itDPPass)
	base := fmt.Sprintf("/api/instances/%d/certs", id)

	w := doJSON(t, r, http.MethodGet, base, admin, nil)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("disabled list: %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Error string `json:"error"`
		Hint  string `json:"hint"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Hint == "" || !strings.Contains(resp.Hint, "--ssl-certs-dir") {
		t.Fatalf("hint missing: %+v", resp)
	}

	w = doJSON(t, r, http.MethodPost, base, admin, map[string]string{"name": "a.pem", "content": mustTestPEM(t)})
	if w.Code != http.StatusBadGateway || !strings.Contains(w.Body.String(), "--ssl-certs-dir") {
		t.Fatalf("disabled upload: %d %s", w.Code, w.Body.String())
	}
}

// v0.9:证书删除触发的 reload 失败,与配置提交同路——后台监视器轮询发现,
// webhook 渠道收到告警 + 审计记录 reload.failed。
func TestCertDeleteReloadFailureAlerts(t *testing.T) {
	r, fake, db := newTestEnvWithDB(t)

	var hits atomic.Int32
	var payload atomic.Value // string
	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		buf := make([]byte, 2048)
		n, _ := req.Body.Read(buf)
		payload.Store(string(buf[:n]))
		hits.Add(1)
	}))
	defer webhook.Close()
	if err := db.Create(&model.AlertChannel{
		Name: "it-webhook", Type: model.ChannelWecom, WebhookURL: webhook.URL, Enabled: true,
	}).Error; err != nil {
		t.Fatal(err)
	}

	admin := loginToken(t, r, "admin", "admin123")
	id := registerInstance(t, r, admin, fake.url, itDPPass)
	base := fmt.Sprintf("/api/instances/%d/certs", id)

	if w := doJSON(t, r, http.MethodPost, base, admin, map[string]string{"name": "fail.pem", "content": mustTestPEM(t)}); w.Code != http.StatusCreated {
		t.Fatalf("upload: %d %s", w.Code, w.Body.String())
	}

	// 制造删除后的 reload 失败
	fake.mu.Lock()
	fake.nextReload = "failed"
	fake.mu.Unlock()

	w := doJSON(t, r, http.MethodDelete, base+"/fail.pem", admin, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", w.Code, w.Body.String())
	}

	// 等待后台监视器轮询并推送
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) && hits.Load() == 0 {
		time.Sleep(50 * time.Millisecond)
	}
	if hits.Load() != 1 {
		t.Fatalf("webhook hits = %d, want 1 (cert-delete reload alert missing)", hits.Load())
	}
	if body, _ := payload.Load().(string); !strings.Contains(body, "证书删除 fail.pem 后 reload 失败") {
		t.Fatalf("alert payload missing cert-delete detail: %s", body)
	}

	// 审计里应有 reload.failed 且描述指向证书删除
	deadline = time.Now().Add(3 * time.Second)
	var audited bool
	for time.Now().Before(deadline) && !audited {
		w := doJSON(t, r, http.MethodGet, "/api/audit-logs?action=reload.failed", admin, nil)
		audited = strings.Contains(w.Body.String(), "证书删除 fail.pem")
		if !audited {
			time.Sleep(50 * time.Millisecond)
		}
	}
	if !audited {
		t.Fatal("audit log missing cert-delete reload.failed")
	}
}
