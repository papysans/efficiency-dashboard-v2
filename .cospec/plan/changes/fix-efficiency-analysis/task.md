# fix-efficiency-analysis: 提效分析三个问题修复

## 阶段 1：后端修改

### 1.1 后端配置：新增 AI estimation 配置
- [ ] 在 `backend/main.go` 的 `Config` 结构体中新增 `AIEstimation` 字段（结构体含 Enabled, APIKey, BaseURL, Model, TimeoutMS, HTTPProxy）
- [ ] 在 `backend/config.yaml` 中新增 `ai_estimation` 配置块
- [ ] 在 `loadConfig` 函数中设置默认值（TimeoutMS=120000, Model="claude-sonnet-4-20250514"）

### 1.2 后端核心：提取 reason + AI 综合评估提效比例
- [ ] 在 `analysisResult` 结构体中新增 `AIEstimatedReasons []string` 和 `TotalCodeLines int64` 字段
- [ ] 在 `computeFromES` 中提取每个 task 的 `ai_estimated_reason` 字段，收集到 reasons 列表；同时累加 `code_lines`
- [ ] 新增 `extractJSON(text string) string` 函数（从 kbcli/ai_estimator.go 复制）
- [ ] 新增 `callAIForEfficiencyAssessment(...)` 函数：读取 appConfig.AIEstimation 配置，构建 prompt，调用 Anthropic Messages API，返回 efficiency_ratio 和 efficiency_reason
- [ ] 修改 `buildEfficiencyResponse` 签名：新增 `reasons []string`, `efficiencyReason string` 参数；在 `ai_estimated` 中加 `reasons` 字段，在 `efficiency` 中加 `reason` 字段
- [ ] 在 `calculateEfficiency` 中：调用 AI 综合评估，优先用 AI 返回的 ratio；AI 失败则降级为 clamp(简单除法, 0.1, 100)
- [ ] 更新所有 `buildEfficiencyResponse` 调用点，传入 reasons 和 efficiencyReason 参数
- [ ] 后端编译验证：`go build ./...`

## 阶段 2：前端修改

### 2.1 EfficiencyPanel.vue：展示 reason + 修复 0 值显示
- [ ] `fmtDays`: 值为 0 或 null 时返回 "-"
- [ ] `fmtRatio`: 值为 0 或 null 时返回 "-"
- [ ] AI 预估人天卡片下方新增 reasons 展示区（可折叠文本）
- [ ] 提效比例区域展示 efficiency_reason（若存在）

### 2.2 ProjectPanel/UserPanel/Dashboard：修复 0 值显示
- [ ] ProjectPanel.vue 的 `fmtDays` 函数：0 值显示 "-"
- [ ] UserPanel.vue 的 `fmtDays` 函数：0 值显示 "-"
- [ ] Dashboard.vue 的 `fmtDays` 函数：0 值显示 "-"
- [ ] 前端构建验证：`npm run build`
