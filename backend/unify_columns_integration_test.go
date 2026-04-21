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

// ============================================================
// TP-01: BatchGetStatTasks 空输入返回空 map
// ============================================================

func TestUnify_BatchGetStatTasks_EmptyInput(t *testing.T) {
	tdb := testDB(t)
	defer tdb.Close()

	result, err := BatchGetStatTasks(tdb, []string{})
	if err != nil {
		t.Fatalf("BatchGetStatTasks 空输入返回错误: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("BatchGetStatTasks 空输入返回非空 map, len=%d", len(result))
	}
}

// ============================================================
// TP-02: BatchGetStatTasks 有效 taskIDs 返回正确 map
// ============================================================

func TestUnify_BatchGetStatTasks_ValidIDs(t *testing.T) {
	tdb := testDB(t)
	defer tdb.Close()

	ts := fmt.Sprintf("%d", time.Now().UnixNano())
	taskID1 := "test-batch-unify-001-" + ts
	taskID2 := "test-batch-unify-002-" + ts

	// 插入测试数据
	_, err := tdb.Exec(`INSERT INTO tasks (task_id, cost, upstream_tokens, downstream_tokens, title)
		VALUES ($1, 0.05, 1000, 500, '测试任务A')`, taskID1)
	if err != nil {
		t.Fatalf("插入 task 1 失败: %v", err)
	}
	defer tdb.Exec(`DELETE FROM tasks WHERE task_id = $1`, taskID1)

	_, err = tdb.Exec(`INSERT INTO tasks (task_id, cost, upstream_tokens, downstream_tokens, title)
		VALUES ($1, 0.10, 2000, 800, '测试任务B')`, taskID2)
	if err != nil {
		t.Fatalf("插入 task 2 失败: %v", err)
	}
	defer tdb.Exec(`DELETE FROM tasks WHERE task_id = $1`, taskID2)

	result, err := BatchGetStatTasks(tdb, []string{taskID1, taskID2})
	if err != nil {
		t.Fatalf("BatchGetStatTasks 失败: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("BatchGetStatTasks len=%d, want 2", len(result))
	}

	// 验证 task 1
	t1 := result[taskID1]
	if t1 == nil {
		t.Fatal("result 缺少 taskID1")
	}
	if t1.Cost == nil || *t1.Cost != 0.05 {
		t.Errorf("task1.Cost = %v, want 0.05", t1.Cost)
	}
	if t1.UpstreamTokens == nil || *t1.UpstreamTokens != 1000 {
		t.Errorf("task1.UpstreamTokens = %v, want 1000", t1.UpstreamTokens)
	}
	if t1.DownstreamTokens == nil || *t1.DownstreamTokens != 500 {
		t.Errorf("task1.DownstreamTokens = %v, want 500", t1.DownstreamTokens)
	}
	if t1.Title == nil || *t1.Title != "测试任务A" {
		t.Errorf("task1.Title = %v, want '测试任务A'", t1.Title)
	}

	// 验证 task 2
	t2 := result[taskID2]
	if t2 == nil {
		t.Fatal("result 缺少 taskID2")
	}
	if t2.Cost == nil || *t2.Cost != 0.10 {
		t.Errorf("task2.Cost = %v, want 0.10", t2.Cost)
	}
}

// ============================================================
// TP-03: BatchGetStatTasks 不存在的 taskIDs 返回空 map
// ============================================================

func TestUnify_BatchGetStatTasks_NonExistentIDs(t *testing.T) {
	tdb := testDB(t)
	defer tdb.Close()

	result, err := BatchGetStatTasks(tdb, []string{"nonexistent-xyz-001", "nonexistent-xyz-002"})
	if err != nil {
		t.Fatalf("BatchGetStatTasks 不存在 IDs 返回错误: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("BatchGetStatTasks 不存在 IDs 返回非空 map, len=%d", len(result))
	}
}

// ============================================================
// TP-04: listCommitsV2 响应包含 cost/upstream_tokens/downstream_tokens 字段
// ============================================================

func setupCommitTestRouter(t *testing.T) (*gin.Engine, func()) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	tdb := testDB(t)

	statDB = tdb
	appConfig.TaskRealMinutes.GapThresholdMinutes = 30
	appConfig.TaskRealMinutes.ExtensionMinutes = 5

	r := gin.New()
	v2 := r.Group("/api/v2")
	v2.GET("/commits", listCommitsV2)

	cleanup := func() { tdb.Close() }
	return r, cleanup
}

func TestUnify_ListCommitsV2_CostTokensFields(t *testing.T) {
	r, cleanup := setupCommitTestRouter(t)
	defer cleanup()

	ts := fmt.Sprintf("%d", time.Now().UnixNano())
	taskID := "test-commit-tokens-t1-" + ts
	commitID := "test-commit-tokens-c1-" + ts
	repoAddr := "test-commit-tokens-repo-" + ts
	commitTime := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

	// 插入 task
	_, err := statDB.Exec(`INSERT INTO tasks (task_id, cost, upstream_tokens, downstream_tokens)
		VALUES ($1, 0.08, 5000, 2000)`, taskID)
	if err != nil {
		t.Fatalf("插入 task 失败: %v", err)
	}
	defer statDB.Exec(`DELETE FROM tasks WHERE task_id = $1`, taskID)

	// 插入 commit
	taskIDsJSON := fmt.Sprintf(`["%s"]`, taskID)
	silicaJSON := `[0.6]`
	_, err = statDB.Exec(`INSERT INTO commits (commit_id, repo_addr, repo_branch, commit_time, task_ids, task_ids_silica)
		VALUES ($1, $2, 'main', $3, $4::jsonb, $5::jsonb)`,
		commitID, repoAddr, commitTime, taskIDsJSON, silicaJSON)
	if err != nil {
		t.Fatalf("插入 commit 失败: %v", err)
	}
	defer statDB.Exec(`DELETE FROM commits WHERE commit_id = $1`, commitID)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v2/commits?startDate=20250615&endDate=20250615", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("JSON 解析失败: %v", err)
	}

	data, ok := resp["data"].([]interface{})
	if !ok || len(data) == 0 {
		t.Fatal("响应 data 数组为空或类型错误")
	}

	// 找到我们插入的 commit
	var found bool
	for _, item := range data {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if m["commit_id"] == commitID {
			found = true
			// cost 直接累加（不乘 silica）
			cost, _ := m["cost"].(float64)
			if cost != 0.08 {
				t.Errorf("cost = %v, want 0.08", cost)
			}
			// upstream_tokens = round(5000 * 0.6) = 3000
			upTokens, _ := m["upstream_tokens"].(float64) // JSON numbers are float64
			if int64(upTokens) != 3000 {
				t.Errorf("upstream_tokens = %v, want 3000", upTokens)
			}
			// downstream_tokens = round(2000 * 0.6) = 1200
			downTokens, _ := m["downstream_tokens"].(float64)
			if int64(downTokens) != 1200 {
				t.Errorf("downstream_tokens = %v, want 1200", downTokens)
			}
			break
		}
	}
	if !found {
		t.Errorf("未在响应中找到 commit_id=%s", commitID)
	}
}

// ============================================================
// TP-05: listCommitsV2 无关联 task 时 cost/tokens 为零值
// ============================================================

func TestUnify_ListCommitsV2_NoTask_ZeroValues(t *testing.T) {
	r, cleanup := setupCommitTestRouter(t)
	defer cleanup()

	ts := fmt.Sprintf("%d", time.Now().UnixNano())
	commitID := "test-commit-notask-c1-" + ts
	repoAddr := "test-commit-notask-repo-" + ts
	commitTime := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

	// 插入无 task_ids 的 commit
	_, err := statDB.Exec(`INSERT INTO commits (commit_id, repo_addr, repo_branch, commit_time, task_ids)
		VALUES ($1, $2, 'main', $3, '[]'::jsonb)`,
		commitID, repoAddr, commitTime)
	if err != nil {
		t.Fatalf("插入 commit 失败: %v", err)
	}
	defer statDB.Exec(`DELETE FROM commits WHERE commit_id = $1`, commitID)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v2/commits?startDate=20250615&endDate=20250615", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data, _ := resp["data"].([]interface{})

	for _, item := range data {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if m["commit_id"] == commitID {
			cost, _ := m["cost"].(float64)
			if cost != 0 {
				t.Errorf("cost = %v, want 0", cost)
			}
			upTokens, _ := m["upstream_tokens"].(float64)
			if int64(upTokens) != 0 {
				t.Errorf("upstream_tokens = %v, want 0", upTokens)
			}
			downTokens, _ := m["downstream_tokens"].(float64)
			if int64(downTokens) != 0 {
				t.Errorf("downstream_tokens = %v, want 0", downTokens)
			}
			return
		}
	}
	t.Errorf("未在响应中找到 commit_id=%s", commitID)
}

// ============================================================
// TP-06: listTasksV2 响应包含 title 字段
// ============================================================

func setupTaskListTestRouter(t *testing.T) (*gin.Engine, func()) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	tdb := testDB(t)

	statDB = tdb
	appConfig.TaskRealMinutes.GapThresholdMinutes = 30
	appConfig.TaskRealMinutes.ExtensionMinutes = 5

	r := gin.New()
	v2 := r.Group("/api/v2")
	v2.GET("/tasks", listTasksV2)

	cleanup := func() { tdb.Close() }
	return r, cleanup
}

func TestUnify_ListTasksV2_TitleField(t *testing.T) {
	r, cleanup := setupTaskListTestRouter(t)
	defer cleanup()

	ts := fmt.Sprintf("%d", time.Now().UnixNano())
	taskID := "test-tasklist-title-" + ts
	startTime := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

	_, err := statDB.Exec(`INSERT INTO tasks (task_id, title, start_time, cost, upstream_tokens, downstream_tokens)
		VALUES ($1, '统一列结构测试', $2, 0.05, 1000, 500)`, taskID, startTime)
	if err != nil {
		t.Fatalf("插入 task 失败: %v", err)
	}
	defer statDB.Exec(`DELETE FROM tasks WHERE task_id = $1`, taskID)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v2/tasks?startDate=20250615&endDate=20250615", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data, _ := resp["data"].([]interface{})

	var found bool
	for _, item := range data {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if m["task_id"] == taskID {
			found = true
			title, _ := m["title"].(string)
			if title != "统一列结构测试" {
				t.Errorf("title = %q, want '统一列结构测试'", title)
			}
			// cost 字段存在
			if _, ok := m["cost"]; !ok {
				t.Error("响应缺少 cost 字段")
			}
			// upstream_tokens 字段存在
			if _, ok := m["upstream_tokens"]; !ok {
				t.Error("响应缺少 upstream_tokens 字段")
			}
			// downstream_tokens 字段存在
			if _, ok := m["downstream_tokens"]; !ok {
				t.Error("响应缺少 downstream_tokens 字段")
			}
			break
		}
	}
	if !found {
		t.Errorf("未在响应中找到 task_id=%s", taskID)
	}
}

// ============================================================
// TP-07: getRepoDetailV2 commits 数组包含 cost/tokens 字段
// ============================================================

func TestUnify_RepoDetailV2_CommitCostTokens(t *testing.T) {
	r, cleanup := setupRepoTestRouter(t)
	defer cleanup()

	ts := fmt.Sprintf("%d", time.Now().UnixNano())
	taskID := "test-repo-detail-t1-" + ts
	commitID := "test-repo-detail-c1-" + ts
	repoAddr := "test-repo-detail-cost-" + ts
	commitTime := time.Date(2025, 3, 15, 10, 0, 0, 0, time.UTC)

	// 插入 task
	_, err := statDB.Exec(`INSERT INTO tasks (task_id, cost, upstream_tokens, downstream_tokens)
		VALUES ($1, 0.15, 8000, 3000)`, taskID)
	if err != nil {
		t.Fatalf("插入 task 失败: %v", err)
	}
	defer statDB.Exec(`DELETE FROM tasks WHERE task_id = $1`, taskID)

	// 插入 commit（silica=1.0）
	taskIDsJSON := fmt.Sprintf(`["%s"]`, taskID)
	silicaJSON := `[1.0]`
	_, err = statDB.Exec(`INSERT INTO commits (commit_id, repo_addr, repo_branch, commit_time, task_ids, task_ids_silica)
		VALUES ($1, $2, 'main', $3, $4::jsonb, $5::jsonb)`,
		commitID, repoAddr, commitTime, taskIDsJSON, silicaJSON)
	if err != nil {
		t.Fatalf("插入 commit 失败: %v", err)
	}
	defer statDB.Exec(`DELETE FROM commits WHERE commit_id = $1`, commitID)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v2/repos/detail?repoAddr="+repoAddr, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	commits, ok := resp["commits"].([]interface{})
	if !ok || len(commits) == 0 {
		t.Fatal("响应 commits 数组为空")
	}

	cm, ok := commits[0].(map[string]interface{})
	if !ok {
		t.Fatal("commit 不是 object 类型")
	}

	cost, _ := cm["cost"].(float64)
	if cost != 0.15 {
		t.Errorf("cost = %v, want 0.15", cost)
	}
	upTokens, _ := cm["upstream_tokens"].(float64)
	if int64(upTokens) != 8000 {
		t.Errorf("upstream_tokens = %v, want 8000", upTokens)
	}
	downTokens, _ := cm["downstream_tokens"].(float64)
	if int64(downTokens) != 3000 {
		t.Errorf("downstream_tokens = %v, want 3000", downTokens)
	}
}

// ============================================================
// TP-08: getRepoDetailV2 多 task 聚合 cost/tokens 正确
// ============================================================

func TestUnify_RepoDetailV2_MultiTaskAggregation(t *testing.T) {
	r, cleanup := setupRepoTestRouter(t)
	defer cleanup()

	ts := fmt.Sprintf("%d", time.Now().UnixNano())
	taskA := "test-repo-multi-tA-" + ts
	taskB := "test-repo-multi-tB-" + ts
	commitID := "test-repo-multi-c1-" + ts
	repoAddr := "test-repo-multi-agg-" + ts
	commitTime := time.Date(2025, 3, 15, 10, 0, 0, 0, time.UTC)

	// 插入 task-A: cost=0.10, upstream=10000, downstream=4000
	_, err := statDB.Exec(`INSERT INTO tasks (task_id, cost, upstream_tokens, downstream_tokens)
		VALUES ($1, 0.10, 10000, 4000)`, taskA)
	if err != nil {
		t.Fatalf("插入 task A 失败: %v", err)
	}
	defer statDB.Exec(`DELETE FROM tasks WHERE task_id = $1`, taskA)

	// 插入 task-B: cost=0.20, upstream=6000, downstream=2000
	_, err = statDB.Exec(`INSERT INTO tasks (task_id, cost, upstream_tokens, downstream_tokens)
		VALUES ($1, 0.20, 6000, 2000)`, taskB)
	if err != nil {
		t.Fatalf("插入 task B 失败: %v", err)
	}
	defer statDB.Exec(`DELETE FROM tasks WHERE task_id = $1`, taskB)

	// 插入 commit: task_ids=[taskA, taskB], silica=[0.5, 0.8]
	taskIDsJSON := fmt.Sprintf(`["%s","%s"]`, taskA, taskB)
	silicaJSON := `[0.5,0.8]`
	_, err = statDB.Exec(`INSERT INTO commits (commit_id, repo_addr, repo_branch, commit_time, task_ids, task_ids_silica)
		VALUES ($1, $2, 'main', $3, $4::jsonb, $5::jsonb)`,
		commitID, repoAddr, commitTime, taskIDsJSON, silicaJSON)
	if err != nil {
		t.Fatalf("插入 commit 失败: %v", err)
	}
	defer statDB.Exec(`DELETE FROM commits WHERE commit_id = $1`, commitID)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v2/repos/detail?repoAddr="+repoAddr, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	commits, ok := resp["commits"].([]interface{})
	if !ok || len(commits) == 0 {
		t.Fatal("响应 commits 数组为空")
	}

	cm, ok := commits[0].(map[string]interface{})
	if !ok {
		t.Fatal("commit 不是 object 类型")
	}

	// cost 直接累加 = 0.10 + 0.20 = 0.30
	cost, _ := cm["cost"].(float64)
	// 使用容差比较浮点数
	if cost < 0.29 || cost > 0.31 {
		t.Errorf("cost = %v, want 0.30", cost)
	}

	// upstream_tokens = round(10000*0.5) + round(6000*0.8) = 5000 + 4800 = 9800
	upTokens, _ := cm["upstream_tokens"].(float64)
	if int64(upTokens) != 9800 {
		t.Errorf("upstream_tokens = %v, want 9800", upTokens)
	}

	// downstream_tokens = round(4000*0.5) + round(2000*0.8) = 2000 + 1600 = 3600
	downTokens, _ := cm["downstream_tokens"].(float64)
	if int64(downTokens) != 3600 {
		t.Errorf("downstream_tokens = %v, want 3600", downTokens)
	}
}
