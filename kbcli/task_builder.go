package main

import "time"

// TaskDoc 任务级聚合文档（从 Request 层按 task_id 归并而来）
type TaskDoc struct {
	Timestamp             time.Time `json:"@timestamp"`
	TaskID                string    `json:"task_id"`
	Caller                string    `json:"caller"`
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
	Org1                  string    `json:"org1"`
	Org2                  string    `json:"org2"`
	Org3                  string    `json:"org3"`
	Org4                  string    `json:"org4"`
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
	APICount              int64     `json:"api_count"`
	SourceFile            string    `json:"source_file"`
	TaskAncientMinutes       float64   `json:"task_ancient_minutes"`
	TaskAncientMinutesReason string    `json:"task_ancient_minutes_reason"`
}

// BuildTaskDocs 按 task_id 聚合 RawDoc，只处理 caller=="chat" 且 task_id 非空的记录
func BuildTaskDocs(rawDocs []RawDoc) []TaskDoc {
	if len(rawDocs) == 0 {
		return []TaskDoc{}
	}

	groups := make(map[string][]RawDoc)
	for _, d := range rawDocs {
		if d.Caller != "chat" || d.TaskID == "" {
			continue
		}
		groups[d.TaskID] = append(groups[d.TaskID], d)
	}

	results := make([]TaskDoc, 0, len(groups))
	for _, docs := range groups {
		first := docs[0]
		td := TaskDoc{
			TaskID:      first.TaskID,
			Caller:      first.Caller,
			ClientID:    first.ClientID,
			UserID:      first.UserID,
			UserName:    first.UserName,
			RepoID:      first.RepoID,
			ProjectPath: first.ProjectPath,
			ProjectID:   first.ProjectID,
			ClientIDE:   first.ClientIDE,
			ClientVersion: first.ClientVersion,
			ClientOS:    first.ClientOS,
			PromptMode:  first.PromptMode,
			Mode:        first.Mode,
			Org1:        first.Org1,
			Org2:        first.Org2,
			Org3:        first.Org3,
			Org4:        first.Org4,
		}

		minReqTime := first.APIRequestTime
		maxEndTime := first.APIEndTime

		for _, d := range docs {
			td.UserInChars += d.UserInChars
			td.AssistantOutCodeLines += d.AssistantOutCodeLines
			td.SystemTokens += d.SystemTokens
			td.UserTokens += d.UserTokens
			td.APIProcessTime += d.APIProcessTime
			td.APITtft += d.APITtft
			td.APIInTokens += d.APIInTokens
			td.APIOutTokens += d.APIOutTokens
			td.APICost += d.APICost

			if !d.APIRequestTime.IsZero() && (minReqTime.IsZero() || d.APIRequestTime.Before(minReqTime)) {
				minReqTime = d.APIRequestTime
			}
			if d.APIEndTime.After(maxEndTime) {
				maxEndTime = d.APIEndTime
			}
		}

		td.APIRequestTime = minReqTime
		td.APIEndTime = maxEndTime
		td.APICount = int64(len(docs))
		td.Timestamp = maxEndTime

		results = append(results, td)
	}

	return results
}
