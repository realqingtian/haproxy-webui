// VRRP 主备切换告警(v0.9):按探测周期对启用集群做 keepalived + VIP 探测
// (复用 v0.8 的 systemd.KeepalivedStatus),节点角色相对上次已知状态发生变化时
// 边沿触发告警(审计 vrrp.change + Webhook 推送)。首轮只记录不告警;
// 探测失败的节点记 unknown,不参与对比也不清空已知状态(失联由连通性告警覆盖)。
package scheduler

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"gorm.io/gorm"

	"haproxy-webui/backend/internal/model"
	"haproxy-webui/backend/internal/notify"
	"haproxy-webui/backend/internal/systemd"
)

// vrrpChanges 对比两次角色快照,返回「节点名: 旧角色 → 新角色」描述列表。
// 仅 master / backup / fault 参与对比;unknown(探测失败)忽略。
func vrrpChanges(prev, next map[uint]string, nameByID map[uint]string) []string {
	var changes []string
	ids := make([]uint, 0, len(next))
	for id := range next {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		oldRole, newRole := prev[id], next[id]
		if !isTrackedRole(oldRole) || !isTrackedRole(newRole) || oldRole == newRole {
			continue
		}
		name := nameByID[id]
		if name == "" {
			name = fmt.Sprintf("实例 %d", id)
		}
		changes = append(changes, fmt.Sprintf("%s: %s → %s", name, roleLabel(oldRole), roleLabel(newRole)))
	}
	return changes
}

func isTrackedRole(role string) bool {
	return role == "master" || role == "backup" || role == "fault"
}

func roleLabel(role string) string {
	switch role {
	case "master":
		return "主"
	case "backup":
		return "备"
	case "fault":
		return "故障"
	default:
		return "未知"
	}
}

// vrrpClusterState 是集群各节点上次已知的 VRRP 角色(内存态,重启后重建基线)。
type vrrpClusterState struct {
	roles map[uint]string // instanceID → role;nil 表示尚未观测
}

func (s *Scheduler) vrrpStateFor(clusterID uint) *vrrpClusterState {
	if s.vrrp == nil {
		s.vrrp = map[uint]*vrrpClusterState{}
	}
	st, ok := s.vrrp[clusterID]
	if !ok {
		st = &vrrpClusterState{}
		s.vrrp[clusterID] = st
	}
	return st
}

// runVRRPChecks 对到期集群执行 VRRP 探测并做边沿告警。
func (s *Scheduler) runVRRPChecks(ctx context.Context, db *gorm.DB, interval time.Duration, instances []model.Instance, now time.Time) {
	// 按集群归组;未配 SSH 或未入组的实例不探测
	byCluster := map[uint][]model.Instance{}
	for _, inst := range instances {
		if inst.ClusterID != nil && inst.SSHConfigured() {
			byCluster[*inst.ClusterID] = append(byCluster[*inst.ClusterID], inst)
		}
	}
	if len(byCluster) == 0 {
		return
	}
	var clusters []model.Cluster
	if err := db.WithContext(ctx).Find(&clusters).Error; err != nil {
		return
	}

	for _, cluster := range clusters {
		instances := byCluster[cluster.ID]
		if instances == nil || !s.due(s.lastVRRPAt, cluster.ID, interval, now) {
			continue
		}

		roles := probeClusterVRRP(ctx, cluster, instances)
		known := s.vrrpStateFor(cluster.ID)

		if known.roles == nil {
			known.roles = roles // 首轮:只记录基线,不告警
			continue
		}
		nameByID := map[uint]string{}
		for _, inst := range instances {
			nameByID[inst.ID] = inst.Name
		}
		changes := vrrpChanges(known.roles, roles, nameByID)
		if len(changes) == 0 {
			continue
		}
		// 更新已知状态:仅刷新发生变化的节点,unknown 不覆盖已知角色
		for id, role := range roles {
			if isTrackedRole(role) {
				known.roles[id] = role
			}
		}

		detail := "集群 " + cluster.Name + " 角色变化:" + joinChanges(changes)
		db.Create(&model.AuditLog{Action: "vrrp.change", Target: cluster.Name, Detail: detail})
		notify.Send(ctx, db, notify.Alert{
			Kind: notify.KindVRRPChange, Instance: cluster.Name, Detail: detail,
		})
	}
}

// probeClusterVRRP 并发探测集群内各节点角色;探测失败记 unknown。
func probeClusterVRRP(ctx context.Context, cluster model.Cluster, instances []model.Instance) map[uint]string {
	roles := make(map[uint]string, len(instances))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, inst := range instances {
		wg.Add(1)
		go func(inst model.Instance) {
			defer wg.Done()
			role := "unknown"
			cfg, err := systemd.ConfigFromInstance(&inst)
			if err == nil {
				probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()
				if st, err := cfg.KeepalivedStatus(probeCtx, cluster.Vip); err == nil {
					role = st.Role
				}
			}
			mu.Lock()
			roles[inst.ID] = role
			mu.Unlock()
		}(inst)
	}
	wg.Wait()
	return roles
}

func joinChanges(changes []string) string {
	out := ""
	for i, c := range changes {
		if i > 0 {
			out += ";"
		}
		out += " " + c
	}
	return out
}
