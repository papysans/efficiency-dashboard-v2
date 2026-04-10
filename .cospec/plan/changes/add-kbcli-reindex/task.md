## 实施

- [x] 1.1 创建 kbcli Go 模块基础结构
     【目标对象】`kbcli/go.mod`, `kbcli/main.go`
     【修改目的】建立 CLI 工具的基础框架
     【修改方式】创建 go.mod（module kanban/kbcli），main.go 加载 .env 和 config.yaml，调用 RunCLI
     【相关依赖】参考 `D:\My\PubCode\comdigger\comdig\main.go`
     【修改内容】
        - 创建 kbcli/go.mod，定义 module kanban/kbcli
        - 创建 kbcli/main.go，实现 loadDotEnv() 和 main() 入口
        - main() 中加载配置、初始化日志、调用 RunCLI

- [x] 1.2 实现配置管理模块
     【目标对象】`kbcli/config.go`, `config.yaml`
     【修改目的】支持 ES 连接配置和模型价格表
     【修改方式】定义 Config 结构体，包含 ES 配置和 ModelPrices map
     【相关依赖】参考 `D:\My\PubCode\comdigger\comdig\config.go`
     【修改内容】
        - 定义 Config 结构体（ESConfig 包含 URL/Username/Password，ModelPrices 为 map[string]struct{InPrice, OutPrice float64}）
        - 实现 LoadConfig(filename) 函数，使用 yaml.Unmarshal 解析
        - 在 config.yaml 文件末尾追加 elasticsearch 配置段（url: https://127.0.0.1:9200, username: costrict, password: costrict）
        - 在 config.yaml 文件末尾追加 model_prices 配置段（每个 model 包含 in_price 和 out_price，单位为美元/百万 token）

- [x] 1.3 实现命令分发逻辑
     【目标对象】`kbcli/cmd_root.go`
     【修改目的】根据第一个参数分发到对应子命令
     【修改方式】实现 RunCLI 函数，switch-case 分发
     【相关依赖】参考 `D:\My\PubCode\comdigger\comdig\cmd_root.go`
     【修改内容】
        - 实现 RunCLI(config) 函数
        - 实现 printUsage() 打印帮助信息
        - 实现 parseFlag/parseBoolFlag 工具函数

- [x] 1.4 实现 ES 客户端封装
     【目标对象】`kbcli/es_client.go`
     【修改目的】封装 ES 连接、索引创建、bulk 写入
     【修改方式】定义 ESClient 结构体，实现 CreateIndex、BulkIndex 方法
     【相关依赖】使用 elastic/go-elasticsearch 库
     【修改内容】
        - 定义 ESClient 结构体
        - 实现 NewESClient(config) 构造函数（忽略 SSL 证书校验）
        - 实现 CreateIndexIfNotExists(indexName, mapping) 方法
        - 实现 BulkIndex(indexName, docs) 方法

- [x] 1.5 实现 rawdata JSON 解析器
     【目标对象】`kbcli/raw_parser.go`
     【修改目的】解析 rawdata JSON 文件，提取并转换为 raw 层文档
     【修改方式】定义 RawDoc 结构体，实现 ParseRawJSON 函数
     【相关依赖】参考 `rawdata/example.json` 结构
     【修改内容】
        - 定义 RawDoc 结构体（包含 proposal.md 中所有输出字段，@timestamp 为 time.Time 类型）
        - 实现 ParseRawJSON(jsonBytes, modelPrices) 函数，解析 JSON 并填充 RawDoc
        - @timestamp 字段：优先从 identity.timestamp 解析（RFC3339 格式），若不存在则 fallback 到顶层 timestamp，转换为 UTC 时间
        - username 字段：从 identity.user_info.name 获取，若为空则按 phone → github_name → user_name 顺序 fallback
        - project_id 字段：若 repo 非空则使用 repo，否则使用 client_id[:10] + ":" + project_path
        - user_in_chars 字段：当 sender=="user" 时，取 llm_params.messages 最后一条 content，若以 <user_message> 开头则提取标签内文本，计算字符数（中文=2，英文=1），否则为 0
        - assistant_out_code_lines 字段：从 response_content.tool_calls 中提取 name=="write_to_file" 或 "apply_diff" 的 arguments，解析 JSON 提取写入内容并计算行数
        - api_request_time 字段：从顶层 timestamp 解析
        - api_end_time 字段：api_request_time + latency.total_latency_ms
        - api_cost 字段：(api_in_tokens/1e6)*model_in_price + (api_out_tokens/1e6)*model_out_price，从 modelPrices map 查询价格
        - 实现 calculateCost(model, inTokens, outTokens, prices) 辅助函数
        - 错误处理：JSON 解析失败、时间解析失败、价格未配置时返回错误

- [x] 1.6 实现 stat 层聚合构建器
     【目标对象】`kbcli/stat_builder.go`
     【修改目的】从 raw 文档聚合生成 project 和 user 维度统计
     【修改方式】实现 BuildStatDocs 函数，按 project_id 和 user_uuid 分组聚合
     【相关依赖】无
     【修改内容】
        - 定义 ProjectStatDoc 结构体，包含 project_id 和 project 维度所有指标字段（project_aic_* 前缀）
        - 定义 UserStatDoc 结构体，包含 user_uuid 和 user 维度所有指标字段（user_aic_* 前缀）
        - 实现 BuildStatDocs(rawDocs []RawDoc) ([]ProjectStatDoc, []UserStatDoc) 函数
        - 两个维度均只聚合 caller=="chat" 的记录
        - project 维度：按 project_id 分组，聚合 user_in_chars 之和、assistant_out_code_lines 之和、最早/最晚 @timestamp、api_in_tokens 之和、api_out_tokens 之和、api_cost 之和、记录数
        - user 维度：按 user_uuid 分组，聚合同上字段（user_aic_* 前缀）
        - lead_time 字段：end_time - start_time（毫秒）
        - 实现 calculateProcessTime(records []RawDoc) int64 辅助函数：
          - 按 api_request_time 升序排序
          - 相邻两条记录间隔 ≤10 分钟（600000ms）则视为连续，累加该段时长（后一条 api_end_time - 前一条 api_request_time）
          - 相邻两条记录间隔 >10 分钟则断开，各段独立计算后累加
          - 返回总 process_time（毫秒）

- [x] 1.7 实现 reindex 命令
     【目标对象】`kbcli/cmd_reindex.go`
     【修改目的】读取指定日期的 rawdata，解析后写入 ES
     【修改方式】实现 runReindex 函数
     【相关依赖】raw_parser.go, stat_builder.go, es_client.go
     【修改内容】
        - 实现 runReindex(config, args) 函数
        - 解析 --date 参数（格式 YYYYMMDD），转换为目录路径格式 YYYY-MM/DD
        - 遍历 rawdata/YYYY-MM/DD/ 目录下所有用户子目录，递归读取 .json 文件
        - 对每个 JSON 文件调用 ParseRawJSON 解析，跳过解析失败的文件并打印警告
        - 收集所有 RawDoc 后调用 BuildStatDocs 生成 stat 文档
        - 使用 CreateIndexIfNotExists 创建 raw 索引（costrict_chat_raw_YYYYMMDD）和 stat 索引（costrict_chat_stat_YYYYMMDD）
        - 调用 BulkIndex 分别写入 raw 文档和 stat 文档（project + user 维度合并写入 stat 索引）
        - 打印处理进度（已处理文件数、写入文档数）和最终汇总

- [x] 1.8 添加 ES mapping 定义
     【目标对象】`kbcli/es_mappings.go`
     【修改目的】定义 raw 和 stat 索引的 mapping
     【修改方式】定义常量字符串存储 JSON mapping
     【相关依赖】参考 rawdata/README.md 中的 ES 查询示例
     【修改内容】
        - 定义 RawIndexMapping 常量：@timestamp 为 date 类型，request_id/user_uuid/project_id/caller/sender/model/client_ide/client_os/prompt_mode/mode 等分类字段为 keyword 类型，user_in_chars/assistant_out_code_lines/system_tokens/user_tokens/api_in_tokens/api_out_tokens/api_process_time/api_ttft 等数值字段为 long/integer 类型，api_cost 为 float 类型
        - 定义 StatIndexMapping 常量：@timestamp 为 date 类型，project_id/user_uuid 为 keyword 类型，所有 *_chars/*_lines/*_tokens/*_count/*_time 为 long 类型，*_cost 为 float 类型，start_time/end_time 为 date 类型
