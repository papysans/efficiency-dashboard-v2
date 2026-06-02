package main

// 内网 portal(opencode) 的 2b auth shim：无真实鉴权，仅供内网内部看板使用。
// opencode 前端启动时 AuthGuard 会请求 /api/auth/me 与 /api/auth/permissions，
// 缺失会跳登录死循环；内网决定不部署 casdoor，故这里直接返回写死的 admin 用户
// 与只含 kanban 的菜单，让 AuthGuard 通过。
// 如将来接入 casdoor，应移除本文件并去掉 main.go 中 /api/auth 的注册。

import "github.com/gin-gonic/gin"

// authShimMe 返回写死的 admin 用户。前端 src/context/auth.tsx 的 fetchUser 读取 payload.user。
func authShimMe(c *gin.Context) {
	c.JSON(200, gin.H{
		"user": gin.H{
			"id":                 "admin",
			"username":           "admin",
			"name":               "管理员",
			"preferred_username": "admin",
			"displayName":        "管理员",
			"email":              "admin@local",
		},
	})
}

// authShimPermissions 仅放行 kanban 菜单。前端 fetchPermissions 读取 payload.menus。
func authShimPermissions(c *gin.Context) {
	c.JSON(200, gin.H{
		"menus":        []string{"kanban"},
		"capabilities": []string{},
	})
}

// authShimLogout 占位登出，无真实会话需要清理。
func authShimLogout(c *gin.Context) {
	c.JSON(200, gin.H{"ok": true})
}
