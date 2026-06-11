package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"kanban/backend/internal/appconfig"
)

func TestGetConfigV2IncludesDashboardTitlePrefix(t *testing.T) {
	oldCfg := appconfig.Cfg
	defer func() {
		appconfig.Cfg = oldCfg
	}()

	gin.SetMode(gin.TestMode)
	appconfig.Cfg.TraditionalDevLinesPerDay = 123
	appconfig.Cfg.DashboardTitlePrefix = "Costrict"

	router := gin.New()
	router.GET("/api/v2/config", getConfigV2)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/config", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var got ConfigResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.TraditionalDevLinesPerDay != 123 {
		t.Fatalf("traditional_dev_lines_per_day = %d, want 123", got.TraditionalDevLinesPerDay)
	}
	if got.DashboardTitlePrefix != "Costrict" {
		t.Fatalf("dashboard_title_prefix = %q, want %q", got.DashboardTitlePrefix, "Costrict")
	}
}
