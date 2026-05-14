package main

import (
	"math"
	"testing"
)

func TestCalculateCost(t *testing.T) {
	tests := []struct {
		name      string
		model     string
		inTokens  int64
		outTokens int64
		prices    map[string]ModelPrice
		want      float64
	}{
		{
			name:      "exact prefix match",
			model:     "claude-3-5-sonnet",
			inTokens:  1_000_000,
			outTokens: 1_000_000,
			prices: map[string]ModelPrice{
				"claude-3-5-sonnet": {InPrice: 3.0, OutPrice: 15.0},
			},
			want: 18.0,
		},
		{
			name:      "longest prefix match",
			model:     "claude-3-5-sonnet-20241022",
			inTokens:  2_000_000,
			outTokens: 1_000_000,
			prices: map[string]ModelPrice{
				"claude":            {InPrice: 1.0, OutPrice: 2.0},
				"claude-3-5-sonnet": {InPrice: 3.0, OutPrice: 15.0},
			},
			want: 21.0,
		},
		{
			name:      "fallback to default when no prefix matches",
			model:     "gpt-4",
			inTokens:  1_000_000,
			outTokens: 1_000_000,
			prices: map[string]ModelPrice{
				"claude-3-5-sonnet": {InPrice: 3.0, OutPrice: 15.0},
				"default":           {InPrice: 5.0, OutPrice: 10.0},
			},
			want: 15.0,
		},
		{
			name:      "return 0 when no prefix matches and no default",
			model:     "gpt-4",
			inTokens:  1_000_000,
			outTokens: 1_000_000,
			prices: map[string]ModelPrice{
				"claude-3-5-sonnet": {InPrice: 3.0, OutPrice: 15.0},
			},
			want: 0,
		},
		{
			name:      "cost formula with fractional result",
			model:     "test-model",
			inTokens:  500_000,
			outTokens: 250_000,
			prices: map[string]ModelPrice{
				"test-model": {InPrice: 2.0, OutPrice: 8.0},
			},
			want: 3.0,
		},
		{
			name:      "case insensitive model matching uppercase",
			model:     "CLAUDE-3-5-SONNET",
			inTokens:  1_000_000,
			outTokens: 0,
			prices: map[string]ModelPrice{
				"claude-3-5-sonnet": {InPrice: 3.0, OutPrice: 15.0},
			},
			want: 3.0,
		},
		{
			name:      "case insensitive model matching mixed case",
			model:     "Claude-3-5-Sonnet",
			inTokens:  0,
			outTokens: 1_000_000,
			prices: map[string]ModelPrice{
				"claude-3-5-sonnet": {InPrice: 3.0, OutPrice: 15.0},
			},
			want: 15.0,
		},
		{
			name:      "zero tokens returns 0 cost",
			model:     "claude-3-5-sonnet",
			inTokens:  0,
			outTokens: 0,
			prices: map[string]ModelPrice{
				"claude-3-5-sonnet": {InPrice: 3.0, OutPrice: 15.0},
			},
			want: 0,
		},
		{
			name:      "zero inTokens only outTokens",
			model:     "claude-3-5-sonnet",
			inTokens:  0,
			outTokens: 2_000_000,
			prices: map[string]ModelPrice{
				"claude-3-5-sonnet": {InPrice: 3.0, OutPrice: 15.0},
			},
			want: 30.0,
		},
		{
			name:      "large token values",
			model:     "claude-3-5-sonnet",
			inTokens:  1_000_000_000,
			outTokens: 500_000_000,
			prices: map[string]ModelPrice{
				"claude-3-5-sonnet": {InPrice: 3.0, OutPrice: 15.0},
			},
			want: 10500.0,
		},
		{
			name:      "empty model string with default",
			model:     "",
			inTokens:  1_000_000,
			outTokens: 1_000_000,
			prices: map[string]ModelPrice{
				"claude-3-5-sonnet": {InPrice: 3.0, OutPrice: 15.0},
				"default":           {InPrice: 1.0, OutPrice: 2.0},
			},
			want: 3.0,
		},
		{
			name:      "empty model string without default returns 0",
			model:     "",
			inTokens:  1_000_000,
			outTokens: 1_000_000,
			prices: map[string]ModelPrice{
				"claude-3-5-sonnet": {InPrice: 3.0, OutPrice: 15.0},
			},
			want: 0,
		},
		{
			name:      "model matches default key as literal prefix",
			model:     "default-model",
			inTokens:  1_000_000,
			outTokens: 0,
			prices: map[string]ModelPrice{
				"default": {InPrice: 1.0, OutPrice: 2.0},
			},
			want: 1.0,
		},
		{
			name:      "shorter prefix does not override longer prefix",
			model:     "abc-123",
			inTokens:  1_000_000,
			outTokens: 0,
			prices: map[string]ModelPrice{
				"a":       {InPrice: 1.0, OutPrice: 1.0},
				"ab":      {InPrice: 2.0, OutPrice: 2.0},
				"abc":     {InPrice: 3.0, OutPrice: 3.0},
				"abc-":    {InPrice: 4.0, OutPrice: 4.0},
				"abc-1":   {InPrice: 5.0, OutPrice: 5.0},
				"abc-12":  {InPrice: 6.0, OutPrice: 6.0},
				"abc-123": {InPrice: 7.0, OutPrice: 7.0},
			},
			want: 7.0,
		},
		{
			name:      "prices map with only default key",
			model:     "any-model",
			inTokens:  2_000_000,
			outTokens: 3_000_000,
			prices: map[string]ModelPrice{
				"default": {InPrice: 2.5, OutPrice: 7.5},
			},
			want: 27.5,
		},
		{
			name:      "single token values",
			model:     "claude-3-5-sonnet",
			inTokens:  1,
			outTokens: 1,
			prices: map[string]ModelPrice{
				"claude-3-5-sonnet": {InPrice: 1_000_000.0, OutPrice: 1_000_000.0},
			},
			want: 2.0,
		},
		{
			name:      "empty prices map returns 0",
			model:     "claude-3-5-sonnet",
			inTokens:  1_000_000,
			outTokens: 1_000_000,
			prices:    map[string]ModelPrice{},
			want:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateCost(tt.model, tt.inTokens, tt.outTokens, tt.prices)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("calculateCost(%q, %d, %d, %v) = %v, want %v",
					tt.model, tt.inTokens, tt.outTokens, tt.prices, got, tt.want)
			}
		})
	}
}
