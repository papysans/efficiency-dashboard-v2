package main

import (
	"encoding/json"
	"testing"
)

func TestSeverityRank(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"error severity", "error", 3},
		{"warn severity", "warn", 2},
		{"info severity", "info", 1},
		{"empty string", "", 0},
		{"unknown severity", "unknown", 0},
		{"random string", "fatal", 0},
		{"uppercase error", "ERROR", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := severityRank(tt.input)
			if got != tt.expected {
				t.Errorf("severityRank(%q) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}

func TestMatchDateFilter(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		dateFilter string
		expected   bool
	}{
		{"empty filter always matches", "/summary/2024/01/15/task.json", "", true},
		{"slash path matches", "/summary/2024/01/15/task.json", "20240115", true},
		{"backslash path matches", "\\summary\\2024\\01\\15\\task.json", "20240115", true},
		{"mixed path matches", "/summary/2024/01/15/task.json", "20240115", true},
		{"path without matching date", "/summary/2024/01/16/task.json", "20240115", false},
		{"wrong date in path", "/summary/2024/02/15/task.json", "20240115", false},
		{"date in filename not path", "/summary/20240115/task.json", "20240115", false},
		{"partial date match rejected", "/summary/2024/01/150/task.json", "20240115", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchDateFilter(tt.path, tt.dateFilter)
			if got != tt.expected {
				t.Errorf("matchDateFilter(%q, %q) = %v, want %v", tt.path, tt.dateFilter, got, tt.expected)
			}
		})
	}
}

func TestIsDiffLinesMismatch(t *testing.T) {
	tests := []struct {
		name     string
		expected int
		actual   int
		want     bool
	}{
		{"expected zero", 0, 5, false},
		{"expected negative", -1, 5, false},
		{"expected le 5 diff ge 3", 5, 1, true},
		{"expected le 5 diff lt 3", 5, 3, false},
		{"expected le 5 exact", 5, 5, false},
		{"expected le 20 diff ge 5", 20, 14, true},
		{"expected le 20 diff ge 30pct", 10, 6, true},
		{"expected le 20 diff lt 5 and lt 30pct", 20, 18, false},
		{"expected gt 20 diff gt 100", 300, 100, true},
		{"expected gt 20 diff le 100 but ratio gt 50pct", 60, 20, true},
		{"expected 300 actual 200 diff 100 ratio 0.33", 300, 200, false},
		{"expected 300 actual 100 diff 200 ratio 0.67", 300, 100, true},
		{"expected gt 20 diff exactly 100", 300, 200, false},
		{"expected gt 20 diff 101", 300, 199, true},
		{"small expected exact match", 3, 3, false},
		{"small expected diff 2", 3, 1, false},
		{"small expected diff 3", 3, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDiffLinesMismatch(tt.expected, tt.actual)
			if got != tt.want {
				t.Errorf("isDiffLinesMismatch(%d, %d) = %v, want %v", tt.expected, tt.actual, got, tt.want)
			}
		})
	}
}

func TestCountDiffLines(t *testing.T) {
	tests := []struct {
		name     string
		diffText string
		expected int
	}{
		{"empty string", "", 0},
		{"whitespace only", "   \n\t  ", 0},
		{"json diff with additions and deletions", func() string {
			entries := []diffJSONEntry{
				{Additions: 3, Deletions: 2},
				{Additions: 1, Deletions: 4},
			}
			b, _ := json.Marshal(entries)
			return string(b)
		}(), 10},
		{"json diff single entry", func() string {
			entries := []diffJSONEntry{{Additions: 5, Deletions: 3}}
			b, _ := json.Marshal(entries)
			return string(b)
		}(), 8},
		{"unified diff with additions and deletions", "@@ -1,3 +1,3 @@\n-old1\n+new1\n context\n-old2\n+new2", 4},
		{"unified diff ignores +++ and ---", "--- a/file.go\n+++ b/file.go\n@@ -1,1 +1,1 @@\n-old\n+new", 2},
		{"unified diff ignores empty +/- lines", "-\n+\n-old\n+new", 2},
		{"unified diff ignores whitespace only +/- lines", "-  \n+  \n-old\n+new", 2},
		{"before after format", "<<< BEFORE\nold line 1\nold line 2\n>>> AFTER\nnew line 1\nnew line 2\n--- file.go", 4},
		{"before after with duplicate lines", "<<< BEFORE\ncommon\nold only\n>>> AFTER\ncommon\nnew only\n--- file.go", 2},
		{"unrecognized format treated as unified", "some text\n-added\n-removed\nmore text", 2},
		{"unified diff with no changes", "@@ -1,3 +1,3 @@\n line1\n line2\n line3", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countDiffLines(tt.diffText)
			if got != tt.expected {
				t.Errorf("countDiffLines(...) = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestCountContentDiffLines(t *testing.T) {
	tests := []struct {
		name          string
		before        string
		after         string
		wantAdditions int
		wantDeletions int
	}{
		{"both empty", "", "", 0, 0},
		{"empty before empty after with whitespace", "  \n\t  ", "  \n  ", 0, 0},
		{"before has lines not in after", "line1\nline2\nline3", "", 0, 3},
		{"after has lines not in before", "", "line1\nline2", 2, 0},
		{"mixed additions and deletions", "old1\ncommon\nold2", "new1\ncommon\nnew2", 2, 2},
		{"empty lines ignored in before", "line1\n\n\t  \nline2", "line1\nline2", 0, 0},
		{"empty lines ignored in after", "line1\nline2", "line1\n\n\t  \nline2", 0, 0},
		{"duplicates treated as set", "line1\nline1\nline1", "line1", 0, 0},
		{"trimmed comparison", "  line1  ", "line1", 0, 0},
		{"all common lines", "a\nb\nc", "a\nb\nc", 0, 0},
		{"completely different", "a\nb", "c\nd", 2, 2},
		{"single addition", "a", "a\nb", 1, 0},
		{"single deletion", "a\nb", "a", 0, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAdditions, gotDeletions := countContentDiffLines(tt.before, tt.after)
			if gotAdditions != tt.wantAdditions || gotDeletions != tt.wantDeletions {
				t.Errorf("countContentDiffLines(%q, %q) = (%d, %d), want (%d, %d)",
					tt.before, tt.after, gotAdditions, gotDeletions, tt.wantAdditions, tt.wantDeletions)
			}
		})
	}
}
