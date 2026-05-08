//go:build integration

package main

import (
	"bytes"
	"encoding/json"
	"kanban/core/models"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// ============================================================
// 测试点 3: GET /api/v2/users 返回 org 相关字段
// ============================================================

func setupUsersV2TestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	tdb := testDB(t)
	statDB = tdb

	// 初始化 orgMappings（若为空则设为空 map，避免 nil panic）
	if orgMappings == nil {
		orgMappings = make(map[string]*models.UserOrg)
	}

	r := gin.New()
	v2 := r.Group("/api/v2")
	v2.GET("/users", listUsersV2)
	v2.POST("/user-groups", createUserGroupHandler)
	v2.DELETE("/user-groups/:groupId", deleteUserGroupHandler)
	return r
}

func TestListUsersV2_OrgFields(t *testing.T) {
	r := setupUsersV2TestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/users?pageSize=10", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 HTTP 200，实际 %d，body: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应 JSON 失败: %v", err)
	}

	data, ok := resp["data"].([]interface{})
	if !ok {
		t.Fatalf("data 应为数组，实际类型: %T", resp["data"])
	}

	// 检查每条记录包含 org 相关字段
	for i, item := range data {
		row, ok := item.(map[string]interface{})
		if !ok {
			t.Errorf("data[%d] 应为对象", i)
			continue
		}
		for _, field := range []string{"org1", "org2", "org3", "org4", "org_display", "is_virtual_group", "org_name"} {
			if _, exists := row[field]; !exists {
				t.Errorf("data[%d] 缺少字段 %s", i, field)
			}
		}
	}
}

// ============================================================
// 测试点 7: GET /api/v2/users 列表中虚拟组数据合并
// ============================================================

func TestListUsersV2_VirtualGroupMerged(t *testing.T) {
	r := setupUsersV2TestRouter(t)

	// 创建测试虚拟组
	createBody := `{"name":"test-vgroup-merge","org_name":"测试虚拟组织","user_ids":["test-uid-vg-001"]}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/v2/user-groups", bytes.NewBufferString(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)

	if createW.Code != http.StatusOK {
		t.Fatalf("创建虚拟组失败: %d, %s", createW.Code, createW.Body.String())
	}
	var createResp map[string]interface{}
	if err := json.Unmarshal(createW.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("解析创建虚拟组响应失败: %v", err)
	}
	groupID, _ := createResp["group_id"].(string)
	defer func() {
		if groupID != "" {
			delReq := httptest.NewRequest(http.MethodDelete, "/api/v2/user-groups/"+groupID, nil)
			delW := httptest.NewRecorder()
			r.ServeHTTP(delW, delReq)
		}
	}()

	// 查询用户列表（大 pageSize 确保虚拟组在结果中）
	req := httptest.NewRequest(http.MethodGet, "/api/v2/users?pageSize=1000", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 HTTP 200，实际 %d，body: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应 JSON 失败: %v", err)
	}

	data, ok := resp["data"].([]interface{})
	if !ok {
		t.Fatalf("data 应为数组")
	}

	var found bool
	for _, item := range data {
		row, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if row["is_virtual_group"] == true && row["user_name"] == "test-vgroup-merge" {
			found = true
			if row["org_display"] != "测试虚拟组织" {
				t.Errorf("虚拟组 org_display 期望 '测试虚拟组织'，实际 '%v'", row["org_display"])
			}
			if row["org_name"] != "测试虚拟组织" {
				t.Errorf("虚拟组 org_name 期望 '测试虚拟组织'，实际 '%v'", row["org_name"])
			}
			for _, orgField := range []string{"org1", "org2", "org3", "org4"} {
				if row[orgField] != "" {
					t.Errorf("虚拟组 %s 期望为空字符串，实际 '%v'", orgField, row[orgField])
				}
			}
		}
	}
	if !found {
		t.Error("用户列表中应包含虚拟组记录 test-vgroup-merge")
	}
}
