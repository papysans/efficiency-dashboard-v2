package efficiencyv2

import (
	"kanban/kbcli/internal/llm"
	"strings"
	"testing"
	"time"

	"kanban/core/models"
)

func TestBuildEfficiencyV2NeedStructuredSummary_NoRawTranscripts(t *testing.T) {
	need := models.Need{
		NeedId:                      "n-1",
		BoundarySource:              efficiencyV2BoundaryPR,
		Status:                      "merged",
		ChangedLoc:                  500,
		FileCount:                   8,
		CommitCount:                 4,
		UncoveredLoc:                30,
		ThinkActiveMin:              25,
		ExecutionActiveMin:          70,
		VerificationActiveMin:       12,
		TotalSessionActivePersonMin: 110,
		UncoveredHumanMin:           45,
		RevertCount:                 1,
	}
	session := models.SessionStageMetric{SessionId: "s-1", UserId: "u-alice", ThinkActiveMin: 5, EditEventCount: 3, VerifyEventCount: 2}

	commits := []models.Commit{
		{CommitId: "c1", Comment: "feat: add OIDC reset"},
		{CommitId: "c2", Comment: "fix: handle empty token"},
	}
	tasks := []models.Task{
		{TaskId: "t1", SessionId: "s-1", Title: "实现密码重置流程"},
		{TaskId: "t2", SessionId: "s-1", Title: "实现密码重置流程"}, // duplicate dedup
		{TaskId: "t3", SessionId: "s-1", Title: ""},         // empty skipped
	}
	summary := BuildEfficiencyV2NeedStructuredSummary(need, []models.SessionStageMetric{session}, commits, tasks)
	if summary.NeedID != "n-1" {
		t.Fatalf("need id = %q", summary.NeedID)
	}
	if len(summary.SessionSummaries) != 1 || summary.SessionSummaries[0] == "" {
		t.Fatalf("session summaries should be populated, got %v", summary.SessionSummaries)
	}
	if strings.Contains(summary.SessionSummaries[0], "<user_message>") {
		t.Fatalf("session summary must not include raw user message, got %q", summary.SessionSummaries[0])
	}
	if len(summary.CommitMessages) != 2 {
		t.Fatalf("commit_messages = %v, want 2 entries", summary.CommitMessages)
	}
	if len(summary.TaskTitles) != 1 || summary.TaskTitles[0] != "实现密码重置流程" {
		t.Fatalf("task_titles = %v, want 1 dedup'd entry", summary.TaskTitles)
	}
}

func TestBuildEfficiencyV2LLMPrompt_DoesNotIncludeRawTranscripts(t *testing.T) {
	summary := EfficiencyV2NeedStructuredSummary{NeedID: "n-1", BoundarySource: efficiencyV2BoundaryBranch, ChangedLOC: 200}
	prompt, err := BuildEfficiencyV2LLMPrompt(summary)
	if err != nil {
		t.Fatalf("build prompt: %v", err)
	}
	for _, forbidden := range []string{"raw_transcript", "<user_message>", "<assistant_message>"} {
		if strings.Contains(prompt, forbidden) {
			t.Fatalf("prompt must not include %q", forbidden)
		}
	}
	if !strings.Contains(prompt, "严格 JSON") {
		t.Fatalf("prompt should require strict JSON output, got: %s", prompt[:200])
	}
}

func TestCallAIForNeedEstimationV4_DisabledReturnsNullWithReason(t *testing.T) {
	result := CallAIForNeedEstimationV4(EfficiencyV2NeedStructuredSummary{NeedID: "n-1"}, llm.AIEstimationConfig{Enabled: false})
	if result.TotalMin != nil {
		t.Fatalf("total should be nil when disabled, got %v", *result.TotalMin)
	}
	if !strings.Contains(result.Reason, "disabled") {
		t.Fatalf("reason should mention disabled, got %q", result.Reason)
	}
}

func TestCallAIForNeedEstimationV4_MissingAPIKeyReturnsDisabled(t *testing.T) {
	result := CallAIForNeedEstimationV4(EfficiencyV2NeedStructuredSummary{NeedID: "n-1"}, llm.AIEstimationConfig{Enabled: true, APIKey: ""})
	if result.TotalMin != nil {
		t.Fatalf("total should be nil when API key missing")
	}
	if !strings.Contains(result.Reason, "disabled") {
		t.Fatalf("reason should mention disabled, got %q", result.Reason)
	}
}

func TestParseEfficiencyV2LLMResponse_ValidJSON(t *testing.T) {
	raw := `{"think_min": 30, "exec_min": 90, "verify_min": 15, "total_min": 135, "confidence": "medium", "reason": "Looks like a small feature"}`
	result := parseEfficiencyV2LLMResponse(raw)
	if result.TotalMin == nil || *result.TotalMin != 135 {
		t.Fatalf("total = %v, want 135", result.TotalMin)
	}
	if result.Confidence != "medium" {
		t.Fatalf("confidence = %q, want medium", result.Confidence)
	}
}

func TestParseEfficiencyV2LLMResponse_InvalidJSONReturnsReason(t *testing.T) {
	result := parseEfficiencyV2LLMResponse("not json")
	if result.TotalMin != nil {
		t.Fatalf("total should be nil for invalid JSON")
	}
	if !strings.Contains(result.Reason, "no_json") && !strings.Contains(result.Reason, "invalid_json") {
		t.Fatalf("reason should mention json error, got %q", result.Reason)
	}
}

func TestParseEfficiencyV2LLMResponse_NegativeMinutesRejected(t *testing.T) {
	result := parseEfficiencyV2LLMResponse(`{"think_min": -5, "exec_min": 10, "verify_min": 5, "total_min": 10}`)
	if result.TotalMin != nil {
		t.Fatalf("total should be nil for negative values")
	}
	if !strings.Contains(result.Reason, "negative") {
		t.Fatalf("reason should mention negative_minutes, got %q", result.Reason)
	}
}

func TestParseEfficiencyV2LLMResponse_TotalAutoComputedFromComponents(t *testing.T) {
	result := parseEfficiencyV2LLMResponse(`{"think_min": 10, "exec_min": 20, "verify_min": 5, "confidence": "high", "reason": "ok"}`)
	if result.TotalMin == nil {
		t.Fatalf("total should be auto-computed")
	}
	if *result.TotalMin != 35 {
		t.Fatalf("total = %.2f, want 35", *result.TotalMin)
	}
}

// 内网 MiniMax-M2.7-Local 等推理模型以 <think>...</think> 开头，旧 parser 见 '<' 即失败。
// 必须先剥推理块再取 JSON，且 work + 工期都要正确解出。
func TestParseEfficiencyV2LLMResponse_ReasoningModelThinkBlockThenFencedJSON(t *testing.T) {
	raw := "<think>\n先分析：两人协作，含评审往返，dev_span 3 天……\n所以单人约 40 分钟。\n</think>\n```json\n{\"think_min\":10,\"exec_min\":25,\"verify_min\":5,\"total_min\":40,\"elapsed_min\":220,\"confidence\":\"medium\",\"reason\":\"两人协作含评审\"}\n```"
	result := parseEfficiencyV2LLMResponse(raw)
	if result.TotalMin == nil || *result.TotalMin != 40 {
		t.Fatalf("total = %v, want 40 (must strip <think> + fence)", result.TotalMin)
	}
	if result.ElapsedMin == nil || *result.ElapsedMin != 220 {
		t.Fatalf("elapsed = %v, want 220", result.ElapsedMin)
	}
}

func TestParseEfficiencyV2LLMResponse_ReasoningModelThinkBlockThenBareJSON(t *testing.T) {
	// 推理块后直接裸 JSON（无 fence），且推理文本里含会误导朴素扫描的花括号 { 和 }。
	raw := "<THINK>估算框架: 用 {loc, commits} 推 work；多人需 elapsed。结论如下</THINK>{\"total_min\":40,\"elapsed_min\":180,\"confidence\":\"high\",\"reason\":\"ok\"}"
	result := parseEfficiencyV2LLMResponse(raw)
	if result.TotalMin == nil || *result.TotalMin != 40 {
		t.Fatalf("total = %v, want 40 (bare JSON after think block)", result.TotalMin)
	}
	if result.ElapsedMin == nil || *result.ElapsedMin != 180 {
		t.Fatalf("elapsed = %v, want 180", result.ElapsedMin)
	}
}

func TestParseEfficiencyV2LLMResponse_LeadingAndTrailingProseAroundJSON(t *testing.T) {
	raw := "好的，根据数据我的估算是：\n{\"total_min\":50,\"elapsed_min\":300,\"confidence\":\"medium\",\"reason\":\"中等复杂\"}\n以上，如需调整请告知。"
	result := parseEfficiencyV2LLMResponse(raw)
	if result.TotalMin == nil || *result.TotalMin != 50 {
		t.Fatalf("total = %v, want 50 (prose around single JSON object)", result.TotalMin)
	}
	if result.ElapsedMin == nil || *result.ElapsedMin != 300 {
		t.Fatalf("elapsed = %v, want 300", result.ElapsedMin)
	}
}

func TestParseEfficiencyV2LLMResponse_ReasonTextWithBracesStillBalances(t *testing.T) {
	// reason 字符串字面量里含花括号与转义引号，配平扫描须把字符串内的 {}/" 跳过。
	raw := `{"total_min":30,"elapsed_min":120,"confidence":"low","reason":"模板 {x} 里的 \"占位符\" 多"}`
	result := parseEfficiencyV2LLMResponse(raw)
	if result.TotalMin == nil || *result.TotalMin != 30 {
		t.Fatalf("total = %v, want 30 (braces inside string must not break balance)", result.TotalMin)
	}
}

func TestParseEfficiencyV2LLMResponse_PureHTMLErrorStillFails(t *testing.T) {
	// 真错误（如 502 网关 HTML，无任何 {}）：必须仍失败、返 low-conf，别误吞成估算。
	result := parseEfficiencyV2LLMResponse("<html><head><title>502 Bad Gateway</title></head><body>nginx</body></html>")
	if result.TotalMin != nil {
		t.Fatalf("pure HTML must not parse into an estimate, got total=%v", *result.TotalMin)
	}
	if result.Confidence != "low" {
		t.Fatalf("confidence = %q, want low for HTML error", result.Confidence)
	}
	if !strings.Contains(result.Reason, "no_json") {
		t.Fatalf("reason should be no_json for HTML without braces, got %q", result.Reason)
	}
}

func TestParseEfficiencyV2LLMResponse_ThinkBlockWithNoJSONFails(t *testing.T) {
	// 推理块吞掉全部内容、剥完无 JSON：仍失败返 low-conf（别把空当估算）。
	result := parseEfficiencyV2LLMResponse("<think>我需要更多信息才能估算，无法给出数字。</think>")
	if result.TotalMin != nil {
		t.Fatalf("think-only response must not parse, got total=%v", *result.TotalMin)
	}
	if !strings.Contains(result.Reason, "no_json") {
		t.Fatalf("reason should be no_json, got %q", result.Reason)
	}
}

// 直接对提取器做单元覆盖（边界更细）。
func TestExtractEfficiencyV2JSON_StripsMultipleThinkBlocks(t *testing.T) {
	raw := "<think>第一段推理</think>噪声<think>第二段推理 {含括号}</think>{\"total_min\":5}"
	got := extractEfficiencyV2JSON(raw)
	if got != `{"total_min":5}` {
		t.Fatalf("extracted = %q, want clean JSON after stripping both think blocks", got)
	}
}

func TestExtractEfficiencyV2JSON_FirstBalancedObjectNotGreedyLastBrace(t *testing.T) {
	// 首个 JSON 对象是答案；后面若还有 } 不应被贪婪吞进来。验证正向配平取首个对象。
	raw := `{"total_min":40,"elapsed_min":200} 备注: 结束}`
	got := extractEfficiencyV2JSON(raw)
	if got != `{"total_min":40,"elapsed_min":200}` {
		t.Fatalf("extracted = %q, want first balanced object only", got)
	}
}

func TestExtractEfficiencyV2JSON_NoBracesReturnsEmpty(t *testing.T) {
	if got := extractEfficiencyV2JSON("纯文本无 JSON 502 error"); got != "" {
		t.Fatalf("no-brace input should yield empty extraction, got %q", got)
	}
}

func TestPersistEfficiencyV2BaselineCOnNeed_NullableFields(t *testing.T) {
	need := models.Need{NeedId: "n-1"}
	PersistEfficiencyV2BaselineCOnNeed(&need, EfficiencyV2LLMResult{Confidence: "low", Reason: "llm:disabled"})
	if need.BaselineLLMTotalWorkMin != nil {
		t.Fatalf("total should remain nil")
	}
	if need.BaselineLLMCalendarMin != nil {
		t.Fatalf("calendar should remain nil when LLM gives no elapsed")
	}
	if need.BaselineLLMConfidence != "low" {
		t.Fatalf("confidence = %q, want low", need.BaselineLLMConfidence)
	}
	if need.BaselineLLMReason != "llm:disabled" {
		t.Fatalf("reason = %q, want llm:disabled", need.BaselineLLMReason)
	}
}

// 块3工期维度：summary 必须携带 contributor_count / dev_span_days / total_wall_min。
func TestBuildEfficiencyV2NeedStructuredSummary_DurationDimensions(t *testing.T) {
	start := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 4, 9, 0, 0, 0, time.UTC) // 3 天跨度
	need := models.Need{
		NeedId:             "n-1",
		Status:             "merged",
		ContributorUserIds: models.StringJSON(`["u-alice","u-bob","u-carol"]`),
		DevStartTs:         &start,
		DevEndTs:           &end,
		TotalWallMin:       720,
	}
	summary := BuildEfficiencyV2NeedStructuredSummary(need, nil, nil, nil)
	if summary.ContributorCount != 3 {
		t.Fatalf("contributor_count = %d, want 3", summary.ContributorCount)
	}
	if summary.DevSpanDays != 3 {
		t.Fatalf("dev_span_days = %.2f, want 3", summary.DevSpanDays)
	}
	if summary.TotalWallMin != 720 {
		t.Fatalf("total_wall_min = %.1f, want 720", summary.TotalWallMin)
	}
}

func TestBuildEfficiencyV2NeedStructuredSummary_DevSpanZeroWhenMissingTs(t *testing.T) {
	summary := BuildEfficiencyV2NeedStructuredSummary(models.Need{NeedId: "n-1"}, nil, nil, nil)
	if summary.DevSpanDays != 0 {
		t.Fatalf("dev_span_days should be 0 when ts missing, got %.2f", summary.DevSpanDays)
	}
	if summary.ContributorCount != 0 {
		t.Fatalf("contributor_count should be 0 for empty list, got %d", summary.ContributorCount)
	}
}

func TestBuildEfficiencyV2LLMPrompt_AsksForElapsedMin(t *testing.T) {
	prompt, err := BuildEfficiencyV2LLMPrompt(EfficiencyV2NeedStructuredSummary{NeedID: "n-1", ContributorCount: 2})
	if err != nil {
		t.Fatalf("build prompt: %v", err)
	}
	if !strings.Contains(prompt, "elapsed_min") {
		t.Fatalf("prompt must request elapsed_min (自然日历工期)")
	}
	if !strings.Contains(prompt, "contributor_count") {
		t.Fatalf("prompt must reference contributor_count for same-team anchoring")
	}
}

// 块3工期维度：parse 必须解出 elapsed_min（正值采纳；0/负/缺省 → nil 回退）。
func TestParseEfficiencyV2LLMResponse_ElapsedMinParsed(t *testing.T) {
	raw := `{"think_min":10,"exec_min":20,"verify_min":5,"total_min":35,"elapsed_min":180,"confidence":"medium","reason":"两人协作含评审"}`
	result := parseEfficiencyV2LLMResponse(raw)
	if result.ElapsedMin == nil || *result.ElapsedMin != 180 {
		t.Fatalf("elapsed = %v, want 180", result.ElapsedMin)
	}
}

func TestParseEfficiencyV2LLMResponse_ElapsedMinMissingStaysNil(t *testing.T) {
	result := parseEfficiencyV2LLMResponse(`{"think_min":10,"exec_min":20,"verify_min":5,"total_min":35,"confidence":"high","reason":"ok"}`)
	if result.TotalMin == nil {
		t.Fatalf("total should parse")
	}
	if result.ElapsedMin != nil {
		t.Fatalf("elapsed should be nil when model omits it, got %v", *result.ElapsedMin)
	}
}

func TestParseEfficiencyV2LLMResponse_ElapsedMinZeroOrNegativeRejected(t *testing.T) {
	for _, raw := range []string{
		`{"total_min":35,"elapsed_min":0,"reason":"x"}`,
		`{"total_min":35,"elapsed_min":-5,"reason":"x"}`,
	} {
		result := parseEfficiencyV2LLMResponse(raw)
		if result.ElapsedMin != nil {
			t.Fatalf("elapsed %s should be rejected → nil, got %v", raw, *result.ElapsedMin)
		}
		// total 仍应可用（elapsed 无效不连累 work 估算）
		if result.TotalMin == nil {
			t.Fatalf("total should still parse for %s", raw)
		}
	}
}

func TestParseEfficiencyV2LLMResponse_PersistCalendarMin(t *testing.T) {
	result := parseEfficiencyV2LLMResponse(`{"total_min":40,"elapsed_min":200,"confidence":"high","reason":"ok"}`)
	need := models.Need{NeedId: "n-1"}
	PersistEfficiencyV2BaselineCOnNeed(&need, result)
	if need.BaselineLLMCalendarMin == nil || *need.BaselineLLMCalendarMin != 200 {
		t.Fatalf("baseline_llm_calendar_min = %v, want 200", need.BaselineLLMCalendarMin)
	}
}

// 块1缓存：指纹覆盖 changed_loc + 排序 commit/session/touched_files，集合等价输入产生同一指纹。
func TestEfficiencyV2NeedLLMInputHash_OrderIndependentStable(t *testing.T) {
	a := models.Need{
		ChangedLoc:   500,
		CommitIds:    models.StringJSON(`["c2","c1"]`),
		SessionIds:   models.StringJSON(`["s2","s1"]`),
		TouchedFiles: models.StringJSON(`["b.go","a.go"]`),
	}
	b := models.Need{
		ChangedLoc:   500,
		CommitIds:    models.StringJSON(`["c1","c2"]`),
		SessionIds:   models.StringJSON(`["s1","s2"]`),
		TouchedFiles: models.StringJSON(`["a.go","b.go"]`),
	}
	if EfficiencyV2NeedLLMInputHash(a) != EfficiencyV2NeedLLMInputHash(b) {
		t.Fatalf("hash must be order-independent for equivalent sets")
	}
}

func TestEfficiencyV2NeedLLMInputHash_ChangesWhenInputChanges(t *testing.T) {
	base := models.Need{ChangedLoc: 500, CommitIds: models.StringJSON(`["c1"]`)}
	h0 := EfficiencyV2NeedLLMInputHash(base)
	// changed_loc 变
	if EfficiencyV2NeedLLMInputHash(models.Need{ChangedLoc: 501, CommitIds: models.StringJSON(`["c1"]`)}) == h0 {
		t.Fatalf("hash should change when changed_loc changes")
	}
	// commit 集合变
	if EfficiencyV2NeedLLMInputHash(models.Need{ChangedLoc: 500, CommitIds: models.StringJSON(`["c1","c2"]`)}) == h0 {
		t.Fatalf("hash should change when commit set changes")
	}
}

// 块3工期输入也必须驱动指纹：contributor_user_ids / dev_start_ts / dev_end_ts 任一变化都要重算，
// 否则 dev_ts 的 now-clamp（时区双偏移修正）会在 LOC/commit 不变时悄悄换跨度、服旧的日历基线。
func TestEfficiencyV2NeedLLMInputHash_ChangesWhenDurationInputsChange(t *testing.T) {
	t0 := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 6, 4, 9, 0, 0, 0, time.UTC)
	t1b := time.Date(2026, 6, 5, 9, 0, 0, 0, time.UTC) // dev_end 漂了一天（clamp 场景）
	base := models.Need{
		ChangedLoc:         500,
		CommitIds:          models.StringJSON(`["c1"]`),
		ContributorUserIds: models.StringJSON(`["u-alice"]`),
		DevStartTs:         &t0,
		DevEndTs:           &t1,
	}
	h0 := EfficiencyV2NeedLLMInputHash(base)

	// contributor 集合变（多人 → 工期不同）
	withMoreContributors := base
	withMoreContributors.ContributorUserIds = models.StringJSON(`["u-alice","u-bob"]`)
	if EfficiencyV2NeedLLMInputHash(withMoreContributors) == h0 {
		t.Fatalf("hash should change when contributor_user_ids changes")
	}

	// dev_end_ts 漂移（now-clamp 改了跨度，LOC/commit 不变）
	withDrift := base
	withDrift.DevEndTs = &t1b
	if EfficiencyV2NeedLLMInputHash(withDrift) == h0 {
		t.Fatalf("hash should change when dev_end_ts drifts (clamp scenario)")
	}

	// dev_start_ts 变
	withStartShift := base
	withStartShift.DevStartTs = &t1b
	if EfficiencyV2NeedLLMInputHash(withStartShift) == h0 {
		t.Fatalf("hash should change when dev_start_ts changes")
	}
}

// dev_ts 统一 UTC 序列化：同一时刻不同时区写法（offset）不应产生不同指纹。
func TestEfficiencyV2NeedLLMInputHash_DevTsTimezoneNormalized(t *testing.T) {
	utc := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	shanghai := utc.In(time.FixedZone("CST", 8*3600)) // 同一时刻，+08:00 表示
	a := models.Need{ChangedLoc: 1, DevStartTs: &utc}
	b := models.Need{ChangedLoc: 1, DevStartTs: &shanghai}
	if EfficiencyV2NeedLLMInputHash(a) != EfficiencyV2NeedLLMInputHash(b) {
		t.Fatalf("same instant in different zones must hash identically")
	}
}

// 块1缓存解耦：指纹命中且已有 work 估算 → 不调 LLM；变化/无估算/force → 调 LLM。
func TestEfficiencyV2ShouldCallLLM_CacheHitSkips(t *testing.T) {
	total := 120.0
	need := models.Need{ChangedLoc: 500, CommitIds: models.StringJSON(`["c1"]`), BaselineLLMTotalWorkMin: &total}
	need.LLMInputHash = EfficiencyV2NeedLLMInputHash(need)
	should, hash := EfficiencyV2ShouldCallLLM(need, false)
	if should {
		t.Fatalf("should skip LLM on cache hit with existing estimate")
	}
	if hash != need.LLMInputHash {
		t.Fatalf("returned hash must equal current input hash")
	}
}

func TestEfficiencyV2ShouldCallLLM_InvalidatedRecalls(t *testing.T) {
	total := 120.0
	need := models.Need{ChangedLoc: 500, CommitIds: models.StringJSON(`["c1"]`), BaselineLLMTotalWorkMin: &total}
	need.LLMInputHash = "stale-hash-from-old-input"
	if should, _ := EfficiencyV2ShouldCallLLM(need, false); !should {
		t.Fatalf("must re-call LLM when input hash no longer matches")
	}
}

func TestEfficiencyV2ShouldCallLLM_NoPriorEstimateRecalls(t *testing.T) {
	need := models.Need{ChangedLoc: 500, CommitIds: models.StringJSON(`["c1"]`)}
	need.LLMInputHash = EfficiencyV2NeedLLMInputHash(need) // hash matches but no prior estimate
	if should, _ := EfficiencyV2ShouldCallLLM(need, false); !should {
		t.Fatalf("must call LLM when no prior baseline_llm_total_work_min exists")
	}
}

func TestEfficiencyV2ShouldCallLLM_ForceAlwaysCalls(t *testing.T) {
	total := 120.0
	need := models.Need{ChangedLoc: 500, CommitIds: models.StringJSON(`["c1"]`), BaselineLLMTotalWorkMin: &total}
	need.LLMInputHash = EfficiencyV2NeedLLMInputHash(need)
	if should, _ := EfficiencyV2ShouldCallLLM(need, true); !should {
		t.Fatalf("force=true must always call LLM even on cache hit")
	}
}
