## 实施

- [x] 1.1 创建 `constants.go` — 集中所有常量定义
     【目标对象】`backend/constants.go`（新建）
     【修改目的】消除 analysis_handler.go 和 aggregate_handler.go 中散布的 7+ 处硬编码魔法值
     【修改方式】新建文件，定义 package main 下的常量
     【修改内容】
        - `MsPerWorkDay = 28800000` (8小时/天的毫秒数)
        - `ProcessTimeGapMs = 600000` (10分钟间隔阈值)
        - `DefaultDailyRate = 400.0` (日费率)
        - `ESMaxSearchSize = 10000`
        - `ESTaskIndexPrefix = "costrict_chat_task_"`
        - `ESRequestIndexPrefix = "costrict_chat_request_"`
        - `ESIndexPattern = "costrict_chat_*"`
        - `DefaultPageSize = 50`
        - `AIMaxTokens = 1024`
        - `EfficiencyRatioMin/Max = 0.1 / 10000.0`

- [x] 1.2 创建 `utils.go` — 提取通用工具函数
     【目标对象】`backend/utils.go`（新建）
     【修改目的】从 analysis_handler.go 中提取通用工具函数，消除 kbcli 中也有的重复实现
     【修改方式】新建文件，移入通用函数
     【修改内容】
        - 移入 `ptrFloat64/ptrInt64/ptrInt/ptrString/ptrTime`（指针包装，5个函数）
        - 移入 `safeIDRegex` + `makeSafeID()`
        - 移入 `parseDateParam()`（YYYYMMDD 解析）
        - 移入 `formatDateYMD()`（日期格式化）
        - 移入 `parseESTime()`（ES时间戳解析）
        - 移入 `getFloat64()`（从 map 取 float64）
        - 移入 `extractJSON()`（从 AI 响应提取 JSON）
        - 移入 `generateIndexNames()`（从 es_handler.go）
        - 添加 `calcProcessTimeMs(timestamps []float64) float64`（统一的 process_time 计算，替换两处重复实现）

- [x] 1.3 创建 `ai_client.go` — 提取 AI 调用封装
     【目标对象】`backend/ai_client.go`（新建）
     【修改目的】从 analysis_handler.go 提取 AI 调用逻辑，单独管理
     【修改方式】新建文件，移入 AI 相关结构体和函数
     【修改内容】
        - 移入 `aiEfficiencyResult` 结构体
        - 移入 `callAIForEfficiencyAssessment()` 函数
        - 使用 constants.go 中的 AIMaxTokens 常量

- [x] 1.4 拆分 analysis_handler.go 为 efficiency_handler.go 和 attribution_handler.go
     【目标对象】`backend/efficiency_handler.go`（新建）、`backend/attribution_handler.go`（新建）、`backend/analysis_handler.go`（删除）
     【修改目的】将 1377 行的 analysis_handler.go 按职责拆分
     【修改方式】将现有函数按类别移入新文件，原文件删除
     【修改内容】
        - `efficiency_handler.go` 移入：
          - `userTaskInfo` / `analysisResult` 结构体
          - `computeFromES()`（使用 utils.go 的 calcProcessTimeMs）
          - `buildEfficiencyResponse()`
          - `buildResponseFromProjectMetrics()` / `buildResponseFromRepoMetrics()`
          - `getEfficiency()` handler
          - `calculateEfficiency()` handler
          - `correctEfficiency()` handler
          - `getEfficiencyHistory()` handler
          - `getEfficiencyFile()` handler
          - `updateUserManualDays()` handler
        - `attribution_handler.go` 移入：
          - `getTaskCommitMappings()` handler
          - `getCodeAttribution()` handler
          - `getCodeSourceStats()` handler
        - 所有移入的函数使用 constants.go 的常量替换硬编码值
        - 确认 analysis_handler.go 的所有内容都已移入新文件后删除

- [x] 1.5 优化 db.go — 提取 scan 辅助函数消除字段列表重复
     【目标对象】`backend/db.go`
     【修改目的】消除 project_metrics 字段列表重复 4 次、repo_metrics 字段列表重复 5 次的维护隐患
     【修改方式】提取 `scanProjectMetrics` 和 `scanRepoMetrics` 辅助函数
     【修改内容】
        - 定义 `projectMetricsColumns` 常量（SQL 列名列表字符串）
        - 定义 `scanProjectMetrics(row scanner, m *ProjectMetrics) error` 辅助函数
        - `UpsertProjectMetrics`/`GetProjectMetrics`/`ListProjectMetrics` 使用公共列名和 scan 函数
        - 同样为 `RepoMetrics` 定义 `repoMetricsColumns` 和 `scanRepoMetrics`
        - 添加 `GetLatestRepoMetrics` 函数（合并 git_handler.go 中的重复实现）

- [x] 1.6 清理 git_handler.go — 删除重复代码
     【目标对象】`backend/git_handler.go`
     【修改目的】删除与 db.go 重复的 `getLatestRepoMetrics` 函数
     【修改方式】改用 db.go 中新增的 `GetLatestRepoMetrics` 函数
     【修改内容】
        - 删除 `getLatestRepoMetrics()` 函数（66-92行）
        - 在 `getGitAnalysis` handler 中改用 `GetLatestRepoMetrics(db, repoID)` 调用

- [x] 1.7 更新 aggregate_handler.go — 使用公共函数和常量
     【目标对象】`backend/aggregate_handler.go`
     【修改目的】使用 utils.go 中的公共 process_time 计算函数和 constants.go 中的常量
     【修改方式】替换本地实现为公共函数调用
     【修改内容】
        - `calcProcessTime()` 改为调用 `calcProcessTimeMs()` 公共函数
        - 使用 `ESMaxSearchSize` 常量替换硬编码的 `10000`
        - 使用 `ProcessTimeGapMs` 常量替换硬编码的 `600000`
        - 使用 `ESTaskIndexPrefix` 常量替换硬编码的索引前缀

- [x] 1.8 更新 es_handler.go 和 task_handler.go — 使用公共常量
     【目标对象】`backend/es_handler.go`、`backend/task_handler.go`
     【修改目的】使用 constants.go 的常量替换硬编码值
     【修改方式】替换硬编码为常量引用
     【修改内容】
        - es_handler.go: `"costrict_chat_*"` → `ESIndexPattern`
        - es_handler.go: `generateIndexNames()` 移到 utils.go 后删除本地定义
        - es_handler.go/task_handler.go: `50` → `DefaultPageSize`

- [x] 1.9 验证后端编译和功能
     【目标对象】`backend/`
     【修改目的】确保拆分重构后编译通过且路由不变
     【修改方式】编译测试
     【修改内容】
        - 运行 `go build ./...` 确保编译通过
        - 确认 main.go 路由注册无需修改（所有函数仍在 package main 中）
        - 确认无循环引用或缺失引用

- [x] refactor-backend-structure | task: 1.4-fix-1 修复遗漏的硬编码常量
     【目标对象】`backend/ai_client.go`、`backend/vgroup_handler.go`
     【修改目的】代码审查发现2处硬编码常量遗漏，需替换为 constants.go 中的常量
     【修改方式】替换硬编码为常量引用
     【修改内容】
        - ai_client.go 第27行: `28800000.0` → `float64(MsPerWorkDay)`
        - ai_client.go 第28行: `28800000.0` → `float64(MsPerWorkDay)`
        - vgroup_handler.go 第180行: `"costrict_chat_task_"` → `ESTaskIndexPrefix`
        - 修改后运行 `go build ./...` 确认编译通过
