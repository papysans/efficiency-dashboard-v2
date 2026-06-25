-- =====================================================================
-- 14GB 瘦身：生产诊断包（库 costrict_stat / PG15，全只读）
-- 用法：psql 内先 \timing on，逐节复制跑；据此把"预计省多少"从估算变实测。
-- 护栏（对抗审查要求）：
--   * 先设 statement_timeout，避免重扫拖垮在线服务；
--   * 标 [HEAVY] 的节是全表扫 + TOAST 解压，建议低峰 / 只读副本跑；
--   * pg_column_size 返回的是 TOAST 压缩后磁盘字节（非原文长度），反映当前盘占。
-- =====================================================================
SET statement_timeout = '120s';

-- ========== A. 整库 + 量表（对账 14GB，定位两表占 98%）==========
SELECT pg_size_pretty(pg_database_size(current_database())) AS db_total;

SELECT c.relname AS table_name,
  pg_size_pretty(pg_total_relation_size(c.oid))                       AS total,
  pg_size_pretty(pg_relation_size(c.oid))                             AS heap_main,
  pg_size_pretty(COALESCE(pg_total_relation_size(c.reltoastrelid),0)) AS toast_total,
  pg_size_pretty(pg_indexes_size(c.oid))                              AS indexes_total,
  c.reltuples::bigint                                                 AS approx_rows
FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
WHERE c.relkind='r' AND n.nspname='public'
ORDER BY pg_total_relation_size(c.oid) DESC;

-- 残留表核查（V1 user_productivity / 一次性表 / clean bug 的错表名）
SELECT to_regclass('public.user_productivity')        AS user_productivity_存在吗,
       to_regclass('public.task_manual_ground_truth') AS task_manual_gt_存在吗,
       to_regclass('public.task_conversations')       AS clean_bug_错表名_应为NULL,
       to_regclass('public.conversations')            AS 真实表名_应非NULL;

-- ========== B. 索引猎杀（conversation_events 实测 14 索引）==========
-- B1 ★5 张表所有索引 idx_scan(0=从未被选中) + 体积
SELECT s.relname AS table_name, s.indexrelname AS index_name, s.idx_scan,
  pg_size_pretty(pg_relation_size(s.indexrelid)) AS index_size,
  ix.indisunique AS is_unique, ix.indisprimary AS is_primary
FROM pg_stat_user_indexes s JOIN pg_index ix ON ix.indexrelid = s.indexrelid
WHERE s.relname IN ('conversation_events','conversations','commits','needs','session_stage_metrics')
ORDER BY s.relname, s.idx_scan ASC, pg_relation_size(s.indexrelid) DESC;

-- B2b 每表索引总占用汇总（回答"索引总占多少"）
SELECT t.relname AS table_name, count(*) AS index_count,
  pg_size_pretty(sum(pg_relation_size(i.oid))) AS indexes_total
FROM pg_index idx JOIN pg_class i ON i.oid=idx.indexrelid
JOIN pg_class t ON t.oid=idx.indrelid JOIN pg_namespace n ON n.oid=t.relnamespace
WHERE n.nspname='public' AND t.relkind='r'
GROUP BY t.relname ORDER BY sum(pg_relation_size(i.oid)) DESC;

-- B3 全库"从未使用且非主键非唯一"可删索引（drop 候选）
SELECT s.relname AS table_name, s.indexrelname AS index_name,
  pg_size_pretty(pg_relation_size(s.indexrelid)) AS size, s.idx_scan
FROM pg_stat_user_indexes s JOIN pg_index ix ON ix.indexrelid=s.indexrelid
WHERE s.idx_scan=0 AND NOT ix.indisprimary AND NOT ix.indisunique
ORDER BY pg_relation_size(s.indexrelid) DESC;

-- B4 ★pg_stat 有效性闸：stats_reset 太近 / 刚重启则 idx_scan=0 不可信
SELECT stats_reset, now()-stats_reset AS observed_span
FROM pg_stat_database WHERE datname=current_database();

-- ========== C. 膨胀体检（死元组=UPDATE置空/批量DELETE的产物）==========
SELECT relname AS table_name, n_live_tup, n_dead_tup,
  CASE WHEN n_live_tup+n_dead_tup>0 THEN round(100.0*n_dead_tup/(n_live_tup+n_dead_tup),1) END AS dead_pct,
  last_vacuum, last_autovacuum
FROM pg_stat_user_tables ORDER BY n_dead_tup DESC;

-- ========== D. 列级体积（卸列/瘦列省多少做实；[HEAVY] 抽样1%×100）==========
-- D1 conversations 列级 [HEAVY-轻] 抽样
SELECT count(*) AS sampled_rows,
  pg_size_pretty(sum(pg_column_size(request_content))::bigint*100)  AS request_content_est,
  pg_size_pretty(sum(pg_column_size(user_input))::bigint*100)       AS user_input_est,
  pg_size_pretty(sum(pg_column_size(response_content))::bigint*100) AS response_content_est
FROM conversations TABLESAMPLE SYSTEM (1);

-- D2 conversation_events 列级 [HEAVY-轻] 抽样：6.2GB 拆 command_text vs payload vs id/地址群
SELECT count(*) AS sampled_rows,
  pg_size_pretty(sum(pg_column_size(command_text))::bigint*100)  AS command_text_est,
  pg_size_pretty(sum(pg_column_size(payload))::bigint*100)       AS payload_est,
  pg_size_pretty(sum(pg_column_size(touched_files))::bigint*100) AS touched_files_est,
  pg_size_pretty(sum(pg_column_size(event_id)+pg_column_size(session_id)+pg_column_size(request_id)
    +pg_column_size(task_id)+pg_column_size(user_id)+pg_column_size(work_dir_id)
    +pg_column_size(repo_addr))::bigint*100) AS id_addr_cols_est
FROM conversation_events TABLESAMPLE SYSTEM (1);

-- ========== E. 压缩杠杆（lz4 是否可用 + 当前压缩态）==========
DO $$ BEGIN
  CREATE TEMP TABLE _lz4_probe(c text COMPRESSION lz4);
  RAISE NOTICE 'lz4 supported';
  DROP TABLE _lz4_probe;
EXCEPTION WHEN others THEN RAISE NOTICE 'lz4 NOT supported: %', SQLERRM; END $$;
SHOW default_toast_compression;

-- ========== F. 裁旧 vs 卸列 决策数据 ==========
-- F1a ★[轻-走索引] conversations 按 floor(20260525) 切：行占比（决定 TRUNCATE 是否丢历史）
SELECT count(*) AS total_rows,
  count(*) FILTER (WHERE start_time <  DATE '2026-05-25') AS rows_before_floor_删候选,
  count(*) FILTER (WHERE start_time >= DATE '2026-05-25') AS rows_in_window_保留
FROM conversations;

-- F1b [HEAVY] floor 前后的重列体积（确认裁旧能省多少；慢，低峰跑）
SELECT
  pg_size_pretty(sum(pg_column_size(request_content)+pg_column_size(response_content)+pg_column_size(user_input))
    FILTER (WHERE start_time <  DATE '2026-05-25')) AS heavy_before_floor,
  pg_size_pretty(sum(pg_column_size(request_content)+pg_column_size(response_content)+pg_column_size(user_input))
    FILTER (WHERE start_time >= DATE '2026-05-25')) AS heavy_in_window
FROM conversations;

-- F2 [轻] 按月行量（保留 N 月能 DROP 多少行）
SELECT date_trunc('month', start_time) AS mon, count(*) AS rows FROM conversations GROUP BY 1 ORDER BY 1;
SELECT date_trunc('month', event_start_ts) AS mon, count(*) AS rows FROM conversation_events GROUP BY 1 ORDER BY 1;

-- F3 [轻] conversations 幂等键能否纳入 start_time 分区（>0=有抖动会破坏去重）
SELECT count(*) AS pairs_with_multiple_start_times
FROM (SELECT session_id, request_id FROM conversations
      GROUP BY session_id, request_id HAVING count(DISTINCT start_time) > 1) x;

-- ========== G. TRUNCATE 前安全核查（FK/触发器）==========
-- conversation_events 是否被任何外键引用（有则 TRUNCATE 需谨慎）
SELECT conrelid::regclass AS from_table, conname, confrelid::regclass AS to_table
FROM pg_constraint WHERE contype='f'
  AND (confrelid='conversation_events'::regclass OR conrelid='conversation_events'::regclass);
