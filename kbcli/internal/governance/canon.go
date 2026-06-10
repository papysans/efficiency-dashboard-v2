package governance

import "strings"

// CanonRepoAddr 规范化 repo 地址写法（http/ssh 协议、大小写、.git 后缀等写法分裂归一），
// 使同一仓库的不同写法在聚合时合并统计。规则按序：
// trim → lower → 去前缀 ssh://git@ / git@ / http:// / https:// →
// host 后第一个 ":" 换 "/"（scp 风格 git@host:org/repo；冒号后第一段是纯数字端口时保留，
// 如 http://10.129.0.33:1080/x、ssh://git@host:29418/x）→ 去尾部 .git → 去尾 /。
// 空串原样返回。
func CanonRepoAddr(addr string) string {
	s := strings.TrimSpace(addr)
	if s == "" {
		return s
	}
	s = strings.ToLower(s)
	for _, prefix := range []string{"ssh://git@", "git@", "http://", "https://"} {
		if strings.HasPrefix(s, prefix) {
			s = strings.TrimPrefix(s, prefix)
			break
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
