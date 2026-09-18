package api

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
)

// vrrpExec 模拟节点 keepalived 探测响应:进程是否存活 + 是否持有 VIP。
func vrrpExec(running, withVip bool) func(string) (string, int) {
	return func(cmd string) (string, int) {
		switch {
		case strings.HasPrefix(cmd, "pgrep -x keepalived"):
			if running {
				return "up\n", 0
			}
			return "down\n", 0
		case cmd == "ip -o -4 addr show":
			out := "11: eth0    inet 172.28.255.12/24 brd 172.28.255.255 scope global eth0\\       valid_lft forever\n"
			if withVip {
				out += "11: eth0    inet 172.28.255.100/24 scope global secondary eth0\\       valid_lft forever\n"
			}
			return out, 0
		}
		return "fake-ssh: unknown command\n", 127
	}
}

// TestClusterVRRPFlow 覆盖 v0.8 集群 VRRP 探测接口:主 / 备 / 未配 SSH 三种节点形态。
func TestClusterVRRPFlow(t *testing.T) {
	r, fake := newTestEnv(t)
	admin := loginToken(t, r, "admin", "admin123")

	// 建集群
	w := doJSON(t, r, http.MethodPost, "/api/clusters", admin, map[string]string{
		"name": "kvrrp-demo", "vip": "172.28.255.100",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create cluster: %d %s", w.Code, w.Body.String())
	}
	var cluster struct {
		ID uint `json:"id"`
	}
	json.Unmarshal(w.Body.Bytes(), &cluster)

	// 两个 fake SSH 节点:lb1 = MASTER(持 VIP),lb2 = BACKUP
	ssh1 := newFakeSSH(t, "sshu", "sshp", vrrpExec(true, true))
	ssh2 := newFakeSSH(t, "sshu", "sshp", vrrpExec(true, false))
	host1, port1, _ := net.SplitHostPort(ssh1)
	host2, port2, _ := net.SplitHostPort(ssh2)

	id1 := registerInstanceNamed(t, r, admin, fake.url, itDPUser, itDPPass, "lb1")
	id2 := registerInstanceNamed(t, r, admin, fake.url, itDPUser, itDPPass, "lb2")
	id3 := registerInstanceNamed(t, r, admin, fake.url, itDPUser, itDPPass, "lb3-nossh")
	// 挂集群 + 配 SSH(lb1/lb2),lb3 只入组不配 SSH
	for _, v := range []struct {
		id        uint
		name      string
		sshHost   string
		sshPort   string
		clusterID uint
	}{
		{id1, "lb1", host1, port1, cluster.ID},
		{id2, "lb2", host2, port2, cluster.ID},
		{id3, "lb3-nossh", "", "", cluster.ID},
	} {
		body := map[string]any{
			"name": v.name, "baseUrl": fake.url, "username": itDPUser,
			"clusterId": v.clusterID,
		}
		if v.sshHost != "" {
			p := mustPort(t, v.sshPort)
			body["sshHost"], body["sshPort"], body["sshUser"], body["sshPassword"] = v.sshHost, p, "sshu", "sshp"
		}
		if w := doJSON(t, r, http.MethodPut, fmt.Sprintf("/api/instances/%d", v.id), admin, body); w.Code != http.StatusOK {
			t.Fatalf("update instance %d: %d %s", v.id, w.Code, w.Body.String())
		}
	}

	// 探测
	w = doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/clusters/%d/vrrp", cluster.ID), admin, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("vrrp: %d %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{
		`"vip":"172.28.255.100"`,
		`"name":"lb1"`, `"role":"master"`,
		`"name":"lb2"`, `"role":"backup"`,
		`"name":"lb3-nossh"`, `"probeable":false`, `"role":"unknown"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("vrrp response missing %s in %s", want, body)
		}
	}

	// keepalived 挂掉 → fault
	ssh3 := newFakeSSH(t, "sshu", "sshp", vrrpExec(false, true))
	host3, port3, _ := net.SplitHostPort(ssh3)
	id4 := registerInstanceNamed(t, r, admin, fake.url, itDPUser, itDPPass, "lb4-fault")
	w = doJSON(t, r, http.MethodPut, fmt.Sprintf("/api/instances/%d", id4), admin, map[string]any{
		"name": "lb4-fault", "baseUrl": fake.url, "username": itDPUser,
		"clusterId": cluster.ID,
		"sshHost":   host3, "sshPort": mustPort(t, port3), "sshUser": "sshu", "sshPassword": "sshp",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update lb4: %d", w.Code)
	}
	w = doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/clusters/%d/vrrp", cluster.ID), admin, nil)
	if !strings.Contains(w.Body.String(), `"name":"lb4-fault"`) ||
		!strings.Contains(w.Body.String(), `"role":"fault"`) {
		t.Fatalf("fault node missing: %s", w.Body.String())
	}
}
