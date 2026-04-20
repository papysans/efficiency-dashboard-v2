package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// PGTaskData PG 任务数据（JSON tag 对齐后端 CostrictTask 字段名）
type PGTaskData struct {
	TaskID                   string     `json:"TaskID"`
	UserID                   *string    `json:"UserID"`
	UserName                 *string    `json:"UserName"`
	ClientID                 *string    `json:"ClientID"`
	IDE                      *string    `json:"IDE"`
	Version                  *string    `json:"Version"`
	OS                       *string    `json:"OS"`
	OSVersion                *string    `json:"OSVersion"`
	Caller                   *string    `json:"Caller"`
	RepoAddr                 *string    `json:"RepoAddr"`
	RepoBranch               *string    `json:"RepoBranch"`
	RepoID                   *string    `json:"RepoID"`
	WorkDir                  *string    `json:"WorkDir"`
	ProjectID                *string    `json:"ProjectID"`
	StartTime                *time.Time `json:"StartTime"`
	EndTime                  *time.Time `json:"EndTime"`
	UpstreamTokens           *int64     `json:"UpstreamTokens"`
	DownstreamTokens         *int64     `json:"DownstreamTokens"`
	Cost                     *float64   `json:"Cost"`
	DiffLines                *int64     `json:"DiffLines"`
	TaskAncientMinutes       *float64   `json:"TaskAncientMinutes"`
	TaskAncientMinutesReason *string    `json:"TaskAncientMinutesReason"`
}

// PGConversationData PG 对话数据（JSON tag 对齐后端 CostrictTaskConversation 字段名）
type PGConversationData struct {
	TaskID           string     `json:"TaskID"`
	RequestID        string     `json:"RequestID"`
	Sender           *string    `json:"Sender"`
	PromptMode       *string    `json:"PromptMode"`
	Mode             *string    `json:"Mode"`
	Model            *string    `json:"Model"`
	StartTime        *time.Time `json:"StartTime"`
	EndTime          *time.Time `json:"EndTime"`
	ProcessTime      *int64     `json:"ProcessTime"`
	ProcessTTFT      *int64     `json:"ProcessTTFT"`
	UpstreamTokens   *int64     `json:"UpstreamTokens"`
	DownstreamTokens *int64     `json:"DownstreamTokens"`
	Cost             *float64   `json:"Cost"`
	RequestContent   *string    `json:"RequestContent"`
	ResponseContent  *string    `json:"ResponseContent"`
	UserInput        *string    `json:"UserInput"`
	Diff             *string    `json:"Diff"`
	DiffLines        *int64     `json:"DiffLines"`
	ErrorCode        *string    `json:"ErrorCode"`
	ErrorReason      *string    `json:"ErrorReason"`
}

func ptrString(s string) *string     { return &s }
func ptrInt64(i int64) *int64        { return &i }
func ptrFloat64(f float64) *float64  { return &f }
func ptrTime(t time.Time) *time.Time { return &t }

// MapTaskDocToPG 将 TaskDoc 映射为 PGTaskData
func MapTaskDocToPG(taskDoc TaskDoc, rawDocs []RawDoc) *PGTaskData {
	pg := &PGTaskData{
		TaskID:                   taskDoc.TaskID,
		UserID:                   ptrString(taskDoc.UserID),
		UserName:                 ptrString(taskDoc.UserName),
		ClientID:                 ptrString(taskDoc.ClientID),
		Caller:                   ptrString(taskDoc.Caller),
		WorkDir:                  ptrString(taskDoc.ProjectPath),
		ProjectID:                ptrString(taskDoc.ProjectID),
		IDE:                      ptrString(taskDoc.ClientIDE),
		Version:                  ptrString(taskDoc.ClientVersion),
		OS:                       ptrString(taskDoc.ClientOS),
		OSVersion:                ptrString(""),
		RepoID:                   ptrString(taskDoc.RepoID),
		StartTime:                ptrTime(taskDoc.APIRequestTime),
		EndTime:                  ptrTime(taskDoc.APIEndTime),
		UpstreamTokens:           ptrInt64(taskDoc.APIInTokens),
		DownstreamTokens:         ptrInt64(taskDoc.APIOutTokens),
		Cost:                     ptrFloat64(taskDoc.APICost),
		DiffLines:                ptrInt64(taskDoc.AssistantOutCodeLines),
		TaskAncientMinutes:       ptrFloat64(taskDoc.TaskAncientMinutes),
		TaskAncientMinutesReason: ptrString(taskDoc.TaskAncientMinutesReason),
	}

	// repo_addr / repo_branch: 按 "#" 分割 RepoID
	if strings.Contains(taskDoc.RepoID, "#") {
		parts := strings.SplitN(taskDoc.RepoID, "#", 2)
		pg.RepoAddr = ptrString(parts[0])
		pg.RepoBranch = ptrString(parts[1])
	} else {
		pg.RepoAddr = ptrString(taskDoc.RepoID)
		pg.RepoBranch = ptrString("")
	}

	return pg
}

// MapRawDocsToConversations 将 RawDoc 列表映射为 PGConversationData 列表
func MapRawDocsToConversations(taskID string, rawDocs []RawDoc, rawDataDir string) []PGConversationData {
	var convs []PGConversationData
	for _, doc := range rawDocs {
		conv := PGConversationData{
			TaskID:           taskID,
			RequestID:        doc.RequestID,
			Sender:           ptrString(doc.Sender),
			PromptMode:       ptrString(doc.PromptMode),
			Mode:             ptrString(doc.Mode),
			Model:            ptrString(doc.Model),
			StartTime:        ptrTime(doc.APIRequestTime),
			EndTime:          ptrTime(doc.APIEndTime),
			ProcessTime:      ptrInt64(doc.APIProcessTime),
			ProcessTTFT:      ptrInt64(doc.APITtft),
			UpstreamTokens:   ptrInt64(doc.APIInTokens),
			DownstreamTokens: ptrInt64(doc.APIOutTokens),
			Cost:             ptrFloat64(doc.APICost),
			RequestContent:   nil,
			ResponseContent:  nil,
			ErrorCode:        nil,
			ErrorReason:      nil,
		}

		// 提取 user_input 和 diff
		if doc.SourcePath != "" {
			fullPath := filepath.Join(rawDataDir, doc.SourcePath)
			data, err := os.ReadFile(fullPath)
			if err == nil {
				var raw rawJSON
				if err := json.Unmarshal(data, &raw); err == nil {
					// 提取 user_input
					if msgs := raw.Params.LLMParams.Messages; len(msgs) > 0 {
						content := contentToString(msgs[len(msgs)-1].Content)
						if strings.HasPrefix(content, "<user_message>") {
							start := len("<user_message>")
							end := strings.Index(content, "</user_message>")
							if end < 0 {
								end = len(content)
							}
							conv.UserInput = ptrString(content[start:end])
						} else {
							conv.UserInput = ptrString(content)
						}
					}

					// 提取 diff
					var diffParts []string
					var diffLines int64
					for _, tc := range raw.ResponseContent.ToolCalls {
						if tc.Function.Name != "write_to_file" && tc.Function.Name != "apply_diff" {
							continue
						}
						var args map[string]interface{}
						if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
							continue
						}
						if tc.Function.Name == "apply_diff" {
							diff, _ := args["diff"].(string)
							code := extractDiffReplaceContent(diff)
							if code != "" {
								diffParts = append(diffParts, code)
							}
							diffLines += countDiffReplaceLines(diff)
						} else {
							content, _ := args["content"].(string)
							if content != "" {
								diffParts = append(diffParts, content)
							}
						}
					}
					if len(diffParts) > 0 {
						conv.Diff = ptrString(strings.Join(diffParts, "\n"))
					}
					conv.DiffLines = ptrInt64(diffLines)
				}
			}
		}

		// 如果未从原始文件提取到 diff_lines，使用 RawDoc 的值
		if conv.DiffLines == nil || *conv.DiffLines == 0 {
			conv.DiffLines = ptrInt64(doc.AssistantOutCodeLines)
		}

		convs = append(convs, conv)
	}
	return convs
}

// SaveTaskToPG 通过 POST /api/v2/tasks 保存任务到 PG
func (c *BackendClient) SaveTaskToPG(task *PGTaskData) error {
	jsonData, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("序列化请求体失败: %w", err)
	}

	resp, err := c.HTTPClient.Post(c.BaseURL+"/api/v2/tasks", "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("调用 backend API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("backend API 返回错误: %d, %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// SaveConversationsToPG 通过 POST /api/v2/tasks/conversations/batch 批量保存对话到 PG
func (c *BackendClient) SaveConversationsToPG(convs []PGConversationData) error {
	if len(convs) == 0 {
		return nil
	}

	jsonData, err := json.Marshal(convs)
	if err != nil {
		return fmt.Errorf("序列化请求体失败: %w", err)
	}

	resp, err := c.HTTPClient.Post(c.BaseURL+"/api/v2/tasks/conversations/batch", "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("调用 backend API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("backend API 返回错误: %d, %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// PGCommitData PG 提交数据（JSON tag 对齐后端字段名）
type PGCommitData struct {
	CommitID                   string     `json:"CommitID"`
	CommitTime                 *time.Time `json:"CommitTime"`
	RepoAddr                   *string    `json:"RepoAddr"`
	RepoBranch                 *string    `json:"RepoBranch"`
	GitUserName                *string    `json:"GitUserName"`
	GitUserEmail               *string    `json:"GitUserEmail"`
	UserID                     *string    `json:"UserID"`
	UserName                   *string    `json:"UserName"`
	ClientID                   *string    `json:"ClientID"`
	WorkDir                    *string    `json:"WorkDir"`
	DiffLines                  *int64     `json:"DiffLines"`
	CommitAncientMinutes       *float64   `json:"CommitAncientMinutes"`
	CommitAncientMinutesReason *string    `json:"CommitAncientMinutesReason"`
}

// MapCommitDetailsToPG 将 CommitDetail 列表映射为 PGCommitData 列表
func MapCommitDetailsToPG(commits []CommitDetail, repoID string, orgProvider *OrgProvider) []PGCommitData {
	var result []PGCommitData
	for _, commit := range commits {
		pg := PGCommitData{
			CommitID:     commit.Hash,
			CommitTime:   ptrTime(commit.Timestamp),
			GitUserName:  ptrString(commit.AuthorName),
			GitUserEmail: ptrString(commit.AuthorEmail),
			DiffLines:    ptrInt64(commit.LinesAdded),
		}

		// repo_addr / repo_branch: 按 "#" 分割 repoID
		if strings.Contains(repoID, "#") {
			parts := strings.SplitN(repoID, "#", 2)
			pg.RepoAddr = ptrString(parts[0])
			pg.RepoBranch = ptrString(parts[1])
		} else {
			pg.RepoAddr = ptrString(repoID)
			pg.RepoBranch = ptrString("")
		}

		// user_id 映射
		if orgProvider != nil {
			userID, found := orgProvider.LookupByGitAuthor(commit.AuthorName, commit.AuthorEmail)
			if found {
				pg.UserID = ptrString(userID)
				orgInfo := orgProvider.GetOrgInfo(userID, "")
				pg.UserName = ptrString(orgInfo.UserName)
			}
		}

		result = append(result, pg)
	}
	return result
}

// SaveCommitsToPG 通过 POST /api/v2/commits/batch 批量保存提交到 PG
func (c *BackendClient) SaveCommitsToPG(commits []PGCommitData) error {
	if len(commits) == 0 {
		return nil
	}

	jsonData, err := json.Marshal(commits)
	if err != nil {
		return fmt.Errorf("序列化请求体失败: %w", err)
	}

	resp, err := c.HTTPClient.Post(c.BaseURL+"/api/v2/commits/batch", "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("调用 backend API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("backend API 返回错误: %d, %s", resp.StatusCode, string(respBody))
	}
	return nil
}
