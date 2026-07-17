package main

import (
	"kanban/kbcli/internal/appconfig"
	"kanban/kbcli/internal/estimator"
	"strings"
	"testing"
	"time"
)

func TestSilicaObjectRelativePath(t *testing.T) {
	got := silicaObjectRelativePath("a7b1d72b-0842-485a-b93d-5d7e3f2e3ea1")
	want := "task/conversation/a7b1d72b-0842-485a-b93d-5d7e3f2e3ea1.silica.json"
	if got != want {
		t.Fatalf("relative silica path = %q, want %q", got, want)
	}
}

func TestIndexedTaskSilicaLocationS3Prefix(t *testing.T) {
	base := "s3://chat-rag/efficiency-dashboard/analysed"
	rel := "task/conversation/a7b1.silica.json"
	got, err := indexedTaskSilicaLocation(base, rel)
	if err != nil {
		t.Fatal(err)
	}
	want := "s3://chat-rag/efficiency-dashboard/analysed/task/conversation/a7b1.silica.json"
	if got != want {
		t.Fatalf("indexed silica location = %q, want %q", got, want)
	}
}

func TestIndexedTaskSilicaLocationRejectsNonRelativePaths(t *testing.T) {
	base := "s3://chat-rag/efficiency-dashboard/analysed"
	invalid := []string{
		"s3://chat-rag/efficiency-dashboard/analysed/task/conversation/a.silica.json",
		"/task/conversation/a.silica.json",
		"../task/conversation/a.silica.json",
		"task/content/a.json",
	}
	for _, objectPath := range invalid {
		if _, err := indexedTaskSilicaLocation(base, objectPath); err == nil {
			t.Errorf("expected invalid object_path %q to fail", objectPath)
		} else if !strings.Contains(err.Error(), "object_path") {
			t.Errorf("error for %q should identify object_path: %v", objectPath, err)
		}
	}
}

// TestLookupGroups 覆盖主分组 / work_dir_id 后备分组 / 合并 / 全不命中 四种情形。
func TestLookupGroups(t *testing.T) {
	mainGI := newGroupIndexer()
	mainGI.Lines["h1"] = &convMeta{sessionId: "s1"}
	fbGI := newGroupIndexer()
	fbGI.Lines["h2"] = &convMeta{sessionId: "s2"}

	idx := &conversationsIndexer{
		groups:   map[groupKey]groupIndexer{{repoAddr: "r", userID: "u"}: mainGI},
		wdGroups: map[wdKey]groupIndexer{{workDirId: "wd", userID: "u"}: fbGI},
	}

	// 仅主分组
	if gi, ok := idx.lookupGroups("r", "u", ""); !ok || gi.Lines["h1"] == nil {
		t.Error("主分组应命中")
	}
	// 仅后备分组（repo 对不上，靠 work_dir_id+userID）
	if gi, ok := idx.lookupGroups("nope", "u", "wd"); !ok || gi.Lines["h2"] == nil {
		t.Error("work_dir_id 后备分组应命中")
	}
	// 后备分组 userID 不匹配则不命中（防串户）
	if _, ok := idx.lookupGroups("nope", "other-user", "wd"); ok {
		t.Error("work_dir_id 相同但 userID 不同不应命中")
	}
	// 合并：同时含主与后备指纹
	if gi, ok := idx.lookupGroups("r", "u", "wd"); !ok || gi.Lines["h1"] == nil || gi.Lines["h2"] == nil {
		t.Error("合并分组应同时含主与后备指纹")
	}
	// 全不命中
	if _, ok := idx.lookupGroups("x", "y", "z"); ok {
		t.Error("无任何分组应返回 false")
	}
}

// TestLookupGroups_MainWinsOnConflict 合并时同一指纹归属以主分组为准。
func TestLookupGroups_MainWinsOnConflict(t *testing.T) {
	mainGI := newGroupIndexer()
	mainGI.Lines["dup"] = &convMeta{sessionId: "main"}
	fbGI := newGroupIndexer()
	fbGI.Lines["dup"] = &convMeta{sessionId: "fallback"}

	idx := &conversationsIndexer{
		groups:   map[groupKey]groupIndexer{{repoAddr: "r", userID: "u"}: mainGI},
		wdGroups: map[wdKey]groupIndexer{{workDirId: "wd", userID: "u"}: fbGI},
	}
	gi, ok := idx.lookupGroups("r", "u", "wd")
	if !ok || gi.Lines["dup"].sessionId != "main" {
		t.Errorf("指纹冲突时应保留主分组归属, 得到 %v", gi.Lines["dup"])
	}
}

// TestAnalyzeCommitSilica_WorkDirFallback 验证缺 repo_addr 的对话能经 work_dir_id 后备分组匹配上 commit。
func TestAnalyzeCommitSilica_WorkDirFallback(t *testing.T) {
	// 全局 Cfg 默认是 nil 指针，先初始化并赋最小合理估算值（本测试不校验耗时，仅防 NaN/Inf）
	appconfig.Cfg = &appconfig.Config{}
	appconfig.Cfg.AlgoEstimation = estimator.EstimateConfig{
		MaxInputChars: 1000, MinFactor: 1, MaxFactor: 2, LinesPerMinutes: 10, MaxRatio: 3, MinMinutes: 1,
	}
	appconfig.Cfg.TaskStatistics.GapThresholdMinutes = 30
	appconfig.Cfg.TaskStatistics.ExtensionMinutes = 5

	commitTime := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	convEnd := commitTime.Add(-1 * time.Hour) // 时间窗内

	cm := &convMeta{
		sessionId: "sess1",
		requestId: "req1",
		DiffLines: 2,
		startTime: convEnd.Add(-10 * time.Minute),
		endTime:   convEnd,
	}
	session := &sessionMeta{sessionId: "sess1", conversations: []*convMeta{cm}}
	gi := newGroupIndexer()
	gi.Lines["fpA"] = cm
	gi.Lines["fpB"] = cm
	gi.Sessions["sess1"] = session

	idx := &conversationsIndexer{
		groups:   map[groupKey]groupIndexer{}, // 主分组空：模拟 commit 的 repo_addr 对不上
		wdGroups: map[wdKey]groupIndexer{{workDirId: "wd-1", userID: "user-none"}: gi},
	}

	// commit 总 3 行，其中 fpA/fpB 命中（2 行）；repo 对不上，靠 work_dir_id=wd-1 后备命中
	fpHashes := []string{"fpA", "fpB", "fpC"}
	p, tms := analyzeCommitSilica("c1", "repo-none", "user-none", "wd-1", commitTime, fpHashes, len(fpHashes), idx, 7)

	if p.totalSilica == 0 {
		t.Fatal("work_dir_id 后备分组应让缺 repo 对话匹配上，silica 不应为 0")
	}
	if len(tms) != 1 {
		t.Fatalf("应匹配出 1 个 task，得到 %d", len(tms))
	}
	// silica = 匹配行/commit总行 = 2/3 ≈ 0.667
	if tms[0].silica < 0.6 || tms[0].silica > 0.7 {
		t.Errorf("silica 期望约 0.667，得到 %.3f", tms[0].silica)
	}
}

// TestAnalyzeCommitSilica_NoGroup 无任何匹配分组时 silica=0、无 task。
func TestAnalyzeCommitSilica_NoGroup(t *testing.T) {
	idx := &conversationsIndexer{
		groups:   map[groupKey]groupIndexer{},
		wdGroups: map[wdKey]groupIndexer{},
	}
	commitTime := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	p, tms := analyzeCommitSilica("c1", "r", "u", "wd", commitTime, []string{"x", "y"}, 2, idx, 7)
	if p.totalSilica != 0 || len(tms) != 0 {
		t.Errorf("无分组应 silica=0 且无 task，得到 silica=%.3f tasks=%d", p.totalSilica, len(tms))
	}
}
