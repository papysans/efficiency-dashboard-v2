# 变更：重构 ES 索引 Schema（raw 字段重命名 + stat 拆分为独立索引）

## 原因
当前 raw 索引字段命名不规范（user_uuid/username/repo 语义不清），stat 索引将 7 个维度的文档混存在同一索引中，导致加载时资源占用高，且无法按维度独立查询。

## 变更内容
### 1. raw 层字段重命名
- `user_uuid` → `user_id`
- `username` → `user_name`
- `repo` → `repo_id`（暂时留空，字段存在）

### 2. stat 层拆分为 7 个独立索引
原：`costrict_chat_stat_YYYYMMDD`（混存所有维度）

新：
- `costrict_chat_stat_project_YYYYMMDD`  — union_id = project_id
- `costrict_chat_stat_repo_YYYYMMDD`     — union_id = repo_id（暂时留空）
- `costrict_chat_stat_user_YYYYMMDD`     — union_id = user_id
- `costrict_chat_stat_org4_YYYYMMDD`     — union_id = org1_org2_org3_org4
- `costrict_chat_stat_org3_YYYYMMDD`     — union_id = org1_org2_org3
- `costrict_chat_stat_org2_YYYYMMDD`     — union_id = org1_org2
- `costrict_chat_stat_org1_YYYYMMDD`     — union_id = org1

### 3. stat 字段命名统一
移除各维度前缀（`project_aic_`、`user_aic_`、`org1_aic_` 等），统一改为 `aic_xxx`：
- `union_id`（keyword，归并主键）
- `aic_start_time`、`aic_end_time`（date）
- `aic_user_in_chars`、`aic_assistant_out_code_lines`（long）
- `aic_lead_time`、`aic_process_time`（long）
- `aic_api_count`、`aic_api_in_tokens`、`aic_api_out_tokens`（long）
- `aic_api_cost`（float）

## 影响
- **受影响的规范**：ES 索引 Schema、数据写入流程、API 查询接口
- **受影响的代码**：
  - `kbcli/es_mappings.go`：更新 RawIndexMapping，替换 StatIndexMapping 为通用 StatIndexMapping（单一模板）
  - `kbcli/raw_parser.go`：RawDoc 结构体字段重命名（UserUUID→UserID，Username→UserName，Repo→RepoID），同步更新 json tag，以及赋值逻辑
  - `kbcli/stat_builder.go`：所有 StatDoc 结构体统一为 `StatDoc`，字段统一为 `union_id + aic_xxx`；BuildStatDocs 返回值类型和聚合键更新
  - `kbcli/stat_builder_test.go`：更新测试代码中的结构体字段引用
  - `kbcli/cmd_reindex.go`：stat 索引名改为按维度生成，分别写入对应独立索引
  - `kbcli/org_provider.go`：GetOrgInfo 函数参数注释更新（userUUID→userID）
  - `backend/es_handler.go`：更新 stat 查询维度逻辑（索引名前缀改变），更新 getStatSummary 聚合字段名（project_aic_xxx → aic_xxx）
