package main

import (
	"os"
	"strings"
	"testing"

	"kanban/core/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestAIPenetrationAggSelectSQL_Shape 无库回归：AI 渗透率聚合 SELECT 常量形态断言（CI 可跑）。
// 防口径被改坏——覆盖率分子(coverage_eligible)、渗透率分子(author_active 的"作者同期有会话"EXISTS 子查询)
// 必须在；否则总览渗透率卡的 ≈28%/≈72% 口径会失真。
func TestAIPenetrationAggSelectSQL_Shape(t *testing.T) {
	for _, want := range []string{
		"COUNT(*) AS total",
		"FILTER (WHERE coverage_eligible) AS eligible",
		"AS author_active",
		"EXISTS(",
		"session_stage_metrics",
		"m.user_id = needs.primary_user_id",
		"session_start_ts",
		"dev_start_ts",
	} {
		if !strings.Contains(aiPenetrationAggSelectSQL, want) {
			t.Fatalf("aiPenetrationAggSelectSQL 缺少口径片段 %q\n常量: %s", want, aiPenetrationAggSelectSQL)
		}
	}
}

// TestQueryNeedsV2_FoldOnlyEligible 验证列表折叠口径：
//   - 默认(非 includeAll)列表只剩 coverage_eligible=true 的 need（折叠掉不进计算的那批，根治满屏 "-"）；
//   - FoldedCount >= 0；
//   - includeAll("显示全部")放开折叠 → FoldedCount 归 0。
//
// 与具体数据无关（空库/旧库亦成立）。需可连测试库；CI 无库时 t.Skip。
func TestQueryNeedsV2_FoldOnlyEligible(t *testing.T) {
	dsn := os.Getenv("TEST_PG_DSN")
	if dsn == "" {
		dsn = "host=127.0.0.1 port=5434 user=postgres password=1 dbname=costrict_stat sslmode=disable"
	}
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true}), &gorm.Config{})
	if err != nil {
		t.Skipf("无测试数据库，跳过折叠测试: %v", err)
	}
	if sqlDB, e := db.DB(); e == nil {
		defer sqlDB.Close()
	}

	resp, err := QueryNeedsV2(db, NeedsV2Filter{Page: 1, PageSize: 50, FoldNonEligible: true})
	if err != nil {
		t.Skipf("查询失败（库不可用/无表），跳过: %v", err)
	}
	for _, n := range resp.Data {
		if !n.CoverageEligible {
			t.Fatalf("默认列表(折叠)不应出现 coverage_eligible=false 的 need: %s", n.NeedId)
		}
	}
	if resp.FoldedCount < 0 {
		t.Fatalf("FoldedCount 应 >= 0, got %d", resp.FoldedCount)
	}
	// 折叠算术精确不变式：FoldedCount + Total == 口径内全部(applyNeedCaliberFilter + NOT outlier_flag)，
	// 即"折叠掉的 + 列表剩的 = 折叠前全部"，锁住 Count 顺序与减法不被改坏（空库时下面 preFold=0、Total=0、
	// FoldedCount=0 亦成立）。注意：不能用 includeAll 的 total 比——includeAll 还放开了 active/主干/非软件用户。
	var preFold int64
	if e := applyNeedCaliberFilter(db.Model(&models.Need{})).Where("NOT outlier_flag").Count(&preFold).Error; e == nil {
		if resp.FoldedCount+resp.Total != preFold {
			t.Fatalf("折叠不变式破: FoldedCount(%d)+Total(%d) != 口径内全部(%d)", resp.FoldedCount, resp.Total, preFold)
		}
	}

	allResp, err := QueryNeedsV2(db, NeedsV2Filter{Page: 1, PageSize: 1, IncludeAll: true})
	if err != nil {
		t.Fatalf("includeAll 查询失败: %v", err)
	}
	if allResp.FoldedCount != 0 {
		t.Fatalf("includeAll 时 FoldedCount 应为 0, got %d", allResp.FoldedCount)
	}

	// 回归保护（Codex finding）：用户详情等复用 QueryNeedsV2 的调用方【不设 FoldNonEligible】→ 不折叠、
	// 不丢 need、FoldedCount=0；且其 Total(不折叠的口径内全部) 恰 = 列表页 折叠后Total + FoldedCount，
	// 证明折叠纯展示层、未真的丢数据。
	noFold, err := QueryNeedsV2(db, NeedsV2Filter{Page: 1, PageSize: 1})
	if err != nil {
		t.Fatalf("默认(不折叠)查询失败: %v", err)
	}
	if noFold.FoldedCount != 0 {
		t.Fatalf("未设 FoldNonEligible 不应折叠, FoldedCount 应为 0, got %d", noFold.FoldedCount)
	}
	if noFold.Total != resp.Total+resp.FoldedCount {
		t.Fatalf("不折叠 Total(%d) 应 = 列表折叠后 Total(%d) + FoldedCount(%d)", noFold.Total, resp.Total, resp.FoldedCount)
	}
}
