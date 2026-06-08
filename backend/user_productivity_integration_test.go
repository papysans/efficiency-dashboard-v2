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
	v2.POST("/user-groups", createUserGroupHandler)
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

// ============================================================

// ============================================================
// UP-API-04: GET /api/v2/user-productivity/:userId 详情 + 400
// ============================================================

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
