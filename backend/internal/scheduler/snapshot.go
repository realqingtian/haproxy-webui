package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"

	"haproxy-webui/backend/internal/cryptoutil"
	"haproxy-webui/backend/internal/dataplane"
	"haproxy-webui/backend/internal/model"
	"haproxy-webui/backend/internal/settings"
)

// 快照巡检结果值(settings 中的 snapshot_status.result)。
const (
	ResultClean    = "clean"
	ResultDrift    = "drift"
	ResultBaseline = "baseline"
	ResultError    = "error"
)

// CaptureRevision 拉取节点当前 raw 配置并落一条快照,返回快照 ID 与 raw。
// 与 gin 解耦,HTTP handler 与后台巡检共用。
func CaptureRevision(ctx context.Context, db *gorm.DB, client *dataplane.Client, instanceID uint, note, createdBy, source string, drifted bool) (uint, string, error) {
	version, err := client.Version(ctx)
	if err != nil {
		return 0, "", fmt.Errorf("读取配置版本: %w", err)
	}
	raw, err := client.RawConfig(ctx)
	if err != nil {
		return 0, "", fmt.Errorf("读取配置内容: %w", err)
	}
	rev := model.ConfigRevision{
		InstanceID: instanceID, Version: version,
		Note: note, CreatedBy: createdBy, Source: source, Drifted: drifted, Raw: raw,
	}
	if err := db.Create(&rev).Error; err != nil {
		return 0, "", fmt.Errorf("保存快照: %w", err)
	}
	return rev.ID, raw, nil
}

// runSnapshotCheck 对单个实例做一次快照巡检:拉取 raw 与最近一条快照比对,
// 仅在漂移或无基线时落新快照(防止快照表按周期无限膨胀),并把结果写入巡检状态。
func (s *Scheduler) runSnapshotCheck(ctx context.Context, inst model.Instance) {
	client := dataplane.NewClient(inst.BaseURL, inst.Username, cryptoutil.DecryptStoredOrDefault(inst.Password))
	status := settings.SnapshotStatus{InstanceID: inst.ID, InstanceName: inst.Name, LastRunAt: time.Now().Format(time.RFC3339)}

	raw, err := client.RawConfig(ctx)
	if err != nil {
		status.Result = ResultError
		status.Detail = err.Error()
	} else {
		var latest model.ConfigRevision
		err := s.db.Select("id, raw").Where("instance_id = ?", inst.ID).Order("id desc").First(&latest).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			// 尚无基线:落第一条定时基线快照,不算漂移
			_, _, err = CaptureRevision(ctx, s.db, client, inst.ID,
				"定时巡检建立基线", "scheduler", model.RevisionSourceScheduled, false)
			if err == nil {
				status.Result = ResultBaseline
				status.Detail = "已建立基线快照"
			}
		case err != nil:
			// 查询失败按错误处理
		case raw != latest.Raw:
			// 漂移:节点配置与最近快照不一致(绕过 UI 的手工修改)
			_, _, err = CaptureRevision(ctx, s.db, client, inst.ID,
				"检测到配置漂移(节点配置与最近快照不一致)", "scheduler", model.RevisionSourceScheduled, true)
			if err == nil {
				status.Result = ResultDrift
				status.Detail = "节点配置与最近快照不一致,已保存漂移快照"
			}
		default:
			status.Result = ResultClean
			status.Detail = "配置无变化"
		}
		if err != nil {
			status.Result = ResultError
			status.Detail = err.Error()
		}
	}

	s.recordSnapshotStatus(ctx, status)
}

// recordSnapshotStatus 把单实例巡检结果合并进聚合状态并落库。
func (s *Scheduler) recordSnapshotStatus(ctx context.Context, st settings.SnapshotStatus) {
	all, err := settings.LoadSnapshotStatus(ctx, s.db)
	if err != nil {
		log.Printf("scheduler: load snapshot status: %v", err)
		return
	}
	replaced := false
	for i := range all {
		if all[i].InstanceID == st.InstanceID {
			all[i] = st
			replaced = true
			break
		}
	}
	if !replaced {
		all = append(all, st)
	}
	if err := settings.SaveSnapshotStatus(ctx, s.db, all); err != nil {
		log.Printf("scheduler: save snapshot status: %v", err)
	}
}
