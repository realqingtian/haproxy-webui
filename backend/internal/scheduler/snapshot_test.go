package scheduler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"haproxy-webui/backend/internal/cryptoutil"
	"haproxy-webui/backend/internal/database"
	"haproxy-webui/backend/internal/model"
	"haproxy-webui/backend/internal/settings"
)

// 快照巡检三阶段:无基线落基线 → 无变化不落 → 变化落漂移快照。
func TestSnapshotCheckBaselineCleanDrift(t *testing.T) {
	cryptoutil.Init("scheduler-test:")
	ctx := context.Background()

	raw := "# version 1\n"
	mux := http.NewServeMux()
	mux.HandleFunc("/v3/services/haproxy/configuration/version", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("1\n"))
	})
	mux.HandleFunc("/v3/services/haproxy/configuration/raw", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(raw))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	inst := model.Instance{Name: "node-x", BaseURL: srv.URL, Username: "u", Password: "p", Enabled: true}
	if err := db.Create(&inst).Error; err != nil {
		t.Fatal(err)
	}

	s := New(db)
	countRevisions := func() int {
		var n int64
		db.Model(&model.ConfigRevision{}).Where("instance_id = ?", inst.ID).Count(&n)
		return int(n)
	}
	readStatus := func() settings.SnapshotStatus {
		st, err := settings.LoadSnapshotStatus(ctx, db)
		if err != nil || len(st) != 1 {
			t.Fatalf("snapshot status: %v %v", st, err)
		}
		return st[0]
	}

	// 1) 无基线 → 落基线快照,状态 baseline
	s.runSnapshotCheck(ctx, inst)
	if n := countRevisions(); n != 1 {
		t.Fatalf("after baseline check revisions = %d", n)
	}
	if st := readStatus(); st.Result != ResultBaseline {
		t.Fatalf("baseline result = %q (%s)", st.Result, st.Detail)
	}

	// 2) 无变化 → 不落新快照,状态 clean
	s.runSnapshotCheck(ctx, inst)
	if n := countRevisions(); n != 1 {
		t.Fatalf("clean check should not add revision, got %d", n)
	}
	if st := readStatus(); st.Result != ResultClean {
		t.Fatalf("clean result = %q", st.Result)
	}

	// 3) 配置变化 → 漂移快照,drifted = true
	raw = "# version 2\nbackend drifted\n"
	s.runSnapshotCheck(ctx, inst)
	if n := countRevisions(); n != 2 {
		t.Fatalf("drift check should add revision, got %d", n)
	}
	var latest model.ConfigRevision
	if err := db.Where("instance_id = ?", inst.ID).Order("id desc").First(&latest).Error; err != nil {
		t.Fatal(err)
	}
	if latest.Source != model.RevisionSourceScheduled || !latest.Drifted {
		t.Fatalf("drift revision source=%q drifted=%v", latest.Source, latest.Drifted)
	}
	if st := readStatus(); st.Result != ResultDrift {
		t.Fatalf("drift result = %q", st.Result)
	}
}

// 探测失败(节点不可达)应记录 error 状态且不落快照。
func TestSnapshotCheckNodeError(t *testing.T) {
	cryptoutil.Init("scheduler-test:")
	ctx := context.Background()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close() // 立即关闭制造不可达

	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	inst := model.Instance{Name: "dead-node", BaseURL: srv.URL, Username: "u", Password: "p", Enabled: true}
	if err := db.Create(&inst).Error; err != nil {
		t.Fatal(err)
	}

	New(db).runSnapshotCheck(ctx, inst)
	st, err := settings.LoadSnapshotStatus(ctx, db)
	if err != nil || len(st) != 1 {
		t.Fatalf("snapshot status: %v %v", st, err)
	}
	if st[0].Result != ResultError {
		t.Fatalf("result = %q, want error", st[0].Result)
	}
	var n int64
	db.Model(&model.ConfigRevision{}).Count(&n)
	if n != 0 {
		t.Fatalf("error check should not create revision, got %d", n)
	}
}
