package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var catAnalysisCmd = &cobra.Command{
	Use:   "cat-analysis",
	Short: "查看分析过程文件内容",
	RunE: func(cmd *cobra.Command, args []string) error {
		dimension, _ := cmd.Flags().GetString("dimension")
		id, _ := cmd.Flags().GetString("id")
		date, _ := cmd.Flags().GetString("date")

		if dimension != "project" && dimension != "repo" {
			return fmt.Errorf("--dimension 必须是 project 或 repo，当前值: %s", dimension)
		}

		if len(date) != 8 {
			return fmt.Errorf("--date 格式必须是 YYYYMMDD，当前值: %s", date)
		}

		yearMonth := date[:4] + "-" + date[4:6]
		safeID := makeSafeID(id)
		filePath := fmt.Sprintf("%s/%s/analysis/%s_%s_%s.json", cfg.RawDataDir, yearMonth, dimension, safeID, date)

		data, err := os.ReadFile(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("文件不存在: %s", filePath)
			}
			return fmt.Errorf("读取文件失败: %w", err)
		}

		var parsed interface{}
		if err := json.Unmarshal(data, &parsed); err != nil {
			fmt.Print(string(data))
			return nil
		}

		formatted, err := json.MarshalIndent(parsed, "", "  ")
		if err != nil {
			fmt.Print(string(data))
			return nil
		}
		fmt.Println(string(formatted))
		return nil
	},
}

func init() {
	catAnalysisCmd.Flags().String("dimension", "", "维度: project 或 repo（必填）")
	catAnalysisCmd.Flags().String("id", "", "维度 ID（必填）")
	catAnalysisCmd.Flags().String("date", "", "日期 YYYYMMDD（必填）")
	catAnalysisCmd.MarkFlagRequired("dimension")
	catAnalysisCmd.MarkFlagRequired("id")
	catAnalysisCmd.MarkFlagRequired("date")
	rootCmd.AddCommand(catAnalysisCmd)
}
