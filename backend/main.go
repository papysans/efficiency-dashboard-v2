package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	_ "kanban/backend/docs"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gopkg.in/yaml.v3"
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

// @host      localhost:9990
// @BasePath  /api

// DatabaseConfig PostgreSQL 数据库连接配置
type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	SSLMode  string `yaml:"sslmode"`
}

// Config 应用配置
type Config struct {
	Server struct {
		Port int `yaml:"port"`
	} `yaml:"server"`
	Database     DatabaseConfig `yaml:"database"`
	StatDatabase DatabaseConfig `yaml:"stat_database"`
	TaskDir      string         `yaml:"task_dir"`
	AnalysedDir  string         `yaml:"analysed_dir"`
	CORS         struct {
		AllowOrigins []string `yaml:"allow_origins"`
	} `yaml:"cors"`
	AIEstimation struct {
		Enabled   bool   `yaml:"enabled"`
		APIKey    string `yaml:"api_key"`
		BaseURL   string `yaml:"base_url"`
		Model     string `yaml:"model"`
		TimeoutMS int    `yaml:"timeout_ms"`
		HTTPProxy string `yaml:"http_proxy"`
	} `yaml:"ai_estimation"`
	TaskRealMinutes struct {
		GapThresholdMinutes int `yaml:"gap_threshold_minutes"`
		ExtensionMinutes    int `yaml:"extension_minutes"`
	} `yaml:"task_real_minutes"`
	TraditionalDevLinesPerDay int `yaml:"traditional_dev_lines_per_day"`
}

var appConfig Config
var reportDB *sql.DB
var statDB *sql.DB

func loadConfig(path string) (Config, error) {
	var cfg Config
	// 默认值
	cfg.Server.Port = 9990
	cfg.Database = DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "1",
		DBName:   "report",
		SSLMode:  "disable",
	}
	cfg.StatDatabase = DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "1",
		DBName:   "costrict_stat",
		SSLMode:  "disable",
	}
	cfg.TaskDir = "../task"
	cfg.AnalysedDir = "../task"
	cfg.CORS.AllowOrigins = []string{"http://localhost:8880"}
	cfg.TaskRealMinutes.GapThresholdMinutes = 30
	cfg.TaskRealMinutes.ExtensionMinutes = 5
	cfg.TraditionalDevLinesPerDay = DefaultTraditionalDevLinesPerDay

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, nil // 文件不存在时使用默认值
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("解析 config.yaml 失败: %w", err)
	}
	if cfg.AIEstimation.TimeoutMS == 0 {
		cfg.AIEstimation.TimeoutMS = 120000
	}
	if cfg.AIEstimation.Model == "" {
		cfg.AIEstimation.Model = "claude-sonnet-4-20250514"
	}
	return cfg, nil
}

func healthCheck(c *gin.Context) {
	status := "ok"
	httpCode := http.StatusOK

	if reportDB != nil {
		if err := reportDB.Ping(); err != nil {
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
	appConfig, err = loadConfig("config.yaml")
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 初始化 report 数据库连接
	reportDB, err = InitDB(appConfig.Database)
	if err != nil {
		log.Fatalf("report数据库初始化失败: %v", err)
	}
	log.Println("report数据库连接成功")

	// 初始化 costrict_stat 数据库连接
	statDB, err = InitDB(appConfig.StatDatabase)
	if err != nil {
		log.Fatalf("costrict_stat数据库初始化失败: %v", err)
	}
	log.Println("costrict_stat数据库连接成功")

	// 确保 stat 数据库表结构存在（幂等，每次启动执行）
	if statDB != nil {
		if err := EnsureStatSchema(statDB); err != nil {
			log.Fatalf("costrict_stat 数据库表结构初始化失败: %v", err)
		}
		log.Println("costrict_stat 数据库表结构检查完成")
	}

	// 从 user_org 表加载组织映射
	if statDB != nil {
		if err := LoadOrgMappingFromDB(statDB); err != nil {
			log.Printf("警告: 从 user_org 表加载组织映射失败: %v", err)
		} else {
			log.Printf("已从 user_org 表加载 %d 条组织映射", len(orgMappings))
		}
	}

	r := gin.Default()

	// Swagger 文档路由（需要先运行 swag init 生成文档）
	// Swagger 文档路由
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 健康检查路由（不在 /api 下，供 K8s 探针直接访问）
	r.GET("/healthz", healthCheck)

	// CORS 配置
	corsOrigins := appConfig.CORS.AllowOrigins
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

		v2.GET("/tasks", listTasksV2)
		v2.GET("/tasks/file", getTaskFile)
		v2.GET("/tasks/:taskId", getTaskDetailV2)
		v2.PUT("/tasks/:taskId/manual", updateTaskManualV2)
		v2.POST("/tasks/estimate-ancient", estimateAncientMinutes)

		v2.GET("/commits", listCommitsV2)
		v2.GET("/commits/:commitId", getCommitDetailV2)
		v2.PUT("/commits/:commitId/manual", updateCommitManualV2)

		v2.GET("/users", listUsersV2)
		v2.GET("/users/:userId", getUserDetailV2)

		v2.GET("/repos", listReposV2)
		v2.GET("/repos/detail", getRepoDetailV2)
		v2.GET("/repos/branches", listRepoBranchesV2)

		v2.GET("/orgs", listOrgV2)
		v2.GET("/orgs/detail", getOrgDetailV2)
		v2.GET("/group", getGroupDetailV2)

		// Projects
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

		// User Groups
		v2.POST("/user-groups", createUserGroupHandler)
		v2.DELETE("/user-groups/:groupId", deleteUserGroupHandler)
		v2.GET("/user-groups/:groupId", getUserGroupDetailHandler)

		// Config
		v2.GET("/config", getConfigV2)
	}

	port := appConfig.Server.Port
	if port == 0 {
		port = 9990
	}
	log.Printf("服务器启动在端口 %d", port)
	log.Printf("Swagger文档地址: http://localhost:%d/swagger/index.html", port)
	if err := r.Run(fmt.Sprintf(":%d", port)); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
