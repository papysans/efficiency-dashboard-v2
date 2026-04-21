//go:build integration

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

// setupRepoTestRouter 创建测试用 gin 路由，指向 costrict_stat 数据库
func setupRepoTestRouter(t *testing.T) (*gin.Engine, func()) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	tdb := testDB(t)

	// 设置全局变量供 handler 使用
	statDB = tdb
	appConfig.TaskRealMinutes.GapThresholdMinutes = 30
	appConfig.TaskRealMinutes.ExtensionMinutes = 5

	r := gin.New()
	v2 := r.Group("/api/v2")
	v2.GET("/repos", listReposV2)
	v2.GET("/repos/detail", getRepoDetailV2)
	v2.GET("/repos/branches", listRepoBranchesV2)

	cleanup := func() { tdb.Close() }
	return r, cleanup
}

// ============================================================
// 测试点 1: ListRepoAggregates task_count 聚合正确性
// ============================================================

func TestRepoAggregates_TaskCount(t *testing.T) {
	tdb := testDB(t)
	defer tdb.Close()

	repoAddr := "test-repo-agg-taskcount-" + fmt.Sprintf("%d", time.Now().UnixNano())
	repoBranch := "main"
	baseTime := time.Date(2025, 3, 15, 10, 0, 0, 0, time.UTC)

	// commit 1: 有效 JSON 数组 ["t1","t2","t3"]
	_, err := tdb.Exec(`INSERT INTO commits (commit_id, repo_addr, repo_branch, commit_time, task_ids)
		VALUES ($1, $2, $3, $4, $5::jsonb)`,
		"test-tc-001-"+repoAddr[:20], repoAddr, repoBranch, baseTime, `["t1","t2","t3"]`)
	if err != nil {
		t.Fatalf("插入 commit 1 失败: %v", err)
	}
	defer tdb.Exec(`DELETE FROM commits WHERE repo_addr = $1`, repoAddr)

	// commit 2: task_ids 为 NULL
	_, err = tdb.Exec(`INSERT INTO commits (commit_id, repo_addr, repo_branch, commit_time, task_ids)
		VALUES ($1, $2, $3, $4, NULL)`,
		"test-tc-002-"+repoAddr[:20], repoAddr, repoBranch, baseTime.Add(time.Hour))
	if err != nil {
		t.Fatalf("插入 commit 2 失败: %v", err)
	}

	// commit 3: task_ids 为 "null" (jsonb null)
	_, err = tdb.Exec(`INSERT INTO commits (commit_id, repo_addr, repo_branch, commit_time, task_ids)
		VALUES ($1, $2, $3, $4, 'null'::jsonb)`,
		"test-tc-003-"+repoAddr[:20], repoAddr, repoBranch, baseTime.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("插入 commit 3 失败: %v", err)
	}

	// commit 4: task_ids 为 "[]" (空数组)
	_, err = tdb.Exec(`INSERT INTO commits (commit_id, repo_addr, repo_branch, commit_time, task_ids)
		VALUES ($1, $2, $3, $4, '[]'::jsonb)`,
		"test-tc-004-"+repoAddr[:20], repoAddr, repoBranch, baseTime.Add(3*time.Hour))
	if err != nil {
		t.Fatalf("插入 commit 4 失败: %v", err)
	}

	// 查询聚合结果
	results, err := ListRepoAggregates(tdb, "", "")
	if err != nil {
		t.Fatalf("ListRepoAggregates 失败: %v", err)
	}

	// 查找测试 repo 的结果
	var found bool
	for _, item := range results {
		addr, _ := item["repo_addr"].(*string)
		if addr != nil && *addr == repoAddr {
			found = true
			taskCount, ok := item["task_count"].(int)
			if !ok {
				t.Fatalf("task_count 类型不是 int: %T", item["task_count"])
			}
			if taskCount != 3 {
				t.Errorf("task_count = %d, want 3 (只有第一条 commit 有 3 个有效 task)", taskCount)
			}
			commitCount, ok := item["commit_count"].(int)
			if !ok {
				t.Fatalf("commit_count 类型不是 int: %T", item["commit_count"])
			}
			if commitCount != 4 {
				t.Errorf("commit_count = %d, want 4", commitCount)
			}
			break
		}
	}
	if !found {
		t.Errorf("未找到 repo_addr=%s 的聚合结果", repoAddr)
	}
}

// ============================================================
// 测试点 2: ListRepoAggregates efficiency_ratio 计算正确性
// ============================================================

func TestRepoAggregates_EfficiencyRatio(t *testing.T) {
	tdb := testDB(t)
	defer tdb.Close()

	baseTime := time.Date(2025, 3, 15, 10, 0, 0, 0, time.UTC)

	// 子测试 2a: 正常情况 ancient=480, real=120 → ratio=400.0
	t.Run("正常计算ratio", func(t *testing.T) {
		repoAddr := "test-repo-agg-eff-normal-" + fmt.Sprintf("%d", time.Now().UnixNano())
		_, err := tdb.Exec(`INSERT INTO commits (commit_id, repo_addr, repo_branch, commit_time, commit_ancient_minutes, commit_real_minutes)
			VALUES ($1, $2, 'main', $3, 480, 120)`,
			"test-eff-001-"+repoAddr[:20], repoAddr, baseTime)
		if err != nil {
			t.Fatalf("插入 commit 失败: %v", err)
		}
		defer tdb.Exec(`DELETE FROM commits WHERE repo_addr = $1`, repoAddr)

		results, err := ListRepoAggregates(tdb, "", "")
		if err != nil {
			t.Fatalf("ListRepoAggregates 失败: %v", err)
		}
		for _, item := range results {
			addr, _ := item["repo_addr"].(*string)
			if addr != nil && *addr == repoAddr {
				ratio, ok := item["efficiency_ratio"].(float64)
				if !ok {
					t.Fatalf("efficiency_ratio 不是 float64: %T (%v)", item["efficiency_ratio"], item["efficiency_ratio"])
				}
				if ratio != 400.0 {
					t.Errorf("efficiency_ratio = %v, want 400.0", ratio)
				}
				return
			}
		}
		t.Errorf("未找到 repo_addr=%s 的聚合结果", repoAddr)
	})

	// 子测试 2b: real 为 NULL → ratio 应为 nil
	t.Run("real为NULL则ratio为nil", func(t *testing.T) {
		repoAddr := "test-repo-agg-eff-nil-" + fmt.Sprintf("%d", time.Now().UnixNano())
		_, err := tdb.Exec(`INSERT INTO commits (commit_id, repo_addr, repo_branch, commit_time, commit_ancient_minutes, commit_real_minutes)
			VALUES ($1, $2, 'main', $3, 100, NULL)`,
			"test-eff-002-"+repoAddr[:20], repoAddr, baseTime)
		if err != nil {
			t.Fatalf("插入 commit 失败: %v", err)
		}
		defer tdb.Exec(`DELETE FROM commits WHERE repo_addr = $1`, repoAddr)

		results, err := ListRepoAggregates(tdb, "", "")
		if err != nil {
			t.Fatalf("ListRepoAggregates 失败: %v", err)
		}
		for _, item := range results {
			addr, _ := item["repo_addr"].(*string)
			if addr != nil && *addr == repoAddr {
				if item["efficiency_ratio"] != nil {
					t.Errorf("efficiency_ratio = %v, want nil (real 为 NULL)", item["efficiency_ratio"])
				}
				return
			}
		}
		t.Errorf("未找到 repo_addr=%s 的聚合结果", repoAddr)
	})

	// 子测试 2c: 两者均为 NULL → ratio 为 nil
	t.Run("两者均NULL则ratio为nil", func(t *testing.T) {
		repoAddr := "test-repo-agg-eff-both-" + fmt.Sprintf("%d", time.Now().UnixNano())
		_, err := tdb.Exec(`INSERT INTO commits (commit_id, repo_addr, repo_branch, commit_time, commit_ancient_minutes, commit_real_minutes)
			VALUES ($1, $2, 'main', $3, NULL, NULL)`,
			"test-eff-003-"+repoAddr[:20], repoAddr, baseTime)
		if err != nil {
			t.Fatalf("插入 commit 失败: %v", err)
		}
		defer tdb.Exec(`DELETE FROM commits WHERE repo_addr = $1`, repoAddr)

		results, err := ListRepoAggregates(tdb, "", "")
		if err != nil {
			t.Fatalf("ListRepoAggregates 失败: %v", err)
		}
		for _, item := range results {
			addr, _ := item["repo_addr"].(*string)
			if addr != nil && *addr == repoAddr {
				if item["efficiency_ratio"] != nil {
					t.Errorf("efficiency_ratio = %v, want nil (两者均 NULL)", item["efficiency_ratio"])
				}
				return
			}
		}
		t.Errorf("未找到 repo_addr=%s 的聚合结果", repoAddr)
	})
}

// ============================================================
// 测试点 3: ListRepoAggregates 日期过滤
// ============================================================

func TestRepoAggregates_DateFilter(t *testing.T) {
	tdb := testDB(t)
	defer tdb.Close()

	repoAddr := "test-repo-agg-date-" + fmt.Sprintf("%d", time.Now().UnixNano())
	jan := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	jun := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

	// commit 在 1 月
	_, err := tdb.Exec(`INSERT INTO commits (commit_id, repo_addr, repo_branch, commit_time, commit_ancient_minutes)
		VALUES ($1, $2, 'main', $3, 100)`,
		"test-date-jan-"+repoAddr[:20], repoAddr, jan)
	if err != nil {
		t.Fatalf("插入 1 月 commit 失败: %v", err)
	}
	defer tdb.Exec(`DELETE FROM commits WHERE repo_addr = $1`, repoAddr)

	// commit 在 6 月
	_, err = tdb.Exec(`INSERT INTO commits (commit_id, repo_addr, repo_branch, commit_time, commit_ancient_minutes)
		VALUES ($1, $2, 'main', $3, 200)`,
		"test-date-jun-"+repoAddr[:20], repoAddr, jun)
	if err != nil {
		t.Fatalf("插入 6 月 commit 失败: %v", err)
	}

	// 用日期范围只查 1 月
	startTime := "2025-01-01T00:00:00Z"
	endTime := "2025-03-01T23:59:59Z"
	results, err := ListRepoAggregates(tdb, startTime, endTime)
	if err != nil {
		t.Fatalf("ListRepoAggregates 日期过滤失败: %v", err)
	}

	for _, item := range results {
		addr, _ := item["repo_addr"].(*string)
		if addr != nil && *addr == repoAddr {
			commitCount, _ := item["commit_count"].(int)
			if commitCount != 1 {
				t.Errorf("日期过滤后 commit_count = %d, want 1", commitCount)
			}
			sumAncient, ok := item["sum_ancient_minutes"].(*float64)
			if !ok || sumAncient == nil || *sumAncient != 100 {
				t.Errorf("日期过滤后 sum_ancient_minutes = %v, want 100", item["sum_ancient_minutes"])
			}
			return
		}
	}
	t.Errorf("日期过滤后未找到 repo_addr=%s", repoAddr)
}

// ============================================================
// 测试点 4: getRepoDetailV2 返回 reason 字段
// ============================================================

func TestRepoDetailV2_ReturnsReasonFields(t *testing.T) {
	r, cleanup := setupRepoTestRouter(t)
	defer cleanup()

	repoAddr := "test-repo-detail-reason-" + fmt.Sprintf("%d", time.Now().UnixNano())
	commitTime := time.Date(2025, 3, 15, 10, 0, 0, 0, time.UTC)

	_, err := statDB.Exec(`INSERT INTO commits (commit_id, repo_addr, repo_branch, commit_time,
		commit_ancient_minutes, commit_ancient_minutes_reason,
		commit_real_minutes, commit_real_minutes_reason)
		VALUES ($1, $2, 'main', $3, 480, 'ancient原因', 120, 'real原因')`,
		"test-reason-001-"+repoAddr[:20], repoAddr, commitTime)
	if err != nil {
		t.Fatalf("插入 commit 失败: %v", err)
	}
	defer statDB.Exec(`DELETE FROM commits WHERE repo_addr = $1`, repoAddr)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v2/repos/detail?repoAddr="+repoAddr, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("JSON 解析失败: %v", err)
	}

	// 验证 efficiency 子对象
	eff, ok := resp["efficiency"].(map[string]interface{})
	if !ok {
		t.Fatal("响应缺少 efficiency 对象")
	}

	ancientReason, _ := eff["repo_ancient_minutes_reason"].(string)
	if ancientReason == "" {
		t.Error("efficiency 缺少 repo_ancient_minutes_reason")
	} else if ancientReason == "" || !containsSubstring(ancientReason, "ancient原因") {
		t.Errorf("repo_ancient_minutes_reason = %q, 应包含 'ancient原因'", ancientReason)
	}

	realReason, _ := eff["repo_real_minutes_reason"].(string)
	if realReason == "" {
		t.Error("efficiency 缺少 repo_real_minutes_reason")
	} else if !containsSubstring(realReason, "real原因") {
		t.Errorf("repo_real_minutes_reason = %q, 应包含 'real原因'", realReason)
	}

	// 验证 efficiency_ratio 存在
	if _, ok := eff["efficiency_ratio"]; !ok {
		t.Error("efficiency 缺少 efficiency_ratio 字段")
	}
}

// ============================================================
// 测试点 5: getRepoDetailV2 reason 优先级 manual > auto
// ============================================================

func TestRepoDetailV2_ReasonManualPriority(t *testing.T) {
	r, cleanup := setupRepoTestRouter(t)
	defer cleanup()

	repoAddr := "test-repo-detail-priority-" + fmt.Sprintf("%d", time.Now().UnixNano())
	commitTime := time.Date(2025, 3, 15, 10, 0, 0, 0, time.UTC)

	_, err := statDB.Exec(`INSERT INTO commits (commit_id, repo_addr, repo_branch, commit_time,
		commit_ancient_minutes, commit_ancient_minutes_reason, commit_ancient_minutes_reason_manual,
		commit_real_minutes, commit_real_minutes_reason, commit_real_minutes_reason_manual)
		VALUES ($1, $2, 'main', $3, 480, '自动原因', '手动原因', 120, '自动real', '手动real')`,
		"test-prio-001-"+repoAddr[:20], repoAddr, commitTime)
	if err != nil {
		t.Fatalf("插入 commit 失败: %v", err)
	}
	defer statDB.Exec(`DELETE FROM commits WHERE repo_addr = $1`, repoAddr)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v2/repos/detail?repoAddr="+repoAddr, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	eff, _ := resp["efficiency"].(map[string]interface{})
	ancientReason, _ := eff["repo_ancient_minutes_reason"].(string)
	if !containsSubstring(ancientReason, "手动原因") {
		t.Errorf("repo_ancient_minutes_reason = %q, 应包含 '手动原因'（manual 优先）", ancientReason)
	}
	if containsSubstring(ancientReason, "自动原因") {
		t.Errorf("repo_ancient_minutes_reason = %q, 不应包含 '自动原因'（已被 manual 覆盖）", ancientReason)
	}

	realReason, _ := eff["repo_real_minutes_reason"].(string)
	if !containsSubstring(realReason, "手动real") {
		t.Errorf("repo_real_minutes_reason = %q, 应包含 '手动real'（manual 优先）", realReason)
	}
}

// ============================================================
// 辅助函数
// ============================================================

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func init() {
	_ = fmt.Sprintf // 避免 import 未使用错误
}
