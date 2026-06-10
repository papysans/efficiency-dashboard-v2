package appconfig

import (
	"fmt"
	"log"
	"os"
	"strings"

	"kanban/core/config"

	"gopkg.in/yaml.v3"
)

// Config 应用配置
type Config struct {
	Server struct {
		Port int `yaml:"port"`
	} `yaml:"server"`
	StatDatabase config.DatabaseConfig `yaml:"stat_database"`
	TaskDir      string                `yaml:"task_dir"`
	AnalysedDir  string                `yaml:"analysed_dir"`
	CORS         struct {
		AllowOrigins []string `yaml:"allow_origins"`
	} `yaml:"cors"`
	TraditionalDevLinesPerDay int             `yaml:"traditional_dev_lines_per_day"`
	CostPerPersonDay          float64         `yaml:"cost_per_person_day"`
	DashboardTitlePrefix      string          `yaml:"dashboard_title_prefix"`
	DeptSync                  DeptSyncConfig  `yaml:"dept_sync"`
	ChatStats                 ChatStatsConfig `yaml:"chat_stats"`
}

// DeptSyncConfig dept-sync 部门同步服务对接配置（组织树代理 handler 使用）。
// 口径与 kbcli DeptSyncConfig 一致：base_url 不含路由前缀；query_key 走数据接口鉴权头 X-Query-Key。
type DeptSyncConfig struct {
	BaseURL  string `yaml:"base_url"`  // dept-sync 服务地址，如 http://127.0.0.1:8080
	QueryKey string `yaml:"query_key"` // 数据接口鉴权头 X-Query-Key 的值
	// RootDeptId 单根公司的 dept_id（优先级最高）。配置后组织树以该节点为唯一根，过滤掉 parent 链断裂的脏数据孤儿部门。
	RootDeptId string `yaml:"root_dept_id"`
	// RootDeptName 单根公司名（RootDeptId 未配置时按名字匹配根节点）。默认 "深信服科技股份有限公司"。
	RootDeptName string `yaml:"root_dept_name"`
}

// DefaultRootDeptName 组织树单根公司默认名（dept-sync /department/tree 返回森林时据此找真正的公司根）。
const DefaultRootDeptName = "深信服科技股份有限公司"

// ChatStatsConfig chat-indicator-statistics 平台客观指标服务对接配置（chat 代理 handler 使用）。
// base_url 不含路由前缀（/chat-indicator-statistics/api/v1 由代理层拼接）；空 = 功能关闭，
// /api/v2/chat/* 返回 503，前端「平台」分组隐藏。
type ChatStatsConfig struct {
	BaseURL string `yaml:"base_url"` // chat-indicator-statistics 服务地址，如 http://chat-indicator-statistics:8080
	// Username/Password 上游 HTTP Basic Auth 账号（外网实例 https://zgsm.sangfor.com 由 nginx 401 保护）。
	// 两者均非空时代理注入 Authorization: Basic；留空 = 不注入（内网栈内 chat-stats 无鉴权场景）。
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

var Cfg Config

func loadConfig(path string) (*Config, error) {
	var cfg Config
	cfg.Server.Port = 9990
	cfg.StatDatabase = config.DatabaseConfig{
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
	cfg.TraditionalDevLinesPerDay = DefaultTraditionalDevLinesPerDay
	cfg.CostPerPersonDay = DefaultCostPerPersonDay
	cfg.DeptSync.RootDeptName = DefaultRootDeptName

	data, err := os.ReadFile(path)
	if err != nil {
		return &cfg, nil
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return &cfg, fmt.Errorf("解析 config.yaml 失败: %w", err)
	}
	// 配置未显式给 root_dept_name 时回落默认，避免 yaml 把空键覆成空串。
	if strings.TrimSpace(cfg.DeptSync.RootDeptName) == "" {
		cfg.DeptSync.RootDeptName = DefaultRootDeptName
	}
	cfg.DashboardTitlePrefix = strings.TrimSpace(cfg.DashboardTitlePrefix)
	return &cfg, nil
}

func LoadFirstConfig(files []string) (*Config, error) {
	for _, fname := range files {
		if fname == "" {
			continue
		}
		if _, err := os.Stat(fname); os.IsNotExist(err) {
			continue
		}
		loadedCfg, err := loadConfig(fname)
		if err == nil {
			log.Printf("load config [%s] ok, cfg: %+v\n", fname, loadedCfg)
			return loadedCfg, nil
		}
		log.Printf("load config [%s] failed: %v\n", fname, err)
	}
	return nil, fmt.Errorf("load config [%s] failed", strings.Join(files, ","))
}
