package api

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestContainerSSLCertificates 面向真实 dataplaneapi 的证书存储链路:
// 上传 → 列表 → 删除。节点未启用 --ssl-certs-dir(旧镜像)时跳过。
func TestContainerSSLCertificates(t *testing.T) {
	base := containerEndpoint(t)
	r, _ := newRouterDB(t)
	token := loginToken(t, r, "admin", "admin123")

	sfx := fmt.Sprintf("%d", time.Now().UnixNano()%1_000_000)
	certName := "it-cert-" + sfx + ".pem"
	id := registerInstanceNamed(t, r, token, base, "dataplaneapi", itDPPass, "it-cert-node-"+sfx)
	certBase := fmt.Sprintf("/api/instances/%d/certs", id)

	// 节点未启用 --ssl-certs-dir(旧镜像)时,列表 502 且 hint 含修复提示 → 跳过本用例
	if w := doJSON(t, r, http.MethodGet, certBase, token, nil); strings.Contains(w.Body.String(), "--ssl-certs-dir") {
		t.Skipf("dataplaneapi cert storage disabled: %s", w.Body.String())
	}

	// 无效内容被 BFF 本地校验拒绝(不打到节点)
	if w := doJSON(t, r, http.MethodPost, certBase, token, map[string]string{"name": certName, "content": "not a pem"}); w.Code != http.StatusBadRequest {
		t.Fatalf("non-PEM should 400 locally: %d %s", w.Code, w.Body.String())
	}

	// 伪 PEM(有 BEGIN 行但内容无效)通过本地校验,由 dataplaneapi 拒绝 → 502 透传
	fake := "-----BEGIN CERTIFICATE-----\nZm9vYmFyCg==\n-----END CERTIFICATE-----\n"
	if w := doJSON(t, r, http.MethodPost, certBase, token, map[string]string{"name": certName, "content": fake}); w.Code != http.StatusBadGateway {
		t.Fatalf("fake pem should be rejected by dataplaneapi: %d %s", w.Code, w.Body.String())
	}

	w := doJSON(t, r, http.MethodPost, certBase, token, map[string]string{"name": certName, "content": mustTestPEM(t)})
	if w.Code != http.StatusCreated {
		t.Fatalf("upload: %d %s", w.Code, w.Body.String())
	}
	if w.Code != http.StatusCreated {
		t.Fatalf("upload: %d %s", w.Code, w.Body.String())
	}

	// 列表包含,且带真实解析出的元数据(CN=test.local)
	w = doJSON(t, r, http.MethodGet, certBase, token, nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), certName) {
		t.Fatalf("list after upload: %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "test.local") {
		t.Errorf("list should carry parsed subject, got %s", w.Body.String())
	}

	// 删除 → 列表不再包含;删除触发的 reload 与配置无关(证书未被引用),应为 succeeded
	w = doJSON(t, r, http.MethodDelete, certBase+"/"+certName, token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", w.Code, w.Body.String())
	}
	w = doJSON(t, r, http.MethodGet, certBase, token, nil)
	if strings.Contains(w.Body.String(), certName) {
		t.Errorf("cert still listed after delete")
	}
}
