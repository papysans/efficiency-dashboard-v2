## 实施

- [x] 1.1 重构组织信息加载：YAML → CSV 文件
     【目标对象】`kbcli/org_provider.go`
     【修改目的】将组织信息从 config.yaml 的 org_mappings（按 user_id 查 map）改为独立 CSV 文件导入，支持 user_id 和 user_name 双重匹配
     【修改方式】重写 `org_provider.go` 整个文件，删除旧的 `GetOrgInfo` 函数，新增 `OrgProvider` 结构体及相关方法
     【相关依赖】`config.yaml` 中新增 `org_csv_file` 配置项；CSV 文件格式参照 a.md 2.4 节
     【修改内容】
        - 保留 `OrgInfo` 结构体定义，移除 `yaml` tag（不再从 YAML 加载），仅保留字段名：Org1, Org2, Org3, Org4 string
        - 新增 `OrgProvider` 结构体：`userIDMap map[string]OrgInfo` + `userNameMap map[string]OrgInfo` + `csvFile string`
        - 实现 `NewOrgProvider(csvFile string) (*OrgProvider, error)` 函数：
          * 用 `encoding/csv` 读取 CSV 文件，解析表头（user_id,user_name,org1,org2,org3,org4）
          * 逐行构建双 map：user_id 非空时写入 userIDMap，user_name 非空时写入 userNameMap
          * CSV 文件不存在或解析失败时返回 error
        - 实现 `(p *OrgProvider) GetOrgInfo(userID, userName string) OrgInfo` 方法：
          * 优先按 user_id 精确匹配（user_id 非空且在 userIDMap 中）
          * 未匹配则按 user_name 精确匹配（user_name 非空且在 userNameMap 中）
          * 都未匹配返回空 OrgInfo（org1-org4 均为空字符串）
        - 实现 `(p *OrgProvider) Reload() error` 方法：重新读取 CSV 文件，替换内部 map（支持热更新）
        - 在项目根目录创建示例 CSV 文件 `org_mapping.csv`，包含 a.md 2.4 节中的示例数据

- [x] 1.2 更新配置结构体和配置文件
     【目标对象】`kbcli/config.go` + `config.yaml`
     【修改目的】适配新的组织信息加载方式（CSV）和 AI 估时配置（从 Anthropic 改为通用 AIEstimation）
     【修改方式】修改 `Config` 结构体字段和 `LoadConfig` 函数，更新 `config.yaml`
     【相关依赖】任务 1.1 的 `OrgProvider`
     【修改内容】
        - Config 结构体修改：
          * 删除 `OrgMappings map[string]OrgInfo \`yaml:"org_mappings"\`` 字段
          * 删除 `Anthropic AnthropicConfig \`yaml:"anthropic"\`` 字段
          * 删除 `TaskDataDir string \`yaml:"taskdata_dir"\`` 字段（task 文件改为存在 rawdata 下）
          * 新增 `OrgCSVFile string \`yaml:"org_csv_file"\`` 字段
          * 新增 `AIEstimation AIEstimationConfig \`yaml:"ai_estimation"\`` 字段
        - 删除 `AnthropicConfig` 结构体，新增 `AIEstimationConfig` 结构体：
          * `Enabled bool \`yaml:"enabled"\``
          * `APIKey string \`yaml:"api_key"\``
          * `BaseURL string \`yaml:"base_url"\``
          * `Model string \`yaml:"model"\``
          * `TimeoutMS int \`yaml:"timeout_ms"\``
          * `Prompt string \`yaml:"prompt"\``
        - 修改 `LoadConfig` 函数：
          * 删除 TaskDataDir 默认值设置
          * 新增 AIEstimation 默认值：timeout_ms 默认 300000，model 默认 "claude-3-5-sonnet-20241022"
          * 新增 OrgCSVFile 默认值："./org_mapping.csv"
        - 更新 `config.yaml`：
          * 删除 `org_mappings` 块及其所有子项
          * 删除 `taskdata_dir` 字段
          * 删除 `anthropic` 块
          * 新增 `org_csv_file: "./org_mapping.csv"`
          * 新增 `ai_estimation` 块（包含 enabled, api_key, base_url, model, timeout_ms, prompt 字段，值参照 a.md 9.2 节示例）

- [x] 1.3 更新 RawDoc 结构体和 Request 解析逻辑
     【目标对象】`kbcli/raw_parser.go`
     【修改目的】添加 source_path 字段用于反向定位原始文件，适配新的 OrgProvider 接口
     【修改方式】修改 `RawDoc` 结构体新增字段，修改 `ParseRawJSON` 函数签名和内部调用
     【相关依赖】任务 1.1 的 `OrgProvider`
     【修改内容】
        - `RawDoc` 结构体新增 `SourcePath string \`json:"source_path"\`` 字段
        - 修改 `ParseRawJSON` 函数签名：
          * 旧签名：`ParseRawJSON(jsonBytes []byte, modelPrices map[string]ModelPrice, orgMappings map[string]OrgInfo) (*RawDoc, error)`
          * 新签名：`ParseRawJSON(jsonBytes []byte, modelPrices map[string]ModelPrice, orgProvider *OrgProvider) (*RawDoc, error)`
        - 修改 `ParseRawJSON` 内部组织信息查询：
          * 旧代码（第156行）：`orgInfo := GetOrgInfo(raw.Identity.UserInfo.UUID, orgMappings)`
          * 新代码：`orgInfo := orgProvider.GetOrgInfo(raw.Identity.UserInfo.UUID, username)`
          * 注意：username 变量已在前面的 fallback 逻辑中计算好，直接传入即可
        - SourcePath 字段不在 ParseRawJSON 内部填充，由调用方（cmd_reindex.go）传入后赋值
          * 格式：`YYYY-MM/DD/用户UUID/文件名.json`（相对于 rawdata 目录的路径）

- [x] 1.4 更新 RequestIndexMapping 添加 source_path 字段
     【目标对象】`kbcli/es_mappings.go`
     【修改目的】在 Request 索引 mapping 中添加 source_path 字段，使 ES 中存储原始文件路径用于反向定位
     【修改方式】在 `RequestIndexMapping` 常量的 JSON properties 中新增字段
     【相关依赖】任务 1.3 的 RawDoc.SourcePath 字段
     【修改内容】
        - 在 `RequestIndexMapping` 的 properties 中（第38行 `"org4"` 之后）添加：`"source_path": { "type": "keyword" }`

- [x] 1.5 删除 Stat 层相关代码
     【目标对象】`kbcli/stat_builder.go` + `kbcli/stat_builder_test.go` + `kbcli/es_mappings.go`
     【修改目的】彻底移除 Stat 物理表相关代码，维度聚合改为实时查询
     【修改方式】删除整个文件和代码块
     【相关依赖】无（被删除的代码不被新代码依赖）
     【修改内容】
        - 删除 `kbcli/stat_builder.go` 整个文件（包含 StatDoc 结构体定义、aggregateToStatDoc 函数、BuildStatDocs 函数、calculateProcessTime 函数）
        - 删除 `kbcli/stat_builder_test.go` 整个文件
        - 删除 `kbcli/es_mappings.go` 中的 `StatIndexMapping` 常量（第43-61行），只保留 RequestIndexMapping 和 TaskIndexMapping
        - 注意：StatDoc 定义在 `stat_builder.go` 第9行（不在 `raw_parser.go` 中），随文件删除即可

- [x] 1.6 新增 TaskDoc 结构体和更新 Task 索引 mapping
     【目标对象】`kbcli/task_builder.go`（新增文件）+ `kbcli/es_mappings.go`
     【修改目的】定义 Task 层数据模型，确保 ES mapping 完整覆盖所有字段
     【修改方式】新增 `task_builder.go` 文件定义结构体，更新 `es_mappings.go` 中的 `TaskIndexMapping` 常量
     【相关依赖】a.md 4.2 节 Task 层字段定义
     【修改内容】
        - 在新文件 `kbcli/task_builder.go` 中定义 `TaskDoc` 结构体，字段如下：
          * `Timestamp time.Time \`json:"@timestamp"\``
          * `TaskID, Caller, ClientID, UserID, UserName, RepoID, ProjectPath, ProjectID string`（json tag 使用 snake_case）
          * `ClientIDE, ClientVersion, ClientOS, PromptMode, Mode string`
          * `Org1, Org2, Org3, Org4 string`
          * `UserInChars, AssistantOutCodeLines, SystemTokens, UserTokens int64`
          * `APIRequestTime, APIEndTime time.Time`
          * `APIProcessTime, APITtft, APIInTokens, APIOutTokens int64`
          * `APICost float64 \`json:"api_cost"\``
          * `APICount int64 \`json:"api_count"\``
          * `SourceFile string \`json:"source_file"\``（task 内容文件路径）
          * `AIEstimatedDays float64 \`json:"ai_estimated_days"\``
          * `AIEstimatedReason string \`json:"ai_estimated_reason"\``
        - 更新 `kbcli/es_mappings.go` 中的 `TaskIndexMapping` 常量，补充当前缺失的字段：
          * 添加 `"prompt_mode": { "type": "keyword" }`
          * 添加 `"mode": { "type": "keyword" }`
          * 添加 `"api_count": { "type": "long" }`
          * 添加 `"source_file": { "type": "keyword" }`
          * 添加 `"org1": { "type": "keyword" }`
          * 添加 `"org2": { "type": "keyword" }`
          * 添加 `"org3": { "type": "keyword" }`
          * 添加 `"org4": { "type": "keyword" }`

- [x] 1.7 实现 Task 聚合逻辑
     【目标对象】`kbcli/task_builder.go`
     【修改目的】从 Request 层数据按 task_id 归并，生成 Task 层数据
     【修改方式】在 `task_builder.go` 中新增 `BuildTaskDocs` 函数
     【相关依赖】任务 1.6 的 `TaskDoc` 结构体；`kbcli/raw_parser.go` 的 `RawDoc` 结构体
     【修改内容】
        - 实现 `BuildTaskDocs(rawDocs []RawDoc) []TaskDoc` 函数：
          * 过滤：只处理 `caller == "chat"` 的记录（与旧 BuildStatDocs 逻辑一致）
          * 按 task_id 分组：构建 `map[string][]RawDoc`
          * 对每个 task_id 组聚合生成一个 TaskDoc：
            - 标识字段（TaskID, Caller, ClientID, UserID, UserName, RepoID, ProjectPath, ProjectID, ClientIDE, ClientVersion, ClientOS, PromptMode, Mode, Org1-Org4）：取组内第一条记录的值
            - 数值字段求和：UserInChars, AssistantOutCodeLines, SystemTokens, UserTokens, APIProcessTime, APITtft, APIInTokens, APIOutTokens, APICost
            - 时间字段：APIRequestTime 取组内最早（min），APIEndTime 取组内最晚（max）
            - APICount = len(组内 RawDoc)
            - Timestamp = APIEndTime（最晚结束时间，用于 @timestamp）
            - SourceFile 和 AIEstimatedDays/AIEstimatedReason 暂不在此函数填充，由 reindex 命令流程后续步骤赋值
        - 边界处理：
          * rawDocs 为空或 nil 时返回空 slice
          * task_id 为空字符串的记录跳过（不参与聚合）
          * 单条记录的 task 也正常生成 TaskDoc（APICount=1）

- [x] 1.8 实现 Task 内容文件提取
     【目标对象】`kbcli/task_content.go`（新增文件）
     【修改目的】提取每个 task 的用户输入和 AI 代码输出，生成 task 内容 JSON 文件用于 AI 估时分析
     【修改方式】新增 `task_content.go` 文件
     【相关依赖】`kbcli/raw_parser.go` 的 RawDoc 结构体 + 原始 JSON 文件（通过 RawDoc.SourcePath 定位）
     【修改内容】
        - 定义 `TaskContentFile` 结构体（参照 a.md 9.4 节）：
          * TaskID, UserID, UserName, ProjectID string
          * StartTime, EndTime string（RFC3339 格式）
          * TotalUserInChars, TotalCodeLines int64
          * Conversations []ConversationEntry
          * AIEstimatedDays float64（可选，AI 分析后回写）
          * AIEstimatedReason string（可选，AI 分析后回写）
        - 定义 `ConversationEntry` 结构体：Timestamp, RequestID string, UserInput string, CodeOutputs []CodeOutput
        - 定义 `CodeOutput` 结构体：Path, Code string
        - 实现 `ExtractTaskContent(taskID string, rawDocs []RawDoc, rawDataDir string) (*TaskContentFile, error)` 函数：
          * 按 APIRequestTime 升序排序 rawDocs
          * 遍历每条 RawDoc，通过 SourcePath 拼接 rawDataDir 得到原始 JSON 文件完整路径
          * 重新读取原始 JSON 文件（因为 RawDoc 不保存原始内容）
          * 提取 user_input：从 `llm_params.messages[-1].content` 提取 `<user_message>` 标签内容；若无标签则直接使用 content
          * 提取 code_outputs：从 `response_content.tool_calls` 过滤 `write_to_file`/`apply_diff`，提取 path + content
          * 如果原始文件读取失败，记录警告但不中断，跳过该条记录
        - 实现 `SaveTaskContent(content *TaskContentFile, rawDataDir string) (string, error)` 函数：
          * 从 StartTime 解析出 YYYY-MM 和 DD
          * 保存到 `{rawDataDir}/YYYY-MM/DD/task_{task_id}.json`（与 a.md 5.4 节路径一致）
          * 自动创建目录（os.MkdirAll）
          * 返回相对路径（相对于 rawDataDir）：`YYYY-MM/DD/task_{task_id}.json`

- [x] 1.9 实现 AI 估时调用
     【目标对象】`kbcli/ai_estimator.go`（新增文件）
     【修改目的】调用大模型 API 分析 task 内容，生成 AI 估时结果（ai_estimated_days 和 ai_estimated_reason）
     【修改方式】新增 `ai_estimator.go` 文件
     【相关依赖】任务 1.2 的 `AIEstimationConfig`；任务 1.8 的 `TaskContentFile`
     【修改内容】
        - 实现 `EstimateTaskDays(config AIEstimationConfig, taskContent *TaskContentFile) (float64, string, error)` 函数：
          * 构建 prompt：使用 config.Prompt 模板，替换占位符 {{user_inputs}}、{{code_outputs}}、{{total_chars}}、{{total_lines}}
          * 如果 config.Prompt 为空，使用 a.md 5.2 节中定义的默认提示词
          * {{user_inputs}} 拼接方式：遍历 Conversations，每条格式为 `[时间] 用户输入内容`
          * {{code_outputs}} 拼接方式：遍历 Conversations 中的 CodeOutputs，每条格式为 `文件路径:\n代码内容`
        - 构建 HTTP 请求（Anthropic Messages API 格式）：
          * URL: `config.BaseURL + "/v1/messages"`
          * Method: POST
          * Headers: `x-api-key: config.APIKey`, `anthropic-version: 2023-06-01`, `Content-Type: application/json`
          * Body: `{"model": config.Model, "max_tokens": 1024, "messages": [{"role": "user", "content": prompt}]}`
          * Timeout: `time.Duration(config.TimeoutMS) * time.Millisecond`
        - 解析响应：
          * 从 Anthropic 响应 JSON 中提取 `content[0].text`
          * 对 text 内容进行 JSON 解析，提取 `ai_estimated_days`(float64) 和 `ai_estimated_reason`(string)
          * 如果 text 包含 markdown 代码块（```json ... ```），先去除代码块标记再解析
        - 错误处理：
          * API 超时：返回 error，包含超时信息
          * HTTP 非 200 响应：返回 error，包含状态码和响应体
          * JSON 解析失败：返回 error，包含原始响应文本
          * ai_estimated_days 为负数或不合理值（>1000）：返回 error
        - 实现 `UpdateTaskContentWithEstimation(content *TaskContentFile, days float64, reason string, filePath string) error` 函数：
          * 将 AIEstimatedDays 和 AIEstimatedReason 回写到 TaskContentFile 结构体
          * 重新序列化并写入文件（覆盖原文件）

- [x] 1.10 重构 reindex 命令流程
     【目标对象】`kbcli/cmd_reindex.go`
     【修改目的】移除 stat 步骤，新增 task 步骤，支持 --step 和 --force 参数，支持日期范围
     【修改方式】重写 `runReindex` 函数
     【相关依赖】所有前序任务（1.1-1.9）；`kbcli/es_client.go` 的 DeleteIndex 方法（任务 1.11）
     【修改内容】
        - 新增参数解析（复用已有 parseFlag/parseBoolFlag 函数）：
          * `--step=request|task`：默认不指定时执行两步（先 request 再 task）
          * `--force`：强制覆盖（先删除已有索引再创建）
          * `--start-date=YYYYMMDD` + `--end-date=YYYYMMDD`：日期范围（替代单个 --date）
          * `--date=YYYYMMDD`：保留兼容，等价于 start-date = end-date = date
        - 初始化 OrgProvider：`NewOrgProvider(config.OrgCSVFile)`，失败时打印错误并退出
        - step=request 流程（改写现有代码第42-114行）：
          * 遍历 rawdata 目录解析 JSON → RawDoc：将 `ParseRawJSON` 调用从 `config.OrgMappings` 改为 `orgProvider`
          * 填充 SourcePath 字段：计算文件相对于 rawDataDir 的路径
          * force 模式：调用 `esClient.DeleteIndex(requestIndexName)` 后再 CreateIndexIfNotExists
          * 非 force 模式：保持 CreateIndexIfNotExists（已存在则跳过）
          * BulkIndex 写入 request 层
        - step=task 流程（替换现有代码第116-151行的 stat 逻辑）：
          * 从 ES request 索引读取数据：调用 `esClient.ScrollAll(requestIndexName)` 获取 json.RawMessage 列表
          * 将 json.RawMessage 反序列化为 []RawDoc
          * 调用 `BuildTaskDocs(rawDocs)` 聚合生成 []TaskDoc
          * 对每个 TaskDoc：
            - 调用 `ExtractTaskContent` 提取 task 内容
            - 调用 `SaveTaskContent` 保存 task 内容文件，获取 sourceFile 路径
            - 如果 `config.AIEstimation.Enabled`：调用 `EstimateTaskDays` 获取估时结果，回填到 TaskDoc
            - 将 sourceFile 赋值到 TaskDoc.SourceFile
          * force 模式：先删除 task 索引再创建
          * CreateIndexIfNotExists + BulkIndex 写入 task 层
        - 删除所有 stat 相关代码：
          * 删除第91-97行的 stat 索引名变量定义
          * 删除第116-151行的 BuildStatDocs 调用和 writeStatDimension 逻辑
        - 日期范围支持：当指定 --start-date 和 --end-date 时，遍历日期范围内每一天执行上述流程

- [x] 1.11 更新 ES 客户端：新增删除索引方法
     【目标对象】`kbcli/es_client.go`
     【修改目的】支持 --force 参数需要先删除已有索引再重建
     【修改方式】在 `ESClient` 结构体上新增 `DeleteIndex` 方法
     【相关依赖】任务 1.10 的 reindex --force 流程
     【修改内容】
        - 新增 `(e *ESClient) DeleteIndex(indexName string) error` 方法：
          * 调用 `e.client.Indices.Delete([]string{indexName})` ES API
          * 检查响应状态码：200/404 均视为成功（404 表示索引本不存在，无需删除）
          * 非 200/404 时返回 error，包含状态码和响应体
          * 日志：打印 "索引 {indexName} 已删除" 或 "索引 {indexName} 不存在，跳过删除"

- [x] 1.12 更新 cmd_root.go 注册新命令
     【目标对象】`kbcli/cmd_root.go`
     【修改目的】注册 reload-org 和 validate-org 子命令
     【修改方式】在 `RunCLI` 函数的 switch 语句（第20-27行）中添加新 case
     【相关依赖】任务 1.1 的 `OrgProvider`
     【修改内容】
        - 在 switch 语句中添加 `case "reload-org":`：
          * 创建 OrgProvider 实例，调用 Reload() 方法
          * 打印加载结果（加载成功/失败，加载了多少条记录）
          * 需要在 OrgProvider 上新增 `Count() int` 方法返回 map 条目数
        - 在 switch 语句中添加 `case "validate-org":`：
          * 解析 `--csv-file` 参数（默认使用 config.OrgCSVFile）
          * 调用 `NewOrgProvider(csvFile)` 验证 CSV 文件格式
          * 打印验证结果（行数、user_id 去重数、user_name 去重数、是否有空行/异常行）
        - 更新 `printUsage` 函数（第30-38行）：
          * 在子命令列表中添加 `reload-org` 和 `validate-org` 的说明和示例

- [x] 1.13 编写单元测试
     【目标对象】`kbcli/task_builder_test.go`（新增）+ `kbcli/ai_estimator_test.go`（新增）+ `kbcli/org_provider_test.go`（新增）+ `kbcli/raw_parser_test.go`（已有，需更新）
     【修改目的】确保新增逻辑的正确性，覆盖关键分支和边界
     【修改方式】新增测试文件和更新已有测试文件
     【相关依赖】任务 1.1, 1.6, 1.7, 1.9 的实现代码
     【修改内容】
        - `kbcli/task_builder_test.go`（新增）：
          * 测试 BuildTaskDocs 正常聚合（多条相同 task_id 记录 → 正确求和/取 min/max）
          * 测试单请求 task（APICount=1）
          * 测试过滤 non-chat caller（caller != "chat" 的记录不参与聚合）
          * 测试空输入（nil/空 slice → 返回空 slice）
          * 测试 task_id 为空的记录被跳过
        - `kbcli/org_provider_test.go`（新增）：
          * 测试 CSV 正常加载（含 user_id + user_name 双字段）
          * 测试 user_id 优先匹配
          * 测试 user_id 未命中时 fallback 到 user_name 匹配
          * 测试都未匹配时返回空 OrgInfo
          * 测试 CSV 中空字段兼容（user_id 为空但 user_name 有值）
          * 测试 CSV 文件不存在时返回 error
        - `kbcli/ai_estimator_test.go`（新增）：
          * 测试 prompt 模板变量替换（{{user_inputs}}, {{code_outputs}}, {{total_chars}}, {{total_lines}}）
          * 测试响应解析：使用 httptest.NewServer 模拟 Anthropic API，返回正常 JSON
          * 测试响应解析：返回含 markdown 代码块的响应
          * 测试错误处理：API 返回非 200、JSON 解析失败、ai_estimated_days 不合理值
        - 更新 `kbcli/raw_parser_test.go`（已有）：
          * 将 ParseRawJSON 调用中的 `orgMappings map[string]OrgInfo` 参数改为 `*OrgProvider`
          * 测试中构造 OrgProvider：可用 NewOrgProvider 加载测试用 CSV，或直接构造内存 map
