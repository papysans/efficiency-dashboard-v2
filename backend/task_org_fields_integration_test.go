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
)

// ============================================================
// 测试点 1-3: listTasksV2 返回 org1-org4 字段
// 验证 task_handler_v2.go 第226-244行的 org 字段补充逻辑
// ============================================================

func setupListTasksOrgTestRouter(t *testing.T) (*gin.Engine, func()) {
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

// TP-01: UserID 存在且在 orgMappings 中能找到 → 返回对应 org 值
func TestListTasksV2_OrgFields_UserInMapping(t *testing.T) {
	r, cleanup := setupListTasksOrgTestRouter(t)
	defer cleanup()

	ts := fmt.Sprintf("%d", time.Now().UnixNano())
	taskID := "test-org-mapped-" + ts
	testUserID := "test-user-org-mapped-" + ts
	startTime := time.Date(2025, 7, 1, 10, 0, 0, 0, time.UTC)

	// 插入带 user_id 的 task
	_, err := statDB.Exec(`INSERT INTO tasks (task_id, user_id, start_time)
		VALUES ($1, $2, $3)`, taskID, testUserID, startTime)
	if err != nil {
		t.Fatalf("插入 task 失败: %v", err)
	}
	defer statDB.Exec(`DELETE FROM tasks WHERE task_id = $1`, taskID)

	// 备份并设置 orgMappings
	origMappings := orgMappings
	orgMappings = map[string]*OrgMapping{
		testUserID: {
			UserID: testUserID, UserName: "测试用户",
			Org1: "研发中心", Org2: "平台部", Org3: "基础架构组", Org4: "云原生团队",
		},
	}
	defer func() { orgMappings = origMappings }()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v2/tasks?startDate=20250701&endDate=20250701", nil)
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
			if m["org1"] != "研发中心" {
				t.Errorf("org1 = %v, want '研发中心'", m["org1"])
			}
			if m["org2"] != "平台部" {
				t.Errorf("org2 = %v, want '平台部'", m["org2"])
			}
			if m["org3"] != "基础架构组" {
				t.Errorf("org3 = %v, want '基础架构组'", m["org3"])
			}
			if m["org4"] != "云原生团队" {
				t.Errorf("org4 = %v, want '云原生团队'", m["org4"])
			}
			break
		}
	}
	if !found {
		t.Errorf("未在响应中找到 task_id=%s", taskID)
	}
}

// TP-02: UserID 存在但不在 orgMappings 中 → 返回空字符串
func TestListTasksV2_OrgFields_UserNotInMapping(t *testing.T) {
	r, cleanup := setupListTasksOrgTestRouter(t)
	defer cleanup()

	ts := fmt.Sprintf("%d", time.Now().UnixNano())
	taskID := "test-org-unmapped-" + ts
	testUserID := "test-user-org-unmapped-" + ts
	startTime := time.Date(2025, 7, 2, 10, 0, 0, 0, time.UTC)

	_, err := statDB.Exec(`INSERT INTO tasks (task_id, user_id, start_time)
		VALUES ($1, $2, $3)`, taskID, testUserID, startTime)
	if err != nil {
		t.Fatalf("插入 task 失败: %v", err)
	}
	defer statDB.Exec(`DELETE FROM tasks WHERE task_id = $1`, taskID)

	// orgMappings 不包含该 user
	origMappings := orgMappings
	orgMappings = map[string]*OrgMapping{
		"some-other-user": {UserID: "some-other-user", Org1: "其他"},
	}
	defer func() { orgMappings = origMappings }()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v2/tasks?startDate=20250702&endDate=20250702", nil)
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
			for _, field := range []string{"org1", "org2", "org3", "org4"} {
				if m[field] != "" {
					t.Errorf("%s = %v, want empty string", field, m[field])
				}
			}
			break
		}
	}
	if !found {
		t.Errorf("未在响应中找到 task_id=%s", taskID)
	}
}

// TP-03: UserID 为 nil（task 没有 user_id）→ 返回空字符串
func TestListTasksV2_OrgFields_UserIDNil(t *testing.T) {
	r, cleanup := setupListTasksOrgTestRouter(t)
	defer cleanup()

	ts := fmt.Sprintf("%d", time.Now().UnixNano())
	taskID := "test-org-nil-user-" + ts
	startTime := time.Date(2025, 7, 3, 10, 0, 0, 0, time.UTC)

	// 插入不带 user_id 的 task（user_id 为 NULL）
	_, err := statDB.Exec(`INSERT INTO tasks (task_id, start_time)
		VALUES ($1, $2)`, taskID, startTime)
	if err != nil {
		t.Fatalf("插入 task 失败: %v", err)
	}
	defer statDB.Exec(`DELETE FROM tasks WHERE task_id = $1`, taskID)

	origMappings := orgMappings
	orgMappings = make(map[string]*OrgMapping)
	defer func() { orgMappings = origMappings }()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v2/tasks?startDate=20250703&endDate=20250703", nil)
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
			for _, field := range []string{"org1", "org2", "org3", "org4"} {
				if m[field] != "" {
					t.Errorf("%s = %v, want empty string", field, m[field])
				}
			}
			break
		}
	}
	if !found {
		t.Errorf("未在响应中找到 task_id=%s", taskID)
	}
}
