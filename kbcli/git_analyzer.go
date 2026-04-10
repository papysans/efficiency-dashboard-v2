package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// GitAnalyzer Git 分析器
type GitAnalyzer struct {
	RepoURL   string
	LocalPath string
	CacheDir  string
	HTTPProxy string // HTTP 代理地址，如 http://127.0.0.1:7890
}

// CommitDetail 单个 commit 的详细信息
type CommitDetail struct {
	Hash         string    `json:"hash"`
	AuthorName   string    `json:"author_name"`
	AuthorEmail  string    `json:"author_email"`
	Timestamp    time.Time `json:"timestamp"`
	Message      string    `json:"message"`
	FilesChanged []string  `json:"files_changed"`
	LinesAdded   int64     `json:"lines_added"`
	LinesDeleted int64     `json:"lines_deleted"`
}

// GitAnalysisResult Git 分析结果
type GitAnalysisResult struct {
	CommitCount      int            `json:"commit_count"`
	ContributorCount int            `json:"contributor_count"`
	LinesAdded       int64          `json:"lines_added"`
	LinesDeleted     int64          `json:"lines_deleted"`
	FilesChanged     int            `json:"files_changed"`
	CommitMessages   []string       `json:"commit_messages"`
	Commits          []CommitDetail `json:"commits"`
}

// NewGitAnalyzer 创建 Git 分析器
func NewGitAnalyzer(repoURL string, cacheDir string, httpProxy string) *GitAnalyzer {
	hash := sha256.Sum256([]byte(repoURL))
	hashStr := fmt.Sprintf("%x", hash)[:12]
	localPath := cacheDir + "/" + hashStr

	return &GitAnalyzer{
		RepoURL:   repoURL,
		LocalPath: localPath,
		CacheDir:  cacheDir,
		HTTPProxy: httpProxy,
	}
}

// gitCmd 创建带代理环境变量的 git 命令
func (g *GitAnalyzer) gitCmd(args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	if g.HTTPProxy != "" {
		cmd.Env = append(os.Environ(),
			"http_proxy="+g.HTTPProxy,
			"https_proxy="+g.HTTPProxy,
		)
	}
	return cmd
}

// EnsureRepo 确保本地仓库存在且最新（克隆或拉取）
// 使用完整 clone（非 partial clone），确保 git log --stat 等操作全在本地完成，无需网络
func (g *GitAnalyzer) EnsureRepo() error {
	if _, err := os.Stat(g.LocalPath); os.IsNotExist(err) {
		fmt.Printf("[Git] 克隆仓库 %s 到 %s\n", g.RepoURL, g.LocalPath)
		cmd := g.gitCmd("clone", g.RepoURL, g.LocalPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("克隆仓库失败: %w", err)
		}
	} else {
		fmt.Printf("[Git] 更新仓库 %s\n", g.LocalPath)
		cmd := g.gitCmd("-C", g.LocalPath, "pull")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			// pull 失败不致命，可能是离线环境，仅警告
			fmt.Printf("[Git] 警告: pull 失败（%v），使用已有本地数据\n", err)
		}
	}
	return nil
}

// formatDate 将 "20260301" 格式转换为 "2026-03-01"
func formatDate(date string) (string, error) {
	if len(date) != 8 {
		return "", fmt.Errorf("日期格式错误，期望8位数字如20260301，实际: %s", date)
	}
	return date[:4] + "-" + date[4:6] + "-" + date[6:8], nil
}

// AnalyzeCommits 分析指定时间范围内的 Git 提交
// 使用两阶段策略：先用 --shortstat 快速获取总览，详细文件列表延迟到关联阶段按需获取
func (g *GitAnalyzer) AnalyzeCommits(startDate, endDate string) (*GitAnalysisResult, error) {
	start, err := formatDate(startDate)
	if err != nil {
		return nil, fmt.Errorf("解析开始日期失败: %w", err)
	}
	end, err := formatDate(endDate)
	if err != nil {
		return nil, fmt.Errorf("解析结束日期失败: %w", err)
	}

	// 用 --shortstat 一次拿到所有 commit 元信息 + 汇总行数（速度快，不逐文件展开）
	logCmd := g.gitCmd("-C", g.LocalPath, "log",
		"--since="+start, "--until="+end,
		"--pretty=format:COMMIT_SEP|%H|%an|%ae|%at|%s",
		"--shortstat")
	logOutput, err := logCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("执行 git log 失败: %w", err)
	}

	result := &GitAnalysisResult{}
	logStr := strings.TrimSpace(string(logOutput))
	if logStr == "" {
		return result, nil
	}

	// 按 COMMIT_SEP 分割解析
	contributors := make(map[string]bool)
	shortStatRe := regexp.MustCompile(`(\d+) files? changed(?:, (\d+) insertions?\(\+\))?(?:, (\d+) deletions?\(-\))?`)
	sections := strings.Split(logStr, "COMMIT_SEP|")

	for _, section := range sections {
		section = strings.TrimSpace(section)
		if section == "" {
			continue
		}

		lines := strings.Split(section, "\n")
		if len(lines) == 0 {
			continue
		}

		// 第一行是 commit 元信息: hash|author|email|timestamp|message
		parts := strings.SplitN(lines[0], "|", 5)
		if len(parts) < 5 {
			continue
		}
		hash := parts[0]
		authorName := parts[1]
		email := parts[2]
		tsStr := parts[3]
		message := parts[4]

		contributors[email] = true
		result.CommitMessages = append(result.CommitMessages, message)

		tsInt, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil {
			fmt.Printf("警告: 解析 commit %s 的时间戳失败: %v\n", hash[:8], err)
			continue
		}

		detail := CommitDetail{
			Hash:        hash,
			AuthorName:  authorName,
			AuthorEmail: email,
			Timestamp:   time.Unix(tsInt, 0),
			Message:     message,
		}

		// 解析 shortstat 行（如 "4 files changed, 7 insertions(+), 8 deletions(-)"）
		for _, sl := range lines[1:] {
			sl = strings.TrimSpace(sl)
			if sl == "" {
				continue
			}
			matches := shortStatRe.FindStringSubmatch(sl)
			if len(matches) > 0 {
				if matches[2] != "" {
					if added, e := strconv.ParseInt(matches[2], 10, 64); e == nil {
						detail.LinesAdded = added
					}
				}
				if matches[3] != "" {
					if deleted, e := strconv.ParseInt(matches[3], 10, 64); e == nil {
						detail.LinesDeleted = deleted
					}
				}
			}
		}

		result.LinesAdded += detail.LinesAdded
		result.LinesDeleted += detail.LinesDeleted
		result.Commits = append(result.Commits, detail)
	}

	result.CommitCount = len(result.Commits)
	result.ContributorCount = len(contributors)

	// FilesChanged 用 --name-only 的去重文件数来统计
	nameCmd := g.gitCmd("-C", g.LocalPath, "log",
		"--since="+start, "--until="+end,
		"--name-only", "--pretty=format:")
	nameOutput, err := nameCmd.Output()
	if err == nil {
		allFiles := make(map[string]bool)
		for _, f := range strings.Split(string(nameOutput), "\n") {
			f = strings.TrimSpace(f)
			if f != "" {
				allFiles[f] = true
			}
		}
		result.FilesChanged = len(allFiles)
	}

	return result, nil
}

// FetchCommitFiles 按需获取单个 commit 的变更文件列表（延迟加载，用于关联匹配）
func (g *GitAnalyzer) FetchCommitFiles(commitHash string) ([]string, error) {
	cmd := g.gitCmd("-C", g.LocalPath, "show", "--name-only", "--pretty=format:", commitHash)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("获取 commit %s 变更文件失败: %w", commitHash[:8], err)
	}
	var files []string
	for _, f := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		f = strings.TrimSpace(f)
		if f != "" {
			files = append(files, f)
		}
	}
	return files, nil
}

const gitEstimationPrompt = `你是一个经验丰富的软件项目经理，擅长评估软件开发工作量。

请根据以下 Git 仓库的代码变更统计，结合 Task 数据，估算该仓库在指定时间段内的总工作量（分钟数）。

Git 统计数据：
- 总代码行数变更：{{total_code_lines}} 行（新增 {{lines_added}}，删除 {{lines_deleted}}）
- Commit 数量：{{commit_count}}
- 贡献者数量：{{contributor_count}}
- 时间范围：{{start_time}} 至 {{end_time}}

Commit 消息摘要：
{{commit_messages}}

请输出 JSON 格式：
{
  "commit_ancient_minutes": 270,
  "commit_ancient_minutes_reason": "估算理由..."
}`

// EstimateFromGit 基于 Git 分析结果进行 AI 二次预估
func EstimateFromGit(config AIEstimationConfig, gitResult *GitAnalysisResult, taskSummary map[string]interface{}) (float64, string, error) {
	totalCodeLines := gitResult.LinesAdded + gitResult.LinesDeleted

	// 收集 commit messages（最多50条）
	messages := gitResult.CommitMessages
	if len(messages) > 50 {
		messages = messages[:50]
	}
	commitMessagesStr := strings.Join(messages, "\n")

	startTime := fmt.Sprintf("%v", taskSummary["start_time"])
	endTime := fmt.Sprintf("%v", taskSummary["end_time"])

	// 变量替换
	prompt := gitEstimationPrompt
	prompt = strings.ReplaceAll(prompt, "{{total_code_lines}}", fmt.Sprintf("%d", totalCodeLines))
	prompt = strings.ReplaceAll(prompt, "{{lines_added}}", fmt.Sprintf("%d", gitResult.LinesAdded))
	prompt = strings.ReplaceAll(prompt, "{{lines_deleted}}", fmt.Sprintf("%d", gitResult.LinesDeleted))
	prompt = strings.ReplaceAll(prompt, "{{commit_count}}", fmt.Sprintf("%d", gitResult.CommitCount))
	prompt = strings.ReplaceAll(prompt, "{{contributor_count}}", fmt.Sprintf("%d", gitResult.ContributorCount))
	prompt = strings.ReplaceAll(prompt, "{{start_time}}", startTime)
	prompt = strings.ReplaceAll(prompt, "{{end_time}}", endTime)
	prompt = strings.ReplaceAll(prompt, "{{commit_messages}}", commitMessagesStr)

	// 构建 HTTP 请求（Anthropic Messages API）
	reqBody := map[string]interface{}{
		"model":      config.Model,
		"max_tokens": 1024,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return 0, "", fmt.Errorf("序列化请求体失败: %w", err)
	}

	url := config.BaseURL + "/v1/messages"
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return 0, "", fmt.Errorf("创建HTTP请求失败: %w", err)
	}
	httpReq.Header.Set("x-api-key", config.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: time.Duration(config.TimeoutMS) * time.Millisecond,
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return 0, "", fmt.Errorf("AI API 请求失败（可能超时）: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, "", fmt.Errorf("读取AI响应失败: %w", err)
	}

	if resp.StatusCode != 200 {
		return 0, "", fmt.Errorf("AI API 返回非200状态码: %d, 响应: %s", resp.StatusCode, string(respBody))
	}

	// 解析 Anthropic 响应
	var anthropicResp struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &anthropicResp); err != nil {
		return 0, "", fmt.Errorf("解析AI响应JSON失败: %w, 原始响应: %s", err, string(respBody))
	}
	if len(anthropicResp.Content) == 0 {
		return 0, "", fmt.Errorf("AI响应content为空, 原始响应: %s", string(respBody))
	}

	text := anthropicResp.Content[0].Text

	// 去除 markdown 代码块标记
	re := regexp.MustCompile("(?s)```(?:json)?\\s*\\n?(.*?)\\n?```")
	if matches := re.FindStringSubmatch(text); len(matches) > 1 {
		text = matches[1]
	}
	text = strings.TrimSpace(text)

	// 解析估时结果
	var aiResult struct {
		CommitAncientMinutes       float64 `json:"commit_ancient_minutes"`
		CommitAncientMinutesReason string  `json:"commit_ancient_minutes_reason"`
	}
	if err := json.Unmarshal([]byte(text), &aiResult); err != nil {
		return 0, "", fmt.Errorf("解析AI估时结果JSON失败: %w, 原始文本: %s", err, text)
	}

	if aiResult.CommitAncientMinutes < 0 || aiResult.CommitAncientMinutes > 100000 {
		return 0, "", fmt.Errorf("AI估时结果异常: %.2f（应在0-100000之间）", aiResult.CommitAncientMinutes)
	}

	return aiResult.CommitAncientMinutes, aiResult.CommitAncientMinutesReason, nil
}
