package main

import (
	"bufio"
	"os"
	"strings"

	_ "kanban/kbcli/docs"
)

// @title KBCLI Serve API
// @version 1.0
// @description kbcli serve RESTful API，支持异步任务管理和定时任务调度
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /

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
	Execute()
}
