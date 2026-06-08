package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"kanban/kbcli/internal/appconfig"
	"kanban/kbcli/internal/logx"
	"os"
	"strconv"
	"strings"
	"time"

	"kanban/core/models"

	"github.com/spf13/cobra"
	"gorm.io/gorm/clause"
)

// import-anchor 从可配置的 CSV 母表灌入 kNN 锚点(anchor_set)。
// 取代手动 psql `\copy`：路径由 --csv 或 efficiency_v2.anchor_set_csv 决定，
// 列映射/校验/幂等 upsert 与 kbcli/scripts/import_efficiency_v2_anchor_set.sql 保持一致。
var importAnchorCmd = &cobra.Command{
	Use:   "import-anchor",
	Short: "从 CSV 母表灌入 kNN 锚点(anchor_set)；路径取 --csv 或 efficiency_v2.anchor_set_csv",
	RunE: func(cmd *cobra.Command, args []string) error {
		csvPath, _ := cmd.Flags().GetString("csv")
		if strings.TrimSpace(csvPath) == "" {
			csvPath = appconfig.Cfg.EfficiencyV2.AnchorSetCSV
		}
		if strings.TrimSpace(csvPath) == "" {
			return fmt.Errorf("未指定锚点 CSV 路径（--csv 或 efficiency_v2.anchor_set_csv）")
		}
		return runImportAnchor(csvPath)
	},
}

func init() {
	importAnchorCmd.Flags().String("csv", "", "锚点母表 CSV 路径（默认取 efficiency_v2.anchor_set_csv）")
	rootCmd.AddCommand(importAnchorCmd)
}

func runImportAnchor(csvPath string) error {
	f, err := os.Open(csvPath)
	if err != nil {
		return fmt.Errorf("打开锚点 CSV 失败 %s: %w", csvPath, err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1 // 容忍尾随空列
	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("读取 CSV 表头失败: %w", err)
	}
	idx := make(map[string]int, len(header))
	for i, h := range header {
		idx[strings.TrimSpace(h)] = i
	}
	for _, col := range []string{"anchor_id", "source", "without_ai_minutes", "feature_vector"} {
		if _, ok := idx[col]; !ok {
			return fmt.Errorf("CSV 缺少必需列: %s", col)
		}
	}
	get := func(rec []string, name string) string {
		if i, ok := idx[name]; ok && i < len(rec) {
			return strings.TrimSpace(rec[i])
		}
		return ""
	}

	rows := make([]models.AnchorSet, 0, 256)
	skipped := 0
	for {
		rec, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("读取 CSV 行失败: %w", err)
		}

		anchorID := get(rec, "anchor_id")
		source := get(rec, "source")
		fv := get(rec, "feature_vector")
		withoutAIStr := get(rec, "without_ai_minutes")
		// 校验口径与 import SQL 的 WHERE 完全一致。
		if anchorID == "" || source == "" || withoutAIStr == "" || fv == "" || fv == "{}" {
			skipped++
			continue
		}
		withoutAI, err := strconv.ParseFloat(withoutAIStr, 64)
		if err != nil {
			skipped++
			continue
		}

		row := models.AnchorSet{
			AnchorId:         anchorID,
			Source:           source,
			SourceVersion:    get(rec, "source_version"),
			AnchorKind:       get(rec, "anchor_kind"),
			WithoutAIMinutes: &withoutAI,
			HumanLabeled:     anchorParseBool(get(rec, "human_labeled"), false),
			Weight:           anchorParseWeight(get(rec, "weight")),
			FeatureVector:    models.ObjectJSON(fv),
			Labels:           anchorObjectJSON(get(rec, "labels")),
			ValidFrom:        anchorParseTime(get(rec, "valid_from")),
			ValidTo:          anchorParseTime(get(rec, "valid_to")),
		}
		if v := get(rec, "human_labeled_minutes"); v != "" {
			if hm, err := strconv.ParseFloat(v, 64); err == nil {
				row.HumanLabeledMinutes = &hm
			}
		}
		rows = append(rows, row)
	}

	if len(rows) == 0 {
		return fmt.Errorf("CSV 无有效锚点行（路径 %s，跳过 %d 行）", csvPath, skipped)
	}

	db, err := models.OpenGormDB(appconfig.Cfg.StatDatabase.DSN())
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "anchor_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"source", "source_version", "anchor_kind",
			"human_labeled_minutes", "without_ai_minutes", "human_labeled",
			"weight", "feature_vector", "labels", "valid_from", "valid_to", "updated_at",
		}),
	}).CreateInBatches(&rows, 500).Error; err != nil {
		return fmt.Errorf("写入 anchor_set 失败: %w", err)
	}

	logx.Infof("import-anchor: 从 %s 灌入/更新 %d 个锚点（跳过 %d 行无效）", csvPath, len(rows), skipped)
	return nil
}

func anchorParseBool(s string, def bool) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "t", "true", "1", "yes", "y":
		return true
	case "f", "false", "0", "no", "n":
		return false
	default:
		return def
	}
}

// anchorParseWeight 复刻 SQL 的 COALESCE(NULLIF(weight,0),1)：空或 0 → 1。
func anchorParseWeight(s string) float64 {
	if s == "" {
		return 1
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v == 0 {
		return 1
	}
	return v
}

func anchorObjectJSON(s string) models.ObjectJSON {
	if strings.TrimSpace(s) == "" {
		return models.ObjectJSON("{}")
	}
	return models.ObjectJSON(s)
}

func anchorParseTime(s string) *time.Time {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05-07", "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return &t
		}
	}
	return nil
}
