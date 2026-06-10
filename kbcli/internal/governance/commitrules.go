package governance

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"kanban/core/models"
	"kanban/kbcli/internal/logx"

	"gorm.io/gorm"
)

// 分批读写 commits 的批大小
const commitRulesBatchSize = 500

// replay 去重参与门槛：仅 diff_lines>100 且 len(trim(comment))>5 的 commit 参与组键比对，
// 避免把小改动 / 短 comment 的正常重复提交误判为 rebase 重放。
const (
	replayDedupMinDiffLines  = 100
	replayDedupMinCommentLen = 5
)

// compiledDownweightRule 编译后的降权规则（pattern 自带 (?i) 等 flag）
type compiledDownweightRule struct {
	re     *regexp.Regexp
	factor float64
}

// commitRuleResult 单个 commit 的规则重算结果（本 pass 全量负责 effective_diff_lines / replay_of 两列）。
// effectiveDiffLines 为 nil = 规则全不命中，聚合侧回退原始 diff_lines。
type commitRuleResult struct {
	effectiveDiffLines *int64
	replayOf           string
}

// applyCommitRules commit 规则治理子 pass：巨型批量提交软上限（diff_lines_softcap）、
// 低价值 comment 降权（downweight_comment_patterns）、merge/rebase 重放去重（replay_dedup）。
// 幂等：每次按当前配置全量重算 effective_diff_lines / replay_of（含清掉不再命中的旧值），
// 规则变化后重跑即可恢复。is_merge 的判定与 merge 行 effective 覆盖归 W1 的 merge 规则，本 pass 不碰。
func applyCommitRules(db *gorm.DB, cfg Config) error {
	rules, err := compileDownweightRules(cfg.CommitRules.DownweightCommentPatterns)
	if err != nil {
		return err
	}

	// 分批 500 读全表（replay 去重组键跨批，需先收齐再统一计算）
	var all []models.Commit
	lastID := ""
	for {
		var batch []models.Commit
		if err := db.Select("commit_id", "user_id", "comment", "diff_lines", "commit_time", "replay_of", "effective_diff_lines").
			Where("commit_id > ?", lastID).
			Order("commit_id ASC").Limit(commitRulesBatchSize).Find(&batch).Error; err != nil {
			return fmt.Errorf("读取 commits 失败: %w", err)
		}
		if len(batch) == 0 {
			break
		}
		all = append(all, batch...)
		lastID = batch[len(batch)-1].CommitId
	}

	results := computeCommitRuleResults(all, cfg.CommitRules, rules)

	// 只更新与现值不同的行（重复跑零写入），不命中的行写回 NULL/'' 即全量重算语义
	updated := 0
	for _, c := range all {
		r := results[c.CommitId]
		if r.replayOf == c.ReplayOf && int64PtrEqual(r.effectiveDiffLines, c.EffectiveDiffLines) {
			continue
		}
		vals := map[string]interface{}{
			"replay_of":            r.replayOf,
			"effective_diff_lines": nil,
		}
		if r.effectiveDiffLines != nil {
			vals["effective_diff_lines"] = *r.effectiveDiffLines
		}
		if err := db.Model(&models.Commit{}).Where("commit_id = ?", c.CommitId).Updates(vals).Error; err != nil {
			return fmt.Errorf("更新 commit %s 治理字段失败: %w", c.CommitId, err)
		}
		updated++
	}
	logx.Infof("governance commit_rules: 扫描 %d 个 commit，更新 %d 行（softcap=%d, 降权规则=%d, replay_dedup=%v）",
		len(all), updated, cfg.CommitRules.DiffLinesSoftcap, len(rules), cfg.CommitRules.ReplayDedup)
	return nil
}

// compileDownweightRules 编译降权 comment 正则（Go regexp，pattern 自带 (?i)），非法正则直接报错。
func compileDownweightRules(rules []DownweightRule) ([]compiledDownweightRule, error) {
	out := make([]compiledDownweightRule, 0, len(rules))
	for _, r := range rules {
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			return nil, fmt.Errorf("编译降权正则 %q 失败: %w", r.Pattern, err)
		}
		out = append(out, compiledDownweightRule{re: re, factor: r.Factor})
	}
	return out, nil
}

// computeCommitRuleResults 纯计算（不触 DB）：对全量 commits 按 softcap / comment 降权 /
// replay 去重重算 effective_diff_lines 与 replay_of，返回 map[commit_id]结果。
// replay 去重组键 (user_id, trim(comment), diff_lines)，组内按 commit_time 升序保最早，
// 其余标 replay_of=最早 commit_id 且 effective=0。
func computeCommitRuleResults(commits []models.Commit, cfg CommitRulesConfig, rules []compiledDownweightRule) map[string]commitRuleResult {
	type replayKey struct {
		userID    string
		comment   string
		diffLines int
	}
	type replayMember struct {
		commitID   string
		commitTime time.Time
	}

	results := make(map[string]commitRuleResult, len(commits))
	groups := map[replayKey][]replayMember{}
	for _, c := range commits {
		results[c.CommitId] = commitRuleResult{
			effectiveDiffLines: computeCommitEffectiveDiffLines(int64(c.DiffLines), c.Comment, cfg.DiffLinesSoftcap, rules),
		}
		trimmed := strings.TrimSpace(c.Comment)
		if cfg.ReplayDedup && c.DiffLines > replayDedupMinDiffLines && len(trimmed) > replayDedupMinCommentLen {
			key := replayKey{userID: c.UserId, comment: trimmed, diffLines: c.DiffLines}
			groups[key] = append(groups[key], replayMember{commitID: c.CommitId, commitTime: c.CommitTime})
		}
	}
	for _, members := range groups {
		if len(members) < 2 {
			continue
		}
		sort.Slice(members, func(i, j int) bool {
			if !members[i].commitTime.Equal(members[j].commitTime) {
				return members[i].commitTime.Before(members[j].commitTime)
			}
			return members[i].commitID < members[j].commitID // 同秒重放按 commit_id 兜底，保证确定性
		})
		earliest := members[0].commitID
		for _, dup := range members[1:] {
			zero := int64(0)
			results[dup.commitID] = commitRuleResult{effectiveDiffLines: &zero, replayOf: earliest}
		}
	}
	return results
}

// computeCommitEffectiveDiffLines 计算单个 commit 的 effective_diff_lines（softcap + comment 降权）：
//   - softcap>0 且 diff_lines>softcap → 截到 softcap
//   - comment 命中降权正则 → effective = round(softcap 生效值 × factor)，多条命中取 factor 最小的一条
//
// 规则全不命中 → 返回 nil（聚合侧用原值）。
func computeCommitEffectiveDiffLines(diffLines int64, comment string, softcap int64, rules []compiledDownweightRule) *int64 {
	effective := diffLines
	hit := false
	if softcap > 0 && diffLines > softcap {
		effective = softcap
		hit = true
	}
	matched := false
	minFactor := 0.0
	for _, r := range rules {
		if r.re.MatchString(comment) && (!matched || r.factor < minFactor) {
			minFactor = r.factor
			matched = true
		}
	}
	if matched {
		effective = int64(math.Round(float64(effective) * minFactor))
		hit = true
	}
	if !hit {
		return nil
	}
	return &effective
}

func int64PtrEqual(a, b *int64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
