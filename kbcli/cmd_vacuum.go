package main

import (
	"fmt"
	"regexp"

	"kanban/core/models"
	"kanban/kbcli/internal/appconfig"
	"kanban/kbcli/internal/logx"

	"github.com/spf13/cobra"
)

// vacuum：对大表跑 VACUUM [FULL] ANALYZE。供「只能 compose、碰不到 psql」的环境在卸载/裁旧后回收物理空间。
// 默认表 conversations + conversation_events。--full 才真缩盘（锁表，低峰跑）；不带 --full 只整理不缩盘（不锁）。
var vacuumTableRe = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

var vacuumCmd = &cobra.Command{
	Use:   "vacuum [table...]",
	Short: "对大表跑 VACUUM [FULL] ANALYZE（compose 环境回收卸载/裁旧后的空间）",
	Long:  "回收 UPDATE 置空列/裁旧删行产生的死元组空间。默认 conversations + conversation_events。--full 缩盘但锁表（低峰跑）。",
	RunE: func(cmd *cobra.Command, args []string) error {
		full, _ := cmd.Flags().GetBool("full")
		tables := args
		if len(tables) == 0 {
			tables = []string{"conversations", "conversation_events"}
		}
		for _, t := range tables {
			if !vacuumTableRe.MatchString(t) { // 防注入：表名只允许标识符字符
				return fmt.Errorf("非法表名: %q", t)
			}
		}
		gdb, err := models.OpenGormDB(appconfig.Cfg.StatDatabase.DSN())
		if err != nil {
			return fmt.Errorf("连接数据库失败: %w", err)
		}
		sqlDB, _ := gdb.DB()
		defer sqlDB.Close()

		kw := "VACUUM (ANALYZE)"
		if full {
			kw = "VACUUM (FULL, ANALYZE)"
		}
		for _, t := range tables {
			logx.Infof("[vacuum] %s %s ...（%s）", kw, t, map[bool]string{true: "FULL 锁表，请耐心", false: "在线整理"}[full])
			// VACUUM 不能在事务块内；*sql.DB.Exec 不开显式事务，OK。
			if _, err := sqlDB.Exec(fmt.Sprintf("%s %s", kw, t)); err != nil {
				logx.Errorf("[vacuum] %s 失败: %v", t, err)
				continue
			}
			logx.Infof("[vacuum] %s 完成", t)
		}
		logx.Info("[vacuum] 全部完成。用 db-diag 的 A 节核对库/表大小。")
		return nil
	},
}

func init() {
	vacuumCmd.Flags().Bool("full", false, "VACUUM FULL（真缩盘，锁表；不带则只在线整理不缩盘）")
	rootCmd.AddCommand(vacuumCmd)
}
