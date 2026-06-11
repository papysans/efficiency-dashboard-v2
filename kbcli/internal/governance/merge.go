package governance

import (
	"regexp"

	"kanban/core/models"
	"kanban/kbcli/internal/logx"

	"gorm.io/gorm"
)

// mergeCommentRe merge commit 的 comment 兜底识别正则：只认 git 生成的规范 merge 文案
// （Merge branch / Merge pull request / Merge remote(-tracking) / Merge tag / Merge commit）。
// 故意不放宽到行首任意 merge：兜底命中会把 effective_diff_lines 清零，
// "merge search results" 这类祈使句开发 commit 被误杀的代价远大于漏认
// （真 merge 的主路径是 import-repo 按 parent_ids 落 is_merge，兜底只为存量旧数据）。
var mergeCommentRe = regexp.MustCompile(`(?i)^merge\s+(branch|pull request|remote|tag|commit)\b`)

// IsMergeComment 判定 commit comment 是否为 merge commit 的提交说明。
// 存量数据没有 parent_ids，靠 comment 兜底识别（增量数据由 import-repo 按 parent_ids 落 is_merge）。
func IsMergeComment(comment string) bool {
	return mergeCommentRe.MatchString(comment)
}

// applyMergeRules merge 治理子 pass：
//  1. comment 兜底识别 merge commit（存量数据没有 parent_ids）→ is_merge=true；
//  2. is_merge=true → effective_diff_lines 置 0（merge 的 diff 不计交付），excluded_flag 不动。
//
// 必须在 commitrules 之后执行：merge 置 0 优先级最高，覆盖软上限/降权对 merge commit 的折算结果。
func applyMergeRules(db *gorm.DB) error {
	var commits []models.Commit
	flaggedCount, zeroedCount := 0, 0
	err := db.Select("commit_id, comment, is_merge, effective_diff_lines").
		FindInBatches(&commits, governanceBatchSize, func(_ *gorm.DB, _ int) error {
			for i := range commits {
				c := &commits[i]
				updates := map[string]interface{}{}
				isMerge := c.IsMerge
				if !isMerge && IsMergeComment(c.Comment) {
					updates["is_merge"] = true
					isMerge = true
					flaggedCount++
				}
				if isMerge && (c.EffectiveDiffLines == nil || *c.EffectiveDiffLines != 0) {
					updates["effective_diff_lines"] = int64(0)
					zeroedCount++
				}
				if len(updates) == 0 {
					continue // 无变化不写
				}
				if err := db.Model(&models.Commit{}).Where("commit_id = ?", c.CommitId).
					Updates(updates).Error; err != nil {
					return err
				}
			}
			return nil
		}).Error
	if err != nil {
		return err
	}
	if flaggedCount > 0 || zeroedCount > 0 {
		logx.Infof("governance merge: comment 兜底标记 %d 条 is_merge，置零 effective_diff_lines %d 条", flaggedCount, zeroedCount)
	}
	return nil
}
