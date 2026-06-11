package governance

import (
	"fmt"
	"strings"

	"kanban/kbcli/internal/logx"

	"gorm.io/gorm"
)

// SanitizeRepoAddr 凭据脱敏：仅当地址是协议形式（http:// / https:// / ssh://）且 authority 段
// 含 userinfo（user@ 或 user:pass@，如 http://oauth2:glpat-xxx@gitlab/...）时，剥掉 userinfo，
// 其余原样保留（scheme、大小写、.git 后缀、路径都不动）。
// scp 形式 git@host:path 的 git@ 是 ssh 登录名不是凭据，原样返回；
// 非协议形式 / 无 userinfo / 空串也原样返回。幂等：脱敏结果再脱敏不变。
// 注意与 CanonRepoAddr 的分工：canon 是聚合用的归一视图（lower/去协议），
// 本函数是基表存储值的安全清洗，必须保留原始写法。
func SanitizeRepoAddr(addr string) string {
	if addr == "" {
		return addr
	}
	lower := strings.ToLower(addr)
	schemeEnd := 0
	for _, scheme := range []string{"http://", "https://", "ssh://"} {
		if strings.HasPrefix(lower, scheme) {
			schemeEnd = len(scheme)
			break
		}
	}
	if schemeEnd == 0 {
		return addr
	}
	// authority 段 = scheme 之后到第一个 "/" 之前；取最后一个 "@" 之后为 host，剥掉 userinfo
	rest := addr[schemeEnd:]
	authority := rest
	if slash := strings.IndexByte(rest, '/'); slash >= 0 {
		authority = rest[:slash]
	}
	at := strings.LastIndexByte(authority, '@')
	if at < 0 {
		return addr
	}
	return addr[:schemeEnd] + rest[at+1:]
}

// applySanitizeRepoAddrs 基表凭据脱敏子 pass：对 commits / conversations / session_stage_metrics
// 三张表，清洗 repo_addr 中内嵌的 URL userinfo（明文 PAT/密码经 API 对前端可见，二轮挖掘实锤 294 行）。
// 凭据是安全例外，优先于"治理只打标记不改原始数据"哲学——明文 token 一旦落库就是泄露面，必须改写。
// 用 SQL LIKE '%://%@%' 预过滤出疑似含凭据的 distinct 地址（conversations 5 万行，不可全表加载），
// SanitizeRepoAddr 后有变化才 UPDATE。幂等：清洗过的行不再命中变化条件，重跑零写入。
func applySanitizeRepoAddrs(db *gorm.DB) error {
	for _, table := range []string{"commits", "conversations", "session_stage_metrics"} {
		var addrs []string
		if err := db.Table(table).Distinct("repo_addr").
			Where("repo_addr LIKE ?", "%://%@%").Pluck("repo_addr", &addrs).Error; err != nil {
			return fmt.Errorf("扫描 %s 表含凭据 repo_addr 失败: %w", table, err)
		}
		updated := 0
		for _, addr := range addrs {
			clean := SanitizeRepoAddr(addr)
			if clean == addr {
				continue
			}
			res := db.Table(table).Where("repo_addr = ?", addr).Update("repo_addr", clean)
			if res.Error != nil {
				return fmt.Errorf("脱敏 %s 表 repo_addr 失败: %w", table, res.Error)
			}
			updated += int(res.RowsAffected)
		}
		if updated > 0 {
			logx.Infof("governance sanitize: %s 表脱敏 %d 行含凭据 repo_addr", table, updated)
		}
	}
	return nil
}
