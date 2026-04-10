# 变更：新增 task 维度聚合与 AI 人天估算

## 原因
当前 kbcli 仅按 project/repo/user/org 维度聚合统计，缺乏按 task_id 维度的聚合能力。需要按 task_id 归并请求记录，提取用户输入与 AI 代码输出，并通过 AI 反推实现该任务所需的人天数。

## 变更内容
1. **ES 查询能力**：为 ESClient 添加 ScrollAll 查询方法，支持从 ES 索引中查询全部文档
2. **路径推算**：通过 request_id 在 rawdata 文件系统中反向定位原始 JSON 文件（文件名格式：`YYYYMMDD-HHMMSS_{request_id}_{random}.json`）
3. **Task 归并**：从 ES 的 `costrict_chat_request_YYYYMMDD` 索引按 task_id 分组，按时间排序，聚合统计字段（sum/earliest/latest），并提取用户输入（`<user_message>` 标签内容）和 AI 代码输出（`write_to_file`/`apply_diff` 工具调用的 content）
4. **归并文件输出**：输出到 `taskdata/{年-月}/{日}/{task_id}.json`，包含按时间排序的用户输入和 AI 代码交互序列
5. **AI 人天估算**：调用 Anthropic 兼容 API（智谱 BigModel 平台），基于可配置提示词分析归并文件，估算人天数（ai_estimated_days）并给出理由（ai_estimated_reason）
6. **Task 索引写入**：创建 `costrict_chat_task_YYYYMMDD` 索引，写入包含聚合统计 + AI 估算结果的 TaskDoc
7. **新增子命令**：`kbcli task --date=YYYYMMDD [--no-estimate]`，一条龙完成归并 + 估算 + 写入
8. **配置扩展**：在 config.yaml 中新增 `anthropic` 和 `taskdata_dir` 配置段

## 影响
- **受影响的代码**：
  - `kbcli/es_client.go`: 新增 ScrollAll 查询方法
  - `kbcli/es_mappings.go`: 新增 TaskIndexMapping 定义
  - `kbcli/config.go`: 新增 AnthropicConfig 结构和 TaskDataDir 字段
  - `kbcli/cmd_root.go`: 注册 task 子命令
  - `kbcli/cmd_task.go`: **新建** — task 子命令入口和主流程
  - `kbcli/task_builder.go`: **新建** — TaskDoc 结构体 + 按 task_id 归并逻辑
  - `kbcli/task_content_extractor.go`: **新建** — 从原始 JSON 提取用户输入和 AI 代码的逻辑 + 路径推算
  - `kbcli/ai_estimator.go`: **新建** — Anthropic API 调用 + 提示词管理
  - `config.yaml`: 新增 anthropic 配置段和 taskdata_dir 配置
