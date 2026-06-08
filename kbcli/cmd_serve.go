package main

import (
	"context"
	"fmt"
	"kanban/kbcli/internal/appconfig"
	"kanban/kbcli/internal/logx"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "kanban/kbcli/docs"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

var taskQueue *TaskQueue

// validTaskTypes 定义支持的任务类型
var validTaskTypes = map[string]bool{
	"import":        true,
	"import-conv":   true,
	"import-repo":   true,
	"import-org":    true,
	"efficiency-v2": true,
	"fix-task":      true,
	"fix-commit":    true,
}

// HealthResponse 健康检查响应
// swagger:response HealthResponse
type HealthResponse struct {
	Status string `json:"status" example:"ok"`
}

// ErrorResponse 错误响应
// swagger:response ErrorResponse
type ErrorResponse struct {
	Error string `json:"error" example:"任务不存在"`
}

// CreateTaskResponse 创建任务响应
// swagger:response CreateTaskResponse
type CreateTaskResponse struct {
	TaskId string `json:"task_id" example:"abc123"`
	Status string `json:"status" example:"pending"`
	Type   string `json:"type" example:"import-conv"`
}

// CancelTaskResponse 取消任务响应
// swagger:response CancelTaskResponse
type CancelTaskResponse struct {
	TaskId  string `json:"task_id" example:"abc123"`
	Status  string `json:"status" example:"cancelled"`
	Message string `json:"message" example:"任务已取消"`
}

// TaskListResponse 任务列表响应
// swagger:response TaskListResponse
type TaskListResponse struct {
	Tasks []*AsyncTask `json:"tasks"`
	Total int          `json:"total" example:"1"`
}

// importConvBody import-conv 请求体
type importConvBody struct {
	TaskDir      string `json:"task_dir" example:"/path/to/task"`
	AnalysedDir  string `json:"analysed_dir" example:"./analysed"`
	Force        bool   `json:"force" example:"false"`
	StartDate    string `json:"start_date" example:"2024-01-01"`
	EndDate      string `json:"end_date" example:"2024-01-31"`
	CreatePseudo bool   `json:"create_pseudo" example:"false"`
}

// ImportRepoBody import-repo 请求体
type ImportRepoBody struct {
	RepoDir     string `json:"repo_dir" example:"/path/to/repo"`
	AnalysedDir string `json:"analysed_dir" example:"./analysed"`
	Force       bool   `json:"force" example:"false"`
}

// ImportOrgBody import-org 请求体
type ImportOrgBody struct {
	FromDB  string `json:"from_db" example:"host=localhost dbname=auth"`
	FromCSV string `json:"from_csv" example:"./org.csv"`
	ToCSV   string `json:"to_csv" example:"./output.csv"`
}

// SilicaBody silica 请求体
type SilicaBody struct {
	AnalysedDir string `json:"analysed_dir" example:"./analysed"`
	Force       bool   `json:"force" example:"false"`
	MaxDays     int    `json:"max_days" example:"7"`
}

// FixTaskBody fix-task 请求体
type FixTaskBody struct {
	TaskDir   string `json:"task_dir" example:"/path/to/task"`
	StartDate string `json:"start_date" example:"2024-01-01"`
	EndDate   string `json:"end_date" example:"2024-01-31"`
	Date      string `json:"date" example:"20240101"`
	Task      string `json:"task" example:"task-id-123"`
	Max       int    `json:"max" example:"10"`
}

// FixCommitBody fix-commit 请求体
type FixCommitBody struct {
	RepoDir   string `json:"repo_dir" example:"/path/to/repo"`
	StartDate string `json:"start_date" example:"2024-01-01"`
	EndDate   string `json:"end_date" example:"2024-01-31"`
	Date      string `json:"date" example:"20240101"`
	Commit    string `json:"commit" example:"abc123"`
	Max       int    `json:"max" example:"10"`
}

// ImportBody import 请求体
type ImportBody struct {
	TaskDir      string `json:"task_dir" example:"/path/to/task"`
	RepoDir      string `json:"repo_dir" example:"/path/to/repo"`
	AnalysedDir  string `json:"analysed_dir" example:"./analysed"`
	Force        bool   `json:"force" example:"false"`
	FromDB       string `json:"from_db" example:"host=localhost dbname=auth"`
	FromCSV      string `json:"from_csv" example:"./org.csv"`
	Date         string `json:"date" example:"20240101"`
	CreatePseudo bool   `json:"create_pseudo" example:"false"`
}

func writeError(c *gin.Context, status int, message string) {
	c.JSON(status, ErrorResponse{Error: message})
}

// healthHandler 健康检查接口
// @Summary 健康检查
// @Description 检查服务器是否正常运行
// @Tags health
// @Produce json
// @Success 200 {object} HealthResponse
// @Router /health [get]
func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, HealthResponse{Status: "ok"})
}

// createTaskHandler 创建异步任务
// @Summary 提交 import-conv 异步任务
// @Description 将 import-conv 逻辑以异步任务方式提交到队列执行
// @Tags tasks
// @Accept json
// @Produce json
// @Param body body importConvBody true "请求参数"
// @Success 202 {object} CreateTaskResponse
// @Failure 400 {object} ErrorResponse
// @Failure 405 {object} ErrorResponse
// @Router /api/tasks/import-conv [post]
func createimportConvHandler(c *gin.Context) {
	createTaskHandlerFunc("import-conv", c)
}

// createImportRepoHandler 创建 import-repo 异步任务
// @Summary 提交 import-repo 异步任务
// @Description 将 import-repo 逻辑以异步任务方式提交到队列执行
// @Tags tasks
// @Accept json
// @Produce json
// @Param body body ImportRepoBody true "请求参数"
// @Success 202 {object} CreateTaskResponse
// @Failure 400 {object} ErrorResponse
// @Failure 405 {object} ErrorResponse
// @Router /api/tasks/import-repo [post]
func createImportRepoHandler(c *gin.Context) {
	createTaskHandlerFunc("import-repo", c)
}

// createImportOrgHandler 创建 import-org 异步任务
// @Summary 提交 import-org 异步任务
// @Description 将 import-org 逻辑以异步任务方式提交到队列执行
// @Tags tasks
// @Accept json
// @Produce json
// @Param body body ImportOrgBody true "请求参数"
// @Success 202 {object} CreateTaskResponse
// @Failure 400 {object} ErrorResponse
// @Failure 405 {object} ErrorResponse
// @Router /api/tasks/import-org [post]
func createImportOrgHandler(c *gin.Context) {
	createTaskHandlerFunc("import-org", c)
}

// createSilicaHandler 创建 silica 异步任务
// @Summary 提交 silica 异步任务
// @Description 将 silica 逻辑以异步任务方式提交到队列执行
// @Tags tasks
// @Accept json
// @Produce json
// @Param body body SilicaBody true "请求参数"
// @Success 202 {object} CreateTaskResponse
// @Failure 400 {object} ErrorResponse
// @Failure 405 {object} ErrorResponse
// @Router /api/tasks/silica [post]
func createSilicaHandler(c *gin.Context) {
	createTaskHandlerFunc("silica", c)
}

// createImportHandler 创建 import 异步任务
// @Summary 提交 import 异步任务
// @Description 将 import 逻辑以异步任务方式提交到队列执行，顺序执行 import-conv → import-repo → import-org → import-dept → efficiency-v2
// @Tags tasks
// @Accept json
// @Produce json
// @Param body body ImportBody true "请求参数"
// @Success 202 {object} CreateTaskResponse
// @Failure 400 {object} ErrorResponse
// @Failure 405 {object} ErrorResponse
// @Router /api/tasks/import [post]
func createImportHandler(c *gin.Context) {
	createTaskHandlerFunc("import", c)
}

// createFixTaskHandler 创建 fix-task 异步任务
// @Summary 提交 fix-task 异步任务
// @Description 将 fix-task 逻辑以异步任务方式提交到队列执行
// @Tags tasks
// @Accept json
// @Produce json
// @Param body body FixTaskBody true "请求参数"
// @Success 202 {object} CreateTaskResponse
// @Failure 400 {object} ErrorResponse
// @Failure 405 {object} ErrorResponse
// @Router /api/tasks/fix-task [post]
func createFixTaskHandler(c *gin.Context) {
	createTaskHandlerFunc("fix-task", c)
}

// createFixCommitHandler 创建 fix-commit 异步任务
// @Summary 提交 fix-commit 异步任务
// @Description 将 fix-commit 逻辑以异步任务方式提交到队列执行
// @Tags tasks
// @Accept json
// @Produce json
// @Param body body FixCommitBody true "请求参数"
// @Success 202 {object} CreateTaskResponse
// @Failure 400 {object} ErrorResponse
// @Failure 405 {object} ErrorResponse
// @Router /api/tasks/fix-commit [post]
func createFixCommitHandler(c *gin.Context) {
	createTaskHandlerFunc("fix-commit", c)
}

func createTaskHandlerFunc(taskType string, c *gin.Context) {
	var params map[string]interface{}
	if err := c.ShouldBindJSON(&params); err != nil {
		writeError(c, http.StatusBadRequest, fmt.Sprintf("解析请求体失败: %v", err))
		return
	}

	if params == nil {
		params = make(map[string]interface{})
	}

	fn, err := createTaskExecutor(taskType, params)
	if err != nil {
		writeError(c, http.StatusBadRequest, err.Error())
		return
	}

	task := taskQueue.Submit(taskType, params, fn)
	c.JSON(http.StatusAccepted, CreateTaskResponse{
		TaskId: task.ID,
		Status: string(task.Status),
		Type:   task.Type,
	})
}

// listTasksHandler 列举所有异步任务
// @Summary 列举异步任务
// @Description 获取所有异步任务的列表
// @Tags tasks
// @Produce json
// @Success 200 {object} TaskListResponse
// @Router /api/tasks [get]
func listTasksHandler(c *gin.Context) {
	tasks := taskQueue.List()
	c.JSON(http.StatusOK, TaskListResponse{
		Tasks: tasks,
		Total: len(tasks),
	})
}

// getTaskHandler 查询单个异步任务
// @Summary 查询异步任务
// @Description 通过任务ID获取异步任务的详细信息和执行结果
// @Tags tasks
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} AsyncTask
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 405 {object} ErrorResponse
// @Router /api/tasks/{id} [get]
func getTaskHandler(c *gin.Context) {
	taskID := c.Param("id")
	if taskID == "" {
		writeError(c, http.StatusBadRequest, "缺少任务ID")
		return
	}

	task, ok := taskQueue.Get(taskID)
	if !ok {
		writeError(c, http.StatusNotFound, "任务不存在")
		return
	}

	c.JSON(http.StatusOK, task)
}

// cancelTaskHandler 取消异步任务
// @Summary 取消异步任务
// @Description 通过任务ID取消指定的异步任务
// @Tags tasks
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} CancelTaskResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 405 {object} ErrorResponse
// @Router /api/tasks/{id} [delete]
func cancelTaskHandler(c *gin.Context) {
	taskID := c.Param("id")
	if taskID == "" {
		writeError(c, http.StatusBadRequest, "缺少任务ID")
		return
	}

	task, ok := taskQueue.Cancel(taskID)
	if !ok {
		writeError(c, http.StatusNotFound, "任务不存在")
		return
	}

	c.JSON(http.StatusOK, CancelTaskResponse{
		TaskId:  task.ID,
		Status:  string(task.Status),
		Message: "任务已取消",
	})
}

// setupRouter 设置HTTP路由
func setupRouter() *gin.Engine {
	r := gin.Default()

	// CORS 配置
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	// Swagger UI
	r.GET("/swagger", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	})
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Prometheus 指标
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// 健康检查
	r.GET("/health", healthHandler)

	// 任务创建接口
	r.POST("/api/tasks/import", createImportHandler)
	r.POST("/api/tasks/import-conv", createimportConvHandler)
	r.POST("/api/tasks/import-repo", createImportRepoHandler)
	r.POST("/api/tasks/import-org", createImportOrgHandler)
	r.POST("/api/tasks/silica", createSilicaHandler)
	r.POST("/api/tasks/fix-task", createFixTaskHandler)
	r.POST("/api/tasks/fix-commit", createFixCommitHandler)

	// 任务管理接口
	r.GET("/api/tasks", listTasksHandler)
	r.GET("/api/tasks/:id", getTaskHandler)
	r.DELETE("/api/tasks/:id", cancelTaskHandler)

	return r
}

func startInit(queue *TaskQueue) {
	if appconfig.Cfg.Serve.Init.Command == "" {
		logx.Debug("init command is emptied")
		return
	}
	cmd := appconfig.Cfg.Serve.Init
	if !validTaskTypes[cmd.Command] {
		logx.Warnf("跳过未知命令类型的定时任务: %s", cmd.Command)
		return
	}

	params := cmd.Params
	if cmd.Params == nil {
		params = make(map[string]interface{})
	}

	fn, err := createTaskExecutor(cmd.Command, params)
	if err != nil {
		logx.Warnf("创建初始化任务执行器失败: %v", err)
		return
	}

	task := queue.Submit(cmd.Command, params, fn)

	logx.Infof("[init] 已启动初始化任务(id=%s): command=%s, params=%v", task.ID, cmd.Command, cmd.Params)
}

// startCron 启动定时任务调度器
func startCron(queue *TaskQueue) *cron.Cron {
	if len(appconfig.Cfg.Serve.Crontab) == 0 {
		logx.Info("没有配置定时任务，跳过cron启动")
		return nil
	}

	c := cron.New(cron.WithSeconds())
	for _, job := range appconfig.Cfg.Serve.Crontab {
		if job.Schedule == "" || job.Command == "" {
			logx.Warnf("跳过无效定时任务配置: schedule=%s, command=%s", job.Schedule, job.Command)
			continue
		}
		if !validTaskTypes[job.Command] {
			logx.Warnf("跳过未知命令类型的定时任务: %s", job.Command)
			continue
		}

		cmd := job.Command
		params := job.Params
		if params == nil {
			params = make(map[string]interface{})
		}

		fn, err := createTaskExecutor(cmd, params)
		if err != nil {
			logx.Warnf("创建定时任务执行器失败 [%s %s]: %v", job.Schedule, cmd, err)
			continue
		}

		entryID, err := c.AddFunc(job.Schedule, func() {
			logx.Infof("[cron] 定时任务触发: %s", cmd)
			task := queue.Submit(cmd, params, fn)
			logx.Infof("[cron] 任务已提交: %s (type=%s)", task.ID, cmd)
		})
		if err != nil {
			logx.Warnf("添加定时任务失败 [%s %s]: %v", job.Schedule, cmd, err)
			continue
		}
		logx.Infof("[cron] 已注册定时任务: schedule=%s, command=%s, entryID=%d", job.Schedule, cmd, entryID)
	}

	c.Start()
	logx.Info("[cron] 定时任务调度器已启动")
	return c
}

// @title           Kbcli API
// @version         1.0
// @description     Kanban CLI HTTP服务API，提供健康检查、异步任务提交与查询、定时任务调度等接口
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.swagger.io/support
// @contact.email  support@swagger.io

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "启动HTTP服务器，提供RESTful API和定时任务调度",
	Long: `启动HTTP服务器，支持以下功能：
  1. RESTful API: 健康检查、异步任务提交、任务查询与取消
  2. 定时任务: 根据config.yaml中的crontab配置定时执行命令`,
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetInt("port")
		if port <= 0 {
			port = appconfig.Cfg.Serve.Port
		}
		if port <= 0 {
			port = 8080
		}

		// 初始化任务队列
		workers, _ := cmd.Flags().GetInt("workers")
		if workers <= 0 {
			workers = 2
		}
		taskQueue = NewTaskQueue(workers)
		defer taskQueue.Stop()

		// 启动初始化任务
		startInit(taskQueue)
		// 启动定时任务
		cronInst := startCron(taskQueue)
		if cronInst != nil {
			defer cronInst.Stop()
		}

		// 设置HTTP路由
		r := setupRouter()
		server := &http.Server{
			Addr:         fmt.Sprintf(":%d", port),
			Handler:      r,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		}

		// 优雅关闭
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			logx.Infof("[serve] HTTP服务器启动，监听端口: %d", port)
			logx.Infof("[serve] Swagger文档地址: http://127.0.0.1:%d/swagger/index.html", port)
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logx.Errorf("[serve] HTTP服务器异常: %v", err)
			}
		}()

		<-quit
		logx.Info("[serve] 正在关闭服务器...")

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			logx.Warnf("[serve] 服务器关闭异常: %v", err)
		}

		logx.Info("[serve] 服务器已关闭")
		return nil
	},
}

func init() {
	serveCmd.Flags().SortFlags = false
	serveCmd.Flags().Int("port", 0, "HTTP服务器端口，默认使用config.yaml中serve.port或8080")
	serveCmd.Flags().Int("workers", 2, "异步任务工作协程数")
	rootCmd.AddCommand(serveCmd)
}
