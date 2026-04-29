package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"eino_agent/internal/filter"
)

type HyDEKnowledgeTool struct {
	base  *KnowledgeTool
	model model.ChatModel
}

type HyDEKnowledgeInput struct {
	Query string `json:"query" description:"原始用户问题。仅在普通知识库检索结果不足时使用 HyDE 重试检索"`
}

type HyDEKnowledgeOutput struct {
	Results []KnowledgeResult `json:"results"`
	Mode    string            `json:"mode"`
	Message string            `json:"message,omitempty"`
}

func NewHyDEKnowledgeTool(base *KnowledgeTool, m model.ChatModel) *HyDEKnowledgeTool {
	return &HyDEKnowledgeTool{base: base, model: m}
}

func (t *HyDEKnowledgeTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "knowledge_search_hyde",
		Desc: `使用 HyDE 策略检索知识库。仅当普通 knowledge_search 结果为空、较少或明显不相关时使用。工具会先生成一段假想答案作为检索查询，再只返回真实知识库文档；假想答案不能作为最终回答依据。`,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"query": {
				Type:     schema.String,
				Desc:     "原始用户问题",
				Required: true,
			},
		}),
	}, nil
}

func (t *HyDEKnowledgeTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	if t.base == nil || t.base.retriever == nil {
		return "", fmt.Errorf("knowledge retriever is not configured")
	}
	if !t.base.HasSearched() {
		return marshalHyDEOutput(HyDEKnowledgeOutput{
			Results: nil,
			Mode:    "hyde_skipped",
			Message: "请先调用普通 knowledge_search；只有普通检索结果不足时才使用 HyDE 重试。",
		})
	}
	if t.model == nil {
		return "", fmt.Errorf("hyde model is not configured")
	}

	var params HyDEKnowledgeInput
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	query := strings.TrimSpace(params.Query)
	if query == "" {
		return "", fmt.Errorf("query is required")
	}

	hypothetical, err := t.generateHypotheticalDocument(ctx, query)
	if err != nil {
		return "", fmt.Errorf("generate hypothetical document: %w", err)
	}

	retrieveStart := time.Now()
	docs, err := t.base.retriever.Retrieve(ctx, hypothetical)
	log.Printf("[Timing][HyDEKnowledgeTool] stage=retrieve duration_ms=%d docs=%d query=%q hypothetical_chars=%d",
		time.Since(retrieveStart).Milliseconds(), len(docs), query, len(hypothetical))
	if err != nil {
		return "", fmt.Errorf("retrieve with hyde: %w", err)
	}

	t.base.lastDocs = append(t.base.lastDocs, docs...)
	results := make([]KnowledgeResult, 0, t.base.topK)
	totalChars := 0
	for i, doc := range docs {
		if i >= t.base.topK {
			break
		}
		score := doc.Score()
		if score == 0 {
			score = 1.0 - float64(i)*0.1
		}
		content := doc.Content
		if len(content) > t.base.maxContentPerDoc {
			content = content[:t.base.maxContentPerDoc] + "..."
		}
		if totalChars+len(content) > t.base.maxTotalContent && i > 0 {
			break
		}
		totalChars += len(content)
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

	output := HyDEKnowledgeOutput{Results: results, Mode: "hyde"}
	return marshalHyDEOutput(output)
}

func marshalHyDEOutput(output HyDEKnowledgeOutput) (string, error) {
	jsonBytes, err := json.Marshal(output)
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}

func (t *HyDEKnowledgeTool) generateHypotheticalDocument(ctx context.Context, query string) (string, error) {
	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: "你是知识库检索查询生成器。根据用户问题写一段可能出现在知识库文档中的简短中文说明，用于向量检索。不要回答用户，不要编造具体来源、链接、数字或人名。只输出检索文本。",
		},
		{Role: schema.User, Content: query},
	}
	resp, err := t.model.Generate(ctx, messages)
	if err != nil {
		return "", err
	}
	content := strings.TrimSpace(filter.StripThinkTags(resp.Content))
	if content == "" {
		return "", fmt.Errorf("empty hypothetical document")
	}
	return content, nil
}

var _ tool.InvokableTool = (*HyDEKnowledgeTool)(nil)
