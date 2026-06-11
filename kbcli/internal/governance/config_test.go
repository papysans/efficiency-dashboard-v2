package governance

import (
	"path/filepath"
	"testing"
)

// assertDefaultConfig 断言 cfg 为内置默认治理配置（黑名单空、规则开关与系数为默认值）。
func assertDefaultConfig(t *testing.T, cfg Config) {
	t.Helper()
	if !cfg.Identity.BuiltinBotPatterns {
		t.Errorf("默认 builtin_bot_patterns 应为 true")
	}
	if len(cfg.Identity.BlockedEmails) != 0 || len(cfg.Identity.BlockedEmailPatterns) != 0 ||
		len(cfg.Identity.BlockedNamePatterns) != 0 || len(cfg.Identity.BlockedUserIds) != 0 {
		t.Errorf("默认黑名单应为空，得到 %+v", cfg.Identity)
	}
	if cfg.Identity.IdentityMap.Enforce {
		t.Errorf("默认 identity_map.enforce 应为 false")
	}
	if cfg.CommitRules.DiffLinesSoftcap != 3000 {
		t.Errorf("默认 diff_lines_softcap 应为 3000，得到 %d", cfg.CommitRules.DiffLinesSoftcap)
	}
	if len(cfg.CommitRules.DownweightCommentPatterns) != 1 || cfg.CommitRules.DownweightCommentPatterns[0].Factor != 0.2 {
		t.Errorf("默认降权规则应为 1 条 factor=0.2，得到 %+v", cfg.CommitRules.DownweightCommentPatterns)
	}
	if !cfg.CommitRules.ReplayDedup {
		t.Errorf("默认 replay_dedup 应为 true")
	}
	if !cfg.Normalization.RepoAddrCanon {
		t.Errorf("默认 repo_addr_canon 应为 true")
	}
}

func TestLoadMissingFileFallsBackToDefault(t *testing.T) {
	// path 为空 → 默认值，不报错
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load(\"\") 不应报错: %v", err)
	}
	assertDefaultConfig(t, cfg)

	// 文件不存在 → 默认值，不报错
	cfg, err = Load(filepath.Join(t.TempDir(), "not-exist.yaml"))
	if err != nil {
		t.Fatalf("Load(不存在文件) 不应报错: %v", err)
	}
	assertDefaultConfig(t, cfg)
}

func TestLoadExampleYaml(t *testing.T) {
	// 真实样例文件必须能按 schema 解析（防止样例与代码 schema 漂移）
	cfg, err := Load(filepath.Join("..", "..", "..", "config", "governance.example.yaml"))
	if err != nil {
		t.Fatalf("加载样例治理配置失败: %v", err)
	}
	if len(cfg.Identity.BlockedEmails) != 5 {
		t.Errorf("样例 blocked_emails 应为 5 条，得到 %d", len(cfg.Identity.BlockedEmails))
	}
	if len(cfg.Identity.BlockedEmailPatterns) != 3 {
		t.Errorf("样例 blocked_email_patterns 应为 3 条，得到 %d", len(cfg.Identity.BlockedEmailPatterns))
	}
	if len(cfg.Identity.BlockedNamePatterns) != 2 {
		t.Errorf("样例 blocked_name_patterns 应为 2 条，得到 %d", len(cfg.Identity.BlockedNamePatterns))
	}
	if len(cfg.Identity.BlockedUserIds) != 0 {
		t.Errorf("样例 blocked_user_ids 应为空列表（真实 user_id 属部署配置），得到 %v", cfg.Identity.BlockedUserIds)
	}
	if cfg.Identity.IdentityMap.Enforce {
		t.Errorf("样例 identity_map.enforce 应为 false")
	}
	if cfg.CommitRules.DiffLinesSoftcap != 3000 || !cfg.CommitRules.ReplayDedup || !cfg.Normalization.RepoAddrCanon {
		t.Errorf("样例 commit_rules/normalization 解析异常: %+v", cfg)
	}
}
