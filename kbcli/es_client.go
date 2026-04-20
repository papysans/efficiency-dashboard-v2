package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

// ESClient ES 客户端封装
type ESClient struct {
	client *elasticsearch.Client
}

// NewESClient 创建 ES 客户端（忽略 SSL 证书校验）
func NewESClient(config *Config) (*ESClient, error) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	cfg := elasticsearch.Config{
		Addresses: []string{config.Elasticsearch.URL},
		Username:  config.Elasticsearch.Username,
		Password:  config.Elasticsearch.Password,
		Transport: transport,
	}
	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("创建ES客户端失败: %w", err)
	}
	return &ESClient{client: client}, nil
}

// CreateIndexIfNotExists 如果索引不存在则创建，并设置 mapping
func (e *ESClient) CreateIndexIfNotExists(indexName, mapping string) error {
	// 检查索引是否存在
	res, err := e.client.Indices.Exists([]string{indexName})
	if err != nil {
		return fmt.Errorf("检查索引 %s 是否存在失败: %w", indexName, err)
	}
	defer res.Body.Close()

	if res.StatusCode == 200 {
		// 索引已存在
		return nil
	}

	// 创建索引（如果 Exists 返回 403/404 都尝试创建）
	req := esapi.IndicesCreateRequest{
		Index: indexName,
		Body:  strings.NewReader(mapping),
	}
	createRes, err := req.Do(context.Background(), e.client)
	if err != nil {
		return fmt.Errorf("创建索引 %s 失败: %w", indexName, err)
	}
	defer createRes.Body.Close()

	if createRes.IsError() {
		body, _ := io.ReadAll(createRes.Body)
		bodyStr := string(body)
		// 索引已存在不视为错误（可能 Exists 检查因权限不足返回非200）
		if strings.Contains(bodyStr, "resource_already_exists_exception") {
			return nil
		}
		return fmt.Errorf("创建索引 %s 返回错误: %s, %s", indexName, createRes.Status(), bodyStr)
	}
	fmt.Printf("索引 %s 创建成功\n", indexName)
	return nil
}

// BulkIndex 批量写入文档到指定索引，docs 是任意结构体切片
func (e *ESClient) BulkIndex(indexName string, docs []interface{}) error {
	if len(docs) == 0 {
		return nil
	}

	var buf bytes.Buffer
	for _, doc := range docs {
		// action line
		meta := fmt.Sprintf(`{"index":{"_index":"%s"}}`, indexName)
		buf.WriteString(meta)
		buf.WriteByte('\n')
		// source line
		data, err := json.Marshal(doc)
		if err != nil {
			return fmt.Errorf("序列化文档失败: %w", err)
		}
		buf.Write(data)
		buf.WriteByte('\n')
	}

	res, err := e.client.Bulk(bytes.NewReader(buf.Bytes()))
	if err != nil {
		return fmt.Errorf("bulk写入失败: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("bulk写入返回错误: %s", res.Status())
	}

	// 解析响应，检查是否有 item 级别错误
	var bulkRes struct {
		Errors bool `json:"errors"`
		Items  []map[string]struct {
			Status int `json:"status"`
			Error  *struct {
				Type   string `json:"type"`
				Reason string `json:"reason"`
			} `json:"error,omitempty"`
		} `json:"items"`
	}
	if err := json.NewDecoder(res.Body).Decode(&bulkRes); err != nil {
		return fmt.Errorf("解析bulk响应失败: %w", err)
	}
	if bulkRes.Errors {
		var errCount int
		for _, item := range bulkRes.Items {
			for _, action := range item {
				if action.Error != nil {
					errCount++
					fmt.Printf("  写入错误: %s - %s\n", action.Error.Type, action.Error.Reason)
				}
			}
		}
		return fmt.Errorf("bulk写入有 %d 条记录失败", errCount)
	}
	return nil
}

// DeleteIndex 删除指定索引。若无 delete_index 权限，降级为 delete_by_query 清空文档
func (e *ESClient) DeleteIndex(indexName string) error {
	res, err := e.client.Indices.Delete([]string{indexName})
	if err != nil {
		return fmt.Errorf("删除索引 %s 失败: %w", indexName, err)
	}
	defer res.Body.Close()

	if res.StatusCode == 200 {
		fmt.Printf("索引 %s 已删除\n", indexName)
		return nil
	}
	if res.StatusCode == 404 {
		fmt.Printf("索引 %s 不存在，跳过删除\n", indexName)
		return nil
	}

	// 403 权限不足时，降级为 delete_by_query 清空所有文档
	if res.StatusCode == 403 {
		fmt.Printf("无删除索引权限，改用 delete_by_query 清空 %s\n", indexName)
		return e.ClearIndex(indexName)
	}

	body, _ := io.ReadAll(res.Body)
	return fmt.Errorf("删除索引 %s 返回错误: 状态码 %d, 响应: %s", indexName, res.StatusCode, string(body))
}

// ClearIndex 使用 delete_by_query 清空索引中的所有文档（不删除索引本身）
func (e *ESClient) ClearIndex(indexName string) error {
	query := strings.NewReader(`{"query":{"match_all":{}}}`)
	req := esapi.DeleteByQueryRequest{
		Index: []string{indexName},
		Body:  query,
	}
	res, err := req.Do(context.Background(), e.client)
	if err != nil {
		return fmt.Errorf("清空索引 %s 失败: %w", indexName, err)
	}
	defer res.Body.Close()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("清空索引 %s 返回错误: 状态码 %d, 响应: %s", indexName, res.StatusCode, string(body))
	}

	// 等待刷新
	e.client.Indices.Refresh(e.client.Indices.Refresh.WithIndex(indexName))
	fmt.Printf("索引 %s 已清空\n", indexName)
	return nil
}

// ScrollAll 从指定索引查询全部文档（使用 Scroll API 分批拉取）
func (e *ESClient) ScrollAll(indexName string) ([]json.RawMessage, error) {
	query := `{"query":{"match_all":{}},"sort":[{"@timestamp":"asc"}]}`
	size := 1000

	// 首次搜索
	res, err := e.client.Search(
		e.client.Search.WithContext(context.Background()),
		e.client.Search.WithIndex(indexName),
		e.client.Search.WithBody(strings.NewReader(query)),
		e.client.Search.WithScroll(2*time.Minute),
		e.client.Search.WithSize(size),
	)
	if err != nil {
		return nil, fmt.Errorf("scroll初始搜索失败: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("scroll初始搜索返回错误: %s, %s", res.Status(), string(body))
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("读取scroll初始响应失败: %w", err)
	}

	var result struct {
		ScrollID string `json:"_scroll_id"`
		Hits     struct {
			Hits []struct {
				Source json.RawMessage `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析scroll初始响应失败: %w", err)
	}

	var allDocs []json.RawMessage
	for _, hit := range result.Hits.Hits {
		allDocs = append(allDocs, hit.Source)
	}

	scrollID := result.ScrollID

	// 循环获取后续批次
	for len(result.Hits.Hits) > 0 {
		scrollRes, err := e.client.Scroll(
			e.client.Scroll.WithContext(context.Background()),
			e.client.Scroll.WithScrollID(scrollID),
			e.client.Scroll.WithScroll(2*time.Minute),
		)
		if err != nil {
			return nil, fmt.Errorf("scroll后续查询失败: %w", err)
		}

		scrollBody, err := io.ReadAll(scrollRes.Body)
		scrollRes.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("读取scroll后续响应失败: %w", err)
		}

		if scrollRes.IsError() {
			return nil, fmt.Errorf("scroll后续查询返回错误: %s, %s", scrollRes.Status(), string(scrollBody))
		}

		result = struct {
			ScrollID string `json:"_scroll_id"`
			Hits     struct {
				Hits []struct {
					Source json.RawMessage `json:"_source"`
				} `json:"hits"`
			} `json:"hits"`
		}{}
		if err := json.Unmarshal(scrollBody, &result); err != nil {
			return nil, fmt.Errorf("解析scroll后续响应失败: %w", err)
		}

		for _, hit := range result.Hits.Hits {
			allDocs = append(allDocs, hit.Source)
		}
		scrollID = result.ScrollID
	}

	// 清除 scroll 上下文
	e.client.ClearScroll(
		e.client.ClearScroll.WithContext(context.Background()),
		e.client.ClearScroll.WithScrollID(scrollID),
	)

	return allDocs, nil
}

// SearchTasks 查询 ES 中的 task 文档
func (e *ESClient) SearchTasks(indexNames []string, query map[string]interface{}, size int) ([]map[string]interface{}, error) {
	body := map[string]interface{}{
		"query": query,
		"size":  size,
	}
	bodyBytes, _ := json.Marshal(body)

	indexStr := strings.Join(indexNames, ",")
	res, err := e.client.Search(
		e.client.Search.WithContext(context.Background()),
		e.client.Search.WithIndex(indexStr),
		e.client.Search.WithBody(bytes.NewReader(bodyBytes)),
		e.client.Search.WithIgnoreUnavailable(true),
		e.client.Search.WithAllowNoIndices(true),
	)
	if err != nil {
		return nil, fmt.Errorf("ES搜索失败: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		errBody, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("ES搜索返回错误: %s, %s", res.Status(), string(errBody))
	}

	var result struct {
		Hits struct {
			Hits []struct {
				Source map[string]interface{} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析ES搜索结果失败: %w", err)
	}

	var hits []map[string]interface{}
	for _, h := range result.Hits.Hits {
		hits = append(hits, h.Source)
	}
	return hits, nil
}

// GetUniqueDimensionIDs 使用 terms aggregation 获取指定字段的所有唯一值
func (e *ESClient) GetUniqueDimensionIDs(indexNames []string, field string) ([]string, error) {
	body := map[string]interface{}{
		"size": 0,
		"aggs": map[string]interface{}{
			"unique_ids": map[string]interface{}{
				"terms": map[string]interface{}{
					"field": field,
					"size":  10000,
				},
			},
		},
	}
	bodyBytes, _ := json.Marshal(body)

	indexStr := strings.Join(indexNames, ",")
	res, err := e.client.Search(
		e.client.Search.WithContext(context.Background()),
		e.client.Search.WithIndex(indexStr),
		e.client.Search.WithBody(bytes.NewReader(bodyBytes)),
		e.client.Search.WithIgnoreUnavailable(true),
		e.client.Search.WithAllowNoIndices(true),
	)
	if err != nil {
		return nil, fmt.Errorf("ES聚合查询失败: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		errBody, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("ES聚合查询返回错误: %s, %s", res.Status(), string(errBody))
	}

	var result struct {
		Aggregations struct {
			UniqueIDs struct {
				Buckets []struct {
					Key string `json:"key"`
				} `json:"buckets"`
			} `json:"unique_ids"`
		} `json:"aggregations"`
	}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析ES聚合结果失败: %w", err)
	}

	var ids []string
	for _, bucket := range result.Aggregations.UniqueIDs.Buckets {
		if bucket.Key != "" {
			ids = append(ids, bucket.Key)
		}
	}
	return ids, nil
}

// generateESIndexNames 根据日期范围生成索引名列表
func generateESIndexNames(prefix, startDate, endDate string) []string {
	start, err := time.Parse("20060102", startDate)
	if err != nil {
		return []string{prefix + "*"}
	}
	end, err := time.Parse("20060102", endDate)
	if err != nil {
		return []string{prefix + "*"}
	}

	var names []string
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		names = append(names, prefix+d.Format("20060102"))
	}
	return names
}
