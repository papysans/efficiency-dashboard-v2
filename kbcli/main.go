package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// loadDotEnv 加载项目根目录的 .env 文件，不覆盖已有的环境变量
func loadDotEnv() {
	// 尝试当前目录和上级目录（兼容从 kbcli/ 子目录运行的情况）
	for _, path := range []string{".env", "../.env"} {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			idx := strings.IndexByte(line, '=')
			if idx < 0 {
				continue
			}
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+1:])
			if ci := strings.Index(val, " #"); ci >= 0 {
				val = strings.TrimSpace(val[:ci])
			}
			if os.Getenv(key) == "" {
				os.Setenv(key, val)
			}
		}
		break
	}
}

func main() {
	loadDotEnv()

	// 加载配置
	config, err := LoadConfig("config.yaml")
	if err != nil {
		// 尝试上级目录
		config, err = LoadConfig("../config.yaml")
		if err != nil {
			fmt.Printf("加载配置文件失败: %v\n", err)
			os.Exit(1)
		}
	}

	// 分发子命令
	RunCLI(config)
}
