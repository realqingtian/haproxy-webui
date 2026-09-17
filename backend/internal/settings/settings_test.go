package settings

import (
	"context"
	"testing"

	"haproxy-webui/backend/internal/database"
)

func TestConfigRoundTripAndDefaults(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}

	// 未写入任何配置时应得到默认值
	cfg, err := LoadConfig(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SnapshotIntervalMinutes != DefaultSnapshotIntervalMinutes ||
		cfg.MonitorIntervalSeconds != DefaultMonitorIntervalSeconds ||
		cfg.AlertCooldownMinutes != DefaultAlertCooldownMinutes {
		t.Fatalf("defaults not applied: %+v", cfg)
	}

	// 保存后再读应一致(upsert 可重复执行)
	cfg.SnapshotIntervalMinutes = 0
	cfg.MonitorIntervalSeconds = 30
	cfg.AlertCooldownMinutes = 5
	if err := SaveConfig(ctx, db, cfg); err != nil {
		t.Fatal(err)
	}
	if err := SaveConfig(ctx, db, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := LoadConfig(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if got != cfg {
		t.Fatalf("round trip mismatch: got %+v want %+v", got, cfg)
	}
}

func TestSnapshotStatusRoundTrip(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}

	// 空状态应返回空切片而非错误
	st, err := LoadSnapshotStatus(ctx, db)
	if err != nil || len(st) != 0 {
		t.Fatalf("empty status: %v %v", st, err)
	}

	want := []SnapshotStatus{
		{InstanceID: 1, InstanceName: "node-a", LastRunAt: "2026-09-17T10:00:00Z", Result: "clean"},
		{InstanceID: 2, InstanceName: "node-b", Result: "drift", Detail: "检测到配置漂移"},
	}
	if err := SaveSnapshotStatus(ctx, db, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSnapshotStatus(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("status round trip mismatch: got %+v", got)
	}
}
