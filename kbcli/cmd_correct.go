package main

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

var correctCmd = &cobra.Command{
	Use:   "correct",
	Short: "纠错命令，修正分析结果中的字段值",
	RunE: func(cmd *cobra.Command, args []string) error {
		dimension, _ := cmd.Flags().GetString("dimension")
		id, _ := cmd.Flags().GetString("id")
		field, _ := cmd.Flags().GetString("field")
		valueStr, _ := cmd.Flags().GetString("value")
		reason, _ := cmd.Flags().GetString("reason")
		by, _ := cmd.Flags().GetString("by")

		if dimension != "project" && dimension != "repo" {
			return fmt.Errorf("--dimension 必须是 project 或 repo，当前值: %s", dimension)
		}

		value, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			return fmt.Errorf("--value 必须是数值，当前值: %s", valueStr)
		}

		bc := NewBackendClient(cfg.BackendURL)
		fmt.Printf("[Correct] 纠错 %s=%s, 字段=%s, 值=%.2f\n", dimension, id, field, value)

		if err := bc.CorrectEfficiency(dimension, id, field, value, reason, by); err != nil {
			return fmt.Errorf("纠错失败: %w", err)
		}

		fmt.Println("[Correct] 纠错成功")
		return nil
	},
}

func init() {
	correctCmd.Flags().String("dimension", "", "维度: project 或 repo（必填）")
	correctCmd.Flags().String("id", "", "维度 ID（必填）")
	correctCmd.Flags().String("field", "", "要纠错的字段名（必填）")
	correctCmd.Flags().String("value", "", "纠错后的值（必填，数值）")
	correctCmd.Flags().String("reason", "", "纠错原因（必填）")
	correctCmd.Flags().String("by", "", "操作人（必填）")
	correctCmd.MarkFlagRequired("dimension")
	correctCmd.MarkFlagRequired("id")
	correctCmd.MarkFlagRequired("field")
	correctCmd.MarkFlagRequired("value")
	correctCmd.MarkFlagRequired("reason")
	correctCmd.MarkFlagRequired("by")
	rootCmd.AddCommand(correctCmd)
}
