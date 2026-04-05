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
)

// QueryDecomposeTool 查询分解工具，将复杂问题拆解为可独立检索的子查询
type QueryDecomposeTool struct {
	lightModel model.ChatModel
}

type queryDecomposeInput struct {
	Query string `json:"query"`
}

type queryDecomposeOutput struct {
	SubQueries []string `json:"sub_queries"`
	Reasoning  string   `json:"reasoning"`
}

// NewQueryDecomposeTool 创建查询分解工具
func NewQueryDecomposeTool(lightModel model.ChatModel) *QueryDecomposeTool {
	return &QueryDecomposeTool{lightModel: lightModel}
}

func (t *QueryDecomposeTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "query_decompose",
		Desc: `将复杂问题拆解为 2-4 个可独立检索的子查询。
仅在以下场景使用：
- 对比类问题（如"A 和 B 的区别"）
- 多跳推理（如"X 导致了什么，对 Y 有什么影响"）
- 聚合类问题（如"总结 Z 的所有优缺点"）
简单的单一事实问题不要使用此工具，直接调用 knowledge_search。`,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"query": {
				Type:     schema.String,
				Desc:     "需要分解的复杂问题",
				Required: true,
			},
		}),
	}, nil
}

func (t *QueryDecomposeTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	var params queryDecomposeInput
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	if params.Query == "" {
		return "", fmt.Errorf("query is required")
	}

	prompt := fmt.Sprintf(`将以下用户查询拆解为 2-4 个可独立检索的子问题。

用户查询：%s

返回格式（严格 JSON，不要附加说明）：
{"sub_queries": ["子问题1", "子问题2", ...], "reasoning": "拆解理由的简短说明"}

拆解规则：
- 每个子问题应是一个完整的、可独立用于知识库检索的短句
- 子问题之间不应有依赖关系
- 保留原始查询中的关键实体和术语
- 最多 4 个子问题

只返回 JSON。`, params.Query)

	start := time.Now()
	resp, err := t.lightModel.Generate(ctx, []*schema.Message{
		{Role: schema.User, Content: prompt},
	})
	log.Printf("[Timing][QueryDecompose] duration_ms=%d query=%q",
		time.Since(start).Milliseconds(), params.Query)
	if err != nil {
		return "", fmt.Errorf("LLM decompose failed: %w", err)
	}

	content := strings.TrimSpace(resp.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var result queryDecomposeOutput
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		log.Printf("[QueryDecompose] JSON parse failed, returning original query: %v (content: %s)", err, content)
		result = queryDecomposeOutput{
			SubQueries: []string{params.Query},
			Reasoning:  "分解失败，返回原始查询",
		}
	}

	if len(result.SubQueries) == 0 {
		result.SubQueries = []string{params.Query}
		result.Reasoning = "未生成子查询，返回原始查询"
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}

var _ tool.InvokableTool = (*QueryDecomposeTool)(nil)
