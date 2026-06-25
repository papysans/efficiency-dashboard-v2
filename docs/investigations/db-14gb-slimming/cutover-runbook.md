# 生产 cutover runbook：conversations 正文卸载 + conv_events 索引瘦身（DB 14GB→~3GB）

> 代码已全部完成并在本地真 PG 端到端验证（见 roadmap.md / tasks）。本 runbook 是 3 个外部确认齐备后的**逐步执行手册**，turnkey。
> 内网 DB：容器 `efficiency-dashboard-v2-postgres-1`，库 `costrict_stat`，user `postgres`，pw `1`。
> 诊断 SQL：同目录 `diagnostic-bundle.sql`。命令均填真实值、可直接复制。

诊断速查 helper（粘到内网 shell）：
```bash
dx(){ docker exec -e PGPASSWORD=1 efficiency-dashboard-v2-postgres-1 psql -U postgres -d costrict_stat -t -A -F ' | ' -c "$1"; }
```

---

## 阶段 0 — 三个外部确认（全绿才往下）
1. **诊断 SQL 跑完**（`diagnostic-bundle.sql`）：确认 conversations 三列真实占比、conv_events 组成、各索引 `idx_scan`（B1 节，复核索引瘦身无误伤）、floor 前后行占比（F1）。
2. **raw-dump 留存窗**（问运维）：`/mnt/.../raw-dump`（或 MinIO）保留多久。这是卸列/裁旧**可逆性的唯一支点**——上游若滚动删旧分片，删除不可逆。
3. **回看历史跨度**（口径负责人）：保留删除的窗口下界，**不能短于 episode 回看跨度**（need_boundary 要全量历史）。

---

## 阶段 1 — 🔴 头号前提：共享存储（否则卸载后 backend 读不到 blob）
**现状**：`compose/kbcli/kbcli.yml` 挂 `./analysed:/app/analysed`（kbcli 写卸载 blob）；但 `compose/server/server.yml` **只挂 task/repo，没挂 analysed** → 卸载后 backend `HydrateContent` 读 `/app/analysed/...` 失败 → 前端"对话历史"空白（best-effort 降级，不崩但显示空）。

**二选一，cutover 前必须做：**
- **方案 A（disk，最快）**：给 server 服务加上与 kbcli **同一宿主目录**的 analysed 卷（只读即可）。注意 compose `extends` 相对路径坑——两服务必须落到**同一个 host 路径**（建议改用 named volume 或写绝对路径），否则各挂各的空目录。
  ```yaml
  # compose/server/server.yml 的 server.volumes 增加（指向与 kbcli 相同的 analysed）：
  - <与 kbcli ./analysed 相同的宿主路径>:/app/analysed:ro
  ```
  校验：卸载若干行后，在 server 容器内 `ls /app/analysed/task/conversation/content/` 能看到 blob。
- **方案 B（S3，prod 更干净，推荐）**：kbcli + server 的 `analysed_dir` 改 `s3://<bucket>/...` 且都配 `storage.s3`（endpoint/ak/sk）指**同一 bucket**。两服务天然共享，无卷路径坑。content_location 变 `s3://...`，backend 直接读。

**校验前提满足**：随便卸载 1 行后，确认 backend 能读回（见阶段 5 前端抽查）。

---

## 阶段 2 — 部署新代码（顺序铁律：先全部署，再卸载）
> ⚠️ **A5 严禁先于 A6**：必须先把带 A6 回读的新 server+kbcli 镜像全部上线，**再**执行卸载。否则旧服务读到空正文 → efficiency-v2 解析退化污染主指标 / 前端空白。

```bash
# 拉新镜像并重启（按你现有 compose 部署流程）
docker compose pull server kbcli
docker compose up -d server kbcli
```
启动时 AutoMigrate 自动完成两件事（**这就是不需确认的 ~1-2GB 白拿**）：
- 给 conversations 加 `content_location` / `user_input_chars` 列（metadata-only，秒级不锁表）。
- conv_events 索引 14→4（DROP 10 个）。⚠️ 普通 DROP 短锁；若怕锁，**部署前**手工 `DROP INDEX CONCURRENTLY` 这 10 个（idx_conversation_events_{session_id,request_id,task_id,user_id,work_dir_id,event_kind,source,parse_quality,task_start,source_quality}）。

校验：
```bash
dx "SELECT count(*) FROM information_schema.columns WHERE table_name='conversations' AND column_name IN ('content_location','user_input_chars')"   # 应=2
dx "SELECT count(*) FROM pg_indexes WHERE tablename='conversation_events'"   # 应=4
```

---

## 阶段 3 — 卸载前快照（基线，用于 0 退化对比）
```bash
dx "SELECT pg_size_pretty(pg_database_size('costrict_stat'))"                                  # 库大小
dx "SELECT pg_size_pretty(pg_total_relation_size('conversations'))"                            # conversations 大小
dx "SELECT parse_quality, count(*) FROM conversation_events GROUP BY 1 ORDER BY 1"             # ★事件质量分布(基线)
dx "SELECT count(*) FROM conversation_events"                                                  # 事件总数(基线)
dx "SELECT count(*) FROM needs; SELECT count(*) FROM user_productivity_v2"                      # 主指标行数(基线)
```
记下 `parse_quality` 分布与各计数。

---

## 阶段 4 — 卸载 conversations 正文
```bash
# 先 dry-run 看待卸载行数（非破坏）
docker compose run --rm kbcli offload-content --dry-run
# 正式卸载（手动子命令即开关；幂等可重跑；失败行保持未卸载）
docker compose run --rm kbcli offload-content
```
校验卸载生效：
```bash
dx "SELECT count(*) FILTER (WHERE content_location<>'' AND content_location IS NOT NULL) AS offloaded,
        count(*) FILTER (WHERE COALESCE(request_content,'')<>'') AS still_has_req FROM conversations"
# offloaded 应≈有正文的行数；still_has_req 应大幅下降
```

---

## 阶段 5 — 🔴 0 退化铁门槛（不通过立即回滚）
```bash
# 重跑 efficiency-v2（走 A6 回读；按你现有 efficiency-v2 命令/口径）
docker compose run --rm kbcli efficiency-v2
# 对比事件质量分布与基线——必须一致(尤其 exact 不减、degraded 不增)
dx "SELECT parse_quality, count(*) FROM conversation_events GROUP BY 1 ORDER BY 1"
dx "SELECT count(*) FROM conversation_events"
dx "SELECT count(*) FROM needs; SELECT count(*) FROM user_productivity_v2"
```
- **事件总数 / parse_quality 分布 / needs / user_productivity_v2 与阶段3基线一致 = 通过。**
- 前端抽查任一任务详情页"对话历史"：正文正常显示（验证 backend 共享存储读回 OK）。
- **任一项退化（exact 减少 / degraded 增加 / 前端空白）→ 立即回滚（见末尾）。** 多半是阶段1共享存储没配对。

---

## 阶段 6 — 空间回收（卸载只置空列，物理空间需手动收）
```bash
# UPDATE 置空产死元组，pg_relation_size 不自动降。VACUUM FULL 锁表，低峰执行：
dx "VACUUM FULL conversations"
dx "SELECT pg_size_pretty(pg_database_size('costrict_stat')), pg_size_pretty(pg_total_relation_size('conversations'))"
# 库应从 ~14GB 降到 ~9GB 级（conversations 正文 ~5.4GB 移出）
```
> 大表 VACUUM FULL 锁表久可改 `pg_repack`（不锁）。

---

## 阶段 7 — 保留删除（裁旧，gated 回看窗口；可选，进一步降到 ~3GB 级）
```bash
# 回看窗口下界由阶段0确认。clean 按 start_time/event_start_ts 删 conversations + conversation_events 旧行。
# 先 dry-run：
docker compose run --rm kbcli clean --before 2026-05-25 --dry-run
docker compose run --rm kbcli clean --before 2026-05-25
dx "VACUUM FULL conversation_events"   # 裁旧后回收
```
> ⚠️ `--before` 必须 = 回看窗口下界，不能短于 episode 回看跨度（否则毁提效口径）。删除不可逆——依赖阶段0的 raw-dump 留存窗确认。

---

## 阶段 8 — 常态化（防再涨回 14GB）
- offload-content + clean 接入定时（确认 raw-dump 留存窗后）：每轮 import→efficiency-v2 后跑 offload-content；按窗口定期 clean。
- 详见 design.md「激活硬门槛」#5（clean 删 conversations 行须同步清 content_location 指向的 blob，避免孤儿——cron 化前补这条）。

---

## 回滚（按阶段）
- **阶段 2（部署）**：回滚镜像 `docker compose up -d` 旧 tag。新增列/删索引无害（旧代码不读新列；索引可 `CREATE INDEX` 重建）。
- **阶段 4-5（卸载后退化）**：① 关键——content 仍在磁盘 blob，未丢；② 退化多因共享存储没配对，修阶段1后重跑 efficiency-v2 即恢复；③ 真要回退卸载：`import -f` 从 raw-dump 重扫重建 DB 正文（依赖 raw-dump 留存）。
- **阶段 7（裁旧）**：不可逆；只能 `import -f` 从 raw-dump 重建（依赖留存窗）。故执行前务必确认阶段0。

## 激活硬门槛 checklist（务必逐条，详见 design.md「激活硬门槛」）
- [ ] A5 不先于 A6（先全部署带 A6 的新镜像，再卸载）
- [ ] 阶段1 共享存储配对（kbcli 写 / backend 读同一 analysed 或同一 S3 bucket）
- [ ] 阶段5 逐项 0 退化对比通过 + 前端抽查
- [ ] 阶段6 VACUUM 回收 + 库大小核对
- [ ] 裁旧/接 cron 前确认 raw-dump 留存窗 + 回看窗口
