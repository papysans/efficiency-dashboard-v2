package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var reindexCmd = &cobra.Command{
	Use:   "reindex",
	Short: "将指定日期的 rawdata 写入 Elasticsearch",
	RunE: func(cmd *cobra.Command, args []string) error {
		step, _ := cmd.Flags().GetString("step")
		force, _ := cmd.Flags().GetBool("force")
		dateStr, _ := cmd.Flags().GetString("date")
		startDate, _ := cmd.Flags().GetString("start-date")
		endDate, _ := cmd.Flags().GetString("end-date")

		// 确定日期列表
		var dates []string
		if startDate != "" && endDate != "" {
			var err error
			dates, err = generateDateRange(startDate, endDate)
			if err != nil {
				return fmt.Errorf("%w", err)
			}
		} else if dateStr != "" {
			if len(dateStr) != 8 {
				return fmt.Errorf("--date 格式不正确，应为 YYYYMMDD，如 20260331")
			}
			dates = []string{dateStr}
		} else {
			return fmt.Errorf("必须指定 --date 或 --start-date/--end-date 参数（格式: YYYYMMDD）")
		}

		// 初始化 OrgProvider
		orgProvider, err := NewOrgProvider(cfg.OrgCSVFile)
		if err != nil {
			fmt.Printf("警告: 加载组织信息失败: %v，将使用空组织信息\n", err)
			orgProvider = &OrgProvider{
				userIDMap:   make(map[string]OrgInfo),
				userNameMap: make(map[string]OrgInfo),
			}
		}

		// 初始化 ESClient
		esClient, err := NewESClient(cfg)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		backendClient := NewBackendClient(cfg.BackendURL)

		for _, d := range dates {
			if err := reindexDate(cfg, esClient, orgProvider, backendClient, d, step, force); err != nil {
				return err
			}
		}

		fmt.Printf("\n✅ reindex 完成！共处理 %d 个日期\n", len(dates))
		return nil
	},
}

func init() {
	reindexCmd.Flags().String("date", "", "指定日期（格式: YYYYMMDD）")
	reindexCmd.Flags().String("start-date", "", "开始日期（格式: YYYYMMDD）")
	reindexCmd.Flags().String("end-date", "", "结束日期（格式: YYYYMMDD）")
	reindexCmd.Flags().String("step", "", "执行步骤（request 或 task，默认全部）")
	reindexCmd.Flags().Bool("force", false, "强制重新创建索引")
	rootCmd.AddCommand(reindexCmd)
}

// reindexDate 处理单个日期的 reindex
func reindexDate(config *Config, esClient *ESClient, orgProvider *OrgProvider, backendClient *BackendClient, dateStr string, step string, force bool) error {
	fmt.Printf("\n处理日期: %s\n", dateStr)

	if step == "" || step == "request" {
		if err := reindexRequest(config, esClient, orgProvider, dateStr, force); err != nil {
			return err
		}
	}
	if step == "" || step == "task" {
		if err := reindexTask(config, esClient, orgProvider, backendClient, dateStr, force); err != nil {
			return err
		}
	}
	return nil
}

// reindexRequest 处理 request 步骤
func reindexRequest(config *Config, esClient *ESClient, orgProvider *OrgProvider, dateStr string, force bool) error {
	year := dateStr[:4]
	month := dateStr[4:6]
	day := dateStr[6:8]
	dateDir := fmt.Sprintf("%s-%s/%s", year, month, day)

	// 尝试两种目录结构: rawdata/request/YYYY-MM/DD (实际) 和 rawdata/YYYY-MM/DD (a.md定义)
	rawDataPath := filepath.Join(config.RawDataDir, "request", dateDir)
	if _, err := os.Stat(rawDataPath); os.IsNotExist(err) {
		rawDataPath = filepath.Join(config.RawDataDir, dateDir)
	}

	fmt.Printf("rawdata 路径: %s\n", rawDataPath)

	if _, err := os.Stat(rawDataPath); os.IsNotExist(err) {
		fmt.Printf("警告: 目录不存在: %s，跳过 request 步骤\n", rawDataPath)
		return nil
	}

	var rawDocs []RawDoc
	var skipped int
	var processed int

	err := filepath.WalkDir(rawDataPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(path), ".json") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Printf("警告: 读取文件失败 %s: %v\n", path, err)
			skipped++
			return nil
		}

		doc, err := ParseRawJSON(data, config.ModelPrices, orgProvider)
		if err != nil {
			fmt.Printf("警告: 解析文件失败 %s: %v\n", path, err)
			skipped++
			return nil
		}

		// 计算相对路径
		relPath, err := filepath.Rel(config.RawDataDir, path)
		if err != nil {
			fmt.Printf("警告: 计算相对路径失败 %s: %v\n", path, err)
			relPath = path
		}
		doc.SourcePath = filepath.ToSlash(relPath)

		rawDocs = append(rawDocs, *doc)
		processed++
		if processed%100 == 0 {
			fmt.Printf("  已处理 %d 个文件...\n", processed)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("遍历目录失败: %w", err)
	}

	fmt.Printf("文件扫描完成: 成功解析 %d 个，跳过 %d 个\n", processed, skipped)

	if len(rawDocs) == 0 {
		fmt.Println("没有可写入的 request 文档，跳过")
		return nil
	}

	requestIndexName := fmt.Sprintf("costrict_chat_request_%s", dateStr)

	if force {
		_ = esClient.DeleteIndex(requestIndexName)
	}

	// 确保索引存在
	if err := esClient.CreateIndexIfNotExists(requestIndexName, RequestIndexMapping); err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Printf("写入 request 层文档: %d 条\n", len(rawDocs))
	requestIfaces := make([]interface{}, len(rawDocs))
	for i, doc := range rawDocs {
		requestIfaces[i] = doc
	}
	if err := esClient.BulkIndex(requestIndexName, requestIfaces); err != nil {
		return fmt.Errorf("request 层写入失败: %w", err)
	}
	fmt.Printf("request 层写入成功: %s（%d 条）\n", requestIndexName, len(rawDocs))
	return nil
}

// reindexTask 处理 task 步骤
func reindexTask(config *Config, esClient *ESClient, orgProvider *OrgProvider, backendClient *BackendClient, dateStr string, force bool) error {
	requestIndexName := fmt.Sprintf("costrict_chat_request_%s", dateStr)

	rawMessages, err := esClient.ScrollAll(requestIndexName)
	if err != nil {
		fmt.Printf("警告: 从 %s 读取数据失败: %v，跳过 task 步骤\n", requestIndexName, err)
		return nil
	}

	var rawDocs []RawDoc
	for _, raw := range rawMessages {
		var doc RawDoc
		if err := json.Unmarshal(raw, &doc); err != nil {
			fmt.Printf("警告: 反序列化 RawDoc 失败: %v\n", err)
			continue
		}
		rawDocs = append(rawDocs, doc)
	}

	fmt.Printf("从 %s 读取 %d 条 request 文档\n", requestIndexName, len(rawDocs))

	taskDocs := BuildTaskDocs(rawDocs)
	fmt.Printf("聚合生成 %d 个 task\n", len(taskDocs))

	// 按 TaskID 分组 rawDocs（仅 caller=="chat"）
	rawDocsByTask := make(map[string][]RawDoc)
	for _, d := range rawDocs {
		if d.Caller == "chat" && d.TaskID != "" {
			rawDocsByTask[d.TaskID] = append(rawDocsByTask[d.TaskID], d)
		}
	}

	for i := range taskDocs {
		td := &taskDocs[i]
		taskRawDocs := rawDocsByTask[td.TaskID]

		taskContent, err := ExtractTaskContent(td.TaskID, taskRawDocs, config.RawDataDir)
		if err != nil {
			fmt.Printf("警告: 提取 task %s 内容失败: %v\n", td.TaskID, err)
			continue
		}

		sourceFile, err := SaveTaskContent(taskContent, config.RawDataDir)
		if err != nil {
			fmt.Printf("警告: 保存 task %s 内容失败: %v\n", td.TaskID, err)
			continue
		}

		if config.AIEstimation.Enabled {
			minutes, reason, err := EstimateTaskMinutes(config.AIEstimation, taskContent)
			if err != nil {
				fmt.Printf("警告: AI估时 task %s 失败: %v\n", td.TaskID, err)
			} else {
				td.TaskAncientMinutes = minutes
				td.TaskAncientMinutesReason = reason
				fullPath := filepath.Join(config.RawDataDir, sourceFile)
				if err := UpdateTaskContentWithEstimation(taskContent, minutes, reason, fullPath); err != nil {
					fmt.Printf("警告: 回写AI估时结果 task %s 失败: %v\n", td.TaskID, err)
				}
			}
		}

		td.SourceFile = sourceFile
	}

	// 写入 PG
	pgSuccess := 0
	for _, td := range taskDocs {
		pgTask := MapTaskDocToPG(td, rawDocsByTask[td.TaskID])
		if err := backendClient.SaveTaskToPG(pgTask); err != nil {
			fmt.Printf("警告: PG写入task失败 %s: %v\n", td.TaskID, err)
		} else {
			pgSuccess++
		}

		convs := MapRawDocsToConversations(td.TaskID, rawDocsByTask[td.TaskID], config.RawDataDir)
		if err := backendClient.SaveConversationsToPG(convs); err != nil {
			fmt.Printf("警告: PG写入conversations失败 %s: %v\n", td.TaskID, err)
		}
	}
	fmt.Printf("PG 写入完成: %d/%d 个 task 成功\n", pgSuccess, len(taskDocs))

	if len(taskDocs) == 0 {
		fmt.Println("没有可写入的 task 文档，跳过")
		return nil
	}

	taskIndexName := fmt.Sprintf("costrict_chat_task_%s", dateStr)

	if force {
		// 尝试清空已有数据（降级为 delete_by_query）
		_ = esClient.DeleteIndex(taskIndexName)
	}

	// 确保索引存在（已存在则跳过）
	if err := esClient.CreateIndexIfNotExists(taskIndexName, TaskIndexMapping); err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Printf("写入 task 层文档: %d 条\n", len(taskDocs))
	taskIfaces := make([]interface{}, len(taskDocs))
	for i, doc := range taskDocs {
		taskIfaces[i] = doc
	}
	if err := esClient.BulkIndex(taskIndexName, taskIfaces); err != nil {
		return fmt.Errorf("task 层写入失败: %w", err)
	}
	fmt.Printf("task 层写入成功: %s（%d 条）\n", taskIndexName, len(taskDocs))
	return nil
}

// generateDateRange 生成从 startDate 到 endDate 的日期列表（YYYYMMDD 格式）
func generateDateRange(startDate, endDate string) ([]string, error) {
	start, err := time.Parse("20060102", startDate)
	if err != nil {
		return nil, fmt.Errorf("解析 start-date 失败: %w", err)
	}
	end, err := time.Parse("20060102", endDate)
	if err != nil {
		return nil, fmt.Errorf("解析 end-date 失败: %w", err)
	}
	if start.After(end) {
		return nil, fmt.Errorf("start-date (%s) 不能晚于 end-date (%s)", startDate, endDate)
	}

	var dates []string
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		dates = append(dates, d.Format("20060102"))
	}
	return dates, nil
}
