// Package settings 提供系统级键值配置(SQLite Setting 表)的读写。
// 巡检周期等运维参数存 DB,经 admin 接口修改即生效、无需重启;定时任务的
// 运行状态也落在这里,便于跨重启展示。
package settings

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"haproxy-webui/backend/internal/model"
)

// 配置项 key。
const (
	KeySnapshotInterval = "snapshot.interval_minutes" // 快照巡检周期(分钟),0 = 关闭
	KeyMonitorInterval  = "monitor.interval_seconds"  // 健康与 stats 探测周期(秒)
	KeyAlertCooldown    = "alerts.cooldown_minutes"   // 同一告警的冷却时间(分钟)
	KeySnapshotStatus   = "scheduler.snapshot_status" // JSON,各实例最近一次快照巡检状态
)

const (
	DefaultSnapshotIntervalMinutes = 60
	DefaultMonitorIntervalSeconds  = 60
	DefaultAlertCooldownMinutes    = 10
)

// Config 是可在运行时调整的运维配置。
type Config struct {
	SnapshotIntervalMinutes int `json:"snapshotIntervalMinutes"`
	MonitorIntervalSeconds  int `json:"monitorIntervalSeconds"`
	AlertCooldownMinutes    int `json:"alertCooldownMinutes"`
}

// LoadConfig 读取运维配置,缺省项回落默认值;无法解析的值忽略并保留默认。
func LoadConfig(ctx context.Context, db *gorm.DB) (Config, error) {
	cfg := Config{
		SnapshotIntervalMinutes: DefaultSnapshotIntervalMinutes,
		MonitorIntervalSeconds:  DefaultMonitorIntervalSeconds,
		AlertCooldownMinutes:    DefaultAlertCooldownMinutes,
	}
	vals, err := getMany(ctx, db, KeySnapshotInterval, KeyMonitorInterval, KeyAlertCooldown)
	if err != nil {
		return cfg, err
	}
	if v, ok := vals[KeySnapshotInterval]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.SnapshotIntervalMinutes = n
		}
	}
	if v, ok := vals[KeyMonitorInterval]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MonitorIntervalSeconds = n
		}
	}
	if v, ok := vals[KeyAlertCooldown]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.AlertCooldownMinutes = n
		}
	}
	return cfg, nil
}

// SaveConfig 全量保存运维配置。
func SaveConfig(ctx context.Context, db *gorm.DB, cfg Config) error {
	return setMany(ctx, db, map[string]string{
		KeySnapshotInterval: strconv.Itoa(cfg.SnapshotIntervalMinutes),
		KeyMonitorInterval:  strconv.Itoa(cfg.MonitorIntervalSeconds),
		KeyAlertCooldown:    strconv.Itoa(cfg.AlertCooldownMinutes),
	})
}

// Get 读取单个 key;不存在返回 ok=false。
func Get(ctx context.Context, db *gorm.DB, key string) (string, bool, error) {
	var row model.Setting
	err := db.WithContext(ctx).Where("key = ?", key).First(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", false, nil
		}
		return "", false, err
	}
	return row.Value, true, nil
}

// Set 写入单个 key(upsert)。
func Set(ctx context.Context, db *gorm.DB, key, value string) error {
	return setMany(ctx, db, map[string]string{key: value})
}

// SnapshotStatus 是单个实例最近一次快照巡检的结果,全部实例聚合存为一份 JSON。
type SnapshotStatus struct {
	InstanceID   uint   `json:"instanceId"`
	InstanceName string `json:"instanceName"`
	LastRunAt    string `json:"lastRunAt"` // RFC3339;空 = 尚未运行
	Result       string `json:"result"`    // clean / drift / error
	Detail       string `json:"detail"`
}

func LoadSnapshotStatus(ctx context.Context, db *gorm.DB) ([]SnapshotStatus, error) {
	v, ok, err := Get(ctx, db, KeySnapshotStatus)
	if err != nil || !ok || v == "" {
		return []SnapshotStatus{}, err
	}
	var out []SnapshotStatus
	if err := json.Unmarshal([]byte(v), &out); err != nil {
		return []SnapshotStatus{}, nil // 状态损坏按空处理,不影响主流程
	}
	return out, nil
}

func SaveSnapshotStatus(ctx context.Context, db *gorm.DB, st []SnapshotStatus) error {
	buf, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return Set(ctx, db, KeySnapshotStatus, string(buf))
}

func getMany(ctx context.Context, db *gorm.DB, keys ...string) (map[string]string, error) {
	var rows []model.Setting
	if err := db.WithContext(ctx).Where("key IN ?", keys).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[r.Key] = r.Value
	}
	return out, nil
}

func setMany(ctx context.Context, db *gorm.DB, kv map[string]string) error {
	now := time.Now()
	rows := make([]model.Setting, 0, len(kv))
	for k, v := range kv {
		rows = append(rows, model.Setting{Key: k, Value: v, UpdatedAt: now})
	}
	return db.WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).Create(&rows).Error
}
