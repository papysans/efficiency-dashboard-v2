//go:build integration

package main

import (
	"fmt"
	"os"
	"testing"

	"kanban/core/config"
	"kanban/core/models"

	"gorm.io/gorm"
)

func TestSeedEfficiencyV2Fixture_PopulatesTestDatabase(t *testing.T) {
	db, err := models.OpenGormDB(efficiencyV2TestDSN())
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	defer sqlDB.Close()

	fixture := BuildEfficiencyV2Fixture()
	if err := SeedEfficiencyV2Fixture(db, fixture); err != nil {
		t.Fatalf("seed fixture: %v", err)
	}
	if err := SeedEfficiencyV2Fixture(db, fixture); err != nil {
		t.Fatalf("seed fixture should be idempotent: %v", err)
	}

	assertFixtureUsersExist(t, db, fixture)
	assertFixtureSessionsExist(t, db, fixture)
	assertFixtureTasksExist(t, db, fixture)
	assertFixtureConversationsExist(t, db, fixture)
	assertFixtureCommitsExist(t, db, fixture)
	assertFixtureManifestExists(t, db, fixture)
}

func efficiencyV2TestDSN() string {
	if dsn := os.Getenv("EFFICIENCY_V2_TEST_DSN"); dsn != "" {
		return dsn
	}
	return config.DatabaseConfig{
		Host:     "127.0.0.1",
		Port:     5432,
		User:     "postgres",
		Password: "1",
		DBName:   "costrict_stat",
		SSLMode:  "disable",
	}.DSN()
}

func assertFixtureUsersExist(t *testing.T, db *gorm.DB, fixture EfficiencyV2Fixture) {
	t.Helper()
	seen := map[string]bool{}
	for _, scenario := range fixture.Scenarios {
		for _, userID := range scenario.ContributorUserIDs {
			if seen[userID] {
				continue
			}
			seen[userID] = true
			assertOneRow(t, db, "user_org", "user_id = ?", userID)
		}
	}
}

func assertFixtureSessionsExist(t *testing.T, db *gorm.DB, fixture EfficiencyV2Fixture) {
	t.Helper()
	for _, scenario := range fixture.Scenarios {
		for _, sessionID := range scenario.SessionIDs {
			assertOneRow(t, db, "sessions", "session_id = ?", sessionID)
		}
	}
}

func assertFixtureTasksExist(t *testing.T, db *gorm.DB, fixture EfficiencyV2Fixture) {
	t.Helper()
	for _, scenario := range fixture.Scenarios {
		for _, sessionID := range scenario.SessionIDs {
			assertOneRow(t, db, "tasks", "task_id = ?", "task-"+sessionID)
		}
	}
}

func assertFixtureConversationsExist(t *testing.T, db *gorm.DB, fixture EfficiencyV2Fixture) {
	t.Helper()
	for _, scenario := range fixture.Scenarios {
		for _, sessionID := range scenario.SessionIDs {
			assertOneRow(t, db, "conversations", "session_id = ? AND request_id = ?", sessionID, "r1")
		}
	}
}

func assertFixtureCommitsExist(t *testing.T, db *gorm.DB, fixture EfficiencyV2Fixture) {
	t.Helper()
	for _, scenario := range fixture.Scenarios {
		for _, commitID := range scenario.CommitIDs {
			assertOneRow(t, db, "commits", "commit_id = ?", commitID)
		}
	}
}

func assertFixtureManifestExists(t *testing.T, db *gorm.DB, fixture EfficiencyV2Fixture) {
	t.Helper()
	records, err := BuildEfficiencyV2FixtureManifest(fixture)
	if err != nil {
		t.Fatalf("build manifest: %v", err)
	}
	for _, record := range records {
		assertOneRow(t, db, efficiencyV2FixtureManifestTable, "kind = ? AND name = ?", record.Kind, record.Name)
	}
}

func assertOneRow(t *testing.T, db *gorm.DB, table, predicate string, args ...interface{}) {
	t.Helper()
	var count int64
	if err := db.Raw(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", table, predicate), args...).Scan(&count).Error; err != nil {
		t.Fatalf("count table %s: %v", table, err)
	}
	if count != 1 {
		t.Fatalf("table %s predicate %q count = %d, want 1", table, predicate, count)
	}
}
