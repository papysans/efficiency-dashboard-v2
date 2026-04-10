package news

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	baseURL = "https://newsapi.ws4.cn/api/v1/dailynews/"
	timeout = 10 * time.Second
)

// FetchPlatform 获取单个平台的新闻数据
func FetchPlatform(platform string) ([]NewsItem, error) {
	client := &http.Client{Timeout: timeout}
	url := fmt.Sprintf("%s?platform=%s", baseURL, platform)

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("请求平台 %s 失败: %w", platform, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取平台 %s 响应失败: %w", platform, err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("解析平台 %s 响应失败: %w", platform, err)
	}

	if apiResp.Status != "200" {
		return nil, fmt.Errorf("平台 %s 返回错误: %s", platform, apiResp.Msg)
	}

	return apiResp.Data, nil
}

// FetchAll 并行获取多个平台的新闻数据，单个平台失败不影响其他平台
func FetchAll(platforms []string) map[string][]NewsItem {
	result := make(map[string][]NewsItem)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, platform := range platforms {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			items, err := FetchPlatform(p)
			if err != nil {
				// 单平台失败只记录，不中断
				fmt.Printf("[新闻] 平台 %s 获取失败: %v\n", p, err)
				return
			}
			mu.Lock()
			result[p] = items
			mu.Unlock()
		}(platform)
	}

	wg.Wait()
	return result
}
