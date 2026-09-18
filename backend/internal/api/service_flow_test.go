package api

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

// ---- 进程内 SSH 服务器:模拟节点 sshd,exec 请求交由回调应答 ----

// newFakeSSH 起一个只支持 exec 的 SSH 服务器,返回监听地址。
func newFakeSSH(t *testing.T, user, pass string, exec func(cmd string) (string, int)) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("gen host key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	config := &ssh.ServerConfig{
		PasswordCallback: func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			if conn.User() == user && string(password) == pass {
				return nil, nil
			}
			return nil, fmt.Errorf("auth failed for %s", conn.User())
		},
	}
	config.AddHostKey(signer)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			sconn, chans, reqs, err := ssh.NewServerConn(conn, config)
			if err != nil {
				continue
			}
			_ = sconn
			go ssh.DiscardRequests(reqs)
			go handleSSHChannels(chans, exec)
		}
	}()
	return ln.Addr().String()
}

func handleSSHChannels(chans <-chan ssh.NewChannel, exec func(string) (string, int)) {
	for nch := range chans {
		if nch.ChannelType() != "session" {
			_ = nch.Reject(ssh.UnknownChannelType, "unsupported")
			continue
		}
		ch, reqs, err := nch.Accept()
		if err != nil {
			continue
		}
		go func(ch ssh.Channel, reqs <-chan *ssh.Request) {
			defer ch.Close()
			for req := range reqs {
				if req.Type != "exec" {
					_ = req.Reply(false, nil)
					continue
				}
				var n uint32
				_ = binary.Read(bytes.NewReader(req.Payload), binary.BigEndian, &n)
				cmd := string(req.Payload[4 : 4+n])
				out, code := exec(cmd)
				_, _ = ch.Write([]byte(out))
				status := make([]byte, 4)
				binary.BigEndian.PutUint32(status, uint32(code))
				_, _ = ch.SendRequest("exit-status", false, status)
				_ = req.Reply(true, nil)
				return
			}
		}(ch, reqs)
	}
}

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

	sshAddr := newFakeSSH(t, "sshu", "sshp", fakeSSHExec)
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
