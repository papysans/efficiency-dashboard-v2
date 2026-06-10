package governance

import "testing"

// TestCanonRepoAddr canon 矩阵：ssh/scp/https/.git/大小写/尾斜杠/IP:port/空串。
func TestCanonRepoAddr(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"ssh 协议", "ssh://git@github.com/acme/repo.git", "github.com/acme/repo"},
		{"scp 风格 git@host:org/repo", "git@github.com:acme/repo.git", "github.com/acme/repo"},
		{"https 协议", "https://github.com/acme/repo", "github.com/acme/repo"},
		{"http 协议", "http://github.com/acme/repo.git", "github.com/acme/repo"},
		{"大小写归一", "HTTPS://GitHub.com/Acme/Repo.Git", "github.com/acme/repo"},
		{"尾斜杠", "https://github.com/acme/repo/", "github.com/acme/repo"},
		{".git 加尾斜杠", "https://github.com/acme/repo.git/", "github.com/acme/repo"},
		{"http 带端口（冒号保留）", "http://10.129.0.33:1080/mom/momproject", "10.129.0.33:1080/mom/momproject"},
		{"ssh 带端口（冒号保留）", "ssh://git@10.129.0.33:29418/mom/momproject.git", "10.129.0.33:29418/mom/momproject"},
		{"两侧空白", "  git@github.com:acme/repo.git  ", "github.com/acme/repo"},
		{"裸地址不变", "github.com/acme/repo", "github.com/acme/repo"},
		{"空串原样返回", "", ""},
		{"双写法归一相等-1", "git@10.2.3.4:mom/momproject.git", "10.2.3.4/mom/momproject"},
		{"双写法归一相等-2", "https://10.2.3.4/mom/momproject", "10.2.3.4/mom/momproject"},
		// URL userinfo（含凭据）必须剥离：token 不属于仓库身份，更不能进 need 边界 key
		{"内嵌 token", "http://oauth2:glpat-abc.01.xyz@gitlab/root/scan-repo.git", "gitlab/root/scan-repo"},
		{"内嵌用户名", "https://deploy@github.com/acme/repo.git", "github.com/acme/repo"},
		{"带 token 与裸地址归一", "http://oauth2:tok@gitlab.example.com/g/r", "gitlab.example.com/g/r"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CanonRepoAddr(tc.in); got != tc.want {
				t.Fatalf("CanonRepoAddr(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
	// 幂等：canon 结果再 canon 不变。
	for _, tc := range cases {
		once := CanonRepoAddr(tc.in)
		if twice := CanonRepoAddr(once); twice != once {
			t.Fatalf("CanonRepoAddr 不幂等: %q → %q → %q", tc.in, once, twice)
		}
	}
}
