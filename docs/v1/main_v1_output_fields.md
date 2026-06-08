# main 分支 v1 全量输出字段解析

## 1. 运行结论

本次使用 `main` 分支实际全量跑了一次 v1 链路，输出已保存为：

- `docs/data/main_v1_fullrun_output.json`

JSON 内容包含：

- `run`：运行来源、命令、输入目录、兼容处理说明。
- `tables.<table>.row_count`：每张表行数。
- `tables.<table>.columns`：每张表实际数据库字段。
- `tables.<table>.rows`：每张表完整数据行。

本次运行来源：

| 项 | 值 |
|---|---|
| 分支 | `main` |
| commit | `7e0f07416e7472efc418b110aa9a8c21365e2780` |
| task 输入 | `工时估算数据/mnt/user-indicator/raw/task` |
| repo 输入 | `工时估算数据/mnt 2/user-indicator/raw/repo` |
| 临时数据库 | `efficiency_main_fullrun` |
| 执行命令 | `import-conv --force` → `import-repo --force` → `import-org --from-csv` → `efficiency` |

实际产出行数：

| 表 | 行数 |
|---|---:|
| `sessions` | 12114 |
| `conversations` | 17073 |
| `commits` | 2616 |
| `tasks` | 0 |
| `user_org` | 129 |
| `user_productivity` | 570 |
| `projects` | 0 |
| `project_tasks` | 0 |
| `project_repos` | 0 |
| `project_commits` | 0 |
| `user_groups` | 0 |

注意：

- `import-repo` 日志显示成功处理 3018 个 commit 文件，但 `commits` 表最终是 2616 行；原因是 `commit_id` 是主键，重复 commit 会 upsert 覆盖。
- 另有 6 个 commit 文件因 `user_id` 为空被 main 跳过。
- `import-repo` 构建 conversation 指纹索引时显示 0 组对话，因此本次没有生成 `tasks`，`conversations.task_id` 也为空。
- `efficiency` 产出的 `user_productivity` 全部来自 commit 侧：`task_count > 0` 的记录为 0，`commit_count > 0` 的记录为 570。
- main fresh DB 直接跑 `efficiency` 会失败；临时库补了 main 运行时实际引用的旧兼容列：`commits.commit_real_ancient_minutes`、`user_productivity.task_ids`、`user_productivity.commit_ids`。这些列会出现在导出 JSON 中，但不是当前 main 模型结构里显式定义的字段。

## 2. 输出表关系

main v1 的输出层级是：

| 层级 | 表 | 说明 |
|---|---|---|
| 原始会话入库 | `sessions` | 一条 summary 对应一个 session |
| 原始对话入库 | `conversations` | JSONL 每行一条 request/response |
| 原始提交入库 | `commits` | 一个 commit JSON 对应一个 commit；main 不保存 `files` |
| silica 关联 | `tasks` | conversation 与 commit 指纹匹配后生成；本次为 0 |
| 用户组织 | `user_org` | 组织映射；本次由已入库用户生成临时 CSV |
| 用户日聚合 | `user_productivity` | v1 最终看板核心输出，按用户和日期聚合 |
| 项目侧表 | `projects` / `project_*` | main 支持项目维度，但本次没有项目数据 |
| 用户分组 | `user_groups` | 本次没有分组数据 |

## 3. `sessions`

来源：`task/summary/YYYY/MM/DD/<task_id>.json`。

| 字段 | 含义 |
|---|---|
| `session_id` | session 主键，来自 summary 的 `task_id` |
| `create_time` | session 开始时间，来自 summary 的 `start_time` |
| `user_id` | 用户 ID |
| `user_name` | 用户名 |
| `client_id` | 客户端设备标识 |
| `client_ide` | 客户端 IDE |
| `client_version` | 客户端版本 |
| `client_os` | 客户端操作系统 |
| `client_os_version` | 客户端系统版本 |
| `session_date` | summary 文件路径日期 |
| `conversation_date` | conversation 文件路径日期 |
| `created_at` | 入库时间 |
| `updated_at` | 更新时间 |

## 4. `conversations`

来源：`task/conversation/YYYY/MM/DD/<task_id>.jsonl`。

| 字段 | 含义 |
|---|---|
| `id` | 自增主键 |
| `session_id` | 所属 session |
| `request_id` | session 内请求 ID |
| `task_id` | silica 匹配后的 task ID；本次为空 |
| `sender` | 消息发送方 |
| `prompt_mode` | 提示模式 |
| `mode` | 对话模式 |
| `model` | 模型名 |
| `start_time` | 本轮对话开始时间 |
| `end_time` | 本轮对话结束时间 |
| `process_time` | 处理耗时，毫秒 |
| `process_ttft` | 首 token 耗时，毫秒 |
| `upstream_tokens` | 输入 token |
| `downstream_tokens` | 输出 token |
| `cost` | 费用 |
| `diff_lines` | 本轮对话产生的变更行数 |
| `repo_addr` | 仓库地址 |
| `repo_branch` | 分支 |
| `work_dir` | 工作目录 |
| `work_dir_id` | 由 `client_id + work_dir` 生成的工作目录 ID |
| `user_input` | 用户输入 |
| `request_content` | 请求正文 |
| `response_content` | 响应正文 |
| `error_code` | 错误码 |
| `error_reason` | 错误原因 |
| `created_at` | 入库时间 |

## 5. `commits`

来源：`repo/<repo>/<branch>/YYYY/MM/DD/<commit_id>.json`。

main 分支不会保存 raw commit 里的 `files` 字段，本表没有 `touched_files`。

| 字段 | 含义 |
|---|---|
| `commit_id` | commit hash，主键 |
| `commit_time` | commit 时间 |
| `repo_addr` | 仓库地址 |
| `repo_branch` | 分支 |
| `git_user_name` | Git 用户名 |
| `git_user_email` | Git 邮箱 |
| `user_id` | 系统用户 ID |
| `user_name` | 系统用户名 |
| `client_id` | 客户端设备标识 |
| `work_dir` | 工作目录 |
| `work_dir_id` | 由 `client_id + work_dir` 生成的工作目录 ID |
| `diff_lines` | main 重新解析 diff 后得到的新增行数 |
| `commit_ancient_minutes` | commit 传统开发估算分钟数 |
| `commit_ancient_reason` | 传统估算原因 |
| `commit_ancient_minutes_manual` | 人工修正传统估算分钟数 |
| `commit_ancient_reason_manual` | 人工修正原因 |
| `upstream_tokens` | 被 silica 关联到该 commit 的对话输入 token |
| `downstream_tokens` | 被 silica 关联到该 commit 的对话输出 token |
| `cost` | 被 silica 关联到该 commit 的对话费用 |
| `silica` | commit 含硅量，即 AI 覆盖比例 |
| `commit_real_ai_minutes` | commit 中 AI 关联工作分钟数 |
| `commit_real_non_ai_minutes` | commit 中未被 AI 覆盖的工作分钟数 |
| `commit_real_minutes` | commit 实际工作分钟数 |
| `commit_real_reason` | 实际工作分钟数说明 |
| `commit_real_minutes_manual` | 人工修正实际分钟数 |
| `commit_real_reason_manual` | 人工修正说明 |
| `comment` | commit message |
| `created_at` | 入库时间 |
| `updated_at` | 更新时间 |
| `commit_real_ancient_minutes` | main `efficiency` SQL 引用的旧兼容列；本次临时补列并由 `commit_real_non_ai_minutes` 回填 |

本次实际结果：

- `silica > 0` 的 commit 数为 0。
- `upstream_tokens/downstream_tokens/cost` 在 commit 侧均没有通过 silica 聚合出有效值。

## 6. `tasks`

来源：`import-repo` 中 conversation 指纹与 commit diff 指纹匹配后生成。

本次行数为 0。字段如下：

| 字段 | 含义 |
|---|---|
| `task_id` | task 主键，通常是 `session_id|commit_id` |
| `commit_id` | 关联 commit |
| `session_id` | 关联 session |
| `user_id` | 用户 ID |
| `user_name` | 用户名 |
| `client_id` | 客户端设备标识 |
| `client_ide` | 客户端 IDE |
| `client_version` | 客户端版本 |
| `client_os` | 客户端操作系统 |
| `client_os_version` | 客户端系统版本 |
| `caller` | 调用来源 |
| `repo_addr` | 仓库地址 |
| `repo_branch` | 分支 |
| `work_dir` | 工作目录 |
| `work_dir_id` | 工作目录 ID |
| `start_time` | task 开始时间 |
| `end_time` | task 结束时间 |
| `diff_lines` | task 变更行数 |
| `silica` | 与 commit 的含硅匹配比例 |
| `accept_ratio` | 采纳比例 |
| `upstream_tokens` | 输入 token |
| `downstream_tokens` | 输出 token |
| `cost` | 费用 |
| `task_real_minutes` | task 实际分钟数 |
| `task_real_reason` | 实际分钟数说明 |
| `task_real_minutes_manual` | 人工修正实际分钟数 |
| `task_real_reason_manual` | 人工修正说明 |
| `task_ancient_minutes` | task 传统估算分钟数 |
| `task_ancient_reason` | 传统估算说明 |
| `task_ancient_minutes_manual` | 人工修正传统估算分钟数 |
| `task_ancient_reason_manual` | 人工修正说明 |
| `title` | task 标题 |
| `session_date` | summary 路径日期 |
| `conversation_date` | conversation 路径日期 |
| `created_at` | 入库时间 |
| `updated_at` | 更新时间 |

## 7. `user_org`

来源：`import-org`。本次没有连接外部 auth/quota 库，而是从已入库 `sessions/commits` 生成 main 可读 CSV。

| 字段 | 含义 |
|---|---|
| `user_id` | 用户 ID，主键 |
| `user_name` | 用户名 |
| `org1` ~ `org9` | 组织层级 |
| `git_user_name` | Git 用户名 |
| `git_user_email` | Git 邮箱 |
| `created_at` | 入库时间 |
| `updated_at` | 更新时间 |

## 8. `user_productivity`

来源：`efficiency` 命令，按用户和日期聚合 `tasks` 与 `commits`。

| 字段 | 含义 |
|---|---|
| `user_productivity_id` | 主键，格式为 `user_id_YYYYMMDD` |
| `create_time` | 聚合日期 |
| `user_id` | 用户 ID |
| `user_name` | 用户名 |
| `upstream_tokens` | 当日 task 输入 token 合计 |
| `downstream_tokens` | 当日 task 输出 token 合计 |
| `cost` | 当日 task 费用合计 |
| `task_count` | 当日 task 数 |
| `task_diff_lines` | 当日 task 变更行数合计 |
| `task_real_minutes` | 当日 task 实际分钟数合计 |
| `task_ancient_minutes` | 当日 task 传统估算分钟数合计 |
| `task_efficiency_ratio` | task 口径提效比，`task_ancient_minutes / task_real_minutes` |
| `commit_count` | 当日 commit 数 |
| `commit_diff_lines` | 当日 commit 新增行数合计 |
| `commit_ancient_minutes` | 当日 commit 传统估算分钟数合计 |
| `commit_real_ai_minutes` | 当日 commit AI 覆盖分钟数合计 |
| `commit_real_ancient_minutes` | 当日 commit 未覆盖分钟数合计；main 聚合 SQL 使用该列名 |
| `commit_real_minutes` | 当日 commit 实际分钟数合计 |
| `commit_efficiency_ratio` | commit 口径提效比，`commit_ancient_minutes / commit_real_minutes` |
| `created_at` | 入库时间 |
| `updated_at` | 更新时间 |
| `task_ids` | main upsert SQL 引用的旧兼容列；本次临时补列，值为空数组 |
| `commit_ids` | main upsert SQL 引用的旧兼容列；本次临时补列，值为空数组 |

本次实际结果：

- `user_productivity` 共 570 行。
- `task_count > 0` 的行数为 0。
- `commit_count > 0` 的行数为 570。
- 因 `silica=0`，commit 侧的实际/传统分钟数大多为 0，提效比也相应为 0。

## 9. 项目与分组表

本次这些表为空，但 main 会建表。

### `projects`

| 字段 | 含义 |
|---|---|
| `project_id` | 项目 ID |
| `name` | 项目名 |
| `description` | 项目描述 |
| `repos` | 项目关联仓库列表 |
| `task_ids` | 项目关联 task 列表 |
| `task_ids_silica` | 含硅 task 列表 |
| `start_time` / `end_time` | 项目自动起止时间 |
| `start_time_manual` / `end_time_manual` | 人工修正起止时间 |
| `upstream_tokens` / `downstream_tokens` / `cost` | 项目 token 与费用 |
| `project_ancient_minutes` / `project_ancient_reason` | 项目传统估算 |
| `project_ancient_minutes_manual` / `project_ancient_reason_manual` | 人工修正传统估算 |
| `project_real_process_minutes` / `project_real_process_reason` | 项目过程耗时 |
| `project_real_process_minutes_manual` / `project_real_process_reason_manual` | 人工修正过程耗时 |
| `project_real_lead_minutes` / `project_real_lead_reason` | 项目 lead time |
| `project_real_lead_minutes_manual` / `project_real_lead_reason_manual` | 人工修正 lead time |
| `created_at` / `updated_at` | 创建/更新时间 |

### `project_tasks`

| 字段 | 含义 |
|---|---|
| `project_id` | 项目 ID |
| `task_id` | task ID |
| `silica` | 项目内该 task 含硅权重 |
| `accept_ratio` | 采纳比例 |
| `created_at` / `updated_at` | 创建/更新时间 |

### `project_repos`

| 字段 | 含义 |
|---|---|
| `project_id` | 项目 ID |
| `repo_addr` | 仓库地址 |
| `repo_branch` | 分支 |
| `start_time` / `end_time` | 仓库纳入项目的时间窗口 |
| `exclude_commits` | 排除 commit 列表 |
| `include_only_commits` | 仅包含 commit 列表 |
| `created_at` / `updated_at` | 创建/更新时间 |

### `project_commits`

| 字段 | 含义 |
|---|---|
| `project_id` | 项目 ID |
| `commit_id` | commit ID |
| `repo_addr` | 仓库地址 |
| `repo_branch` | 分支 |
| `created_at` / `updated_at` | 创建/更新时间 |

### `user_groups`

| 字段 | 含义 |
|---|---|
| `group_id` | 分组 ID |
| `name` | 分组名 |
| `org_name` | 组织名 |
| `user_ids` | 分组内用户 ID 列表 |
| `created_at` / `updated_at` | 创建/更新时间 |

## 10. 对比 v2 输入的直接结论

main v1 原本不会输出或持久化 commit raw 里的 `files`，因此 main 输出 JSON 中没有 `files` / `touched_files` 这一类字段。

如果 v2 要利用 `mnt 2` 的 `files` 信号，必须在 v2 分支额外接入：

- import 层：从 raw commit JSON 读取 `files`。
- storage 层：保存到 `commits.touched_files`。
- Need 层：从 `commits.touched_files` 进入 `needs.touched_files` 和 `lv4_cluster`。

