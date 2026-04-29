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

	"github.com/cloudwego/eino/components/model"
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
	searched         bool
	lightModel       model.ChatModel // 轻量模型（用于冲突检测，可为 nil）
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
	Content        string  `json:"content"`
	Source         string  `json:"source"`
	SourceFilename string  `json:"source_filename,omitempty"`
	UploadedAt     string  `json:"uploaded_at,omitempty"`
	Score          float64 `json:"score"`
}

// NewKnowledgeTool 创建知识库工具
func NewKnowledgeTool(r retriever.Retriever, topK, maxContentPerDoc, maxTotalContent int) *KnowledgeTool {
	if topK <= 0 {
		topK = 5
	}
	if maxContentPerDoc <= 0 {
		maxContentPerDoc = 1500
	}
	if maxTotalContent <= 0 {
		maxTotalContent = 15000
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

	t.searched = true
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
		// 从 metadata 提取来源文件名和上传时间
		sourceFilename, _ := doc.MetaData["source_filename"].(string)
		uploadedAt, _ := doc.MetaData["uploaded_at"].(string)

		results = append(results, KnowledgeResult{
			Content:        content,
			Source:         doc.ID,
			SourceFilename: sourceFilename,
			UploadedAt:     uploadedAt,
			Score:          score,
		})
	}

	output := KnowledgeToolOutput{Results: results, Mode: mode}
	jsonBytes, err := json.Marshal(output)
	if err != nil {
		return "", err
	}

	// P1 冲突检测：使用 lightModel 分析检索结果间的矛盾
	conflictWarning := t.detectConflicts(ctx, params.Query, results)

	return string(jsonBytes) + conflictWarning, nil
}

// Ensure interface implementation
var _ tool.InvokableTool = (*KnowledgeTool)(nil)

// SetLightModel 注入轻量模型（用于冲突检测）
func (t *KnowledgeTool) SetLightModel(m model.ChatModel) {
	t.lightModel = m
}

// LastDocs 返回所有检索的文档结果（支持多次检索累积）
func (t *KnowledgeTool) LastDocs() []*schema.Document {
	return t.lastDocs
}

// ResetDocs 重置缓存的文档（每次请求开始时调用）
func (t *KnowledgeTool) ResetDocs() {
	t.lastDocs = nil
	t.searched = false
}

func (t *KnowledgeTool) HasSearched() bool {
	return t.searched
}

// detectConflicts 使用 lightModel 检测检索结果中的信息冲突
func (t *KnowledgeTool) detectConflicts(ctx context.Context, query string, results []KnowledgeResult) string {
	if t.lightModel == nil || len(results) < 2 {
		return ""
	}

	// 构建文档摘要供 LLM 分析
	var sb strings.Builder
	for i, r := range results {
		source := r.SourceFilename
		if source == "" {
			source = r.Source
		}
		sb.WriteString(fmt.Sprintf("[文档%d] 来源: %s\n%s\n\n", i+1, source, r.Content))
	}

	prompt := fmt.Sprintf(`你是一个信息一致性检查器。分析以下检索结果，判断不同来源对同一事实是否存在矛盾或冲突。

用户查询：%s

检索结果：
%s

分析要求：
1. 只关注**事实性矛盾**（数值不同、结论相反、流程步骤不一致），忽略表述差异和补充性信息
2. 如果没有发现冲突，返回：{"conflicts": []}
3. 如果有冲突，返回 JSON：
{"conflicts": [{"topic": "冲突主题", "doc_a": "来源A说法", "doc_b": "来源B说法", "sources": ["文档1", "文档2"]}]}

只返回 JSON，不要附加说明。`, query, sb.String())

	detectStart := time.Now()
	resp, err := t.lightModel.Generate(ctx, []*schema.Message{
		{Role: schema.User, Content: prompt},
	})
	log.Printf("[Timing][ConflictDetect] duration_ms=%d query=%q",
		time.Since(detectStart).Milliseconds(), query)

	if err != nil {
		log.Printf("[ConflictDetect] LLM 调用失败: %v", err)
		return ""
	}

	content := strings.TrimSpace(resp.Content)
	// 去除推理模型的 <think>...</think> 标签
	if idx := strings.Index(content, "</think>"); idx >= 0 {
		content = strings.TrimSpace(content[idx+len("</think>"):])
	}
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	// 解析冲突结果
	var result struct {
		Conflicts []struct {
			Topic   string   `json:"topic"`
			DocA    string   `json:"doc_a"`
			DocB    string   `json:"doc_b"`
			Sources []string `json:"sources"`
		} `json:"conflicts"`
	}
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		log.Printf("[ConflictDetect] JSON 解析失败: %v (content: %s)", err, content)
		return ""
	}

	if len(result.Conflicts) == 0 {
		return ""
	}

	// 格式化冲突警告
	var warnings strings.Builder
	warnings.WriteString("\n\n⚠️ 检索结果冲突警告：\n")
	for _, c := range result.Conflicts {
		warnings.WriteString(fmt.Sprintf("• %s：", c.Topic))
		if len(c.Sources) >= 2 {
			warnings.WriteString(fmt.Sprintf("[%s] 认为「%s」", c.Sources[0], c.DocA))
			warnings.WriteString(fmt.Sprintf("，而 [%s] 认为「%s」", c.Sources[1], c.DocB))
		} else {
			warnings.WriteString(fmt.Sprintf("一方认为「%s」，另一方认为「%s」", c.DocA, c.DocB))
		}
		warnings.WriteString("\n")
	}
	warnings.WriteString("请在回答中标记这些冲突，不要只采信一方。")

	log.Printf("[ConflictDetect] 发现 %d 处冲突", len(result.Conflicts))
	return warnings.String()
}
