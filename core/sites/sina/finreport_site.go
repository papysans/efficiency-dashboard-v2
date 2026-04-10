package sina

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"comdigger/core/sites"
)

// SinaFinReportSite 实现 Site 接口，下载新浪财报并存储为 Markdown
type SinaFinReportSite struct{}

// Name 返回插件名称
func (s *SinaFinReportSite) Name() string {
	return "sina.finreport"
}

// Fetch 下载新浪财报
func (s *SinaFinReportSite) Fetch(ctx context.Context, opts sites.FetchOptions) error {
	// 从 companyID（如 sz300454）提取纯数字股票代码
	reDigits := regexp.MustCompile(`\d+`)
	stockCode := reDigits.FindString(opts.CompanyID)
	if stockCode == "" {
		return fmt.Errorf("无法从 companyID %q 提取股票代码", opts.CompanyID)
	}

	// 确定存储根目录
	workdir, err := os.Getwd()
	if err != nil {
		workdir = "."
	}
	baseDir := filepath.Join(workdir, "sina-fin-report", opts.CompanyID)

	// 创建客户端
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	client, err := NewFinReportClient(timeout)
	if err != nil {
		return fmt.Errorf("初始化浏览器失败: %w", err)
	}
	defer client.Close()

	// 初始化计数器
	skipped, downloaded, failed := 0, 0, 0

	// 解析日期范围（From/To 格式：2018, 2024）
	fromYear, toYear := 0, 9999
	if opts.From != "" {
		if _, err := fmt.Sscanf(opts.From, "%d", &fromYear); err != nil {
			fmt.Printf("警告：无法解析 -from 参数 %q，将下载所有年份\n", opts.From)
			fromYear = 0
		}
	}
	if opts.To != "" {
		if _, err := fmt.Sscanf(opts.To, "%d", &toYear); err != nil {
			fmt.Printf("警告：无法解析 -to 参数 %q，将下载所有年份\n", opts.To)
			toYear = 9999
		}
	}

	// 遍历四种报告类型
	for _, rt := range allReportTypes {
		entries, err := client.FetchListPage(stockCode, rt)
		if err != nil {
			fmt.Printf("警告：获取%s列表失败: %v\n", rt.Name, err)
			continue
		}

		fmt.Printf("找到 %d 篇%s\n", len(entries), rt.Name)

		for _, entry := range entries {
			// 日期过滤：从 Period 中提取年份（格式：2018anna）
			var year int
			if n, _ := fmt.Sscanf(entry.Period, "%d", &year); n == 1 {
				// 检查是否在指定范围内
				if year < fromYear || year > toYear {
					fmt.Printf("跳过（超出日期范围 %d-%d）：%s\n", fromYear, toYear, entry.Period)
					skipped++
					continue
				}
			}

			filePath := filepath.Join(baseDir, entry.Period+".md")

			// 检查文件是否已存在（非 force 模式跳过）
			if !opts.Force {
				if _, err := os.Stat(filePath); err == nil {
					fmt.Printf("跳过（已存在）：%s\n", entry.Period)
					skipped++
					continue
				}
			}

			// 下载内容
			markdown, innerHTML, err := client.FetchReportContent(entry.URL)
			if err != nil {
				if errors.Is(err, ErrOnlyAttachment) {
					fmt.Printf("  跳过：%s（仅附件，无正文）\n", entry.Title)
					skipped++
					continue
				}
				fmt.Printf("下载失败 [%s]: %v\n", entry.Period, err)
				failed++
				continue
			}

			// 确保目录存在
			if err := os.MkdirAll(baseDir, 0755); err != nil {
				fmt.Printf("创建目录失败: %v\n", err)
				failed++
				continue
			}

			// 写入 Markdown 文件
			if err := os.WriteFile(filePath, []byte(markdown), 0644); err != nil {
				fmt.Printf("写入 Markdown 失败 [%s]: %v\n", entry.Period, err)
				failed++
				continue
			}

			// 写入原始 HTML 文件（供对比调试）
			htmlPath := filepath.Join(baseDir, entry.Period+".html")
			if err := os.WriteFile(htmlPath, []byte(innerHTML), 0644); err != nil {
				fmt.Printf("写入 HTML 失败 [%s]: %v\n", entry.Period, err)
				// HTML 写入失败不计入 failed，Markdown 已成功
			}

			fmt.Printf("已下载：%s → %s\n", entry.Period, filePath)
			downloaded++
		}
	}

	fmt.Printf("完成：下载 %d 篇，跳过 %d 篇，失败 %d 篇\n", downloaded, skipped, failed)

	if failed > 0 {
		return fmt.Errorf("有 %d 篇财报下载失败", failed)
	}
	return nil
}

func init() {
	sites.Register(&SinaFinReportSite{})
}
