package api

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

// ---- SSL 证书管理(dataplaneapi storage ssl_certificates,v0.7)----
//
// 节点侧前提:dataplaneapi 启动参数带 --ssl-certs-dir,否则 dataplaneapi 不注册
// storage 路由(404 path not found),本文件所有接口以 502 + hint 透出该情况。

// 证书文件名约束:防止路径穿越,仅允许安全字符;dataplaneapi 以文件名作为存储键。
var certNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// certNotFoundHint 识别「storage 路由未注册」型 404(dataplaneapi 对路由缺失与
// 对象缺失都返回 404,靠 message 区分),给前端可操作的修复提示。
func certNotFoundHint(err error) string {
	msg := err.Error()
	if strings.Contains(msg, "returned 404") && strings.Contains(msg, "path") {
		return "节点 dataplaneapi 未启用证书存储:需在其启动参数中增加 --ssl-certs-dir(如 /etc/haproxy/ssl)后重启 dataplaneapi"
	}
	return ""
}

// ListSSLCerts GET /api/instances/:id/certs — 证书元数据列表(dataplaneapi 不提供内容读取)。
// dataplaneapi 3.4.3 的列表只做文件列举(不解析证书),对零值元数据逐个补查详情;
// 补查失败时保留列表原值,不影响整体展示。
func (h *NodeHandler) ListSSLCerts(c *gin.Context) {
	client, ok := h.clientFromContext(c)
	if !ok {
		return
	}
	certs, err := client.ListSSLCertificates(c.Request.Context())
	if err != nil {
		hint := certNotFoundHint(err)
		resp := gin.H{"error": err.Error()}
		if hint != "" {
			resp["hint"] = hint
		}
		c.JSON(http.StatusBadGateway, resp)
		return
	}
	for i := range certs {
		cert := &certs[i]
		if cert.Subject != "" || cert.StorageName == "" {
			continue
		}
		if detail, err := client.GetSSLCertificate(c.Request.Context(), cert.StorageName); err == nil {
			cert.Subject, cert.Issuers, cert.Serial = detail.Subject, detail.Issuers, detail.Serial
			cert.NotBefore, cert.NotAfter, cert.Size = detail.NotBefore, detail.NotAfter, detail.Size
		}
	}
	c.JSON(http.StatusOK, certs)
}

type certUploadRequest struct {
	Name    string `json:"name" binding:"required"`
	Content string `json:"content" binding:"required"` // PEM 文本(可含私钥)
}

// UploadSSLCert POST /api/instances/:id/certs — 上传证书(只写文件,不触发 reload;
// 生效需配置引用该证书并 reload)。
func (h *NodeHandler) UploadSSLCert(c *gin.Context) {
	client, ok := h.clientFromContext(c)
	if !ok {
		return
	}
	var req certUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !certNameRe.MatchString(req.Name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "证书文件名仅允许字母、数字与 . _ - 字符,并以字母或数字开头"})
		return
	}
	content := []byte(req.Content)
	if len(content) > 128<<10 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "证书内容超过 128KB 上限"})
		return
	}
	if !strings.Contains(req.Content, "BEGIN CERTIFICATE") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "内容不是 PEM 证书(缺少 BEGIN CERTIFICATE)"})
		return
	}

	cert, err := client.CreateSSLCertificate(c.Request.Context(), req.Name, content)
	if err != nil {
		hint := certNotFoundHint(err)
		resp := gin.H{"error": err.Error()}
		if hint != "" {
			resp["hint"] = hint
		}
		c.JSON(http.StatusBadGateway, resp)
		return
	}
	audit(c, "cert.upload", req.Name, "subject="+cert.Subject)
	c.JSON(http.StatusCreated, cert)
}

// DeleteSSLCert DELETE /api/instances/:id/certs/:name — 删除证书文件。
// dataplaneapi 默认删除后触发 reload:被引用的证书删除后 reload 会失败(旧进程继续服务),
// 错误立即暴露并进入 v0.6 告警,前端也会拿到 Reload-ID 供查询结果。
func (h *NodeHandler) DeleteSSLCert(c *gin.Context) {
	client, ok := h.clientFromContext(c)
	if !ok {
		return
	}
	name := c.Param("name")
	if !certNameRe.MatchString(name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid certificate name"})
		return
	}
	reloadID, err := client.DeleteSSLCertificate(c.Request.Context(), name)
	if err != nil {
		hint := certNotFoundHint(err)
		resp := gin.H{"error": err.Error()}
		if hint != "" {
			resp["hint"] = hint
		}
		c.JSON(http.StatusBadGateway, resp)
		return
	}
	detail := "deleted"
	if reloadID != "" {
		detail = "deleted, reload " + reloadID
	}
	audit(c, "cert.delete", name, detail)
	c.JSON(http.StatusOK, gin.H{"ok": true, "reloadId": reloadID})
}
