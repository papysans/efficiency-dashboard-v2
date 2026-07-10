package main

import (
	"os"
	"testing"

	"kanban/core/storage"
	"kanban/kbcli/internal/appconfig"
)

func TestRawIndexLiveConnectivity(t *testing.T) {
	if os.Getenv("RAW_INDEX_LIVE") != "1" {
		t.Skip("set RAW_INDEX_LIVE=1 to run live raw index/S3 connectivity checks")
	}

	cfg, err := appconfig.LoadConfig("../configs/kbcli-config.yaml")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	old := appconfig.Cfg
	appconfig.Cfg = cfg
	t.Cleanup(func() { appconfig.Cfg = old })

	if err := storage.Configure(cfg.Storage); err != nil {
		t.Fatalf("configure storage: %v", err)
	}
	if err := storage.ValidateLocations(cfg.TaskDir, cfg.RepoDir); err != nil {
		t.Fatalf("validate S3 bucket: %v", err)
	}

	rawDB, closeRawDB, err := openRawIndexDB()
	if err != nil {
		t.Fatalf("open raw index db: %v", err)
	}
	defer closeRawDB()

	var summaryCount, convCount, commitCount int64
	if err := rawDB.Table("s3_task_summary_index").Count(&summaryCount).Error; err != nil {
		t.Fatalf("count s3_task_summary_index: %v", err)
	}
	if err := rawDB.Table("s3_task_conversation_index").Count(&convCount).Error; err != nil {
		t.Fatalf("count s3_task_conversation_index: %v", err)
	}
	if err := rawDB.Table("s3_commit_index").Count(&commitCount).Error; err != nil {
		t.Fatalf("count s3_commit_index: %v", err)
	}
	t.Logf("raw index counts: summary=%d conversation=%d commit=%d", summaryCount, convCount, commitCount)

	var sample rawIndexTaskSummaryRow
	if err := rawDB.Table("s3_task_summary_index").
		Select("task_id, s3_key, file_size, created_date").
		Order("created_date DESC, id DESC").
		Limit(1).
		Find(&sample).Error; err != nil {
		t.Fatalf("query sample summary: %v", err)
	}
	if sample.S3Key == "" {
		t.Skip("raw index has no task summary sample to check S3 object")
	}
	loc := rawIndexObjectLoc(sample.S3Key)
	if _, err := storage.Stat(loc); err != nil {
		t.Fatalf("stat sample S3 object %s: %v", loc, err)
	}
	t.Logf("sample summary object is readable by HeadObject: %s", loc)
}
