//go:build integration

package main

import (
	"os"
	"testing"
	"time"

	"kanban/core/models"

	"gorm.io/gorm"
)

// 真库验证 queryUserSilicaAggs 的 SQL 口径。默认不跑（integration tag），本地：
//
//	docker exec -e PGPASSWORD=1 kanban-pg psql -U postgres -c "CREATE DATABASE v2_silica_e2e;"
//	SILICA_TEST_DSN="host=127.0.0.1 port=5442 user=postgres password=1 dbname=v2_silica_e2e sslmode=disable" \
//	  go test ./backend/ -tags integration -run Silica -v
func silicaTestDSN() string {
	if dsn := os.Getenv("SILICA_TEST_DSN"); dsn != "" {
		return dsn
	}
	return "host=127.0.0.1 port=5442 user=postgres password=1 dbname=v2_silica_e2e sslmode=disable"
}

func openSilicaTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := models.OpenGormDB(silicaTestDSN())
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.Exec("TRUNCATE TABLE commits").Error; err != nil {
		t.Fatalf("truncate commits: %v", err)
	}
	return db
}

func ts(s string) time.Time {
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return v
}

// 数据形态照抄真实链路：commits 由 kbcli import-repo 写入，silica 是
// analyzeCommitSilica 算出的 0~1 比值，分母是原始 diff_lines。
func seedSilicaCommits(t *testing.T, db *gorm.DB) {
	t.Helper()
	rows := []models.Commit{
		// u-alice：两个 commit，一小一大，专门覆盖「加权 vs 平均」的差异。
		// 期望 = (3*1.0 + 300*0.01) / 303 = 6/303
		{CommitId: "c-a1", UserId: "u-alice", CommitTime: ts("2026-07-10T10:00:00Z"), DiffLines: 3, Silica: 1.0},
		{CommitId: "c-a2", UserId: "u-alice", CommitTime: ts("2026-07-11T10:00:00Z"), DiffLines: 300, Silica: 0.01},

		// u-bob：一个正常 commit + 一个被治理排除的 commit（不应计入）。
		// 期望 = 100*0.5 / 100 = 0.5（若把 excluded 算进来会变成 (50+1000)/1100≈0.95）
		{CommitId: "c-b1", UserId: "u-bob", CommitTime: ts("2026-07-10T10:00:00Z"), DiffLines: 100, Silica: 0.5},
		{CommitId: "c-b2", UserId: "u-bob", CommitTime: ts("2026-07-10T11:00:00Z"), DiffLines: 1000, Silica: 1.0, ExcludedFlag: true},

		// u-carol：commit 有行数但指纹一行没中 → 0，**不是 nil**（真实的「没用 AI」）。
		{CommitId: "c-c1", UserId: "u-carol", CommitTime: ts("2026-07-10T10:00:00Z"), DiffLines: 240, Silica: 0},

		// u-dave：只有 diff_lines=0 的 commit（空提交/纯删除）→ 应完全不出现在结果里。
		{CommitId: "c-d1", UserId: "u-dave", CommitTime: ts("2026-07-10T10:00:00Z"), DiffLines: 0, Silica: 0},

		// u-eve：commit 落在窗口外，用于验证日期过滤。
		{CommitId: "c-e1", UserId: "u-eve", CommitTime: ts("2026-06-01T10:00:00Z"), DiffLines: 50, Silica: 0.8},

		// 空 user_id：不该进聚合（防止串到某个"空用户"上）。
		{CommitId: "c-x1", UserId: "", CommitTime: ts("2026-07-10T10:00:00Z"), DiffLines: 80, Silica: 0.9},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed commits: %v", err)
	}
}

func TestQueryUserSilicaAggs_RealDB(t *testing.T) {
	db := openSilicaTestDB(t)
	seedSilicaCommits(t, db)

	start := "2026-07-01T00:00:00Z"
	end := "2026-07-31T23:59:59Z"

	aggs, err := queryUserSilicaAggs(db, start, end, "")
	if err != nil {
		t.Fatalf("queryUserSilicaAggs: %v", err)
	}

	assertRatio := func(uid string, want float64) {
		t.Helper()
		agg, ok := aggs[uid]
		if !ok {
			t.Fatalf("%s 应出现在聚合结果里", uid)
		}
		got := calcSilicaRatio(agg.SilicaWeighted, agg.SilicaWeight)
		if got == nil {
			t.Fatalf("%s 比值不应为 nil", uid)
		}
		if diff := *got - want; diff > 1e-9 || diff < -1e-9 {
			t.Fatalf("%s 含硅量 got %v, want %v (weighted=%v weight=%d)",
				uid, *got, want, agg.SilicaWeighted, agg.SilicaWeight)
		}
	}
	assertCommitAgg := func(uid string, wantCount, wantLines int64) {
		t.Helper()
		agg, ok := aggs[uid]
		if !ok {
			t.Fatalf("%s 应出现在聚合结果里", uid)
		}
		if agg.CommitCount != wantCount || agg.SilicaWeight != wantLines {
			t.Fatalf("%s commit 聚合 got %d/%d, want %d/%d",
				uid, agg.CommitCount, agg.SilicaWeight, wantCount, wantLines)
		}
	}

	// 加权而非平均
	assertRatio("u-alice", 6.0/303.0)
	assertCommitAgg("u-alice", 2, 303)
	// 治理排除的 commit 不计入
	assertRatio("u-bob", 0.5)
	assertCommitAgg("u-bob", 1, 100)
	// 零匹配是 0，不是无数据
	assertRatio("u-carol", 0)
	assertCommitAgg("u-carol", 1, 240)

	// diff_lines=0 的用户不进结果（否则会得到一个分母为 0 的假行）
	if _, ok := aggs["u-dave"]; ok {
		t.Fatal("u-dave 只有 0 行 commit，不应出现在聚合结果里")
	}
	// 窗口外的 commit 被日期过滤挡掉
	if _, ok := aggs["u-eve"]; ok {
		t.Fatal("u-eve 的 commit 在窗口外，不应出现在聚合结果里")
	}
	// 空 user_id 不成组
	if _, ok := aggs[""]; ok {
		t.Fatal("空 user_id 不应成为聚合分组")
	}
}

func TestQueryUserSilicaAggs_ScopedToSingleUser(t *testing.T) {
	db := openSilicaTestDB(t)
	seedSilicaCommits(t, db)

	aggs, err := queryUserSilicaAggs(db, "", "", "u-bob")
	if err != nil {
		t.Fatalf("queryUserSilicaAggs: %v", err)
	}
	if len(aggs) != 1 {
		t.Fatalf("指定 userID 时应只返回 1 组，got %d", len(aggs))
	}
	agg := aggs["u-bob"]
	if got := calcSilicaRatio(agg.SilicaWeighted, agg.SilicaWeight); got == nil || *got != 0.5 {
		t.Fatalf("u-bob 含硅量 got %v, want 0.5", got)
	}
	if agg.CommitCount != 1 || agg.SilicaWeight != 100 {
		t.Fatalf("u-bob commit 聚合 got %d/%d, want 1/100", agg.CommitCount, agg.SilicaWeight)
	}
}

// 无日期窗口时不应漏掉任何用户——防止有人给 applySilicaDateFilter 加上"默认近 N 天"。
func TestQueryUserSilicaAggs_NoDateFilterIncludesAll(t *testing.T) {
	db := openSilicaTestDB(t)
	seedSilicaCommits(t, db)

	aggs, err := queryUserSilicaAggs(db, "", "", "")
	if err != nil {
		t.Fatalf("queryUserSilicaAggs: %v", err)
	}
	if _, ok := aggs["u-eve"]; !ok {
		t.Fatal("无日期过滤时 u-eve 应被包含")
	}
	if agg := aggs["u-eve"]; agg.CommitCount != 1 || agg.SilicaWeight != 50 {
		t.Fatalf("u-eve commit 聚合 got %d/%d, want 1/50", agg.CommitCount, agg.SilicaWeight)
	}
}
