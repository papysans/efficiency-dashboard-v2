# dept-sync 离线部门树服务 接入设计

> 目标读者：efficiency-dashboard 后端 / kbcli / 前端开发者
> 状态：设计稿（仅调研，未改业务代码）
> 关联代码分支：`feat/efficiency-v2-pipeline`

dept-sync 是一个独立的离线部门同步服务（Go + 内置 SQLite，可切 PostgreSQL），对接深信服 HR / 部门 API，提供权威的「部门树 + 人员归属 + 真名 + 职级」。本文梳理看板侧 org 维度现状，并给出把 dept-sync 接入看板的两种方案对比、数据映射、鉴权口径与分阶段落地步骤。

---

## 1. 现状梳理：看板当前 org 维度怎么实现的

### 1.1 数据来源：手工 CSV / 上游 DB 二选一，落到一张扁平表 `user_org`

看板的「组织」维度完全建立在单表 `user_org` 上，结构是把组织层级拍平成 `org1`～`org9` 九个字符串列，外加用户标识与 git 信息：

- 模型定义：`core/models/models.go:10-28`（`UserOrg`，主键 `user_id`，列 `org1..org9 / git_user_name / git_user_email`，表名 `user_org`）。
- 导入逻辑：`kbcli/cmd_import_org.go`，`import-org` 命令。三条数据来路（优先级从高到低）：
  1. `--from-csv` 显式 CSV → `loadUserOrgsFromCSV`（`cmd_import_org.go:45-90`，固定 13 列）。
  2. `--from-db` 源库 → `loadUserOrgsFromDB`（`cmd_import_org.go:92-174`）：连源库 `auth.auth_users` + `quota_manager.employee_department`，按 `employee_number` 关联，把 `dept_full_level_names`（逗号分隔的层级全名）拆成 `org1..org9`。
  3. 兜底 → 配置项 `org_csv_file`（`cmd_import_org.go:324-335`）失败后再回落到本地 `tasks/commits/sessions` 生成临时记录，全部归入 `org1="临时组织"`（`loadDefaultUserOrgsFromLocalData`，`cmd_import_org.go:176-253`）。
- 配置项：`org_csv_file`（默认 `analysed/org_mapping.csv`）与 `org_dsn`，见 `kbcli/config.go:170/177/274-275`、`compose/kbcli/config.yaml:123-124`、`kbcli-config.yaml:179-180`。内网 compose 里 `org_dsn` 实际指向 `10.72.10.64:25432` 的 `costrict_reader` 只读库。

项目根的 `org_mapping.csv` 是这套机制的样例 / 兜底 CSV：13 列 `user_id,user_name,org1..org9,git_user_name,git_user_email`，内容全是示例公司假数据（"示例公司/研发体系/平台部/基础设施组" 等），**粒度是「公司/体系/部/组」四级，但靠纯文本字符串硬编码，没有任何稳定 ID 或父子关系**。

### 1.2 后端有两套并存的 org handler，且当前真正生效的是「扁平 native」那套

| | V1 多级 handler | V2 native handler（当前生效） |
|---|---|---|
| 文件 | `backend/org_handler_v2.go` | `backend/user_org_v2_native_handler.go` |
| 列表入口 | `listOrgV2`（`org_handler_v2.go:225`，支持 `level=org1..org4` / `parent` 级联 / 时间序列） | `listOrgsV2Native`（`user_org_v2_native_handler.go:213`） |
| 路由 `GET /api/v2/orgs` 实际绑定 | ❌ 未绑定 | ✅ 绑定（`backend/main.go:125`） |
| org 粒度 | 完整 org1..org9 树（`filterUsersByParent` 按路径逐级匹配，`org_handler_v2.go:181-205`） | **只取 `org1` 一级**，其余忽略；无 `org1` 的用户归入「未分组」（`user_org_v2_native_handler.go:224-256`） |
| 数据底座 | `user_productivity`（V1 聚合表） | `user_productivity_v2`（V2 周聚合，`QueryEfficiencyV2Aggregate`） |
| org 映射来源 | 内存 `orgMappings`（启动时 `LoadUserOrgs` 一次性加载，`main.go:66`、`db.go:1713`） | 每次请求直接 `statDB.Find(&userOrgs)` 现查 |

仍绑定到路由的：`GET /api/v2/orgs`→`listOrgsV2Native`、`GET /api/v2/orgs/detail`→`getOrgDetailV2`（V1 文件里的多级详情，按 `org_path` 逐级匹配 org1..org9）、`POST /api/v2/orgs/refresh`→`refreshOrgMappingV2`（重载内存映射）、`GET /api/v2/group`→`getGroupDetailV2`（org1..org4 维度详情）。见 `backend/main.go:125-128`。

> 注意一个口径裂缝：列表页 `/v2/orgs` 走 native（只认 org1），而详情页 `/v2/orgs/detail` 走 V1 多级匹配。前端 React `OrgDetail.tsx:5-6` 注释已明确记录"native handler 忽略 level/parent，四级级联在 native 下退化"。

### 1.3 前端：org 维度被前端自己标注为"仅供参考"

- `frontend-react/src/pages/orgs/OrgList.tsx`：列表读 `/v2/orgs`（native），当 `no_org_mapping=true`（即没有任何用户的 `org1` 非空）时渲染黄色警告条"当前数据集缺少完整的用户↔组织映射（user_org 多数为空），未映射用户已归入「未分组」。组织维度仅供参考"（`OrgList.tsx:194-204`）。
- `frontend-react/src/pages/orgs/OrgDetail.tsx`：详情页做 org1..org4 四级级联选择，但级联选项来自 `/v2/orgs`（native 忽略 level/parent），已退化。

### 1.4 用户标识口径：user_id=UUID，真名靠 git commit 反推（无权威真名）

- 看板事实主键是 `user_id`（UUID 格式，如 `43612100-...`），见 `tasks/commits/sessions/user_productivity` 各表的 `user_id` 列。
- **`user_name` 多数也是 UUID 或不可读**：`/v2/users` 返回的 `user_name` 多为 UUID（`user_org_v2_native_handler.go` 聚合自 v2 周表，名字不可靠）；needs 只有 `primary_user_id=UUID`。
- **真名目前唯一来源是 commits 的 `git_user_name`**：前端 hook `frontend-react/src/hooks/useUserNameMap.ts` 从 `getCommitsV2({pageSize:250})` 建 `user_id → 真名` 映射，优先级 `git_user_name > user_name > user_id`（`useUserNameMap.ts:22-26`）。注释里举的例子正是 "IronRookieCoder/林凯90331"。
- kbcli 侧也有三级回退（`user_org > tasks > commits`，`kbcli/cmd_efficiency.go:99-148`），但 `user_org` 在当前数据集多为空，最终落到 git 名。

**痛点小结**：
1. **手工 CSV**：org 层级是手写文本，无 ID、无父子关系，公司/部门改名即失效；`org_mapping.csv` 还是假数据。
2. **UUID 无权威真名**：真名靠 commit `git_user_name` 反推，未提交代码的人、改过 git 配置的人就拿不到名字；前端每页拉 250 条 commit 现建映射，规模一大就漏。
3. **无真实部门层级**：扁平 org1..org9 没有稳定 dept_id / dept_path，做不到真正的树形下钻、上卷；native handler 干脆只用 org1。
4. **无职级（position）维度**：完全缺失。
5. **两套 handler 口径裂缝**：列表扁平 / 详情多级，级联退化。

---

## 2. 对接目标

用 dept-sync 的权威部门树替换 / 增强看板现有 org 维度，具体：

1. **真名**：用 dept-sync `user_department.username`（如 林凯、邓彬）替换 commit 反推的不稳定真名。
2. **部门归属 + 层级**：用 `department`（邻接表 `parent_dept_id` + 物化路径 `dept_path` + `dept_level`）替换手工 org1..org9，支持真正的树形下钻。
3. **职级**：引入 `user_department.position`（如 "架构师"/"开发技术专家"/"实习生"），看板新增职级维度。
4. **JOIN 锚点 = 工号（employee_number）**：dept-sync `user_department.user_id`（工号，如 `90331`）↔ 看板 `commits.git_user_email` 前缀（`90331@sangfor.com` 去掉 `@sangfor.com`）。**不是 universal_id**。

**关键证据（已实测，2026-06-05）**

| 假设锚点 | 验证方法 | 结果 |
|---|---|---|
| ~~`universal_id`(UUID) ↔ 看板 `user_id`(UUID)~~ | 16 个 universal_id `IN` 看板 user_org/sessions 的 user_name/user_id | **0/16 命中**，口径完全不同（universal_id 是 Casdoor `custom` 字段，看板 user_id 是 Casdoor `user.id`，两套 UUID 无交集） |
| **`user_id`(工号) ↔ `git_user_email` 前缀** | dept-sync 18 个工号 `IN` `split_part(commits.git_user_email,'@',1)` | **2/18 命中**（`邓彬84569↔84569`、`林凯90331↔90331`，工号精确相等） |

结论：**universal_id 是错误的锚点（0%）；工号才是正确锚点**，且数据重叠处 100% 精确匹配。2/18 的低绝对命中纯属**两边都是样例子集**：看板侧 sangfor 真实工号总共仅 5 个（`14315/35882/74179/84569/90331`），其中 2 个落在 dept-sync 的 18 人样例内；另外 3 个需 dept-sync 全量库（2087 人）才覆盖。即口径一致、可对接，缺的是数据完整度。

桥接链路：`看板 user_id(UUID)` --[`commits.git_user_email`]--> `工号` --[`dept-sync.user_department.user_id`]--> `真名 + dept_id + position + universal_id`。universal_id 仅作为 dept-sync 附带字段保留，**不参与 JOIN**。

> ⚠️ HTTP API `GET /api/v1/user/{user_id}/departments` 的 `{user_id}` 实为工号口径（与内置库 `user_department.user_id` 一致），原任务背景"对应 universal_id"的提示有误，已按实测纠正。

---

## 3. 两种对接方案对比

### 方案 A：实时 API（看板后端按需调 dept-sync `/api/v1/...`）

看板后端在渲染 org / user 维度时，实时请求 dept-sync 的 HTTP 接口（`/costrict-dept-info/api/v1/department/tree`、`/user/{工号}/departments` 等），拿到部门树与人员归属后在内存里 JOIN 看板数据（仍须经 `commits.git_user_email` 把看板 user_id 桥接到工号）。

**优点**
- 数据零延迟，dept-sync 每天 cron 同步（`config.yaml` `sync.cron: "0 4 * * *"`），看板永远看最新。
- 看板不落部门数据，不引入新表、不改 DDL。

**缺点**
- 强耦合：dept-sync 必须常驻可达，挂了看板 org 维度直接不可用。
- 需处理鉴权：dept-sync `jwtSettings.enable=true`（casdoor JWT）或 `query_key`。**看板内网当前不部署 casdoor，后端只有写死的 auth shim（`backend/auth_shim_handler.go:3-7` 明确"内网决定不部署 casdoor"）**，因此只能走 `query_key` / `admin_key`，需在看板后端配置并保管密钥。
- 性能：org 列表 / 详情每次都要远程拉树 + 按 user 查部门，N 个用户可能 N 次调用（`/user/{id}/departments`）。需在看板侧加缓存层（dept-sync 自身有 5 分钟缓存，但那是它进程内的）。
- 改动面集中在后端 handler，要新写 HTTP client + 缓存 + 错误降级，**无法复用现有 `user_org` / `LoadUserOrgs` 这套成熟代码**。
- 与现有 `org_mapping.csv` 机制并存会造成两套口径。

### 方案 B：数据同步 / 落库（定期把 dept-sync 数据同步进看板 PG）★ 推荐

定期（或一次性）把 dept-sync 的 `department` + `user_department` 同步进看板自己的 PostgreSQL（`costrict_stat` 库），新增两张表 `dept` / `dept_user`，并保留 / 复用现有 `user_org` 这条链路（把 `user_org` 当作"投影视图"由同步任务回填）。

**优点**
- **解耦**：dept-sync 临时不可达不影响看板（数据已落库）。契合内网"compose-only、不依赖外部常驻服务"的交付风格。
- **可 JOIN**：部门树和看板 `commits/tasks/user_productivity_v2` 在同一 PG 库，能用 SQL 直接 JOIN / 下钻 / 上卷，性能好。
- **最大化复用现有代码**：现有 `user_org`（org1..org9）+ `orgMappings` 内存映射 + `listOrgV2/getOrgDetailV2/getGroupDetailV2` 整套 handler 几乎不用改 —— 同步任务把 dept-sync 的 `dept_path` 拆成 org1..org9 回填 `user_org` 即可，路由 / 前端列继续工作。这正是 `loadUserOrgsFromDB` 已有的"`dept_full_level_names` 拆 org1..org9"模式（`cmd_import_org.go:155-165`），dept-sync 的 `dept_path` 是同形态字符串。
- **离线一次性导入也可**：dept-sync 内置 SQLite（`dept_sync-inner.sqlite`）可被 kbcli 直接读取做一次性导入，无需 dept-sync 进程在线。
- **数据沉淀**：额外保留 `position`（职级）、`dept_id`/`dept_path`（稳定 ID + 物化路径），为后续真正的树形维度打底，不丢信息。

**缺点**
- 有同步延迟（取决于同步频率；可与 dept-sync 的每日 cron 对齐，凌晨同步）。
- 引入新表 + 同步任务（DDL + 一个 kbcli 子命令），改动面比 A 略大，但都是新增、不破坏现网。

### 推荐：方案 B（数据同步 / 落库）

理由：
1. **复用优先**（CLAUDE.md 核心原则）：B 能直接复用 `user_org` + `orgMappings` + 现有 org handler + `import-org` 的"层级字符串拆 org1..org9"逻辑，后端 / 前端几乎零改动即可让 org 维度有真实数据；A 要新写 client+缓存+降级且无法复用。
2. **解耦 + 内网交付习惯**：内网是 compose-only、明确不部署 casdoor。B 不依赖 dept-sync 常驻、不需要在看板里集成 casdoor JWT，鉴权只在同步那一刻用 `query_key`，运行期零外部依赖。
3. **可演进**：先用 B 的"拆 org1..org9 回填 user_org"零改动跑通；后续若要真正的树形维度，再用已落库的 `dept`/`dept_user`（带 dept_id/dept_path/position）平滑升级，不返工。

### 与现有 `org_mapping.csv` 的迁移关系

- `org_mapping.csv`（假数据兜底）在方案 B 下被同步任务**取代**：同步任务成为 `user_org` 的权威写入源，`org_mapping.csv` 仅保留为"dept-sync 都不可用时"的最后兜底（现有 `import-org` 的回落链已支持，无需改）。
- 不删除 `org_mapping.csv` 链路，避免破坏现有兜底（删除前需 `rg` 确认无调用，符合 CLAUDE.md "删除前必须验证"）。

---

## 4. 数据映射设计

### 4.1 dept-sync 表 → 看板表字段映射

dept-sync 内置库关键表（实测 schema，`temp/dept-sync/dept_sync-inner.sqlite`）：

```
department(dept_id, dept_name, parent_dept_id, dept_path, dept_level, order_num, leader_id, child_dept_count, status, version, ...)
user_department(user_id<工号>, username, dept_id, universal_id<UUID>, is_main, position, entry_time, status, version, ...)
```

**新增看板表（方案 B，落 `costrict_stat` 库）**

```
-- 部门树（邻接表 + 物化路径双存，1:1 镜像 dept-sync.department）
dept(
  dept_id          varchar(64) PRIMARY KEY,   -- dept-sync.dept_id（稳定 ID）
  dept_name        varchar(128),
  parent_dept_id   varchar(64),               -- 邻接表父指针
  dept_path        varchar(1024),             -- 物化路径 /深信服.../研发体系/...
  dept_level       int,
  leader_id        varchar(64),               -- dept-sync 工号口径
  child_dept_count int,
  status           int,
  updated_at       timestamptz
)

-- 人员归属（来自 dept-sync.user_department，以「工号」为看板侧锚点）
dept_user(
  emp_no       varchar(64) PRIMARY KEY,  -- = dept-sync.user_department.user_id（工号，JOIN 锚点）
  real_name    varchar(128),             -- = dept-sync.username（权威真名）
  universal_id varchar(64),              -- = dept-sync.universal_id（UUID，仅留存，不参与 JOIN，可为空）
  dept_id      varchar(64),              -- 主部门（is_main=1）
  position     varchar(128),             -- 职级
  is_main      int,
  entry_time   date,
  status       int,
  updated_at   timestamptz
)
-- 看板 user_id(UUID) → 工号 的桥接靠 commits 表已有的 (user_id, git_user_email)：
--   工号 = split_part(commits.git_user_email,'@',1)；再 JOIN dept_user.emp_no
```

> SQL 占位符用 `$1,$2`（CLAUDE.md）。新表跟现有表一样走 GORM AutoMigrate（`core/models/models.go:280-315` 增两个 model）。

字段映射表：

| dept-sync | 看板 dept / dept_user | 说明 |
|---|---|---|
| `department.dept_id` | `dept.dept_id` (PK) | 稳定部门 ID |
| `department.dept_name` | `dept.dept_name` | |
| `department.parent_dept_id` | `dept.parent_dept_id` | 邻接表 |
| `department.dept_path` | `dept.dept_path` | 物化路径，用于下钻 / 拆 org1..org9 |
| `department.dept_level` | `dept.dept_level` | |
| `department.leader_id` | `dept.leader_id` | 工号口径，展示时再 JOIN dept_user.emp_no |
| `user_department.user_id` | `dept_user.emp_no` (PK) | **工号 = JOIN 锚点**（↔ `commits.git_user_email` 前缀） |
| `user_department.universal_id` | `dept_user.universal_id` | UUID，仅留存，不参与 JOIN，可为空 |
| `user_department.username` | `dept_user.real_name` | 权威真名 |
| `user_department.dept_id` | `dept_user.dept_id` | 主部门（取 is_main=1） |
| `user_department.position` | `dept_user.position` | 职级 |

### 4.2 部门树（邻接表 / 物化路径）如何落到看板 org 维度

两种落法，建议同时做，互不冲突：

1. **零改动投影到现有 `user_org`（先做，跑通 org 维度）**：同步任务对每个 `dept_user`，先用其 `emp_no`（工号）在 `commits` 表里反查对应看板 `user_id`（`SELECT DISTINCT user_id FROM commits WHERE split_part(git_user_email,'@',1)=$1`），再用 `dept_id` 查 `dept.dept_path`，把 `dept_path` 按 `/` 拆段写入 `user_org.org1..org9`，同时回填 `user_org.user_name=real_name`。这复用了 `cmd_import_org.go:155-165` 已有的"层级字符串拆 org1..org9"写法（`dept_path` 与 `dept_full_level_names` 同形态）。完成后现有 `listOrgsV2Native`（org1）、`getOrgDetailV2`/`getGroupDetailV2`（org1..org4 多级）、前端 OrgList/OrgDetail 立即有真实层级，`no_org_mapping` 警告消失，**后端 / 前端零改动**。

   - ⚠️ 关键：`user_org.user_id` 是看板 UUID，必须经 `commits.git_user_email` 工号桥接拿到，**不能直接写 universal_id**（universal_id 与看板 user_id 0% 命中，见 §2）。工号在 commits 里查不到的 dept_user（即该员工在看板无 sangfor commit）本轮跳过——这正是覆盖率受限的根因。

   - 路径示例：`/深信服科技股份有限公司/研发体系/Costrict研发部/开发组` → org1=深信服科技股份有限公司, org2=研发体系, org3=Costrict研发部, org4=开发组。
   - 注意去掉 `dept_path` 前导 `/` 空段（拆出来第 0 段是空字符串）。

2. **保留稳定 ID 树（后续做，真正树形维度）**：`dept` 表带 `parent_dept_id`（递归 CTE 上卷 / 下钻）+ `dept_path`（前缀匹配下钻）。后续可新增 `GET /api/v2/depts/tree` 等接口，按 dept_id 而非文本聚合，规避改名失效。这一步是增量演进，不阻塞第 1 步。

### 4.3 工号如何 JOIN 看板用户表

- 桥接两跳：`dept_user.emp_no(工号)` ↔ `split_part(commits.git_user_email,'@',1)` ↔ 拿到看板 `commits.user_id`，再 ↔ `tasks.user_id / user_productivity_v2.user_id`。**universal_id 不参与**（0% 命中，见 §2）。
- 真名解析改造：把前端 `useUserNameMap.ts` 的"从 commits.git_user_name 反推"升级为"先查 dept_user.real_name（经工号桥接到 user_id），未命中再回退 commit 名"。后端可新增轻量 `GET /api/v2/users/names`（返回 user_id→real_name+position+dept_path）供前端建映射，替代现在每页拉 250 条 commit 的做法。
- kbcli 三级回退（`cmd_efficiency.go:142-148`）的 `userNameMap` 改为优先从 `dept_user` 加载，自然提升真名命中率。

### 4.4 部门层级（公司/体系/部/组）→ 看板 org/project 粒度

dept-sync 实测树（`dept.csv` / 内置库一致）：

```
深信服科技股份有限公司(49,lvl1)
└─ 研发体系(1416,lvl2)
   └─ Costrict研发部(6560,lvl3)
      ├─ 开发组(6571,lvl4)
      └─ 客户成功组(6572,lvl4)
```

映射约定（org 是「人/组织」维度，project 是「需求/仓库」维度，两者不混）：

| dept_level | dept 示例 | 看板 org 列 |
|---|---|---|
| 1 公司 | 深信服科技股份有限公司 | org1 |
| 2 体系 | 研发体系 | org2 |
| 3 部 | Costrict研发部 | org3 |
| 4 组 | 开发组 / 客户成功组 | org4 |

- **org 维度**：直接吃 dept_level 1-4 → org1-4（与现有 `getGroupDetailV2` 的 org1..org4 入参完全对齐，`org_handler_v2.go:811-851`）。`listOrgsV2Native` 仍只用 org1 展示顶层，符合现状。
- **project 维度**：**不映射** dept。project 是看板自己的需求 / 仓库聚合（`projects` 表，`models.go:181-211`，主键 uuid、含 repos/task_ids），与部门正交，本次接入不动。
- **职级（position）**：作为 `dept_user.position` 独立列落库；前端可在用户详情 / org 成员表加一列"职级"，属增量展示。

---

## 5. 鉴权对接

### 5.1 看板侧 casdoor 现状：未部署，用写死 shim

- 内网看板后端**没有真实 casdoor**：`backend/auth_shim_handler.go:3-7` 明确"内网决定不部署 casdoor，故这里直接返回写死的 admin 用户"；`/api/auth/me`、`/api/auth/permissions`、`/api/auth/logout` 全是占位（`main.go:157-161`）。
- compose 内未发现 casdoor 服务定义（`compose/docker-compose.yml` 等无 casdoor/JWT 配置）。
- 因此**看板与 dept-sync 的 casdoor JWT 口径不一致**：dept-sync `jwtSettings.enable=true`（userIdField=`properties.oauth_Custom_id`、走 casdoor），而看板内网根本没有 casdoor 会话可签发 JWT。

> 注：dept-sync 的 JWT 口径只在它自己的前端页面 `/costrict-dept-info/app/dept-tree` 和需要用户态的接口上生效；服务对服务调用应走密钥。

### 5.2 接入用 query_key / admin_key（与方案 B 完全契合）

- dept-sync 内置 `query_key` 表（schema 已确认）+ `admin_key`（`config.yaml` 默认 `123456`）。
- **方案 B 同步任务**：用 `query_key` 调 dept-sync 数据接口（`/api/v1/department/tree`、`/api/v1/...`），或干脆**离线读 SQLite 内置库 `dept_sync-inner.sqlite`**（一次性 / 离线导入，连密钥都不用）。
- `admin_key` 仅用于 dept-sync 自身管理（`/api/admin/keys` 维护 query_key、`/api/sync/trigger` 触发同步），看板不需要持有 admin_key。
- **结论**：方案 B 下看板**不需要也不应该**集成 casdoor JWT。同步任务持有一个 query_key（或直接读内置 SQLite）即可，密钥放 kbcli config（如新增 `dept_sync.query_key` / `dept_sync.base_url` / `dept_sync.sqlite_path`），不进镜像、走 config 挂载（与现有 org_dsn 同款保管方式）。

---

## 6. 落地步骤清单（最小改动 + 复用优先，分阶段）

### 阶段 0：验证锚点（✅ 已完成，2026-06-05）
- 实测结论：**universal_id 锚点 0/16 命中（废弃）；工号锚点正确**——`邓彬84569 / 林凯90331` 经 `commits.git_user_email` 前缀精确对上 dept-sync `user_department.user_id`。
- 验证 SQL：`SELECT split_part(git_user_email,'@',1) FROM commits WHERE git_user_email ILIKE '%@sangfor.com'` 得看板真实工号集（仅 5 个：14315/35882/74179/84569/90331），与 dept-sync 18 工号交集 = 2。低绝对命中是双边样例子集所致，非口径问题，全量库可解（见 §7）。

### 阶段 1：落库 + 投影到 user_org（零后端/前端改动，最快见效）
1. **DDL**：在 `core/models/models.go` 新增 `Dept` / `DeptUser` 两个 GORM model（§4.1），加入 `AutoMigrate`（`models.go:285-306`）。
2. **同步命令**：在 kbcli 新增 `import-dept` 子命令（**以 `cmd_import_org.go` 为基础**，复用 `replaceDBName/saveUserOrgs/写 user_org` 套路）：
   - 数据来路二选一：① 读 dept-sync 内置 SQLite（`dept_sync-inner.sqlite`，离线）；② 调 dept-sync `/api/v1/department/tree` + 人员接口（带 query_key）。
   - 写 `dept` / `dept_user` 两表（UPSERT，`clause.OnConflict`，同 `saveUserOrgs` 写法 `cmd_import_org.go:278-294`）。
   - **投影回填 `user_org`**：对每个 `dept_user`，先用 `emp_no`（工号）经 `commits.git_user_email` 反查看板 `user_id`（查不到则跳过），按其主部门 `dept_path` 拆 org1..org9（复用 `cmd_import_org.go:155-165` 拆段逻辑），`user_id=看板UUID`、`user_name=real_name` UPSERT 进 `user_org`。**严禁 `user_id=universal_id`**（0% 命中）。
3. **配置**：kbcli config 新增 `dept_sync.sqlite_path` / `dept_sync.base_url` / `dept_sync.query_key`（config 挂载，不进镜像）。
4. **刷新内存映射**：导入后调现有 `POST /api/v2/orgs/refresh`（`refreshOrgMappingV2`，`org_handler_v2.go:1049`）重载 `orgMappings`，或重启 backend（`main.go:66` 启动时加载）。
5. **验证**：`/v2/orgs` 不再 `no_org_mapping`；OrgList 出现真实"深信服/研发体系/Costrict研发部/开发组"层级；OrgDetail 级联 org1..org4 有值。用真实数据核对（非 MOCK，CLAUDE.md）。

> 阶段 1 完成后，后端 handler、前端 OrgList/OrgDetail 代码**一行不改**就有了真实部门维度。

### 阶段 2：真名 + 职级（小幅前后端改动）
6. **真名权威化**：后端新增轻量 `GET /api/v2/users/names`（返回 user_id→{real_name, position, dept_path}，从 `dept_user` 查），前端 `useUserNameMap.ts` 改为优先用它、commit 名兜底。
7. **职级展示**：org 成员表 / 用户详情加"职级"列（读 `dept_user.position`），属增量。

### 阶段 3（可选）：稳定 ID 树形维度
8. 新增基于 `dept` 表（`parent_dept_id` 递归 CTE / `dept_path` 前缀）的树形接口与前端下钻，规避改名失效。仅在业务需要"真正树"时做，不阻塞前两阶段。

### 阶段 4：运维
9. 同步频率与 dept-sync 凌晨 cron（`0 4 * * *`）对齐，看板侧排个稍晚的定时跑 `import-dept`（compose 已有手动跑 kbcli 的运维方式，见仓库内 kbcli compose ops 记录）。

---

## 7. 风险与未决项

1. **双边都是样例子集，绝对命中仅 2**：dept-sync 内置库只有 Costrict 18 行（全量约 2087 人需真实 HR/部门 API）；看板侧 sangfor 真实工号也只有 5 个（其余多为 UUID 命名的 bot/噪声数据）。两边交集 = 2（邓彬/林凯）。**这是数据完整度问题，不是口径问题**——全量 dept-sync 库 + 看板真实用户扩充后，5 个看板工号可全部覆盖。全量同步需提供 `provider.sangfor.hr_url/dept_url/hr_key/dept_key`（`config.yaml:41-47` 现为空）。
2. **工号桥接依赖 commits.git_user_email**：JOIN 锚点是工号，须经 `commits.git_user_email`（`工号@sangfor.com`）反查看板 user_id。**风险点**：① 用户若在看板无 sangfor 邮箱 commit（纯 UUID 用户、外部邮箱、bot）则桥接断裂，无法落 org；② universal_id 已证实 0% 命中，**不可作锚点**；③ dept-sync 缺 universal_id 的行（实习生朱海俊/31874、徐佳林/79944）不影响工号桥接（它们有工号），但这些人若也无 sangfor commit 仍会落空。
3. **深信服 HR API 密钥未提供**：`hr_key/dept_key`（AES 解密）、`hr_url/dept_url` 全空，全量同步暂不可行；阶段 1 用内置 SQLite 离线导入绕开此阻塞。
4. **职级 / 部门会变动**：position、归属随调岗变化。方案 B 有同步延迟（建议每日同步对齐 dept-sync cron）；历史看板数据是否需要"按当时部门"归属（缓变维度 SCD）属未决项，本期默认用最新归属。
5. **leader_id 是工号口径**：`department.leader_id`（如 29219）是工号，展示负责人姓名需再 JOIN `dept_user.emp_no`，且负责人本人不一定在 user_department 子集内（如 56953/29219 不在 18 行里）。
6. **多部门归属**：`user_department.is_main` 表明一人可属多部门，本设计只取 `is_main=1` 作主部门落 org；如需多归属展示需扩展 `dept_user` 为多行（去掉 emp_no 单一主键，改 emp_no+dept_id 复合键）。
7. **两套 org handler 的口径裂缝**：列表 native（org1）/ 详情多级（org1..org4）的不一致在阶段 1 后仍在；若要统一需后端改动，超出"最小改动"范围，列为后续技术债。
8. **casdoor 口径分歧**：看板内网无 casdoor、dept-sync 默认开 JWT。本设计用 query_key/离线 SQLite 规避；若未来看板真接 casdoor，可复用 dept-sync 同款 `oauth_Custom_id` 口径，但属另一议题。

---

## 附：关键文件索引

| 主题 | 文件:行 |
|---|---|
| user_org 模型 | `core/models/models.go:10-28` |
| AutoMigrate（加新表处） | `core/models/models.go:285-306` |
| org 导入 / 拆 org1..9 | `kbcli/cmd_import_org.go:45-90, 155-165, 278-294` |
| org_csv_file / org_dsn 配置 | `kbcli/config.go:170-177, 274-306`；`compose/kbcli/config.yaml:123-124` |
| org native handler（生效） | `backend/user_org_v2_native_handler.go:213-293` |
| org 多级 handler / 详情 / 分组 | `backend/org_handler_v2.go:181-205, 225, 613, 811, 1049` |
| LoadUserOrgs / orgMappings | `backend/db.go:1713`；`backend/org_handler_v2.go:129`；`backend/main.go:66` |
| org 路由绑定 | `backend/main.go:118-128` |
| 真名反推 hook | `frontend-react/src/hooks/useUserNameMap.ts:14-39` |
| kbcli 用户名三级回退 | `kbcli/cmd_efficiency.go:99-148` |
| 看板 auth shim（无 casdoor） | `backend/auth_shim_handler.go:3-34`；`backend/main.go:157-161` |
| dept-sync 内置库 schema | `temp/dept-sync/dept_sync-inner.sqlite`（department / user_department） |
| dept-sync 样例数据 | `temp/dept-sync/dept.csv`、`uuid_map.csv` |
| dept-sync 配置 | `temp/dept-sync/config.yaml` |
| 现有兜底 CSV（假数据） | `org_mapping.csv` |
