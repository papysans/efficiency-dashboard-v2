package main

import (
	"fmt"
	"kanban/backend/internal/appconfig"
	"log"
	"net/http"

	_ "kanban/backend/docs"

	"kanban/core/models"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/gorm"
)

// @title           Efficiency Dashboard API
// @version         1.0
// @description     效率仪表盘后端API，提供任务、提交、用户、仓库、项目、组织等数据管理接口
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.swagger.io/support
// @contact.email  support@swagger.io

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

var statDB *gorm.DB

func healthCheck(c *gin.Context) {
	status := "ok"
	httpCode := http.StatusOK

	if statDB != nil {
		sqlDB, err := statDB.DB()
		if err != nil || sqlDB.Ping() != nil {
			status = "db_error"
			httpCode = http.StatusServiceUnavailable
		}
	} else {
		status = "db_not_connected"
		httpCode = http.StatusServiceUnavailable
	}

	c.JSON(httpCode, gin.H{"status": status})
}

func main() {
	var err error
	c, err := appconfig.LoadFirstConfig([]string{"config.yaml", "configs/server-config.yaml", "server-config.yaml", "../server-config.yaml"})
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	appconfig.Cfg = *c

	statDB, err = models.OpenGormDB(appconfig.Cfg.StatDatabase.DSN())
	if err != nil {
		log.Fatalf("costrict_stat数据库初始化失败: %v", err)
	}
	log.Println("costrict_stat数据库连接成功")

	if statDB != nil {
		maps, err := LoadUserOrgs(statDB)
		if err != nil {
			log.Printf("警告: 从 user_org 表加载组织映射失败: %v", err)
		} else {
			orgMappings = maps
			log.Printf("已从 user_org 表加载 %d 条组织映射", len(orgMappings))
		}
	}

	r := gin.Default()

	r.GET("/swagger", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	})
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	r.GET("/healthz", healthCheck)

	r.GET("/metrics", MetricsHandler())

	r.Use(MetricsMiddleware())

	corsOrigins := appconfig.Cfg.CORS.AllowOrigins
	if len(corsOrigins) == 0 {
		corsOrigins = []string{"*"}
	}
	r.Use(cors.New(cors.Config{
		AllowOrigins:     corsOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
	}))

	api := r.Group("/api")

	v2 := api.Group("/v2")
	{
		v2.GET("/dashboard/summary", getDashboardSummary)

		v2.GET("/sessions", listSessionsV2)
		v2.GET("/sessions/:session_id", getSessionDetailV2)

		v2.GET("/tasks", listTasksV2)
		v2.GET("/tasks/file", getTaskFile)
		v2.GET("/tasks/:taskId", getTaskDetailV2)
		v2.PUT("/tasks/:taskId/manual", updateTaskManualV2)

		v2.GET("/commits", listCommitsV2)
		v2.GET("/commits/:commitId", getCommitDetailV2)
		v2.PUT("/commits/:commitId/manual", updateCommitManualV2)

		v2.GET("/users", listUsersV2Native)
		v2.GET("/users/:userId", getUserV2DetailNative)
		v2.GET("/user-names", getUserNamesV2)

		v2.GET("/repos", listReposV2)
		v2.GET("/repos/detail", getRepoDetailV2)
		v2.GET("/repos/branches", listRepoBranchesV2)

		v2.GET("/orgs", listOrgsV2Native)
		v2.GET("/orgs/detail", getOrgDetailV2)
		v2.POST("/orgs/refresh", refreshOrgMappingV2)
		v2.GET("/group", getGroupDetailV2)

		// 组织树（dept-sync 权威全量树 + API 懒加载，代理 dept-sync /department/*）
		v2.GET("/dept-tree", getDeptTreeV2)
		v2.GET("/dept-tree/members", getDeptTreeMembersV2)

		v2.POST("/projects", createProjectV2)
		v2.GET("/projects", listProjectsV2)
		v2.POST("/projects/check-conflicts", checkProjectConflictsV2)
		v2.GET("/projects/:projectId", getProjectDetailV2)
		v2.PUT("/projects/:projectId", updateProjectV2)
		v2.DELETE("/projects/:projectId", deleteProjectV2)
		v2.PUT("/projects/:projectId/manual", updateProjectManualV2)
		v2.POST("/projects/:projectId/tasks", addTasksToProjectV2)
		v2.POST("/projects/:projectId/repos", addRepoToProjectV2)
		v2.DELETE("/projects/:projectId/repos/:index", removeRepoFromProjectV2)
		v2.DELETE("/projects/:projectId/tasks", removeTasksFromProjectV2)
		v2.PUT("/projects/:projectId/tasks/silica", updateTaskSilicaInProjectV2)

		v2.POST("/user-groups", createUserGroupHandler)
		v2.DELETE("/user-groups/:groupId", deleteUserGroupHandler)
		v2.GET("/user-groups/:groupId", getUserGroupDetailHandler)

		v2.GET("/config", getConfigV2)

		v2.GET("/needs", listNeedsV2)
		// /needs/*needId 是 catch-all 路由；gin 不允许同级再注册静态 /needs/distribution（会 panic），
		// 故 /needs/distribution 在 getNeedV2 内按 needId=="distribution" 分发到 getNeedsDistributionV2。
		v2.GET("/needs/*needId", getNeedV2)
		v2.GET("/efficiency", getEfficiencyV2Aggregate)
	}

	// 内网 portal(opencode) 的 2b auth shim：无 casdoor 时让 AuthGuard 通过。详见 auth_shim_handler.go。
	auth := api.Group("/auth")
	{
		auth.GET("/me", authShimMe)
		auth.GET("/permissions", authShimPermissions)
		auth.POST("/logout", authShimLogout)
	}

	port := appconfig.Cfg.Server.Port
	if port == 0 {
		port = 9990
	}
	log.Printf("服务器启动在端口 %d", port)
	log.Printf("Swagger文档地址: http://localhost:%d/swagger/index.html", port)
	if err := r.Run(fmt.Sprintf(":%d", port)); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
