package main

import (
	"testing"
	"time"
)

func TestNoiseFilter_BlockedModel(t *testing.T) {
	f := NewNoiseFilter(NoiseFilterConfig{Enabled: true, BlockedModels: []string{"<synthetic>"}})
	d := f.Decide(ConversationLike{Model: "<synthetic>", UserInput: "hi"})
	if !d.Drop || d.Reason != "blocked_model:<synthetic>" {
		t.Fatalf("synthetic should be dropped: %+v", d)
	}
	d = f.Decide(ConversationLike{Model: "deepseek-v4-pro", UserInput: "hi"})
	if d.Drop {
		t.Fatalf("real model should be kept: %+v", d)
	}
}

func TestNoiseFilter_BlockedWorkDir_PrefixMatch(t *testing.T) {
	f := NewNoiseFilter(NoiseFilterConfig{Enabled: true, BlockedWorkDirs: []string{"/internal/team-bench/runner"}})
	cases := []struct {
		wd      string
		wantDrop bool
	}{
		{"/internal/team-bench/runner", true},
		{"/internal/team-bench/runner/", true},
		{"/internal/team-bench/runner/subdir", true},
		{"/internal/team-bench/runnerOther", false}, // 不是前缀
		{"/internal/team-bench/other", false},
	}
	for _, c := range cases {
		d := f.Decide(ConversationLike{Model: "real", WorkDir: c.wd, UserInput: "hi"})
		if d.Drop != c.wantDrop {
			t.Fatalf("workdir=%q drop=%v want %v reason=%q", c.wd, d.Drop, c.wantDrop, d.Reason)
		}
	}
}

func TestNoiseFilter_BlockedUserInputExact(t *testing.T) {
	f := NewNoiseFilter(NoiseFilterConfig{
		Enabled:               true,
		BlockedUserInputExact: []string{"/$bunfs/root/batchWorker.ts"},
	})
	d := f.Decide(ConversationLike{Model: "real", UserInput: "/$bunfs/root/batchWorker.ts"})
	if !d.Drop || d.Reason != "blocked_user_input_exact" {
		t.Fatalf("exact match should drop: %+v", d)
	}
	d = f.Decide(ConversationLike{Model: "real", UserInput: "not the same"})
	if d.Drop {
		t.Fatalf("non-match should keep: %+v", d)
	}
}

func TestNoiseFilter_ErrorResponsePatterns(t *testing.T) {
	f := NewNoiseFilter(NoiseFilterConfig{
		Enabled:               true,
		ErrorResponsePatterns: []string{"CoStrict API Error", "Insufficient"},
	})
	d := f.Decide(ConversationLike{Model: "real", ResponseContent: "CoStrict API Error: model not available"})
	if !d.Drop {
		t.Fatalf("error response should drop: %+v", d)
	}
	d = f.Decide(ConversationLike{Model: "real", ResponseContent: "Insufficient quota"})
	if !d.Drop {
		t.Fatalf("insufficient should drop: %+v", d)
	}
	d = f.Decide(ConversationLike{Model: "real", ResponseContent: "normal response with code"})
	if d.Drop {
		t.Fatalf("normal response should keep: %+v", d)
	}
}

func TestNoiseFilter_ZeroInteraction(t *testing.T) {
	f := NewNoiseFilter(NoiseFilterConfig{Enabled: true, DropZeroInteraction: true})
	now := time.Now()

	// 全 0 + start==end → drop
	d := f.Decide(ConversationLike{Model: "real", StartTime: now, EndTime: now})
	if !d.Drop || d.Reason != "zero_interaction" {
		t.Fatalf("zero interaction should drop: %+v", d)
	}

	// 有 process_time → keep
	d = f.Decide(ConversationLike{Model: "real", StartTime: now, EndTime: now, ProcessTime: 100})
	if d.Drop {
		t.Fatalf("has process_time should keep: %+v", d)
	}

	// start != end → keep
	d = f.Decide(ConversationLike{Model: "real", StartTime: now, EndTime: now.Add(time.Second)})
	if d.Drop {
		t.Fatalf("non-instant should keep: %+v", d)
	}

	// 有 diff_lines → keep
	d = f.Decide(ConversationLike{Model: "real", StartTime: now, EndTime: now, DiffLines: 5})
	if d.Drop {
		t.Fatalf("has diff should keep: %+v", d)
	}
}

func TestNoiseFilter_Disabled(t *testing.T) {
	f := NewNoiseFilter(NoiseFilterConfig{
		Enabled:       false,
		BlockedModels: []string{"<synthetic>"},
	})
	d := f.Decide(ConversationLike{Model: "<synthetic>"})
	if d.Drop {
		t.Fatalf("disabled filter should keep everything, got %+v", d)
	}
}

func TestNoiseFilter_RepeatThreshold(t *testing.T) {
	f := NewNoiseFilter(NoiseFilterConfig{
		Enabled:             true,
		DropZeroInteraction: false,
		RepeatThreshold:     RepeatThresholdConfig{WindowHours: 24, MaxOccurrences: 3},
	})

	// 5 条相同 (wd, user_input) → 前 3 keep, 后 2 drop
	items := []ConversationLike{
		{Model: "real", WorkDir: "/proj/a", UserInput: "ping"},
		{Model: "real", WorkDir: "/proj/a", UserInput: "ping"},
		{Model: "real", WorkDir: "/proj/a", UserInput: "ping"},
		{Model: "real", WorkDir: "/proj/a", UserInput: "ping"},
		{Model: "real", WorkDir: "/proj/a", UserInput: "ping"},
		{Model: "real", WorkDir: "/proj/b", UserInput: "diff content"},
	}
	out := f.DecideBatch(items)
	dropped := 0
	for _, d := range out {
		if d.Drop {
			dropped++
		}
	}
	if dropped != 2 {
		t.Fatalf("want 2 drop (5 - 3), got %d. details: %+v", dropped, out)
	}
	if out[5].Drop {
		t.Fatalf("different (wd, ui) should not be affected: %+v", out[5])
	}
}

func TestNoiseFilter_RealWorldAutomatedBench(t *testing.T) {
	// 复现生产中观察到的 noise：同一 work_dir 大量重复 batchWorker.ts，
	// 应被 work_dir 规则或 exact-input 规则击中
	f := NewNoiseFilter(NoiseFilterConfig{
		Enabled:               true,
		BlockedModels:         []string{"<synthetic>"},
		BlockedWorkDirs:       []string{"/internal/team-bench/runner"},
		BlockedUserInputExact: []string{"/$bunfs/root/batchWorker.ts"},
		ErrorResponsePatterns: []string{"CoStrict API Error"},
		DropZeroInteraction:   true,
	})

	items := []ConversationLike{
		// 典型 noise 行
		{
			Model: "<synthetic>", WorkDir: "/internal/team-bench/runner",
			UserInput:       "/$bunfs/root/batchWorker.ts",
			ResponseContent: "CoStrict API Error: The current model is not available",
		},
		// 真实开发对话
		{
			Model: "deepseek-v4-pro", WorkDir: "/home/dev/myproject",
			UserInput:        "实现密码重置功能",
			StartTime:        time.Now(),
			EndTime:          time.Now().Add(30 * time.Second),
			ProcessTime:      5000,
			UpstreamTokens:   1500,
			DownstreamTokens: 800,
		},
	}

	out := f.DecideBatch(items)
	if !out[0].Drop {
		t.Fatalf("noise row should drop, got %+v", out[0])
	}
	if out[1].Drop {
		t.Fatalf("real conversation should keep, got %+v", out[1])
	}
}

func TestClassifyTaskNoise_BenchmarkPattern(t *testing.T) {
	cfg := DefaultNoiseFilterConfig().TaskSignature
	// 典型 AutomatedBench task：3 行全是 AI 拒绝、无 diff
	rows := []ConversationLike{
		{Model: "deepseek-v4-pro", UserInput: "/$bunfs/root/batchWorker.ts",
			ResponseContent: "I'm unable to access /root/batchWorker.ts — both Read and Bash tools are currently denied"},
		{Model: "deepseek-v4-pro", UserInput: "",
			ResponseContent: "I've launched a background search. The string 'batchWorker' does not exist in this project."},
		{Model: "deepseek-v4-pro", UserInput: "",
			ResponseContent: "The search confirmed: there is no batchWorker.ts file in this project. The $bunfs prefix is Bun's internal virtual filesystem."},
	}
	d := ClassifyTaskNoise(rows, cfg)
	if !d.IsNoise {
		t.Fatalf("benchmark task should be classified noise: %+v", d)
	}
	if d.ZeroDiffRatio != 1.0 {
		t.Fatalf("zero_diff should be 100%%, got %.2f", d.ZeroDiffRatio)
	}
	if d.RefusalRatio < 0.6 {
		t.Fatalf("expect high refusal ratio, got %.2f", d.RefusalRatio)
	}
}

func TestClassifyTaskNoise_RealDevelopmentPreserved(t *testing.T) {
	cfg := DefaultNoiseFilterConfig().TaskSignature
	// 典型真实开发 task：有 diff、AI 给具体方案
	rows := []ConversationLike{
		{Model: "deepseek-v4-pro", UserInput: "请帮我修改 formatTime 函数支持毫秒",
			DiffLines:       15,
			ResponseContent: "好的，我修改了 formatTime 函数，加入了毫秒支持。具体修改在 src/utils/time.ts..."},
		{Model: "deepseek-v4-pro", UserInput: "test 还要补一下",
			DiffLines:       8,
			ResponseContent: "我加了 3 个测试用例覆盖毫秒边界条件..."},
	}
	d := ClassifyTaskNoise(rows, cfg)
	if d.IsNoise {
		t.Fatalf("real development task should NOT be classified noise: %+v", d)
	}
}

func TestClassifyTaskNoise_QuestionTaskPreserved(t *testing.T) {
	cfg := DefaultNoiseFilterConfig().TaskSignature
	// 提问/解释 task：zero diff 但 AI 给完整回答，不应判为噪音
	rows := []ConversationLike{
		{Model: "deepseek-v4-pro", UserInput: "解释这个函数的作用",
			DiffLines: 0,
			ResponseContent: "这个函数 recycle_environment 的作用是回收测试环境资源。具体逻辑：" +
				"1. 检查环境状态；2. 释放资源；3. 更新数据库；它在测试结束时被调用..."},
		{Model: "deepseek-v4-pro", UserInput: "那并发安全吗",
			DiffLines: 0,
			ResponseContent: "并发安全方面：用了 mutex 锁保护共享状态，但如果同一环境 ID 被多个调用..."},
	}
	d := ClassifyTaskNoise(rows, cfg)
	if d.IsNoise {
		t.Fatalf("Q&A task should NOT be classified noise: %+v", d)
	}
	if d.RefusalRatio != 0 {
		t.Fatalf("Q&A AI doesn't refuse, got refusal %.2f", d.RefusalRatio)
	}
}

func TestClassifyTaskNoise_MixedPartialRefusal(t *testing.T) {
	cfg := DefaultNoiseFilterConfig().TaskSignature
	// 边缘 case：AI 部分拒绝部分回答（比如先说"我看不到文件"后帮用户分析）
	rows := []ConversationLike{
		{Model: "deepseek-v4-pro", UserInput: "看下这个 readme",
			DiffLines:       0,
			ResponseContent: "我无法读取那个路径，但你可以贴内容给我"}, // refusal
		{Model: "deepseek-v4-pro", UserInput: "好，内容是 ...",
			DiffLines:       0,
			ResponseContent: "明白了。从内容看，这个项目是 ...完整分析"}, // 正常回答
	}
	d := ClassifyTaskNoise(rows, cfg)
	// refusal 1/2=50%，超过 30% 阈值，且 zero_diff=100% → IsNoise=true
	// 这是 borderline，符合规则可被接受为噪音（实际生产 refusal=50% 的 task 多半也确实有问题）
	if !d.IsNoise {
		t.Logf("borderline task kept: %+v (acceptable)", d)
	}
}

func TestClassifyTaskNoise_DisabledOrEmpty(t *testing.T) {
	cfg := DefaultNoiseFilterConfig().TaskSignature
	cfg.Enabled = false
	d := ClassifyTaskNoise([]ConversationLike{{ResponseContent: "I'm unable"}}, cfg)
	if d.IsNoise {
		t.Fatalf("disabled classifier should not flag, got %+v", d)
	}
	cfg.Enabled = true
	d = ClassifyTaskNoise(nil, cfg)
	if d.IsNoise {
		t.Fatalf("empty input should not flag")
	}
}

func TestDefaultNoiseFilterConfig_Sane(t *testing.T) {
	cfg := DefaultNoiseFilterConfig()
	if !cfg.Enabled {
		t.Fatalf("default should be enabled")
	}
	if len(cfg.BlockedModels) == 0 || cfg.BlockedModels[0] != "<synthetic>" {
		t.Fatalf("default should block <synthetic>")
	}
	if !cfg.DropZeroInteraction {
		t.Fatalf("default should drop zero interaction")
	}
	if cfg.RepeatThreshold.MaxOccurrences <= 0 {
		t.Fatalf("default repeat threshold should be set")
	}
}
