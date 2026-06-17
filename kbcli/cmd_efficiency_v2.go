package main

import (
	"fmt"
	"kanban/kbcli/internal/appconfig"
	"kanban/kbcli/internal/efficiencyv2"
	"kanban/kbcli/internal/estimator"
	"kanban/kbcli/internal/governance"
	"kanban/kbcli/internal/llm"
	"kanban/kbcli/internal/logx"
	"kanban/kbcli/internal/util"
	"strings"
	"time"

	"kanban/core/models"

	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

var efficiencyV2Cmd = &cobra.Command{
	Use:   "efficiency-v2",
	Short: "运行v2效率管道：事件归一化→阶段切分→Need 边界→实绩聚合→基线→融合→用户周聚合",
	RunE: func(cmd *cobra.Command, args []string) error {
		dateStr, _ := cmd.Flags().GetString("date")
		startDate, _ := cmd.Flags().GetString("start-date")
		endDate, _ := cmd.Flags().GetString("end-date")
		remote, _ := cmd.Flags().GetString("remote")

		if remote != "" {
			return util.SendToRemote(remote, "efficiency-v2", map[string]interface{}{
				"date":       dateStr,
				"start_date": startDate,
				"end_date":   endDate,
			})
		}
		// 未显式传 start-date 且非单日(date)模式时，套全局分析起始日下界。
		if dateStr == "" {
			startDate = appconfig.ApplyAnalysisFloor(startDate)
		}
		return runEfficiencyV2(startDate, endDate, dateStr)
	},
}

func init() {
	efficiencyV2Cmd.Flags().SortFlags = false
	efficiencyV2Cmd.Flags().String("date", "", "限定日期，格式YYYYMMDD")
	efficiencyV2Cmd.Flags().String("start-date", "", "限定起始日期，格式YYYYMMDD")
	efficiencyV2Cmd.Flags().String("end-date", "", "限定结束日期，格式YYYYMMDD")
	efficiencyV2Cmd.Flags().String("remote", "", "远程kbcli服务地址")
	rootCmd.AddCommand(efficiencyV2Cmd)
	validTaskTypes["efficiency-v2"] = true
}

// ParseEfficiencyV2DateParams normalises CLI date inputs to YYYY-MM-DD strings
// suitable for downstream SQL filters. It accepts YYYYMMDD or YYYY-MM-DD.
func ParseEfficiencyV2DateParams(startDate, endDate, dateStr string) (string, string, error) {
	if dateStr != "" {
		d, err := efficiencyV2NormaliseDate(dateStr)
		if err != nil {
			return "", "", fmt.Errorf("--date %w", err)
		}
		return d, d, nil
	}
	var start, end string
	if startDate != "" {
		d, err := efficiencyV2NormaliseDate(startDate)
		if err != nil {
			return "", "", fmt.Errorf("--start-date %w", err)
		}
		start = d
	}
	if endDate != "" {
		d, err := efficiencyV2NormaliseDate(endDate)
		if err != nil {
			return "", "", fmt.Errorf("--end-date %w", err)
		}
		end = d
	}
	if start != "" && end != "" && start > end {
		return "", "", fmt.Errorf("start-date %q must be <= end-date %q", start, end)
	}
	return start, end, nil
}

func efficiencyV2NormaliseDate(value string) (string, error) {
	value = strings.TrimSpace(value)
	switch len(value) {
	case 8:
		if _, err := time.Parse("20060102", value); err != nil {
			return "", err
		}
		return value[:4] + "-" + value[4:6] + "-" + value[6:8], nil
	case 10:
		if _, err := time.Parse("2006-01-02", value); err != nil {
			return "", err
		}
		return value, nil
	default:
		return "", fmt.Errorf("invalid date %q (expected YYYYMMDD or YYYY-MM-DD)", value)
	}
}

// runEfficiencyV2 wires the full v2 pipeline against the configured stats DB.
func runEfficiencyV2(startDateStr, endDateStr, dateStr string) error {
	start, end, err := ParseEfficiencyV2DateParams(startDateStr, endDateStr, dateStr)
	if err != nil {
		return err
	}
	db, err := models.OpenGormDB(appconfig.Cfg.StatDatabase.DSN())
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	// commit governance pass：管线取数前先按治理配置重判 commits 排除标记（改名单→重跑即生效）。
	govCfg, err := governance.Load(appconfig.Cfg.GovernanceFile)
	if err != nil {
		return fmt.Errorf("加载治理配置失败: %w", err)
	}
	logx.Infof("efficiency-v2: commit governance pass (governance_file=%q, window=[%s,%s])", appconfig.Cfg.GovernanceFile, start, end)
	if err := governance.ApplyCommitGovernance(db, govCfg, start, end); err != nil {
		return fmt.Errorf("commit governance pass 失败: %w", err)
	}
	// repo 地址归一开关由治理配置注入（normalization.repo_addr_canon），不走 efficiency_v2 yaml。
	effCfg := appconfig.Cfg.EfficiencyV2
	effCfg.RepoAddrCanon = govCfg.Normalization.RepoAddrCanon
	// user_id 黑名单同样由治理配置注入（identity.blocked_user_ids）：
	// blocked 账号的 session/conversation 不进任何统计（commit 侧由 identity 子 pass 排除）。
	effCfg.BlockedUserIds = govCfg.Identity.BlockedUserIds
	return RunEfficiencyV2Pipeline(db, EfficiencyV2PipelineArgs{
		StartDate:      start,
		EndDate:        end,
		EfficiencyV2:   effCfg,
		AIEstimation:   appconfig.Cfg.AIEstimation,
		AlgoEstimation: appconfig.Cfg.AlgoEstimation,
	})
}

type EfficiencyV2PipelineArgs struct {
	StartDate      string
	EndDate        string
	EfficiencyV2   efficiencyv2.EfficiencyV2Config
	AIEstimation   llm.AIEstimationConfig
	AlgoEstimation estimator.EstimateConfig
}

type EfficiencyV2PipelineCounts struct {
	Events           int
	StageMetrics     int
	Needs            int
	UserProductivity int
}

// RunEfficiencyV2Pipeline executes every step of the v2 pipeline against the
// provided DB and returns step counts. The implementation is idempotent: rerun
// produces the same logical rows.
func RunEfficiencyV2Pipeline(db *gorm.DB, args EfficiencyV2PipelineArgs) error {
	_, err := RunEfficiencyV2PipelineWithCounts(db, args)
	return err
}

func RunEfficiencyV2PipelineWithCounts(db *gorm.DB, args EfficiencyV2PipelineArgs) (EfficiencyV2PipelineCounts, error) {
	counts := EfficiencyV2PipelineCounts{}
	args.EfficiencyV2 = efficiencyv2.NormalizeEfficiencyV2Config(args.EfficiencyV2)
	args.AlgoEstimation = efficiencyv2.NormalizeEfficiencyV2AlgoConfig(args.AlgoEstimation)

	if args.StartDate == "" || args.EndDate == "" {
		convStart, convEnd, err := LookupEfficiencyV2ConversationDateRange(db)
		if err != nil {
			return counts, fmt.Errorf("lookup conversation date range: %w", err)
		}
		if args.StartDate == "" {
			args.StartDate = convStart
		}
		if args.EndDate == "" {
			args.EndDate = convEnd
		}
		logx.Infof("efficiency-v2: 日期窗自动夹紧到 conversation 范围 %s ~ %s", args.StartDate, args.EndDate)
	}

	if err := efficiencyv2.EnsureEfficiencyV2BaselineACoefficients(db, ""); err != nil {
		return counts, fmt.Errorf("ensure baseline coefficients: %w", err)
	}
	if err := efficiencyv2.EnsureEfficiencyV2FusionWeightSnapshot(db, efficiencyv2.EfficiencyV2DefaultTeamID, efficiencyv2.EfficiencyV2MondayAnchor(time.Now().UTC()), args.EfficiencyV2.BaselineDefaults); err != nil {
		return counts, fmt.Errorf("ensure fusion weights: %w", err)
	}

	events, err := efficiencyv2.NormalizeAndUpsertEfficiencyV2ConversationEvents(db, efficiencyv2.EfficiencyV2ConversationEventQuery{
		StartDate:      args.StartDate,
		EndDate:        args.EndDate,
		BlockedUserIds: args.EfficiencyV2.BlockedUserIds,
	})
	if err != nil {
		return counts, fmt.Errorf("normalize events: %w", err)
	}
	counts.Events = len(events)

	metrics, err := efficiencyv2.BuildAndUpsertEfficiencyV2SessionStageMetrics(db, events, args.EfficiencyV2)
	if err != nil {
		return counts, fmt.Errorf("build stage metrics: %w", err)
	}
	counts.StageMetrics = len(metrics)

	// Need 边界解析必须看全量历史（commits 不带日期窗）；日期窗只约束上面的
	// 事件归一化与下面的用户周聚合。详见 ResolveAndUpsertEfficiencyV2Needs 注释。
	needs, prunedNeeds, err := efficiencyv2.ResolveAndUpsertEfficiencyV2Needs(db, args.EfficiencyV2)
	if err != nil {
		return counts, fmt.Errorf("resolve needs: %w", err)
	}
	if _, err := efficiencyv2.AggregateAndUpsertEfficiencyV2NeedActuals(db, needs, args.EfficiencyV2, args.AlgoEstimation); err != nil {
		return counts, fmt.Errorf("aggregate need actuals: %w", err)
	}
	needs, err = ReloadEfficiencyV2Needs(db, needs)
	if err != nil {
		return counts, fmt.Errorf("reload needs: %w", err)
	}
	// P1：把每个 need 的交付物(commit LOC/AI)与努力(会话人时)按工号(emp_no)拆进
	// need_emp_attribution。emp map 在 need 边界 stage 已加载一次，这里复用同口径加载器
	// 单独取一份（仅 commits JOIN dept_user，开销小），供按 committer 工号拆交付物。
	// 需在 need 聚合+reload 之后跑：依赖已持久化的 commit_ids/session_ids 与 emp_no_count。
	empMap, err := efficiencyv2.LoadEfficiencyV2UserEmpMap(db)
	if err != nil {
		return counts, fmt.Errorf("load user→emp map for attribution: %w", err)
	}
	if attributionRows, err := efficiencyv2.AggregateAndUpsertEfficiencyV2NeedEmpAttribution(db, needs, empMap, args.EfficiencyV2); err != nil {
		return counts, fmt.Errorf("need emp attribution: %w", err)
	} else {
		logx.Infof("efficiency-v2: 写入 %d 行 need_emp_attribution（按工号拆交付物/努力）", attributionRows)
	}
	if err := RunEfficiencyV2BaselineAndFusion(db, needs, args); err != nil {
		return counts, fmt.Errorf("baseline+fusion: %w", err)
	}
	counts.Needs = len(needs)

	// Reload needs with the freshly-persisted baseline/fusion fields so we can
	// self-bootstrap anchors. Per design §4.3 + line 798, the team's own merged
	// high-confidence Needs become anchors for future kNN runs.
	if needsReloaded, err := ReloadEfficiencyV2Needs(db, needs); err == nil {
		if added, err := efficiencyv2.UpsertEfficiencyV2SelfBootstrapAnchors(db, needsReloaded); err != nil {
			logx.Warnf("self-bootstrap anchor 写入失败: %v", err)
		} else if added > 0 {
			logx.Infof("self-bootstrap 写入 %d 个 team anchor（下次跑 kNN 生效）", added)
		}
	}

	// need prune 删过旧行（key 换代）时，窗口化重写不够：窗口外历史周的
	// user_productivity_v2.need_ids 会悬挂引用已删 need。此时强制全量重写并
	// 清理未再生成的周行；prune=0 的常规重算仍走窗口化路径（幂等不受影响）。
	var weeklyCount int
	if prunedNeeds > 0 {
		logx.Infof("efficiency-v2: need prune 删除 %d 行，user_productivity_v2 扩为全量重写以刷新历史周引用", prunedNeeds)
		weeklyCount, err = efficiencyv2.RebuildEfficiencyV2UserProductivityAll(db, args.EfficiencyV2)
	} else {
		weeklyCount, err = efficiencyv2.AggregateAndUpsertEfficiencyV2UserProductivity(db, args.EfficiencyV2, args.StartDate, args.EndDate)
	}
	if err != nil {
		return counts, fmt.Errorf("user productivity v2: %w", err)
	}
	counts.UserProductivity = weeklyCount
	return counts, nil
}

func ReloadEfficiencyV2Needs(db *gorm.DB, needs []models.Need) ([]models.Need, error) {
	if len(needs) == 0 {
		return nil, nil
	}
	ids := make([]string, 0, len(needs))
	for _, n := range needs {
		ids = append(ids, n.NeedId)
	}
	var reloaded []models.Need
	if err := db.Where("need_id IN ?", ids).Order("need_id ASC").Find(&reloaded).Error; err != nil {
		return nil, err
	}
	return reloaded, nil
}

// RunEfficiencyV2BaselineAndFusion computes Baseline A, B, C and fusion for
// every Need, then persists the results.
func RunEfficiencyV2BaselineAndFusion(db *gorm.DB, needs []models.Need, args EfficiencyV2PipelineArgs) error {
	if len(needs) == 0 {
		return nil
	}
	coefs := efficiencyv2.LoadEfficiencyV2BaselineACoefficients(db, "")
	// Wire yaml algo_estimation.commit_line_per_minutes into baseline_algo exec.
	// Per design (line 235-240, 731): baseline_exec and actual_uncovered MUST use
	// the same "古法 lines_per_min" rate. Without this override the baseline uses
	// the hardcoded default (0.21) regardless of yaml config.
	if args.AlgoEstimation.CommitLinePerMinutes > 0 {
		coefs.ExecLinesPerMin = args.AlgoEstimation.CommitLinePerMinutes
	}
	// yaml efficiency_v2.baseline_algo 覆盖古法基线主系数（think 每轮 / exec 每文件协调）。
	if v := args.EfficiencyV2.BaselineAlgo.ThinkTurnMin; v > 0 {
		coefs.ThinkTurnMin = v
	}
	if v := args.EfficiencyV2.BaselineAlgo.ExecFileCoordMin; v > 0 {
		coefs.ExecFileCoordMin = v
	}
	anchors, err := efficiencyv2.LoadEfficiencyV2KNNAnchors(db)
	if err != nil {
		return err
	}
	weights, density, _, err := efficiencyv2.LookupEfficiencyV2FusionWeights(db, efficiencyv2.EfficiencyV2DefaultTeamID, args.EfficiencyV2.BaselineDefaults)
	if err != nil {
		return err
	}

	for i := range needs {
		logx.Progress("[efficiency-v2] 基线融合(含LLM)", i+1, len(needs), 1)
		need := &needs[i]
		sessionIDs := efficiencyv2.EfficiencyV2StringsFromJSON(need.SessionIds)
		commitIDs := efficiencyv2.EfficiencyV2StringsFromJSON(need.CommitIds)

		var sessions []models.SessionStageMetric
		if len(sessionIDs) > 0 {
			if err := db.Where("session_id IN ?", sessionIDs).Find(&sessions).Error; err != nil {
				return err
			}
		}
		var commits []models.Commit
		if len(commitIDs) > 0 {
			// 治理排除的 commit 不进基线/LLM 估时（双保险，聚合口径处亦排除）
			if err := db.Where("commit_id IN ? AND excluded_flag = false", commitIDs).Find(&commits).Error; err != nil {
				return err
			}
		}
		var tasks []models.Task
		if len(sessionIDs) > 0 {
			if err := db.Where("session_id IN ?", sessionIDs).Find(&tasks).Error; err != nil {
				return err
			}
		}

		algoResult := efficiencyv2.ComputeEfficiencyV2BaselineA(*need, sessions, nil, commits, coefs)
		efficiencyv2.PersistEfficiencyV2BaselineAOnNeed(need, algoResult)

		knnResult := efficiencyv2.ComputeEfficiencyV2BaselineB(efficiencyv2.BuildEfficiencyV2NeedFeatureVector(*need, sessions), anchors, efficiencyv2.EfficiencyV2KNNDefaultK)
		efficiencyv2.PersistEfficiencyV2BaselineBOnNeed(need, knnResult)

		llmResult := efficiencyv2.CallAIForNeedEstimationV4(efficiencyv2.BuildEfficiencyV2NeedStructuredSummary(*need, sessions, commits, tasks), args.AIEstimation)
		efficiencyv2.PersistEfficiencyV2BaselineCOnNeed(need, llmResult)

		fusionResult := efficiencyv2.ComputeEfficiencyV2Fusion(*need, efficiencyv2.EfficiencyV2FusionInputs{
			AlgoMin:     algoResult.TotalMin,
			KNNMin:      knnResult.Estimate,
			LLMMin:      llmResult.TotalMin,
			Weights:     weights,
			TeamDensity: density,
		}, args.EfficiencyV2)
		efficiencyv2.PersistEfficiencyV2FusionOnNeed(need, fusionResult, args.EfficiencyV2)
	}
	return persistEfficiencyV2NeedBaselineFusion(db, needs)
}

func persistEfficiencyV2NeedBaselineFusion(db *gorm.DB, needs []models.Need) error {
	if len(needs) == 0 {
		return nil
	}
	tx := db.Begin()
	for i := range needs {
		if err := tx.Model(&needs[i]).Updates(map[string]interface{}{
			"baseline_algo_think_work_min":        needs[i].BaselineAlgoThinkWorkMin,
			"baseline_algo_execution_work_min":    needs[i].BaselineAlgoExecutionWorkMin,
			"baseline_algo_verification_work_min": needs[i].BaselineAlgoVerificationWorkMin,
			"baseline_algo_total_work_min":        needs[i].BaselineAlgoTotalWorkMin,
			"baseline_anchor_knn_work_min":        needs[i].BaselineAnchorKnnWorkMin,
			"baseline_anchor_knn_reason":          needs[i].BaselineAnchorKnnReason,
			"baseline_llm_think_work_min":         needs[i].BaselineLLMThinkWorkMin,
			"baseline_llm_execution_work_min":     needs[i].BaselineLLMExecutionWorkMin,
			"baseline_llm_verification_work_min":  needs[i].BaselineLLMVerificationWorkMin,
			"baseline_llm_total_work_min":         needs[i].BaselineLLMTotalWorkMin,
			"baseline_llm_confidence":             needs[i].BaselineLLMConfidence,
			"baseline_llm_reason":                 needs[i].BaselineLLMReason,
			"baseline_fused_work_min":             needs[i].BaselineFusedWorkMin,
			"baseline_spread_work_min":            needs[i].BaselineSpreadWorkMin,
			"baseline_calendar_min":               needs[i].BaselineCalendarMin,
			"team_work_density_used":              needs[i].TeamWorkDensityUsed,
			"team_profile_used":                   needs[i].TeamProfileUsed,
			"efficiency_ratio":                    needs[i].EfficiencyRatio,
			"efficiency_band_low":                 needs[i].EfficiencyLowerBand,
			"efficiency_band_high":                needs[i].EfficiencyUpperBand,
			"work_efficiency_ratio":               needs[i].WorkEfficiencyRatio,
			"confidence_level":                    needs[i].ConfidenceLevel,
			"outlier_flag":                        needs[i].OutlierFlag,
			"calendar_outlier_flag":               needs[i].CalendarOutlierFlag,
			"work_outlier_flag":                   needs[i].WorkOutlierFlag,
			"reason":                              needs[i].Reason,
		}).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("update need %s baseline fusion: %w", needs[i].NeedId, err)
		}
	}
	return tx.Commit().Error
}

// LookupEfficiencyV2ConversationDateRange returns the earliest and latest
// `DATE(start_time)` present in the conversations table, formatted as
// YYYY-MM-DD. If there are no rows, both returns are empty strings.
func LookupEfficiencyV2ConversationDateRange(db *gorm.DB) (string, string, error) {
	var row struct {
		MinDate *time.Time
		MaxDate *time.Time
	}
	if err := db.Raw("SELECT MIN(start_time) AS min_date, MAX(start_time) AS max_date FROM conversations").Scan(&row).Error; err != nil {
		return "", "", err
	}
	if row.MinDate == nil || row.MaxDate == nil {
		return "", "", nil
	}
	return row.MinDate.UTC().Format("2006-01-02"), row.MaxDate.UTC().Format("2006-01-02"), nil
}
