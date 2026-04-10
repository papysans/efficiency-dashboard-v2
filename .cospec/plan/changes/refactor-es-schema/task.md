## 实施

- [x] 1.1 更新 raw 层 ES Mapping 中的字段名
     【目标对象】`kbcli/es_mappings.go`
     【修改目的】将 user_uuid → user_id，username → user_name，repo（不在mapping中，需要新增 repo_id）
     【修改方式】直接修改 RawIndexMapping JSON 常量中的字段名
     【相关依赖】无
     【修改内容】
        - 将 `"user_uuid"` → `"user_id"`
        - 将 `"username"` → `"user_name"`
        - 新增 `"repo_id": { "type": "keyword" }`（原 repo 字段在 mapping 中不存在，需要新增）
        - 将 StatIndexMapping 替换为通用的单一模板 `StatIndexMapping`，字段只有 `union_id (keyword)` + `@timestamp (date)` + `aic_start_time/aic_end_time (date)` + `aic_user_in_chars/aic_assistant_out_code_lines/aic_lead_time/aic_process_time/aic_api_count/aic_api_in_tokens/aic_api_out_tokens (long)` + `aic_api_cost (float)`

- [x] 1.2 更新 RawDoc 结构体字段名及 json tag
     【目标对象】`kbcli/raw_parser.go`
     【修改目的】RawDoc 中 Go 字段名和 json tag 同步重命名
     【修改方式】修改结构体定义和赋值代码
     【相关依赖】stat_builder.go 中使用 RawDoc 字段
     【修改内容】
        - `UserUUID string json:"user_uuid"` → `UserID string json:"user_id"`
        - `Username string json:"username"` → `UserName string json:"user_name"`
        - `Repo string json:"repo"` → `RepoID string json:"repo_id"`
        - 对应更新 doc 赋值部分：`UserUUID: raw.Identity.UserInfo.UUID` → `UserID: raw.Identity.UserInfo.UUID`，`Username: username` → `UserName: username`，`Repo: repo` → `RepoID: repo`

- [x] 1.3 统一 StatDoc 结构体为通用结构，更新 BuildStatDocs 逻辑
     【目标对象】`kbcli/stat_builder.go`
     【修改目的】将 6 个各自独立的 XxxStatDoc 合并为 1 个通用 StatDoc，字段名统一为 aic_xxx，新增 union_id 字段；BuildStatDocs 返回按维度分组的 []StatDoc；org 维度的 union_id 使用 org1_org2_org3_org4 等复合 key
     【修改方式】重写结构体定义和 BuildStatDocs 函数
     【相关依赖】cmd_reindex.go（消费返回值），stat_builder_test.go（测试）
     【修改内容】
        - 删除 ProjectStatDoc/UserStatDoc/Org1StatDoc/Org2StatDoc/Org3StatDoc/Org4StatDoc
        - 定义通用 `StatDoc` 结构体，字段：`Timestamp time.Time json:"@timestamp"`，`UnionID string json:"union_id"`，`AICUserInChars int64 json:"aic_user_in_chars"`，`AICAssistantOutCodeLines int64 json:"aic_assistant_out_code_lines"`，`AICStartTime time.Time json:"aic_start_time"`，`AICEndTime time.Time json:"aic_end_time"`，`AICLeadTime int64 json:"aic_lead_time"`，`AICProcessTime int64 json:"aic_process_time"`，`AICAPICount int64 json:"aic_api_count"`，`AICAPIInTokens int64 json:"aic_api_in_tokens"`，`AICAPIOutTokens int64 json:"aic_api_out_tokens"`，`AICAPICost float64 json:"aic_api_cost"`
        - 修改 BuildStatDocs 返回 `(project, repo, user, org1, org2, org3, org4 []StatDoc)`（7 个维度）
        - project 维度：union_id = doc.ProjectID，key 按 ProjectID 分组
        - repo 维度：union_id = doc.RepoID，key 按 RepoID 分组（跳过空 RepoID）
        - user 维度：union_id = doc.UserID（原 UserUUID），key 按 UserID 分组
        - org1 维度：union_id = org1，key 按 Org1 分组
        - org2 维度：union_id = org1_org2（下划线拼接），key 按 org1+"_"+org2 分组
        - org3 维度：union_id = org1_org2_org3，key 按 org1+"_"+org2+"_"+org3 分组
        - org4 维度：union_id = org1_org2_org3_org4，key 按 org1+"_"+org2+"_"+org3+"_"+org4 分组
        - 保持 calculateProcessTime 函数不变（仍使用 []RawDoc）

- [x] 1.4 更新 cmd_reindex.go：stat 索引分维度写入 7 个独立索引
     【目标对象】`kbcli/cmd_reindex.go`
     【修改目的】将 stat 写入逻辑从单一索引改为按维度写入 7 个独立索引
     【修改方式】修改 stat 索引创建和写入部分
     【相关依赖】BuildStatDocs 新返回值、StatIndexMapping
     【修改内容】
        - 删除旧 `statIndexName := fmt.Sprintf("costrict_chat_stat_%s", dateStr)` 
        - 新增 7 个索引名变量：`statProjectIdx`, `statRepoIdx`, `statUserIdx`, `statOrg4Idx`, `statOrg3Idx`, `statOrg2Idx`, `statOrg1Idx`，命名格式 `costrict_chat_stat_project_YYYYMMDD` 等
        - 为每个非空的维度 stat 列表单独调用 CreateIndexIfNotExists + BulkIndex
        - 更新 BuildStatDocs 调用（新签名返回 7 个切片）
        - 更新打印日志，增加 repo 维度的统计

- [x] 1.5 更新 stat_builder_test.go 中的字段引用
     【目标对象】`kbcli/stat_builder_test.go`
     【修改目的】测试中使用了旧字段名（UserUUID、ProjectAICAPICount 等）和旧 BuildStatDocs 6返回值签名，需要同步更新
     【修改方式】修改 makeDoc 辅助函数体中的字段赋值、所有测试函数中的断言字段名及 BuildStatDocs 调用
     【相关依赖】RawDoc 新字段名（UserID，来自任务 1.2）、StatDoc 新字段名（AICAPICount 等，来自任务 1.3）
     【修改内容】
        - `makeDoc` 函数（第10-25行）：将函数体内 `UserUUID: userUUID` 改为 `UserID: userUUID`（参数名 userUUID 保持不变，仅修改结构体字段赋值）
        - 所有测试中 `BuildStatDocs` 调用：从 6 返回值 `pStats, uStats, _, _, _, _` 改为 7 返回值 `pStats, _, uStats, _, _, _, _`（新增 repo 维度作为第2返回值，user 变为第3）
        - 所有 project stat 断言：`p.ProjectAICAPICount` → `p.AICAPICount`，`p.ProjectAICUserInChars` → `p.AICUserInChars`，`p.ProjectAICAssistantOutCodeLines` → `p.AICAssistantOutCodeLines`，`p.ProjectAICAPIInTokens` → `p.AICAPIInTokens`，`p.ProjectAICAPICost` → `p.AICAPICost`，`p.ProjectAICStartTime` → `p.AICStartTime`，`p.ProjectAICEndTime` → `p.AICEndTime`，`p.ProjectAICLeadTime` → `p.AICLeadTime`
        - 所有 user stat 断言：`uStats[0].UserAICAPICount` → `uStats[0].AICAPICount` 等（同上规则，去除 User 前缀）

- [x] 1.6 更新 raw_parser_test.go 中的字段断言
     【目标对象】`kbcli/raw_parser_test.go`
     【修改目的】测试中直接断言 `doc.Username`，随 RawDoc.Username → UserName 重命名后编译失败，需同步更新
     【修改方式】修改所有引用了 `doc.Username` 字段的断言语句（共5处：TestParseRawJSON_Normal 第97行、TestUsernameFromName 第202行、TestUsernameFromPhone 第226行、TestUsernameFromGithubName 第250行、TestUsernameFromUserName 第274行）
     【相关依赖】RawDoc 新字段名（UserName，来自任务 1.2）
     【修改内容】
        - 将所有 `doc.Username` → `doc.UserName`（共5处，仅改字段名，期望值字符串不变）
        - 对应的错误提示字符串中 `"Username:"` → `"UserName:"` 以保持可读性

- [x] 1.7 更新 org_provider.go 中函数注释
     【目标对象】`kbcli/org_provider.go`
     【修改目的】GetOrgInfo 函数注释与参数名使用 `user_uuid`，与重命名后的语义（userID）不一致，需更新注释以对齐新命名规范
     【修改方式】修改 GetOrgInfo 函数的注释行（第11行）
     【相关依赖】无（函数签名接收字符串值，与 ES 字段名无关，仅注释需更新）
     【修改内容】
        - 将注释 `// GetOrgInfo 根据 user_uuid 查询组织信息` → `// GetOrgInfo 根据 user_id 查询组织信息`
        - 注意：函数参数名 `userUUID string` 建议同步改为 `userID string`，内部 `orgMappings[userUUID]` 改为 `orgMappings[userID]`，保持代码与注释一致

- [x] 1.8 更新 backend/es_handler.go：stat 查询适配新索引名和字段名
     【目标对象】`backend/es_handler.go`
     【修改目的】stat 查询索引名前缀改变，字段名由 project_aic_xxx 改为 aic_xxx，dimension 查询逻辑改为直接使用对应索引
     【修改方式】修改 getStatData 和 getStatSummary 两个 handler
     【相关依赖】新 stat 索引命名规则
     【修改内容】
        - `getStatData` 中：索引名生成改为根据 dimension 参数选择对应前缀，如 dimension=project → `costrict_chat_stat_project_`，dimension=user → `costrict_chat_stat_user_`，dimension=org1 → `costrict_chat_stat_org1_` 等；不再用 exists query 区分文档类型（每个索引内只有对应维度的数据）；query 改为 `match_all`
        - `getStatSummary` 中：索引名前缀改为 `costrict_chat_stat_project_`，聚合字段名由 `project_aic_user_in_chars` → `aic_user_in_chars` 等（所有 `project_aic_` 前缀改为 `aic_`），聚合 key 字段由 `project_id` → `union_id`
        - `getIndices` 中：stat 索引识别逻辑 `_stat_` 已能匹配新格式（无需修改，因新索引名仍含 `_stat_`）
