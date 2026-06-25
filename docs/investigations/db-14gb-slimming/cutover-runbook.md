# 生产 cutover runbook：DB 14GB→~3GB（内联卸载 + 索引瘦身 + 裁旧）

> 适配「**只能更新 3 个镜像、只能跑 compose 命令、碰不到 psql**」的内网环境：所有诊断与操作都是 kbcli 子命令，经 `kb <cmd>` 触发。代码已全部在本地真 PG 端到端验证。
> 治本逻辑（学 V3）：import **内联卸载**——正文从源头落盘、DB 只留指针，不进热库；配合裁旧，行数 bounded，重扫不再涨回。

## 部署是安全的（pull 镜像只做加列，破坏性操作都手动触发）
拉新镜像启动后，AutoMigrate 只做**安全变更**：给 conversations 加 `content_location`/`user_input_chars` 列（metadata-only 秒级）+ 建 `idx_event_start_ts`。**不自动删索引、不自动卸载**——这些是显式命令。

```bash
docker compose pull server kbcli         # 3 个镜像（含 frontend 若有改动）
docker compose up -d server kbcli
```

---

## 阶段 0 — 三个外部确认（裁旧/常态化前必备）
1. **raw-dump 留存窗**（运维）：裁旧/卸列可逆性的唯一支点。
2. **回看历史跨度**（口径负责人）：裁旧窗口下界，不能短于 episode 回看跨度。
3. 诊断 → 见阶段 1（用 db-diag，不用 psql）。

## 阶段 1 — 诊断（替代跑不了的 SQL，全程 compose）
```bash
kb db-diag          # 抽样估算，快；--full 精确
```
把输出**整段贴回**。重点看：A 节两表占比对账 14GB；B 节 conv_events 14 索引各自 idx_scan（确认待删的 10 个都=0）；D 节三列体量（卸载能省多少）；F 节 floor 切分（裁旧能删多少行）。
> ⚠️ idx_scan 看 B2 节 stats_reset：若太近（刚重启/reset）idx_scan 不可信，需观察一个 import→efficiency-v2 周期再看。

## 阶段 2 — 索引瘦身（db-diag 确认 idx_scan=0 后）
```bash
kb slim-indexes --dry-run        # 列出将删的 10 个
kb slim-indexes --concurrently   # CONCURRENTLY 避锁，删
kb db-diag                        # H 节确认 conv_events 索引 14→4
```
省 ~GB，即时回收（DROP INDEX 直接释放）。删错可秒级 CREATE 重建（非正确性风险）。

## 阶段 3 — 开启内联卸载（常态化：从此 import 不再把正文写进 DB）
🔴 **前提**：server 能读回卸载 blob，二选一（compose server 默认**没挂** analysed）：
- **disk**：给 compose server 服务挂上与 kbcli **同一宿主路径**的 analysed 卷（`:ro`）；或
- **S3（推荐）**：kbcli+server 的 `analysed_dir` 改 `s3://bucket/...` + 配 `storage.s3` 指同 bucket。

然后在 **kbcli 配置**里开开关（改 `compose/kbcli/config.yaml`，随镜像/挂载更新）：
```yaml
content_offload:
  enabled: true
```
重启 kbcli 生效。**从此每次 import 的新对话正文直接落盘、DB 只留指针**——库不再因新数据涨。
> ⚠️ A5 不先于 A6：A6 回读已随镜像就位，开关开了即安全。开关只影响新导入；存量旧行见阶段 4。

## 阶段 4 — 一次性 backfill 存量 369 万行（把历史正文挪出 DB）
```bash
kb offload-content --dry-run      # 看待卸载行数(COALESCE 含 NULL 存量行)
kb offload-content                # 落盘+写指针+置空列，幂等可重跑
```

## 阶段 5 — 🔴 0 退化复核（卸载前后对比，全程 db-diag）
**卸载前**先存一份 db-diag 的 **G 节（parse_quality 分布）+ G2 节（needs/user_prod_v2 行数）** 作基线，卸载/重算后再跑一次对比：
```bash
kb efficiency-v2     # 重算(走 A6 回读)
kb db-diag           # G/G2 节必须与基线一致(exact 不减/degraded 不增)
```
+ 前端抽查任一任务详情「对话历史」正文正常显示（验 server 读回 OK）。**任一退化→回滚**（多半是阶段3共享存储没配对）。

## 阶段 6 — 空间回收
```bash
kb db-diag           # C 节看死元组；A 节看库/表大小
```
UPDATE 置空列产死元组，需一次 `VACUUM FULL conversations`（锁表，低峰）才真缩盘。
> ⚠️ VACUUM 目前需 DB 侧执行。若内网只能 compose，可加一个 `kbcli vacuum` 子命令——需要的话告诉我，我补。

## 阶段 7 — 裁旧（gated 回看窗口，进一步降到 ~3GB 级）
```bash
kb clean --before 2026-05-25 --dry-run   # --before = 回看窗口下界
kb clean --before 2026-05-25
```
按 start_time/event_start_ts 删 conversations + conversation_events 旧行。删除不可逆——依赖阶段0 raw-dump 留存窗。

## 阶段 8 — 常态化
- 内联卸载（阶段3）已让新数据不进 DB；裁旧（clean）按窗定期跑（确认留存窗后接 cron）。
- ✅ clean 已会同步删 content_location 指向的卸载 blob（DELETE...RETURNING + storage.Remove，无孤儿）。

## 回滚
- **镜像**：回滚旧 tag。加列/event_start_ts 索引无害；slim-indexes 删的索引可 CREATE 重建。
- **内联卸载/backfill**：正文在磁盘 blob 未丢；退化多因共享存储没配对，修阶段3后重跑 efficiency-v2 恢复；真要回退用 `import -f` 从 raw-dump 重建 DB 正文。
- **裁旧**：不可逆，只能 import -f 重建（依赖留存窗）。

## 命令清单（都经 `kb` = `docker compose exec kbcli /app/bin/kbcli --config /app/config.yaml`）
> ⚠️ kbcli 二进制在 `/app/bin/kbcli`、镜像 CMD 是 `serve` 常驻服务。**不能** `docker compose run --rm kbcli db-diag`（会把 `db-diag` 当二进制 exec→`executable file not found`），必须 `kb db-diag`。
| 命令 | 作用 | 破坏性 |
|---|---|---|
| `db-diag [--full]` | 只读诊断 + 0退化复核 | 无 |
| `slim-indexes [--dry-run] [--concurrently]` | 删 10 个无用 conv_events 索引 | 删索引(可重建) |
| `offload-content [--dry-run]` | 一次性 backfill 存量正文卸载 | 改 DB(blob 兜底) |
| `efficiency-v2` | 重算(走 A6 回读) | 重写派生表 |
| `clean --before <date> [--dry-run]` | 裁旧删旧行 | 不可逆删除 |
