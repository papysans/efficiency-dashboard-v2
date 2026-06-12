package governance

import (
	"encoding/json"
	"strings"

	"kanban/core/models"
	"kanban/kbcli/internal/logx"

	"gorm.io/gorm"
)

// docOnlyReason 纯文档 commit 的排除原因标识（稳定字符串，幂等清理按本前缀匹配）。
const docOnlyReason = "doc_only_commit"

// normalizeDocExtensions 把配置的文档后缀规整为小写、确保以 "." 开头，过滤空项。
// 大小写不敏感由这里统一转小写、判定时也转小写实现。
func normalizeDocExtensions(exts []string) []string {
	out := make([]string, 0, len(exts))
	for _, e := range exts {
		e = strings.ToLower(strings.TrimSpace(e))
		if e == "" {
			continue
		}
		if !strings.HasPrefix(e, ".") {
			e = "." + e
		}
		out = append(out, e)
	}
	return out
}

// IsDocOnlyCommit 判定一条 commit 是否为纯文档 commit：
// touchedFilesJSON（jsonb 字符串数组）非空，且每个文件路径（小写后）以 docExts 中某个后缀结尾。
// 任一文件不是文档后缀（含解析失败/空列表）→ 非纯文档（mixed 或无文件，不排除）。
// docExts 须为已 normalize 的小写后缀列表；空列表直接返回 false（规则禁用）。
func IsDocOnlyCommit(touchedFilesJSON string, docExts []string) bool {
	if len(docExts) == 0 {
		return false
	}
	var files []string
	if err := json.Unmarshal([]byte(strings.TrimSpace(touchedFilesJSON)), &files); err != nil {
		return false
	}
	nonEmpty := 0
	for _, f := range files {
		f = strings.ToLower(strings.TrimSpace(f))
		if f == "" {
			continue
		}
		nonEmpty++
		if !hasAnySuffix(f, docExts) {
			return false
		}
	}
	return nonEmpty > 0
}

// hasAnySuffix path 是否以 suffixes（已小写）中任一后缀结尾。
func hasAnySuffix(path string, suffixes []string) bool {
	for _, s := range suffixes {
		if strings.HasSuffix(path, s) {
			return true
		}
	}
	return false
}

// applyDocOnlyRules 纯文档 commit 治理子 pass：分批（每批 500）扫全表 commits，逐条按 IsDocOnlyCommit 重判。
// 命中 → excluded_flag=true + excluded_reason=doc_only_commit（打标记不删数据，GetEffectiveDiffLines 自动归零）；
// 未命中且现有 excluded_reason==doc_only_commit → 清回 false/''（幂等：改后缀名单重跑即恢复）。
// 不覆盖其它规则的排除：已被 identity 等规则排除的 commit（reason 非本规则）不动，避免规则叠加冲突。
// 只 update 有变化的行。
func applyDocOnlyRules(db *gorm.DB, cfg Config) error {
	docExts := normalizeDocExtensions(cfg.CommitRules.DocFileExtensions)
	if len(docExts) == 0 {
		// 规则禁用（空后缀）：不能直接跳过——须恢复本规则曾排除的所有 commit，
		// 否则上一轮打的 doc_only_commit 标记残留，禁用后 LOC 仍被错误归零（违背幂等）。
		res := db.Model(&models.Commit{}).Where("excluded_reason = ?", docOnlyReason).
			Updates(map[string]interface{}{"excluded_flag": false, "excluded_reason": ""})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected > 0 {
			logx.Infof("governance doc_only: 规则禁用，恢复 %d 条曾排除的纯文档 commit", res.RowsAffected)
		}
		return nil
	}

	var commits []models.Commit
	excludedCount, restoredCount := 0, 0
	err := db.Select("commit_id, touched_files, excluded_flag, excluded_reason").
		FindInBatches(&commits, governanceBatchSize, func(_ *gorm.DB, _ int) error {
			for i := range commits {
				c := &commits[i]
				isDocOnly := IsDocOnlyCommit(string(c.TouchedFiles), docExts)
				switch {
				case isDocOnly:
					if c.ExcludedFlag {
						// 已被排除：本规则命中则补/纠原因为 doc_only_commit；若被其它规则排除（reason 非本规则）则让位，不动。
						if c.ExcludedReason == docOnlyReason {
							continue // 无变化不写
						}
						if c.ExcludedReason != "" {
							continue // 让位给其它规则（identity/replay 等），不覆盖语义
						}
					}
					if err := db.Model(&models.Commit{}).Where("commit_id = ?", c.CommitId).
						Updates(map[string]interface{}{"excluded_flag": true, "excluded_reason": docOnlyReason}).Error; err != nil {
						return err
					}
					excludedCount++
				case c.ExcludedReason == docOnlyReason:
					// 曾被本规则排除但现行后缀名单已不命中 → 清回（幂等恢复）
					if err := db.Model(&models.Commit{}).Where("commit_id = ?", c.CommitId).
						Updates(map[string]interface{}{"excluded_flag": false, "excluded_reason": ""}).Error; err != nil {
						return err
					}
					restoredCount++
				}
			}
			return nil
		}).Error
	if err != nil {
		return err
	}
	if excludedCount > 0 || restoredCount > 0 {
		logx.Infof("governance doc_only: 新排除 %d 条纯文档 commit，恢复 %d 条（后缀=%v）", excludedCount, restoredCount, docExts)
	}
	return nil
}
