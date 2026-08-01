package main

import (
	"strings"

	"kanban/kbcli/internal/logx"

	"github.com/spf13/cobra"
)

// warnIgnoredRemoteFlags 在 --remote 模式下提示哪些本地 flag 不会生效。
//
// 服务端（exec_cmd.go）对路径类与 DSN 类参数一律取自身配置，不再读请求体——
// 否则任何能访问 kbcli serve 的请求都能指定任意写入路径或任意数据库连接串。
// 这里显式告知，避免运维以为传了 --to-csv / --from-db 却发现没生效。
func warnIgnoredRemoteFlags(cmd *cobra.Command, flagNames ...string) {
	var ignored []string
	for _, name := range flagNames {
		if cmd.Flags().Changed(name) {
			ignored = append(ignored, "--"+name)
		}
	}
	if len(ignored) > 0 {
		logx.Warnf("--remote 模式下以下参数由服务端配置决定，本地指定已忽略: %s", strings.Join(ignored, " "))
	}
}
