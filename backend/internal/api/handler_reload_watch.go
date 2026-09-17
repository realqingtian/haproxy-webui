package api

import (
	"context"
	"log"
	"time"

	"gorm.io/gorm"

	"haproxy-webui/backend/internal/dataplane"
	"haproxy-webui/backend/internal/model"
	"haproxy-webui/backend/internal/notify"
)

// watchReloadStatus 在配置提交后台轮询 reload 结果:成功静默结束,失败则
// 告警 + 审计。独立 ctx(请求返回后仍继续),总时长约 30s。
// 仅事务化提交(commit 返回 reload-id)可跟踪;回滚走 raw 整体推送,无 reload-id 可查。
func watchReloadStatus(db *gorm.DB, instanceID string, client *dataplane.Client, reloadID, note string) {
	if reloadID == "" {
		return
	}
	var inst model.Instance
	if err := db.First(&inst, instanceID).Error; err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var status string
	for {
		reload, err := client.ReloadStatus(ctx, reloadID)
		if err != nil {
			log.Printf("reload watch %s: %v", reloadID, err)
			return // 探测不到状态时不误报,由连通性告警兜底
		}
		status = reload.Status
		if status != "in_progress" {
			break
		}
		select {
		case <-ctx.Done():
			return // 超时:reload 卡住属于异常但难以定性,不告警
		case <-ticker.C:
		}
	}
	if status != "failed" {
		return
	}

	detail := "配置提交后 reload 失败: " + note
	db.Create(&model.AuditLog{
		Action: "reload.failed", Target: inst.Name, Detail: detail,
	})
	log.Printf("ALERT: instance %q %s (reload %s)", inst.Name, detail, reloadID)
	notify.Send(ctx, db, notify.Alert{Kind: notify.KindReloadFailed, Instance: inst.Name, Detail: detail})
}
