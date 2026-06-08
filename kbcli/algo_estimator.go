package main

import (
	"fmt"
	"kanban/kbcli/internal/appconfig"
)

func estimateCommitAncientMinutes(diffLines int) (float64, string) {
	if diffLines <= 0 {
		return appconfig.Cfg.AlgoEstimation.MinMinutes, "默认估算:无代码变更"
	}
	minutes := float64(diffLines) / appconfig.Cfg.AlgoEstimation.CommitLinePerMinutes
	if minutes < appconfig.Cfg.AlgoEstimation.MinMinutes {
		minutes = appconfig.Cfg.AlgoEstimation.MinMinutes
	}
	return minutes, fmt.Sprintf("基于diff_lines=%d估算(%.2f行/分钟)", diffLines, appconfig.Cfg.AlgoEstimation.CommitLinePerMinutes)
}
