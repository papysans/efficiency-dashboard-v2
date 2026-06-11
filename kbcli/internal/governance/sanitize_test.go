package governance

import "testing"

// TestSanitizeRepoAddr 脱敏矩阵：user:pass / 仅 user / scp 不动 / 无 userinfo / 空串 / 幂等。
func TestSanitizeRepoAddr(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		// user:pass（明文 PAT/密码）必须剥掉，其余原样（scheme、.git、路径不动）
		{"http 内嵌 token", "http://oauth2:glpat-abc.01.xyz@gitlab/root/scan-repo.git", "http://gitlab/root/scan-repo.git"},
		{"https 内嵌账密", "https://user:Passw0rd@10.2.3.4/mom/momproject", "https://10.2.3.4/mom/momproject"},
		// 仅 user（无密码）也剥掉
		{"https 仅用户名", "https://deploy@github.com/x", "https://github.com/x"},
		{"ssh 协议 userinfo", "ssh://git@10.129.0.33:29418/mom/momproject.git", "ssh://10.129.0.33:29418/mom/momproject.git"},
		// scp 形式 git@host:path 的 git@ 是 ssh 登录名不是凭据，原样返回
		{"scp 形式不动", "git@github.com:a/b.git", "git@github.com:a/b.git"},
		// 协议形式但无 userinfo → 原样（含路径中 @ 不误剥）
		{"无 userinfo 不动", "https://github.com/acme/repo.git", "https://github.com/acme/repo.git"},
		{"路径中 @ 不动", "https://github.com/acme/repo@v2", "https://github.com/acme/repo@v2"},
		// 非协议形式 / 空串原样
		{"裸地址不动", "github.com/acme/repo", "github.com/acme/repo"},
		{"空串原样返回", "", ""},
		// scheme 大小写不敏感识别，且不改写原大小写
		{"scheme 大写", "HTTPS://deploy@GitHub.com/Acme/Repo.Git", "HTTPS://GitHub.com/Acme/Repo.Git"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SanitizeRepoAddr(tc.in); got != tc.want {
				t.Fatalf("SanitizeRepoAddr(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
	// 幂等：脱敏结果再脱敏不变
	for _, tc := range cases {
		once := SanitizeRepoAddr(tc.in)
		if twice := SanitizeRepoAddr(once); twice != once {
			t.Fatalf("SanitizeRepoAddr 不幂等: %q → %q → %q", tc.in, once, twice)
		}
	}
}
