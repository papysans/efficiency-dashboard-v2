package utils

import "strings"

// CanonRepoAddr 规范化 repo 地址写法（http/ssh 协议、大小写、.git 后缀等写法分裂归一），
// 使同一仓库的不同写法在聚合/匹配时合并统计。kbcli 写入 needs.repo_addr 与 backend 读侧匹配
// 共用此唯一实现（core 共享叶子），避免两套规范化漂移。规则按序：
// trim → lower → 去前缀 ssh://git@ / ssh:// / git@ / http:// / https:// →
// 剥离 authority 段 userinfo（user[:pass]@host，凭据不属于仓库身份，绝不进 UI / need 边界 key）→
// host 后第一个 ":" 换 "/"（scp 风格 git@host:org/repo；冒号后第一段是纯数字端口时保留，
// 如 http://10.129.0.33:1080/x、ssh://git@host:29418/x）→ 去尾部 .git → 去尾 /。
// 空串原样返回。幂等（结果再 canon 不变）。
func CanonRepoAddr(addr string) string {
	s := strings.TrimSpace(addr)
	if s == "" {
		return s
	}
	s = strings.ToLower(s)
	// 注意 "ssh://git@" 必须排在 "ssh://" 之前命中（更具体的前缀优先）；裸 ssh://user:tok@host
	// 形式也要剥掉 scheme，否则 authority 段被误判为 "ssh:"，userinfo(凭据)漏剥泄露。
	for _, prefix := range []string{"ssh://git@", "ssh://", "git@", "http://", "https://"} {
		if strings.HasPrefix(s, prefix) {
			s = strings.TrimPrefix(s, prefix)
			break
		}
	}
	// 剥离 URL userinfo（user[:password]@host，如 http://oauth2:glpat-xxx@gitlab/...）：
	// 凭据不属于仓库身份，带 token 与不带 token 是同一仓库；token 也绝不能进 need 边界 key。
	// 只看第一个 "/" 之前的 authority 段，取最后一个 "@" 之后的部分。
	if slash := strings.IndexByte(s, '/'); slash != 0 {
		authority := s
		if slash > 0 {
			authority = s[:slash]
		}
		if at := strings.LastIndexByte(authority, '@'); at >= 0 {
			s = s[at+1:]
		}
	}
	// host 后（第一个 "/" 之前）的冒号：scp 风格换成 "/"；冒号后第一段是纯数字端口则保留。
	slash := strings.IndexByte(s, '/')
	colon := strings.IndexByte(s, ':')
	if colon >= 0 && (slash < 0 || colon < slash) {
		rest := s[colon+1:]
		seg := rest
		if i := strings.IndexByte(rest, '/'); i >= 0 {
			seg = rest[:i]
		}
		if seg != "" && !isAllDigits(seg) {
			s = s[:colon] + "/" + rest
		}
	}
	s = strings.TrimRight(s, "/")
	s = strings.TrimSuffix(s, ".git")
	s = strings.TrimRight(s, "/")
	return s
}

// isAllDigits 判断字符串是否全为 ASCII 数字（用于识别 host:port 的端口段）。
func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}
