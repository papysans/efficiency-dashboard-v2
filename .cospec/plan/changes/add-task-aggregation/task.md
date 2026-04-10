## 实施

- [ ] 1.1 为 ESClient 添加 ScrollAll 查询方法
     【目标对象】`kbcli/es_client.go` 的 `ESClient` 结构体
     【修改目的】支持从 ES 索引中分批查询全部文档，供 task 归并使用
     【修改方式】新增 `ScrollAll(indexName string) ([]RawDoc, error)` 方法
     【相关依赖】`github.com/elastic/go-elasticsearch/v8`，`kbcli/raw_parser.go` 的 `RawDoc` 结构体
     【修改内容】
        - 新增 `ScrollAll` 方法，返回 `[]RawDoc`（与仓库中方法签名风格一致，不使用 interface{} 传出参数）
        - 使用 ES Scroll API 发起初始搜索请求（match_all），设置 scroll 有效期 2 分钟，每批 1000 条
        - 解析 ES 响应体中的 `hits.hits[]._source` 字段，逐条反序列化为 RawDoc
        - 循环调用 Scroll API 获取后续批次，直到 `hits.hits` 为空
        - 查询完毕后调用 ClearScroll 清除 scroll 上下文
        - 错误处理：ES 请求失败、响应解析失败、状态码非 200 时返回 fmt.Errorf（遵循仓库既有的 `fmt.Errorf("中文描述: %w", err)` 风格）

- [ ] 1.2 新增 TaskIndexMapping 定义
     【目标对象】`kbcli/es_mappings.go`
     【修改目的】定义 `costrict_chat_task_YYYYMMDD` 索引的 mapping，用于存储 task 维度聚合结果
     【修改方式】新增 `TaskIndexMapping` 常量（与现有 `RequestIndexMapping`、`StatIndexMapping` 并列）
     【相关依赖】无
     【修改内容】
        - 定义 JSON mapping 字符串，字段包括：
          - 元信息字段：@timestamp(date), caller(keyword), task_id(keyword), client_id(keyword), user_id(keyword), user_name(keyword), repo_id(keyword), project_path(keyword), project_id(keyword), client_ide(keyword), client_version(keyword), client_os(keyword)
          - 聚合统计字段：user_in_chars(long), assistant_out_code_lines(long), system_tokens(long), user_tokens(long), api_request_time(date), api_end_time(date), api_process_time(long), api_ttft(long), api_in_tokens(long), api_out_tokens(long), api_cost(float)
          - AI 估算字段：ai_estimated_days(float), ai_estimated_reason(text)
        - 格式与现有 `RequestIndexMapping` 的缩进/排版风格保持一致

- [ ] 1.3 扩展配置结构
     【目标对象】`kbcli/config.go` 的 `Config` 结构体 + 项目根目录 `config.yaml`
     【修改目的】添加 Anthropic 兼容 API 配置和 taskdata 输出目录配置
     【修改方式】在 `Config` 结构体中新增字段，新增 `AnthropicConfig` 结构体；在 `LoadConfig` 函数中添加默认值；在 `config.yaml` 中添加配置段
     【相关依赖】无
     【修改内容】
        - 在 config.go 中新增 `AnthropicConfig` 结构体，字段：`BaseURL string`(yaml:base_url)、`AuthToken string`(yaml:auth_token)、`TimeoutMS int`(yaml:timeout_ms)、`Model string`(yaml:model)、`EstimatePrompt string`(yaml:estimate_prompt)
        - 在 `Config` 结构体中添加 `Anthropic AnthropicConfig`(yaml:anthropic) 和 `TaskDataDir string`(yaml:taskdata_dir) 字段
        - 在 `LoadConfig` 函数中为 `TaskDataDir` 设置默认值 `"../taskdata"`（与现有 `RawDataDir` 默认值设置风格一致）
        - 在项目根目录 `config.yaml` 中添加 anthropic 配置段（base_url, auth_token, timeout_ms, model, estimate_prompt）和 taskdata_dir 配置项

- [ ] 1.4 新建 task_content_extractor.go — 路径推算与内容提取
     【目标对象】`kbcli/task_content_extractor.go`（新建）
     【修改目的】实现通过 request_id 定位原始 JSON 文件，并提取用户输入和 AI 代码输出
     【修改方式】新建文件，包含 `CodeBlock` 结构体、`FindRawFile` 函数、`ExtractUserMessage` 函数、`ExtractAssistantCode` 函数
     【相关依赖】`kbcli/raw_parser.go` 中的 `contentToString` 函数和 `rawJSON` 结构体
     【修改内容】
        - `CodeBlock` 结构体：`{ToolName string, FilePath string, Content string}`，表示一个代码写入块
        - `FindRawFile(rawDataDir, dateStr, requestID string) (string, error)` 函数：
          - 根据 dateStr(YYYYMMDD) 构造路径 `rawDataDir/{YYYY-MM}/{DD}/`（路径构造方式与 `cmd_reindex.go` 第22-26行一致）
          - 遍历该日期目录下的所有用户子目录，用 filepath.Glob 匹配 `*_{requestID}_*.json`
          - 找到则返回完整路径，找不到返回 error
          - 边界处理：dateStr 长度不为 8 时返回错误、目录不存在时返回明确错误
        - `ExtractUserMessage(jsonBytes []byte) string` 函数：
          - 复用 `raw_parser.go` 的 `rawJSON` 结构体解析 JSON
          - 复用 `contentToString` 函数将 messages 最后一条的 Content 转为字符串
          - 从中提取 `<user_message>...</user_message>` 标签内的文本（逻辑与 `raw_parser.go` 的 `calcUserInChars` 第233-245行一致，但返回文本而非字符数）
          - 仅当 sender=user 时有效；若无 user_message 标签则返回空字符串
        - `ExtractAssistantCode(jsonBytes []byte) []CodeBlock` 函数：
          - 复用 `rawJSON` 结构体解析 JSON
          - 从 `response_content.tool_calls` 中筛选 `write_to_file`/`apply_diff` 的调用（逻辑与 `raw_parser.go` 的 `calcOutCodeLines` 第270-298行一致，但返回代码块内容而非行数）
          - 解析每个 tool_call 的 arguments JSON，提取 content 和 file_path 字段
          - 返回 `[]CodeBlock`；无匹配时返回空切片

- [ ] 1.5 新建 task_builder.go — TaskDoc 结构体与归并逻辑
     【目标对象】`kbcli/task_builder.go`（新建）
     【修改目的】定义 TaskDoc 结构体，实现按 task_id 归并 RawDoc 记录
     【修改方式】新建文件，包含 `TaskDoc`、`TaskInteraction`、`TaskMergeFile` 结构体和 `GroupByTaskID`、`AggregateToTaskDoc` 函数
     【相关依赖】`kbcli/raw_parser.go` 的 `RawDoc` 结构体，`kbcli/stat_builder.go` 的 `aggregateToStatDoc` 聚合模式
     【修改内容】
        - `TaskDoc` 结构体（json tag 与 ES mapping 字段一一对应）：
          - 元信息字段：Timestamp(time.Time, json:"@timestamp"), Caller(string), TaskID(string, json:"task_id"), ClientID(string, json:"client_id"), UserID(string, json:"user_id"), UserName(string, json:"user_name"), RepoID(string, json:"repo_id"), ProjectPath(string, json:"project_path"), ProjectID(string, json:"project_id"), ClientIDE(string, json:"client_ide"), ClientVersion(string, json:"client_version"), ClientOS(string, json:"client_os")
          - 聚合统计字段：UserInChars(int64), AssistantOutCodeLines(int64), SystemTokens(int64), UserTokens(int64), APIRequestTime(time.Time), APIEndTime(time.Time), APIProcessTime(int64), APITtft(int64), APIInTokens(int64), APIOutTokens(int64), APICost(float64)
          - AI 估算字段：AIEstimatedDays(float64, json:"ai_estimated_days"), AIEstimatedReason(string, json:"ai_estimated_reason")
        - `TaskInteraction` 结构体：`{Timestamp time.Time, Type string, Content string}`，Type 取值为 "user_input" 或 "ai_code"
        - `TaskMergeFile` 结构体：`{TaskID string, UserName string, ProjectPath string, Interactions []TaskInteraction}`，用于序列化为归并 JSON 文件
        - `GroupByTaskID(rawDocs []RawDoc) map[string][]RawDoc` 函数：
          - 只处理 caller=="chat" 的记录（与 `stat_builder.go` 第83行过滤方式一致）
          - 跳过 task_id 为空的记录
          - 按 task_id 分组返回
        - `AggregateToTaskDoc(taskID string, records []RawDoc) TaskDoc` 函数：
          - 数值字段（UserInChars, AssistantOutCodeLines, SystemTokens, UserTokens, APIInTokens, APIOutTokens, APICost）累加求和
          - APIProcessTime 和 APITtft 累加求和
          - APIRequestTime 取所有记录中最早的，APIEndTime 取所有记录中最晚的
          - Timestamp 设为最早的 APIRequestTime
          - 元信息字段（Caller, ClientID, UserID, UserName, RepoID, ProjectPath, ProjectID, ClientIDE, ClientVersion, ClientOS）取第一条记录的值
          - 聚合逻辑参考 `stat_builder.go` 的 `aggregateToStatDoc` 函数风格

- [x] 1.6 新建 ai_estimator.go — AI 人天估算
     【目标对象】`kbcli/ai_estimator.go`（新建）
     【修改目的】调用 Anthropic 兼容 API（智谱 BigModel 平台）分析归并文件，估算人天数
     【修改方式】新建文件，包含 `AIEstimator` 结构体、`NewAIEstimator` 构造函数、`EstimateResult` 结构体、`Estimate` 方法
     【相关依赖】`kbcli/config.go` 的 `AnthropicConfig`，`kbcli/task_builder.go` 的 `TaskMergeFile`
     【修改内容】
        - `AIEstimator` 结构体：持有 `*http.Client` 和 `AnthropicConfig`
        - `NewAIEstimator(config AnthropicConfig) *AIEstimator`：创建带超时的 HTTP 客户端（超时使用 config.TimeoutMS 毫秒）
        - `EstimateResult` 结构体：`{Days float64, Reason string}`
        - `Estimate(mergeFile TaskMergeFile) (*EstimateResult, error)` 方法，主流程：
          a. 构造 Anthropic Messages API 请求体：model 使用 config.Model，messages 包含 system prompt + 用户消息
          b. system prompt 优先使用 config.EstimatePrompt，若为空则使用内置默认 prompt：分析用户与 AI 编码助手的交互记录，评估该编码任务如果完全由人工完成需要多少人天，重点分析用户输入（代表需求复杂度），AI 输出的代码仅作辅助参考，输出 JSON 格式 {"days": 数字, "reason": "简要理由"}
          c. 用户消息：将 TaskMergeFile 序列化为 JSON 字符串
          d. HTTP POST 请求到 `{BaseURL}/v1/chat/completions`（注意：智谱 BigModel 平台使用 OpenAI 兼容接口，非 Anthropic 原生接口），Header 含 `Authorization: Bearer {AuthToken}` 和 `Content-Type: application/json`
          e. 解析响应 JSON，从 choices[0].message.content 中提取 JSON 结果
          f. 将提取的 JSON 反序列化为 EstimateResult
          g. 错误处理：HTTP 请求失败、响应状态码非 200、JSON 解析失败时返回 fmt.Errorf

- [ ] 1.7 新建 cmd_task.go — task 子命令入口
     【目标对象】`kbcli/cmd_task.go`（新建）
     【修改目的】实现 `kbcli task` 子命令的主流程，一条龙完成归并 + 估算 + 写入
     【修改方式】新建文件，包含 `runTask(config *Config, args []string)` 函数（与现有 `cmd_reindex.go` 的 `runReindex` 函数签名风格一致）
     【相关依赖】`kbcli/es_client.go`、`kbcli/task_builder.go`、`kbcli/task_content_extractor.go`、`kbcli/ai_estimator.go`、`kbcli/cmd_root.go` 的 `parseFlag`/`parseBoolFlag` 函数
     【修改内容】
        - `runTask(config *Config, args []string)` 函数，主流程：
          1. 使用 `parseFlag` 解析 `--date=YYYYMMDD` 参数（必填，格式校验与 `cmd_reindex.go` 第12-20行一致）
          2. 使用 `parseBoolFlag` 解析 `--no-estimate` 布尔参数
          3. 创建 ESClient，从 `costrict_chat_request_YYYYMMDD` 索引使用 `ScrollAll` 查询全部 RawDoc
          4. 调用 `GroupByTaskID` 按 task_id 分组，打印统计（共 N 个 task，M 条记录）
          5. 遍历每个 task_id 组：
             a. 调用 `AggregateToTaskDoc` 生成 TaskDoc（不含 AI 估算字段）
             b. 按 Timestamp 排序组内记录
             c. 遍历排序后的记录，通过 `FindRawFile` 定位原始 JSON 文件：
                - 对 UserInChars != 0 的记录调用 `ExtractUserMessage` 提取用户输入，生成 type="user_input" 的 TaskInteraction
                - 对 AssistantOutCodeLines != 0 的记录调用 `ExtractAssistantCode` 提取代码，生成 type="ai_code" 的 TaskInteraction
                - FindRawFile 失败时打印警告并跳过该记录（不中断流程）
             d. 组装 `TaskMergeFile`，写入 `{TaskDataDir}/{YYYY-MM}/{DD}/{task_id}.json`（目录不存在时用 os.MkdirAll 创建）
          6. 若未指定 `--no-estimate`：
             a. 创建 AIEstimator
             b. 对每个 TaskMergeFile 调用 `Estimate`，回填 TaskDoc 的 AIEstimatedDays 和 AIEstimatedReason
             c. 估算失败时打印警告并跳过（不中断流程），保留 AIEstimatedDays=0
          7. 创建 `costrict_chat_task_YYYYMMDD` 索引（使用 TaskIndexMapping），批量写入所有 TaskDoc（使用 BulkIndex）
          8. 打印完成摘要（与 `cmd_reindex.go` 第149-151行风格一致）
        - 错误处理风格：与 `cmd_reindex.go` 一致，致命错误使用 fmt.Fprintf(os.Stderr, "错误: ...") + os.Exit(1)，非致命错误使用 fmt.Printf("警告: ...")

- [ ] 1.8 注册 task 子命令到 CLI 入口
     【目标对象】`kbcli/cmd_root.go` 的 `RunCLI` 函数和 `printUsage` 函数
     【修改目的】在命令分发 switch 中添加 task 子命令
     【修改方式】在 `RunCLI` 函数的 switch 中新增 case "task"；在 `printUsage` 函数中添加说明
     【相关依赖】`kbcli/cmd_task.go` 的 `runTask` 函数
     【修改内容】
        - 在 `RunCLI` 函数的 switch subCmd 中，在 `case "reindex":` 之后添加 `case "task": runTask(config, subArgs)`
        - 在 `printUsage` 函数的子命令列表中添加 `task     按 task_id 归并请求记录并估算人天`
        - 在示例中添加 `kbcli task --date=20260331` 和 `kbcli task --date=20260331 --no-estimate`
