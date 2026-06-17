package efficiencyv2

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"kanban/kbcli/internal/llm"
	"strings"
	"time"

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
	// 工期维度（同等团队锚定）：喂给 LLM 估「同 N 人无 AI 自然日历工期」。
	ContributorCount int      `json:"contributor_count"`        // = len(contributor_user_ids)，参与该 need 的人数
	DevSpanDays      float64  `json:"dev_span_days"`            // = (dev_end_ts - dev_start_ts) 自然天，实测开发跨度
	TotalWallMin     float64  `json:"total_wall_min,omitempty"` // 会话挂钟分钟（含等待），人时分布参考
	KeyDecisions     []string `json:"key_decisions,omitempty"`
	SessionSummaries []string `json:"session_summaries,omitempty"`
	CommitMessages   []string `json:"commit_messages,omitempty"`
	TaskTitles       []string `json:"task_titles,omitempty"`
	TouchedFiles     []string `json:"touched_files,omitempty"`
	DegradedReasons  []string `json:"degraded_reasons,omitempty"`
}

type EfficiencyV2LLMResult struct {
	ThinkMin    *float64
	ExecMin     *float64
	VerifyMin   *float64
	TotalMin    *float64
	ElapsedMin  *float64 // 同等团队无 AI 的自然日历工期（块3工期维度）；缺省/无效时为 nil → 融合走回退
	Confidence  string
	Reason      string
	RawResponse string
}

type efficiencyV2LLMParseResponse struct {
	ThinkMin   float64  `json:"think_min"`
	ExecMin    float64  `json:"exec_min"`
	VerifyMin  float64  `json:"verify_min"`
	TotalMin   float64  `json:"total_min"`
	ElapsedMin *float64 `json:"elapsed_min"` // *float64 区分"模型没给"与"给了 0"
	Confidence string   `json:"confidence"`
	Reason     string   `json:"reason"`
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
		ContributorCount:  len(EfficiencyV2StringsFromJSON(need.ContributorUserIds)),
		DevSpanDays:       efficiencyV2NeedDevSpanDays(need),
		TotalWallMin:      need.TotalWallMin,
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

// efficiencyV2NeedDevSpanDays 实测开发跨度（自然天）= (dev_end_ts - dev_start_ts)。
// 任一端缺失或反向时返回 0（让 LLM 知道无实测跨度可参考，回退到其它信号）。
func efficiencyV2NeedDevSpanDays(need models.Need) float64 {
	if need.DevStartTs == nil || need.DevEndTs == nil {
		return 0
	}
	d := need.DevEndTs.Sub(*need.DevStartTs).Hours() / 24
	if d < 0 {
		return 0
	}
	return d
}

// EfficiencyV2NeedLLMInputHash 计算 LLM 输入指纹：覆盖**全部**会改变估算输入的字段——
// 块1 work 侧：changed_loc + 排序去重 commit_ids/session_ids/touched_files；
// 块3 工期侧：排序去重 contributor_user_ids + dev_start_ts + dev_end_ts（驱动 elapsed_min）。
// 指纹命中且已有 baseline_llm_total_work_min 时复用缓存、跳过网络调用（块1缓存解耦）。
// 排序保证集合等价输入产生同一指纹；任一字段变化即指纹变化触发重调。
// 注意：dev_start/end_ts 含 now-clamp（时区双偏移修正），若工期输入漏进指纹，clamp 后
// LOC/commit 不变但跨度变了会服旧的日历基线——故工期三维必须进指纹。
func EfficiencyV2NeedLLMInputHash(need models.Need) string {
	commitIDs := EfficiencyV2SortedUnique(EfficiencyV2StringsFromJSON(need.CommitIds))
	sessionIDs := EfficiencyV2SortedUnique(EfficiencyV2StringsFromJSON(need.SessionIds))
	files := EfficiencyV2SortedUnique(EfficiencyV2StringsFromJSON(need.TouchedFiles))
	contributors := EfficiencyV2SortedUnique(EfficiencyV2StringsFromJSON(need.ContributorUserIds))
	devStart := efficiencyV2HashTimePtr(need.DevStartTs)
	devEnd := efficiencyV2HashTimePtr(need.DevEndTs)
	payload := struct {
		ChangedLOC   int64    `json:"changed_loc"`
		CommitIDs    []string `json:"commit_ids"`
		SessionIDs   []string `json:"session_ids"`
		Files        []string `json:"touched_files"`
		Contributors []string `json:"contributor_user_ids"`
		DevStartTs   string   `json:"dev_start_ts"`
		DevEndTs     string   `json:"dev_end_ts"`
	}{
		ChangedLOC:   need.ChangedLoc,
		CommitIDs:    commitIDs,
		SessionIDs:   sessionIDs,
		Files:        files,
		Contributors: contributors,
		DevStartTs:   devStart,
		DevEndTs:     devEnd,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		// 退化：拼接字符串兜底，绝不返回空（空 hash 会被误判"未算过"反复重调）。
		raw = []byte(fmt.Sprintf("%d|%s|%s|%s|%s|%s|%s", need.ChangedLoc,
			strings.Join(commitIDs, ","), strings.Join(sessionIDs, ","), strings.Join(files, ","),
			strings.Join(contributors, ","), devStart, devEnd))
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// efficiencyV2HashTimePtr 把可空时间稳定序列化进指纹：nil→空串，否则 UTC RFC3339Nano。
// 统一 UTC 避免同一时刻不同位置写法（offset）产生不同指纹。
func efficiencyV2HashTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// EfficiencyV2ShouldCallLLM 决定是否需要真正发起 LLM 网络调用。
// force=true 永远调；否则当指纹与上次一致且已有缓存的 work 估算时，复用缓存、跳过调用。
// 返回 (shouldCall, newHash)。批量重算默认 force=false → 命中缓存不再无条件打 LLM（避免 429）。
func EfficiencyV2ShouldCallLLM(need models.Need, force bool) (bool, string) {
	newHash := EfficiencyV2NeedLLMInputHash(need)
	if force {
		return true, newHash
	}
	cached := strings.TrimSpace(need.LLMInputHash) != "" &&
		need.LLMInputHash == newHash &&
		need.BaselineLLMTotalWorkMin != nil
	return !cached, newHash
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
	prompt := fmt.Sprintf(`你是软件工时估算专家。下面的 Need 是一组关联的 commit + 会话，已经用 AI 完成（可能由 contributor_count 个人协作）。请做两项估算。

【一】单人工时 total_min：如果一位熟悉本项目的 3-5 年中级开发者**不用 AI** 独自完成同样的产出，纯敲代码+必要测试需要多少分钟（不含等待/协调/评审）。

【二】自然日历工期 elapsed_min：如果**同样这 contributor_count 个人都不用 AI**、按真实团队节奏交付同样产出，从开工到完成的**自然日历分钟**约多少（要计入：代码评审往返、跨人协调沟通、串行依赖等待；并行人数越多越能压缩，但协调成本上升）。dev_span_days 是本次有 AI 时的实测开发跨度，仅作量级参考——无 AI 通常更慢。

重点判读：
- commit_messages / task_titles 透露语义复杂度（自动生成 vs 手写逻辑 vs 简单 bug fix）
- changed_loc 巨大且 ai_code_ratio 接近 1.0 时，极可能是 bulk 自动生成，对应不用 AI 也不会按 LOC 线性算
- 多个 commit 描述高度相似时，往往是反复重试，不是多份独立工作
- touched_files 路径含 lock/generated 字样的应该折扣
- think/exec/verify 之和应等于 total_min；elapsed_min 通常 ≥ total_min（多人时被协调/评审/串行拉长，单人时约等于 total_min）

Need 结构化数据（无原文 transcript / 无 diff 内容）：
%s

请仅输出严格 JSON：
{
  "think_min": number,
  "exec_min": number,
  "verify_min": number,
  "total_min": number,
  "elapsed_min": number,
  "confidence": "high" | "medium" | "low",
  "reason": "≤80 字中文说明，引用上面的具体证据（含为何 elapsed_min 这样定）"
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
		{Role: "system", Content: systemPrompt + " 仅返回 JSON 对象，无任何前置说明文字，不要 markdown 代码块包装。必须包含 think_min/exec_min/verify_min/total_min/elapsed_min/confidence/reason 字段，total_min 与 elapsed_min 必须是数字。"},
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
	out := EfficiencyV2LLMResult{
		ThinkMin:    &think,
		ExecMin:     &exec,
		VerifyMin:   &verify,
		TotalMin:    &total,
		Confidence:  confidence,
		Reason:      parsed.Reason,
		RawResponse: rawContent,
	}
	// 工期维度：仅在模型给出且为正且不离谱时采纳，否则留 nil → 融合走 work/density 回退。
	// 负/0/超大都视为缺失（与 work 侧同 1e6 上界，0 没有"自然工期为零"的合理语义）。
	if parsed.ElapsedMin != nil {
		e := *parsed.ElapsedMin
		if e > 0 && e <= 1_000_000 {
			out.ElapsedMin = &e
		}
	}
	return out
}

func PersistEfficiencyV2BaselineCOnNeed(need *models.Need, result EfficiencyV2LLMResult) {
	need.BaselineLLMThinkWorkMin = result.ThinkMin
	need.BaselineLLMExecutionWorkMin = result.ExecMin
	need.BaselineLLMVerificationWorkMin = result.VerifyMin
	need.BaselineLLMTotalWorkMin = result.TotalMin
	// 块3工期维度：LLM 直接估的「同等团队无 AI 自然日历工期」独立落列，供融合作日历基线锚定。
	need.BaselineLLMCalendarMin = result.ElapsedMin
	if result.Confidence != "" {
		need.BaselineLLMConfidence = result.Confidence
	}
	need.BaselineLLMReason = result.Reason
}

func extractEfficiencyV2KeyDecisions(need models.Need) []string {
	var decisions []string
	if need.UncoveredLoc > 0 {
		decisions = append(decisions, fmt.Sprintf("uncovered_loc=%d", need.UncoveredLoc))
	}
	if need.RevertCount > 0 {
		decisions = append(decisions, fmt.Sprintf("revert_count=%d", need.RevertCount))
	}
	return decisions
}
