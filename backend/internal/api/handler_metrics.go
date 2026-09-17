package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"haproxy-webui/backend/internal/model"
)

// metricsProbeClient 探测节点 /metrics 用:与 dataplaneapi 无关的普通 HTTP,超时收紧。
var metricsProbeClient = &http.Client{Timeout: 5 * time.Second}

// metricsEndpoint 返回实例的 metrics 完整地址:优先手工配置的 MetricsURL,
// 否则按 BaseURL host 的 8404 端口推导(haproxy 官方 stats 导出默认端口)。
func metricsEndpoint(inst *model.Instance) string {
	if u := strings.TrimSpace(inst.MetricsURL); u != "" {
		u = strings.TrimSuffix(u, "/")
		if strings.HasSuffix(u, "/metrics") {
			return u
		}
		return u + "/metrics"
	}
	base, err := url.Parse(inst.BaseURL)
	if err != nil || base.Hostname() == "" {
		return ""
	}
	return fmt.Sprintf("http://%s:8404/metrics", base.Hostname())
}

// MetricsProbe GET /api/instances/:id/metrics-probe:
// 探测节点 Prometheus /metrics 是否可访问,未接入时返回接入指引。
func (h *NodeHandler) MetricsProbe(c *gin.Context) {
	var inst model.Instance
	if err := h.db.First(&inst, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "instance not found"})
		return
	}
	endpoint := metricsEndpoint(&inst)
	if endpoint == "" {
		c.JSON(http.StatusOK, gin.H{
			"ok":     false,
			"url":    "",
			"detail": "无法推导 metrics 地址(BaseURL 无效且未配置 Metrics 地址)",
			"hint":   "在实例管理中编辑该实例,填写 Prometheus metrics 基地址(如 http://节点IP:8404)",
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "url": endpoint, "detail": err.Error()})
		return
	}
	resp, err := metricsProbeClient.Do(req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"ok":     false,
			"url":    endpoint,
			"detail": err.Error(),
			"hint":   "节点 8404 端口未开通或防火墙拦截。可在 haproxy.cfg 中启用 stats 导出后重试,参见部署文档「监控接入」一节",
		})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusOK, gin.H{
			"ok":     false,
			"url":    endpoint,
			"detail": fmt.Sprintf("HTTP %d", resp.StatusCode),
			"hint":   "metrics 端点返回异常,检查 stats 导出配置(需 haproxy 2.6+ 且启用 prometheus-exporter)",
		})
		return
	}
	// 简单校验内容形态:Prometheus 导出应包含 haproxy 前缀的指标
	hasMetrics := strings.Contains(string(body), "haproxy_")
	detail := fmt.Sprintf("HTTP 200,%.1f KB", float64(len(body))/1024)
	if hasMetrics {
		detail += ",内容含 haproxy 指标"
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "url": endpoint, "detail": detail})
}
