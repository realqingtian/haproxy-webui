package scheduler

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"haproxy-webui/backend/internal/cryptoutil"
	"haproxy-webui/backend/internal/dataplane"
	"haproxy-webui/backend/internal/model"
	"haproxy-webui/backend/internal/notify"
	"haproxy-webui/backend/internal/settings"
)

// monitorState 记录监控边沿状态:只在状态翻转时告警(附恢复通知),重启后
// 首轮探测只记录不告警,避免进程重启刷屏;冷却时间防抖动。
type monitorState struct {
	nodeUpKnown bool
	nodeUp      bool
	backendAll  map[string]bool // backend 名 -> 是否全 DOWN(已知状态)
	lastAlertAt map[string]time.Time
}

func (s *Scheduler) stateFor(instID uint) *monitorState {
	if s.mon == nil {
		s.mon = map[uint]*monitorState{}
	}
	st, ok := s.mon[instID]
	if !ok {
		st = &monitorState{backendAll: map[string]bool{}, lastAlertAt: map[string]time.Time{}}
		s.mon[instID] = st
	}
	return st
}

// cooledDown 该 key 是否已过冷却期;未过返回 false,已过则记录本次时间并返回 true。
func (m *monitorState) cooledDown(key string, cooldown time.Duration, now time.Time) bool {
	if last, ok := m.lastAlertAt[key]; ok && now.Sub(last) < cooldown {
		return false
	}
	m.lastAlertAt[key] = now
	return true
}

// runMonitorChecks 对到期实例执行连通性探测与 stats 拉取,按边沿触发告警。
func runMonitorChecks(ctx context.Context, s *Scheduler, cfg settings.Config, instances []model.Instance, now time.Time) {
	if cfg.MonitorIntervalSeconds < 10 {
		return // 未启用(或非法值)
	}
	interval := time.Duration(cfg.MonitorIntervalSeconds) * time.Second
	cooldown := time.Duration(cfg.AlertCooldownMinutes) * time.Minute

	for _, inst := range instances {
		if !s.due(s.lastMonitorAt, inst.ID, interval, now) {
			continue
		}
		st := s.stateFor(inst.ID)
		client := dataplane.NewClient(inst.BaseURL, inst.Username, cryptoutil.DecryptStoredOrDefault(inst.Password))

		// 连通性:dataplaneapi /v3/info
		if _, err := client.Info(ctx); err != nil {
			if st.nodeUpKnown && st.nodeUp && st.cooledDown("node", cooldown, now) {
				notify.Send(ctx, s.db, notify.Alert{
					Kind: notify.KindNodeDown, Instance: inst.Name,
					Detail: fmt.Sprintf("dataplaneapi 探测失败:%v", err),
				})
			}
			st.nodeUpKnown, st.nodeUp = true, false
			continue // 节点不通则不必再拉 stats
		}
		if st.nodeUpKnown && !st.nodeUp && st.cooledDown("node", cooldown, now) {
			notify.Send(ctx, s.db, notify.Alert{
				Kind: notify.KindNodeRecovered, Instance: inst.Name, Detail: "dataplaneapi 恢复可达",
			})
		}
		st.nodeUpKnown, st.nodeUp = true, true

		// backend 全 DOWN:按 backend 聚合 server 状态,边沿触发
		stats, err := client.NativeStats(ctx, "")
		if err != nil {
			log.Printf("monitor: stats %q: %v", inst.Name, err)
			continue
		}
		allDown := map[string]bool{}
		serverNames := map[string][]string{}
		for _, obj := range stats {
			if obj.Type != "server" || obj.BackendName == "" {
				continue
			}
			status, _ := obj.Stats["status"].(string)
			if !strings.HasPrefix(status, "DOWN") && status != "" {
				allDown[obj.BackendName] = false // 有一个非 DOWN 即不算全 DOWN
				continue
			}
			if _, seen := allDown[obj.BackendName]; !seen {
				allDown[obj.BackendName] = true
			}
			serverNames[obj.BackendName] = append(serverNames[obj.BackendName], obj.Name)
		}
		for backend, down := range allDown {
			known, seen := st.backendAll[backend]
			if !seen {
				st.backendAll[backend] = down // 首轮只记录
				continue
			}
			if down == known {
				continue
			}
			st.backendAll[backend] = down
			if down && st.cooledDown("backend:"+backend, cooldown, now) {
				notify.Send(ctx, s.db, notify.Alert{
					Kind: notify.KindBackendDown, Instance: inst.Name,
					Detail: fmt.Sprintf("backend %q 全部服务器 DOWN(%s)", backend, strings.Join(serverNames[backend], ", ")),
				})
			} else if !down {
				notify.Send(ctx, s.db, notify.Alert{
					Kind: notify.KindBackendUp, Instance: inst.Name,
					Detail: fmt.Sprintf("backend %q 恢复(部分服务器 UP)", backend),
				})
			}
		}
	}
}
