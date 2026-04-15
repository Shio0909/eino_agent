// Package graphrag - 图谱检索器
//
// 实现 Eino retriever.Retriever 接口，可直接插入 HybridRetriever 的 graphRetriever 插槽
//
// 【查询流程】
// 1. 用 LLM 从用户 query 中抽取实体
// 2. 用实体名称在 Neo4j 中搜索匹配节点及其关系
// 3. 将命中的关联 chunk ID 作为 Document 返回
// 4. HybridRetriever 的 RRF 融合机制自动将结果与向量/关键词检索结果合并
package graphrag

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

// GraphRetriever 图谱检索器
// 实现 Eino retriever.Retriever 接口
type GraphRetriever struct {
	extractor  *Extractor       // 实体抽取器（查询时抽取实体）
	repository GraphRepository  // Neo4j 存储
	namespace  *NameSpace       // 图谱命名空间（知识库隔离）
	topK       int              // 最大返回 chunk 数
}

// GraphRetrieverConfig 图谱检索器配置
type GraphRetrieverConfig struct {
	Extractor  *Extractor
	Repository GraphRepository
	Namespace  *NameSpace
	TopK       int
}

// NewGraphRetriever 创建图谱检索器
func NewGraphRetriever(cfg *GraphRetrieverConfig) *GraphRetriever {
	topK := cfg.TopK
	if topK <= 0 {
		topK = 10
	}
	return &GraphRetriever{
		extractor:  cfg.Extractor,
		repository: cfg.Repository,
		namespace:  cfg.Namespace,
		topK:       topK,
	}
}

// Retrieve 实现 Eino retriever.Retriever 接口
// 【流程】query → LLM 抽取实体 → Neo4j 搜索 → 返回关联 chunk 文档
func (g *GraphRetriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	// 1. 从 query 中抽取实体
	entities, err := g.extractor.ExtractEntitiesFromQuery(ctx, query)
	if err != nil {
		log.Printf("[GraphRetriever] 实体抽取失败: %v", err)
		return nil, nil // 不中断主流程
	}

	if len(entities) == 0 {
		log.Println("[GraphRetriever] 未抽取到实体，跳过图谱检索")
		return nil, nil
	}

	log.Printf("[GraphRetriever] 抽取到实体: %v", entityNames(entities))

	// 2. 在 Neo4j 中搜索实体及关系
	graphData, err := g.repository.SearchNode(ctx, *g.namespace, entities)
	if err != nil {
		log.Printf("[GraphRetriever] Neo4j 搜索失败: %v", err)
		return nil, nil // 图谱搜索失败不影响主流程
	}

	// 3. 收集所有关联的 chunk ID（去重）
	chunkSet := make(map[string]struct{})
	entityInfo := make([]string, 0) // 用于构建上下文

	for _, node := range graphData.Node {
		for _, chunk := range node.Chunks {
			chunkSet[chunk] = struct{}{}
		}
		// 构建实体描述（包含类型信息）
		desc := node.Name
		if node.Type != "" && node.Type != "Other" {
			desc = fmt.Sprintf("[%s]%s", node.Type, node.Name)
		}
		if len(node.Attributes) > 0 {
			desc = fmt.Sprintf("%s(%s)", desc, strings.Join(node.Attributes, ", "))
		}
		entityInfo = append(entityInfo, desc)
	}

	// 4. 构建关系描述
	relationInfo := make([]string, 0, len(graphData.Relation))
	for _, rel := range graphData.Relation {
		relationInfo = append(relationInfo, fmt.Sprintf("%s -[%s]-> %s", rel.Node1, rel.Type, rel.Node2))
	}

	log.Printf("[GraphRetriever] 发现 %d 个实体, %d 个关系, %d 个关联 chunk",
		len(graphData.Node), len(graphData.Relation), len(chunkSet))

	// 5. 构建结果文档
	// 策略：将图谱上下文作为附加文档返回，让 RRF 融合决定最终排序
	docs := make([]*schema.Document, 0)

	// 5a. 如果有命中 chunk，将 chunk ID 作为文档返回（由下游检索 chunk 内容）
	i := 0
	for chunkID := range chunkSet {
		if i >= g.topK {
			break
		}
		docs = append(docs, &schema.Document{
			ID:      chunkID,
			Content: "", // chunk 内容由下游补充
			MetaData: map[string]any{
				"match_type": "graph",
				"source":     "graphrag",
			},
		})
		i++
	}

	// 5b. 将图谱关系上下文作为额外文档（提供实体关系信息增强回答）
	if len(entityInfo) > 0 || len(relationInfo) > 0 {
		var contextParts []string
		if len(entityInfo) > 0 {
			contextParts = append(contextParts, "【相关实体】"+strings.Join(entityInfo, "; "))
		}
		if len(relationInfo) > 0 {
			contextParts = append(contextParts, "【实体关系】"+strings.Join(relationInfo, "; "))
		}
		graphContext := strings.Join(contextParts, "\n")

		docs = append(docs, &schema.Document{
			ID:      "graph-context",
			Content: graphContext,
			MetaData: map[string]any{
				"match_type": "graph_context",
				"source":     "graphrag",
				"entities":   len(entityInfo),
				"relations":  len(relationInfo),
			},
		})
	}

	return docs, nil
}

// entityNames 从 QueryEntity 列表中提取名称列表（用于日志输出）
func entityNames(entities []QueryEntity) []string {
	names := make([]string, len(entities))
	for i, e := range entities {
		if e.Type != "" && e.Type != "Other" {
			names[i] = fmt.Sprintf("%s(%s)", e.Name, e.Type)
		} else {
			names[i] = e.Name
		}
	}
	return names
}
