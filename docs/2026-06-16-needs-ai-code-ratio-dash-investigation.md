# Needs 看板「AI 代码占比」满屏 "-" 勘探报告

- 日期：2026-06-16
- 范围：内网生产库 `costrict_stat`（容器 `efficiency-dashboard-v2-postgres-1`），列表口径取自 `/kanban/needs-v2`（startDate=20260318, endDate=20260615）
- 结论一句话：**满屏 "-" 不是 bug、不是过滤失效，而是 need 边界把跨仓库/跨分支的 AI 开发切成碎片（约 60% 用了 AI 但配不上）+ 约 40% 提交真没用 AI。看板 `repo_addr` 归一化与 need 聚合本身工作正常（漏聚仅 0.4%）。**

---

## 1. 问题

需求列表「AI 代码占比」列大面积显示 "-"（内网约 84% 的行）。诉求：搞清根因，且此前已做过「排除没采集数据的人」，为何仍满屏 "-"。

## 2. 数据来源与方法

- **内网生产库** `costrict_stat`（一切结论以此为准）。
- 本地 `costrict_stat_archive` 是 06-01~06-10 的 dump 副本，**仅用于验证 SQL 语法**；本次勘探中它多次给出与内网相反的比例（见 §7 教训），**不可代表内网**。
- 统一诊断入口（团队后续沿用，见记忆 `intranet-dx-helper`）：
  ```bash
  dx(){ docker exec -e PGPASSWORD=1 efficiency-dashboard-v2-postgres-1 psql -U postgres -d costrict_stat -t -A -F ' | ' -c "$1"; }
  ```

## 3. 关键机制（代码实证）

1. **「AI 代码占比」列读的是 `ai_code_ratio`**（`frontend-react/.../NeedList.tsx:439` `row.ai_code_ratio || null`）——**0 和 null 都渲染成 "-"**（注释："0=silica 无数据非真 0"）。
2. **`ai_code_ratio = aiCoveredLoc / changedLoc`**（`efficiency_v2_need_aggregate.go:420`），`aiCoveredLoc` 判定是 **`covered-rule: temporal-only`**——只看 commit 提交时刻是否落在某 AI 会话时间窗内，**不看代码指纹**。这与 `silica`（真·指纹匹配）是两个字段。
3. **commit 来源 = csc 客户端每次 `git commit` 上报一个 JSON**（`RepoCommitData`, `cmd_import_repo.go:25`，字段仅 commit/repo/branch/diff/作者等，**无任何 AI/会话关联字段**）→ 无差别上报所有提交，"是否 AI" 全靠看板事后用 `commit_time` 套会话时间窗推断。
4. **need 边界**（`efficiency_v2_need_boundary.go`）：候选先过 `governance.CanonRepoAddr`（`canon.go`：小写 / 去 `ssh://git@`·`https://` / 剥凭据 / scp 冒号转 `/`（端口保留）/ 去 `.git`）归一化，再按 **`(canon仓库 + 分支)`** 分桶聚合成 need。`need.repo_addr` 存归一化值；`session_stage_metrics.repo_addr` 存原始值。
5. **列表口径**（`backend/needs_v2_handler.go` + `db.go:1458 applyNeedCaliberFilter`）：`status<>active` + 非主干分支 + **软件用户人级过滤**（`needSoftwareUserCaliberSQL`：primary_user_id ∈ 有过带 session need 的用户集）+ `NOT outlier_flag`。**不过滤 `coverage_eligible`**——后者只在 KPI 聚合里 `FILTER`。

## 4. 完整成因谱（内网真数，关键数经原始数据人工核对）

列表口径下 commit-only need ≈ **2928** 个（满屏 "-" 的主体）。用看板**同款 CanonRepoAddr + 分支桶**重判：

| 类别 | 数量 | 占比 | 含义 | 是否用了 AI |
|---|---|---|---|---|
| 完全没会话 | 1178 | 40.2% | 提交那几天作者零会话 | **真没用 AI** |
| 会话在别物理仓库 | 918 | 31.4% | 多仓库工作，会话标到别子仓库 | 用了（被切散） |
| 切分支提交 | 820 | 28.0% | 会话在同仓别分支 | 用了（被切散） |
| 同仓同分支漏聚 | 12 | 0.4% | 看板该聚没聚（真 bug） | 用了（漏聚） |

**聚合**：约 **60%（1750）作者其实在用 AI**，只是被 need 边界切散配不上；**真没用 AI 仅 40%（1178）**。

自洽校验：「同期有会话」820+918+12=1750 ≈ 早先独立查的 1736；「完全没会话」1178 ≈ 1086（带日期口径差异）。

## 5. 根因

**真实 AI 开发是跨仓库、跨分支的「项目级」活动，而 need 边界按「单个物理仓库 + 单个分支」切——把一个人的 AI 工作切成碎片，碎片之间配不上。** 叠加 40% 确实没用 AI。

典型个案：作者 `de594e67` 有 62 次会话全标在 `phxoss.git`（分支 `person-532-vectorstore-dev-44060-*`），而提交落在同项目的 `vector_store` 仓库 → 两个物理仓库 → need 配不上 → `vector_store` need 显示 "-"。（EDS/x86 下 phxoss / vector_store / phxdfs / phxrpc / ceph / eds-deps 是一组子仓库、同一开发流。）

## 6. 「排除过为什么还有这 40%」

那次排除是**人级**（`needSoftwareUserCaliberSQL`，排掉"整个人从没 session"的用户）+ `coverage_eligible`（排出统计、不排列表），**都不删单个"没 session 的 need"**。这 40% 的作者都是软件用户（有别的带 session need），只是这次提交纯手写（同期零会话）→ commit-only → 显示 "-"。即：排掉了"从不用 AI 的人"，没排掉"用 AI 的人偶尔手写的那次提交"。

## 7. 方法教训（重要，避免重蹈）

1. **`repo_addr` 归一化格式陷阱（制造过 1736 假象）**：`need.repo_addr` 存 CanonRepoAddr 归一化值（小写/去 .git），`session_stage_metrics.repo_addr` 存原始值（`git@..git`），两表精确相等交集仅 **1/1903**。直接 `m.repo_addr = need.repo_addr` 比，会把"同仓库不同写法"全误判成"repo 错位"，一度得出"1736 个 repo 错位、该大改关联"的错误结论。**必须两边都过同一 canon 再比。**
2. **archive 副本反复误导**：archive 给"85% 如实/15% 冤"，内网却是相反；archive `same_repo` 非 0，内网为 0。**凡结论必须落到内网真数。**
3. **个案泛化翻车**：曾把"时区误标 clamp"当系统性问题，实测全库仅 2/8063 个 need 中招（0.06%）。
4. **calendar 机制一度讲错**：`total_calendar_min = dev_end - dev_start`（减 >3 天空档，`IdleThresholdDays=3`），不是"活跃段累加 / idle 30 分钟"。
5. **诊断 SQL 复制坑**：超长多行 + 正则转义（`\.git$` 的 `\` 和 `$`）在终端粘贴易断；改用**纯 `replace` 链 + 压成单行**最稳。
6. **方向性结论反复横跳**（折叠→修关联→否决修关联）的根因：数据/方法未坐实就下倾向判断。**纪律：聚合结论必须用原始数据抽样人工核对后才采信**（本次最终对 10 个 no_session 样本逐条核对 NEED 仓库不在会话仓库列表，才确认 71.6% no_session 判定无误）。

## 8. 方向建议（分层）

| 对象 | 量 | 措施 | 代价 |
|---|---|---|---|
| 完全没用 AI | 40%（1178）| 列表折叠 + 作为「AI 渗透率」真实分母 | 小 |
| 切分支提交 | 28%（820）| need 边界跨分支归并（注意 `need-split-by-branch-switch` 是已知设计） | 中 |
| 多仓库工作 | 31%（918）| need 边界升「项目=一组仓库」级归并 | 大 + 口径决策（先定义"项目=哪些仓库"）|
| 漏聚 bug | 0.4%（12）| 忽略 | — |

- **修 commit↔session 精确 repo 关联**：否决（仅 0.4% 收益）。
- 关联记忆：`ai-code-ratio-temporal-covered`、`intranet-dx-helper`、`silica-bare-code-fix`、`archive-dataset-eval`、`need-split-by-branch-switch`。

## 9. 关键诊断 SQL 附录

```sql
-- A. 列表口径 commit-only 构成（强可救/切分支/无会话三分，看板同款 canon）
WITH co AS (
  SELECT need_id, primary_user_id,
    replace(replace(replace(replace(replace(replace(lower(repo_addr),'ssh://',''),'https://',''),'http://',''),'git@',''),'.git',''),':','/') cr,
    lower(trim(coalesce(repo_branch,''))) br, dev_start_ts, dev_end_ts
  FROM needs
  WHERE status<>'active'
    AND NOT (LOWER(TRIM(COALESCE(repo_branch,''))) IN ('main','master','develop','release') OR LOWER(TRIM(COALESCE(repo_branch,''))) LIKE 'release/%')
    AND NULLIF(primary_user_id,'') IN (SELECT DISTINCT primary_user_id FROM needs WHERE COALESCE(primary_user_id,'')<>'' AND session_ids IS NOT NULL AND session_ids::text NOT IN ('[]','null',''))
    AND NOT outlier_flag
    AND COALESCE(jsonb_array_length(session_ids),0)=0
), x AS (
  SELECT co.need_id,
    EXISTS(SELECT 1 FROM session_stage_metrics m WHERE m.user_id=co.primary_user_id AND m.session_start_ts::date BETWEEN co.dev_start_ts::date-2 AND COALESCE(co.dev_end_ts,co.dev_start_ts)::date+2 AND replace(replace(replace(replace(replace(replace(lower(m.repo_addr),'ssh://',''),'https://',''),'http://',''),'git@',''),'.git',''),':','/')=co.cr) AS sr,
    EXISTS(SELECT 1 FROM session_stage_metrics m WHERE m.user_id=co.primary_user_id AND m.session_start_ts::date BETWEEN co.dev_start_ts::date-2 AND COALESCE(co.dev_end_ts,co.dev_start_ts)::date+2 AND replace(replace(replace(replace(replace(replace(lower(m.repo_addr),'ssh://',''),'https://',''),'http://',''),'git@',''),'.git',''),':','/')=co.cr AND lower(trim(coalesce(m.repo_branch,'')))=co.br) AS srb
  FROM co
)
SELECT count(*) total,
  count(*) FILTER (WHERE srb) strong_recoverable,
  count(*) FILTER (WHERE sr AND NOT srb) cross_branch,
  count(*) FILTER (WHERE NOT sr) no_session
FROM x;
-- 内网结果：total=2928  strong=12  cross_branch=820  no_session=2096

-- B. no_session 再二分（会话在别物理仓库 / 完全没会话）：把 x 的 WHERE NOT sr 子集
--    再按"作者同期是否有任何会话"FILTER → 内网 918 / 1178
```

> 关联诊断 JOIN 链：`needs.commit_ids`/`session_ids`(jsonb) + `session_stage_metrics.need_id`(+`session_start_ts`/`session_end_ts`)；commits 主键 `commit_id`；need 有效行列 `total_loc_net`。
