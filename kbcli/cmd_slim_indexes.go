package main

import (
	"fmt"

	"kanban/core/models"
	"kanban/kbcli/internal/appconfig"
	"kanban/kbcli/internal/logx"

	"github.com/spf13/cobra"
)

// conv_events 二级索引瘦身：删除 10 个经代码实证零查询命中的索引（task_start/source_quality 复合 + 8 单列）。
// 保留 pkey/ux_logical/idx_session_start/idx_event_start_ts。删的最坏只是某查询变慢、可秒级 CREATE 重建（非正确性风险）。
// 拆成手动命令而非 AutoMigrate 自动删：让运维能先用 db-diag 看 idx_scan 复核、再确认删除。
var slimIndexDropList = []string{
	"idx_conversation_events_session_id",
	"idx_conversation_events_request_id",
	"idx_conversation_events_task_id",
	"idx_conversation_events_user_id",
	"idx_conversation_events_work_dir_id",
	"idx_conversation_events_event_kind",
	"idx_conversation_events_source",
	"idx_conversation_events_parse_quality",
	"idx_conversation_events_task_start",
	"idx_conversation_events_source_quality",
}

var slimIndexesCmd = &cobra.Command{
	Use:   "slim-indexes",
	Short: "删除 conversation_events 上 10 个零查询命中的二级索引（手动、可 dry-run）",
	Long:  "删 conv_events 的 8 个单列 + task_start/source_quality 复合索引（代码实证无查询使用）。建议先 `db-diag` 看 idx_scan 复核。--dry-run 只列不删；--concurrently 用 CONCURRENTLY 避锁（不能在事务内，本命令逐条独立执行）。",
	RunE: func(cmd *cobra.Command, args []string) error {
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		concurrently, _ := cmd.Flags().GetBool("concurrently")
		gdb, err := models.OpenGormDB(appconfig.Cfg.StatDatabase.DSN())
		if err != nil {
			return fmt.Errorf("连接数据库失败: %w", err)
		}
		sqlDB, _ := gdb.DB()
		defer sqlDB.Close()

		kw := ""
		if concurrently {
			kw = "CONCURRENTLY "
		}
		var dropped int
		for _, name := range slimIndexDropList {
			if dryRun {
				logx.Infof("[slim-indexes] 将删除 %s（dry-run）", name)
				continue
			}
			// 逐条独立 Exec（autocommit）：DROP INDEX CONCURRENTLY 不能在事务块内，*sql.DB.Exec 不开显式事务，OK。
			if _, err := sqlDB.Exec(fmt.Sprintf("DROP INDEX %sIF EXISTS %s", kw, name)); err != nil {
				logx.Errorf("[slim-indexes] 删除 %s 失败: %v", name, err)
				continue
			}
			dropped++
			logx.Infof("[slim-indexes] 已删除 %s", name)
		}
		if dryRun {
			logx.Infof("[slim-indexes] dry-run：共 %d 个待删（未执行）。先 db-diag 确认这些 idx_scan=0 再实跑。", len(slimIndexDropList))
		} else {
			logx.Infof("[slim-indexes] 完成：删除 %d/%d。空间回收即时（DROP INDEX 直接释放）。", dropped, len(slimIndexDropList))
		}
		return nil
	},
}

func init() {
	slimIndexesCmd.Flags().Bool("dry-run", false, "只列出将删除的索引，不执行")
	slimIndexesCmd.Flags().Bool("concurrently", false, "用 DROP INDEX CONCURRENTLY 避免锁表（生产推荐）")
	rootCmd.AddCommand(slimIndexesCmd)
}
