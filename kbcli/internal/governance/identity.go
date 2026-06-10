package governance

import (
	"path"
	"regexp"
	"strings"

	"kanban/core/models"
	"kanban/kbcli/internal/logx"

	"gorm.io/gorm"
)

// builtinBotEmailRe 内置 bot 邮箱正则（builtin_bot_patterns=true 时启用），
// 命中即视为机器人身份（noreply/CI bot 等非真人交付）。
var builtinBotEmailRe = regexp.MustCompile(`(?i)bot|noreply|no-reply|actions|robot`)

// governanceGlobMatch 大小写不敏感的通配符匹配（* 匹配任意串，glob 语法）；
// 模式非法时视为不命中，不报错。
func governanceGlobMatch(pattern, s string) bool {
	ok, err := path.Match(strings.ToLower(pattern), strings.ToLower(s))
	return err == nil && ok
}

// JudgeIdentity 按身份治理配置判定一条 commit 的作者身份是否应被排除，
// 返回 (是否排除, 排除原因)。规则按优先级依次判定，先命中先返回：
//  1. 内置 bot 邮箱正则（builtin_bot_patterns 开关）→ identity:bot
//  2. 邮箱精确黑名单（大小写不敏感）→ identity:blocked_email
//  3. 邮箱通配模式（glob，* 通配，大小写不敏感）→ identity:blocked_email_pattern
//  4. git user name 黑名单模式（glob）→ identity:blocked_name
//  5. identity_map enforce 且该 user_id 有映射条目但 email 不在允许列表 → identity:foreign_author
func JudgeIdentity(cfg Config, email, name, userId string) (bool, string) {
	id := cfg.Identity
	if id.BuiltinBotPatterns && builtinBotEmailRe.MatchString(email) {
		return true, "identity:bot"
	}
	for _, blocked := range id.BlockedEmails {
		if strings.EqualFold(strings.TrimSpace(blocked), email) {
			return true, "identity:blocked_email"
		}
	}
	for _, pattern := range id.BlockedEmailPatterns {
		if governanceGlobMatch(pattern, email) {
			return true, "identity:blocked_email_pattern"
		}
	}
	for _, pattern := range id.BlockedNamePatterns {
		if governanceGlobMatch(pattern, name) {
			return true, "identity:blocked_name"
		}
	}
	if id.IdentityMap.Enforce {
		if allowed, ok := id.IdentityMap.Users[userId]; ok {
			matched := false
			for _, a := range allowed {
				if strings.EqualFold(strings.TrimSpace(a), email) {
					matched = true
					break
				}
			}
			if !matched {
				return true, "identity:foreign_author"
			}
		}
	}
	return false, ""
}

// applyIdentityRules 身份治理子 pass：分批（每批 500）扫全表 commits，逐条按 JudgeIdentity 重判。
// 命中 → excluded_flag=true + excluded_reason（打标记不删数据）；
// 未命中且现有 excluded_reason 以 identity: 开头 → 清回 false/”（幂等：改名单重跑即恢复）。
// 只 update 有变化的行。
func applyIdentityRules(db *gorm.DB, cfg Config) error {
	var commits []models.Commit
	excludedCount, restoredCount := 0, 0
	err := db.Select("commit_id, git_user_email, git_user_name, user_id, excluded_flag, excluded_reason").
		FindInBatches(&commits, governanceBatchSize, func(_ *gorm.DB, _ int) error {
			for i := range commits {
				c := &commits[i]
				excluded, reason := JudgeIdentity(cfg, c.GitUserEmail, c.GitUserName, c.UserId)
				switch {
				case excluded:
					if c.ExcludedFlag && c.ExcludedReason == reason {
						continue // 无变化不写
					}
					if err := db.Model(&models.Commit{}).Where("commit_id = ?", c.CommitId).
						Updates(map[string]interface{}{"excluded_flag": true, "excluded_reason": reason}).Error; err != nil {
						return err
					}
					excludedCount++
				case strings.HasPrefix(c.ExcludedReason, "identity:"):
					// 曾被身份规则排除但现行名单已不命中 → 清回（幂等恢复）
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
		logx.Infof("governance identity: 新排除 %d 条，恢复 %d 条", excludedCount, restoredCount)
	}
	return nil
}
