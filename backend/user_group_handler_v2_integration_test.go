//go:build integration

package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// ============================================================
// 测试点 1: user_groups 表包含 org_name 字段
// ============================================================

func TestUserGroupOrgName_DBSchema(t *testing.T) {
	tdb := testDB(t)
	defer tdb.Close()

	var colName, dataType string
	var maxLen int
	err := tdb.QueryRow(`
		SELECT column_name, data_type, character_maximum_length
		FROM information_schema.columns
		WHERE table_name = 'user_groups' AND column_name = 'org_name'
	`).Scan(&colName, &dataType, &maxLen)
	if err != nil {
		t.Fatalf("org_name 字段应存在于 user_groups 表，查询失败: %v", err)
	}
	if colName != "org_name" {
		t.Errorf("column_name 期望 org_name，实际 %s", colName)
	}
	if dataType != "character varying" {
		t.Errorf("data_type 期望 character varying，实际 %s", dataType)
	}
	if maxLen != 200 {
		t.Errorf("character_maximum_length 期望 200，实际 %d", maxLen)
	}
}

// ============================================================
// 测试点 6: POST /api/v2/user-groups 支持 org_name 字段
// ============================================================

func setupUserGroupTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	tdb := testDB(t)
	statDB = tdb

	r := gin.New()
	v2 := r.Group("/api/v2")
	v2.POST("/user-groups", createUserGroupHandler)
	v2.DELETE("/user-groups/:groupId", deleteUserGroupHandler)
	return r
}

func TestCreateUserGroup_WithOrgName(t *testing.T) {
	r := setupUserGroupTestRouter(t)

	body := `{"name":"test-group-orgname","org_name":"技术架构组织","user_ids":["test-uid-001"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v2/user-groups", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 HTTP 200，实际 %d，body: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应 JSON 失败: %v", err)
	}
	if resp["org_name"] != "技术架构组织" {
		t.Errorf("org_name 期望 '技术架构组织'，实际 '%v'", resp["org_name"])
	}

	// 清理
	groupID, _ := resp["group_id"].(string)
	if groupID != "" {
		delReq := httptest.NewRequest(http.MethodDelete, "/api/v2/user-groups/"+groupID, nil)
		delW := httptest.NewRecorder()
		r.ServeHTTP(delW, delReq)
	}
}

func TestCreateUserGroup_WithoutOrgName(t *testing.T) {
	r := setupUserGroupTestRouter(t)

	body := `{"name":"test-group-no-orgname","user_ids":["test-uid-002"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v2/user-groups", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 HTTP 200，实际 %d，body: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应 JSON 失败: %v", err)
	}
	if resp["org_name"] != "" {
		t.Errorf("未传 org_name 时默认应为空字符串，实际 '%v'", resp["org_name"])
	}

	// 清理
	groupID, _ := resp["group_id"].(string)
	if groupID != "" {
		delReq := httptest.NewRequest(http.MethodDelete, "/api/v2/user-groups/"+groupID, nil)
		delW := httptest.NewRecorder()
		r.ServeHTTP(delW, delReq)
	}
}

func TestCreateUserGroup_MissingRequired(t *testing.T) {
	r := setupUserGroupTestRouter(t)

	// 缺少 user_ids
	body := `{"name":"test-group-missing-ids"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v2/user-groups", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("缺少 user_ids 时期望 HTTP 400，实际 %d", w.Code)
	}
}
