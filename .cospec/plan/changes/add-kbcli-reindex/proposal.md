# 变更：实现 kbcli 命令行工具，支持 reindex 命令将 rawdata 写入 ES

## 原因
需要将 rawdata 目录下的原始 JSON 请求日志解析、转换并写入 Elasticsearch，以支持后续的指标看板查询和统计分析。

## 变更内容
- 新建 `kbcli/` Go 模块，实现 CLI 入口（仿照 comdigger/comdig 模式，无 cobra）
- 实现 `reindex --date=YYYYMMDD` 命令，处理指定日期的 rawdata，写入两个 ES 索引：
  - `costrict_chat_raw_YYYYMMDD`：每条请求一条文档（raw 层）
  - `costrict_chat_stat_YYYYMMDD`：按 project_id / user_uuid 聚合的统计文档（stat 层）
- 模型价格从 `config.yaml` 读取，支持按 model 名称查价
- 不依赖 PostgreSQL，只连接 ES

## 字段映射说明（raw 层）

| 输出字段 | 来源 |
|---|---|
| @timestamp | identity.timestamp（转 UTC） |
| caller | identity.caller |
| sender | identity.sender |
| task_id | identity.task_id |
| request_id | identity.request_id |
| client_id | identity.client_id |
| user_uuid | identity.user_info.uuid |
| username | identity.user_info.name，fallback: phone → github_name → user_name |
| repo | 暂无，留空 |
| project_path | identity.project_path |
| project_id | 若 repo 非空则为 repo，否则 client_id[:10] + ":" + project_path |
| client_ide | identity.client_ide |
| client_version | identity.client_version |
| client_os | identity.client_os |
| prompt_mode | params.llm_params.extra_body.prompt_mode |
| mode | params.llm_params.extra_body.mode |
| model | params.model |
| user_in_chars | sender=="user" 时，取 llm_params.messages 最后一条 content，若以 `<user_message>` 开头则提取标签内文本计算字符数（中文=2，英文=1），否则为 0 |
| assistant_out_code_lines | response_content.tool_calls 中 name==write_to_file 或 apply_diff 的 arguments 提取写入内容并计算行数 |
| system_tokens | tokens.original.system_tokens |
| user_tokens | tokens.original.user_tokens |
| api_request_time | timestamp（文件顶层） |
| api_end_time | timestamp + latency.total_latency_ms |
| api_process_time | latency.total_latency_ms |
| api_ttft | latency.first_token_latency_ms |
| api_in_tokens | usage.prompt_tokens |
| api_out_tokens | usage.completion_tokens |
| api_cost | (api_in_tokens/1e6)*model_in_price + (api_out_tokens/1e6)*model_out_price，价格从 config.yaml 读取 |

## stat 层聚合指标

### project 维度（按 project_id 聚合，caller=="chat"）
- project_aic_user_in_chars：user_in_chars 之和
- project_aic_assistant_out_code_lines：assistant_out_code_lines 之和
- project_aic_start_time：最早 @timestamp
- project_aic_end_time：最晚 @timestamp
- project_aic_lead_time：end_time - start_time（毫秒）
- project_aic_process_time：按 api_request_time 排序，相邻记录间隔 ≤10min 则合并计算，>10min 则断开，累加各段时长
- project_aic_api_count：记录数
- project_aic_api_in_tokens：api_in_tokens 之和
- project_aic_api_out_tokens：api_out_tokens 之和
- project_aic_api_cost：api_cost 之和

### user 维度（按 user_uuid 聚合，caller=="chat"）
- user_aic_user_in_chars
- user_aic_assistant_out_code_lines
- user_aic_start_time
- user_aic_end_time
- user_aic_lead_time
- user_aic_process_time
- user_aic_api_count
- user_aic_api_in_tokens
- user_aic_api_out_tokens
- user_aic_api_cost

## 影响
- **受影响的规范**：数据采集与 ES 写入
- **受影响的代码**：
  - `kbcli/main.go`：CLI 入口，加载 .env 和 config.yaml，分发子命令
  - `kbcli/config.go`：配置结构，含 ES 连接信息和模型价格表
  - `kbcli/cmd_root.go`：命令分发（RunCLI）
  - `kbcli/cmd_reindex.go`：reindex 命令实现
  - `kbcli/es_client.go`：ES 客户端封装（创建索引、bulk 写入）
  - `kbcli/raw_parser.go`：rawdata JSON 解析与字段提取
  - `kbcli/stat_builder.go`：stat 层聚合逻辑
  - `kbcli/go.mod`：Go 模块定义
  - `config.yaml`：新增 ES 连接配置和模型价格表
