package governance

import "gorm.io/gorm"

// applyCommitRules commit 规则治理子 pass：巨型批量提交软上限（diff_lines_softcap）、
// 低价值 comment 降权（downweight_comment_patterns）、merge/rebase 重放去重（replay_dedup），
// 结果写 effective_diff_lines / is_merge / replay_of / excluded_*（打标记不删数据）。
// TODO(W2): 由并行流 W2 实现，当前为桩。
func applyCommitRules(db *gorm.DB, cfg Config) error {
	return nil
}
