package main

import (
	"testing"
	"time"
)

func TestParseDateRange(t *testing.T) {
	mustParse := func(s string) time.Time {
		v, err := time.Parse("20060102", s)
		if err != nil {
			t.Fatalf("test setup: invalid date %q: %v", s, err)
		}
		return v
	}

	tests := []struct {
		name            string
		startDateStr    string
		endDateStr      string
		dateStr         string
		wantStartDate   *time.Time
		wantEndDate     *time.Time
		wantErr         bool
		wantErrContains string
	}{
		{
			name:          "all empty",
			startDateStr:  "",
			endDateStr:    "",
			dateStr:       "",
			wantStartDate: nil,
			wantEndDate:   nil,
			wantErr:       false,
		},
		{
			name:          "dateStr set ignores startDateStr and endDateStr",
			startDateStr:  "20260101",
			endDateStr:    "20260110",
			dateStr:       "20260105",
			wantStartDate: datePtr(mustParse("20260105")),
			wantEndDate:   datePtr(mustParse("20260106")),
			wantErr:       false,
		},
		{
			name:          "only startDateStr set",
			startDateStr:  "20260315",
			endDateStr:    "",
			dateStr:       "",
			wantStartDate: datePtr(mustParse("20260315")),
			wantEndDate:   nil,
			wantErr:       false,
		},
		{
			name:          "only endDateStr set",
			startDateStr:  "",
			endDateStr:    "20260320",
			dateStr:       "",
			wantStartDate: nil,
			wantEndDate:   datePtr(mustParse("20260321")),
			wantErr:       false,
		},
		{
			name:          "both startDateStr and endDateStr set",
			startDateStr:  "20260101",
			endDateStr:    "20261231",
			dateStr:       "",
			wantStartDate: datePtr(mustParse("20260101")),
			wantEndDate:   datePtr(mustParse("20270101")),
			wantErr:       false,
		},
		{
			name:            "invalid startDate format",
			startDateStr:    "2026-01-01",
			endDateStr:      "",
			dateStr:         "",
			wantStartDate:   nil,
			wantEndDate:     nil,
			wantErr:         true,
			wantErrContains: "startDate格式错误",
		},
		{
			name:            "invalid endDate format",
			startDateStr:    "",
			endDateStr:      "01-01-2026",
			dateStr:         "",
			wantStartDate:   nil,
			wantEndDate:     nil,
			wantErr:         true,
			wantErrContains: "endDate格式错误",
		},
		{
			name:            "invalid dateStr format",
			startDateStr:    "",
			endDateStr:      "",
			dateStr:         "2026/01/01",
			wantStartDate:   nil,
			wantEndDate:     nil,
			wantErr:         true,
			wantErrContains: "date格式错误",
		},
		{
			name:          "startDateStr with leading whitespace fails",
			startDateStr:  " 20260101",
			endDateStr:    "",
			dateStr:       "",
			wantStartDate: nil,
			wantEndDate:   nil,
			wantErr:       true,
		},
		{
			name:          "endDateStr with trailing whitespace fails",
			startDateStr:  "",
			endDateStr:    "20260101 ",
			dateStr:       "",
			wantStartDate: nil,
			wantEndDate:   nil,
			wantErr:       true,
		},
		{
			name:          "dateStr with whitespace fails",
			startDateStr:  "",
			endDateStr:    "",
			dateStr:       " 20260101",
			wantStartDate: nil,
			wantEndDate:   nil,
			wantErr:       true,
		},
		{
			name:          "Feb 29 leap year",
			startDateStr:  "20240229",
			endDateStr:    "",
			dateStr:       "",
			wantStartDate: datePtr(mustParse("20240229")),
			wantEndDate:   nil,
			wantErr:       false,
		},
		{
			name:            "Feb 29 non-leap year",
			startDateStr:    "20230229",
			endDateStr:      "",
			dateStr:         "",
			wantStartDate:   nil,
			wantEndDate:     nil,
			wantErr:         true,
			wantErrContains: "startDate格式错误",
		},
		{
			name:          "endDate is next day",
			startDateStr:  "",
			endDateStr:    "20260301",
			dateStr:       "",
			wantStartDate: nil,
			wantEndDate:   datePtr(mustParse("20260302")),
			wantErr:       false,
		},
		{
			name:          "year boundary",
			startDateStr:  "",
			endDateStr:    "20231231",
			dateStr:       "",
			wantStartDate: nil,
			wantEndDate:   datePtr(mustParse("20240101")),
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStart, gotEnd, err := parseDateRange(tt.startDateStr, tt.endDateStr, tt.dateStr)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseDateRange() error = nil, wantErr = true")
				}
				if tt.wantErrContains != "" && !contains(err.Error(), tt.wantErrContains) {
					t.Fatalf("parseDateRange() error = %q, want contains %q", err.Error(), tt.wantErrContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseDateRange() unexpected error = %v", err)
			}
			if !timesEqual(gotStart, tt.wantStartDate) {
				t.Errorf("startDate = %v, want %v", formatTimePtr(gotStart), formatTimePtr(tt.wantStartDate))
			}
			if !timesEqual(gotEnd, tt.wantEndDate) {
				t.Errorf("endDate = %v, want %v", formatTimePtr(gotEnd), formatTimePtr(tt.wantEndDate))
			}
		})
	}
}

func TestIsActiveTimeInRange(t *testing.T) {
	mustParse := func(s string) time.Time {
		v, err := time.Parse("20060102", s)
		if err != nil {
			t.Fatalf("test setup: invalid date %q: %v", s, err)
		}
		return v
	}

	tests := []struct {
		name       string
		activeTime time.Time
		startDate  *time.Time
		endDate    *time.Time
		want       bool
	}{
		{
			name:       "both nil always true",
			activeTime: mustParse("20260115"),
			startDate:  nil,
			endDate:    nil,
			want:       true,
		},
		{
			name:       "only startDate set, activeTime after start",
			activeTime: mustParse("20260115"),
			startDate:  datePtr(mustParse("20260101")),
			endDate:    nil,
			want:       true,
		},
		{
			name:       "only startDate set, activeTime before start",
			activeTime: mustParse("20251231"),
			startDate:  datePtr(mustParse("20260101")),
			endDate:    nil,
			want:       false,
		},
		{
			name:       "only startDate set, activeTime exactly at start",
			activeTime: mustParse("20260101"),
			startDate:  datePtr(mustParse("20260101")),
			endDate:    nil,
			want:       true,
		},
		{
			name:       "only endDate set, activeTime before end",
			activeTime: mustParse("20260110"),
			startDate:  nil,
			endDate:    datePtr(mustParse("20260115")),
			want:       true,
		},
		{
			name:       "only endDate set, activeTime exactly at end boundary",
			activeTime: mustParse("20260115"),
			startDate:  nil,
			endDate:    datePtr(mustParse("20260115")),
			want:       false,
		},
		{
			name:       "only endDate set, activeTime after end",
			activeTime: mustParse("20260116"),
			startDate:  nil,
			endDate:    datePtr(mustParse("20260115")),
			want:       false,
		},
		{
			name:       "both set, activeTime in range",
			activeTime: mustParse("20260110"),
			startDate:  datePtr(mustParse("20260101")),
			endDate:    datePtr(mustParse("20260115")),
			want:       true,
		},
		{
			name:       "both set, activeTime exactly at startDate boundary",
			activeTime: mustParse("20260101"),
			startDate:  datePtr(mustParse("20260101")),
			endDate:    datePtr(mustParse("20260115")),
			want:       true,
		},
		{
			name:       "both set, activeTime exactly at endDate boundary",
			activeTime: mustParse("20260115"),
			startDate:  datePtr(mustParse("20260101")),
			endDate:    datePtr(mustParse("20260115")),
			want:       false,
		},
		{
			name:       "both set, activeTime before startDate",
			activeTime: mustParse("20251231"),
			startDate:  datePtr(mustParse("20260101")),
			endDate:    datePtr(mustParse("20260115")),
			want:       false,
		},
		{
			name:       "both set, activeTime after endDate",
			activeTime: mustParse("20260116"),
			startDate:  datePtr(mustParse("20260101")),
			endDate:    datePtr(mustParse("20260115")),
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isActiveTimeInRange(tt.activeTime, tt.startDate, tt.endDate)
			if got != tt.want {
				t.Errorf("isActiveTimeInRange(%v, %v, %v) = %v, want %v",
					tt.activeTime, formatTimePtr(tt.startDate), formatTimePtr(tt.endDate), got, tt.want)
			}
		})
	}
}

func datePtr(t time.Time) *time.Time {
	return &t
}

func timesEqual(a, b *time.Time) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Equal(*b)
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return "<nil>"
	}
	return t.Format("2006-01-02 15:04:05")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
