package api

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"haproxy-webui/backend/internal/cryptoutil"
	"haproxy-webui/backend/internal/model"
	"haproxy-webui/backend/internal/systemd"
)

// ---- dataplaneapi 服务管理(SSH → systemctl,v0.7)----
//
// 状态查询登录即可读;重启为 operator+(路由层 RBAC),均审计留痕。
// 节点侧前提:实例配置了 SSH 连接;重启还需 sudoers 免密白名单(见 systemd 包注释)。

// systemdConfig 从实例记录构造 SSH 管理配置(地址回退 / 端口与 unit 默认值在这里兜底)。
func systemdConfig(inst *model.Instance) (systemd.Config, error) {
	host := strings.TrimSpace(inst.SSHHost)
	if host == "" {
		u, err := url.Parse(inst.BaseURL)
		if err != nil || u.Hostname() == "" {
			return systemd.Config{}, errors.New("SSH 地址为空且无法从 BaseURL 推导")
		}
		host = u.Hostname()
	}
	port := inst.SSHPort
	if port == 0 {
		port = 22
	}
	unit := strings.TrimSpace(inst.SSHUnit)
	if unit == "" {
		unit = "dataplaneapi"
	}
	if !systemd.ValidUnit(unit) {
		return systemd.Config{}, errors.New("unit 名包含非法字符(仅允许字母数字与 @ . _ -)")
	}
	return systemd.Config{
		Host:       host,
		Port:       port,
		User:       inst.SSHUser,
		Password:   cryptoutil.DecryptStoredOrDefault(inst.SSHPassword),
		PrivateKey: cryptoutil.DecryptStoredOrDefault(inst.SSHPrivateKey),
		Unit:       unit,
	}, nil
}

// sshErrorHint 把常见 SSH / sudo 失败翻译成可操作提示。
func sshErrorHint(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "no ssh credential"):
		return "实例未配置 SSH 凭据:编辑实例填写 SSH 用户与密码或私钥"
	case strings.Contains(msg, "parse ssh private key"):
		return "SSH 私钥格式无法解析,请提供 PEM 格式(OpenSSH 新格式需以 -----BEGIN OPENSSH PRIVATE KEY----- 开头)"
	case strings.Contains(msg, "unable to authenticate"), strings.Contains(msg, "auth failed"):
		return "SSH 认证失败:检查用户名与密码 / 私钥"
	case strings.Contains(msg, "connection refused"), strings.Contains(msg, "i/o timeout"), strings.Contains(msg, "timed out"), strings.Contains(msg, "no route"):
		return "无法建立 SSH 连接:检查地址 / 端口与节点防火墙、安全组"
	case strings.Contains(msg, "password is required"), strings.Contains(msg, "a password is required"):
		return "sudo 需要密码:请为 SSH 用户配置免密白名单,如 `sshuser ALL=(root) NOPASSWD: /usr/bin/systemctl restart dataplaneapi`"
	case strings.Contains(msg, "not found"), strings.Contains(msg, "Unknown"):
		return "节点上不存在该 unit,确认服务名是否为 dataplaneapi(可在实例设置中修改)"
	default:
		return ""
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
	cfg, err := systemdConfig(&inst)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	st, err := cfg.Status(ctx)
	if err != nil {
		resp := gin.H{"configured": true, "error": err.Error()}
		if hint := sshErrorHint(err); hint != "" {
			resp["hint"] = hint
		}
		c.JSON(http.StatusBadGateway, resp)
		return
	}
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
	cfg, err := systemdConfig(&inst)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	output, err := cfg.Restart(ctx)
	if err != nil {
		resp := gin.H{"error": err.Error()}
		if hint := sshErrorHint(err); hint != "" {
			resp["hint"] = hint
		}
		c.JSON(http.StatusBadGateway, resp)
		return
	}
	detail := strings.TrimSpace(inst.SSHUnit + "@" + cfg.Host)
	audit(c, "service.restart", detail, "systemctl restart "+cfg.Unit)
	c.JSON(http.StatusOK, gin.H{"ok": true, "output": output})
}
