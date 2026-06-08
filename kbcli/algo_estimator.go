package main

import "fmt"

func estimateCommitAncientMinutes(diffLines int) (float64, string) {
	if diffLines <= 0 {
		return cfg.AlgoEstimation.MinMinutes, "默认估算:无代码变更"
	}
	minutes := float64(diffLines) / cfg.AlgoEstimation.CommitLinePerMinutes
	if minutes < cfg.AlgoEstimation.MinMinutes {
		minutes = cfg.AlgoEstimation.MinMinutes
	}
	return minutes, fmt.Sprintf("基于diff_lines=%d估算(%.2f行/分钟)", diffLines, cfg.AlgoEstimation.CommitLinePerMinutes)
}
