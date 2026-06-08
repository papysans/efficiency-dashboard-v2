# main_v1_fullrun_output.json 数据分析

## 1. 结论

`main_v1_fullrun_output.json` 不是一份可直接用于评估 AI 提效的有效 v1 结果；它更适合作为“main 分支字段输出基线”和“当前 raw 数据进入 main 后会丢失哪些信号”的证据。

核心原因：

1. `tasks = 0`，`conversations.task_id` 全为空，说明 conversation 与 commit 没有形成 silica 关联。
2. `commits.silica > 0` 的记录为 0，`commit_real_*` 与 `commit_ancient_minutes` 基本没有有效估算结果。
3. `user_productivity` 570 行全部来自 commit 日聚合，没有 task 侧贡献。
4. main 没有保存 commit raw 的 `files`，所以输出 JSON 中没有 `files/touched_files`。

因此，这份 JSON 说明：在这批 raw 数据上，main v1 的可用输出主要是“谁在哪天提交了多少 commit / diff_lines”，不是“AI 覆盖了多少工作量”。

## 2. 数据规模

| 表 | 行数 | 说明 |
|---|---:|---|
| `sessions` | 12114 | 会话 summary 已入库 |
| `conversations` | 17073 | 对话 JSONL 已入库 |
| `commits` | 2616 | commit 主键去重后的结果 |
| `tasks` | 0 | 没有生成 silica task |
| `user_org` | 129 | 本次由入库用户生成临时 org CSV |
| `user_productivity` | 570 | 按用户/日期聚合结果 |

时间范围：

| 数据 | 最早 | 最晚 |
|---|---|---|
| sessions | 2026-05-18 08:26:14 | 2026-05-22 12:09:15 |
| conversations | 2026-05-18 08:26:14 | 2026-05-22 12:09:15 |
| commits | 2025-12-31 10:20:23 | 2026-05-22 10:03:11 |
| user_productivity | 2025-12-31 | 2026-05-22 |

明显不对齐：conversation 只有 2026-05-18 到 2026-05-22，但 commit 覆盖 2025-12-31 到 2026-05-22。main 用时间窗口和指纹做关联时，这种覆盖范围不一致会让大量 commit 只能作为裸 commit 存在。

## 3. 字段覆盖与质量

### sessions

| 指标 | 数量 |
|---|---:|
| sessions 总数 | 12114 |
| 有 user_id | 12114 |
| 有 user_name | 12114 |
| 有 client_id | 12114 |
| distinct user_id | 25 |

会话身份字段完整，但分布极不均衡。Top 1 用户有 11981 个 session，占全部 session 的 98.9%。这会让按 session 侧统计的任何结果高度偏向单个用户。

Top session 用户：

| user_name | sessions |
|---|---:|
| eb58a46a-8503-43d0-9f6a-b076a80c79c4 | 11981 |
| mini2s | 37 |
| XDfield | 15 |
| f4582e2f-7f9c-4fe4-97c5-2199c7649ac1 | 9 |
| dengbinbox | 9 |

### conversations

| 指标 | 数量 |
|---|---:|
| conversations 总数 | 17073 |
| 有 session_id | 17073 |
| 有 request_id | 17073 |
| 有 repo_addr | 16824 |
| 缺 repo_addr | 249 |
| 有 repo_branch | 17044 |
| 缺 repo_branch | 29 |
| 有 work_dir | 17073 |
| 有 user_input | 11482 |
| 有 request_content | 11494 |
| 有 response_content | 12597 |
| request/response 都为空 | 151 |
| diff_lines > 0 | 0 |
| 有 task_id | 0 |

主要问题是 `diff_lines` 全为 0。main 的 silica 依赖 conversation 侧 diff 指纹；没有 conversation diff 行，就无法把对话和 commit 可靠关联起来。

conversation 成本合计为 214.39，但由于没有 task/silica 关联，这部分成本没有进入最终 `user_productivity.cost`。

按天分布：

| 日期 | conversations | cost |
|---|---:|---:|
| 2026-05-18 | 1489 | 42.94 |
| 2026-05-19 | 14908 | 144.20 |
| 2026-05-20 | 402 | 5.10 |
| 2026-05-21 | 269 | 22.10 |
| 2026-05-22 | 5 | 0.04 |

### commits

| 指标 | 数量 |
|---|---:|
| commits 总数 | 2616 |
| 有 user_id | 2616 |
| 有 repo_addr | 2616 |
| 有 repo_branch | 2616 |
| 有 work_dir | 2616 |
| 有 comment | 2616 |
| diff_lines > 0 | 2360 |
| diff_lines = 0 | 256 |
| silica > 0 | 0 |
| distinct user_id | 113 |
| distinct repo_addr | 254 |
| distinct repo_branch | 237 |

commit 基础字段完整，`diff_lines` 也有足够信号：总 diff_lines 为 1,229,519，p50=25，p90=630，p99=9230，最大值 112,938。

但 main 没有保存 raw `files`，也没有形成 silica，所以 commit 侧只能做代码量统计，不能还原需求边界或 AI 覆盖。

diff_lines 分布：

| 指标 | 值 |
|---|---:|
| min | 0 |
| p50 | 25 |
| p90 | 630 |
| p99 | 9230 |
| max | 112938 |
| diff_lines > 10000 | 25 |
| diff_lines > 50000 | 1 |

按 diff_lines 排名前 10 的用户：

| user_name | commits | diff_lines |
|---|---:|---:|
| SaiD2z | 18 | 171436 |
| jyolo | 28 | 93667 |
| heyifei | 14 | 86429 |
| 8290708c-1911-4d7a-b064-403f3396f820 | 12 | 63514 |
| XDfield | 411 | 56501 |
| whliucitictel | 14 | 46942 |
| 99c03cf7-ba5f-4680-8ece-691df6d84411 | 6 | 34778 |
| b02e580c-0bb7-48dc-be2d-c2019c3cfe90 | 28 | 29899 |
| 6be60285-2b5a-4b6f-b3d9-7a716e98416b | 101 | 28630 |
| mini2s | 160 | 27695 |

按 commit 数排名前 10 的用户：

| user_name | commits | diff_lines |
|---|---:|---:|
| XDfield | 411 | 56501 |
| uwuclxdy | 191 | 27409 |
| mini2s | 160 | 27695 |
| 6be60285-2b5a-4b6f-b3d9-7a716e98416b | 101 | 28630 |
| f4582e2f-7f9c-4fe4-97c5-2199c7649ac1 | 98 | 14985 |
| linkai0924 | 74 | 7740 |
| dengbinbox | 69 | 23243 |
| dc11292a-8086-4bf0-82db-13bfdeeebbd1 | 59 | 9426 |
| zbchun | 59 | 15782 |
| ZongruiL | 50 | 2296 |

按 repo diff_lines 排名前 10：

| repo_addr | commits | diff_lines |
|---|---:|---:|
| `https://SaiD2z:ghp_...@github.com/SaiD2z/EMOS2.git` | 18 | 171436 |
| `https://git.yy8.pw/jyolo/coin-trade.git` | 28 | 93667 |
| `https://gitee.com/autumnus/jiaofu-server.git` | 15 | 86434 |
| `http://192.168.1.4/eWorldCloud/WebFrontEnd/CallingClient.git` | 10 | 63491 |
| `https://gitee.com/plutooo/rsiic_-mk8_-v1.git` | 28 | 29899 |
| `https://github.com/limpeter1631/inventory-template.git` | 11 | 26761 |
| `https://ubgitlab.dev.ctt/development/sourcesafe/linux/ramdb.git` | 5 | 25645 |
| `git@gitee.com:yaajun/present.git` | 19 | 22433 |
| `https://gitee.com/autumnus/jiao-fu-web.git` | 8 | 22092 |
| `https://github.com/IronRookieCoder/docs-extractor.git` | 2 | 21979 |

安全注意：导出 JSON 保留了完整 `repo_addr`，其中有 18 条 commit 记录包含 token-like repo URL，涉及 1 个唯一 repo 地址。该 JSON 不应直接外发，除非先脱敏。

### user_productivity

| 指标 | 数量 |
|---|---:|
| user_productivity 总数 | 570 |
| distinct user_id | 113 |
| distinct days | 48 |
| task_count > 0 | 0 |
| commit_count > 0 | 570 |
| commit_count 合计 | 2616 |
| commit_diff_lines 合计 | 1229519 |
| task_count 合计 | 0 |
| task_diff_lines 合计 | 0 |
| cost 合计 | 0 |

`user_productivity` 与 `commits` 在 commit_count 和 commit_diff_lines 上是守恒的：

- `sum(user_productivity.commit_count) = 2616 = commits 行数`
- `sum(user_productivity.commit_diff_lines) = 1229519 = sum(commits.diff_lines)`

但由于 `tasks=0`，所有 task 侧字段都是 0；由于 `silica=0`，commit 侧的实际/传统分钟数和效率比也没有有效业务含义。

按天 commit 分布最密集的区间是 2026-05-05 到 2026-05-13，尤其：

| 日期 | commits | diff_lines |
|---|---:|---:|
| 2026-05-09 | 203 | 92381 |
| 2026-05-11 | 208 | 95054 |
| 2026-05-12 | 274 | 96947 |
| 2026-05-13 | 197 | 194009 |

## 4. 表间一致性

一致的部分：

- `conversations` 都能关联到 `sessions` 的 session 范围，distinct conversation session 为 12114，与 sessions 行数一致。
- `user_productivity.commit_count` 合计等于 `commits` 行数。
- `user_productivity.commit_diff_lines` 合计等于 `commits.diff_lines` 合计。
- `user_productivity` 用户数 113，与 commit 用户数一致。

断裂的部分：

- `conversations.task_id = 0`，没有任何 conversation 被关联到 task。
- `tasks = 0`，没有形成 session + commit 的工作单元。
- `commits.silica > 0 = 0`，没有 AI 覆盖信号。
- `conversation.cost` 有 214.39，但 `user_productivity.cost` 为 0，成本没有进入最终聚合。

## 5. 对 main 输出能力的判断

这份 output JSON 证明 main v1 在当前 raw 数据上能稳定产出以下东西：

- session 明细
- conversation 明细
- commit 明细
- 用户/日期 commit_count
- 用户/日期 commit_diff_lines
- 用户组织映射

不能有效产出以下东西：

- task 级工作单元
- AI 覆盖率 silica
- AI 成本按用户归集
- task 侧真实/传统工时
- commit 侧有效真实/传统工时
- 基于需求的聚类或边界
- commit touched files

## 6. 对 v2 的含义

这份 JSON 支持两个判断：

1. 只拿 main v1 输出字段，不足以做 v2 Need 级分析。因为 main 没有 Need、没有 touched_files、没有可靠 task/silica 关联。
2. 如果要用现有 raw 做 v2，`mnt 2` repo raw 里的 `files` 是有价值的额外信号；它不是 main 输出字段，但可以补上主干 commit 的文件聚类能力，减少低置信 orphan。

因此，v2 对比 main 时应把 main output 视为“v1 可见字段基线”，不能视为“完整上游信号基线”。

