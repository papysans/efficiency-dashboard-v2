## 实施

- [x] 2.1 创建组织信息查询接口
     【目标对象】`kbcli/org_provider.go`, `config.yaml`
     【修改目的】提供根据 user_uuid 查询组织信息的接口
     【修改方式】定义 OrgInfo 结构体和 GetOrgInfo 函数，当前使用 mock 数据，预留 SQL 扩展点
     【相关依赖】无
     【修改内容】
        - 定义 OrgInfo 结构体（Org1/Org2/Org3/Org4 字段）
        - 实现 GetOrgInfo(userUUID, orgMappings) 函数，从 config 的 org_mappings 查询
        - 在 config.yaml 末尾追加 org_mappings 配置段（map[user_uuid]OrgInfo）
        - 添加注释说明后续可替换为 SQL 查询

- [x] 2.2 在 raw_parser.go 中添加 org 字段提取
     【目标对象】`kbcli/raw_parser.go`
     【修改目的】在 RawDoc 中添加 org1/org2/org3/org4 字段
     【修改方式】修改 RawDoc 结构体，在 ParseRawJSON 中调用 GetOrgInfo
     【相关依赖】org_provider.go
     【修改内容】
        - 在 RawDoc 结构体中添加 Org1/Org2/Org3/Org4 字段（string 类型）
        - 在 ParseRawJSON 函数中调用 GetOrgInfo(userUUID, orgMappings)
        - 将查询结果填充到 RawDoc 的 org 字段

- [x] 2.3 在 stat_builder.go 中添加组织维度聚合
     【目标对象】`kbcli/stat_builder.go`
     【修改目的】添加 org1/org2/org3/org4 四个维度的统计
     【修改方式】定义 4 个 OrgStatDoc 结构体，在 BuildStatDocs 中添加聚合逻辑
     【相关依赖】无
     【修改内容】
        - 定义 Org1StatDoc/Org2StatDoc/Org3StatDoc/Org4StatDoc 结构体
        - 修改 BuildStatDocs 返回值，增加 4 个组织维度的返回
        - 按 org1/org2/org3/org4 分组聚合（跳过空字符串）
        - 每个维度包含：org_aic_user_in_chars、org_aic_assistant_out_code_lines、org_aic_start_time、org_aic_end_time、org_aic_lead_time、org_aic_process_time、org_aic_api_count、org_aic_api_in_tokens、org_aic_api_out_tokens、org_aic_api_cost

- [x] 2.4 更新 cmd_reindex.go 写入组织维度数据
     【目标对象】`kbcli/cmd_reindex.go`
     【修改目的】将组织维度统计写入 ES
     【修改方式】修改 runReindex 函数，传递 orgMappings 给 ParseRawJSON，写入 org stat 文档
     【相关依赖】stat_builder.go
     【修改内容】
        - 从 config 读取 org_mappings
        - 调用 ParseRawJSON 时传递 orgMappings 参数
        - 调用 BuildStatDocs 获取 org 维度统计
        - 将 org1/org2/org3/org4 统计文档合并写入 stat 索引

- [x] 2.5 更新 ES mapping 定义
     【目标对象】`kbcli/es_mappings.go`
     【修改目的】在 mapping 中添加 org 字段定义
     【修改方式】更新 RawIndexMapping 和 StatIndexMapping 常量
     【相关依赖】无
     【修改内容】
        - RawIndexMapping 中添加 org1/org2/org3/org4 字段（keyword 类型）
        - StatIndexMapping 中添加 org1/org2/org3/org4 字段（keyword 类型）
        - StatIndexMapping 中添加 org*_aic_* 系列指标字段
