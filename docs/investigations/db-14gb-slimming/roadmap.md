# 14GB 数据库瘦身路线图（2026-06-24 调研定稿）

> 来源：两轮 workflow 盘点（session tasks `wt3j6l0ce` 正文卸载 + `w4mxk8uf3` 全库总盘点），均含对抗审查。仅 V2 本仓。诊断 SQL 见同目录 `diagnostic-bundle.sql`。

## DB 画像
库总 **14 GB** = `conversations`(7.7GB) + `conversation_events`(6.2GB) **两表占 98%**，其余 20 张表全部 <300MB。
- `conversations`：369 万行；`request_content` 3.7GB + `user_input` 1.3GB + `response_content` 445MB + 结构列/5 索引 ~2.4GB。
- `conversation_events`：**100% 派生缓存表**（可由 conversations 重算），serving 层零引用；payload 仅 641MB，其余 ~5.5GB = `command_text` + 结构列 + **14 个索引**（实测，非 4；GORM 隐式索引被漏数）。

## 🔴 根因：没有有效的数据保留
生产从 `analysis_start_date`(20260525) 起原始数据无限增长，因为：
1. `kbcli/cmd_clean.go:50` 删的是 **`task_conversations`**（`fmt.Sprintf` 拼字面量），但真实表名是 `conversations` → 删不存在的表，报错。
2. `clean` 命令**未接 cron**（定时任务只有 import/fix-task/fix-commit）。
3. `conversation_events` **不在 clean 列表里**，从无清理。
→ 纯 bug 修复 + 接 cron 是最便宜的止血，但**上 cron 前必须确认 raw-dump 留存窗**（见开放问题）。

## 杠杆路线图（对抗审查已修正，verdict=needs_adjustment）

| # | 杠杆 | 目标 | 手法 | 预计省 | 工作量 | 风险 | 关键约束 |
|---|---|---|---|---|---|---|---|
| 0 | 跑诊断包 | 两表/索引/膨胀/floor 切分 | — | 把估算变实测 | S | 低 | 加 statement_timeout，重扫低峰跑 |
| 1 | 回收 conversation_events | 6.2GB 派生表 + 14 索引 | prune | ~6GB（裁旧≥4GB） | S | 低→中 | **不能裸 TRUNCATE**（见下） |
| 2 | conversations 正文卸载（任务 06-24-db-conversations-disk）| req+user+resp 5.5GB | 卸列 | ~5.4GB | L | 中 | 卸列+回读非同事务/DoNothing/新chars列 |
| 3 | 删 conv_events 冗余/未命中索引 | idx_scan=0 的 + ux_logical | drop_index | 1-3.5GB | S | 低 | 仅当 rank1 选"保留表只删旧行"才需；进 migration + CONCURRENTLY |
| 4 | 修 cmd_clean bug + 接 cron 常态保留 | cmd_clean.go:50 | prune | 防再涨回 14GB | S | 中 | 先确认 raw-dump 留存窗 |
| 5 | conv_events 瘦列（payload 只存 char_count / command_text 截断）| 641MB+ | trim_column | 百MB~GB | M | 低 | 仅当保留表不 TRUNCATE 才有意义 |
| 6 | 两表按月 RANGE 分区 + DROP 旧分区 | 替代批量 DELETE | partition | 秒级回收 | XL | 高 | **events 唯一索引×分区键冲突**（见下）|
| 7 | 大列改 lz4 + autovacuum 调勤 | 两表大列 | compress | CPU 为主 | S | 低 | 先验生产 build 带 lz4；存量需 rewrite |

## 对抗审查的 4 个硬伤（落地前必避）
1. **"cron 自动重建 events"对 floor 前事件不成立**：cron import 套 floor(20260525)，efficiency-v2 只重建 `start_time>=floor` 的事件。裸 TRUNCATE → **floor 前事件永久丢失**，必须**手工跑一次显式空窗（start/end 都留空触发夹紧全量）的 efficiency-v2** 才全量重建，且会触发远超几分钟的全量 LLM/kNN 重算（429 风险）。
2. **rank1 与 rank2 省量不可叠加**：conv_events(6.2GB) 派生自 conversations 三列(5.5GB)；要重建 events，这三列必须留库当源。不是独立的 11.7GB。
3. **TRUNCATE 与重建非同事务**：中途失败（LLM 429/被 kill）→ events 长期为空、stage_metrics 失输入。应 DELETE 旧行或 advisory-lock 保护，不裸 TRUNCATE。
4. **分区对 events 不"约束友好"**：`ux_conversation_events_logical`(6列唯一) + `event_id` 主键都不含 `event_start_ts` → RANGE 分区会报错，须把分区键塞进所有唯一约束、改 OnConflict 语义。

## 卸列侧补充硬伤（来自正文卸载调研）
- **卸列必须同时改 kbcli efficiency-v2 events 重建读取路径走 storage 回读**，不只 backend 懒加载——否则重算读到空串 → 同源 26% 退化。
- 修 `OnConflict DoNothing`（击穿迁移/回滚）+ 新建 `user_input_chars` 真列（pseudo_task 估时分母）+ 落盘非原子（temp+rename）+ VACUUM FULL 回收空间。

## 推荐顺序（修正后）
0. 跑诊断包（实测 floor 切分/索引/膨胀）+ 向运维确认 raw-dump 留存窗。
1. **修 cmd_clean.go:50 表名 bug**（纯 bug，零口径风险，止血）。
2. 回收 conv_events：若 F1 证实 metrics 相关 conversations 全 ≥floor → 可 TRUNCATE，但**回收后立刻手工跑显式全量 efficiency-v2 并断言 needs/stage_metrics/user_productivity_v2/parse_quality 守恒**；若 floor 前有需保留回看的历史 → 改 DELETE 旧行（保留窗由口径负责人按 episode 回看跨度拍板）+ CONCURRENTLY drop idx_scan=0 索引。→ 库降到 ~8GB。
3. 固化常态：cmd_clean 修好接 cron（确认 raw-dump 留存窗后）。
4. 中期：推进 conversations 正文卸载任务 → 库再压到 ~3GB 量级。
5. 长期可选：分区（先解唯一索引×分区键冲突）/ lz4。

## 开放问题（需用户/运维/口径负责人拍板）
1. **raw-dump(/mnt/user-indicator/raw) 留存窗多久？** 所有裁旧/卸列可逆性的唯一支点——上游若也滚动删旧分片，删除变不可逆，cmd_clean 不能上 cron。
2. **回看历史跨度定多长？** episode 分桶要全量历史（need_boundary 注释禁加日期窗），任何按时间裁剪/分区 DROP 的保留窗不能短于业务回看跨度，否则毁提效口径。
3. **目标库尺寸？** 只清 events→~8GB；再卸 conversations 正文→~3GB。
4. **VACUUM FULL/pg_repack 锁表窗口何时排？**（TRUNCATE 例外不需要）

关联记忆：[[db-offload-content-to-disk]]。
