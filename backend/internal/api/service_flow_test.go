package api

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"haproxy-webui/backend/internal/systemd/systemdtest"
)

// fakeSSHExec 模拟节点 systemd:状态查询与重启(含 sudo 免密 / 免密未配置两种形态)。
func fakeSSHExec(cmd string) (string, int) {
	switch {
	case strings.HasPrefix(cmd, "systemctl show 'dataplaneapi'"):
		return "ActiveState=active\nSubState=running\n" +
			"ExecMainStartTimestamp=Tue 2026-09-15 10:00:00 UTC\nExecMainPid=4321\n", 0
	case cmd == "sudo -n systemctl restart 'dataplaneapi'":
		return "", 0
	case cmd == "sudo -n systemctl restart 'other'":
		return "sudo: a password is required\n", 1
	default:
		return "fake-ssh: unknown command\n", 127
	}
}

func mustPort(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(s)
	if err != nil {
		t.Fatalf("port %q: %v", s, err)
	}
	return n
}

func TestServiceManagementFlow(t *testing.T) {
	r, fake := newTestEnv(t)
	admin := loginToken(t, r, "admin", "admin123")
	id := registerInstance(t, r, admin, fake.url, itDPPass)

	sshAddr, _ := systemdtest.NewServer(t, "sshu", "sshp", fakeSSHExec)
	host, port, _ := net.SplitHostPort(sshAddr)

	// 配置 SSH(unit 非默认值路径与默认路径都覆盖:这里用默认 dataplaneapi)
	w := doJSON(t, r, http.MethodPut, fmt.Sprintf("/api/instances/%d", id), admin, map[string]any{
		"name": "it-node", "baseUrl": fake.url, "username": itDPUser,
		"sshHost": host, "sshPort": mustPort(t, port), "sshUser": "sshu", "sshPassword": "sshp",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update instance with ssh: %d %s", w.Code, w.Body.String())
	}

	// 状态查询
	w = doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/instances/%d/service", id), admin, nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"activeState":"active"`) || !strings.Contains(w.Body.String(), "4321") {
		t.Fatalf("service status: %d %s", w.Code, w.Body.String())
	}

	// viewer 可读不可重启
	if w := doJSON(t, r, http.MethodPost, "/api/users", admin, map[string]string{"username": "v2", "password": "pass1234", "role": "viewer"}); w.Code != http.StatusCreated {
		t.Fatalf("create viewer: %d", w.Code)
	}
	viewer := loginToken(t, r, "v2", "pass1234")
	if w := doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/instances/%d/service", id), viewer, nil); w.Code != http.StatusOK {
		t.Fatalf("viewer read service: %d", w.Code)
	}
	if w := doJSON(t, r, http.MethodPost, fmt.Sprintf("/api/instances/%d/service/restart", id), viewer, nil); w.Code != http.StatusForbidden {
		t.Fatalf("viewer restart should 403: %d", w.Code)
	}

	// 重启(admin)→ 200 + 审计
	w = doJSON(t, r, http.MethodPost, fmt.Sprintf("/api/instances/%d/service/restart", id), admin, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("restart: %d %s", w.Code, w.Body.String())
	}
	if w := doJSON(t, r, http.MethodGet, "/api/audit-logs?action=service.restart", admin, nil); !strings.Contains(w.Body.String(), "service.restart") {
		t.Fatalf("audit missing service.restart: %s", w.Body.String())
	}

	// sudo 未免密:unit=other 时 fake 返回密码报错 → 502 + sudoers 提示
	w = doJSON(t, r, http.MethodPut, fmt.Sprintf("/api/instances/%d", id), admin, map[string]any{
		"name": "it-node", "baseUrl": fake.url, "username": itDPUser,
		"sshHost": host, "sshPort": mustPort(t, port), "sshUser": "sshu", "sshPassword": "sshp", "sshUnit": "other",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update ssh unit: %d %s", w.Code, w.Body.String())
	}
	w = doJSON(t, r, http.MethodPost, fmt.Sprintf("/api/instances/%d/service/restart", id), admin, nil)
	if w.Code != http.StatusBadGateway || !strings.Contains(w.Body.String(), "NOPASSWD") {
		t.Fatalf("sudo password restart: %d %s", w.Code, w.Body.String())
	}

	// unit 名注入被拒
	w = doJSON(t, r, http.MethodPut, fmt.Sprintf("/api/instances/%d", id), admin, map[string]any{
		"name": "it-node", "baseUrl": fake.url, "username": itDPUser,
		"sshHost": host, "sshPort": mustPort(t, port), "sshUser": "sshu", "sshPassword": "sshp",
		"sshUnit": "dp; reboot",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update bad unit: %d", w.Code)
	}
	w = doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/instances/%d/service", id), admin, nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bad unit should 400: %d %s", w.Code, w.Body.String())
	}
}

func TestServiceNotConfigured(t *testing.T) {
	r, fake := newTestEnv(t)
	admin := loginToken(t, r, "admin", "admin123")
	id := registerInstance(t, r, admin, fake.url, itDPPass)

	w := doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/instances/%d/service", id), admin, nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"configured":false`) {
		t.Fatalf("not configured status: %d %s", w.Code, w.Body.String())
	}
	if w := doJSON(t, r, http.MethodPost, fmt.Sprintf("/api/instances/%d/service/restart", id), admin, nil); w.Code != http.StatusBadRequest {
		t.Fatalf("restart without ssh should 400: %d", w.Code)
	}
}

// v0.9 host key 指纹:TOFU 首连自动记录;钉扎后不匹配拒绝连接并给重录提示。
func TestSSHHostKeyPinning(t *testing.T) {
	r, fake := newTestEnv(t)
	admin := loginToken(t, r, "admin", "admin123")
	id := registerInstance(t, r, admin, fake.url, itDPPass)

	sshAddr, hostFP := systemdtest.NewServer(t, "sshu", "sshp", fakeSSHExec)
	host, port, _ := net.SplitHostPort(sshAddr)
	applySSH := func(fp string) {
		t.Helper()
		w := doJSON(t, r, http.MethodPut, fmt.Sprintf("/api/instances/%d", id), admin, map[string]any{
			"name": "it-node", "baseUrl": fake.url, "username": itDPUser,
			"sshHost": host, "sshPort": mustPort(t, port), "sshUser": "sshu", "sshPassword": "sshp",
			"sshHostKey": fp,
		})
		if w.Code != http.StatusOK {
			t.Fatalf("update instance: %d %s", w.Code, w.Body.String())
		}
	}

	// TOFU:未记录指纹时首连成功,实际指纹自动回写实例
	applySSH("")
	if w := doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/instances/%d/service", id), admin, nil); w.Code != http.StatusOK {
		t.Fatalf("tofu status: %d %s", w.Code, w.Body.String())
	}
	if w := doJSON(t, r, http.MethodGet, "/api/instances", admin, nil); !strings.Contains(w.Body.String(), hostFP) {
		t.Fatalf("captured fingerprint not persisted: %s (want %s)", w.Body.String(), hostFP)
	}

	// 钉扎:记录正确指纹 → 正常;错误指纹 → 拒绝连接并给重录提示
	applySSH(hostFP)
	if w := doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/instances/%d/service", id), admin, nil); w.Code != http.StatusOK {
		t.Fatalf("pinned status: %d", w.Code)
	}
	applySSH("SHA256:Zm9vYmFy")
	w := doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/instances/%d/service", id), admin, nil)
	if w.Code != http.StatusBadGateway || !strings.Contains(w.Body.String(), "host key") {
		t.Fatalf("mismatch should 502 with hint: %d %s", w.Code, w.Body.String())
	}
}
