// Package tool Web 搜索工具实现
package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// WebSearchTool Web 搜索工具
// 【Eino 特点】实现 Eino 的 tool.InvokableTool 接口
type WebSearchTool struct {
	config *WebSearchConfig
	client *http.Client
}

// WebSearchConfig Web 搜索配置
type WebSearchConfig struct {
	// Tavily API
	TavilyAPIKey string
	TavilyURL    string

	// 备选：SerpAPI
	SerpAPIKey string

	// 通用配置
	MaxResults int
	Timeout    int // 秒
}

// WebSearchInput Web 搜索输入
type WebSearchInput struct {
	Query string `json:"query" description:"搜索关键词"`
}

// WebSearchOutput Web 搜索输出
type WebSearchOutput struct {
	Results []WebSearchResult `json:"results"`
}

// WebSearchResult 搜索结果
type WebSearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"`
}

// NewWebSearchTool 创建 Web 搜索工具
func NewWebSearchTool(config *WebSearchConfig) *WebSearchTool {
	if config.MaxResults <= 0 {
		config.MaxResults = 5
	}
	if config.TavilyURL == "" {
		config.TavilyURL = "https://api.tavily.com/search"
	}
	return &WebSearchTool{
		config: config,
		client: &http.Client{},
	}
}

// Info 返回工具信息
func (t *WebSearchTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "web_search",
		Desc: "在互联网上搜索最新信息。当用户需要实时信息、新闻或知识库中没有的内容时使用。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"query": {
				Type:     schema.String,
				Desc:     "搜索关键词",
				Required: true,
			},
		}),
	}, nil
}

// InvokableRun 执行 Web 搜索
// 【Eino 特点】实现 tool.InvokableTool 接口
func (t *WebSearchTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	var params WebSearchInput
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	if params.Query == "" {
		return "", fmt.Errorf("query is required")
	}

	// 优先使用 Tavily
	if t.config.TavilyAPIKey != "" {
		return t.searchTavily(ctx, params.Query)
	}

	// 备选 SerpAPI
	if t.config.SerpAPIKey != "" {
		return t.searchSerp(ctx, params.Query)
	}

	return "", fmt.Errorf("no search API configured")
}

// searchTavily 使用 Tavily API 搜索
func (t *WebSearchTool) searchTavily(ctx context.Context, query string) (string, error) {
	reqBody := map[string]any{
		"api_key":     t.config.TavilyAPIKey,
		"query":       query,
		"max_results": t.config.MaxResults,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", t.config.TavilyURL, 
		bytes.NewReader(jsonBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("tavily request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("tavily error: %s - %s", resp.Status, string(body))
	}

	var tavilyResp struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tavilyResp); err != nil {
		return "", err
	}

	output := WebSearchOutput{
		Results: make([]WebSearchResult, len(tavilyResp.Results)),
	}
	for i, r := range tavilyResp.Results {
		output.Results[i] = WebSearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Content: r.Content,
		}
	}

	jsonBytes, err := json.Marshal(output)
	return string(jsonBytes), err
}

// searchSerp 使用 SerpAPI 搜索
func (t *WebSearchTool) searchSerp(ctx context.Context, query string) (string, error) {
	u := fmt.Sprintf("https://serpapi.com/search?api_key=%s&q=%s&num=%d",
		t.config.SerpAPIKey,
		url.QueryEscape(query),
		t.config.MaxResults,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return "", err
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("serp request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("serp error: %s - %s", resp.Status, string(body))
	}

	var serpResp struct {
		OrganicResults []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"organic_results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&serpResp); err != nil {
		return "", fmt.Errorf("serp decode: %w", err)
	}

	output := WebSearchOutput{
		Results: make([]WebSearchResult, len(serpResp.OrganicResults)),
	}
	for i, r := range serpResp.OrganicResults {
		output.Results[i] = WebSearchResult{
			Title:   r.Title,
			URL:     r.Link,
			Content: r.Snippet,
		}
	}

	jsonBytes, err := json.Marshal(output)
	return string(jsonBytes), err
}

// Ensure interface implementation
var _ tool.InvokableTool = (*WebSearchTool)(nil)
