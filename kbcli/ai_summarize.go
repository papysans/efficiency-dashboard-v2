package main

import (
	"encoding/json"
	"fmt"
	"kanban/kbcli/internal/llm"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

// callAIForTaskTitle 调用 AI 从对话记录提取任务标题（不超过100字符）
func callAIForTaskTitle(db *gorm.DB, taskID string, userInputs []string) (string, error) {
	aiCfg := cfg.AIEstimation
	if !aiCfg.Enabled || aiCfg.APIKey == "" {
		return "", fmt.Errorf("AI estimation not enabled or API key missing")
	}

	prompt := fmt.Sprintf(`请根据以下用户与AI助手的对话记录，用一句简短的中文描述这个任务的目标，不超过100个字符。
只输出标题文本，不要任何额外格式或引号。

用户输入记录：
%s`, truncateSlice(userInputs, 3000))

	messages := []llm.ChatMessage{
		{Role: "system", Content: "请回答问题"},
		{Role: "user", Content: prompt},
	}
	content, err := llm.CallLLM(aiCfg, messages, 256)
	if err != nil {
		return "", err
	}

	title := strings.TrimSpace(content)
	title = strings.Trim(title, "\"'`")
	runes := []rune(title)
	if len(runes) > 100 {
		title = string(runes[:100])
	}
	if title == "" {
		return "", fmt.Errorf("AI返回空标题")
	}

	return title, nil
}

// ============================================================================
// v4.5 估算：first principles + anchor sanity check
// 同步源文件：/Volumes/Work/Projects/efficiency-dashboard/prompt_v4.5.md
// ============================================================================

const promptV4System = `你是软件开发工作量评估专家。给定一段开发者与 AI 的对话记录，估算一位**熟悉本项目代码的 3-5 年中级开发者**（使用现代 IDE/搜索引擎/官方文档，但不使用 AI 编码助手）完成相同结果所需的分钟数。

⚠️ 重要：估值对象是"熟手实际操作时间"，不是"新手探索 + 完整工程闭环"。开发者熟悉代码结构，知道改哪、知道工具用法，估的是**实际敲键盘 + 必要测试**的时间。`

const promptV4UserTemplate = `你将先**独立推理**给出初步估值，再用参考样本做 sanity check。

【校准基准】（熟手在熟悉代码库里的实际操作时间，必须遵守此区间）
  · 单字段/typo/console 修复 (diff 1-5)              : 5-15 min
  · 加 if 判断 + 简单告警/日志 (diff 5-30)            : 10-30 min
  · 加按钮/小组件/单接口扩展 (diff 10-50)             : 15-45 min
  · 单文件小功能新增 (diff 50-150)                   : 30-90 min
  · 中等功能/多文件协调 (diff 150-500)                : 60-180 min
  · 大型重构/新模块/跨多服务 (diff > 500)             : 180-480 min
  · 工程化文档/设计 (零 diff, 纯思考)                 : 看任务范围 30-240 min

  这些是熟手估值范围，**不是新手探索的范围**。超出上限的估值需要明确理由（如确实涉及大量调研、跨多个不熟悉的系统）。

【STEP 1 - 独立推理（先不看下面的参考样本）】
基于任务标题、diff 行数、输入字符数，回答：
  · 这个任务的核心目标和范围是什么？
  · 涉及多少模块？技术难度如何（low/medium/high）？
  · 熟手开发者（有 IDE+搜索、无 AI）大约要多久？拆分四项给分钟：
    - 理解需求 + 查资料（项目内：通常 5-15 min；跨项目：可适当增加）
    - 编码（参考"校准基准"对应区间）
    - 调试 + 单测（不是完整 TDD，是必要的几次试跑；小修复可为 0）
    - 集成验证 + 评审（**小修复跳过，给 0**；中等以上才需要）
  · 求和 = preliminary_minutes（初步估值）

【STEP 2 - 参考样本 sanity check】
现在阅读下面的参考样本，找出 1-3 个最相似的（注意是真正相似，不是仅标题词相同）：
  · 它们的人估值是多少？
  · 你的 preliminary_minutes 是否在合理范围（最相似锚点的 0.5x-2x 内）？
  · 如果差距很大（>2x），是你估错了还是任务真的不一样？

【STEP 3 - 最终估值与置信度】
  · 如果锚点支持你的初步估值 → 用初步估值，confidence=high
  · 如果锚点提示偏差，你判断锚点更对 → 调整估值，confidence=medium
  · 如果没有真正相似的锚点 → 用初步估值，confidence=low
  · **禁止无脑取锚点值**：标题词相同不等于任务相同
  · **禁止累加 4 项默认值**：对小修复，"调试测试"和"集成验证"通常应该是 0

【关键提醒（违反这些规则的估值必须重新审视）】
- 估值是"熟手实际操作时间"，不是"新手从零探索 + 4 道完整工序累加"
- 加 if 判断 + 日志告警这类小修复，**整体通常 ≤ 30 min**，不要套测试+评审完整流程
- typo 修复、单参数调整、改默认值，**通常 5-15 min**
- 加按钮 + window.open / 单文件小 UI 改动，**通常 10-30 min**
- "实现 X 执行器/系统/框架" 这种短语在锚点里可能对应 2400 min 的大型项目，但你的待估任务可能只是一个小型实现，要看 diff、字符数、范围综合判断
- "新增字段" 多数 15-30 min，只在确实跨多组件/接口/迁移时才到 60+ min
- diff_lines > 1000 极大概率是 AI 误展开/复制大段代码，按真实意图估，不按行数
- diff=0 但涉及"分析/咨询/查询"类任务：按实际讨论的复杂度估，**不要按完整开发任务估**

【参考样本（按类别分组）】
  · [a 新增/实现] 前端实现分支选择下拉框 (diff=52, 输入=1549字, 人估120min)
  · [a 新增/实现] 在需求详情页右侧新增交互原型标签页，用iframe嵌套显示原型URL (diff=34, 输入=3445字, 人估240min)
  · [b 分析/排查] 分析修改formatTime函数导致前端无法正常显示的原因 (diff=22, 输入=0字, 人估40min)
  · [b 分析/排查] 解释后端回调接口如何通过网关层调用 (diff=7, 输入=1002字, 人估120min)
  · [c 修复/bug] 修复Vue文件中executor_names属性缺失的类型错误 (diff=1, 输入=1631字, 人估10min)
  · [c 修复/bug] 修复CasesView.vue中3个TypeScript类型错误。 (diff=4, 输入=1866字, 人估20min)
  · [d 测试] 为 gptprocessor/utils/time.go 编写单元测试 (diff=481, 输入=9579字, 人估210min)
  · [d 测试] 为 gptprocessor/utils/file.go 编写单元测试。 (diff=207, 输入=3879字, 人估270min)
  · [e 修改/调整] 修改前端使"用户"字段显示当前登录用户名 (diff=115, 输入=2419字, 人估30min)
  · [e 修改/调整] 修改前端展示用户名为当前登录用户名 (diff=800, 输入=119073字, 人估70min)
  · [f 文档/Skill] 创建一个包含元信息、工作流程、执行逻辑、文档更新和踩坑记录管理的SKILL.md文件。 (diff=235, 输入=13045字, 人估45min)
  · [f 文档/Skill] 参照ACL策略接口文档，编写安全策略接口适配文档 (diff=231, 输入=5448字, 人估270min)
  · [g 配置/部署] 将队列并发数量改为可配置参数 (diff=19, 输入=5968字, 人估60min)
  · [g 配置/部署] 将队列并发数量参数抽取为可配置项 (diff=19, 输入=5968字, 人估120min)
  · [h review/审查] 检查Lua代码生成Policy ID是否存在问题 (diff=1844, 输入=25108字, 人估10min)
  · [h review/审查] 检查后端中间件的状态数量 (diff=3, 输入=13754字, 人估120min)

【待估算任务】
标题: {{task_title}}
diff_lines: {{diff_lines}}
用户输入字符数: {{total_chars}}

【输出 JSON（严格按此格式）】
{
  "step1_first_principles": {
    "complexity": "low|medium|high",
    "breakdown": {"理解": <数字>, "编码": <数字>, "调试测试": <数字>, "集成验证": <数字>},
    "preliminary_minutes": <数字>
  },
  "step2_anchor_check": {
    "similar_anchors": ["<锚点描述1>", "<锚点描述2>"],
    "anchor_estimates": [<数字>, <数字>],
    "agreement": "high|medium|low"
  },
  "step3_final": {
    "final_minutes": <数字>,
    "confidence": "high|medium|low",
    "reason": "<100-200 字>"
  }
}`

// callAIForAncientEstimation 使用 prompt v4.5 估算古法工时
//
// 入参：
//   - title:      任务标题（来自 DB 或 callAIForTaskTitle 取得）
//   - diffLines:  代码变更行数
//   - totalChars: 用户输入总字符数
//
// 返回：
//   - minutes:    final_minutes
//   - reason:     reason 字段（含 confidence 标记）
func callAIForAncientEstimation(title string, diffLines int, totalChars int64) (float64, string, error) {
	aiCfg := cfg.AIEstimation
	if !aiCfg.Enabled || aiCfg.APIKey == "" {
		return 0, "", fmt.Errorf("AI estimation not enabled or API key missing")
	}

	userPrompt := strings.NewReplacer(
		"{{task_title}}", title,
		"{{diff_lines}}", strconv.Itoa(diffLines),
		"{{total_chars}}", strconv.FormatInt(totalChars, 10),
	).Replace(promptV4UserTemplate)

	messages := []llm.ChatMessage{
		{Role: "system", Content: promptV4System},
		{Role: "user", Content: userPrompt},
	}
	content, err := llm.CallLLM(aiCfg, messages, 2048)
	if err != nil {
		return 0, "", err
	}

	jsonText := llm.ExtractJSON(content)
	var parsed struct {
		Step3 struct {
			FinalMinutes float64 `json:"final_minutes"`
			Confidence   string  `json:"confidence"`
			Reason       string  `json:"reason"`
		} `json:"step3_final"`
	}
	if err := json.Unmarshal([]byte(jsonText), &parsed); err != nil {
		return 0, "", fmt.Errorf("解析 v4 估时结果失败: %w, text: %s", err, content)
	}

	if parsed.Step3.FinalMinutes < 0 || parsed.Step3.FinalMinutes > 100000 {
		return 0, "", fmt.Errorf("v4 估时结果异常: %.2f", parsed.Step3.FinalMinutes)
	}

	reason := fmt.Sprintf("[confidence=%s] %s", parsed.Step3.Confidence, parsed.Step3.Reason)
	return parsed.Step3.FinalMinutes, reason, nil
}

// truncateSlice 将字符串切片拼接后截断到 maxLen 字符
func truncateSlice(items []string, maxLen int) string {
	var sb strings.Builder
	for i, s := range items {
		if sb.Len()+len(s) > maxLen {
			remaining := maxLen - sb.Len()
			if remaining > 0 {
				sb.WriteString(s[:remaining])
				sb.WriteString("...(截断)")
			}
			break
		}
		if i > 0 {
			sb.WriteString("\n---\n")
		}
		sb.WriteString(s)
	}
	return sb.String()
}
