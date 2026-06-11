package governance

import "testing"

// testIdentityConfig 构造与 config/governance.example.yaml 一致的身份治理配置（真实部署名单）。
func testIdentityConfig() Config {
	cfg := DefaultConfig()
	cfg.Identity.BlockedEmails = []string{
		"test@example.com", "xjm@test.com", "yaajun@example.com", "lisama@example.com", "incremental@test.local",
		"your@email.com", "root@localhost.localdomain",
	}
	cfg.Identity.BlockedEmailPatterns = []string{"*@example.com", "*@test.*", "*@test.local", "*@*.internal.cloudapp.net"}
	cfg.Identity.BlockedNamePatterns = []string{"Test User", "Incremental Tester"}
	return cfg
}

func TestJudgeIdentityMatrix(t *testing.T) {
	cfg := testIdentityConfig()
	cases := []struct {
		desc       string
		email      string
		name       string
		userId     string
		wantHit    bool
		wantReason string
	}{
		// a. 内置 bot 正则（大小写不敏感，内网实测 bot/noreply 312 个 commit）
		{"bot邮箱", "ci-bot@sangfor.com.cn", "CI Bot", "u1", true, "identity:bot"},
		{"noreply邮箱", "noreply@github.com", "GitHub", "u1", true, "identity:bot"},
		{"no-reply邮箱", "no-reply@gitee.com", "Gitee", "u1", true, "identity:bot"},
		{"actions邮箱", "github-actions@github.com", "Actions", "u1", true, "identity:bot"},
		{"actions[bot]隐私域", "41898282+github-actions[bot]@users.noreply.github.com", "github-actions[bot]", "u1", true, "identity:bot"},
		{"ROBOT大写", "ROBOT@SANGFOR.COM", "Robot", "u1", true, "identity:bot"},
		// GitHub 隐私邮箱是真人提交特性（12345+name@users.noreply.github.com），
		// noreply 锚定 local-part 开头，域名含 noreply 不算 bot（实测子串匹配误排 28 人 300 个 commit）
		{"GitHub隐私邮箱不误伤", "12345+zhangsan@users.noreply.github.com", "zhangsan", "u1", false, ""},
		// \b 锚定不误伤拼音
		{"拼音bot不误伤", "zhangbotao@sangfor.com.cn", "张博韬", "u1", false, ""},
		// b. 精确黑名单（大小写不敏感；test@example.com 内网实测 18 个 commit/17.1 万行）
		{"精确黑名单", "test@example.com", "tester", "u1", true, "identity:blocked_email"},
		{"精确黑名单大小写", "Test@Example.COM", "tester", "u1", true, "identity:blocked_email"},
		{"精确黑名单local域", "incremental@test.local", "tester", "u1", true, "identity:blocked_email"},
		{"精确黑名单占位身份", "your@email.com", "Your Name", "u1", true, "identity:blocked_email"},
		// c. glob 邮箱模式（* 通配，大小写不敏感；精确黑名单优先于模式，见上 test@example.com 用例）
		{"glob example.com", "anyone@example.com", "anyone", "u1", true, "identity:blocked_email_pattern"},
		{"glob test.*", "foo@test.cn", "foo", "u1", true, "identity:blocked_email_pattern"},
		{"glob test.local", "bar@test.local", "bar", "u1", true, "identity:blocked_email_pattern"},
		{"glob azure测试机", "user@vm.internal.cloudapp.net", "user", "u1", true, "identity:blocked_email_pattern"},
		{"glob大小写", "FOO@TEST.COM", "foo", "u1", true, "identity:blocked_email_pattern"},
		{"glob不误伤相似域", "zhangsan@test-env.sangfor.com.cn", "张三", "u1", false, ""},
		// 守护：曾配 "*@*.local" 误伤内网真实交付 ai@claude-code.local 1665 行，
		// 该模式已改为 "*@test.local"，此用例防止误伤行为复活
		{"local域真实交付不误伤", "ai@claude-code.local", "ai", "u1", false, ""},
		// d. name glob 黑名单
		{"name精确命中", "zhangsan@sangfor.com.cn", "Test User", "u1", true, "identity:blocked_name"},
		{"name第二条", "zhangsan@sangfor.com.cn", "Incremental Tester", "u1", true, "identity:blocked_name"},
		{"name不含通配不部分匹配", "zhangsan@sangfor.com.cn", "A Test User B", "u1", false, ""},
		// 正常身份不误伤
		{"正常身份", "zhangsan@sangfor.com.cn", "张三", "u1", false, ""},
		{"空邮箱空名字", "", "", "u1", false, ""},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			hit, reason := JudgeIdentity(cfg, c.email, c.name, c.userId)
			if hit != c.wantHit || reason != c.wantReason {
				t.Fatalf("JudgeIdentity(%q,%q,%q) = (%v,%q)，期望 (%v,%q)",
					c.email, c.name, c.userId, hit, reason, c.wantHit, c.wantReason)
			}
		})
	}
}

func TestJudgeIdentityBuiltinBotSwitch(t *testing.T) {
	cfg := testIdentityConfig()
	cfg.Identity.BuiltinBotPatterns = false
	// 关掉内置 bot 正则后，bot 邮箱不再因 bot 规则命中（也不命中黑名单）
	if hit, reason := JudgeIdentity(cfg, "ci-bot@sangfor.com.cn", "CI Bot", "u1"); hit {
		t.Fatalf("builtin_bot_patterns=false 时 bot 邮箱不应命中，得到 reason=%q", reason)
	}
	// 但仍可被 glob 模式兜住
	if hit, reason := JudgeIdentity(cfg, "bot@test.local", "CI Bot", "u1"); !hit || reason != "identity:blocked_email_pattern" {
		t.Fatalf("bot@test.local 应命中 glob 模式，得到 (%v,%q)", hit, reason)
	}
}

func TestJudgeIdentityMap(t *testing.T) {
	cfg := testIdentityConfig()
	cfg.Identity.IdentityMap.Users = map[string][]string{
		"user-id-123": {"zhangsan@sangfor.com.cn"},
	}

	// enforce=false：有条目也不硬排
	cfg.Identity.IdentityMap.Enforce = false
	if hit, reason := JudgeIdentity(cfg, "other@sangfor.com.cn", "李四", "user-id-123"); hit {
		t.Fatalf("enforce=false 不应硬排，得到 reason=%q", reason)
	}

	// enforce=true：映射内 user_id 用映射外邮箱 → 硬排
	cfg.Identity.IdentityMap.Enforce = true
	if hit, reason := JudgeIdentity(cfg, "other@sangfor.com.cn", "李四", "user-id-123"); !hit || reason != "identity:foreign_author" {
		t.Fatalf("enforce=true 映射外邮箱应硬排，得到 (%v,%q)", hit, reason)
	}
	// 允许列表内邮箱（大小写不敏感）→ 不排
	if hit, reason := JudgeIdentity(cfg, "ZhangSan@Sangfor.com.cn", "张三", "user-id-123"); hit {
		t.Fatalf("允许列表内邮箱不应硬排，得到 reason=%q", reason)
	}
	// 映射外 user_id → 不受 identity_map 约束
	if hit, reason := JudgeIdentity(cfg, "other@sangfor.com.cn", "李四", "user-id-999"); hit {
		t.Fatalf("映射外 user_id 不应硬排，得到 reason=%q", reason)
	}
}
