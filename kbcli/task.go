package main

import (
	"fmt"
	"kanban/kbcli/internal/estimator"
	"sort"
	"strings"
	"time"
)

// taskSilicaData 用于生成任务级别的 silica 摘要文件，保存到 analysedDir 中。
type taskSilicaData struct {
	SessionId       string                   `json:"session_id"`
	UserId          string                   `json:"user_id"`
	Size            int64                    `json:"size"`
	ConversationNum int                      `json:"conversation_num"`
	Conversations   []taskSilicaConversation `json:"conversations"`
}

// taskSilicaConversation 为 taskSilicaData 中的单条对话摘要，记录指纹信息。
type taskSilicaConversation struct {
	RequestId      string   `json:"request_id"`
	StartTime      string   `json:"start_time"`
	EndTime        string   `json:"end_time"`
	RepoAddr       string   `json:"repo_addr"`
	UserInputChars int      `json:"user_input_chars"`
	Fingerprints   []string `json:"fingerprints"`
}

// estimateTaskAncientMinutes 估算任务的“原始工作量”分钟数。
// 功能：基于用户输入字符数和代码新增行数，结合配置参数，通过线性因子模型估算开发者完成该任务所需的原始时间。
// 参数：
//   - cfg: 估算算法的配置参数，包含最大输入字符、因子范围、每行代码耗时等。
//   - convs: 任务下的所有对话列表。
//   - realMinutes: 已计算出的真实工作时长，用于设置上下界约束。
//
// 返回值：
//   - float64: 估算的原始工作量分钟数。
//   - string: 估算依据的说明文本。
//
// 关键技术原理：
//  1. 汇总所有对话的 user_input 长度（totalInchars）和 diff 新增行数（totalDiffLines）。
//  2. 输入字符数经截断后，映射到 [MinFactor, MaxFactor] 区间的线性因子 factor。
//  3. workload = (totalLines / LinesPerMinutes) * factor，反映“按代码量估算的时间”。
//  4. 用 maxWorkload（真实时长的 MaxRatio 倍）和 minWorkload（不小于 MinMinutes 且不小于真实时长）进行上下界裁剪，防止极端偏差。
func estimateTaskAncientMinutes(ec *estimator.EstimateConfig, totalInchars, totalLines, realMinutes float64) (float64, string) {
	// 输入字符数超过配置上限时截断，避免异常大输入导致 factor 越界
	if totalInchars >= ec.MaxInputChars {
		totalInchars = ec.MaxInputChars
	}

	// 线性插值计算因子：输入越多，因子越接近 MaxFactor
	factor := ec.MinFactor + (totalInchars/ec.MaxInputChars)*(ec.MaxFactor-ec.MinFactor)
	// 基于代码行数和因子计算初步工作量
	workload := (totalLines / ec.LinesPerMinutes) * factor

	// 计算上下界：上限为真实时长的 MaxRatio 倍；下限不小于 MinMinutes 且不小于真实时长
	maxWorkload := ec.MaxRatio * realMinutes
	minWorkload := max(ec.MinMinutes, realMinutes)

	// 对 workload 进行裁剪，确保结果在合理区间内
	if workload > maxWorkload {
		workload = maxWorkload
	}
	if workload < minWorkload {
		workload = minWorkload
	}

	return workload, fmt.Sprintf(
		"基于diff_lines=%.0f, user_input=%.0f字符, factor=%.2f, real_minutes=%.2f估算",
		totalLines, float64(totalInchars), factor, realMinutes,
	)
}

// calcTaskRealMinutes 计算任务的真实工作时长（分钟）。
// 功能：根据所有对话的开始时间，将相邻且间隔不超过阈值的时间合并为连续工作片段，累加各片段时长得到总工作时长。
// 参数：
//   - conversations: 任务下的所有对话列表。
//   - gapThreshold: 两条对话之间允许的最大间隔（分钟），超过则认为进入新的工作片段。
//   - extensionMin: 每个工作片段结束时追加的延展时间（分钟），用于覆盖末尾等待或思考时间。
//
// 返回值：
//   - float64: 真实工作总时长（分钟）。
//   - string: 各时间片段的详细说明文本。
//
// 关键技术原理：
//  1. 提取所有有效开始时间并排序。
//  2. 顺序遍历时间序列：若相邻时间差 <= gapThreshold，则合并到当前片段；否则结束当前片段并追加 extensionMin，同时开启新片段。
//  3. 最后一个片段同样追加 extensionMin。
//  4. 累加各片段 (end - start) 的分钟数得到总时长。
func calcTaskRealMinutes(validTimes []time.Time, gapThreshold, extensionMin int) (float64, string) {
	// 边界情况：无有效对话或仅一条对话时返回固定默认值
	if len(validTimes) == 0 {
		return 0, "无有效对话"
	}
	if len(validTimes) == 1 {
		return float64(extensionMin), fmt.Sprintf("仅1条对话，默认%d分钟", extensionMin)
	}
	// 按时间先后排序，保证片段合并顺序正确
	sort.Slice(validTimes, func(i, j int) bool {
		return validTimes[i].Before(validTimes[j])
	})
	gapDur := time.Duration(gapThreshold) * time.Minute
	ext := time.Duration(extensionMin) * time.Minute
	// timeSeg 表示一个连续工作片段：start 为片段开始时间，end 为片段结束时间（含追加的延展时间），convCount 为片段内对话数
	type timeSeg struct {
		start     time.Time
		end       time.Time
		convCount int
	}
	// 初始化第一个片段
	segments := []timeSeg{{start: validTimes[0], end: validTimes[0], convCount: 1}}
	for i := 1; i < len(validTimes); i++ {
		cur := &segments[len(segments)-1]
		gap := validTimes[i].Sub(cur.end)
		// 若间隔在阈值内，合并到当前片段
		if gap <= gapDur {
			cur.end = validTimes[i]
			cur.convCount++
		} else {
			// 结束当前片段并追加延展时间，同时开启新片段
			cur.end = cur.end.Add(ext)
			segments = append(segments, timeSeg{start: validTimes[i], end: validTimes[i], convCount: 1})
		}
	}
	// 最后一个片段也需要追加延展时间
	segments[len(segments)-1].end = segments[len(segments)-1].end.Add(ext)
	var totalMinutes float64
	var parts []string
	// 累加所有片段时长并生成说明文本
	for _, seg := range segments {
		mins := seg.end.Sub(seg.start).Minutes()
		totalMinutes += mins
		parts = append(parts, fmt.Sprintf("%s~%s(%d条对话)",
			seg.start.Format("2006-01-02 15:04"),
			seg.end.Format("2006-01-02 15:04"),
			seg.convCount))
	}
	reason := fmt.Sprintf("%d个时间片段: [%s]", len(segments), strings.Join(parts, ", "))
	return totalMinutes, reason
}
