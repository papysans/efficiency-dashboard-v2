package main

import (
	"testing"
	"time"

	"kanban/kbcli/internal/appconfig"
)

func withRawIndexTestConfig(t *testing.T) {
	t.Helper()
	old := appconfig.Cfg
	appconfig.Cfg = &appconfig.Config{}
	appconfig.Cfg.RawIndex.Enabled = true
	appconfig.Cfg.RawIndex.S3Base = "s3://user-indicator/raw-dump"
	t.Cleanup(func() { appconfig.Cfg = old })
}

func TestRawIndexConversationPathsChunked(t *testing.T) {
	withRawIndexTestConfig(t)

	got := rawIndexConversationPaths("task/conversation/2026/07/10/sid-1", 3)
	want := []string{
		"s3://user-indicator/raw-dump/task/conversation/2026/07/10/sid-1/000001.jsonl",
		"s3://user-indicator/raw-dump/task/conversation/2026/07/10/sid-1/000002.jsonl",
		"s3://user-indicator/raw-dump/task/conversation/2026/07/10/sid-1/000003.jsonl",
	}
	if len(got) != len(want) {
		t.Fatalf("path count = %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("path[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestRawIndexConversationPathsSingleFileFallback(t *testing.T) {
	withRawIndexTestConfig(t)

	got := rawIndexConversationPaths("task/conversation/2026/07/10/sid-1", 0)
	if len(got) != 1 {
		t.Fatalf("path count = %d, want 1: %v", len(got), got)
	}
	want := "s3://user-indicator/raw-dump/task/conversation/2026/07/10/sid-1.jsonl"
	if got[0] != want {
		t.Fatalf("path = %q, want %q", got[0], want)
	}
}

func TestRawIndexCommitMetaFromS3Key(t *testing.T) {
	withRawIndexTestConfig(t)

	meta := rawIndexCommitMeta(rawIndexCommitRow{
		S3Key:       "repo/repo-safe/branch-safe/2026/07/10/abc123.json",
		RepoAddr:    "https://example.com/repo.git",
		RepoBranch:  "main",
		CommitID:    "abc123",
		CreatedDate: time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC),
	})
	if meta.Path != "s3://user-indicator/raw-dump/repo/repo-safe/branch-safe/2026/07/10/abc123.json" {
		t.Fatalf("Path = %q", meta.Path)
	}
	if meta.RelPath != "repo-safe/branch-safe/2026/07/10/abc123.json" {
		t.Fatalf("RelPath = %q", meta.RelPath)
	}
	if meta.Repo != "repo-safe" || meta.Branch != "branch-safe" || meta.Year != "2026" || meta.Month != "07" || meta.Day != "10" || meta.CommitId != "abc123" {
		t.Fatalf("meta parsed incorrectly: %+v", meta)
	}
}
