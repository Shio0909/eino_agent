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
	"strings"
	"time"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ModeRetriever 支持按模式检索的接口（由 CompositeRetriever 实现）
type ModeRetriever interface {
	RetrieveWithMode(ctx context.Context, query string, mode string) ([]*schema.Document, error)
}

// KnowledgeTool 知识库检索工具
type KnowledgeTool struct {
	retriever        retriever.Retriever
	topK             int
	maxContentPerDoc int                // 每篇文档最大字符数
	maxTotalContent  int                // 总内容最大字符数
	lastDocs         []*schema.Document // 缓存最近一次检索结果，供 Agent 模式回填 sources
}

// KnowledgeToolInput 知识库工具输入
type KnowledgeToolInput struct {
	Query string `json:"query" description:"要检索的问题或关键词"`
	Mode  string `json:"mode,omitempty" description:"检索模式：auto(默认混合检索)|semantic(语义向量)|exact(关键词精确)|graph(知识图谱)"`
}

// KnowledgeToolOutput 知识库工具输出
type KnowledgeToolOutput struct {
	Results []KnowledgeResult `json:"results"`
	Mode    string            `json:"mode"`
}

// KnowledgeResult 检索结果
type KnowledgeResult struct {
	Content string  `json:"content"`
	Source  string  `json:"source"`
	Score   float64 `json:"score"`
}

// NewKnowledgeTool 创建知识库工具
func NewKnowledgeTool(r retriever.Retriever, topK, maxContentPerDoc, maxTotalContent int) *KnowledgeTool {
	if topK <= 0 {
		topK = 5
	}
	if maxContentPerDoc <= 0 {
		maxContentPerDoc = 800
	}
	if maxTotalContent <= 0 {
		maxTotalContent = 8000
	}
	return &KnowledgeTool{
		retriever:        r,
		topK:             topK,
		maxContentPerDoc: maxContentPerDoc,
		maxTotalContent:  maxTotalContent,
	}
}

// Info 返回工具信息
func (t *KnowledgeTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "knowledge_search",
		Desc: `在知识库中检索相关文档。当用户询问特定领域的知识时使用此工具。
支持多种检索模式：
- auto（默认）：混合检索，综合向量、关键词、图谱结果
- semantic：语义向量检索，适合概念性/模糊问题
- exact：关键词精确检索，适合术语、命令、错误码
- graph：知识图谱检索，适合实体关系和关联查询`,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"query": {
				Type:     schema.String,
				Desc:     "要检索的问题或关键词",
				Required: true,
			},
			"mode": {
				Type: schema.String,
				Desc: "检索模式：auto(默认), semantic, exact, graph",
			},
		}),
	}, nil
}

// InvokableRun 执行工具
func (t *KnowledgeTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	var params KnowledgeToolInput
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	if params.Query == "" {
		return "", fmt.Errorf("query is required")
	}

	mode := strings.ToLower(strings.TrimSpace(params.Mode))
	if mode == "" {
		mode = "auto"
	}

	retrieveStart := time.Now()
	var docs []*schema.Document
	var err error

	// 如果 retriever 支持按模式检索且 mode 非 auto，使用模式检索
	if mode != "auto" {
		if mr, ok := t.retriever.(ModeRetriever); ok {
			docs, err = mr.RetrieveWithMode(ctx, params.Query, mode)
		} else {
			log.Printf("[KnowledgeTool] retriever 不支持 mode=%s，降级为 auto", mode)
			mode = "auto"
			docs, err = t.retriever.Retrieve(ctx, params.Query)
		}
	} else {
		docs, err = t.retriever.Retrieve(ctx, params.Query)
	}

	log.Printf("[Timing][KnowledgeTool] stage=retrieve mode=%s duration_ms=%d docs=%d query=%q",
		mode, time.Since(retrieveStart).Milliseconds(), len(docs), params.Query)
	if err != nil {
		return "", fmt.Errorf("retrieve (mode=%s): %w", mode, err)
	}

	// 缓存检索结果，供 ChatService 回填 sources 时直接使用
	t.lastDocs = append(t.lastDocs, docs...)

	// 转换结果，截断内容防止上下文爆炸
	results := make([]KnowledgeResult, 0, t.topK)
	totalChars := 0
	for i, doc := range docs {
		if i >= t.topK {
			break
		}
		score := doc.Score()
		if score == 0 {
			score = 1.0 - float64(i)*0.1
		}
		content := doc.Content
		if len(content) > t.maxContentPerDoc {
			content = content[:t.maxContentPerDoc] + "..."
		}
		if totalChars+len(content) > t.maxTotalContent && i > 0 {
			break
		}
		totalChars += len(content)
		results = append(results, KnowledgeResult{
			Content: content,
			Source:  doc.ID,
			Score:   score,
		})
	}

	output := KnowledgeToolOutput{Results: results, Mode: mode}
	jsonBytes, err := json.Marshal(output)
	if err != nil {
		return "", err
	}

	return string(jsonBytes), nil
}

// Ensure interface implementation
var _ tool.InvokableTool = (*KnowledgeTool)(nil)

// LastDocs 返回所有检索的文档结果（支持多次检索累积）
func (t *KnowledgeTool) LastDocs() []*schema.Document {
	return t.lastDocs
}

// ResetDocs 重置缓存的文档（每次请求开始时调用）
func (t *KnowledgeTool) ResetDocs() {
	t.lastDocs = nil
}
