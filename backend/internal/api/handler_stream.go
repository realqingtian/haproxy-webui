package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"haproxy-webui/backend/internal/model"
	"haproxy-webui/backend/internal/systemd"
)

// ---- SSE 实时推送(v0.11)----
//
// 技术决策(见 PLAN v0.11):服务端单向推送用 SSE 而非 WebSocket——零新依赖,
// Bearer 鉴权头天然可用;前端用 fetch 流式读取 + 自动重连/回落。
// 两个流:stats 快照(周期拉 dataplaneapi 后推送)与 haproxy 日志尾部(SSH tail -f)。

const statsStreamInterval = 5 * time.Second

// 默认日志路径与安全字符集:绝对路径,防注入(命令拼装另有单引号包裹兜底)。
const defaultLogPath = "/var/log/haproxy.log"

var logPathRe = regexp.MustCompile(`^/[A-Za-z0-9/._-]{1,250}$`)

// sseHeaders 写入 SSE 必需的响应头并返回 Flusher。
func sseHeaders(c *gin.Context) http.Flusher {
	w := c.Writer
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no") // nginx 反代不缓冲
	w.WriteHeader(http.StatusOK)
	f := w.(http.Flusher)
	f.Flush()
	return f
}

// sseData 写一条 data 事件(line 为 JSON 编码后的载荷)。
func sseData(c *gin.Context, f http.Flusher, payload any) {
	b, err := json.Marshal(payload)
	if err != nil {
		return
	}
	fmt.Fprintf(c.Writer, "data: %s\n\n", b)
	f.Flush()
}

// StatsStream GET /api/instances/:id/stats/stream — 每 5s 拉取全量 stats 并推送。
// dataplaneapi 拉取失败时输出注释行保持连接(由连通性告警兜底),客户端断开即结束。
func (h *NodeHandler) StatsStream(c *gin.Context) {
	client, ok := h.clientFromContext(c)
	if !ok {
		return
	}
	flusher := sseHeaders(c)
	ctx := c.Request.Context()

	push := func() bool {
		stats, err := client.NativeStats(ctx, "")
		if err != nil {
			fmt.Fprintf(c.Writer, ": pull failed %s\n\n", strings.ReplaceAll(err.Error(), "\n", " "))
			flusher.Flush()
			return ctx.Err() == nil
		}
		sseData(c, flusher, stats)
		return ctx.Err() == nil
	}

	if !push() {
		return
	}
	ticker := time.NewTicker(statsStreamInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !push() {
				return
			}
		}
	}
}

// LogsStream GET /api/instances/:id/logs/stream?path= — SSH tail -f 日志尾部,
// 逐行 JSON 推送;需实例配置 SSH(读为登录可见,与 raw 配置一致)。
func (h *NodeHandler) LogsStream(c *gin.Context) {
	var inst model.Instance
	if err := h.db.First(&inst, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "instance not found"})
		return
	}
	if !inst.SSHConfigured() {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "实例未配置 SSH,无法读取节点日志",
			"hint":  "编辑实例填写 SSH 用户与凭据后即可实时查看日志尾部",
		})
		return
	}
	path := c.Query("path")
	if path == "" {
		path = inst.LogPath
	}
	if path == "" {
		path = defaultLogPath
	}
	if !logPathRe.MatchString(path) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "日志路径需为绝对路径且仅含字母数字与 / . _ -"})
		return
	}
	cfg, err := systemd.ConfigFromInstance(&inst)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	flusher := sseHeaders(c)
	ctx := c.Request.Context()
	defer persistHostKey(h.db, &inst, &cfg)

	err = cfg.TailStream(ctx, path, func(line string) {
		sseData(c, flusher, line)
	})
	if err != nil && ctx.Err() == nil {
		// 连接已建立(SSE 头已发),以 error 事件透出:节点侧失败(路径不存在 / 认证失败等)
		sseData(c, flusher, map[string]string{"error": err.Error(), "hint": systemd.Hint(err)})
	}
}
