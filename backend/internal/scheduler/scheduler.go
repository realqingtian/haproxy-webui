// Package scheduler 是后端定时任务:周期性醒来读取运维配置,对启用的实例
// 执行快照巡检(漂移检测)与健康/stats 探测(告警触发点,见 monitor 部分)。
// 由 main 在后台 goroutine 中启动,进程收到退出信号后随 ctx 取消而停止。
package scheduler

import (
	"context"
	"log"
	"sync"
	"time"

	"gorm.io/gorm"

	"haproxy-webui/backend/internal/model"
	"haproxy-webui/backend/internal/settings"
)

// tickInterval 是调度粒度:所有周期配置的生效误差不超过一个 tick。
const tickInterval = 15 * time.Second

type Scheduler struct {
	db *gorm.DB

	// 各实例上次运行时间(内存态,重启后立即执行一轮属预期行为)
	mu             sync.Mutex
	lastSnapshotAt map[uint]time.Time
	lastMonitorAt  map[uint]time.Time

	// 监控边沿状态,由 tick 单协程访问,无需加锁
	mon map[uint]*monitorState
}

func New(db *gorm.DB) *Scheduler {
	return &Scheduler{
		db:             db,
		lastSnapshotAt: map[uint]time.Time{},
		lastMonitorAt:  map[uint]time.Time{},
	}
}

// Run 阻塞运行直到 ctx 取消(进程退出信号)。
func (s *Scheduler) Run(ctx context.Context) {
	log.Printf("scheduler started (tick %s)", tickInterval)
	s.tick(ctx)
	t := time.NewTicker(tickInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Println("scheduler stopped")
			return
		case <-t.C:
			s.tick(ctx)
		}
	}
}

// due 返回是否到达运行时间(首见实例立即运行),并在 true 时记录本次时间。
func (s *Scheduler) due(m map[uint]time.Time, id uint, interval time.Duration, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	last, ok := m[id]
	if ok && now.Sub(last) < interval {
		return false
	}
	m[id] = now
	return true
}

func (s *Scheduler) tick(ctx context.Context) {
	cfg, err := settings.LoadConfig(ctx, s.db)
	if err != nil {
		log.Printf("scheduler: load settings: %v", err)
		return
	}
	var instances []model.Instance
	if err := s.db.WithContext(ctx).Where("enabled = ?", true).Find(&instances).Error; err != nil {
		log.Printf("scheduler: list instances: %v", err)
		return
	}

	now := time.Now()
	if cfg.SnapshotIntervalMinutes > 0 {
		interval := time.Duration(cfg.SnapshotIntervalMinutes) * time.Minute
		for _, inst := range instances {
			if s.due(s.lastSnapshotAt, inst.ID, interval, now) {
				s.runSnapshotCheck(ctx, inst)
			}
		}
	}
	runMonitorChecks(ctx, s, cfg, instances, now)
}
