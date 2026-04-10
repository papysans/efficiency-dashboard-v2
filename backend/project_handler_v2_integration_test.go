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

// setupProjectTestRouter 创建测试用 gin 路由，注册所有 project V2 端点
func setupProjectTestRouter(t *testing.T) (*gin.Engine, *sql.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	tdb := testDB(t)

	// 设置全局变量供 handler 使用
	statDB = tdb

	r := gin.New()
	v2 := r.Group("/api/v2")
	v2.POST("/projects", createProjectV2)
	v2.GET("/projects", listProjectsV2)
	v2.POST("/projects/check-conflicts", checkProjectConflictsV2)
	v2.GET("/projects/:projectId", getProjectDetailV2)
	v2.PUT("/projects/:projectId", updateProjectV2)
	v2.DELETE("/projects/:projectId", deleteProjectV2)
	v2.PUT("/projects/:projectId/manual", updateProjectManualV2)
	v2.POST("/projects/:projectId/tasks", addTasksToProjectV2)
	v2.POST("/projects/:projectId/repos", addRepoToProjectV2)
	v2.DELETE("/projects/:projectId/repos/:index", removeRepoFromProjectV2)

	return r, tdb
}

// createTestProject 辅助函数：通过 API 创建测试项目，返回 project_id
func createTestProject(t *testing.T, r *gin.Engine, name string) string {
	t.Helper()
	body := fmt.Sprintf(`{"name":"%s","description":"test project"}`, name)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v2/projects", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("创建测试项目失败: HTTP %d, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	projectID, ok := resp["project_id"].(string)
	if !ok || projectID == "" {
		t.Fatalf("创建测试项目未返回有效 project_id: %v", resp)
	}
	return projectID
}

// ============================================================
// TP-DB-01: projects 表存在且列结构正确
// ============================================================

func TestProjectDB_TableColumnsExist(t *testing.T) {
	tdb := testDB(t)
	defer tdb.Close()

	expectedColumns := map[string]string{
		"project_id":                            "uuid",
		"name":                                  "character varying",
		"description":                           "text",
		"repos":                                 "jsonb",
		"task_ids":                              "jsonb",
		"task_ids_silica":                       "jsonb",
		"start_time":                            "timestamp with time zone",
		"end_time":                              "timestamp with time zone",
		"start_time_manual":                     "timestamp with time zone",
		"end_time_manual":                       "timestamp with time zone",
		"upstream_tokens":                       "bigint",
		"downstream_tokens":                     "bigint",
		"cost":                                  "double precision",
		"project_ancient_minutes":               "double precision",
		"project_ancient_minutes_reason":        "text",
		"project_ancient_minutes_manual":        "double precision",
		"project_ancient_minutes_reason_manual": "text",
		"project_real_process_minutes":          "double precision",
		"project_real_process_minutes_reason":   "text",
		"project_real_process_minutes_manual":   "double precision",
		"project_real_process_minutes_reason_manual": "text",
		"project_real_lead_minutes":                  "double precision",
		"project_real_lead_minutes_reason":           "text",
		"project_real_lead_minutes_manual":           "double precision",
		"project_real_lead_minutes_reason_manual":    "text",
		"created_at": "timestamp with time zone",
		"updated_at": "timestamp with time zone",
	}

	for col, expectedType := range expectedColumns {
		var dataType string
		err := tdb.QueryRow(`
			SELECT data_type FROM information_schema.columns
			WHERE table_name = 'projects' AND column_name = $1`, col).Scan(&dataType)
		if err == sql.ErrNoRows {
			t.Errorf("projects 表缺少列: %s", col)
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
}

// ============================================================
// TP-DB-02: projects 表默认值正确
// ============================================================

func TestProjectDB_DefaultValues(t *testing.T) {
	tdb := testDB(t)
	defer tdb.Close()

	// 只插入必填字段 name，验证默认值
	var projectID string
	err := tdb.QueryRow(`
		INSERT INTO projects (name) VALUES ('test-default-values')
		RETURNING project_id`).Scan(&projectID)
	if err != nil {
		t.Fatalf("插入 projects 失败: %v", err)
	}
	defer tdb.Exec(`DELETE FROM projects WHERE project_id = $1`, projectID)

	var repos, taskIDs, taskIDsSilica string
	var upTokens, downTokens int64
	var cost float64
	err = tdb.QueryRow(`
		SELECT repos::text, task_ids::text, task_ids_silica::text,
		       upstream_tokens, downstream_tokens, cost
		FROM projects WHERE project_id = $1`, projectID).
		Scan(&repos, &taskIDs, &taskIDsSilica, &upTokens, &downTokens, &cost)
	if err != nil {
		t.Fatalf("查询默认值失败: %v", err)
	}

	if repos != "[]" {
		t.Errorf("repos 默认值 = %s, want []", repos)
	}
	if taskIDs != "[]" {
		t.Errorf("task_ids 默认值 = %s, want []", taskIDs)
	}
	if taskIDsSilica != "[]" {
		t.Errorf("task_ids_silica 默认值 = %s, want []", taskIDsSilica)
	}
	if upTokens != 0 {
		t.Errorf("upstream_tokens 默认值 = %d, want 0", upTokens)
	}
	if downTokens != 0 {
		t.Errorf("downstream_tokens 默认值 = %d, want 0", downTokens)
	}
	if cost != 0 {
		t.Errorf("cost 默认值 = %f, want 0", cost)
	}
}

// ============================================================
// TP-DB-03: projects 表索引存在
// ============================================================

func TestProjectDB_IndexesExist(t *testing.T) {
	tdb := testDB(t)
	defer tdb.Close()

	indexes := []string{"idx_projects_name", "idx_projects_updated_at"}
	for _, idx := range indexes {
		var exists bool
		err := tdb.QueryRow(`
			SELECT EXISTS(
				SELECT 1 FROM pg_indexes
				WHERE tablename = 'projects' AND indexname = $1
			)`, idx).Scan(&exists)
		if err != nil {
			t.Errorf("检查索引 %s 失败: %v", idx, err)
			continue
		}
		if !exists {
			t.Errorf("projects 表缺少索引: %s", idx)
		}
	}
}

// ============================================================
// TP-API-01: POST /api/v2/projects 正常创建返回有效 UUID
// ============================================================

func TestProjectCreate_Normal(t *testing.T) {
	r, tdb := setupProjectTestRouter(t)
	defer tdb.Close()

	ts := fmt.Sprintf("%d", time.Now().UnixNano())
	name := "test-create-project-" + ts

	body := fmt.Sprintf(`{"name":"%s","description":"integration test project"}`, name)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v2/projects", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	projectID, ok := resp["project_id"].(string)
	if !ok || projectID == "" {
		t.Fatal("响应缺少有效的 project_id")
	}
	defer tdb.Exec(`DELETE FROM projects WHERE project_id = $1`, projectID)

	// UUID 长度为 36（含连字符: 8-4-4-4-12）
	if len(projectID) != 36 {
		t.Errorf("project_id 长度 = %d, want 36 (UUID 格式)", len(projectID))
	}

	// 验证返回的 name 正确
	if resp["name"] != name {
		t.Errorf("name = %v, want %s", resp["name"], name)
	}
}

// ============================================================
// TP-API-02: POST /api/v2/projects 空 name 返回 400
// ============================================================

func TestProjectCreate_EmptyName(t *testing.T) {
	r, tdb := setupProjectTestRouter(t)
	defer tdb.Close()

	body := `{"name":"","description":"should fail"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v2/projects", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("HTTP status = %d, want 400, body: %s", w.Code, w.Body.String())
	}
}

// ============================================================
// TP-API-03: GET /api/v2/projects 列表返回计算字段
// ============================================================

func TestProjectList_ComputedFields(t *testing.T) {
	r, tdb := setupProjectTestRouter(t)
	defer tdb.Close()

	ts := fmt.Sprintf("%d", time.Now().UnixNano())
	name := "test-list-project-" + ts

	// 创建项目
	projectID := createTestProject(t, r, name)
	defer tdb.Exec(`DELETE FROM projects WHERE project_id = $1`, projectID)

	// 添加 task_ids 和 repos 以便列表有计算字段
	_, err := tdb.Exec(`
		UPDATE projects SET
			task_ids = '["task-a","task-b"]'::jsonb,
			repos = '[{"repo_addr":"test-repo","repo_branch":"main"}]'::jsonb,
			project_ancient_minutes = 480,
			project_real_process_minutes = 120
		WHERE project_id = $1`, projectID)
	if err != nil {
		t.Fatalf("更新测试数据失败: %v", err)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v2/projects", nil)
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

	// 查找我们创建的项目
	var found bool
	for _, item := range data {
		p, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if p["project_id"] == projectID {
			found = true

			// 验证计算字段
			repoCount, ok := p["repo_count"].(float64)
			if !ok {
				t.Errorf("repo_count 类型不正确: %T", p["repo_count"])
			} else if repoCount != 1 {
				t.Errorf("repo_count = %v, want 1", repoCount)
			}

			taskCount, ok := p["task_count"].(float64)
			if !ok {
				t.Errorf("task_count 类型不正确: %T", p["task_count"])
			} else if taskCount != 2 {
				t.Errorf("task_count = %v, want 2", taskCount)
			}

			effRatio, ok := p["efficiency_ratio"].(float64)
			if !ok {
				t.Errorf("efficiency_ratio 类型不正确: %T", p["efficiency_ratio"])
			} else if effRatio != 400 {
				t.Errorf("efficiency_ratio = %v, want 400 (480/120*100)", effRatio)
			}

			break
		}
	}
	if !found {
		t.Errorf("列表中未找到 project_id=%s", projectID)
	}
}

// ============================================================
// TP-API-04: GET /api/v2/projects/:projectId 详情和 404
// ============================================================

func TestProjectDetail_Normal(t *testing.T) {
	r, tdb := setupProjectTestRouter(t)
	defer tdb.Close()

	ts := fmt.Sprintf("%d", time.Now().UnixNano())
	projectID := createTestProject(t, r, "test-detail-"+ts)
	defer tdb.Exec(`DELETE FROM projects WHERE project_id = $1`, projectID)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v2/projects/"+projectID, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	// 验证返回结构包含必要字段
	if _, ok := resp["project"]; !ok {
		t.Error("响应缺少 project 字段")
	}
	if _, ok := resp["commits"]; !ok {
		t.Error("响应缺少 commits 字段")
	}
	if _, ok := resp["tasks"]; !ok {
		t.Error("响应缺少 tasks 字段")
	}
	if _, ok := resp["efficiency_ratio"]; !ok {
		t.Error("响应缺少 efficiency_ratio 字段")
	}
}

func TestProjectDetail_NotFound(t *testing.T) {
	r, tdb := setupProjectTestRouter(t)
	defer tdb.Close()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v2/projects/00000000-0000-0000-0000-000000000000", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("HTTP status = %d, want 404", w.Code)
	}
}

// ============================================================
// TP-API-05: PUT /api/v2/projects/:projectId 更新项目
// ============================================================

func TestProjectUpdate_Normal(t *testing.T) {
	r, tdb := setupProjectTestRouter(t)
	defer tdb.Close()

	ts := fmt.Sprintf("%d", time.Now().UnixNano())
	projectID := createTestProject(t, r, "test-update-before-"+ts)
	defer tdb.Exec(`DELETE FROM projects WHERE project_id = $1`, projectID)

	newName := "test-update-after-" + ts
	body := fmt.Sprintf(`{
		"name":"%s",
		"description":"updated description",
		"repos":[{"repo_addr":"new-repo","repo_branch":"main"}],
		"task_ids":["tid-1","tid-2"],
		"task_ids_silica":[1.0,0.5]
	}`, newName)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v2/projects/"+projectID, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	// 验证数据库已更新
	project, err := GetProject(tdb, projectID)
	if err != nil {
		t.Fatalf("GetProject 失败: %v", err)
	}
	if project.Name != newName {
		t.Errorf("name = %s, want %s", project.Name, newName)
	}
	if project.Description == nil || *project.Description != "updated description" {
		t.Errorf("description = %v, want 'updated description'", project.Description)
	}

	// 验证 repos JSON
	var repos []RepoFilter
	json.Unmarshal(project.Repos, &repos)
	if len(repos) != 1 || repos[0].RepoAddr != "new-repo" {
		t.Errorf("repos = %s, want [{repo_addr:'new-repo',...}]", string(project.Repos))
	}

	// 验证 task_ids
	var taskIDs []string
	json.Unmarshal(project.TaskIDs, &taskIDs)
	if len(taskIDs) != 2 {
		t.Errorf("task_ids 长度 = %d, want 2", len(taskIDs))
	}
}

func TestProjectUpdate_NotFound(t *testing.T) {
	r, tdb := setupProjectTestRouter(t)
	defer tdb.Close()

	body := `{"name":"no-such-project"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v2/projects/00000000-0000-0000-0000-000000000000", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("HTTP status = %d, want 404, body: %s", w.Code, w.Body.String())
	}
}

// ============================================================
// TP-API-06: DELETE /api/v2/projects/:projectId 删除项目
// ============================================================

func TestProjectDelete_Normal(t *testing.T) {
	r, tdb := setupProjectTestRouter(t)
	defer tdb.Close()

	ts := fmt.Sprintf("%d", time.Now().UnixNano())
	projectID := createTestProject(t, r, "test-delete-"+ts)
	// 不用 defer 删除，因为测试本身会删除

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v2/projects/"+projectID, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	// 验证确实已删除
	project, err := GetProject(tdb, projectID)
	if err != nil {
		t.Fatalf("GetProject 失败: %v", err)
	}
	if project != nil {
		t.Error("删除后 GetProject 应返回 nil")
	}
}

func TestProjectDelete_NotFound(t *testing.T) {
	r, tdb := setupProjectTestRouter(t)
	defer tdb.Close()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v2/projects/00000000-0000-0000-0000-000000000000", nil)
	r.ServeHTTP(w, req)

	// DeleteProject 对不存在的 ID 返回 error，handler 返回 500
	if w.Code != http.StatusInternalServerError {
		t.Errorf("HTTP status = %d, want 500 (project 不存在)", w.Code)
	}
}

// ============================================================
// TP-API-07: PUT /api/v2/projects/:projectId/manual 手动修正
// ============================================================

func TestProjectManualUpdate_ValidFields(t *testing.T) {
	r, tdb := setupProjectTestRouter(t)
	defer tdb.Close()

	ts := fmt.Sprintf("%d", time.Now().UnixNano())
	projectID := createTestProject(t, r, "test-manual-"+ts)
	defer tdb.Exec(`DELETE FROM projects WHERE project_id = $1`, projectID)

	body := `{
		"project_ancient_minutes_manual": 600,
		"project_ancient_minutes_reason_manual": "手动修正传统耗时",
		"project_real_process_minutes_manual": 150,
		"project_real_process_minutes_reason_manual": "手动修正实际耗时"
	}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v2/projects/"+projectID+"/manual", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	// 验证数据库
	project, err := GetProject(tdb, projectID)
	if err != nil {
		t.Fatalf("GetProject 失败: %v", err)
	}
	if project.ProjectAncientMinutesManual == nil || *project.ProjectAncientMinutesManual != 600 {
		t.Errorf("ProjectAncientMinutesManual = %v, want 600", project.ProjectAncientMinutesManual)
	}
	if project.ProjectRealProcessMinutesManual == nil || *project.ProjectRealProcessMinutesManual != 150 {
		t.Errorf("ProjectRealProcessMinutesManual = %v, want 150", project.ProjectRealProcessMinutesManual)
	}
}

func TestProjectManualUpdate_DisallowedField(t *testing.T) {
	r, tdb := setupProjectTestRouter(t)
	defer tdb.Close()

	ts := fmt.Sprintf("%d", time.Now().UnixNano())
	projectID := createTestProject(t, r, "test-manual-disallow-"+ts)
	defer tdb.Exec(`DELETE FROM projects WHERE project_id = $1`, projectID)

	// 尝试更新不允许的字段 name
	body := `{"name":"hacked"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v2/projects/"+projectID+"/manual", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("HTTP status = %d, want 500 (不允许更新的字段), body: %s", w.Code, w.Body.String())
	}

	// 验证 name 未被修改
	project, err := GetProject(tdb, projectID)
	if err != nil {
		t.Fatalf("GetProject 失败: %v", err)
	}
	if project.Name == "hacked" {
		t.Error("不允许的字段 name 不应被修改")
	}
}

// ============================================================
// TP-API-08: POST /api/v2/projects/:projectId/tasks 添加 tasks（去重）
// ============================================================

func TestProjectAddTasks_Deduplication(t *testing.T) {
	r, tdb := setupProjectTestRouter(t)
	defer tdb.Close()

	ts := fmt.Sprintf("%d", time.Now().UnixNano())
	projectID := createTestProject(t, r, "test-add-tasks-"+ts)
	defer tdb.Exec(`DELETE FROM projects WHERE project_id = $1`, projectID)

	// 第一次添加 task_ids
	body1 := `{"task_ids":["t1","t2"],"task_ids_silica":[1.0,0.8]}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v2/projects/"+projectID+"/tasks", bytes.NewBufferString(body1))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("第一次添加 tasks HTTP status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	// 第二次添加（t2 重复，t3 新增）
	body2 := `{"task_ids":["t2","t3"],"task_ids_silica":[0.5,0.9]}`
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v2/projects/"+projectID+"/tasks", bytes.NewBufferString(body2))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("第二次添加 tasks HTTP status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	// 验证去重结果
	project, err := GetProject(tdb, projectID)
	if err != nil {
		t.Fatalf("GetProject 失败: %v", err)
	}

	var taskIDs []string
	json.Unmarshal(project.TaskIDs, &taskIDs)
	if len(taskIDs) != 3 {
		t.Errorf("task_ids 长度 = %d, want 3 (去重后 t1,t2,t3), actual: %v", len(taskIDs), taskIDs)
	}

	var silica []float64
	json.Unmarshal(project.TaskIDsSilica, &silica)
	if len(silica) != 3 {
		t.Errorf("task_ids_silica 长度 = %d, want 3, actual: %v", len(silica), silica)
	}
	// t2 是重复的，应保留第一次的 silica 值 0.8
	if len(silica) >= 2 && silica[1] != 0.8 {
		t.Errorf("t2 的 silica = %v, want 0.8 (保留首次值)", silica[1])
	}
}

func TestProjectAddTasks_NotFound(t *testing.T) {
	r, tdb := setupProjectTestRouter(t)
	defer tdb.Close()

	body := `{"task_ids":["t1"],"task_ids_silica":[1.0]}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v2/projects/00000000-0000-0000-0000-000000000000/tasks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("HTTP status = %d, want 404", w.Code)
	}
}

// ============================================================
// TP-API-09: Repo 添加和删除
// ============================================================

func TestProjectAddRepo_Normal(t *testing.T) {
	r, tdb := setupProjectTestRouter(t)
	defer tdb.Close()

	ts := fmt.Sprintf("%d", time.Now().UnixNano())
	projectID := createTestProject(t, r, "test-add-repo-"+ts)
	defer tdb.Exec(`DELETE FROM projects WHERE project_id = $1`, projectID)

	// 添加一个 repo
	body := `{"repo_addr":"https://github.com/test/repo","repo_branch":"main"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v2/projects/"+projectID+"/repos", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("添加 repo HTTP status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	// 验证 repos
	project, err := GetProject(tdb, projectID)
	if err != nil {
		t.Fatalf("GetProject 失败: %v", err)
	}
	var repos []RepoFilter
	json.Unmarshal(project.Repos, &repos)
	if len(repos) != 1 {
		t.Errorf("repos 长度 = %d, want 1", len(repos))
	}
	if len(repos) > 0 && repos[0].RepoAddr != "https://github.com/test/repo" {
		t.Errorf("repos[0].RepoAddr = %s, want https://github.com/test/repo", repos[0].RepoAddr)
	}
}

func TestProjectRemoveRepo_Normal(t *testing.T) {
	r, tdb := setupProjectTestRouter(t)
	defer tdb.Close()

	ts := fmt.Sprintf("%d", time.Now().UnixNano())
	projectID := createTestProject(t, r, "test-remove-repo-"+ts)
	defer tdb.Exec(`DELETE FROM projects WHERE project_id = $1`, projectID)

	// 先添加两个 repos
	_, err := tdb.Exec(`
		UPDATE projects SET repos = '[{"repo_addr":"repo-a","repo_branch":"main"},{"repo_addr":"repo-b","repo_branch":"dev"}]'::jsonb
		WHERE project_id = $1`, projectID)
	if err != nil {
		t.Fatalf("设置 repos 失败: %v", err)
	}

	// 删除 index=0 (repo-a)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v2/projects/"+projectID+"/repos/0", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("删除 repo HTTP status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	// 验证只剩 repo-b
	project, err := GetProject(tdb, projectID)
	if err != nil {
		t.Fatalf("GetProject 失败: %v", err)
	}
	var repos []RepoFilter
	json.Unmarshal(project.Repos, &repos)
	if len(repos) != 1 {
		t.Errorf("repos 长度 = %d, want 1", len(repos))
	}
	if len(repos) > 0 && repos[0].RepoAddr != "repo-b" {
		t.Errorf("剩余 repo = %s, want repo-b", repos[0].RepoAddr)
	}
}

func TestProjectRemoveRepo_IndexOutOfBounds(t *testing.T) {
	r, tdb := setupProjectTestRouter(t)
	defer tdb.Close()

	ts := fmt.Sprintf("%d", time.Now().UnixNano())
	projectID := createTestProject(t, r, "test-remove-repo-oob-"+ts)
	defer tdb.Exec(`DELETE FROM projects WHERE project_id = $1`, projectID)

	// repos 为空，删除 index=0 应返回 400
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v2/projects/"+projectID+"/repos/0", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("HTTP status = %d, want 400 (index 超出范围), body: %s", w.Code, w.Body.String())
	}

	// 测试负数 index
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/api/v2/projects/"+projectID+"/repos/-1", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("负数 index HTTP status = %d, want 400", w.Code)
	}
}

// ============================================================
// TP-API-10: POST /api/v2/projects/check-conflicts 冲突检测
// ============================================================

func TestProjectCheckConflicts_NoConflict(t *testing.T) {
	r, tdb := setupProjectTestRouter(t)
	defer tdb.Close()

	body := `{"commit_ids":["nonexistent-commit-1","nonexistent-commit-2"]}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v2/projects/check-conflicts", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	conflicts, ok := resp["conflicts"]
	if !ok {
		t.Fatal("响应缺少 conflicts 字段")
	}
	// 无冲突时 conflicts 应为 null 或空数组
	if conflicts != nil {
		arr, ok := conflicts.([]interface{})
		if ok && len(arr) > 0 {
			t.Errorf("conflicts 应为空, got %v", conflicts)
		}
	}
}

func TestProjectCheckConflicts_WithConflict(t *testing.T) {
	r, tdb := setupProjectTestRouter(t)
	defer tdb.Close()

	ts := fmt.Sprintf("%d", time.Now().UnixNano())
	repoAddr := "test-conflict-repo-" + ts
	commitID := "test-conflict-commit-" + ts
	commitTime := time.Date(2025, 6, 1, 10, 0, 0, 0, time.UTC)

	// 插入一条 commit
	_, err := tdb.Exec(`INSERT INTO commits (commit_id, repo_addr, repo_branch, commit_time)
		VALUES ($1, $2, 'main', $3)`, commitID, repoAddr, commitTime)
	if err != nil {
		t.Fatalf("插入 commit 失败: %v", err)
	}
	defer tdb.Exec(`DELETE FROM commits WHERE commit_id = $1`, commitID)

	// 创建项目并关联此 repo
	projectID := createTestProject(t, r, "test-conflict-project-"+ts)
	defer tdb.Exec(`DELETE FROM projects WHERE project_id = $1`, projectID)

	reposJSON := fmt.Sprintf(`[{"repo_addr":"%s","repo_branch":"main"}]`, repoAddr)
	_, err = tdb.Exec(`UPDATE projects SET repos = $2::jsonb WHERE project_id = $1`, projectID, reposJSON)
	if err != nil {
		t.Fatalf("更新 project repos 失败: %v", err)
	}

	// 检测冲突
	body := fmt.Sprintf(`{"commit_ids":["%s"]}`, commitID)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v2/projects/check-conflicts", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	conflicts, ok := resp["conflicts"].([]interface{})
	if !ok || len(conflicts) == 0 {
		t.Fatalf("期望有冲突但 conflicts 为空: %v", resp)
	}

	// 验证冲突内容
	conflict := conflicts[0].(map[string]interface{})
	if conflict["commit_id"] != commitID {
		t.Errorf("conflict.commit_id = %v, want %s", conflict["commit_id"], commitID)
	}
	if conflict["project_id"] != projectID {
		t.Errorf("conflict.project_id = %v, want %s", conflict["project_id"], projectID)
	}
}

// ============================================================
// 辅助：避免 import 未使用错误
// ============================================================

func init() {
	_ = fmt.Sprintf
	_ = time.Now
}
