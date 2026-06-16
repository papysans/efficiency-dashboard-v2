package governance

import "kanban/core/utils"

// CanonRepoAddr 规范化 repo 地址写法（协议/大小写/.git/userinfo 归一）。
// 实现已迁至 core/utils（kbcli 写入与 backend 读侧匹配共用的单一真源），
// 此处保留 governance 入口供本包及 efficiencyv2 既有调用复用，避免大面积改 import。
func CanonRepoAddr(addr string) string {
	return utils.CanonRepoAddr(addr)
}
