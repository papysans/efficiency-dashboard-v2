# 变更：重构数据处理管道（移除Stat层，新增Task层+AI估时+CSV组织信息）

## 原因
当前数据管道只有 Request → Stat 两层，Stat 是按 7 个维度预计算的物理索引。根据 a.md 重新设计，需要移除 Stat 层，新增 Task 层（按 task_id 归并 + AI 估时），维度聚合改为实时查询。同时组织信息需从 YAML 配置改为 CSV 文件导入。

## 变更内容
- 移除 Stat 层：删除 `StatDoc` 结构体、`BuildStatDocs`/`calculateProcessTime` 函数、`StatIndexMapping`、reindex 中 stat 聚合代码、`stat_builder_test.go`
- 新增 Task 聚合层：从 Request 层按 task_id 分组聚合，数值字段求和，时间字段取 min/max，计数 api_count
- 新增 Task 内容文件提取：提取每个 task 的用户输入和 AI 代码输出，生成 `rawdata/task/YYYY-MM/DD/$task_id.json`
- 集成 AI 估时：调用配置的大模型 API（Anthropic 兼容接口）分析 task 内容，生成 `ai_estimated_days` 和 `ai_estimated_reason`
- 组织信息重构：从 `config.yaml` 的 `org_mappings`（按 user_id 查 map）改为 CSV 文件（支持 user_id + user_name 双重匹配）
- CLI 命令重构：`kbcli reindex --step=request|task`，支持分步执行；新增 `--force` 覆盖；`--step=task` 时自动执行 AI 估时
- 新增 `source_path` 字段到 Request 索引（用于反向定位原始文件）

## 影响
- **受影响的代码**：
    - `kbcli/stat_builder.go`: 整文件删除（Stat 聚合逻辑）
    - `kbcli/stat_builder_test.go`: 整文件删除（Stat 测试）
    - `kbcli/es_mappings.go`: 删除 `StatIndexMapping`，更新 `TaskIndexMapping` 添加全部字段，更新 `RequestIndexMapping` 添加 `source_path`
    - `kbcli/raw_doc.go`（如有）或 `kbcli/raw_parser.go`: 删除 `StatDoc` 结构体，新增 `TaskDoc` 结构体
    - `kbcli/cmd_reindex.go`: 移除 stat 步骤（第6步），新增 task 步骤（--step=task），支持 --step 和 --force 参数
    - `kbcli/raw_parser.go`: 增加 `source_path` 字段填充
    - `kbcli/task_builder.go`: **新增文件**，Task 聚合逻辑（按 task_id 分组、数值求和、时间取极值）
    - `kbcli/task_content.go`: **新增文件**，Task 内容提取（提取 user_input + code_outputs，生成 task JSON 文件）
    - `kbcli/ai_estimator.go`: **新增文件**，AI 估时调用（Anthropic 兼容 API，生成 ai_estimated_days）
    - `kbcli/org_provider.go`: **重写**，从 YAML map 查询改为 CSV 文件加载（双 map：userIDMap + userNameMap）
    - `kbcli/config.go`: 更新 Config 结构体（新增 `org_csv_file`、重命名/重构 `ai_estimation` 配置块，移除 `org_mappings`）
    - `config.yaml`: 更新配置项（新增 `org_csv_file`，重构 `ai_estimation` 块，移除 `org_mappings`）
