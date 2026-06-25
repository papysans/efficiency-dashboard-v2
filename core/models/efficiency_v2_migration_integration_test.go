//go:build integration

package models

import (
	"database/sql"
	"os"
	"strings"
	"testing"
)

func TestEfficiencyV2AutoMigrateCreatesTablesColumnsAndIndexes(t *testing.T) {
	db, err := OpenGormDB(efficiencyV2MigrationTestDSN())
	if err != nil {
		t.Fatalf("open gorm db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	defer sqlDB.Close()

	assertEfficiencyV2Tables(t, sqlDB)
	assertEfficiencyV2Columns(t, sqlDB)
	assertEfficiencyV2Indexes(t, sqlDB)
	assertEfficiencyV2JSONDefaults(t, sqlDB)
}

func efficiencyV2MigrationTestDSN() string {
	if dsn := os.Getenv("EFFICIENCY_V2_TEST_DSN"); dsn != "" {
		return dsn
	}
	return "host=127.0.0.1 port=5432 user=postgres password=1 dbname=costrict_stat sslmode=disable"
}

func assertEfficiencyV2Tables(t *testing.T, db *sql.DB) {
	t.Helper()
	tables := []string{
		"conversation_events",
		"session_stage_metrics",
		"needs",
		"need_emp_attribution",
		"user_productivity_v2",
		"anchor_set",
		"baseline_coefficients",
		"baseline_fusion_weights",
	}
	for _, table := range tables {
		var exists bool
		err := db.QueryRow(`
			SELECT EXISTS(
				SELECT 1 FROM information_schema.tables
				WHERE table_schema = 'public' AND table_name = $1
			)`, table).Scan(&exists)
		if err != nil {
			t.Fatalf("check table %s: %v", table, err)
		}
		if !exists {
			t.Errorf("missing v2 table: %s", table)
		}
	}
}

func assertEfficiencyV2Columns(t *testing.T, db *sql.DB) {
	t.Helper()
	expected := map[string]map[string]string{
		"commits": {
			"touched_files": "jsonb",
		},
		"conversation_events": {
			"event_id":       "character varying",
			"session_id":     "character varying",
			"request_id":     "character varying",
			"task_id":        "character varying",
			"user_id":        "character varying",
			"repo_addr":      "text",
			"repo_branch":    "character varying",
			"work_dir_id":    "character varying",
			"event_start_ts": "timestamp with time zone",
			"event_end_ts":   "timestamp with time zone",
			"duration_sec":   "bigint",
			"event_kind":     "character varying",
			"tool_name":      "character varying",
			"command_text":   "text",
			"touched_files":  "jsonb",
			"payload":        "jsonb",
			"source":         "character varying",
			"parse_quality":  "character varying",
			"created_at":     "timestamp with time zone",
			"updated_at":     "timestamp with time zone",
		},
		"session_stage_metrics": {
			"session_id":           "character varying",
			"need_id":              "character varying",
			"user_id":              "character varying",
			"repo_addr":            "text",
			"repo_branch":          "character varying",
			"work_dir_id":          "character varying",
			"session_start_ts":     "timestamp with time zone",
			"session_end_ts":       "timestamp with time zone",
			"first_edit_ts":        "timestamp with time zone",
			"last_edit_ts":         "timestamp with time zone",
			"total_active_min":     "double precision",
			"total_wall_min":       "double precision",
			"think_active_min":     "double precision",
			"think_wall_min":       "double precision",
			"exec_active_min":      "double precision",
			"verify_active_min":    "double precision",
			"other_active_min":     "double precision",
			"exec_wall_min":        "double precision",
			"verify_wall_min":      "double precision",
			"other_wall_min":       "double precision",
			"message_event_count":  "bigint",
			"read_event_count":     "bigint",
			"edit_event_count":     "bigint",
			"verify_event_count":   "bigint",
			"other_event_count":    "bigint",
			"degraded_event_count": "bigint",
			"event_kind_counts":    "jsonb",
			"ai_token_ratio":       "double precision",
			"re_prompt_count":      "bigint",
			"revert_count":         "bigint",
			"compaction_count":     "bigint",
			"total_cost_usd":       "double precision",
			"stage_confidence":     "character varying",
			"confidence_reason":    "text",
			"summary":              "text",
			"summary_source":       "character varying",
			"created_at":           "timestamp with time zone",
			"updated_at":           "timestamp with time zone",
		},
		"needs": {
			"need_id":                             "character varying",
			"boundary_source":                     "character varying",
			"boundary_confidence":                 "character varying",
			"boundary_key":                        "character varying",
			"boundary_evidence":                   "jsonb",
			"status":                              "character varying",
			"repo_addr":                           "text",
			"repo_branch":                         "character varying",
			"primary_user_id":                     "character varying",
			"session_ids":                         "jsonb",
			"commit_ids":                          "jsonb",
			"contributor_user_ids":                "jsonb",
			"touched_files":                       "jsonb",
			"dev_start_ts":                        "timestamp with time zone",
			"dev_end_ts":                          "timestamp with time zone",
			"dev_duration_min":                    "double precision",
			"total_session_active_person_min":     "double precision",
			"estimate_uncovered_human_min":        "double precision",
			"total_active_work_corrected_min":     "double precision",
			"total_wall_min":                      "double precision",
			"total_calendar_min":                  "double precision",
			"total_think_min":                     "double precision",
			"total_exec_min":                      "double precision",
			"total_verify_min":                    "double precision",
			"total_other_min":                     "double precision",
			"commit_count":                        "bigint",
			"total_loc_net":                       "bigint",
			"total_files_touched":                 "bigint",
			"ai_covered_loc":                      "bigint",
			"uncovered_commit_ids":                "jsonb",
			"uncovered_loc":                       "bigint",
			"uncovered_human_min":                 "double precision",
			"uncovered_work_ratio":                "double precision",
			"ai_code_ratio":                       "double precision",
			"silica":                              "double precision",
			"churn_ratio":                         "double precision",
			"duplication_ratio":                   "double precision",
			"revert_count":                        "bigint",
			"revert_rate":                         "double precision",
			"post_generation_deletion_ratio":      "double precision",
			"quality_signals":                     "jsonb",
			"confidence_signals":                  "jsonb",
			"baseline_algo_think_work_min":        "double precision",
			"baseline_algo_execution_work_min":    "double precision",
			"baseline_algo_verification_work_min": "double precision",
			"baseline_algo_total_work_min":        "double precision",
			"baseline_anchor_knn_work_min":        "double precision",
			"baseline_anchor_knn_reason":          "text",
			"baseline_llm_think_work_min":         "double precision",
			"baseline_llm_execution_work_min":     "double precision",
			"baseline_llm_verification_work_min":  "double precision",
			"baseline_llm_total_work_min":         "double precision",
			"baseline_llm_confidence":             "character varying",
			"baseline_llm_reason":                 "text",
			"baseline_fused_work_min":             "double precision",
			"baseline_spread_work_min":            "double precision",
			"baseline_calendar_min":               "double precision",
			"team_work_density_used":              "double precision",
			"team_profile_used":                   "character varying",
			"efficiency_ratio":                    "double precision",
			"efficiency_band_low":                 "double precision",
			"efficiency_band_high":                "double precision",
			"work_efficiency_ratio":               "double precision",
			"confidence_level":                    "character varying",
			"outlier_flag":                        "boolean",
			"coverage_eligible":                   "boolean",
			"feature_dependency_risk":             "character varying",
			"silica_signal":                       "character varying",
			"ai_code_ratio_signal":                "character varying",
			"uncovered_work_signal":               "character varying",
			"reason":                              "text",
			"created_at":                          "timestamp with time zone",
			"updated_at":                          "timestamp with time zone",
		},
		"user_productivity_v2": {
			"user_productivity_v2_id":          "character varying",
			"week_start":                       "date",
			"user_id":                          "character varying",
			"user_name":                        "character varying",
			"need_ids":                         "jsonb",
			"merged_need_count":                "bigint",
			"active_need_count":                "bigint",
			"abandoned_need_count":             "bigint",
			"actual_calendar_min":              "double precision",
			"baseline_calendar_min":            "double precision",
			"efficiency_ratio":                 "double precision",
			"actual_active_work_corrected_min": "double precision",
			"baseline_fused_work_min":          "double precision",
			"work_efficiency_ratio":            "double precision",
			"coverage_high_confidence":         "double precision",
			"coverage_medium":                  "double precision",
			"coverage_low_unreported":          "double precision",
			"coverage_abandoned":               "double precision",
			"coverage_active":                  "double precision",
			"confidence_limited":               "boolean",
			"confidence_reason":                "text",
			"created_at":                       "timestamp with time zone",
			"updated_at":                       "timestamp with time zone",
		},
		"anchor_set": {
			"anchor_id":             "character varying",
			"source":                "character varying",
			"source_version":        "character varying",
			"anchor_kind":           "character varying",
			"human_labeled_minutes": "double precision",
			"without_ai_minutes":    "double precision",
			"human_labeled":         "boolean",
			"weight":                "double precision",
			"feature_vector":        "jsonb",
			"labels":                "jsonb",
			"valid_from":            "timestamp with time zone",
			"valid_to":              "timestamp with time zone",
			"created_at":            "timestamp with time zone",
			"updated_at":            "timestamp with time zone",
		},
		"baseline_coefficients": {
			"coef_version":   "character varying",
			"created_ts":     "timestamp with time zone",
			"algo":           "jsonb",
			"metadata":       "jsonb",
			"effective_from": "timestamp with time zone",
			"effective_to":   "timestamp with time zone",
			"source":         "character varying",
			"created_at":     "timestamp with time zone",
			"updated_at":     "timestamp with time zone",
		},
		"baseline_fusion_weights": {
			"fusion_weight_id":   "uuid",
			"team_id":            "character varying",
			"snapshot_ts":        "timestamp with time zone",
			"week_start":         "date",
			"weight_algo":        "double precision",
			"weight_knn":         "double precision",
			"weight_llm":         "double precision",
			"mad_algo":           "double precision",
			"mad_knn":            "double precision",
			"mad_llm":            "double precision",
			"team_work_density":  "double precision",
			"density_source":     "character varying",
			"cold_start_default": "boolean",
			"hold_out_mae_algo":  "double precision",
			"hold_out_mae_knn":   "double precision",
			"hold_out_mae_llm":   "double precision",
			"sample_count":       "bigint",
			"reason":             "text",
			"metadata":           "jsonb",
			"created_at":         "timestamp with time zone",
			"updated_at":         "timestamp with time zone",
		},
	}

	for table, columns := range expected {
		for column, expectedType := range columns {
			assertColumnType(t, db, table, column, expectedType)
		}
	}
}

func assertEfficiencyV2Indexes(t *testing.T, db *sql.DB) {
	t.Helper()
	type expectedIndex struct {
		table    string
		name     string
		unique   bool
		contains []string
	}
	expected := []expectedIndex{
		{"conversation_events", "ux_conversation_events_logical", true, []string{"session_id", "request_id", "event_start_ts", "event_kind", "source", "COALESCE(tool_name"}},
		{"conversation_events", "idx_conversation_events_session_start", false, []string{"session_id", "event_start_ts"}},
		// WS-B 索引瘦身：task_start/source_quality 及 8 个单列二级索引已删，仅保留 event_start_ts（供保留删除）。
		{"conversation_events", "idx_conversation_events_event_start_ts", false, []string{"event_start_ts"}},
		{"session_stage_metrics", "idx_session_stage_metrics_user_start", false, []string{"user_id", "session_start_ts"}},
		{"session_stage_metrics", "idx_session_stage_metrics_confidence", false, []string{"stage_confidence"}},
		{"needs", "ux_needs_boundary_key", true, []string{"boundary_source", "boundary_key"}},
		{"needs", "idx_needs_repo_branch", false, []string{"repo_addr", "repo_branch"}},
		{"needs", "idx_needs_status_confidence", false, []string{"status", "boundary_confidence"}},
		{"needs", "idx_needs_primary_user_status", false, []string{"primary_user_id", "status"}},
		{"needs", "idx_needs_dev_end_ts", false, []string{"dev_end_ts"}},
		{"needs", "idx_needs_outlier_flag", false, []string{"outlier_flag"}},
		{"user_productivity_v2", "ux_user_productivity_v2_user_week", true, []string{"user_id", "week_start"}},
		{"user_productivity_v2", "idx_user_productivity_v2_week_start", false, []string{"week_start"}},
		{"user_productivity_v2", "idx_user_productivity_v2_confidence_limited", false, []string{"confidence_limited"}},
		{"anchor_set", "ux_anchor_set_source_anchor", true, []string{"source", "source_version", "anchor_id"}},
		{"anchor_set", "idx_anchor_set_source", false, []string{"source"}},
		{"anchor_set", "idx_anchor_set_valid_from", false, []string{"valid_from"}},
		{"baseline_coefficients", "ux_baseline_coefficients_coef_version", true, []string{"coef_version"}},
		{"baseline_coefficients", "idx_baseline_coefficients_effective_from", false, []string{"effective_from"}},
		{"baseline_fusion_weights", "ux_baseline_fusion_weights_team_snapshot", true, []string{"team_id", "snapshot_ts"}},
		{"baseline_fusion_weights", "idx_baseline_fusion_weights_snapshot_ts", false, []string{"snapshot_ts"}},
		{"baseline_fusion_weights", "idx_baseline_fusion_weights_week_start", false, []string{"week_start"}},
	}
	for _, item := range expected {
		assertIndexDefinition(t, db, item.table, item.name, item.unique, item.contains)
	}
}

func assertEfficiencyV2JSONDefaults(t *testing.T, db *sql.DB) {
	t.Helper()
	defaults := []struct {
		table    string
		column   string
		fragment string
	}{
		{"conversation_events", "touched_files", "'[]'::jsonb"},
		{"commits", "touched_files", "'[]'::jsonb"},
		{"conversation_events", "payload", "'{}'::jsonb"},
		{"session_stage_metrics", "event_kind_counts", "'{}'::jsonb"},
		{"needs", "boundary_evidence", "'{}'::jsonb"},
		{"needs", "contributor_user_ids", "'[]'::jsonb"},
		{"needs", "session_ids", "'[]'::jsonb"},
		{"needs", "commit_ids", "'[]'::jsonb"},
		{"needs", "touched_files", "'[]'::jsonb"},
		{"needs", "uncovered_commit_ids", "'[]'::jsonb"},
		{"needs", "quality_signals", "'{}'::jsonb"},
		{"needs", "confidence_signals", "'{}'::jsonb"},
		{"user_productivity_v2", "need_ids", "'[]'::jsonb"},
		{"anchor_set", "feature_vector", "'{}'::jsonb"},
		{"anchor_set", "labels", "'{}'::jsonb"},
		{"baseline_coefficients", "algo", "'{}'::jsonb"},
		{"baseline_coefficients", "metadata", "'{}'::jsonb"},
		{"baseline_fusion_weights", "metadata", "'{}'::jsonb"},
	}
	for _, item := range defaults {
		var columnDefault sql.NullString
		err := db.QueryRow(`
			SELECT column_default FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = $1 AND column_name = $2`,
			item.table, item.column).Scan(&columnDefault)
		if err != nil {
			t.Fatalf("check default %s.%s: %v", item.table, item.column, err)
		}
		if !columnDefault.Valid || !strings.Contains(columnDefault.String, item.fragment) {
			t.Errorf("%s.%s default = %q, want containing %q", item.table, item.column, columnDefault.String, item.fragment)
		}
	}
}

func assertColumnType(t *testing.T, db *sql.DB, table, column, expectedType string) {
	t.Helper()
	var dataType string
	err := db.QueryRow(`
		SELECT data_type FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = $1 AND column_name = $2`,
		table, column).Scan(&dataType)
	if err == sql.ErrNoRows {
		t.Errorf("%s missing column: %s", table, column)
		return
	}
	if err != nil {
		t.Errorf("check column %s.%s: %v", table, column, err)
		return
	}
	if dataType != expectedType {
		t.Errorf("%s.%s type = %s, want %s", table, column, dataType, expectedType)
	}
}

func assertIndexDefinition(t *testing.T, db *sql.DB, table, index string, unique bool, contains []string) {
	t.Helper()
	var indexDef string
	err := db.QueryRow(`
		SELECT indexdef
		FROM pg_indexes
		WHERE schemaname = 'public' AND tablename = $1 AND indexname = $2`,
		table, index).Scan(&indexDef)
	if err == sql.ErrNoRows {
		t.Errorf("%s missing index: %s", table, index)
		return
	}
	if err != nil {
		t.Errorf("check index %s.%s: %v", table, index, err)
		return
	}
	if unique && !strings.Contains(indexDef, "CREATE UNIQUE INDEX") {
		t.Errorf("%s.%s indexdef = %q, want unique", table, index, indexDef)
	}
	if !unique && !strings.Contains(indexDef, "CREATE INDEX") {
		t.Errorf("%s.%s indexdef = %q, want non-unique index", table, index, indexDef)
	}
	for _, fragment := range contains {
		if !strings.Contains(indexDef, fragment) {
			t.Errorf("%s.%s indexdef = %q, want containing %q", table, index, indexDef, fragment)
		}
	}
}
