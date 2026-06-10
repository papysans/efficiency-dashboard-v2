package main

import (
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"kanban/backend/internal/appconfig"

	"github.com/gin-gonic/gin"
)

// chat-indicator-statistics 平台客观指标服务（token/成本/时延/错误）的反向代理。
// 把 /api/v2/chat/<path> 全量透传为 <chat_stats.base_url>/chat-indicator-statistics/api/v1/<path>
// （method/query/body/header 原样转发；上游响应是裸 JSON，不套看板 {data,error} 包装）。
// chat 服务自身无鉴权，靠内网 compose 网络隔离（不 publish 端口）+ 看板 auth shim 兜安全。
// 配置模式照 dept_tree_handler.go 的 dept_sync：base_url 空 = 功能关闭。

// chatStatsAPIPrefix chat-indicator-statistics 服务自带的 API 路由前缀，代理时拼在 base_url 后。
const chatStatsAPIPrefix = "/chat-indicator-statistics/api/v1"

// chatStatsTransport 代理用的共享 Transport。
// 实时查询（/stats/realtime、/stats/detail/query）直查源库可能较慢，响应头超时放宽到 60s。
var chatStatsTransport = &http.Transport{
	DialContext:           (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
	ResponseHeaderTimeout: 60 * time.Second,
}

// proxyChatStatsV2 ANY /api/v2/chat/*path
// @Summary 代理 chat-indicator-statistics 全量 API
// @Description 把 /api/v2/chat/<path> 反代为 chat-indicator-statistics 的 /chat-indicator-statistics/api/v1/<path>，透传 method/query/body/header
// @Tags ChatStats
// @Produce json
// @Failure 502 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /api/v2/chat/{path} [get]
func proxyChatStatsV2(c *gin.Context) {
	baseURL := strings.TrimSpace(appconfig.Cfg.ChatStats.BaseURL)
	if baseURL == "" {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "chat stats not configured"})
		return
	}
	target, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil || target.Scheme == "" || target.Host == "" {
		log.Printf("[ERROR] chat-stats 代理: chat_stats.base_url 配置非法 %q: %v", baseURL, err)
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "chat stats base_url 配置非法"})
		return
	}

	subPath := c.Param("path") // 含前导 "/"，如 /pricing/models
	// 上游 Basic Auth（外网实例由 nginx 401 保护）：username/password 均非空才注入，否则原样透传。
	authUser := strings.TrimSpace(appconfig.Cfg.ChatStats.Username)
	authPass := strings.TrimSpace(appconfig.Cfg.ChatStats.Password)
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.URL.Path = target.Path + chatStatsAPIPrefix + subPath
			req.Host = target.Host
			if authUser != "" && authPass != "" {
				req.SetBasicAuth(authUser, authPass)
			}
			// 其余 query/body/header 留在 req 上原样透传，不改写。
		},
		Transport: chatStatsTransport,
		ErrorHandler: func(w http.ResponseWriter, req *http.Request, err error) {
			log.Printf("[ERROR] chat-stats 代理失败 [%s %s]: %v", req.Method, subPath, err)
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":"chat stats upstream unreachable"}`))
		},
	}
	proxy.ServeHTTP(c.Writer, c.Request)
}
