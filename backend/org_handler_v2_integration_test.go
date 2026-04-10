//go:build integration

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// ============================================================
// 测试点 5: GET /api/v2/group 返回正确结构
// 测试点 8: GET /api/v2/group 日期参数过滤
// ============================================================

func setupGroupDetailTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	tdb := testDB(t)
	statDB = tdb

	// 初始化 orgMappings（若为空则设为空 map，避免 nil panic）
	if orgMappings == nil {
		orgMappings = make(map[string]*OrgMapping)
	}

	r := gin.New()
	v2 := r.Group("/api/v2")
	v2.GET("/group", getGroupDetailV2)
	return r
}

func TestGetGroupDetailV2_Structure(t *testing.T) {
	r := setupGroupDetailTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/group", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 HTTP 200，实际 %d，body: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应 JSON 失败: %v", err)
	}

	for _, field := range []string{"org_path", "summary", "daily", "members"} {
		if _, exists := resp[field]; !exists {
			t.Errorf("响应缺少字段 %s", field)
		}
	}
}

func TestGetGroupDetailV2_NoMatchReturnsEmpty(t *testing.T) {
	r := setupGroupDetailTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/group?org1=__不存在的组织__", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 HTTP 200，实际 %d，body: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应 JSON 失败: %v", err)
	}

	if resp["org_path"] != "__不存在的组织__" {
		t.Errorf("org_path 期望 '__不存在的组织__'，实际 '%v'", resp["org_path"])
	}

	daily, ok := resp["daily"].([]interface{})
	if !ok {
		t.Errorf("daily 应为数组，实际类型: %T", resp["daily"])
	} else if len(daily) != 0 {
		t.Errorf("无匹配用户时 daily 应为空数组，实际长度 %d", len(daily))
	}

	members, ok := resp["members"].([]interface{})
	if !ok {
		t.Errorf("members 应为数组，实际类型: %T", resp["members"])
	} else if len(members) != 0 {
		t.Errorf("无匹配用户时 members 应为空数组，实际长度 %d", len(members))
	}
}

func TestGetGroupDetailV2_InvalidDate(t *testing.T) {
	r := setupGroupDetailTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/group?startDate=invalid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("非法日期格式期望 HTTP 400，实际 %d，body: %s", w.Code, w.Body.String())
	}
}

func TestGetGroupDetailV2_ValidDateRange(t *testing.T) {
	r := setupGroupDetailTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/group?startDate=20250101&endDate=20250131", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("合法日期范围期望 HTTP 200，实际 %d，body: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应 JSON 失败: %v", err)
	}
	// 验证基本结构存在
	for _, field := range []string{"org_path", "summary", "daily", "members"} {
		if _, exists := resp[field]; !exists {
			t.Errorf("响应缺少字段 %s", field)
		}
	}
}
