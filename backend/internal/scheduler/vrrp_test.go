package scheduler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"haproxy-webui/backend/internal/database"
	"haproxy-webui/backend/internal/model"
	"haproxy-webui/backend/internal/systemd/systemdtest"
)

func TestVrrpChanges(t *testing.T) {
	prev := map[uint]string{1: "master", 2: "backup"}
	next := map[uint]string{1: "backup", 2: "master", 3: "fault"} // 3 未在基线中 → 首见不告警
	names := map[uint]string{1: "lb1", 2: "lb2", 3: "lb3"}
	got := vrrpChanges(prev, next, names)
	if len(got) != 2 {
		t.Fatalf("changes = %v, want 2", got)
	}
	if got[0] != "lb1: 主 → 备" || got[1] != "lb2: 备 → 主" {
		t.Fatalf("changes = %v", got)
	}
	// 角色未变 / unknown 均忽略
	if got := vrrpChanges(map[uint]string{1: "master"}, map[uint]string{1: "master"}, names); len(got) != 0 {
		t.Fatalf("no-change should be empty, got %v", got)
	}
	if got := vrrpChanges(map[uint]string{1: "master"}, map[uint]string{1: "unknown"}, names); len(got) != 0 {
		t.Fatalf("unknown should be ignored, got %v", got)
	}
}

// 全链路:双节点集群探测 → 主备翻转 → webhook 收到 vrrp_change 告警 + 审计 vrrp.change;
// 首轮静默;角色不变不重复告警。
func TestVRRPFailoverAlerts(t *testing.T) {
	var master atomic.Int32
	master.Store(1)
	mkExec := func(node int32) func(string) (string, int) {
		return func(cmd string) (string, int) {
			switch {
			case strings.HasPrefix(cmd, "pgrep -x keepalived"):
				return "up\n", 0
			case cmd == "ip -o -4 addr show":
				base := fmt.Sprintf("11: eth0    inet 172.28.255.1%d/24 scope global eth0\\\n", node)
				if master.Load() == node {
					base += "11: eth0    inet 172.28.255.100/24 scope global secondary eth0\\\n"
				}
				return base, 0
			}
			return "unknown\n", 127
		}
	}
	addr1, _ := systemdtest.NewServer(t, "u", "p", mkExec(1))
	addr2, _ := systemdtest.NewServer(t, "u", "p", mkExec(2))

	var alerts atomic.Int32
	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		alerts.Add(1)
	}))
	defer webhook.Close()

	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	db.Create(&model.AlertChannel{Name: "ch", Type: model.ChannelWecom, WebhookURL: webhook.URL, Enabled: true})
	cluster := model.Cluster{Name: "kvrrp-demo", Vip: "172.28.255.100"}
	db.Create(&cluster)

	host1, port1 := splitAddr(t, addr1)
	host2, port2 := splitAddr(t, addr2)
	lb1 := model.Instance{Name: "lb1", BaseURL: "http://10.0.0.1", Username: "u", Password: "p",
		Enabled: true, ClusterID: &cluster.ID,
		SSHHost: host1, SSHPort: port1, SSHUser: "u", SSHPassword: "p"}
	lb2 := model.Instance{Name: "lb2", BaseURL: "http://10.0.0.2", Username: "u", Password: "p",
		Enabled: true, ClusterID: &cluster.ID,
		SSHHost: host2, SSHPort: port2, SSHUser: "u", SSHPassword: "p"}
	db.Create(&lb1)
	db.Create(&lb2)

	s := New(db)
	ctx := context.Background()
	instances := []model.Instance{lb1, lb2}
	now := time.Now()
	run := func() {
		now = now.Add(61 * time.Second)
		s.runVRRPChecks(ctx, db, time.Minute, instances, now)
	}

	// 首轮:建立基线,不告警
	run()
	if alerts.Load() != 0 {
		t.Fatalf("first round should be silent, alerts = %d", alerts.Load())
	}

	// 主备翻转:lb1 → 备,lb2 → 主,应告警一次
	master.Store(2)
	run()
	if alerts.Load() != 1 {
		t.Fatalf("failover should alert once, alerts = %d", alerts.Load())
	}
	var logs []model.AuditLog
	db.Where("action = ?", "vrrp.change").Order("id desc").Limit(1).Find(&logs)
	if len(logs) == 0 || !strings.Contains(logs[0].Detail, "lb1: 主 → 备") || !strings.Contains(logs[0].Detail, "lb2: 备 → 主") {
		t.Fatalf("audit detail missing role transitions: %+v", logs)
	}

	// 角色不变:不重复告警
	run()
	if alerts.Load() != 1 {
		t.Fatalf("no-change should not alert, alerts = %d", alerts.Load())
	}
}

func splitAddr(t *testing.T, addr string) (string, int) {
	t.Helper()
	i := strings.LastIndex(addr, ":")
	host := addr[:i]
	var port int
	if _, err := fmt.Sscanf(addr[i+1:], "%d", &port); err != nil {
		t.Fatalf("parse port: %v", err)
	}
	return host, port
}
