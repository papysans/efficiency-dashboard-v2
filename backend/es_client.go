package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"
)

// ESConfig ES 连接配置
type ESConfig struct {
	URL      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// ESClient ES 客户端封装
type ESClient struct {
	client *elasticsearch.Client
}

// NewESClient 创建 ES 客户端（忽略 SSL 证书校验）
func NewESClient(config ESConfig) (*ESClient, error) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	cfg := elasticsearch.Config{
		Addresses: []string{config.URL},
		Username:  config.Username,
		Password:  config.Password,
		Transport: transport,
	}
	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("创建ES客户端失败: %w", err)
	}
	return &ESClient{client: client}, nil
}

// SearchResult 搜索结果
type SearchResult struct {
	Total int                      `json:"total"`
	Hits  []map[string]interface{} `json:"hits"`
}

// Search 封装 ES _search API，支持多索引查询
// 使用 ignore_unavailable=true 和 allow_no_indices=true 避免不存在的索引导致 404
func (e *ESClient) Search(indexNames []string, query map[string]interface{}, from, size int, sortField, sortOrder string) (*SearchResult, error) {
	body := map[string]interface{}{
		"query": query,
		"from":  from,
		"size":  size,
	}
	if sortField != "" {
		body["sort"] = []map[string]interface{}{
			{sortField: map[string]interface{}{"order": sortOrder}},
		}
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("序列化查询体失败: %w", err)
	}

	indexStr := strings.Join(indexNames, ",")
	res, err := e.client.Search(
		e.client.Search.WithIndex(indexStr),
		e.client.Search.WithBody(bytes.NewReader(bodyBytes)),
		e.client.Search.WithIgnoreUnavailable(true),
		e.client.Search.WithAllowNoIndices(true),
		e.client.Search.WithPretty(),
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
			Total struct {
				Value int `json:"value"`
			} `json:"total"`
			Hits []struct {
				Source map[string]interface{} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析ES搜索结果失败: %w", err)
	}

	hits := make([]map[string]interface{}, 0, len(result.Hits.Hits))
	for _, h := range result.Hits.Hits {
		hits = append(hits, h.Source)
	}

	return &SearchResult{
		Total: result.Hits.Total.Value,
		Hits:  hits,
	}, nil
}

// SearchWithFilter 基于 filters 构建 bool query 并搜索
// 支持的 filter key: taskId→term("task_id"), projectId→term("project_id"), userId→term("user_id")
func (e *ESClient) SearchWithFilter(indexNames []string, filters map[string]interface{}, from, size int, sortField, sortOrder string) (*SearchResult, error) {
	filterKeyMap := map[string]string{
		"taskId":    "task_id",
		"projectId": "project_id",
		"userId":    "user_id",
	}

	var filterClauses []map[string]interface{}
	for key, esField := range filterKeyMap {
		if val, ok := filters[key]; ok && val != "" {
			filterClauses = append(filterClauses, map[string]interface{}{
				"term": map[string]interface{}{esField: val},
			})
		}
	}

	var query map[string]interface{}
	if len(filterClauses) == 0 {
		query = map[string]interface{}{"match_all": map[string]interface{}{}}
	} else {
		query = map[string]interface{}{
			"bool": map[string]interface{}{
				"filter": filterClauses,
			},
		}
	}

	return e.Search(indexNames, query, from, size, sortField, sortOrder)
}

// IndexInfo 索引信息
type IndexInfo struct {
	Name     string `json:"name"`
	DocCount string `json:"docCount"`
}

// GetIndices 获取匹配 pattern 的索引列表
// 使用 _resolve/index API 替代 _cat/indices，避免权限问题
func (e *ESClient) GetIndices(pattern string) ([]IndexInfo, error) {
	// 使用底层 HTTP 调用 _resolve/index API
	req, err := http.NewRequest("GET", "/_resolve/index/"+pattern, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	res, err := e.client.Perform(req)
	if err != nil {
		return nil, fmt.Errorf("获取索引列表失败: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		errBody, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("获取索引列表返回错误: %d, %s", res.StatusCode, string(errBody))
	}

	var resolveResult struct {
		Indices []struct {
			Name string `json:"name"`
		} `json:"indices"`
	}
	if err := json.NewDecoder(res.Body).Decode(&resolveResult); err != nil {
		return nil, fmt.Errorf("解析索引列表失败: %w", err)
	}

	result := make([]IndexInfo, 0, len(resolveResult.Indices))
	for _, idx := range resolveResult.Indices {
		result = append(result, IndexInfo{
			Name:     idx.Name,
			DocCount: "",
		})
	}
	return result, nil
}

// Aggregate 封装 ES 聚合查询（size=0，不返回文档，只返回聚合结果）
// 使用 ignore_unavailable=true 和 allow_no_indices=true 避免不存在的索引导致 404
func (e *ESClient) Aggregate(indexNames []string, aggsQuery map[string]interface{}) (map[string]interface{}, error) {
	return e.AggregateWithQuery(indexNames, map[string]interface{}{"match_all": map[string]interface{}{}}, aggsQuery)
}

// AggregateWithQuery 封装带 query 条件的 ES 聚合查询
func (e *ESClient) AggregateWithQuery(indexNames []string, query map[string]interface{}, aggsQuery map[string]interface{}) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"size":  0,
		"query": query,
		"aggs":  aggsQuery,
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("序列化聚合查询体失败: %w", err)
	}

	indexStr := strings.Join(indexNames, ",")
	res, err := e.client.Search(
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

	var result map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析ES聚合结果失败: %w", err)
	}

	aggregations, ok := result["aggregations"].(map[string]interface{})
	if !ok {
		// 没有聚合结果（可能所有索引都不存在），返回空 map
		return make(map[string]interface{}), nil
	}
	return aggregations, nil
}
