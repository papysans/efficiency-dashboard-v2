package main

import (
	"fmt"

	"kanban/core/models"
	"kanban/kbcli/internal/appconfig"
	"kanban/kbcli/internal/logx"

	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

// offload-content：把 conversations 表中仍有正文且未卸载的行，三列正文落盘/对象存储 + 写 content_location 指针
// + 置空 DB 列（同步持久化 user_input_chars）。这是 WS-A 卸载执行器，也是存量 backfill 通道。
//
// 安全约定（见 .trellis/tasks/06-24-db-conversations-disk/design.md 激活硬门槛）：
//   - 手动子命令即开关，不接 cron；生产 cutover 须先确认 raw-dump 留存窗/回看窗口。
//   - A6 回读已就位（QueryEfficiencyV2Conversations + backend ListConversations 均 HydrateContent），
//     故卸载后 efficiency-v2/前端读到的仍是完整正文，不会复现解析退化。
//   - 幂等：以 content_location='' 为待卸载判据，已卸载行天然跳过；可随时重跑。
//   - 固定顺序：先 Offload 落对象成功，再单条 UPDATE 写指针+置空列；对象失败则跳过该行不动 DB。
var offloadContentCmd = &cobra.Command{
	Use:   "offload-content",
	Short: "把 conversations 正文卸载到磁盘/对象存储（DB 留 content_location 指针）",
	Long:  "把 conversations 表中仍有正文且未卸载(content_location 为空)的行，三列正文落盘 + 写指针 + 置空 DB 列。幂等、手动调用，不接 cron。",
	RunE: func(cmd *cobra.Command, args []string) error {
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		batchSize, _ := cmd.Flags().GetInt("batch-size")
		analysedDir, _ := cmd.Flags().GetString("analysed-dir")
		if analysedDir == "" {
			analysedDir = appconfig.Cfg.AnalysedDir
		}
		if analysedDir == "" {
			return fmt.Errorf("必须指定 --analysed-dir 或在配置中设置 analysed_dir")
		}
		if batchSize <= 0 {
			batchSize = 500
		}
		db, err := models.OpenGormDB(appconfig.Cfg.StatDatabase.DSN())
		if err != nil {
			return fmt.Errorf("连接数据库失败: %w", err)
		}
		sqlDB, _ := db.DB()
		defer sqlDB.Close()
		return runOffloadContent(db, analysedDir, batchSize, dryRun)
	},
}

// offloadPendingWhere 待卸载判据：未卸载(指针空)且三列里还有正文。
// COALESCE：content_location 是 AutoMigrate 新增列，存量行是 NULL 而非 ''——
// 必须把 NULL 也视为待卸载，否则 backfill 静默跳过所有存量行（Codex review P1）。
const offloadPendingWhere = "COALESCE(content_location, '') = '' AND (COALESCE(request_content,'') <> '' OR COALESCE(response_content,'') <> '' OR COALESCE(user_input,'') <> '')"

func runOffloadContent(db *gorm.DB, analysedDir string, batchSize int, dryRun bool) error {
	if dryRun {
		var cnt int64
		if err := db.Model(&models.Conversation{}).Where(offloadPendingWhere).Count(&cnt).Error; err != nil {
			return fmt.Errorf("统计待卸载行失败: %w", err)
		}
		logx.Infof("[offload] 待卸载 %d 行（dry-run，未执行）", cnt)
		return nil
	}

	// keyset 分页（id 游标）：按主键 id 单调推进，每行只处理一次（成功或失败都跨过，失败行本次不重试），
	// 天然去重、确定性顺序、必然终止，且失败计数准确（无跨批重复计数）。
	var total, failed int64
	var lastID int
	for {
		var rows []models.Conversation
		if err := db.Where(offloadPendingWhere).Where("id > ?", lastID).
			Order("id").Limit(batchSize).Find(&rows).Error; err != nil {
			return fmt.Errorf("查询待卸载行失败: %w", err)
		}
		if len(rows) == 0 {
			break
		}
		for i := range rows {
			r := &rows[i]
			lastID = r.ID // 游标推进：无论成败，本行处理后不再被捞出
			// 固定顺序①：先落对象（失败则该行保持未卸载、不动 DB）
			if err := r.Offload(analysedDir); err != nil {
				logx.Errorf("[offload] 落对象失败(session=%s request=%s)，本次跳过: %v", r.SessionId, r.RequestId, err)
				failed++
				continue
			}
			// 固定顺序②：对象已成功 → 单条 UPDATE 写指针 + 置空三列 + 持久化 chars
			if err := db.Model(&models.Conversation{}).
				Where("session_id = ? AND request_id = ?", r.SessionId, r.RequestId).
				Updates(map[string]interface{}{
					"content_location": r.ContentLocation,
					"request_content":  "",
					"response_content": "",
					"user_input":       "",
					"user_input_chars": r.UserInputChars,
				}).Error; err != nil {
				// 对象已落但指针未写：重跑会以同 key 覆盖重写并补 UPDATE，幂等无累积孤儿。
				logx.Errorf("[offload] 写指针/置空失败(session=%s request=%s)，本次跳过: %v", r.SessionId, r.RequestId, err)
				failed++
				continue
			}
			total++
		}
		logx.Infof("[offload] 进度：累计成功 %d 行（失败累计 %d）", total, failed)
	}
	logx.Infof("[offload] 完成：成功 %d 行，失败 %d 行（失败行保持未卸载，可修因后重跑）", total, failed)
	return nil
}

func init() {
	offloadContentCmd.Flags().Bool("dry-run", false, "仅统计待卸载行数，不执行卸载")
	offloadContentCmd.Flags().Int("batch-size", 500, "每批处理行数")
	offloadContentCmd.Flags().String("analysed-dir", "", "卸载对象根目录(默认用配置 analysed_dir)")
	rootCmd.AddCommand(offloadContentCmd)
}
