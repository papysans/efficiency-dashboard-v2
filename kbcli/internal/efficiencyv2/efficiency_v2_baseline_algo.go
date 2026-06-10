package efficiencyv2

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"kanban/core/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	efficiencyV2BaselineAlgoVersionDefault = "algo_v1"
)

type EfficiencyV2BaselineACoefficients struct {
	Version              string
	ThinkUserCharsMin    float64 `json:"think_user_chars_min_per_char"`
	ThinkFilesReadMin    float64 `json:"think_files_read_min"`
	ThinkTurnMin         float64 `json:"think_turn_min"`
	ThinkPlanReadMin     float64 `json:"think_plan_read_min"`
	ExecLinesPerMin      float64 `json:"exec_lines_per_min"`
	ExecFileCoordMin     float64 `json:"exec_file_coord_min"`
	ExecLargeDeleteRatio float64 `json:"exec_large_delete_ratio"`
	VerifyTestMin        float64 `json:"verify_test_min"`
	VerifyRePromptMin    float64 `json:"verify_reprompt_min"`
	VerifyReadMin        float64 `json:"verify_read_min"`
	MinThinkMin          float64 `json:"min_think_min"`
	MinExecMin           float64 `json:"min_exec_min"`
	MinVerifyMin         float64 `json:"min_verify_min"`
}

type EfficiencyV2BaselineAResult struct {
	ThinkMin        *float64
	ExecMin         *float64
	VerifyMin       *float64
	TotalMin        *float64
	Reasons         []string
	CoefVersionUsed string
}

func DefaultEfficiencyV2BaselineACoefficients() EfficiencyV2BaselineACoefficients {
	return EfficiencyV2BaselineACoefficients{
		Version: efficiencyV2BaselineAlgoVersionDefault,
		// 古法时代每个字符的"读+理解"成本（chars 来自用户输入）。
		ThinkUserCharsMin: 0.02,
		// 古法时代单次 grep/read 的"想关键词+IDE 搜+读结果+判断"循环。
		// 注意：当前真实数据里 conversation 归一化层很少产生 read event_kind，
		// read_event_count 基本为 0；该系数主要在 fixture 路径生效。
		ThinkFilesReadMin: 5.0,
		// 古法时代 1 轮交互（message turn）≈ 想问题+查资料+决策约 5 分钟。
		// 由于 read events 在真实数据中缺失，turn 是当前 think 段的主要信号。
		ThinkTurnMin:         5.0,
		ThinkPlanReadMin:     0.5,
		ExecLinesPerMin:      efficiencyV2DefaultLinesPerMinute,
		ExecFileCoordMin:     30,
		ExecLargeDeleteRatio: 0.6,
		VerifyTestMin:        5,
		VerifyRePromptMin:    5,
		VerifyReadMin:        1,
		MinThinkMin:          5,
		MinExecMin:           5,
		MinVerifyMin:         5,
	}
}

// LoadEfficiencyV2BaselineACoefficients returns the effective coefficients,
// preferring a persisted version when available. Cold-start fallback is the
// deterministic default set.
func LoadEfficiencyV2BaselineACoefficients(db *gorm.DB, version string) EfficiencyV2BaselineACoefficients {
	if version == "" {
		version = efficiencyV2BaselineAlgoVersionDefault
	}
	defaults := DefaultEfficiencyV2BaselineACoefficients()
	defaults.Version = version
	if db == nil {
		return defaults
	}
	var row models.BaselineCoefficient
	if err := db.Where("coef_version = ?", version).First(&row).Error; err != nil {
		return defaults
	}
	merged := defaults
	if err := json.Unmarshal([]byte(row.Algo), &merged); err == nil {
		merged.Version = row.CoefVersion
	}
	return merged
}

// EnsureEfficiencyV2BaselineACoefficients writes the default coefficient row if
// no row exists for the requested version. This is the cold-start bootstrap.
func EnsureEfficiencyV2BaselineACoefficients(db *gorm.DB, version string) error {
	if version == "" {
		version = efficiencyV2BaselineAlgoVersionDefault
	}
	defaults := DefaultEfficiencyV2BaselineACoefficients()
	defaults.Version = version
	algo, err := json.Marshal(defaults)
	if err != nil {
		return fmt.Errorf("marshal baseline coefficients: %w", err)
	}
	row := models.BaselineCoefficient{
		CoefVersion: version,
		CreatedTs:   time.Now().UTC(),
		Algo:        models.ObjectJSON(algo),
		Metadata:    models.ObjectJSON("{}"),
		Source:      "cold_start",
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error
}

// ComputeEfficiencyV2BaselineA produces think/exec/verify/total baseline work
// minutes for a Need from its sessions, events and commits. The implementation
// uses versioned coefficients and never sums legacy commit_ancient_minutes.
func ComputeEfficiencyV2BaselineA(need models.Need, sessions []models.SessionStageMetric, events []models.ConversationEvent, commits []models.Commit, coefs EfficiencyV2BaselineACoefficients) EfficiencyV2BaselineAResult {
	coefs = mergeEfficiencyV2BaselineADefaults(coefs)

	think, thinkReason := computeEfficiencyV2BaselineThink(sessions, events, coefs)
	exec, execReason := computeEfficiencyV2BaselineExec(need, commits, coefs)
	verify, verifyReason := computeEfficiencyV2BaselineVerify(sessions, coefs)

	reasons := make([]string, 0, 3)
	if thinkReason != "" {
		reasons = append(reasons, thinkReason)
	}
	if execReason != "" {
		reasons = append(reasons, execReason)
	}
	if verifyReason != "" {
		reasons = append(reasons, verifyReason)
	}

	var total *float64
	if think != nil || exec != nil || verify != nil {
		sum := 0.0
		if think != nil {
			sum += *think
		}
		if exec != nil {
			sum += *exec
		}
		if verify != nil {
			sum += *verify
		}
		total = &sum
	}

	return EfficiencyV2BaselineAResult{
		ThinkMin:        think,
		ExecMin:         exec,
		VerifyMin:       verify,
		TotalMin:        total,
		Reasons:         reasons,
		CoefVersionUsed: coefs.Version,
	}
}

func computeEfficiencyV2BaselineThink(sessions []models.SessionStageMetric, events []models.ConversationEvent, coefs EfficiencyV2BaselineACoefficients) (*float64, string) {
	if len(sessions) == 0 && len(events) == 0 {
		return nil, "think:no_signals"
	}

	userChars := 0
	readCount := int64(0)
	turnCount := int64(0)

	for _, s := range sessions {
		readCount += s.ReadEventCount
		turnCount += s.MessageEventCount
	}
	for _, e := range events {
		if e.EventKind == "message" || e.EventKind == "user_message" {
			userChars += efficiencyV2PayloadCharCount(e.Payload)
		}
	}

	if userChars == 0 && readCount == 0 && turnCount == 0 {
		think := coefs.MinThinkMin
		return &think, "think:missing_feature_default"
	}

	think := float64(userChars)*coefs.ThinkUserCharsMin +
		float64(readCount)*coefs.ThinkFilesReadMin +
		float64(turnCount)*coefs.ThinkTurnMin
	if think < coefs.MinThinkMin {
		think = coefs.MinThinkMin
	}
	return &think, ""
}

func computeEfficiencyV2BaselineExec(need models.Need, commits []models.Commit, coefs EfficiencyV2BaselineACoefficients) (*float64, string) {
	if len(commits) == 0 {
		return nil, "exec:no_commits"
	}

	touched := EfficiencyV2StringsFromJSON(need.TouchedFiles)
	filteredOut, kept := classifyEfficiencyV2ExecFiles(touched)
	fileCount := int64(len(kept))

	totalLOC := int64(0)
	var grossInsertions, grossDeletions int64
	for _, commit := range commits {
		// 估时 loc 口径统一走治理后的有效行数（softcap/降权/重放去重折算）
		lines := commit.GetEffectiveDiffLines()
		totalLOC += lines
		if lines >= 0 {
			grossInsertions += lines
		} else {
			grossDeletions += -lines
		}
	}

	keptRatio := 1.0
	if len(touched) > 0 {
		keptRatio = float64(len(kept)) / float64(len(touched))
	}
	filteredLOC := int64(float64(totalLOC) * keptRatio)
	if filteredLOC < 0 {
		filteredLOC = 0
	}

	exec := float64(filteredLOC)/positiveOrDefault(coefs.ExecLinesPerMin, efficiencyV2DefaultLinesPerMinute) +
		float64(fileCount)*coefs.ExecFileCoordMin
	if exec < coefs.MinExecMin {
		exec = coefs.MinExecMin
	}

	reasonParts := []string{}
	if len(filteredOut) > 0 {
		reasonParts = append(reasonParts, fmt.Sprintf("exec:filtered_files=%d", len(filteredOut)))
	}
	if grossDeletions > 0 && grossInsertions > 0 && coefs.ExecLargeDeleteRatio > 0 {
		ratio := float64(grossDeletions) / float64(grossDeletions+grossInsertions)
		if ratio >= coefs.ExecLargeDeleteRatio {
			reasonParts = append(reasonParts, fmt.Sprintf("exec:large_deletion_ratio=%.2f", ratio))
		}
	}
	if isEfficiencyV2FormatterOnlyByMessages(commits) {
		reasonParts = append(reasonParts, "exec:formatter_only_commits")
		exec = coefs.MinExecMin
	}

	return &exec, strings.Join(reasonParts, "; ")
}

func computeEfficiencyV2BaselineVerify(sessions []models.SessionStageMetric, coefs EfficiencyV2BaselineACoefficients) (*float64, string) {
	if len(sessions) == 0 {
		return nil, "verify:no_sessions"
	}

	var verifyEvents, rePrompts, readEvents, editEvents int64
	for _, s := range sessions {
		verifyEvents += s.VerifyEventCount
		rePrompts += s.RePromptCount
		readEvents += s.ReadEventCount
		editEvents += s.EditEventCount
	}

	if verifyEvents == 0 && rePrompts == 0 {
		verify := coefs.MinVerifyMin
		return &verify, "verify:missing_feature_default"
	}

	verify := float64(verifyEvents)*coefs.VerifyTestMin +
		float64(rePrompts)*coefs.VerifyRePromptMin
	if editEvents > 0 && readEvents > 0 {
		// review reads after edits — reuse a portion of read count proportional to verify weight
		reviewReadEstimate := float64(readEvents) * float64(verifyEvents) / float64(verifyEvents+editEvents)
		verify += reviewReadEstimate * coefs.VerifyReadMin
	}
	if verify < coefs.MinVerifyMin {
		verify = coefs.MinVerifyMin
	}
	return &verify, ""
}

func mergeEfficiencyV2BaselineADefaults(coefs EfficiencyV2BaselineACoefficients) EfficiencyV2BaselineACoefficients {
	defaults := DefaultEfficiencyV2BaselineACoefficients()
	if coefs.Version == "" {
		coefs.Version = defaults.Version
	}
	if coefs.ThinkUserCharsMin == 0 {
		coefs.ThinkUserCharsMin = defaults.ThinkUserCharsMin
	}
	if coefs.ThinkFilesReadMin == 0 {
		coefs.ThinkFilesReadMin = defaults.ThinkFilesReadMin
	}
	if coefs.ThinkTurnMin == 0 {
		coefs.ThinkTurnMin = defaults.ThinkTurnMin
	}
	if coefs.ThinkPlanReadMin == 0 {
		coefs.ThinkPlanReadMin = defaults.ThinkPlanReadMin
	}
	if coefs.ExecLinesPerMin <= 0 {
		coefs.ExecLinesPerMin = defaults.ExecLinesPerMin
	}
	if coefs.ExecFileCoordMin == 0 {
		coefs.ExecFileCoordMin = defaults.ExecFileCoordMin
	}
	if coefs.ExecLargeDeleteRatio == 0 {
		coefs.ExecLargeDeleteRatio = defaults.ExecLargeDeleteRatio
	}
	if coefs.VerifyTestMin == 0 {
		coefs.VerifyTestMin = defaults.VerifyTestMin
	}
	if coefs.VerifyRePromptMin == 0 {
		coefs.VerifyRePromptMin = defaults.VerifyRePromptMin
	}
	if coefs.VerifyReadMin == 0 {
		coefs.VerifyReadMin = defaults.VerifyReadMin
	}
	if coefs.MinThinkMin == 0 {
		coefs.MinThinkMin = defaults.MinThinkMin
	}
	if coefs.MinExecMin == 0 {
		coefs.MinExecMin = defaults.MinExecMin
	}
	if coefs.MinVerifyMin == 0 {
		coefs.MinVerifyMin = defaults.MinVerifyMin
	}
	return coefs
}

func classifyEfficiencyV2ExecFiles(files []string) (filteredOut, kept []string) {
	for _, file := range files {
		if isEfficiencyV2LockOrGeneratedFile(file) {
			filteredOut = append(filteredOut, file)
			continue
		}
		kept = append(kept, file)
	}
	return filteredOut, kept
}

func isEfficiencyV2LockOrGeneratedFile(path string) bool {
	lower := strings.ToLower(strings.TrimSpace(path))
	if lower == "" {
		return false
	}
	lockFiles := []string{
		"package-lock.json", "yarn.lock", "pnpm-lock.yaml",
		"go.sum", "cargo.lock", "pipfile.lock", "poetry.lock",
		"composer.lock", "gemfile.lock", "mix.lock",
	}
	for _, lock := range lockFiles {
		if strings.HasSuffix(lower, "/"+lock) || lower == lock {
			return true
		}
	}
	generatedSuffixes := []string{
		".pb.go", ".pb.cc", ".pb.h", ".gen.go", ".gen.ts", ".gen.js",
		"_generated.go", "_generated.ts", "_pb.go", "_pb2.py",
		".designer.cs", ".g.cs",
	}
	for _, suffix := range generatedSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

func isEfficiencyV2FormatterOnlyByMessages(commits []models.Commit) bool {
	if len(commits) == 0 {
		return false
	}
	for _, commit := range commits {
		msg := strings.ToLower(strings.TrimSpace(commit.Comment))
		if !(strings.HasPrefix(msg, "style:") ||
			strings.HasPrefix(msg, "format:") ||
			strings.HasPrefix(msg, "fmt:") ||
			strings.HasPrefix(msg, "chore: format") ||
			strings.HasPrefix(msg, "chore: gofmt")) {
			return false
		}
	}
	return true
}

func positiveOrDefault(value, fallback float64) float64 {
	if value <= 0 {
		return fallback
	}
	return value
}

// PersistEfficiencyV2BaselineAOnNeed sets the baseline_algo_* fields on the
// Need from a computed result. It does not write to the DB.
func PersistEfficiencyV2BaselineAOnNeed(need *models.Need, result EfficiencyV2BaselineAResult) {
	need.BaselineAlgoThinkWorkMin = result.ThinkMin
	need.BaselineAlgoExecutionWorkMin = result.ExecMin
	need.BaselineAlgoVerificationWorkMin = result.VerifyMin
	need.BaselineAlgoTotalWorkMin = result.TotalMin
}
