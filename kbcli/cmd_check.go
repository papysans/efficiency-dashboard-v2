package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"kanban/core/rawdump"
	"kanban/core/storage"
	"kanban/kbcli/internal/appconfig"
	"kanban/kbcli/internal/logx"
	"kanban/kbcli/internal/util"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type CheckIssue struct {
	Severity string `json:"severity"`
	Path     string `json:"path"`
	TaskId   string `json:"task_id"`
	CommitId string `json:"commit_id"`
	Issue    string `json:"issue"`
	Field    string `json:"field"`
	Comment  string `json:"comment"`
}

type checkContext struct {
	taskDir     string
	repoDir     string
	dateFilter  string
	minLevel    string
	ignoreSet   map[string]bool // issue类型 -> 是否忽略
	modelPrices map[string]appconfig.ModelPrice
	issues      []CheckIssue
	summaryMap  map[string]string                  // taskID -> summary path
	convMap     map[string]rawdump.ConversationRef // taskID -> conversation 来源(单文件或分片)
	repoFileMap map[string]string                  // commitID -> repo file path
}

func severityRank(s string) int {
	switch s {
	case "error":
		return 3
	case "warn":
		return 2
	case "info":
		return 1
	default:
		return 0
	}
}

func (ctx *checkContext) meetsMinLevel(severity string) bool {
	return severityRank(severity) >= severityRank(ctx.minLevel)
}

func (ctx *checkContext) addIssue(severity, path, taskID, commitID, issue, field, comment string) {
	if ctx.ignoreSet[issue] {
		return
	}
	if !ctx.meetsMinLevel(severity) {
		return
	}
	ctx.issues = append(ctx.issues, CheckIssue{
		Severity: severity,
		Path:     path,
		TaskId:   taskID,
		CommitId: commitID,
		Issue:    issue,
		Field:    field,
		Comment:  comment,
	})
}

func runCheck(taskDir, repoDir, dateFilter, output, minLevel string, ignoreList []string) error {
	ignoreSet := make(map[string]bool)
	for _, i := range ignoreList {
		if i != "" {
			ignoreSet[i] = true
		}
	}

	ctx := &checkContext{
		taskDir:     taskDir,
		repoDir:     repoDir,
		dateFilter:  dateFilter,
		minLevel:    minLevel,
		ignoreSet:   ignoreSet,
		modelPrices: appconfig.Cfg.ModelPrices,
		issues:      make([]CheckIssue, 0),
		summaryMap:  make(map[string]string),
		convMap:     make(map[string]rawdump.ConversationRef),
		repoFileMap: make(map[string]string),
	}

	logx.Info("===== 开始数据质量检查 =====")

	if err := ctx.scanSummaries(); err != nil {
		return fmt.Errorf("扫描summary目录失败: %w", err)
	}
	if err := ctx.scanConversations(); err != nil {
		return fmt.Errorf("扫描conversation目录失败: %w", err)
	}
	if err := ctx.scanRepos(); err != nil {
		return fmt.Errorf("扫描repo目录失败: %w", err)
	}

	logx.Infof("扫描完成: %d 个summary, %d 个conversation, %d 个repo commit",
		len(ctx.summaryMap), len(ctx.convMap), len(ctx.repoFileMap))

	if err := ctx.checkSummaries(); err != nil {
		return fmt.Errorf("检查summary文件失败: %w", err)
	}
	if err := ctx.checkConversations(); err != nil {
		return fmt.Errorf("检查conversation文件失败: %w", err)
	}
	if err := ctx.checkRepos(); err != nil {
		return fmt.Errorf("检查repo文件失败: %w", err)
	}
	if err := ctx.checkCrossReferences(); err != nil {
		return fmt.Errorf("检查交叉引用失败: %w", err)
	}

	if err := ctx.writeReport(output); err != nil {
		return fmt.Errorf("写入报告失败: %w", err)
	}
	logx.Infof("报告已输出: %s", output)

	ctx.printSummary()

	return nil
}

func (ctx *checkContext) printSummary() {
	totalIssues := len(ctx.issues)
	if totalIssues == 0 {
		logx.Prompt("===== 检查完成 =====")
		logx.Prompt("未发现问题")
		return
	}

	// 按级别统计
	severityCounts := map[string]int{"error": 0, "warn": 0, "info": 0}
	for _, issue := range ctx.issues {
		if _, ok := severityCounts[issue.Severity]; ok {
			severityCounts[issue.Severity]++
		}
	}

	// 按问题类型统计
	typeCounts := make(map[string]int)
	for _, issue := range ctx.issues {
		typeCounts[issue.Issue]++
	}

	// 受影响文件统计
	summaryAffected := make(map[string]bool)
	convAffected := make(map[string]bool)
	repoAffected := make(map[string]bool)
	for _, issue := range ctx.issues {
		path := issue.Path
		if strings.Contains(path, "/summary/") || strings.Contains(path, "\\summary\\") {
			summaryAffected[path] = true
		} else if strings.Contains(path, "/conversation/") || strings.Contains(path, "\\conversation\\") {
			convAffected[path] = true
		} else {
			repoAffected[path] = true
		}
	}

	totalScanned := len(ctx.summaryMap) + len(ctx.convMap) + len(ctx.repoFileMap)

	logx.Prompt("===== 检查完成 =====")
	logx.Promptf("扫描文件总计: %d (summary: %d, conversation: %d, repo: %d)",
		totalScanned, len(ctx.summaryMap), len(ctx.convMap), len(ctx.repoFileMap))
	logx.Promptf("发现问题总计: %d", totalIssues)

	logx.Prompt("  按级别:")
	for _, sev := range []string{"error", "warn", "info"} {
		cnt := severityCounts[sev]
		if cnt > 0 {
			logx.Promptf("    %s: %d (%.1f%%)", sev, cnt, float64(cnt)*100.0/float64(totalIssues))
		}
	}

	logx.Prompt("  按类型:")
	// 按数量降序排列类型
	var typeNames []string
	for name := range typeCounts {
		typeNames = append(typeNames, name)
	}
	for i := 0; i < len(typeNames); i++ {
		for j := i + 1; j < len(typeNames); j++ {
			if typeCounts[typeNames[i]] < typeCounts[typeNames[j]] {
				typeNames[i], typeNames[j] = typeNames[j], typeNames[i]
			}
		}
	}
	for _, name := range typeNames {
		cnt := typeCounts[name]
		logx.Promptf("    %s: %d (%.1f%%)", name, cnt, float64(cnt)*100.0/float64(totalIssues))
	}

	logx.Prompt("  受影响文件:")
	if len(ctx.summaryMap) > 0 {
		cnt := len(summaryAffected)
		logx.Promptf("    summary: %d / %d (%.1f%%)", cnt, len(ctx.summaryMap), float64(cnt)*100.0/float64(len(ctx.summaryMap)))
	}
	if len(ctx.convMap) > 0 {
		cnt := len(convAffected)
		logx.Promptf("    conversation: %d / %d (%.1f%%)", cnt, len(ctx.convMap), float64(cnt)*100.0/float64(len(ctx.convMap)))
	}
	if len(ctx.repoFileMap) > 0 {
		cnt := len(repoAffected)
		logx.Promptf("    repo: %d / %d (%.1f%%)", cnt, len(ctx.repoFileMap), float64(cnt)*100.0/float64(len(ctx.repoFileMap)))
	}
}

func matchDateFilter(path, dateFilter string) bool {
	if dateFilter == "" {
		return true
	}
	// 路径格式: .../summary/YYYY/MM/DD/... 或 ...\summary\YYYY\MM\DD\...
	// 显式把反斜杠统一成斜杠（filepath.ToSlash 在 Unix 上是 no-op，无法处理 Windows 风格
	// 路径；这里数据可能来自 Windows 采集，故跨平台都要规范化）。再查找 /YYYY/MM/DD/ 模式。
	normalized := strings.ReplaceAll(path, "\\", "/")
	formattedDate := dateFilter[:4] + "/" + dateFilter[4:6] + "/" + dateFilter[6:]
	return strings.Contains(normalized, "/"+formattedDate+"/")
}

func (ctx *checkContext) scanSummaries() error {
	summaryDir := storage.Join(ctx.taskDir, "summary")
	if ok, err := storage.Exists(summaryDir); err != nil || !ok {
		return err
	}
	return storage.Walk(summaryDir, func(path string, info storage.FileInfo) error {
		if !strings.HasSuffix(info.Name, ".json") {
			return nil
		}
		if !matchDateFilter(path, ctx.dateFilter) {
			return nil
		}
		taskID := strings.TrimSuffix(info.Name, ".json")
		ctx.summaryMap[taskID] = path
		return nil
	})
}

func (ctx *checkContext) scanConversations() error {
	convDir := storage.Join(ctx.taskDir, "conversation")
	if ok, err := storage.Exists(convDir); err != nil || !ok {
		return err
	}
	// 按 sessionId 聚合分片：新分片布局下文件名是 00000N.jsonl，必须按父目录(sessionId)归组，
	// 否则每个分片会被当成独立 conversation，巡检将满屏误报 orphan/missing-conversation。
	groups := make(map[string][]string)
	err := storage.Walk(convDir, func(path string, info storage.FileInfo) error {
		if !strings.HasSuffix(info.Name, ".jsonl") {
			return nil
		}
		if !matchDateFilter(path, ctx.dateFilter) {
			return nil
		}
		relPath, err := storage.Rel(convDir, path)
		if err != nil {
			return err
		}
		sessionID, _, _, ok := rawdump.ClassifyRelPath(relPath)
		if !ok {
			return nil
		}
		groups[sessionID] = append(groups[sessionID], path)
		return nil
	})
	if err != nil {
		return err
	}
	for sessionID, paths := range groups {
		rawdump.SortChunkPaths(paths)
		ctx.convMap[sessionID] = rawdump.ConversationRef{SessionID: sessionID, Paths: paths}
	}
	return nil
}

func (ctx *checkContext) scanRepos() error {
	if ok, err := storage.Exists(ctx.repoDir); err != nil || !ok {
		return err
	}
	return storage.Walk(ctx.repoDir, func(path string, info storage.FileInfo) error {
		if !strings.HasSuffix(info.Name, ".json") {
			return nil
		}
		if !matchDateFilter(path, ctx.dateFilter) {
			return nil
		}
		commitID := strings.TrimSuffix(info.Name, ".json")
		ctx.repoFileMap[commitID] = path
		return nil
	})
}

func (ctx *checkContext) checkSummaries() error {
	cnt := 0
	for taskID, path := range ctx.summaryMap {
		cnt++
		logx.PromptProgress(cnt, 50)

		data, err := storage.ReadFile(path)
		if err != nil {
			ctx.addIssue("error", path, taskID, "", "read-failed", "", fmt.Sprintf("读取文件失败: %v", err))
			continue
		}

		var summary taskSession
		if err := json.Unmarshal(data, &summary); err != nil {
			ctx.addIssue("error", path, taskID, "", "json-parse-failed", "", fmt.Sprintf("JSON解析失败: %v", err))
			continue
		}

		if summary.SessionId == "" {
			ctx.addIssue("error", path, taskID, "", "missing-session-id", "session_id", "session_id为空，该数据无法被索引和关联")
		} else if summary.SessionId != taskID {
			ctx.addIssue("warn", path, taskID, "", "session-id-mismatch", "session_id",
				fmt.Sprintf("文件内session_id(%s)与文件名(%s)不一致，可能导致数据关联混乱", summary.SessionId, taskID))
		}
		if summary.UserId == "" {
			ctx.addIssue("warn", path, taskID, "", "missing-user-id", "user_id", "user_id为空，将导致该任务无法按用户聚合统计")
		}
		if summary.UserName == "" {
			ctx.addIssue("info", path, taskID, "", "missing-user-name", "user_name", "user_name为空，用户显示名可能缺失")
		}
		if summary.RepoAddr == "" {
			ctx.addIssue("info", path, taskID, "", "missing-repo-addr", "repo_addr", "repo_addr为空，conversation将无法继承该值参与silica含硅量计算")
		}
		if summary.ClientId == "" {
			ctx.addIssue("info", path, taskID, "", "missing-client-id", "client_id", "client_id为空，work_dir_id生成将受影响")
		}
		if summary.WorkDir == "" {
			ctx.addIssue("info", path, taskID, "", "missing-work-dir", "work_dir", "work_dir为空，work_dir_id生成将受影响")
		}
	}
	return nil
}

func (ctx *checkContext) checkConversations() error {
	cnt := 0
	for taskID, ref := range ctx.convMap {
		cnt++
		logx.PromptProgress(cnt, 50)

		// path 取首片用于问题定位；data 为按序重组后的完整会话（兼容单文件/分片）
		path := ""
		if len(ref.Paths) > 0 {
			path = ref.Paths[0]
		}
		data, err := ref.Read()
		if err != nil {
			ctx.addIssue("error", path, taskID, "", "read-failed", "", fmt.Sprintf("读取文件失败: %v", err))
			continue
		}

		// 分片缺号检测：序号有洞=会话可能不完整（多为上游写入/列举问题）
		if missing := ref.MissingChunkNumbers(); len(missing) > 0 {
			ctx.addIssue("warn", path, taskID, "", "chunk-gap", "",
				fmt.Sprintf("分片序号缺号%v，会话可能不完整(现存%d片)", missing, ref.ChunkCount()))
		}

		// 检查对应summary是否存在
		if _, ok := ctx.summaryMap[taskID]; !ok {
			ctx.addIssue("error", path, taskID, "", "orphan-conversation", "",
				fmt.Sprintf("该conversation文件没有对应的summary文件(task_id=%s)，数据无法被导入", taskID))
		}

		convs, err := parseConversationLines(data, path, ctx)
		if err != nil {
			ctx.addIssue("error", path, taskID, "", "parse-failed", "", fmt.Sprintf("解析失败: %v", err))
			continue
		}

		var totalDiffLines int64
		var totalActualDiffLines int64
		var totalUpstream, totalDownstream int64
		var validStartCount, totalConvWithDiff int
		var anyDiffContent bool
		lineNum := 0
		for _, conv := range convs {
			lineNum++
			if conv.RequestId == "" {
				ctx.addIssue("warn", path, taskID, "", "missing-request-id", "request_id",
					fmt.Sprintf("第%d行request_id为空，该对话无法被去重和追踪", lineNum))
			}
			if conv.StartTime != "" {
				if _, err := time.Parse(time.RFC3339, conv.StartTime); err != nil {
					ctx.addIssue("warn", path, taskID, "", "invalid-start-time", "start_time",
						fmt.Sprintf("第%d行start_time格式错误(%s)，将影响task时间计算", lineNum, conv.StartTime))
				} else {
					validStartCount++
				}
			} else {
				ctx.addIssue("info", path, taskID, "", "missing-start-time", "start_time",
					fmt.Sprintf("第%d行start_time为空，该对话不纳入task时间范围计算", lineNum))
			}
			if conv.EndTime != "" {
				if _, err := time.Parse(time.RFC3339, conv.EndTime); err != nil {
					ctx.addIssue("warn", path, taskID, "", "invalid-end-time", "end_time",
						fmt.Sprintf("第%d行end_time格式错误(%s)", lineNum, conv.EndTime))
				}
			}
			if conv.StartTime != "" && conv.EndTime != "" {
				t1, err1 := time.Parse(time.RFC3339, conv.StartTime)
				t2, err2 := time.Parse(time.RFC3339, conv.EndTime)
				if err1 == nil && err2 == nil && t1.After(t2) {
					ctx.addIssue("warn", path, taskID, "", "start-after-end", "start_time",
						fmt.Sprintf("第%d行start_time(%s)晚于end_time(%s)，将导致task时间计算异常", lineNum, conv.StartTime, conv.EndTime))
				}
			}
			if conv.UpstreamTokens < 0 {
				ctx.addIssue("error", path, taskID, "", "negative-tokens", "upstream_tokens",
					fmt.Sprintf("第%d行upstream_tokens为负值(%d)", lineNum, conv.UpstreamTokens))
			}
			if conv.DownstreamTokens < 0 {
				ctx.addIssue("error", path, taskID, "", "negative-tokens", "downstream_tokens",
					fmt.Sprintf("第%d行downstream_tokens为负值(%d)", lineNum, conv.DownstreamTokens))
			}
			if (conv.UpstreamTokens > 0) != (conv.DownstreamTokens > 0) {
				ctx.addIssue("warn", path, taskID, "", "tokens-correlation-mismatch", "upstream_tokens",
					fmt.Sprintf("第%d行upstream_tokens(%d)与downstream_tokens(%d)不相关：一个大于0而另一个等于0，通常两者应同大于0或同时等于0",
						lineNum, conv.UpstreamTokens, conv.DownstreamTokens))
			}
			if conv.ProcessTime < 0 {
				ctx.addIssue("warn", path, taskID, "", "negative-process-time", "process_time",
					fmt.Sprintf("第%d行process_time为负值(%d)", lineNum, conv.ProcessTime))
			}
			if conv.Cost < 0 {
				ctx.addIssue("error", path, taskID, "", "negative-cost", "cost",
					fmt.Sprintf("第%d行cost为负值(%.4f)", lineNum, conv.Cost))
			}
			if conv.Model == "" && conv.UpstreamTokens > 0 {
				ctx.addIssue("info", path, taskID, "", "missing-model", "model",
					fmt.Sprintf("第%d行model为空但upstream_tokens=%d，cost无法自动计算", lineNum, conv.UpstreamTokens))
			}
			if conv.ErrorCode != "" && conv.ErrorReason == "" {
				ctx.addIssue("info", path, taskID, "", "missing-error-reason", "error_reason",
					fmt.Sprintf("第%d行error_code=%s但error_reason为空，不利于问题排查", lineNum, conv.ErrorCode))
			}
			if conv.UserInput == "" {
				ctx.addIssue("info", path, taskID, "", "empty-user-input", "user_input",
					fmt.Sprintf("第%d行user_input为空，将影响ancient_minutes估时计算的factor因子", lineNum))
			}
			if conv.RequestContent == "" {
				ctx.addIssue("info", path, taskID, "", "empty-request-content", "request_content",
					fmt.Sprintf("第%d行request_content为空", lineNum))
			}
			if conv.ResponseContent == "" {
				ctx.addIssue("info", path, taskID, "", "empty-response-content", "response_content",
					fmt.Sprintf("第%d行response_content为空", lineNum))
			}
			if conv.Caller == "" {
				ctx.addIssue("info", path, taskID, "", "missing-caller", "caller",
					fmt.Sprintf("第%d行caller为空，将尝试从session继承", lineNum))
			}
			if conv.RepoAddr == "" {
				ctx.addIssue("info", path, taskID, "", "missing-repo-addr", "repo_addr",
					fmt.Sprintf("第%d行repo_addr为空，将尝试从session继承，若session也为空则无法参与silica含硅量计算", lineNum))
			}
			if conv.RepoBranch == "" {
				ctx.addIssue("info", path, taskID, "", "missing-repo-branch", "repo_branch",
					fmt.Sprintf("第%d行repo_branch为空，将尝试从session继承", lineNum))
			}
			if conv.WorkDir == "" {
				ctx.addIssue("info", path, taskID, "", "missing-work-dir", "work_dir",
					fmt.Sprintf("第%d行work_dir为空，将尝试从session继承，work_dir_id生成将受影响", lineNum))
			}

			// diff_lines 检查
			actualLines := countDiffLines(conv.Diff)
			if conv.DiffLines > 0 && actualLines == 0 {
				ctx.addIssue("warn", path, taskID, "", "empty-diff-content", "diff",
					fmt.Sprintf("第%d行diff_lines=%d但diff内容为空，无法生成指纹参与silica含硅量计算", lineNum, conv.DiffLines))
			} else if conv.DiffLines == 0 && actualLines > 0 {
				ctx.addIssue("warn", path, taskID, "", "diff-lines-mismatch", "diff_lines",
					fmt.Sprintf("第%d行diff_lines=0 但diff内容实际可解析出%d行", lineNum, actualLines))
			} else if conv.DiffLines > 0 && actualLines > 0 {
				anyDiffContent = true
				totalConvWithDiff++
				if isDiffLinesMismatch(int(conv.DiffLines), actualLines) {
					diff := int(conv.DiffLines) - actualLines
					if diff < 0 {
						diff = -diff
					}
					ctx.addIssue("warn", path, taskID, "", "diff-lines-mismatch", "diff_lines",
						fmt.Sprintf("第%d行diff_lines=%d 与diff内容解析出的%d行差异过大(差值=%d)", lineNum, conv.DiffLines, actualLines, diff))
				}
			}

			totalDiffLines += int64(conv.DiffLines)
			totalActualDiffLines += int64(countDiffLines(conv.Diff))
			totalUpstream += int64(conv.UpstreamTokens)
			totalDownstream += int64(conv.DownstreamTokens)
		}

		if totalUpstream == 0 && len(convs) > 0 {
			ctx.addIssue("info", path, taskID, "", "zero-upstream-tokens", "upstream_tokens",
				"所有conversation的upstream_tokens累加为0，token消耗统计将为空")
		}
		if totalDownstream == 0 && len(convs) > 0 {
			ctx.addIssue("info", path, taskID, "", "zero-downstream-tokens", "downstream_tokens",
				"所有conversation的downstream_tokens累加为0，token消耗统计将为空")
		}
		if len(convs) > 0 && validStartCount == 0 {
			ctx.addIssue("warn", path, taskID, "", "all-start-times-invalid", "start_time",
				fmt.Sprintf("共%d条对话均无有效start_time，task_real_minutes将为0", len(convs)))
		}
		if len(convs) > 0 && !anyDiffContent {
			ctx.addIssue("info", path, taskID, "", "no-fingerprints", "diff",
				"所有conversation的diff内容均为空，无法生成代码指纹，该task将无法参与silica含硅量计算")
		}

	}
	return nil
}

func (ctx *checkContext) checkRepos() error {
	cnt := 0
	for commitID, path := range ctx.repoFileMap {
		cnt++
		logx.PromptProgress(cnt, 50)

		data, err := storage.ReadFile(path)
		if err != nil {
			ctx.addIssue("error", path, "", commitID, "read-failed", "", fmt.Sprintf("读取文件失败: %v", err))
			continue
		}

		var commitData RepoCommitData
		if err := json.Unmarshal(data, &commitData); err != nil {
			ctx.addIssue("error", path, "", commitID, "json-parse-failed", "", fmt.Sprintf("JSON解析失败: %v", err))
			continue
		}

		if commitData.CommitId == "" {
			ctx.addIssue("error", path, "", commitID, "missing-commit-id", "commit_id", "commit_id为空，该提交无法被索引")
		} else if commitData.CommitId != commitID {
			ctx.addIssue("warn", path, "", commitID, "commit-id-mismatch", "commit_id",
				fmt.Sprintf("文件内commit_id(%s)与文件名(%s)不一致，可能导致数据关联混乱", commitData.CommitId, commitID))
		}
		if commitData.CommitTime == "" {
			ctx.addIssue("error", path, "", commitID, "missing-commit-time", "commit_time", "commit_time为空，该提交无法按日期聚合")
		} else {
			if _, err := time.Parse(time.RFC3339, commitData.CommitTime); err != nil {
				ctx.addIssue("error", path, "", commitID, "invalid-commit-time", "commit_time",
					fmt.Sprintf("commit_time格式错误(%s)，无法解析为有效时间", commitData.CommitTime))
			}
		}
		if commitData.UserId == "" {
			ctx.addIssue("warn", path, "", commitID, "missing-user-id", "user_id", "user_id为空，该提交无法按用户聚合统计")
		}
		if commitData.RepoAddr == "" {
			ctx.addIssue("warn", path, "", commitID, "missing-repo-addr", "repo_addr", "repo_addr为空，含硅量计算中无法与task关联")
		}
		if commitData.RepoBranch == "" {
			ctx.addIssue("info", path, "", commitID, "missing-repo-branch", "repo_branch", "repo_branch为空")
		}
		if commitData.GitUserName == "" && commitData.GitUserEmail == "" {
			ctx.addIssue("info", path, "", commitID, "missing-git-user", "git_user_name",
				"git_user_name和git_user_email均为空，无法通过git信息关联用户")
		}
		if commitData.DiffLines < 0 {
			ctx.addIssue("error", path, "", commitID, "commit-diff-lines-negative", "diff_lines",
				fmt.Sprintf("diff_lines为负值(%d)，将导致估时计算异常", commitData.DiffLines))
		}

		// diff_lines 检查
		actualLines := countDiffLines(commitData.Diff)
		if commitData.DiffLines > 0 && actualLines == 0 {
			ctx.addIssue("warn", path, "", commitID, "commit-empty-diff-content", "diff",
				fmt.Sprintf("diff_lines=%d但diff内容为空，无法生成指纹参与silica含硅量计算", commitData.DiffLines))
		} else if commitData.DiffLines == 0 && actualLines > 0 {
			ctx.addIssue("warn", path, "", commitID, "diff-lines-mismatch", "diff_lines",
				fmt.Sprintf("diff_lines=0 但diff内容实际可解析出%d行", actualLines))
		} else if commitData.DiffLines > 0 && actualLines > 0 {
			if isDiffLinesMismatch(commitData.DiffLines, actualLines) {
				diff := commitData.DiffLines - actualLines
				if diff < 0 {
					diff = -diff
				}
				ctx.addIssue("warn", path, "", commitID, "diff-lines-mismatch", "diff_lines",
					fmt.Sprintf("diff_lines=%d 与diff内容解析出的%d行差异过大(差值=%d)", commitData.DiffLines, actualLines, diff))
			}
		}
	}
	return nil
}

func (ctx *checkContext) checkCrossReferences() error {
	// 检查每个summary是否有对应的conversation
	for taskID, summaryPath := range ctx.summaryMap {
		if _, ok := ctx.convMap[taskID]; !ok {
			ctx.addIssue("error", summaryPath, taskID, "", "missing-conversation", "",
				fmt.Sprintf("未找到关联的conversation文件(task_id=%s)，该任务将被视为无对话数据导入", taskID))
		}
		// 注：原「同日目录」启发式(conversation-misplaced)已移除——上游 summary 用入库日、
		// conversation 用内容日，两者目录日期本就可不同(实测 06/13 vs 05/13)；且新分片布局多一层
		// <sessionId> 目录，按目录比对必假阳。存在性校验(上面 missing-conversation)已足够。
	}

	// 检查conversation文件名与实际内容一致性（如果conversation内部有task_id字段的话）
	// 当前conversation行内没有task_id，跳过
	return nil
}

func isDiffLinesMismatch(expected, actual int) bool {
	if expected <= 0 {
		return false
	}
	diff := expected - actual
	if diff < 0 {
		diff = -diff
	}
	if expected <= 5 {
		return diff >= 3
	}
	if expected <= 20 {
		return diff >= 5 || float64(diff)/float64(expected) > 0.3
	}
	return diff > 100 || float64(diff)/float64(expected) > 0.5
}

func parseConversationLines(data []byte, path string, ctx *checkContext) ([]taskConversation, error) {
	var convs []taskConversation
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var conv taskConversation
		if err := json.Unmarshal([]byte(line), &conv); err != nil {
			return nil, fmt.Errorf("第%d行JSON解析失败: %w", lineNum, err)
		}
		if conv.Cost == 0 && conv.UpstreamTokens > 0 && conv.Model != "" {
			conv.Cost = flexFloat64(util.CalculateCost(conv.Model, int64(conv.UpstreamTokens), int64(conv.DownstreamTokens), ctx.modelPrices))
		}
		convs = append(convs, conv)
	}
	return convs, scanner.Err()
}

// countDiffLines 从diff文本中统计变更行数（新增+删除）
func countDiffLines(diffText string) int {
	if strings.TrimSpace(diffText) == "" {
		return 0
	}

	// 尝试JSON diff格式
	var jsonDiff []diffJSONEntry
	if err := json.Unmarshal([]byte(diffText), &jsonDiff); err == nil && len(jsonDiff) > 0 {
		return countJSONDiffLines(jsonDiff)
	}

	// 尝试 before/after 格式
	if strings.Contains(diffText, "<<< BEFORE") && strings.Contains(diffText, ">>> AFTER") {
		return countBeforeAfterDiffLines(diffText)
	}

	// 统一diff格式
	return countUnifiedDiffLines(diffText)
}

func countJSONDiffLines(entries []diffJSONEntry) int {
	total := 0
	for _, e := range entries {
		total += e.Additions + e.Deletions
	}
	return total
}

func countUnifiedDiffLines(diffText string) int {
	additions, deletions := 0, 0
	for _, line := range strings.Split(diffText, "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			if strings.TrimSpace(line[1:]) != "" {
				additions++
			}
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			if strings.TrimSpace(line[1:]) != "" {
				deletions++
			}
		}
	}
	return additions + deletions
}

func countBeforeAfterDiffLines(diffText string) int {
	var additions, deletions int
	var beforeContent, afterContent strings.Builder
	var inBefore, inAfter bool

	for _, line := range strings.Split(diffText, "\n") {
		if strings.TrimSpace(line) == "<<< BEFORE" {
			if afterContent.Len() > 0 || beforeContent.Len() > 0 {
				add, del := countContentDiffLines(beforeContent.String(), afterContent.String())
				additions += add
				deletions += del
				beforeContent.Reset()
				afterContent.Reset()
			}
			inBefore = true
			inAfter = false
			continue
		}
		if strings.TrimSpace(line) == ">>> AFTER" {
			inBefore = false
			inAfter = true
			continue
		}
		if strings.HasPrefix(line, "--- ") {
			if afterContent.Len() > 0 || beforeContent.Len() > 0 {
				add, del := countContentDiffLines(beforeContent.String(), afterContent.String())
				additions += add
				deletions += del
				beforeContent.Reset()
				afterContent.Reset()
			}
			inBefore = false
			inAfter = false
			continue
		}
		if inBefore {
			beforeContent.WriteString(line)
			beforeContent.WriteByte('\n')
		} else if inAfter {
			afterContent.WriteString(line)
			afterContent.WriteByte('\n')
		}
	}
	if afterContent.Len() > 0 || beforeContent.Len() > 0 {
		add, del := countContentDiffLines(beforeContent.String(), afterContent.String())
		additions += add
		deletions += del
	}
	return additions + deletions
}

func countContentDiffLines(before, after string) (additions, deletions int) {
	beforeLines := make(map[string]bool)
	for _, line := range strings.Split(before, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			beforeLines[trimmed] = true
		}
	}
	afterLines := make(map[string]bool)
	for _, line := range strings.Split(after, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			afterLines[trimmed] = true
		}
	}
	for line := range afterLines {
		if !beforeLines[line] {
			additions++
		}
	}
	for line := range beforeLines {
		if !afterLines[line] {
			deletions++
		}
	}
	return additions, deletions
}

func (ctx *checkContext) writeReport(output string) error {
	data, err := json.MarshalIndent(ctx.issues, "", "\t")
	if err != nil {
		return fmt.Errorf("序列化报告失败: %w", err)
	}
	if err := os.WriteFile(output, data, 0644); err != nil {
		return fmt.Errorf("写入报告文件失败: %w", err)
	}
	return nil
}

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "检查summary和conversation目录下数据文件的字段缺失和错误",
	Long: `提前检查summary和conversation目录下的原始数据正确性，输出问题清单文件，作为排障依据或提前将问题数据剔除。

支持的 --ignore issue 类型：
  all-start-times-invalid, chunk-gap, commit-diff-lines-negative,
  commit-empty-diff-content, commit-id-mismatch, diff-lines-mismatch,
  empty-diff-content, empty-request-content, empty-response-content,
  empty-user-input, invalid-commit-time, invalid-end-time,
  invalid-start-time, json-parse-failed, missing-caller, missing-client-id,
  missing-commit-id, missing-commit-time, missing-conversation,
  missing-error-reason, missing-git-user, missing-model, missing-repo-addr,
  missing-repo-branch, missing-request-id, missing-session-id,
  missing-start-time, missing-user-id, missing-user-name, missing-work-dir,
  negative-cost, negative-process-time, negative-tokens, no-fingerprints,
  orphan-conversation, parse-failed, read-failed, session-id-mismatch,
  start-after-end, tokens-correlation-mismatch, zero-downstream-tokens,
  zero-upstream-tokens`,
	RunE: func(cmd *cobra.Command, args []string) error {
		taskDir, _ := cmd.Flags().GetString("task-dir")
		repoDir, _ := cmd.Flags().GetString("repo-dir")
		dateFilter, _ := cmd.Flags().GetString("date")
		output, _ := cmd.Flags().GetString("output")
		level, _ := cmd.Flags().GetString("level")

		if taskDir == "" {
			taskDir = appconfig.Cfg.TaskDir
		}
		if repoDir == "" {
			repoDir = appconfig.Cfg.RepoDir
		}
		if output == "" {
			output = fmt.Sprintf("check-%s.json", time.Now().Format("20060102"))
		}

		ignoreList, _ := cmd.Flags().GetStringSlice("ignore")

		return runCheck(taskDir, repoDir, dateFilter, output, level, ignoreList)
	},
}

func init() {
	checkCmd.Flags().SortFlags = false
	checkCmd.Flags().String("task-dir", "", "task 目录路径（包含summary和conversation子目录），默认从配置文件获取")
	checkCmd.Flags().String("repo-dir", "", "repo 目录路径，默认从配置文件获取")
	checkCmd.Flags().String("date", "", "指定分析的日期（格式YYYYMMDD），不指定则分析全部")
	checkCmd.Flags().String("output", "", "分析报告输出文件路径，默认为check-{当前日期}.json")
	checkCmd.Flags().String("level", "warn", "指定输出issue的最低级别，可选值为info/warn/error，默认warn")
	checkCmd.Flags().StringSlice("ignore", nil, "忽略指定类型的issue，可多次使用或逗号分隔，如--ignore=diff-lines-mismatch,missing-model")
	rootCmd.AddCommand(checkCmd)
}
