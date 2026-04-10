//go:build integration

package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

// setupUserProductivityTestRouter 创建测试用 gin 路由，注册 user-productivity 和 user-groups 端点
func setupUserProductivityTestRouter(t *testing.T) (*gin.Engine, *sql.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	tdb := testDB(t)

	// 设置全局变量供 handler 使用
	statDB = tdb

	r := gin.New()
	v2 := r.Group("/api/v2")
	v2.POST("/user-productivity/rebuild", rebuildUserProductivity)
	v2.GET("/users", listUsersV2)
	v2.GET("/users/:userId", getUserDetailV2)
	v2.POST("/user-groups", createUserGroupHandler)
	v2.GET("/user-groups", listUserGroupsHandler)
	v2.DELETE("/user-groups/:groupId", deleteUserGroupHandler)
	v2.GET("/user-groups/:groupId", getUserGroupDetailHandler)

	return r, tdb
}

// ============================================================
// UP-DB-01: user_productivity 表结构和索引验证
// ============================================================

func TestUserProductivityDB_TableAndIndexes(t *testing.T) {
	tdb := testDB(t)
	defer tdb.Close()

	// 验证列结构
	expectedColumns := map[string]string{
		"user_productivity_id":        "character varying",
		"create_time":                 "timestamp with time zone",
		"user_id":                     "character varying",
		"user_name":                   "character varying",
		"task_ids":                    "jsonb",
		"work_dir_ids":                "jsonb",
		"task_diff_lines":             "integer",
		"upstream_tokens":             "bigint",
		"downstream_tokens":           "bigint",
		"cost":                        "double precision",
		"task_real_minutes":           "double precision",
		"task_ancient_minutes":        "double precision",
		"task_efficiency_ratio":       "double precision",
		"commit_ids":                  "jsonb",
		"commit_diff_lines":           "integer",
		"commit_ancient_minutes":      "double precision",
		"commit_real_ai_minutes":      "double precision",
		"commit_real_ancient_minutes": "double precision",
		"commit_real_minutes":         "double precision",
		"commit_efficiency_ratio":     "double precision",
		"created_at":                  "timestamp with time zone",
		"updated_at":                  "timestamp with time zone",
	}

	for col, expectedType := range expectedColumns {
		var dataType string
		err := tdb.QueryRow(`
			SELECT data_type FROM information_schema.columns
			WHERE table_name = 'user_productivity' AND column_name = $1`, col).Scan(&dataType)
		if err == sql.ErrNoRows {
			t.Errorf("user_productivity 表缺少列: %s", col)
			continue
		}
		if err != nil {
			t.Errorf("检查列 %s 失败: %v", col, err)
			continue
		}
		if dataType != expectedType {
			t.Errorf("列 %s 类型 = %s, want %s", col, dataType, expectedType)
		}
	}

	// 验证索引
	indexes := []string{"idx_user_productivity_user_id", "idx_user_productivity_create_time"}
	for _, idx := range indexes {
		var exists bool
		err := tdb.QueryRow(`
			SELECT EXISTS(
				SELECT 1 FROM pg_indexes
				WHERE tablename = 'user_productivity' AND indexname = $1
			)`, idx).Scan(&exists)
		if err != nil {
			t.Errorf("检查索引 %s 失败: %v", idx, err)
			continue
		}
		if !exists {
			t.Errorf("user_productivity 表缺少索引: %s", idx)
		}
	}
}

// ============================================================
// UP-DB-02: user_groups 表结构和索引验证
// ============================================================

func TestUserGroupDB_TableAndIndexes(t *testing.T) {
	tdb := testDB(t)
	defer tdb.Close()

	// 验证列结构
	expectedColumns := map[string]string{
		"group_id":   "uuid",
		"name":       "character varying",
		"user_ids":   "jsonb",
		"created_at": "timestamp with time zone",
		"updated_at": "timestamp with time zone",
	}

	for col, expectedType := range expectedColumns {
		var dataType string
		err := tdb.QueryRow(`
			SELECT data_type FROM information_schema.columns
			WHERE table_name = 'user_groups' AND column_name = $1`, col).Scan(&dataType)
		if err == sql.ErrNoRows {
			t.Errorf("user_groups 表缺少列: %s", col)
			continue
		}
		if err != nil {
			t.Errorf("检查列 %s 失败: %v", col, err)
			continue
		}
		if dataType != expectedType {
			t.Errorf("列 %s 类型 = %s, want %s", col, dataType, expectedType)
		}
	}

	// 验证默认值：插入只有 name 的记录
	var groupID string
	err := tdb.QueryRow(`INSERT INTO user_groups (name) VALUES ('test-default') RETURNING group_id`).Scan(&groupID)
	if err != nil {
		t.Fatalf("插入 user_groups 失败: %v", err)
	}
	defer tdb.Exec(`DELETE FROM user_groups WHERE group_id = $1`, groupID)

	var userIDs string
	err = tdb.QueryRow(`SELECT user_ids::text FROM user_groups WHERE group_id = $1`, groupID).Scan(&userIDs)
	if err != nil {
		t.Fatalf("查询默认值失败: %v", err)
	}
	if userIDs != "[]" {
		t.Errorf("user_ids 默认值 = %s, want []", userIDs)
	}

	// 验证索引
	var exists bool
	err = tdb.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM pg_indexes
			WHERE tablename = 'user_groups' AND indexname = 'idx_user_groups_name'
		)`).Scan(&exists)
	if err != nil {
		t.Fatalf("检查索引失败: %v", err)
	}
	if !exists {
		t.Error("user_groups 表缺少索引: idx_user_groups_name")
	}
}

// ============================================================
// UP-API-01: POST /api/v2/user-productivity/rebuild 正常场景
// 验证从 tasks/commits 聚合 + 效率比计算 + upsert
// ============================================================

func TestRebuildUserProductivity_Normal(t *testing.T) {
	r, tdb := setupUserProductivityTestRouter(t)
	defer tdb.Close()

	ts := fmt.Sprintf("%d", time.Now().UnixNano())
	userID := "test-user-rebuild-" + ts
	testDate := time.Date(2099, 1, 15, 10, 0, 0, 0, time.UTC)

	// 插入两条 tasks 测试数据（同一天、同一用户）
	taskID1 := "test-task-rebuild-1-" + ts
	taskID2 := "test-task-rebuild-2-" + ts
	_, err := tdb.Exec(`INSERT INTO tasks (task_id, user_id, user_name, start_time, diff_lines,
		upstream_tokens, downstream_tokens, cost, task_real_minutes, task_ancient_minutes, work_dir_id)
		VALUES ($1, $2, 'TestUser', $3, 100, 5000, 3000, 0.5, 30, 120, 'wd-1')`,
		taskID1, userID, testDate)
	if err != nil {
		t.Fatalf("插入 task1 失败: %v", err)
	}
	defer tdb.Exec(`DELETE FROM tasks WHERE task_id = $1`, taskID1)

	_, err = tdb.Exec(`INSERT INTO tasks (task_id, user_id, user_name, start_time, diff_lines,
		upstream_tokens, downstream_tokens, cost, task_real_minutes, task_ancient_minutes, work_dir_id)
		VALUES ($1, $2, 'TestUser', $3, 50, 2000, 1500, 0.3, 20, 80, 'wd-2')`,
		taskID2, userID, testDate.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("插入 task2 失败: %v", err)
	}
	defer tdb.Exec(`DELETE FROM tasks WHERE task_id = $1`, taskID2)

	// 插入一条 commit 测试数据（同一天、同一用户）
	commitID := "test-commit-rebuild-" + ts
	_, err = tdb.Exec(`INSERT INTO commits (commit_id, user_id, commit_time, diff_lines,
		commit_ancient_minutes, commit_real_ai_minutes, commit_real_ancient_minutes, commit_real_minutes,
		repo_addr, repo_branch)
		VALUES ($1, $2, $3, 200, 300, 10, 15, 60, 'test-repo', 'main')`,
		commitID, userID, testDate.Add(3*time.Hour))
	if err != nil {
		t.Fatalf("插入 commit 失败: %v", err)
	}
	defer tdb.Exec(`DELETE FROM commits WHERE commit_id = $1`, commitID)

	// 清理可能残留的 user_productivity 数据
	defer tdb.Exec(`DELETE FROM user_productivity WHERE user_id = $1`, userID)

	// 调用 rebuild API
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v2/user-productivity/rebuild?startDate=20990115&endDate=20990115", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "ok" {
		t.Errorf("status = %v, want ok", resp["status"])
	}
	count, ok := resp["count"].(float64)
	if !ok || count < 1 {
		t.Errorf("count = %v, want >= 1", resp["count"])
	}

	// 验证数据库中的聚合结果
	expectedID := userID + "_20990115"
	var taskDiffLines int
	var upTokens, downTokens int64
	var costVal, taskRealMin, taskAncientMin float64
	var taskEffRatio, commitEffRatio sql.NullFloat64
	var commitDiffLines int
	var commitRealMin, commitAncientMin float64

	err = tdb.QueryRow(`SELECT task_diff_lines, upstream_tokens, downstream_tokens, cost,
		task_real_minutes, task_ancient_minutes, task_efficiency_ratio,
		commit_diff_lines, commit_real_minutes, commit_ancient_minutes, commit_efficiency_ratio
		FROM user_productivity WHERE user_productivity_id = $1`, expectedID).
		Scan(&taskDiffLines, &upTokens, &downTokens, &costVal,
			&taskRealMin, &taskAncientMin, &taskEffRatio,
			&commitDiffLines, &commitRealMin, &commitAncientMin, &commitEffRatio)
	if err != nil {
		t.Fatalf("查询 user_productivity 聚合结果失败: %v", err)
	}

	// 验证 task 聚合: 100+50=150 diff_lines, 5000+2000=7000 upstream, 3000+1500=4500 downstream
	if taskDiffLines != 150 {
		t.Errorf("task_diff_lines = %d, want 150", taskDiffLines)
	}
	if upTokens != 7000 {
		t.Errorf("upstream_tokens = %d, want 7000", upTokens)
	}
	if downTokens != 4500 {
		t.Errorf("downstream_tokens = %d, want 4500", downTokens)
	}
	// cost: 0.5+0.3=0.8
	if costVal < 0.79 || costVal > 0.81 {
		t.Errorf("cost = %f, want ~0.8", costVal)
	}
	// task_real_minutes: 30+20=50, task_ancient_minutes: 120+80=200
	if taskRealMin != 50 {
		t.Errorf("task_real_minutes = %f, want 50", taskRealMin)
	}
	if taskAncientMin != 200 {
		t.Errorf("task_ancient_minutes = %f, want 200", taskAncientMin)
	}
	// task_efficiency_ratio: round(200/50 * 100) = 400
	if taskEffRatio.Valid && taskEffRatio.Float64 != 400 {
		t.Errorf("task_efficiency_ratio = %v, want 400", taskEffRatio.Float64)
	}

	// 验证 commit 聚合
	if commitDiffLines != 200 {
		t.Errorf("commit_diff_lines = %d, want 200", commitDiffLines)
	}
	if commitRealMin != 60 {
		t.Errorf("commit_real_minutes = %f, want 60", commitRealMin)
	}
	if commitAncientMin != 300 {
		t.Errorf("commit_ancient_minutes = %f, want 300", commitAncientMin)
	}
	// commit_efficiency_ratio: round(300/60 * 100) = 500
	if commitEffRatio.Valid && commitEffRatio.Float64 != 500 {
		t.Errorf("commit_efficiency_ratio = %v, want 500", commitEffRatio.Float64)
	}
}

// ============================================================
// UP-API-02: POST /api/v2/user-productivity/rebuild 缺少参数返回 400
// ============================================================

func TestRebuildUserProductivity_MissingParams(t *testing.T) {
	r, tdb := setupUserProductivityTestRouter(t)
	defer tdb.Close()

	// 缺少两个参数
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v2/user-productivity/rebuild", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("无参数时 HTTP status = %d, want 400", w.Code)
	}

	// 只有 startDate
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v2/user-productivity/rebuild?startDate=20990101", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("只有 startDate 时 HTTP status = %d, want 400", w.Code)
	}

	// 日期格式错误
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v2/user-productivity/rebuild?startDate=bad&endDate=20990101", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("日期格式错误时 HTTP status = %d, want 400", w.Code)
	}
}

// ============================================================
// UP-API-03: GET /api/v2/user-productivity 列表汇总 + 分页 + 日期过滤
// ============================================================

func TestListUserProductivitySummary_Normal(t *testing.T) {
	r, tdb := setupUserProductivityTestRouter(t)
	defer tdb.Close()

	ts := fmt.Sprintf("%d", time.Now().UnixNano())
	userID := "test-user-list-" + ts
	testDate := time.Date(2099, 3, 10, 0, 0, 0, 0, time.UTC)

	// 插入两天的 user_productivity 数据
	upID1 := userID + "_20990310"
	upID2 := userID + "_20990311"
	_, err := tdb.Exec(`INSERT INTO user_productivity
		(user_productivity_id, create_time, user_id, user_name,
		 task_ids, task_diff_lines, upstream_tokens, downstream_tokens, cost,
		 task_real_minutes, task_ancient_minutes, commit_ids, commit_diff_lines,
		 commit_ancient_minutes, commit_real_minutes)
		VALUES ($1, $2, $3, 'ListUser', '["t1","t2"]', 100, 5000, 3000, 0.5, 30, 120, '["c1"]', 200, 300, 60)`,
		upID1, testDate, userID)
	if err != nil {
		t.Fatalf("插入 user_productivity 1 失败: %v", err)
	}
	defer tdb.Exec(`DELETE FROM user_productivity WHERE user_productivity_id = $1`, upID1)

	_, err = tdb.Exec(`INSERT INTO user_productivity
		(user_productivity_id, create_time, user_id, user_name,
		 task_ids, task_diff_lines, upstream_tokens, downstream_tokens, cost,
		 task_real_minutes, task_ancient_minutes, commit_ids, commit_diff_lines,
		 commit_ancient_minutes, commit_real_minutes)
		VALUES ($1, $2, $3, 'ListUser', '["t3"]', 50, 2000, 1500, 0.3, 20, 80, '["c2"]', 100, 150, 30)`,
		upID2, testDate.AddDate(0, 0, 1), userID)
	if err != nil {
		t.Fatalf("插入 user_productivity 2 失败: %v", err)
	}
	defer tdb.Exec(`DELETE FROM user_productivity WHERE user_productivity_id = $1`, upID2)

	// 不带日期参数查询
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v2/users?page=1&pageSize=100", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	// 验证分页字段存在
	if _, ok := resp["total"]; !ok {
		t.Error("响应缺少 total 字段")
	}
	if _, ok := resp["page"]; !ok {
		t.Error("响应缺少 page 字段")
	}
	if _, ok := resp["pageSize"]; !ok {
		t.Error("响应缺少 pageSize 字段")
	}

	data, ok := resp["data"].([]interface{})
	if !ok {
		t.Fatal("响应缺少 data 数组")
	}

	// 查找我们的测试用户
	var found bool
	for _, item := range data {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if m["user_id"] == userID {
			found = true
			// 验证汇总: day_count=2, task_count=3 (2+1), commit_count=2 (1+1)
			if m["day_count"] != float64(2) {
				t.Errorf("day_count = %v, want 2", m["day_count"])
			}
			if m["task_count"] != float64(3) {
				t.Errorf("task_count = %v, want 3", m["task_count"])
			}
			if m["commit_count"] != float64(2) {
				t.Errorf("commit_count = %v, want 2", m["commit_count"])
			}
			// task_diff_lines: 100+50=150
			if m["task_diff_lines"] != float64(150) {
				t.Errorf("task_diff_lines = %v, want 150", m["task_diff_lines"])
			}
			// task_efficiency_ratio: round((120+80)/(30+20)*100) = 400
			if m["task_efficiency_ratio"] != float64(400) {
				t.Errorf("task_efficiency_ratio = %v, want 400", m["task_efficiency_ratio"])
			}
			break
		}
	}
	if !found {
		t.Errorf("列表中未找到 user_id=%s", userID)
	}

	// 带日期过滤查询：只查 20990310 一天
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v2/users?startDate=20990310&endDate=20990310", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("带日期过滤 HTTP status = %d, want 200", w.Code)
	}

	json.Unmarshal(w.Body.Bytes(), &resp)
	data = resp["data"].([]interface{})
	for _, item := range data {
		m := item.(map[string]interface{})
		if m["user_id"] == userID {
			// 只有一天数据
			if m["day_count"] != float64(1) {
				t.Errorf("日期过滤后 day_count = %v, want 1", m["day_count"])
			}
			if m["task_count"] != float64(2) {
				t.Errorf("日期过滤后 task_count = %v, want 2", m["task_count"])
			}
			break
		}
	}
}

// ============================================================
// UP-API-04: GET /api/v2/user-productivity/:userId 详情 + 400
// ============================================================

func TestGetUserProductivityDetail_Normal(t *testing.T) {
	r, tdb := setupUserProductivityTestRouter(t)
	defer tdb.Close()

	ts := fmt.Sprintf("%d", time.Now().UnixNano())
	userID := "test-user-detail-" + ts
	testDate := time.Date(2099, 5, 20, 0, 0, 0, 0, time.UTC)

	// 插入测试数据
	upID := userID + "_20990520"
	_, err := tdb.Exec(`INSERT INTO user_productivity
		(user_productivity_id, create_time, user_id, user_name,
		 task_ids, task_diff_lines, upstream_tokens, downstream_tokens, cost,
		 task_real_minutes, task_ancient_minutes, commit_ids, commit_diff_lines,
		 commit_ancient_minutes, commit_real_minutes)
		VALUES ($1, $2, $3, 'DetailUser', '["t1"]', 100, 5000, 3000, 0.5, 30, 120, '["c1"]', 200, 300, 60)`,
		upID, testDate, userID)
	if err != nil {
		t.Fatalf("插入 user_productivity 失败: %v", err)
	}
	defer tdb.Exec(`DELETE FROM user_productivity WHERE user_productivity_id = $1`, upID)

	// 查询详情
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v2/users/"+userID, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	// 验证返回结构
	if _, ok := resp["summary"]; !ok {
		t.Error("响应缺少 summary 字段")
	}
	if _, ok := resp["daily"]; !ok {
		t.Error("响应缺少 daily 字段")
	}
	if _, ok := resp["total"]; !ok {
		t.Error("响应缺少 total 字段")
	}

	// 验证 summary 汇总
	summary, ok := resp["summary"].(map[string]interface{})
	if !ok {
		t.Fatal("summary 不是对象")
	}
	if summary["day_count"] != float64(1) {
		t.Errorf("summary.day_count = %v, want 1", summary["day_count"])
	}
	if summary["task_diff_lines"] != float64(100) {
		t.Errorf("summary.task_diff_lines = %v, want 100", summary["task_diff_lines"])
	}
	// task_efficiency_ratio: round(120/30*100) = 400
	if summary["task_efficiency_ratio"] != float64(400) {
		t.Errorf("summary.task_efficiency_ratio = %v, want 400", summary["task_efficiency_ratio"])
	}
	// commit_efficiency_ratio: round(300/60*100) = 500
	if summary["commit_efficiency_ratio"] != float64(500) {
		t.Errorf("summary.commit_efficiency_ratio = %v, want 500", summary["commit_efficiency_ratio"])
	}

	// 验证 daily 数组
	daily, ok := resp["daily"].([]interface{})
	if !ok {
		t.Fatal("daily 不是数组")
	}
	if len(daily) != 1 {
		t.Errorf("daily 长度 = %d, want 1", len(daily))
	}
}

func TestGetUserProductivityDetail_EmptyUserId(t *testing.T) {
	r, tdb := setupUserProductivityTestRouter(t)
	defer tdb.Close()

	// Gin 路由中 :userId 不可能为空字符串（会变成 404），
	// 但查不存在的用户应返回空 daily
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v2/users/nonexistent-user-xyz", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200 (空结果), body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["total"] != float64(0) {
		t.Errorf("total = %v, want 0", resp["total"])
	}
}

// ============================================================
// UP-API-05: POST /api/v2/user-groups 创建 + 参数校验
// ============================================================

func TestCreateUserGroup_Normal(t *testing.T) {
	r, tdb := setupUserProductivityTestRouter(t)
	defer tdb.Close()

	ts := fmt.Sprintf("%d", time.Now().UnixNano())
	groupName := "test-group-" + ts

	body := fmt.Sprintf(`{"name":"%s","user_ids":["user-a","user-b"]}`, groupName)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v2/user-groups", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	groupID, ok := resp["group_id"].(string)
	if !ok || groupID == "" {
		t.Fatal("响应缺少有效的 group_id")
	}
	defer tdb.Exec(`DELETE FROM user_groups WHERE group_id = $1`, groupID)

	// UUID 格式验证 (36 字符含连字符)
	if len(groupID) != 36 {
		t.Errorf("group_id 长度 = %d, want 36 (UUID 格式)", len(groupID))
	}

	if resp["name"] != groupName {
		t.Errorf("name = %v, want %s", resp["name"], groupName)
	}

	// 验证 user_ids
	userIDsRaw, ok := resp["user_ids"]
	if !ok {
		t.Fatal("响应缺少 user_ids")
	}
	userIDsJSON, _ := json.Marshal(userIDsRaw)
	var userIDs []string
	json.Unmarshal(userIDsJSON, &userIDs)
	if len(userIDs) != 2 {
		t.Errorf("user_ids 长度 = %d, want 2", len(userIDs))
	}
}

func TestCreateUserGroup_MissingParams(t *testing.T) {
	r, tdb := setupUserProductivityTestRouter(t)
	defer tdb.Close()

	// 缺少 name
	body := `{"name":"","user_ids":["user-a"]}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v2/user-groups", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("缺少 name 时 HTTP status = %d, want 400", w.Code)
	}

	// 缺少 user_ids
	body = `{"name":"test-group"}`
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v2/user-groups", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("缺少 user_ids 时 HTTP status = %d, want 400", w.Code)
	}

	// 空 user_ids 数组
	body = `{"name":"test-group","user_ids":[]}`
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v2/user-groups", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("空 user_ids 时 HTTP status = %d, want 400", w.Code)
	}
}

// ============================================================
// UP-API-06: GET /api/v2/user-groups 列表返回
// ============================================================

func TestListUserGroups_Normal(t *testing.T) {
	r, tdb := setupUserProductivityTestRouter(t)
	defer tdb.Close()

	ts := fmt.Sprintf("%d", time.Now().UnixNano())

	// 创建测试组
	group, err := CreateUserGroup(tdb, "test-list-group-"+ts, "", []string{"u1", "u2"})
	if err != nil {
		t.Fatalf("创建测试组失败: %v", err)
	}
	defer tdb.Exec(`DELETE FROM user_groups WHERE group_id = $1`, group.GroupID)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v2/user-groups", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	data, ok := resp["data"].([]interface{})
	if !ok {
		t.Fatal("响应缺少 data 数组")
	}

	// 查找测试组
	var found bool
	for _, item := range data {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if m["group_id"] == group.GroupID {
			found = true
			if m["name"] != "test-list-group-"+ts {
				t.Errorf("name = %v, want test-list-group-%s", m["name"], ts)
			}
			break
		}
	}
	if !found {
		t.Errorf("列表中未找到 group_id=%s", group.GroupID)
	}
}

// ============================================================
// UP-API-07: DELETE /api/v2/user-groups/:groupId 删除 + 404
// ============================================================

func TestDeleteUserGroup_Normal(t *testing.T) {
	r, tdb := setupUserProductivityTestRouter(t)
	defer tdb.Close()

	ts := fmt.Sprintf("%d", time.Now().UnixNano())
	group, err := CreateUserGroup(tdb, "test-delete-group-"+ts, "", []string{"u1"})
	if err != nil {
		t.Fatalf("创建测试组失败: %v", err)
	}
	// 不需要 defer 删除，测试本身会删除

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v2/user-groups/"+group.GroupID, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "ok" {
		t.Errorf("status = %v, want ok", resp["status"])
	}

	// 验证确实已删除
	deleted, _ := GetUserGroup(tdb, group.GroupID)
	if deleted != nil {
		t.Error("删除后 GetUserGroup 应返回 nil")
	}
}

func TestDeleteUserGroup_NotFound(t *testing.T) {
	r, tdb := setupUserProductivityTestRouter(t)
	defer tdb.Close()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v2/user-groups/00000000-0000-0000-0000-000000000000", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("HTTP status = %d, want 404, body: %s", w.Code, w.Body.String())
	}
}

// ============================================================
// UP-API-08: GET /api/v2/user-groups/:groupId 详情 + 组内汇总 + 404
// ============================================================

func TestGetUserGroupDetail_Normal(t *testing.T) {
	r, tdb := setupUserProductivityTestRouter(t)
	defer tdb.Close()

	ts := fmt.Sprintf("%d", time.Now().UnixNano())
	userA := "test-grp-user-a-" + ts
	userB := "test-grp-user-b-" + ts
	testDate := time.Date(2099, 7, 1, 0, 0, 0, 0, time.UTC)

	// 创建用户组
	group, err := CreateUserGroup(tdb, "test-detail-group-"+ts, "", []string{userA, userB})
	if err != nil {
		t.Fatalf("创建测试组失败: %v", err)
	}
	defer tdb.Exec(`DELETE FROM user_groups WHERE group_id = $1`, group.GroupID)

	// 为两个用户各插入 user_productivity 数据
	upIDA := userA + "_20990701"
	_, err = tdb.Exec(`INSERT INTO user_productivity
		(user_productivity_id, create_time, user_id, user_name,
		 task_ids, task_diff_lines, upstream_tokens, downstream_tokens, cost,
		 task_real_minutes, task_ancient_minutes, commit_ids, commit_diff_lines,
		 commit_ancient_minutes, commit_real_minutes)
		VALUES ($1, $2, $3, 'UserA', '["t1"]', 100, 5000, 3000, 0.5, 30, 120, '["c1"]', 200, 300, 60)`,
		upIDA, testDate, userA)
	if err != nil {
		t.Fatalf("插入 userA productivity 失败: %v", err)
	}
	defer tdb.Exec(`DELETE FROM user_productivity WHERE user_productivity_id = $1`, upIDA)

	upIDB := userB + "_20990701"
	_, err = tdb.Exec(`INSERT INTO user_productivity
		(user_productivity_id, create_time, user_id, user_name,
		 task_ids, task_diff_lines, upstream_tokens, downstream_tokens, cost,
		 task_real_minutes, task_ancient_minutes, commit_ids, commit_diff_lines,
		 commit_ancient_minutes, commit_real_minutes)
		VALUES ($1, $2, $3, 'UserB', '["t2","t3"]', 200, 8000, 6000, 1.2, 60, 240, '["c2"]', 400, 600, 120)`,
		upIDB, testDate, userB)
	if err != nil {
		t.Fatalf("插入 userB productivity 失败: %v", err)
	}
	defer tdb.Exec(`DELETE FROM user_productivity WHERE user_productivity_id = $1`, upIDB)

	// 查询组详情
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v2/user-groups/"+group.GroupID, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	// 验证返回结构
	if _, ok := resp["group"]; !ok {
		t.Error("响应缺少 group 字段")
	}
	if _, ok := resp["summary"]; !ok {
		t.Error("响应缺少 summary 字段")
	}

	// 验证 members
	members, ok := resp["members"].([]interface{})
	if !ok {
		t.Fatal("响应缺少 members 数组")
	}
	if len(members) != 2 {
		t.Errorf("members 长度 = %d, want 2", len(members))
	}

	// 验证组级汇总
	summary, ok := resp["summary"].(map[string]interface{})
	if !ok {
		t.Fatal("summary 不是对象")
	}
	// day_count: 1+1=2
	if summary["day_count"] != float64(2) {
		t.Errorf("summary.day_count = %v, want 2", summary["day_count"])
	}
	// task_count: 1+2=3
	if summary["task_count"] != float64(3) {
		t.Errorf("summary.task_count = %v, want 3", summary["task_count"])
	}
	// task_diff_lines: 100+200=300
	if summary["task_diff_lines"] != float64(300) {
		t.Errorf("summary.task_diff_lines = %v, want 300", summary["task_diff_lines"])
	}
	// cost: 0.5+1.2=1.7
	costVal, _ := summary["cost"].(float64)
	if costVal < 1.69 || costVal > 1.71 {
		t.Errorf("summary.cost = %v, want ~1.7", costVal)
	}
	// task_efficiency_ratio: round((120+240)/(30+60)*100) = 400
	if summary["task_efficiency_ratio"] != float64(400) {
		t.Errorf("summary.task_efficiency_ratio = %v, want 400", summary["task_efficiency_ratio"])
	}
}

func TestGetUserGroupDetail_NotFound(t *testing.T) {
	r, tdb := setupUserProductivityTestRouter(t)
	defer tdb.Close()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v2/user-groups/00000000-0000-0000-0000-000000000000", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("HTTP status = %d, want 404, body: %s", w.Code, w.Body.String())
	}
}
