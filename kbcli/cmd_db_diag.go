package main

import (
	"database/sql"
	"fmt"
	"strings"

	"kanban/core/models"
	"kanban/kbcli/internal/appconfig"

	"github.com/spf13/cobra"
)

// db-diag：只读数据库瘦身诊断。把 diagnostic-bundle.sql 的关键查询做成 kbcli 子命令，
// 供只能跑 compose 命令、碰不到 psql 的环境用：docker compose run --rm kbcli db-diag。
// 全部只读，不改任何数据。重列用 TABLESAMPLE 1%（×100 估算）避免在 14GB 库上全表扫。
var dbDiagCmd = &cobra.Command{
	Use:   "db-diag",
	Short: "数据库瘦身诊断（只读）：量表/列/索引/膨胀/裁旧切分",
	Long:  "只读输出 DB 减负决策所需数据：每表大小、conversations 三列体量、conv_events 各索引大小+idx_scan、死元组、floor 裁旧切分。把结果贴回即可。",
	RunE: func(cmd *cobra.Command, args []string) error {
		full, _ := cmd.Flags().GetBool("full")
		// 只读连接：不跑 AutoMigrate（db-diag 号称只读，绝不在诊断时改 schema/加锁）。
		// 依赖 schema 已由常驻 server/kbcli 迁过；引用新列的小节若列不存在会单节报 ERR、不影响其余。
		gdb, err := models.OpenGormDBReadOnly(appconfig.Cfg.StatDatabase.DSN())
		if err != nil {
			return fmt.Errorf("连接数据库失败: %w", err)
		}
		sqlDB, err := gdb.DB()
		if err != nil {
			return err
		}
		defer sqlDB.Close()
		_, _ = sqlDB.Exec("SET statement_timeout = '180s'")

		sample := "TABLESAMPLE SYSTEM (1)"
		mult := "*100"
		note := "（抽样 1% ×100 估算；--full 改全表精确但慢）"
		if full {
			sample, mult, note = "", "", "（全表精确）"
		}

		runAndPrint(sqlDB, "A. 整库 + 每表大小（对账 14GB / 定位两表占比）", `
SELECT c.relname AS table_name,
  pg_size_pretty(pg_total_relation_size(c.oid))                       AS total,
  pg_size_pretty(pg_relation_size(c.oid))                             AS heap_main,
  pg_size_pretty(COALESCE(pg_total_relation_size(c.reltoastrelid),0)) AS toast_total,
  pg_size_pretty(pg_indexes_size(c.oid))                              AS indexes_total,
  c.reltuples::bigint                                                 AS approx_rows
FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
WHERE c.relkind='r' AND n.nspname='public'
ORDER BY pg_total_relation_size(c.oid) DESC LIMIT 15`)
		runAndPrint(sqlDB, "A0. 整库大小", `SELECT pg_size_pretty(pg_database_size(current_database())) AS db_total`)

		runAndPrint(sqlDB, "B. conv_events 各索引体积 + idx_scan（0=从未被用，验证可删）", `
SELECT s.indexrelname AS index_name, s.idx_scan,
  pg_size_pretty(pg_relation_size(s.indexrelid)) AS size,
  ix.indisunique AS is_unique, ix.indisprimary AS is_primary
FROM pg_stat_user_indexes s JOIN pg_index ix ON ix.indexrelid=s.indexrelid
WHERE s.relname='conversation_events'
ORDER BY pg_relation_size(s.indexrelid) DESC`)
		runAndPrint(sqlDB, "B2. idx_scan 有效性闸（stats_reset 太近则 idx_scan 不可信）", `
SELECT stats_reset, now()-stats_reset AS observed_span FROM pg_stat_database WHERE datname=current_database()`)

		runAndPrint(sqlDB, "C. 膨胀体检（死元组）", `
SELECT relname AS table_name, n_live_tup, n_dead_tup,
  CASE WHEN n_live_tup+n_dead_tup>0 THEN round(100.0*n_dead_tup/(n_live_tup+n_dead_tup),1) END AS dead_pct,
  last_vacuum, last_autovacuum
FROM pg_stat_user_tables ORDER BY n_dead_tup DESC LIMIT 12`)

		runAndPrint(sqlDB, "D. conversations 三列体量 "+note+"（卸载能省多少）", `
SELECT count(*) AS sampled_rows,
  pg_size_pretty((sum(pg_column_size(request_content))`+mult+`)::bigint)  AS request_content,
  pg_size_pretty((sum(pg_column_size(user_input))`+mult+`)::bigint)       AS user_input,
  pg_size_pretty((sum(pg_column_size(response_content))`+mult+`)::bigint) AS response_content
FROM conversations `+sample)
		runAndPrint(sqlDB, "D2. conv_events 大列体量 "+note, `
SELECT count(*) AS sampled_rows,
  pg_size_pretty((sum(pg_column_size(command_text))`+mult+`)::bigint) AS command_text,
  pg_size_pretty((sum(pg_column_size(payload))`+mult+`)::bigint)      AS payload
FROM conversation_events `+sample)

		runAndPrint(sqlDB, "F. conversations 按 floor(20260525) 切（裁旧能删多少行；走 start_time 索引，轻）", `
SELECT count(*) AS total_rows,
  count(*) FILTER (WHERE start_time <  DATE '2026-05-25') AS rows_before_floor,
  count(*) FILTER (WHERE start_time >= DATE '2026-05-25') AS rows_in_window
FROM conversations`)
		runAndPrint(sqlDB, "F2. conversations 卸载状态（content_location 已写几行）", `
SELECT count(*) FILTER (WHERE content_location IS NOT NULL AND content_location<>'') AS offloaded,
  count(*) AS total FROM conversations`)

		runAndPrint(sqlDB, "G. conv_events parse_quality 分布（★0退化基线/复核：卸载前后必须一致）", `
SELECT parse_quality, count(*) FROM conversation_events GROUP BY 1 ORDER BY 1`)
		runAndPrint(sqlDB, "G2. 主指标行数（卸载/重算前后应一致）", `
SELECT
  (SELECT count(*) FROM conversation_events)    AS conv_events,
  (SELECT count(*) FROM session_stage_metrics)  AS stage_metrics,
  (SELECT count(*) FROM needs)                  AS needs,
  (SELECT count(*) FROM user_productivity_v2)   AS user_prod_v2`)
		runAndPrint(sqlDB, "H. schema 状态（部署/迁移自检）", `
SELECT
  (SELECT count(*) FROM information_schema.columns
     WHERE table_name='conversations' AND column_name IN ('content_location','user_input_chars')) AS new_cols_should_be_2,
  (SELECT count(*) FROM pg_indexes WHERE tablename='conversation_events') AS conv_events_indexes_14before_4after`)

		fmt.Println("\n[db-diag] 完成。把以上输出贴回即可。卸载前后各跑一次对比 G/G2 节即验 0 退化。")
		return nil
	},
}

// runAndPrint 执行一条只读查询并以 " | " 分隔打印（任意列形状通用）。
func runAndPrint(sqlDB *sql.DB, title, query string) {
	fmt.Printf("\n===== %s =====\n", title)
	rows, err := sqlDB.Query(query)
	if err != nil {
		fmt.Printf("  ERR: %v\n", err)
		return
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	fmt.Println("  " + strings.Join(cols, " | "))
	for rows.Next() {
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			fmt.Printf("  scan err: %v\n", err)
			continue
		}
		strs := make([]string, len(cols))
		for i, v := range vals {
			switch t := v.(type) {
			case []byte:
				strs[i] = string(t)
			case nil:
				strs[i] = "NULL"
			default:
				strs[i] = fmt.Sprintf("%v", t)
			}
		}
		fmt.Println("  " + strings.Join(strs, " | "))
	}
}

func init() {
	dbDiagCmd.Flags().Bool("full", false, "列体量用全表精确扫（默认抽样 1% 估算，更快）")
	rootCmd.AddCommand(dbDiagCmd)
}
