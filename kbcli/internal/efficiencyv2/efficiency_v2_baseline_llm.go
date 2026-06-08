package efficiencyv2

import (
	"encoding/json"
	"fmt"
	"kanban/kbcli/internal/llm"
	"strings"

	"kanban/core/models"
)

const efficiencyV2LLMV4Version = "v4.0"

type EfficiencyV2NeedStructuredSummary struct {
	NeedID            string   `json:"need_id"`
	BoundarySource    string   `json:"boundary_source"`
	Status            string   `json:"status"`
	RepoAddr          string   `json:"repo_addr,omitempty"`
	RepoBranch        string   `json:"repo_branch,omitempty"`
	ChangedLOC        int64    `json:"changed_loc"`
	FileCount         int64    `json:"file_count"`
	CommitCount       int64    `json:"commit_count"`
	UncoveredLOC      int64    `json:"uncovered_loc"`
	AICodeRatio       *float64 `json:"ai_code_ratio,omitempty"`
	Silica            *float64 `json:"silica,omitempty"`
	ThinkMin          float64  `json:"think_active_min"`
	ExecutionMin      float64  `json:"exec_active_min"`
	VerificationMin   float64  `json:"verify_active_min"`
	ActivePersonMin   float64  `json:"active_person_min"`
	UncoveredHumanMin float64  `json:"uncovered_human_min"`
	KeyDecisions      []string `json:"key_decisions,omitempty"`
	SessionSummaries  []string `json:"session_summaries,omitempty"`
	CommitMessages    []string `json:"commit_messages,omitempty"`
	TaskTitles        []string `json:"task_titles,omitempty"`
	TouchedFiles      []string `json:"touched_files,omitempty"`
	DegradedReasons   []string `json:"degraded_reasons,omitempty"`
}

type EfficiencyV2LLMResult struct {
	ThinkMin    *float64
	ExecMin     *float64
	VerifyMin   *float64
	TotalMin    *float64
	Confidence  string
	Reason      string
	RawResponse string
}

type efficiencyV2LLMParseResponse struct {
	ThinkMin   float64 `json:"think_min"`
	ExecMin    float64 `json:"exec_min"`
	VerifyMin  float64 `json:"verify_min"`
	TotalMin   float64 `json:"total_min"`
	Confidence string  `json:"confidence"`
	Reason     string  `json:"reason"`
}

// BuildEfficiencyV2NeedStructuredSummary projects a Need to the structured
// fields shipped to the LLM. Internal LLM may receive commit messages, task
// titles, and touched file paths (semantic signals). Raw conversation
// transcripts and diff bodies are still excluded.
func BuildEfficiencyV2NeedStructuredSummary(need models.Need, sessions []models.SessionStageMetric, commits []models.Commit, tasks []models.Task) EfficiencyV2NeedStructuredSummary {
	summary := EfficiencyV2NeedStructuredSummary{
		NeedID:            need.NeedId,
		BoundarySource:    need.BoundarySource,
		Status:            need.Status,
		RepoAddr:          need.RepoAddr,
		RepoBranch:        need.RepoBranch,
		ChangedLOC:        need.ChangedLoc,
		FileCount:         need.FileCount,
		CommitCount:       need.CommitCount,
		UncoveredLOC:      need.UncoveredLoc,
		AICodeRatio:       need.AICodeRatio,
		Silica:            need.Silica,
		ThinkMin:          need.ThinkActiveMin,
		ExecutionMin:      need.ExecutionActiveMin,
		VerificationMin:   need.VerificationActiveMin,
		ActivePersonMin:   need.TotalSessionActivePersonMin,
		UncoveredHumanMin: need.UncoveredHumanMin,
	}
	for _, s := range sessions {
		summary.SessionSummaries = append(summary.SessionSummaries, BuildEfficiencyV2DeterministicSessionSummary(s))
	}
	for _, c := range commits {
		if msg := strings.TrimSpace(c.Comment); msg != "" {
			summary.CommitMessages = append(summary.CommitMessages, truncateForPrompt(msg, 200))
		}
	}
	titleSeen := map[string]bool{}
	for _, t := range tasks {
		title := strings.TrimSpace(t.Title)
		if title == "" || titleSeen[title] {
			continue
		}
		titleSeen[title] = true
		summary.TaskTitles = append(summary.TaskTitles, truncateForPrompt(title, 200))
	}
	if files := EfficiencyV2StringsFromJSON(need.TouchedFiles); len(files) > 0 {
		if len(files) > 30 {
			files = files[:30]
		}
		summary.TouchedFiles = files
	}
	summary.KeyDecisions = extractEfficiencyV2KeyDecisions(need)
	if need.Reason != "" {
		summary.DegradedReasons = append(summary.DegradedReasons, need.Reason)
	}
	return summary
}

func truncateForPrompt(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}

// BuildEfficiencyV2DeterministicSessionSummary produces a zero-cost template
// summary used both as the LLM input and as the always-on fallback when
// LLM cost is undesirable.
func BuildEfficiencyV2DeterministicSessionSummary(s models.SessionStageMetric) string {
	return fmt.Sprintf(
		"session=%s user=%s think=%.1f exec=%.1f verify=%.1f edit=%d verify_events=%d read=%d turns=%d confidence=%s",
		s.SessionId, s.UserId, s.ThinkActiveMin, s.ExecutionActiveMin, s.VerificationActiveMin,
		s.EditEventCount, s.VerifyEventCount, s.ReadEventCount, s.MessageEventCount, s.StageConfidence,
	)
}

// BuildEfficiencyV2LLMPrompt formats the structured prompt. It MUST not embed
// raw conversation transcripts.
func BuildEfficiencyV2LLMPrompt(summary EfficiencyV2NeedStructuredSummary) (string, error) {
	payload, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal structured summary: %w", err)
	}
	prompt := fmt.Sprintf(`你是软件工时估算专家。下面的 Need 是一组关联的 commit + 会话，已经用 AI 完成。请估算「如果一位熟悉本项目的 3-5 年中级开发者**不用 AI** 完成同样的产出，需要多少分钟」。

重点判读：
- commit_messages / task_titles 透露语义复杂度（自动生成 vs 手写逻辑 vs 简单 bug fix）
- changed_loc 巨大且 ai_code_ratio 接近 1.0 时，极可能是 bulk 自动生成，对应不用 AI 也不会按 LOC 线性算
- 多个 commit 描述高度相似时，往往是反复重试，不是多份独立工作
- touched_files 路径含 lock/generated 字样的应该折扣
- 估值应该是「熟手敲代码 + 必要测试」时间，不是「新手探索 + 完整工程闭环」

Need 结构化数据（无原文 transcript / 无 diff 内容）：
%s

请仅输出严格 JSON：
{
  "think_min": number,
  "exec_min": number,
  "verify_min": number,
  "total_min": number,
  "confidence": "high" | "medium" | "low",
  "reason": "≤80 字中文说明，引用上面的具体证据"
}`, string(payload))
	return prompt, nil
}

// CallAIForNeedEstimationV4 invokes the v2 structured LLM estimator. It returns
// a nullable result with explicit reasons when the LLM is disabled, fails, or
// returns invalid output. Retries once on JSON parse failure with a stricter
// format-only reminder (handles deepseek-flash occasional truncation / leading
// reasoning text).
func CallAIForNeedEstimationV4(summary EfficiencyV2NeedStructuredSummary, aiCfg llm.AIEstimationConfig) EfficiencyV2LLMResult {
	if !aiCfg.Enabled || strings.TrimSpace(aiCfg.APIKey) == "" {
		return EfficiencyV2LLMResult{Confidence: efficiencyV2StageConfidenceLow, Reason: "llm:disabled"}
	}
	prompt, err := BuildEfficiencyV2LLMPrompt(summary)
	if err != nil {
		return EfficiencyV2LLMResult{Confidence: efficiencyV2StageConfidenceLow, Reason: fmt.Sprintf("llm:build_prompt_failed:%v", err)}
	}
	systemPrompt := "You estimate without-AI development minutes from structured Need summaries. Output strict JSON."

	// First attempt
	messages := []llm.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: prompt},
	}
	content, err := llm.CallLLM(aiCfg, messages, 1024)
	if err != nil {
		return EfficiencyV2LLMResult{Confidence: efficiencyV2StageConfidenceLow, Reason: fmt.Sprintf("llm:call_failed:%v", err)}
	}
	result := parseEfficiencyV2LLMResponse(content)
	if result.TotalMin != nil {
		return result
	}

	// Retry once with stricter format reminder when JSON parse fails (truncated /
	// no_json / invalid_json reasons). Bump max_tokens to reduce truncation risk.
	retryMessages := []llm.ChatMessage{
		{Role: "system", Content: systemPrompt + " 仅返回 JSON 对象，无任何前置说明文字，不要 markdown 代码块包装。必须包含 think_min/exec_min/verify_min/total_min/confidence/reason 字段，total_min 必须是数字。"},
		{Role: "user", Content: prompt},
	}
	retryContent, retryErr := llm.CallLLM(aiCfg, retryMessages, 2048)
	if retryErr != nil {
		// Surface original parse reason with retry suffix
		result.Reason = result.Reason + "|retry_call_failed:" + retryErr.Error()
		return result
	}
	retryResult := parseEfficiencyV2LLMResponse(retryContent)
	if retryResult.TotalMin != nil {
		retryResult.Reason = "llm:retry_ok|" + retryResult.Reason
		return retryResult
	}
	// Both attempts failed — return second attempt's diagnostics
	retryResult.Reason = "llm:retry_failed|" + retryResult.Reason
	return retryResult
}

func parseEfficiencyV2LLMResponse(rawContent string) EfficiencyV2LLMResult {
	jsonText := llm.ExtractJSON(rawContent)
	if strings.TrimSpace(jsonText) == "" {
		return EfficiencyV2LLMResult{Confidence: efficiencyV2StageConfidenceLow, Reason: "llm:no_json", RawResponse: rawContent}
	}
	var parsed efficiencyV2LLMParseResponse
	if err := json.Unmarshal([]byte(jsonText), &parsed); err != nil {
		return EfficiencyV2LLMResult{Confidence: efficiencyV2StageConfidenceLow, Reason: fmt.Sprintf("llm:invalid_json:%v", err), RawResponse: rawContent}
	}
	if parsed.ThinkMin < 0 || parsed.ExecMin < 0 || parsed.VerifyMin < 0 || parsed.TotalMin < 0 {
		return EfficiencyV2LLMResult{Confidence: efficiencyV2StageConfidenceLow, Reason: "llm:negative_minutes", RawResponse: rawContent}
	}
	if parsed.TotalMin > 1_000_000 {
		return EfficiencyV2LLMResult{Confidence: efficiencyV2StageConfidenceLow, Reason: "llm:total_too_large", RawResponse: rawContent}
	}
	think := parsed.ThinkMin
	exec := parsed.ExecMin
	verify := parsed.VerifyMin
	total := parsed.TotalMin
	if total == 0 {
		total = think + exec + verify
	}
	confidence := strings.ToLower(strings.TrimSpace(parsed.Confidence))
	if confidence == "" {
		confidence = efficiencyV2StageConfidenceMedium
	}
	return EfficiencyV2LLMResult{
		ThinkMin:    &think,
		ExecMin:     &exec,
		VerifyMin:   &verify,
		TotalMin:    &total,
		Confidence:  confidence,
		Reason:      parsed.Reason,
		RawResponse: rawContent,
	}
}

func PersistEfficiencyV2BaselineCOnNeed(need *models.Need, result EfficiencyV2LLMResult) {
	need.BaselineLLMThinkWorkMin = result.ThinkMin
	need.BaselineLLMExecutionWorkMin = result.ExecMin
	need.BaselineLLMVerificationWorkMin = result.VerifyMin
	need.BaselineLLMTotalWorkMin = result.TotalMin
	if result.Confidence != "" {
		need.BaselineLLMConfidence = result.Confidence
	}
	need.BaselineLLMReason = result.Reason
}

func extractEfficiencyV2KeyDecisions(need models.Need) []string {
	var decisions []string
	if need.MergeTs != nil {
		decisions = append(decisions, "merged_pr")
	}
	if need.WaitForReviewMin > 0 {
		decisions = append(decisions, fmt.Sprintf("review_wait=%.1fmin", need.WaitForReviewMin))
	}
	if need.UncoveredLoc > 0 {
		decisions = append(decisions, fmt.Sprintf("uncovered_loc=%d", need.UncoveredLoc))
	}
	if need.RevertCount > 0 {
		decisions = append(decisions, fmt.Sprintf("revert_count=%d", need.RevertCount))
	}
	return decisions
}
