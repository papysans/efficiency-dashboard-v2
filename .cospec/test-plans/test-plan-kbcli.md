# 测试方案：kbcli — rawdata 解析与 stat 聚合

> 生成日期：2026-04-01  
> 覆盖模块：`raw_parser.go`、`stat_builder.go`、`config.go`

---

## 概述

kbcli 是一个 CLI 工具，负责将 rawdata 目录下的 JSON 请求日志解析后批量写入 Elasticsearch。
核心功能分三层：

1. **配置加载**（`LoadConfig`）：解析 YAML 获取 ES 连接信息和模型价格
2. **JSON 解析**（`ParseRawJSON`）：将单个 rawdata JSON 文件转换为 `RawDoc`
3. **stat 聚合**（`BuildStatDocs` + `calculateProcessTime`）：按 project/user 维度聚合 raw 记录

测试策略以**单元测试**为主，优先覆盖核心计算逻辑（中英文字符计数、process_time 断点合并、
cost 计算），兼顾边界值与异常输入；ES 集成测试作为验收补充，不纳入自动化 CI。

---

## 运行测试

```powershell
# 在 kbcli 目录下执行
cd D:\My\PubCode\kanban\kbcli
go test ./... -v

# 查看覆盖率
go test ./... -cover

# 运行指定测试
go test -run TestCountChars -v
go test -run TestCalculateProcessTime -v
```

---

## 测试点列表

### 模块一：配置加载（`config.go`）

#### 1. 正常加载 YAML 配置
- **类型**: unit
- **描述**: 验证 `LoadConfig` 能正确解析完整 YAML，所有字段均映射到 Config 结构体
- **测试场景**: 
  - 输入：包含 elasticsearch、model_prices、rawdata_dir 的完整 YAML 临时文件
  - 操作：调用 `LoadConfig(filename)`
- **预期结果**: ES URL/Username/Password 正确、ModelPrices 包含指定模型及价格、RawDataDir 正确
- **测试用例文件**: `config_test.go` → `TestLoadConfig_Normal`

#### 2. rawdata_dir 未配置时使用默认值
- **类型**: unit
- **描述**: YAML 中未写 rawdata_dir 字段时，默认值应为 `../rawdata`
- **测试场景**: YAML 不含 rawdata_dir 字段，调用 `LoadConfig`
- **预期结果**: `cfg.RawDataDir == "../rawdata"`
- **测试用例文件**: `config_test.go` → `TestLoadConfig_DefaultRawDataDir`

#### 3. 文件不存在 → 返回 error
- **类型**: unit
- **描述**: 传入不存在的路径时 LoadConfig 应返回 error 而非 panic
- **测试场景**: 路径 `/nonexistent/path/config.yaml`
- **预期结果**: `err != nil`
- **测试用例文件**: `config_test.go` → `TestLoadConfig_FileNotFound`

#### 4. YAML 格式非法 → 返回 error
- **类型**: unit
- **描述**: 文件内容不是有效 YAML 时应返回解析错误
- **测试场景**: 写入 `"invalid: yaml: content: [unclosed"` 
- **预期结果**: `err != nil`
- **测试用例文件**: `config_test.go` → `TestLoadConfig_InvalidYAML`

#### 5. 多模型价格正确加载
- **类型**: unit
- **描述**: 验证含 4 个模型价格条目的 YAML 全部正确解析
- **测试场景**: GLM-4.7、GLM-5、Kimi-K2.5-Moonshot、Auto 四个模型
- **预期结果**: `len(ModelPrices) == 4`，各模型 in_price/out_price 正确
- **测试用例文件**: `config_test.go` → `TestLoadConfig_MultipleModelPrices`

---

### 模块二：JSON 解析 — 基础字段（`raw_parser.go`）

#### 6. 正常场景 — 字段全量验证
- **类型**: unit
- **描述**: 完整 JSON 能正确提取 task_id、model、system_tokens、api_process_time 等基础字段
- **测试场景**: 标准测试 JSON，含所有必填字段
- **预期结果**: 各字段值与输入一一对应
- **测试用例文件**: `raw_parser_test.go` → `TestParseRawJSON_Normal`

#### 7. timestamp 解析 — RFC3339Nano 格式并转 UTC
- **类型**: unit
- **描述**: 带纳秒和时区偏移（+08:00）的 timestamp 应解析成功并转为 UTC
- **测试场景**: `"2026-03-31T09:39:02.474731526+08:00"` → UTC = 01:39:02
- **预期结果**: `doc.Timestamp.Location() == time.UTC`，hour == 1
- **测试用例文件**: `raw_parser_test.go` → `TestParseRawJSON_TimestampRFC3339Nano`

#### 8. timestamp 解析 — RFC3339 格式（无纳秒）
- **类型**: unit
- **描述**: 无纳秒的 RFC3339 格式也能正确解析
- **测试场景**: `"2026-03-31T09:39:02+08:00"`
- **预期结果**: `!doc.Timestamp.IsZero()`
- **测试用例文件**: `raw_parser_test.go` → `TestParseRawJSON_TimestampRFC3339`

#### 9. timestamp 非法格式 → error
- **类型**: unit
- **描述**: 完全错误的时间字符串应返回解析错误
- **测试场景**: timestamp = `"not-a-time"`
- **预期结果**: `err != nil`，错误信息包含 "timestamp"
- **测试用例文件**: `raw_parser_test.go` → `TestParseRawJSON_InvalidTimestamp`

#### 10. JSON 格式非法 → error
- **类型**: unit
- **描述**: 非 JSON 字节串不应导致 panic
- **测试场景**: 输入 `{invalid json}`
- **预期结果**: `err != nil`
- **测试用例文件**: `raw_parser_test.go` → `TestParseRawJSON_InvalidJSON`

#### 11. api_end_time = api_request_time + total_latency_ms
- **类型**: unit
- **描述**: 验证 end_time 计算公式
- **测试场景**: latency.total_latency_ms = 18040
- **预期结果**: `doc.APIEndTime - doc.APIRequestTime == 18040ms`
- **测试用例文件**: `raw_parser_test.go` → `TestParseRawJSON_EndTimeCalculation`

---

### 模块三：username fallback 逻辑

#### 12. name 非空 → 使用 name
- **类型**: unit
- **描述**: user_info.name 有值时应优先使用
- **预期结果**: `doc.Username == "正式姓名"`
- **测试用例文件**: `raw_parser_test.go` → `TestUsernameFromName`

#### 13. name 为空 → fallback 到 phone
- **类型**: unit
- **描述**: name 为空时取 user_info.phone
- **预期结果**: `doc.Username == "13800000001"`
- **测试用例文件**: `raw_parser_test.go` → `TestUsernameFromPhone`

#### 14. name/phone 均空 → fallback 到 github_name
- **类型**: unit
- **预期结果**: `doc.Username == "gh_user"`
- **测试用例文件**: `raw_parser_test.go` → `TestUsernameFromGithubName`

#### 15. 全部为空 → fallback 到 user_name
- **类型**: unit
- **描述**: 四级 fallback 最终使用 identity.user_name
- **预期结果**: `doc.Username == "fallback_user"`
- **测试用例文件**: `raw_parser_test.go` → `TestUsernameFromUserName`

---

### 模块四：project_id 计算

#### 16. project_id = client_id 前10字符 + ":" + project_path
- **类型**: unit
- **描述**: client_id 长度 ≥ 10 时取前10字符
- **测试场景**: client_id=`"clientid1234567890abcdef"` → prefix=`"clientid12"`
- **预期结果**: `doc.ProjectID == "clientid12:/workspace/proj"`
- **测试用例文件**: `raw_parser_test.go` → `TestProjectID_FromClientIDAndPath`

#### 17. client_id 短于10字符时取全部
- **类型**: unit（边界值）
- **测试场景**: client_id=`"abc"` (3字符)
- **预期结果**: `doc.ProjectID == "abc:/proj"`
- **测试用例文件**: `raw_parser_test.go` → `TestProjectID_ShortClientID`

---

### 模块五：user_in_chars 计算

#### 18. sender=user，含 `<user_message>` 标签，中英文混合
- **类型**: unit
- **描述**: 从最后一条消息的 `<user_message>` 标签内提取文本计算字符数
- **测试场景**: 消息内容 `<user_message>hello world</user_message>`（10个可见字符）
- **预期结果**: `calcUserInChars("user", msgs) == 10`
- **测试用例文件**: `raw_parser_test.go` → `TestCalcUserInChars_Normal`

#### 19. 纯中文消息（每字计2）
- **类型**: unit
- **测试场景**: `<user_message>你好</user_message>` → 2字×2 = 4
- **预期结果**: `== 4`
- **测试用例文件**: `raw_parser_test.go` → `TestCalcUserInChars_CJK`

#### 20. 消息无 `<user_message>` 标签 → 返回0
- **类型**: unit（边界值）
- **预期结果**: `== 0`
- **测试用例文件**: `raw_parser_test.go` → `TestCalcUserInChars_NoTag`

#### 21. sender=system → 返回0（不提取）
- **类型**: unit
- **描述**: system 消息不统计用户输入字符
- **预期结果**: `== 0`
- **测试用例文件**: `raw_parser_test.go` → `TestCalcUserInChars_SenderSystem`

#### 22. messages 为空 → 返回0
- **类型**: unit（边界值）
- **预期结果**: `== 0`
- **测试用例文件**: `raw_parser_test.go` → `TestCalcUserInChars_EmptyMessages`

#### 23. 无结束标签时提取到末尾
- **类型**: unit（边界值）
- **测试场景**: `<user_message>abc`（无 `</user_message>`）
- **预期结果**: `== 3`
- **测试用例文件**: `raw_parser_test.go` → `TestCalcUserInChars_NoEndTag`

#### 24. content 为 `[]object` 格式（非字符串）
- **类型**: unit
- **描述**: 部分消息 content 是 `[{"type":"text","text":"..."}]` 数组格式
- **预期结果**: 正确提取 text 字段内容后计算字符数
- **测试用例文件**: `raw_parser_test.go` → `TestCalcUserInChars_ArrayContent`

---

### 模块六：countChars 字符计数函数

#### 25. 纯 ASCII 字母计数
- **类型**: unit
- **测试场景**: `"hello"` → 5
- **测试用例文件**: `raw_parser_test.go` → `TestCountChars_ASCII`

#### 26. 空格不计入
- **类型**: unit（边界值）
- **测试场景**: `"a b"` → 2（空格=0x20，不满足 `> 0x20`）
- **测试用例文件**: `raw_parser_test.go` → `TestCountChars_SpaceNotCounted`

#### 27. 中文字符每字计2
- **类型**: unit
- **测试场景**: `"中文"` → 4
- **测试用例文件**: `raw_parser_test.go` → `TestCountChars_CJK`

#### 28. 空字符串 → 0
- **类型**: unit（边界值）
- **测试用例文件**: `raw_parser_test.go` → `TestCountChars_Empty`

---

### 模块七：assistant_out_code_lines 计算

#### 29. write_to_file tool_call 统计行数
- **类型**: unit
- **测试场景**: content=`"line1\nline2\nline3"` → 3行
- **测试用例文件**: `raw_parser_test.go` → `TestCalcOutCodeLines_WriteToFile`

#### 30. apply_diff tool_call 统计行数
- **类型**: unit
- **测试场景**: content=`"a\nb\nc\nd"` → 4行
- **测试用例文件**: `raw_parser_test.go` → `TestCalcOutCodeLines_ApplyDiff`

#### 31. 非写文件 tool_call 不计入
- **类型**: unit
- **测试场景**: tool_call name=`"read_file"`
- **预期结果**: `== 0`
- **测试用例文件**: `raw_parser_test.go` → `TestCalcOutCodeLines_OtherToolIgnored`

#### 32. 末尾 `\n` 不计空行
- **类型**: unit（边界值）
- **测试场景**: content=`"line1\nline2\n"` → 2（尾部空串不算）
- **测试用例文件**: `raw_parser_test.go` → `TestCalcOutCodeLines_TrailingNewline`

#### 33. 多个 tool_call 累加
- **类型**: unit
- **测试场景**: write_to_file 2行 + apply_diff 3行 → 5
- **测试用例文件**: `raw_parser_test.go` → `TestCalcOutCodeLines_MultipleToolCalls`

#### 34. tool_calls 为空 → 0
- **类型**: unit（边界值）
- **测试用例文件**: `raw_parser_test.go` → `TestCalcOutCodeLines_Empty`

---

### 模块八：api_cost 计算

#### 35. 已知模型正常计费
- **类型**: unit
- **测试场景**: GLM-4.7，1M in_tokens + 500K out_tokens → 0.5 + 0.5 = 1.0
- **测试用例文件**: `raw_parser_test.go` → `TestCalculateCost_KnownModel`

#### 36. 未知模型返回 0
- **类型**: unit
- **测试场景**: model=`"Unknown-Model"`
- **预期结果**: `== 0`
- **测试用例文件**: `raw_parser_test.go` → `TestCalculateCost_UnknownModel`

#### 37. Auto 模型（价格为0）返回 0
- **类型**: unit
- **测试场景**: model=`"Auto"`，in_price=out_price=0
- **预期结果**: `== 0`
- **测试用例文件**: `raw_parser_test.go` → `TestCalculateCost_AutoModel`

#### 38. tokens 为 0 → 0
- **类型**: unit（边界值）
- **测试用例文件**: `raw_parser_test.go` → `TestCalculateCost_ZeroTokens`

#### 39. api_cost 集成验证（通过 ParseRawJSON）
- **类型**: integration
- **描述**: 通过完整 JSON 解析路径验证 APICost 字段最终值
- **测试场景**: GLM-4.7，usage.prompt_tokens=1M，completion_tokens=0 → cost=0.5
- **测试用例文件**: `raw_parser_test.go` → `TestParseRawJSON_APICostIntegration`

---

### 模块九：真实数据场景

#### 40. sender=system 时 user_in_chars=0
- **类型**: integration
- **描述**: 基于真实日志数据结构，验证 system sender 不统计输入字符
- **测试场景**: 参考真实文件 `13003666923/20260331-113727_*.json`（sender=system）
- **预期结果**: `doc.UserInChars == 0`
- **测试用例文件**: `raw_parser_test.go` → `TestParseRawJSON_SenderSystem`

#### 41. usage 全为0（报错/quota 不足场景）
- **类型**: integration
- **描述**: 真实数据中存在 quota 不足导致 usage 全0 的记录，cost 应为 0
- **测试场景**: `usage.prompt_tokens=0, completion_tokens=0`
- **预期结果**: `doc.APICost == 0`
- **测试用例文件**: `raw_parser_test.go` → `TestParseRawJSON_ZeroUsage`

---

### 模块十：stat 聚合（`stat_builder.go`）

#### 42. 单条 chat 记录 → 各生成1条 project/user stat
- **类型**: unit
- **测试用例文件**: `stat_builder_test.go` → `TestBuildStatDocs_SingleRecord`

#### 43. caller≠chat 的记录不参与聚合
- **类型**: unit
- **描述**: caller=`"task"` 的记录应被过滤
- **预期结果**: `len(pStats)==0, len(uStats)==0`
- **测试用例文件**: `stat_builder_test.go` → `TestBuildStatDocs_NonChatCallerIgnored`

#### 44. 空输入 → 空输出
- **类型**: unit（边界值）
- **测试用例文件**: `stat_builder_test.go` → `TestBuildStatDocs_EmptyInput`

#### 45. 相同 project_id 多条记录 → project 维度累加聚合
- **类型**: unit
- **测试场景**: 2条同 project_id 记录，user_in_chars 分别10/20，api_cost 分别0.5/1.0
- **预期结果**: 聚合后 user_in_chars=30，api_cost=1.5，api_count=2
- **测试用例文件**: `stat_builder_test.go` → `TestBuildStatDocs_ProjectAggregation`

#### 46. start_time/end_time/lead_time 计算
- **类型**: unit
- **描述**: start_time 取最早 Timestamp，end_time 取最晚，lead_time=end-start（毫秒）
- **测试用例文件**: `stat_builder_test.go` → `TestBuildStatDocs_StartEndTime`

#### 47. 不同 project_id → 各自独立一条 stat
- **类型**: unit
- **测试场景**: 3条不同 project_id，同一 user
- **预期结果**: `len(pStats)==3, len(uStats)==1`
- **测试用例文件**: `stat_builder_test.go` → `TestBuildStatDocs_MultipleProjects`

#### 48. 单条记录时 lead_time = 0
- **类型**: unit（边界值）
- **测试用例文件**: `stat_builder_test.go` → `TestBuildStatDocs_SingleRecord_LeadTimeZero`

#### 49. chat+task 混合记录 → 只有 chat 计入 stat
- **类型**: unit
- **测试场景**: 1条 chat + 1条 task，同 project_id
- **预期结果**: 聚合结果仅含 chat 记录的数据
- **测试用例文件**: `stat_builder_test.go` → `TestBuildStatDocs_MixedCaller`

---

### 模块十一：process_time 计算（`calculateProcessTime`）

#### 50. 单条记录 → process_time = end - start
- **类型**: unit
- **测试场景**: req=09:00, end=09:00:05 → 5000ms
- **测试用例文件**: `stat_builder_test.go` → `TestCalculateProcessTime_Single`

#### 51. 空输入 → 0
- **类型**: unit（边界值）
- **测试用例文件**: `stat_builder_test.go` → `TestCalculateProcessTime_Empty`

#### 52. 间隔 < 10 分钟 → 合并为一个段
- **类型**: unit
- **测试场景**: end1=09:01, req2=09:05（间隔4分钟）→ 合并，总=end2-req1=7min
- **测试用例文件**: `stat_builder_test.go` → `TestCalculateProcessTime_GapLessThan10Min`

#### 53. 间隔 > 10 分钟 → 断开，各自独立
- **类型**: unit
- **测试场景**: end1=09:01, req2=09:15（间隔14分钟）→ 断开，总=1+2=3min
- **测试用例文件**: `stat_builder_test.go` → `TestCalculateProcessTime_GapMoreThan10Min`

#### 54. 恰好 10 分钟间隔（边界值）→ 合并
- **类型**: unit（边界值）
- **描述**: gap = 10*60*1000ms（≤maxGapMS），应合并
- **测试用例文件**: `stat_builder_test.go` → `TestCalculateProcessTime_ExactlyAt10Min`

#### 55. 3条记录，前两合并、第3条断开
- **类型**: unit
- **测试场景**: 段1(09:00-09:06) + 段2(09:30-09:32) = 8min = 480000ms
- **测试用例文件**: `stat_builder_test.go` → `TestCalculateProcessTime_ThreeRecords_SomeGap`

#### 56. 记录无序输入 → 自动排序后正确计算
- **类型**: unit
- **描述**: 倒序输入两条记录，结果应与有序输入相同
- **测试用例文件**: `stat_builder_test.go` → `TestCalculateProcessTime_UnorderedInput`

#### 57. 并发请求（时间重叠）→ segEnd 取最大值
- **类型**: unit（边界值）
- **描述**: req2 在 req1 进行中到来，end2 < end1，segEnd 应保持 end1
- **测试场景**: req1=09:00,end1=09:05; req2=09:02,end2=09:04 → process=5min
- **测试用例文件**: `stat_builder_test.go` → `TestCalculateProcessTime_OverlappingRequests`

---

### 模块十二：ES 集成验收（手动执行）

> 以下测试需要 ES 实例可用，不纳入 CI，由开发者手动验证。

#### 58. reindex 命令写入 raw 层验收
- **类型**: integration（手动）
- **描述**: 执行 `kbcli reindex --date=20260331`，验证 ES 索引字段完整性
- **验证方式**:
  ```bash
  # 查询 raw 索引的一条文档
  curl -k -u costrict:costrict https://127.0.0.1:9200/costrict_chat_raw_20260331/_search?size=1
  ```
  - 检查字段：`@timestamp`, `username`, `project_id`, `model`, `api_cost`, `user_in_chars`
  - 验证 `username` 非空（不应为空字符串）
  - 验证 `project_id` 格式为 `{10字符前缀}:{project_path}`

#### 59. reindex 命令写入 stat 层验收
- **类型**: integration（手动）
- **描述**: 验证 stat 索引聚合正确
- **验证方式**:
  ```bash
  curl -k -u costrict:costrict https://127.0.0.1:9200/costrict_chat_stat_20260331/_search?size=5
  ```
  - 检查 project stat：`project_aic_api_count >= 1`
  - 检查 user stat：`user_aic_api_count >= 1`
  - 验证 `project_aic_process_time >= 0`
  - 验证 caller=system 的记录对应 user_in_chars=0

---

## 关键考虑事项

1. **countChars 空格处理**：空格（0x20）不满足 `r > 0x20` 条件，不计入字符数。制表符、换行等控制字符同理。中文标点（如：，。）属于 CJK 范围以外，需确认实际表现。

2. **calculateProcessTime 的 gap 判断**：gap 使用 `curr.APIRequestTime - segEnd`，而非 `curr.APIRequestTime - prevRequest`。当请求有时间重叠（并发）时，gapMS 为负数，负数 ≤ maxGapMS，会正确合并。

3. **project_id 的 client_id 截断**：使用 `[]rune` 切片保证 Unicode 字符安全截断，不会截断多字节字符。

4. **timestamp fallback 解析顺序**：先尝试 RFC3339Nano，再 RFC3339。真实数据中常见带纳秒的格式（如 `2026-03-31T09:39:02.474731526+08:00`）。

5. **sender=system 与 user_in_chars**：真实数据中多数记录 sender=system（代表系统触发），此时 user_in_chars=0，只有 sender=user 才提取 `<user_message>` 内容。

6. **BuildStatDocs 的 caller 过滤**：仅 `caller=="chat"` 的记录进入聚合。`caller=="task"` 等其他类型被忽略，不产生任何 stat 文档。

7. **ES 测试依赖外部服务**：ES 集成测试依赖 `https://127.0.0.1:9200` 且忽略 SSL 证书，仅适合本地开发环境手动执行，不应加入自动化 CI。

---

## 测试用例文件清单

| 文件 | 覆盖模块 | 测试函数数 |
|------|---------|-----------|
| `kbcli/config_test.go` | config.go → LoadConfig | 6 |
| `kbcli/raw_parser_test.go` | raw_parser.go → ParseRawJSON、calcUserInChars、countChars、calcOutCodeLines、calculateCost | 31 |
| `kbcli/stat_builder_test.go` | stat_builder.go → BuildStatDocs、calculateProcessTime | 16 |

**合计：53 个自动化测试用例，覆盖正常/边界/异常共57个测试点（其中4个为手动集成验收）。**

- `kbcli/config_test.go`
- `kbcli/raw_parser_test.go`
- `kbcli/stat_builder_test.go`
