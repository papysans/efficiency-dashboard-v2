package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"
)

// RawDoc raw 层文档结构（对应 ES 索引字段）
type RawDoc struct {
	Timestamp             time.Time `json:"@timestamp"`
	Caller                string    `json:"caller"`
	Sender                string    `json:"sender"`
	TaskID                string    `json:"task_id"`
	RequestID             string    `json:"request_id"`
	ClientID              string    `json:"client_id"`
	UserID                string    `json:"user_id"`
	UserName              string    `json:"user_name"`
	RepoID                string    `json:"repo_id"`
	ProjectPath           string    `json:"project_path"`
	ProjectID             string    `json:"project_id"`
	ClientIDE             string    `json:"client_ide"`
	ClientVersion         string    `json:"client_version"`
	ClientOS              string    `json:"client_os"`
	PromptMode            string    `json:"prompt_mode"`
	Mode                  string    `json:"mode"`
	Model                 string    `json:"model"`
	UserInChars           int64     `json:"user_in_chars"`
	AssistantOutCodeLines int64     `json:"assistant_out_code_lines"`
	SystemTokens          int64     `json:"system_tokens"`
	UserTokens            int64     `json:"user_tokens"`
	APIRequestTime        time.Time `json:"api_request_time"`
	APIEndTime            time.Time `json:"api_end_time"`
	APIProcessTime        int64     `json:"api_process_time"`
	APITtft               int64     `json:"api_ttft"`
	APIInTokens           int64     `json:"api_in_tokens"`
	APIOutTokens          int64     `json:"api_out_tokens"`
	APICost               float64   `json:"api_cost"`
	Org1                  string    `json:"org1"`
	Org2                  string    `json:"org2"`
	Org3                  string    `json:"org3"`
	Org4                  string    `json:"org4"`
	SourcePath            string    `json:"source_path"`
}

// rawJSON 原始 JSON 结构（用于解析）
type rawJSON struct {
	Identity struct {
		TaskID        string `json:"task_id"`
		RequestID     string `json:"request_id"`
		ClientID      string `json:"client_id"`
		ClientIDE     string `json:"client_ide"`
		ClientVersion string `json:"client_version"`
		ClientOS      string `json:"client_os"`
		UserName      string `json:"user_name"`
		ProjectPath   string `json:"project_path"`
		Caller        string `json:"caller"`
		Sender        string `json:"sender"`
		UserInfo      struct {
			UUID       string `json:"uuid"`
			Phone      string `json:"phone"`
			GithubName string `json:"github_name"`
			Name       string `json:"name"`
		} `json:"user_info"`
	} `json:"identity"`
	Timestamp string `json:"timestamp"`
	Tokens    struct {
		Original struct {
			SystemTokens int64 `json:"system_tokens"`
			UserTokens   int64 `json:"user_tokens"`
		} `json:"original"`
	} `json:"tokens"`
	Latency struct {
		TotalLatencyMS      int64 `json:"total_latency_ms"`
		FirstTokenLatencyMS int64 `json:"first_token_latency_ms"`
	} `json:"latency"`
	Params struct {
		Model     string `json:"model"`
		LLMParams struct {
			ExtraBody struct {
				Mode       string `json:"mode"`
				PromptMode string `json:"prompt_mode"`
			} `json:"extra_body"`
			Messages []struct {
				Role    string          `json:"role"`
				Content json.RawMessage `json:"content"`
			} `json:"messages"`
		} `json:"llm_params"`
	} `json:"params"`
	ResponseContent struct {
		ToolCalls []struct {
			Function struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			} `json:"function"`
		} `json:"tool_calls"`
	} `json:"response_content"`
	Usage struct {
		PromptTokens     int64 `json:"prompt_tokens"`
		CompletionTokens int64 `json:"completion_tokens"`
	} `json:"usage"`
}

// ParseRawJSON 解析 rawdata JSON 文件，返回 RawDoc
func ParseRawJSON(jsonBytes []byte, modelPrices map[string]ModelPrice, orgProvider *OrgProvider) (*RawDoc, error) {
	var raw rawJSON
	if err := json.Unmarshal(jsonBytes, &raw); err != nil {
		return nil, fmt.Errorf("JSON解析失败: %w", err)
	}

	// 解析时间
	reqTime, err := time.Parse(time.RFC3339Nano, raw.Timestamp)
	if err != nil {
		reqTime, err = time.Parse(time.RFC3339, raw.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("解析timestamp失败: %w", err)
		}
	}
	reqTime = reqTime.UTC()

	// 计算 api_end_time
	endTime := reqTime.Add(time.Duration(raw.Latency.TotalLatencyMS) * time.Millisecond)

	// username fallback
	username := raw.Identity.UserInfo.Name
	if username == "" {
		username = raw.Identity.UserInfo.Phone
	}
	if username == "" {
		username = raw.Identity.UserInfo.GithubName
	}
	if username == "" {
		username = raw.Identity.UserName
	}

	// repo_id: rawdata JSON 中不含 repo 信息，暂为空
	repo := ""

	// project_id: 取 clientID 前10字符 + projectPath
	clientIDRunes := []rune(raw.Identity.ClientID)
	prefix := string(clientIDRunes[:min(10, len(clientIDRunes))])
	projectID := toPathSafeID(prefix + ":" + raw.Identity.ProjectPath)

	// user_in_chars
	userInChars := calcUserInChars(raw.Identity.Sender, raw.Params.LLMParams.Messages)

	// assistant_out_code_lines
	outCodeLines := calcOutCodeLines(raw.ResponseContent.ToolCalls)

	// api_cost
	cost := calculateCost(raw.Params.Model, raw.Usage.PromptTokens, raw.Usage.CompletionTokens, modelPrices)

	// org info
	var orgInfo OrgInfo
	if orgProvider != nil {
		orgInfo = orgProvider.GetOrgInfo(raw.Identity.UserInfo.UUID, username)
	}

	doc := &RawDoc{
		Timestamp:             reqTime,
		Caller:                raw.Identity.Caller,
		Sender:                raw.Identity.Sender,
		TaskID:                raw.Identity.TaskID,
		RequestID:             raw.Identity.RequestID,
		ClientID:              raw.Identity.ClientID,
		UserID:                raw.Identity.UserInfo.UUID,
		UserName:              username,
		RepoID:                repo,
		ProjectPath:           raw.Identity.ProjectPath,
		ProjectID:             projectID,
		ClientIDE:             raw.Identity.ClientIDE,
		ClientVersion:         raw.Identity.ClientVersion,
		ClientOS:              raw.Identity.ClientOS,
		PromptMode:            raw.Params.LLMParams.ExtraBody.PromptMode,
		Mode:                  raw.Params.LLMParams.ExtraBody.Mode,
		Model:                 raw.Params.Model,
		UserInChars:           userInChars,
		AssistantOutCodeLines: outCodeLines,
		SystemTokens:          raw.Tokens.Original.SystemTokens,
		UserTokens:            raw.Tokens.Original.UserTokens,
		APIRequestTime:        reqTime,
		APIEndTime:            endTime,
		APIProcessTime:        raw.Latency.TotalLatencyMS,
		APITtft:               raw.Latency.FirstTokenLatencyMS,
		APIInTokens:           raw.Usage.PromptTokens,
		APIOutTokens:          raw.Usage.CompletionTokens,
		APICost:               cost,
		Org1:                  orgInfo.Org1,
		Org2:                  orgInfo.Org2,
		Org3:                  orgInfo.Org3,
		Org4:                  orgInfo.Org4,
	}
	return doc, nil
}

// contentToString 将 content（string 或 []object）转为字符串
func contentToString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// 尝试 string
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	// 尝试 []{"type":"text","text":"..."}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err == nil {
		var sb strings.Builder
		for _, p := range parts {
			if p.Type == "text" {
				sb.WriteString(p.Text)
			}
		}
		return sb.String()
	}
	return ""
}

// calcUserInChars 计算用户输入字符数
func calcUserInChars(sender string, messages []struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}) int64 {
	if sender != "user" {
		return 0
	}
	if len(messages) == 0 {
		return 0
	}
	lastMsg := messages[len(messages)-1]
	content := contentToString(lastMsg.Content)
	if !strings.HasPrefix(content, "<user_message>") {
		return 0
	}
	// 提取 <user_message>...</user_message> 内的文本
	start := len("<user_message>")
	end := strings.Index(content, "</user_message>")
	if end < 0 {
		end = len(content)
	}
	text := content[start:end]
	return countChars(text)
}

// countChars 计算字符数（中文=2，英文=1）
func countChars(s string) int64 {
	var count int64
	for _, r := range s {
		if isCJK(r) {
			count += 2
		} else if r > 0x20 && r < 0x7F {
			count++
		}
	}
	return count
}

// isCJK 判断是否为CJK字符
func isCJK(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		(r >= 0x3040 && r <= 0x309F) || // Hiragana
		(r >= 0x30A0 && r <= 0x30FF) || // Katakana
		(r >= 0xAC00 && r <= 0xD7AF) // Korean
}

// countDiffReplaceLines 统计 diff 中所有 REPLACE 部分的行数
func countDiffReplaceLines(diff string) int64 {
	var total int64
	lines := strings.Split(diff, "\n")
	inReplace := false
	for _, line := range lines {
		if line == "=======" {
			inReplace = true
			continue
		}
		if line == ">>>>>>> REPLACE" {
			inReplace = false
			continue
		}
		if inReplace {
			total++
		}
	}
	return total
}

// extractDiffReplaceContent 提取 diff 中所有 REPLACE 部分的内容
func extractDiffReplaceContent(diff string) string {
	lines := strings.Split(diff, "\n")
	var parts []string
	inReplace := false
	for _, line := range lines {
		if line == "=======" {
			inReplace = true
			continue
		}
		if line == ">>>>>>> REPLACE" {
			inReplace = false
			continue
		}
		if inReplace {
			parts = append(parts, line)
		}
	}
	return strings.Join(parts, "\n")
}

// calcOutCodeLines 计算助手输出代码行数
func calcOutCodeLines(toolCalls []struct {
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}) int64 {
	var total int64
	for _, tc := range toolCalls {
		name := tc.Function.Name
		if name != "write_to_file" && name != "apply_diff" {
			continue
		}
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			continue
		}
		if name == "apply_diff" {
			diff, _ := args["diff"].(string)
			total += countDiffReplaceLines(diff)
		} else {
			content, ok := args["content"].(string)
			if !ok || content == "" {
				continue
			}
			lines := strings.Split(content, "\n")
			count := int64(len(lines))
			if count > 0 && lines[count-1] == "" {
				count--
			}
			total += count
		}
	}
	return total
}

// calculateCost 计算 API 调用费用
func calculateCost(model string, inTokens, outTokens int64, prices map[string]ModelPrice) float64 {
	price, ok := prices[model]
	if !ok {
		return 0
	}
	return (float64(inTokens)/1e6)*price.InPrice + (float64(outTokens)/1e6)*price.OutPrice
}
