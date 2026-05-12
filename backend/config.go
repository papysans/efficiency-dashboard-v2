package main

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
	TraditionalDevLinesPerDay int `yaml:"traditional_dev_lines_per_day"`
}

var appConfig Config

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

	data, err := os.ReadFile(path)
	if err != nil {
		return &cfg, nil
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return &cfg, fmt.Errorf("解析 config.yaml 失败: %w", err)
	}
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
