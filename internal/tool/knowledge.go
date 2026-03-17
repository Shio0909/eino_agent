// Package tool 实现 RAG 相关的工具
//
// 【Eino 特点】使用 Eino 的 tool.BaseTool 接口定义工具
// 工具可被 ReAct Agent 自动调用
package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// KnowledgeTool 知识库检索工具
// 【Eino 特点】实现 Eino 的 tool.BaseTool 接口
type KnowledgeTool struct {
	retriever   retriever.Retriever
	topK       int
	lastDocs   []*schema.Document // 缓存最近一次检索结果，供 Agent 模式回填 sources
}

// KnowledgeToolInput 知识库工具输入
type KnowledgeToolInput struct {
	Query string `json:"query" description:"要检索的问题或关键词"`
}

// KnowledgeToolOutput 知识库工具输出
type KnowledgeToolOutput struct {
	Results []KnowledgeResult `json:"results"`
}

// KnowledgeResult 检索结果
type KnowledgeResult struct {
	Content string  `json:"content"`
	Source  string  `json:"source"`
	Score   float64 `json:"score"`
}

// NewKnowledgeTool 创建知识库工具
func NewKnowledgeTool(r retriever.Retriever, topK int) *KnowledgeTool {
	if topK <= 0 {
		topK = 5
	}
	return &KnowledgeTool{
		retriever: r,
		topK:      topK,
	}
}

// Info 返回工具信息
// 【Eino 特点】定义工具的 schema，让 LLM 知道如何调用
func (t *KnowledgeTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "knowledge_search",
		Desc: "在知识库中检索相关文档。当用户询问特定领域的知识时使用此工具。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"query": {
				Type:     schema.String,
				Desc:     "要检索的问题或关键词",
				Required: true,
			},
		}),
	}, nil
}

// Run 执行工具
// 【Eino 特点】工具的实际执行逻辑
// InvokableRun 执行工具
// 【Eino 特点】实现 tool.InvokableTool 接口
func (t *KnowledgeTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	var params KnowledgeToolInput
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	if params.Query == "" {
		return "", fmt.Errorf("query is required")
	}

	// 执行检索
	retrieveStart := time.Now()
	docs, err := t.retriever.Retrieve(ctx, params.Query)
	log.Printf("[Timing][KnowledgeTool] stage=retrieve duration_ms=%d docs=%d query=%q", time.Since(retrieveStart).Milliseconds(), len(docs), params.Query)
	if err != nil {
		return "", fmt.Errorf("retrieve: %w", err)
	}

	// 缓存检索结果，供 ChatService 回填 sources 时直接使用
	t.lastDocs = docs

	// 转换结果
	results := make([]KnowledgeResult, 0, t.topK)
	for i, doc := range docs {
		if i >= t.topK {
			break
		}
		results = append(results, KnowledgeResult{
			Content: doc.Content,
			Source:  doc.ID,
			Score:   1.0 - float64(i)*0.1, // 简单的位置分数
		})
	}

	output := KnowledgeToolOutput{Results: results}
	jsonBytes, err := json.Marshal(output)
	if err != nil {
		return "", err
	}

	return string(jsonBytes), nil
}

// Ensure interface implementation
var _ tool.InvokableTool = (*KnowledgeTool)(nil)

// LastDocs 返回最近一次检索的文档结果（供 Agent 回填 sources，避免二次检索）
func (t *KnowledgeTool) LastDocs() []*schema.Document {
	return t.lastDocs
}
