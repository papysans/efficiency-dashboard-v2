//go:build integration

package main

import (
	"testing"

	"kanban/core/models"

	"gorm.io/gorm"
)

func TestEfficiencyV2IngestionStages_PopulatesEventsAndStageMetrics(t *testing.T) {
	db := openEfficiencyV2IntegrationDB(t)
	fixture := BuildEfficiencyV2Fixture()
	if err := SeedEfficiencyV2Fixture(db, fixture); err != nil {
		t.Fatalf("seed fixture: %v", err)
	}

	events, err := NormalizeAndUpsertEfficiencyV2ConversationEvents(db, efficiencyV2ConversationEventQuery{
		StartDate: "2026-05-18",
		EndDate:   "2026-05-24",
	})
	if err != nil {
		t.Fatalf("normalize/upsert events: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected normalized events")
	}
	if err := assertEfficiencyV2RowsUnique(t, db, "conversation_events", "event_id"); err != nil {
		t.Fatal(err)
	}
	assertEfficiencyV2EventKinds(t, db)

	cfg := Config{}
	applyEfficiencyV2Defaults(&cfg)
	metrics, err := BuildAndUpsertEfficiencyV2SessionStageMetrics(db, events, cfg.EfficiencyV2)
	if err != nil {
		t.Fatalf("build/upsert stage metrics: %v", err)
	}
	if len(metrics) == 0 {
		t.Fatal("expected stage metrics")
	}
	assertEfficiencyV2StageMetric(t, db, "s-pr-101-a", func(metric models.SessionStageMetric) {
		if metric.UserId != "u-alice" {
			t.Fatalf("user_id = %q, want u-alice", metric.UserId)
		}
		if metric.EditEventCount != 2 || metric.VerifyEventCount != 2 {
			t.Fatalf("edit/verify counts = %d/%d, want 2/2", metric.EditEventCount, metric.VerifyEventCount)
		}
		if metric.ThinkActiveMin <= 0 || metric.ExecutionActiveMin <= 0 || metric.VerificationActiveMin <= 0 {
			t.Fatalf("expected think/exec/verify minutes > 0: %#v", metric)
		}
		if metric.StageConfidence == "" || metric.ConfidenceReason == "" {
			t.Fatalf("confidence fields should be populated: %#v", metric)
		}
	})
	assertEfficiencyV2StageMetric(t, db, "s-cluster-401-a", func(metric models.SessionStageMetric) {
		if metric.EditEventCount != 0 {
			t.Fatalf("no-edit fixture edit count = %d, want 0", metric.EditEventCount)
		}
		if metric.ThinkActiveMin <= 0 || metric.ExecutionActiveMin != 0 || metric.VerificationActiveMin != 0 {
			t.Fatalf("no-edit fixture stage minutes mismatch: %#v", metric)
		}
	})

	eventsAgain, err := NormalizeAndUpsertEfficiencyV2ConversationEvents(db, efficiencyV2ConversationEventQuery{
		StartDate: "2026-05-18",
		EndDate:   "2026-05-24",
	})
	if err != nil {
		t.Fatalf("rerun normalize/upsert events: %v", err)
	}
	if _, err := BuildAndUpsertEfficiencyV2SessionStageMetrics(db, eventsAgain, cfg.EfficiencyV2); err != nil {
		t.Fatalf("rerun build/upsert stage metrics: %v", err)
	}
	if err := assertEfficiencyV2RowsUnique(t, db, "conversation_events", "event_id"); err != nil {
		t.Fatal(err)
	}
	if err := assertEfficiencyV2RowsUnique(t, db, "session_stage_metrics", "session_id"); err != nil {
		t.Fatal(err)
	}
}

func openEfficiencyV2IntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := models.OpenGormDB(efficiencyV2TestDSN())
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func assertEfficiencyV2EventKinds(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, kind := range []string{"message", "edit", "verify"} {
		assertOneRowAtLeast(t, db, "conversation_events", "event_kind = ?", kind)
	}
}

func assertEfficiencyV2StageMetric(t *testing.T, db *gorm.DB, sessionID string, check func(models.SessionStageMetric)) {
	t.Helper()
	var metric models.SessionStageMetric
	if err := db.Where("session_id = ?", sessionID).First(&metric).Error; err != nil {
		t.Fatalf("load stage metric %s: %v", sessionID, err)
	}
	check(metric)
}

func assertEfficiencyV2RowsUnique(t *testing.T, db *gorm.DB, table, column string) error {
	t.Helper()
	var duplicateCount int64
	if err := db.Raw("SELECT COUNT(*) FROM (SELECT " + column + " FROM " + table + " GROUP BY " + column + " HAVING COUNT(*) > 1) d").Scan(&duplicateCount).Error; err != nil {
		return err
	}
	if duplicateCount != 0 {
		t.Fatalf("%s.%s has %d duplicate logical rows", table, column, duplicateCount)
	}
	return nil
}

func assertOneRowAtLeast(t *testing.T, db *gorm.DB, table, predicate string, args ...interface{}) {
	t.Helper()
	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM "+table+" WHERE "+predicate, args...).Scan(&count).Error; err != nil {
		t.Fatalf("count table %s: %v", table, err)
	}
	if count <= 0 {
		t.Fatalf("table %s predicate %q should have rows", table, predicate)
	}
}
