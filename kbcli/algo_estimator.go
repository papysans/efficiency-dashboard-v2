package main

import "fmt"

func estimateCommitAncientMinutes(diffLines int) (float64, string) {
	if diffLines <= 0 {
		return 5, "默认估算:无代码变更"
	}
	minutes := float64(diffLines) / cfg.AlgoEstimation.LinesPerMinutes
	if minutes < 5 {
		minutes = 5
	}
	return minutes, fmt.Sprintf("基于diff_lines=%d估算(%f行/分钟)", diffLines, cfg.AlgoEstimation.LinesPerMinutes)
}
