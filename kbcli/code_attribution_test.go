package main

import (
	"testing"
)

func TestSummarizeAttributions(t *testing.T) {
	attributions := []CodeAttribution{
		{CommitHash: "aaa", TaskID: "task-1", OurAICodeLines: 10, HumanCodeLines: 5, TotalAddedLines: 15},
		{CommitHash: "bbb", TaskID: "task-2", OurAICodeLines: 20, HumanCodeLines: 3, TotalAddedLines: 23},
	}

	summary := SummarizeAttributions(attributions)

	if summary.TotalOurAILines != 30 {
		t.Errorf("TotalOurAILines: want 30, got %d", summary.TotalOurAILines)
	}
	if summary.TotalHumanLines != 8 {
		t.Errorf("TotalHumanLines: want 8, got %d", summary.TotalHumanLines)
	}
	if summary.CommitCount != 2 {
		t.Errorf("CommitCount: want 2, got %d", summary.CommitCount)
	}
	if len(summary.Details) != 2 {
		t.Errorf("Details length: want 2, got %d", len(summary.Details))
	}
}

func TestSummarizeAttributions_Empty(t *testing.T) {
	var empty []CodeAttribution
	summary := SummarizeAttributions(empty)

	if summary.TotalOurAILines != 0 {
		t.Errorf("TotalOurAILines: want 0, got %d", summary.TotalOurAILines)
	}
	if summary.TotalHumanLines != 0 {
		t.Errorf("TotalHumanLines: want 0, got %d", summary.TotalHumanLines)
	}
	if summary.CommitCount != 0 {
		t.Errorf("CommitCount: want 0, got %d", summary.CommitCount)
	}
}
