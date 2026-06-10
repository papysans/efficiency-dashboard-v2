package governance

// CanonRepoAddr 规范化 repo 地址写法（http/ssh 协议、大小写、.git 后缀等写法分裂归一），
// 使同一仓库的不同写法在聚合时合并统计。
// TODO(W3): 由并行流 W3 实现，当前原样返回。
func CanonRepoAddr(addr string) string {
	return addr
}
