package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

type TaskSummary struct {
	TaskID        string `json:"task_id"`
	UserID        string `json:"user_id"`
	UserName      string `json:"user_name"`
	ClientID      string `json:"client_id"`
	ClientIDE     string `json:"client_ide"`
	ClientVersion string `json:"client_version"`
	ClientOS      string `json:"client_os"`
	ClientOSVer   string `json:"client_os_version"`
	Caller        string `json:"caller"`
	RepoAddr      string `json:"repo_addr"`
	RepoBranch    string `json:"repo_branch"`
	WorkDir       string `json:"work_dir"`
	Diff          string `json:"diff"`
	DiffLines     int    `json:"diff_lines"`
}

type ConversationEntry struct {
	Sender          string  `json:"sender"`
	RequestID       string  `json:"request_id"`
	PromptMode      string  `json:"prompt_mode"`
	Mode            string  `json:"mode"`
	Model           string  `json:"model"`
	StartTime       string  `json:"start_time"`
	EndTime         string  `json:"end_time"`
	ProcessTime     int64   `json:"process_time"`
	ProcessTTFT     int64   `json:"process_ttft"`
	UpstreamTokens  int64   `json:"upstream_tokens"`
	DownstreamTokens int64  `json:"downstream_tokens"`
	Cost            float64 `json:"cost"`
	RequestContent  string  `json:"request_content"`
	ResponseContent string  `json:"response_content"`
	UserInput       string  `json:"user_input"`
	Diff            string  `json:"diff"`
	DiffLines       int     `json:"diff_lines"`
	ErrorCode       *string `json:"error_code"`
	ErrorReason     *string `json:"error_reason"`
}

type userInfo struct {
	id, name string
}

type repoInfo struct {
	addr, branch string
}

type clientInfo struct {
	id, ide, version, os, osVersion, workDir string
}

var users = []userInfo{
	{"a3f1b2c4-d5e6-47f8-9a0b-1c2d3e4f5a6b", "张三"},
	{"b7c8d9e0-f1a2-43b4-85c6-d7e8f9a0b1c2", "李四"},
	{"c4d5e6f7-a8b9-40c1-92d3-e4f5a6b7c8d9", "王五"},
	{"d1e2f3a4-b5c6-47d8-a9e0-f1a2b3c4d5e6", "赵六"},
}

var repos = []repoInfo{
	{"https://github.com/example/frontend.git", "main"},
	{"https://github.com/example/backend.git", "dev"},
	{"https://github.com/example/mobile-app.git", "feature-auth"},
	{"https://github.com/example/data-service.git", "release-v2"},
}

var clients = []clientInfo{
	{"a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6", "vscode", "2.5.3", "Windows", "10.0", "d:\\project\\frontend"},
	{"f6e5d4c3b2a1f0e9d8c7b6a5f4e3d2c1", "jetbrains", "3.1.0", "macOS", "14.2", "/Users/dev/backend"},
	{"1234567890abcdef1234567890abcdef", "cli", "1.0.0", "Linux", "5.15", "/home/user/mobile-app"},
	{"abcdef1234567890abcdef1234567890", "vscode", "2.6.0", "Windows", "11.0", "d:\\work\\data-service"},
}

var callers = []string{"chat", "agent", "inline"}
var models = []string{"GLM-4.7", "GLM-5", "Kimi-K2.5-Moonshot"}
var promptModes = []string{"vibe", "normal", "creative"}
var modes = []string{"code", "chat", "review"}

var requestContents = []string{
	"帮我实现一个用户登录功能",
	"这段代码有什么性能问题",
	"帮我写一个单元测试",
	"重构这个函数，提高可读性",
	"帮我添加错误处理逻辑",
	"这个 SQL 查询怎么优化",
	"帮我实现分页功能",
	"代码审查一下这个 PR",
	"帮我写一个 Dockerfile",
	"这个接口需要加什么参数校验",
}

var responseContents = []string{
	"好的，我来帮你实现登录功能。首先需要创建一个认证中间件...",
	"这段代码的主要性能问题在于 N+1 查询，建议使用批量查询替代...",
	"以下是单元测试代码，覆盖了正常流程和边界情况...",
	"重构建议：将复杂条件提取为独立函数，使用 early return 模式...",
	"建议使用 errors.Wrap 包装错误，并在最外层统一处理...",
	"这个查询可以通过添加复合索引来优化，同时建议使用 EXPLAIN 分析...",
	"分页功能实现：使用 OFFSET/LIMIT 配合总数查询...",
	"代码审查发现几个问题：1. 缺少输入验证 2. 错误处理不完整...",
	"Dockerfile 已生成，使用多阶段构建减小镜像体积...",
	"建议添加以下参数校验：必填字段检查、格式验证、长度限制...",
}

var errorCodes = []string{"500", "429", "408", "502", "503"}
var errorReasons = []string{
	"服务器内部错误",
	"请求频率超限",
	"请求超时",
	"网关错误",
	"服务暂时不可用",
}

var diffSnippets = []string{
	"+ func login(username, password string) error {\n+     if username == \"\" {\n+         return errors.New(\"username required\")\n+     }\n+     return nil\n+ }",
	"- old_query = db.Query(sql)\n+ new_query = db.QueryBatch(sql, params)",
	"+ // Add pagination support\n+ func (s *Service) List(page, size int) ([]Item, error) {\n+     offset := (page - 1) * size\n+     return s.repo.Find(offset, size)\n+ }",
	"- fmt.Println(err)\n+ log.Error(\"operation failed\", zap.Error(err))\n+ return fmt.Errorf(\"operation failed: %w\", err)",
	"+ func TestCreateUser(t *testing.T) {\n+     user := NewUser(\"test\", \"test@example.com\")\n+     assert.NotNil(t, user)\n+     assert.Equal(t, \"test\", user.Name)\n+ }",
}

func genUUID(rng *rand.Rand) string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		rng.Uint32(),
		rng.Uint32()&0xFFFF,
		(rng.Uint32()&0x0FFF)|0x4000,
		(rng.Uint32()&0x3FFF)|0x8000,
		rng.Int63()&0xFFFFFFFFFFFF,
	)
}

func main() {
	outputDir := flag.String("output-dir", "../../task", "output directory for generated test data")
	flag.Parse()

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	loc, _ := time.LoadLocation("Asia/Shanghai")

	// 5 days: 2026-04-01 ~ 2026-04-05, 3 tasks per day = 15 tasks
	type dayTasks struct {
		date  time.Time
		count int
	}
	days := []dayTasks{
		{time.Date(2026, 4, 1, 0, 0, 0, 0, loc), 3},
		{time.Date(2026, 4, 2, 0, 0, 0, 0, loc), 3},
		{time.Date(2026, 4, 3, 0, 0, 0, 0, loc), 3},
		{time.Date(2026, 4, 4, 0, 0, 0, 0, loc), 3},
		{time.Date(2026, 4, 5, 0, 0, 0, 0, loc), 3},
	}

	totalGenerated := 0
	for _, dt := range days {
		dd := fmt.Sprintf("%02d", dt.date.Day())
		summaryDir := filepath.Join(*outputDir, "summary", "2026", "04", dd)
		convDir := filepath.Join(*outputDir, "conversation", "2026", "04", dd)

		if err := os.MkdirAll(summaryDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "创建目录失败 %s: %v\n", summaryDir, err)
			os.Exit(1)
		}
		if err := os.MkdirAll(convDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "创建目录失败 %s: %v\n", convDir, err)
			os.Exit(1)
		}

		for i := 0; i < dt.count; i++ {
			taskID := genUUID(rng)
			user := users[rng.Intn(len(users))]
			repo := repos[rng.Intn(len(repos))]
			client := clients[rng.Intn(len(clients))]
			caller := callers[rng.Intn(len(callers))]

			summary := TaskSummary{
				TaskID:        taskID,
				UserID:        user.id,
				UserName:      user.name,
				ClientID:      client.id,
				ClientIDE:     client.ide,
				ClientVersion: client.version,
				ClientOS:      client.os,
				ClientOSVer:   client.osVersion,
				Caller:        caller,
				RepoAddr:      repo.addr,
				RepoBranch:    repo.branch,
				WorkDir:       client.workDir,
				Diff:          diffSnippets[rng.Intn(len(diffSnippets))],
				DiffLines:     50 + rng.Intn(1951), // 50~2000
			}

			summaryData, err := json.MarshalIndent(summary, "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "序列化 task summary 失败: %v\n", err)
				continue
			}
			summaryPath := filepath.Join(summaryDir, taskID+".json")
			if err := os.WriteFile(summaryPath, summaryData, 0644); err != nil {
				fmt.Fprintf(os.Stderr, "写入文件失败 %s: %v\n", summaryPath, err)
				continue
			}

			// Generate 3~8 conversations
			convCount := 3 + rng.Intn(6)
			// Start time: random hour between 9:00 and 17:00
			startHour := 9 + rng.Intn(8)
			startMinute := rng.Intn(60)
			currentTime := time.Date(dt.date.Year(), dt.date.Month(), dt.date.Day(),
				startHour, startMinute, rng.Intn(60), 0, loc)

			convPath := filepath.Join(convDir, taskID+".jsonl")
			convFile, err := os.Create(convPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "创建文件失败 %s: %v\n", convPath, err)
				continue
			}

			for j := 0; j < convCount; j++ {
				// Alternate sender: user then agent
				sender := "user"
				if j%2 == 1 {
					sender = "agent"
				}

				// Duration: 30s ~ 5min
				durationSec := 30 + rng.Intn(270)
				startT := currentTime
				endT := startT.Add(time.Duration(durationSec) * time.Second)
				processTimeMs := int64(durationSec) * 1000
				processTTFT := int64(100 + rng.Intn(1901)) // 100~2000ms

				upTokens := int64(500 + rng.Intn(4501))    // 500~5000
				downTokens := int64(200 + rng.Intn(2801))   // 200~3000
				cost := float64(upTokens+downTokens) * 0.000002

				reqContent := requestContents[rng.Intn(len(requestContents))]
				respContent := responseContents[rng.Intn(len(responseContents))]

				var userInput string
				var convDiff string
				var convDiffLines int
				if sender == "user" {
					userInput = reqContent
					convDiff = ""
					convDiffLines = 0
				} else {
					userInput = ""
					convDiff = diffSnippets[rng.Intn(len(diffSnippets))]
					convDiffLines = rng.Intn(101) // 0~100
				}

				entry := ConversationEntry{
					Sender:           sender,
					RequestID:        genUUID(rng),
					PromptMode:       promptModes[rng.Intn(len(promptModes))],
					Mode:             modes[rng.Intn(len(modes))],
					Model:            models[rng.Intn(len(models))],
					StartTime:        startT.Format(time.RFC3339),
					EndTime:          endT.Format(time.RFC3339),
					ProcessTime:      processTimeMs,
					ProcessTTFT:      processTTFT,
					UpstreamTokens:   upTokens,
					DownstreamTokens: downTokens,
					Cost:             cost,
					RequestContent:   reqContent,
					ResponseContent:  respContent,
					UserInput:        userInput,
					Diff:             convDiff,
					DiffLines:        convDiffLines,
				}

				// ~15% chance of error
				if rng.Float64() < 0.15 {
					idx := rng.Intn(len(errorCodes))
					code := errorCodes[idx]
					reason := errorReasons[idx]
					entry.ErrorCode = &code
					entry.ErrorReason = &reason
				}

				lineData, _ := json.Marshal(entry)
				convFile.Write(lineData)
				convFile.Write([]byte("\n"))

				// Gap between conversations: 2~30 minutes
				gapMinutes := 2 + rng.Intn(29)
				currentTime = endT.Add(time.Duration(gapMinutes) * time.Minute)
			}

			convFile.Close()
			totalGenerated++
			fmt.Printf("生成 task %d/%d: %s (日期: %s, %d 条对话)\n",
				totalGenerated, 15, taskID, dt.date.Format("2006-01-02"), convCount)
		}
	}

	fmt.Printf("\n完成！共生成 %d 个 task，输出目录: %s\n", totalGenerated, *outputDir)
}
