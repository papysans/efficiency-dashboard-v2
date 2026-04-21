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

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

// testDB 返回连接到本地测试数据库的 *sql.DB，并自动在测试结束时关闭
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := "host=localhost port=5432 user=postgres password=1 dbname=costrict_stat sslmode=disable"
	tdb, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("无法连接测试数据库: %v", err)
	}
	if err := tdb.Ping(); err != nil {
		t.Fatalf("测试数据库 ping 失败: %v", err)
	}
	// 注意：不在此处注册 t.Cleanup 关闭，因为多个测试共享全局 statDB/db，
	// 调用方应自行管理连接生命周期
	return tdb
}

// ============================================================
// 测试点 5: UpdateStatTaskManual DB 函数
// ============================================================

// 5.1 正常更新 manual 字段
func TestUpdateStatTaskManual_Normal(t *testing.T) {
	tdb := testDB(t)
	defer tdb.Close()

	// 先插入一条测试 task
	testTaskID := "test-manual-update-001"
	_, err := tdb.Exec(`
		INSERT INTO tasks (task_id) VALUES ($1)
		ON CONFLICT(task_id) DO NOTHING`, testTaskID)
	if err != nil {
		t.Fatalf("插入测试数据失败: %v", err)
	}
	defer tdb.Exec(`DELETE FROM tasks WHERE task_id = $1`, testTaskID)

	realManual := 120.0
	realReason := "手动修正实际耗时"
	ancientManual := 480.0
	ancientReason := "手动修正传统耗时"

	err = UpdateStatTaskManual(tdb, testTaskID, &realManual, &realReason, &ancientManual, &ancientReason)
	if err != nil {
		t.Fatalf("UpdateStatTaskManual 失败: %v", err)
	}

	// 验证更新结果
	task, err := GetStatTask(tdb, testTaskID)
	if err != nil {
		t.Fatalf("查询 task 失败: %v", err)
	}
	if task == nil {
		t.Fatal("task 不应为 nil")
	}
	if task.TaskRealMinutesManual == nil || *task.TaskRealMinutesManual != 120.0 {
		t.Errorf("TaskRealMinutesManual = %v, want 120.0", task.TaskRealMinutesManual)
	}
	if task.TaskAncientMinutesManual == nil || *task.TaskAncientMinutesManual != 480.0 {
		t.Errorf("TaskAncientMinutesManual = %v, want 480.0", task.TaskAncientMinutesManual)
	}
}

// 5.2 不存在的 taskID → 返回错误
func TestUpdateStatTaskManual_NotFound(t *testing.T) {
	tdb := testDB(t)
	defer tdb.Close()

	realManual := 120.0
	err := UpdateStatTaskManual(tdb, "nonexistent-task-id-xyz", &realManual, nil, nil, nil)
	if err == nil {
		t.Fatal("应该返回错误（task 不存在）")
	}
}

// 5.3 部分字段为 nil → 允许更新为 NULL
func TestUpdateStatTaskManual_PartialNil(t *testing.T) {
	tdb := testDB(t)
	defer tdb.Close()

	testTaskID := "test-manual-partial-nil-001"
	_, err := tdb.Exec(`
		INSERT INTO tasks (task_id, task_real_minutes_manual, task_real_minutes_reason_manual)
		VALUES ($1, 100, '旧原因')
		ON CONFLICT(task_id) DO UPDATE SET task_real_minutes_manual = 100, task_real_minutes_reason_manual = '旧原因'`,
		testTaskID)
	if err != nil {
		t.Fatalf("插入测试数据失败: %v", err)
	}
	defer tdb.Exec(`DELETE FROM tasks WHERE task_id = $1`, testTaskID)

	// 将 real_manual 设为 nil，ancient_manual 设为 200
	ancientManual := 200.0
	err = UpdateStatTaskManual(tdb, testTaskID, nil, nil, &ancientManual, nil)
	if err != nil {
		t.Fatalf("UpdateStatTaskManual 失败: %v", err)
	}

	task, err := GetStatTask(tdb, testTaskID)
	if err != nil {
		t.Fatalf("查询 task 失败: %v", err)
	}
	if task.TaskRealMinutesManual != nil {
		t.Errorf("TaskRealMinutesManual 应为 nil（被清空），got %v", *task.TaskRealMinutesManual)
	}
	if task.TaskAncientMinutesManual == nil || *task.TaskAncientMinutesManual != 200.0 {
		t.Errorf("TaskAncientMinutesManual = %v, want 200.0", task.TaskAncientMinutesManual)
	}
}

// ============================================================
// 测试点 6: GET /api/v2/tasks/:taskId 返回新字段
// ============================================================

func setupTestRouter(t *testing.T) (*gin.Engine, *sql.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	tdb := testDB(t)

	// 设置全局变量供 handler 使用（task handler 同时使用 db 和 statDB）
	db = tdb
	statDB = tdb
	appConfig.TaskRealMinutes.GapThresholdMinutes = 30
	appConfig.TaskRealMinutes.ExtensionMinutes = 5

	// 使用 t.Cleanup 确保测试结束后关闭连接，避免手动 defer tdb.Close() 与全局 db/statDB 冲突
	t.Cleanup(func() { tdb.Close() })

	r := gin.New()
	v2 := r.Group("/api/v2")
	v2.GET("/tasks/:taskId", getTaskDetailV2)
	v2.PUT("/tasks/:taskId/manual", updateTaskManualV2)
	return r, tdb
}

func TestGetTaskDetailV2_ReturnsNewFields(t *testing.T) {
	r, tdb := setupTestRouter(t)

	testTaskID := "test-api-detail-001"
	ancient := 480.0
	_, err := tdb.Exec(`
		INSERT INTO tasks (task_id, task_ancient_minutes, task_ancient_minutes_reason)
		VALUES ($1, $2, '测试传统耗时')
		ON CONFLICT(task_id) DO UPDATE SET task_ancient_minutes = $2`,
		testTaskID, ancient)
	if err != nil {
		t.Fatalf("插入测试数据失败: %v", err)
	}
	defer tdb.Exec(`DELETE FROM tasks WHERE task_id = $1`, testTaskID)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v2/tasks/"+testTaskID, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("JSON 解析失败: %v", err)
	}

	// 验证返回结构包含新字段
	if _, ok := resp["task"]; !ok {
		t.Error("响应缺少 task 字段")
	}
	if _, ok := resp["time_segments"]; !ok {
		t.Error("响应缺少 time_segments 字段")
	}
	if _, ok := resp["efficiency_ratio"]; !ok {
		t.Error("响应缺少 efficiency_ratio 字段")
	}
	if _, ok := resp["conversations"]; !ok {
		t.Error("响应缺少 conversations 字段")
	}

	// 验证 task 子对象包含 task_real_minutes 字段
	taskMap, ok := resp["task"].(map[string]interface{})
	if !ok {
		t.Fatal("task 字段不是 object 类型")
	}
	if _, ok := taskMap["task_real_minutes"]; !ok {
		t.Error("task 对象缺少 task_real_minutes 字段")
	}
	if _, ok := taskMap["task_ancient_minutes"]; !ok {
		t.Error("task 对象缺少 task_ancient_minutes 字段")
	}
}

// 6.2 task 不存在 → 404
func TestGetTaskDetailV2_NotFound(t *testing.T) {
	r, _ := setupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v2/tasks/nonexistent-task-xyz", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("HTTP status = %d, want 404", w.Code)
	}
}

// ============================================================
// 测试点 7: PUT /api/v2/tasks/:taskId/manual API
// ============================================================

// 7.1 正常更新
func TestUpdateTaskManualV2_Normal(t *testing.T) {
	r, tdb := setupTestRouter(t)

	testTaskID := "test-api-manual-001"
	_, err := tdb.Exec(`INSERT INTO tasks (task_id) VALUES ($1) ON CONFLICT(task_id) DO NOTHING`, testTaskID)
	if err != nil {
		t.Fatalf("插入测试数据失败: %v", err)
	}
	defer tdb.Exec(`DELETE FROM tasks WHERE task_id = $1`, testTaskID)

	body := `{"task_real_minutes_manual": 120, "task_real_minutes_reason_manual": "手动修正"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v2/tasks/"+testTaskID+"/manual", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "ok" {
		t.Errorf("status = %v, want ok", resp["status"])
	}
}

// 7.2 无效 JSON → 400
func TestUpdateTaskManualV2_InvalidJSON(t *testing.T) {
	r, _ := setupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v2/tasks/some-task/manual", bytes.NewBufferString("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("HTTP status = %d, want 400", w.Code)
	}
}

// ============================================================
// 测试点 8: 数据库字段验证 — 新字段存在
// ============================================================

func TestDBSchema_NewFieldsExist(t *testing.T) {
	tdb := testDB(t)
	defer tdb.Close()

	newColumns := []string{
		"task_real_minutes",
		"task_real_minutes_reason",
		"task_real_minutes_manual",
		"task_real_minutes_reason_manual",
		"task_ancient_minutes",
		"task_ancient_minutes_reason",
		"task_ancient_minutes_manual",
		"task_ancient_minutes_reason_manual",
	}

	for _, col := range newColumns {
		var exists bool
		err := tdb.QueryRow(`
			SELECT EXISTS(
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'tasks' AND column_name = $1
			)`, col).Scan(&exists)
		if err != nil {
			t.Errorf("检查列 %s 失败: %v", col, err)
			continue
		}
		if !exists {
			t.Errorf("tasks 缺少列: %s", col)
		}
	}
}

// ============================================================
// 测试点 9: 数据库字段验证 — 旧字段不存在
// ============================================================

func TestDBSchema_OldFieldsRemoved(t *testing.T) {
	tdb := testDB(t)
	defer tdb.Close()

	oldColumns := []string{
		"ai_estimated_ancient_days",
		"ai_estimated_ancient_reason",
	}

	for _, col := range oldColumns {
		var exists bool
		err := tdb.QueryRow(`
			SELECT EXISTS(
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'tasks' AND column_name = $1
			)`, col).Scan(&exists)
		if err != nil {
			t.Errorf("检查列 %s 失败: %v", col, err)
			continue
		}
		if exists {
			t.Errorf("tasks 不应存在旧列: %s（已重命名）", col)
		}
	}
}

// ============================================================
// 辅助：打印测试摘要
// ============================================================

func init() {
	_ = fmt.Sprintf // 避免 import 未使用错误
}
