package main

import (
	"fmt"
	"strings"
	"time"
)

// NoiseFilterConfig 控制 conversation 噪音过滤。规则用于清洗下游导入的
// 自动化测评/客户端占位/API 错误占位等无信息行。
//
// 三层规则按优先级递进：
//  1. 单条 conversation 规则（model / work_dir / user_input / error_response / zero_interaction）
//  2. 重复阈值（同 work_dir + 同 user_input 高频重复 → 自动化探针）
//  3. **Task signature 分类**（无需配 work_dir，靠 task 内多条行为特征自动识别 bench 噪音）
//
// 推荐：尽量用 task_signature，少用 blocked_work_dirs（避免误伤同目录的真实开发）。
type NoiseFilterConfig struct {
	Enabled               bool                  `yaml:"enabled"`
	BlockedModels         []string              `yaml:"blocked_models"`
	ErrorResponsePatterns []string              `yaml:"error_response_patterns"`
	BlockedWorkDirs       []string              `yaml:"blocked_work_dirs"`
	BlockedUserInputExact []string              `yaml:"blocked_user_input_exact"`
	DropZeroInteraction   bool                  `yaml:"drop_zero_interaction"`
	RepeatThreshold       RepeatThresholdConfig `yaml:"repeat_threshold"`
	TaskSignature         TaskSignatureConfig   `yaml:"task_signature"`
}

type RepeatThresholdConfig struct {
	WindowHours    int `yaml:"window_hours"`
	MaxOccurrences int `yaml:"max_occurrences"`
}

// TaskSignatureConfig 控制 task-level 自动识别。
// 一个 task 同时满足「零代码产出」+ (「AI 大量被拒/澄清」 OR 「AI 几乎没说话」) 时被判为 bench 噪音。
type TaskSignatureConfig struct {
	Enabled                   bool     `yaml:"enabled"`
	RefusalKeywords           []string `yaml:"refusal_keywords"`
	RefusalRatioAbove         float64  `yaml:"refusal_ratio_above"`          // refusal 命中比例 > 此值
	UselessResponseRatioAbove float64  `yaml:"useless_response_ratio_above"` // 空/极短 response 比例 > 此值
	UselessResponseMaxChars   int      `yaml:"useless_response_max_chars"`   // response 短于此字符数算 "无信息"
	RequireZeroDiff           bool     `yaml:"require_zero_diff"`            // 还要求 100% 零 diff
	MinConversationRows       int      `yaml:"min_conversation_rows"`
}

// NoiseFilterDecision 单条 conversation 的过滤判定。Drop=true 时 Reason 给出原因。
type NoiseFilterDecision struct {
	Drop   bool
	Reason string
}

// ConversationLike 是过滤器关心的字段子集，足以让 import-conv 流程、
// 离线清洗脚本、单元测试三方共用。
type ConversationLike struct {
	Model            string
	UserInput        string
	ResponseContent  string
	WorkDir          string
	StartTime        time.Time
	EndTime          time.Time
	ProcessTime      int64 // ms
	UpstreamTokens   int64
	DownstreamTokens int64
	DiffLines        int64
}

// NoiseFilter 应用配置中的规则。第三方调用 Decide() 拿单条判定，
// 对一批 conversation 用 DecideBatch 可同时统计重复模式。
type NoiseFilter struct {
	cfg NoiseFilterConfig

	// 预编译的查找集合（小写）
	blockedModels         map[string]struct{}
	blockedWorkDirs       map[string]struct{}
	blockedUserInputExact map[string]struct{}
	errorPatterns         []string // 已 lower
}

func NewNoiseFilter(cfg NoiseFilterConfig) *NoiseFilter {
	f := &NoiseFilter{
		cfg:                   cfg,
		blockedModels:         map[string]struct{}{},
		blockedWorkDirs:       map[string]struct{}{},
		blockedUserInputExact: map[string]struct{}{},
	}
	for _, m := range cfg.BlockedModels {
		f.blockedModels[strings.ToLower(strings.TrimSpace(m))] = struct{}{}
	}
	for _, w := range cfg.BlockedWorkDirs {
		w = strings.TrimRight(strings.TrimSpace(w), "/")
		if w != "" {
			f.blockedWorkDirs[w] = struct{}{}
		}
	}
	for _, u := range cfg.BlockedUserInputExact {
		f.blockedUserInputExact[strings.TrimSpace(u)] = struct{}{}
	}
	for _, p := range cfg.ErrorResponsePatterns {
		if p = strings.ToLower(strings.TrimSpace(p)); p != "" {
			f.errorPatterns = append(f.errorPatterns, p)
		}
	}
	return f
}

// Decide 判定单条 conversation 是否应丢弃。规则按优先级短路：
//
//	R1 blocked_models     → drop
//	R2 blocked_work_dir   → drop（前缀匹配）
//	R3 blocked_user_input → drop（完全匹配）
//	R4 error_response     → drop（response_content 含关键字）
//	R5 zero_interaction   → drop（process_time / tokens / diff 全 0 且 start==end）
//	否则                  → keep
func (f *NoiseFilter) Decide(c ConversationLike) NoiseFilterDecision {
	if !f.cfg.Enabled {
		return NoiseFilterDecision{Drop: false}
	}

	model := strings.ToLower(strings.TrimSpace(c.Model))
	if model != "" {
		if _, ok := f.blockedModels[model]; ok {
			return NoiseFilterDecision{Drop: true, Reason: "blocked_model:" + model}
		}
	}

	wd := strings.TrimRight(strings.TrimSpace(c.WorkDir), "/")
	for prefix := range f.blockedWorkDirs {
		if wd == prefix || strings.HasPrefix(wd, prefix+"/") {
			return NoiseFilterDecision{Drop: true, Reason: "blocked_work_dir:" + prefix}
		}
	}

	user := strings.TrimSpace(c.UserInput)
	if _, ok := f.blockedUserInputExact[user]; ok {
		return NoiseFilterDecision{Drop: true, Reason: "blocked_user_input_exact"}
	}

	resp := strings.ToLower(c.ResponseContent)
	for _, p := range f.errorPatterns {
		if strings.Contains(resp, p) {
			return NoiseFilterDecision{Drop: true, Reason: "error_response_pattern:" + p}
		}
	}

	if f.cfg.DropZeroInteraction && isZeroInteraction(c) {
		return NoiseFilterDecision{Drop: true, Reason: "zero_interaction"}
	}

	return NoiseFilterDecision{Drop: false}
}

// DecideBatch 在一批 conversation 上跑规则，并额外应用 repeat_threshold：
// 同一 (work_dir, user_input) 在 window_hours 内出现次数超过 max_occurrences 时，
// 后续重复行全部 drop（典型自动化探针特征）。
//
// 返回 decisions[i] 跟 input[i] 一一对应。
func (f *NoiseFilter) DecideBatch(items []ConversationLike) []NoiseFilterDecision {
	out := make([]NoiseFilterDecision, len(items))
	for i, c := range items {
		out[i] = f.Decide(c)
	}
	if !f.cfg.Enabled || f.cfg.RepeatThreshold.MaxOccurrences <= 0 {
		return out
	}

	window := time.Duration(f.cfg.RepeatThreshold.WindowHours) * time.Hour
	if window <= 0 {
		window = 24 * time.Hour
	}
	maxOcc := f.cfg.RepeatThreshold.MaxOccurrences

	// 按 (work_dir, user_input) group 收集 timestamps
	type key struct{ wd, ui string }
	groups := map[key][]int{}
	for i, c := range items {
		if out[i].Drop {
			continue // 已被前面的规则 drop，跳过
		}
		k := key{wd: strings.TrimSpace(c.WorkDir), ui: strings.TrimSpace(c.UserInput)}
		if k.ui == "" {
			continue
		}
		groups[k] = append(groups[k], i)
	}

	for k, idxs := range groups {
		if len(idxs) <= maxOcc {
			continue
		}
		// 简化：超过阈值就把多出来的全部 drop（不滑窗，避免复杂度）
		// 更精确的滑窗实现可以后续加，目前生产数据自动化探针都是密集出现，不滑窗也够
		reason := fmt.Sprintf("repeat_threshold:%d>%d in window=%dh wd=%s",
			len(idxs), maxOcc, f.cfg.RepeatThreshold.WindowHours, k.wd)
		for _, i := range idxs[maxOcc:] {
			out[i] = NoiseFilterDecision{Drop: true, Reason: reason}
		}
	}
	return out
}

func isZeroInteraction(c ConversationLike) bool {
	if c.ProcessTime != 0 || c.UpstreamTokens != 0 || c.DownstreamTokens != 0 || c.DiffLines != 0 {
		return false
	}
	if !c.StartTime.IsZero() && !c.EndTime.IsZero() && !c.StartTime.Equal(c.EndTime) {
		return false
	}
	return true
}

// DefaultNoiseFilterConfig 给一组安全的默认规则：业内常见自动化/占位 noise。
// 适合作为 kbcli-config.yaml 的 starter，操作员后续按数据加规则。
func DefaultNoiseFilterConfig() NoiseFilterConfig {
	return NoiseFilterConfig{
		Enabled:       true,
		BlockedModels: []string{"<synthetic>"},
		ErrorResponsePatterns: []string{
			"CoStrict API Error",
			"API Error: 4",
			"Connection error",
			"Insufficient",
			"is not available",
			"does not support image",
			"身份验证失败",
		},
		BlockedWorkDirs:       []string{}, // 不建议用，优先 task_signature
		BlockedUserInputExact: []string{},
		DropZeroInteraction:   true,
		RepeatThreshold: RepeatThresholdConfig{
			WindowHours:    24,
			MaxOccurrences: 20,
		},
		TaskSignature: TaskSignatureConfig{
			Enabled: true,
			RefusalKeywords: []string{
				"unable to access",
				"i'm unable",
				"i am unable",
				"cannot read",
				"does not exist",
				"is not available",
				"could you help me understand",
				"denied",
				"no such file",
				"the current model is not available",
			},
			RefusalRatioAbove:         0.3, // > 30% AI 在拒绝
			UselessResponseRatioAbove: 0.5, // > 50% AI 几乎没回话（空 / 极短）
			UselessResponseMaxChars:   20,
			RequireZeroDiff:           true,
			MinConversationRows:       1,
		},
	}
}

// TaskNoiseDecision 一个 task 的整体判定。
type TaskNoiseDecision struct {
	IsNoise              bool
	Reason               string
	ZeroDiffRatio        float64
	RefusalRatio         float64
	UselessResponseRatio float64
	UniformInputs        bool
	RowCount             int
}

// ClassifyTaskNoise 对一个 task 的所有 conversation 做行为签名判定。
// 返回 IsNoise=true 时，调用方应丢弃整个 task（包括它的 session）。
//
// 算法：
//
//	noise = (RequireZeroDiff ? 100% 零 diff : true)
//	        AND (refusal_ratio > RefusalRatioAbove
//	             OR useless_response_ratio > UselessResponseRatioAbove)
//
// 真实开发的 task 即使 zero_diff（提问/解释类）也几乎不会触发：
//   - 不会大量 AI 道歉（refusal_ratio 低）
//   - 不会大量 AI 空回（useless_ratio 低，问答 AI 会给完整解释）
func ClassifyTaskNoise(rows []ConversationLike, cfg TaskSignatureConfig) TaskNoiseDecision {
	d := TaskNoiseDecision{RowCount: len(rows)}
	if !cfg.Enabled || len(rows) < max1(cfg.MinConversationRows) {
		return d
	}

	zeroDiff := 0
	refusal := 0
	useless := 0
	maxUseless := cfg.UselessResponseMaxChars
	if maxUseless <= 0 {
		maxUseless = 20
	}
	inputs := map[string]struct{}{}
	for _, r := range rows {
		if r.DiffLines == 0 {
			zeroDiff++
		}
		resp := strings.ToLower(r.ResponseContent)
		trimmedLen := len(strings.TrimSpace(r.ResponseContent))
		if trimmedLen < maxUseless {
			useless++
		}
		for _, kw := range cfg.RefusalKeywords {
			if kw != "" && strings.Contains(resp, strings.ToLower(kw)) {
				refusal++
				break
			}
		}
		inputs[strings.TrimSpace(r.UserInput)] = struct{}{}
	}
	d.ZeroDiffRatio = float64(zeroDiff) / float64(len(rows))
	d.RefusalRatio = float64(refusal) / float64(len(rows))
	d.UselessResponseRatio = float64(useless) / float64(len(rows))
	d.UniformInputs = len(inputs) <= 2

	if cfg.RequireZeroDiff && d.ZeroDiffRatio < 1.0 {
		return d
	}
	if d.RefusalRatio > cfg.RefusalRatioAbove {
		d.IsNoise = true
		d.Reason = fmt.Sprintf("task_sig: refusal=%.0f%% zero_diff=%.0f%%",
			d.RefusalRatio*100, d.ZeroDiffRatio*100)
		return d
	}
	if cfg.UselessResponseRatioAbove > 0 && d.UselessResponseRatio > cfg.UselessResponseRatioAbove {
		d.IsNoise = true
		d.Reason = fmt.Sprintf("task_sig: empty_resp=%.0f%% zero_diff=%.0f%%",
			d.UselessResponseRatio*100, d.ZeroDiffRatio*100)
	}
	return d
}

func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}
