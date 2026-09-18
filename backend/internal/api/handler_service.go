package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"haproxy-webui/backend/internal/model"
	"haproxy-webui/backend/internal/systemd"
)

// ---- dataplaneapi 服务管理(SSH → systemctl,v0.7)----
//
// 状态查询登录即可读;重启为 operator+(路由层 RBAC),均审计留痕。
// 节点侧前提:实例配置了 SSH 连接;重启还需 sudoers 免密白名单(见 systemd 包注释)。

// persistHostKey 指纹回写:实例尚未记录 host key(TOFU)时,把本次连接观察到的
// 指纹写入实例,之后进入钉扎校验;已有记录(钉扎模式)则不改动。
func persistHostKey(db *gorm.DB, inst *model.Instance, cfg *systemd.Config) {
	if inst.SSHHostKey == "" && cfg.CapturedFingerprint != "" {
		db.Model(&model.Instance{}).Where("id = ?", inst.ID).
			Update("ssh_host_key", cfg.CapturedFingerprint)
	}
}

// ServiceStatus GET /api/instances/:id/service — dataplaneapi 服务状态。
// 未配置 SSH 时返回 configured:false(前端展示设置引导而非报错)。
func (h *NodeHandler) ServiceStatus(c *gin.Context) {
	var inst model.Instance
	if err := h.db.First(&inst, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "instance not found"})
		return
	}
	if !inst.SSHConfigured() {
		c.JSON(http.StatusOK, gin.H{"configured": false})
		return
	}
	cfg, err := systemd.ConfigFromInstance(&inst)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	st, err := cfg.Status(ctx)
	if err != nil {
		resp := gin.H{"configured": true, "error": err.Error()}
		if hint := systemd.Hint(err); hint != "" {
			resp["hint"] = hint
		}
		c.JSON(http.StatusBadGateway, resp)
		return
	}
	persistHostKey(h.db, &inst, &cfg)
	c.JSON(http.StatusOK, gin.H{
		"configured":  true,
		"unit":        st.Unit,
		"activeState": st.ActiveState,
		"subState":    st.SubState,
		"since":       st.Since,
		"pid":         st.PID,
	})
}

// ServiceRestart POST /api/instances/:id/service/restart — 远程重启 dataplaneapi(operator+)。
func (h *NodeHandler) ServiceRestart(c *gin.Context) {
	var inst model.Instance
	if err := h.db.First(&inst, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "instance not found"})
		return
	}
	if !inst.SSHConfigured() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "实例未配置 SSH,无法远程重启服务"})
		return
	}
	cfg, err := systemd.ConfigFromInstance(&inst)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	output, err := cfg.Restart(ctx)
	if err != nil {
		resp := gin.H{"error": err.Error()}
		if hint := systemd.Hint(err); hint != "" {
			resp["hint"] = hint
		}
		c.JSON(http.StatusBadGateway, resp)
		return
	}
	persistHostKey(h.db, &inst, &cfg)
	detail := strings.TrimSpace(inst.SSHUnit + "@" + cfg.Host)
	audit(c, "service.restart", detail, "systemctl restart "+cfg.Unit)
	c.JSON(http.StatusOK, gin.H{"ok": true, "output": output})
}
