// Package graphrag - 图谱检索器
//
// 实现 Eino retriever.Retriever 接口，由 container.CompositeRetriever 作为图谱信号源调用
//
// 【查询流程】
//  1. 用 LLM 从用户 query 中抽取实体
//  2. 用实体名称在 Neo4j 中搜索种子节点，并做有界变长遍历找到相关实体
//  3. 按跳数/关系权重给关联 chunk 打分，确定性排序后取 topK 作为 Document 返回
//  4. 调用方（CompositeRetriever）不把这些结果送入 RRF：命中 chunk 只作为已融合结果的加分信号，
//     实体/关系摘要（ID 为 graph-context 的文档）作为附加上下文注入最终结果
package graphrag

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

// GraphRetriever 图谱检索器
// 实现 Eino retriever.Retriever 接口
type GraphRetriever struct {
	extractor  *Extractor      // 实体抽取器（查询时抽取实体）
	repository GraphRepository // Neo4j 存储
	namespace  *NameSpace      // 图谱命名空间（知识库隔离）
	topK       int             // 最大返回 chunk 数
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
	namespace := NameSpace{}
	if g.namespace != nil {
		namespace = *g.namespace
	}
	graphData, err := g.repository.SearchNode(ctx, namespace, entities)
	if err != nil {
		log.Printf("[GraphRetriever] Neo4j 搜索失败: %v", err)
		return nil, nil // 图谱搜索失败不影响主流程
	}
	if graphData == nil {
		return nil, nil
	}

	docs := buildGraphDocuments(graphData, g.topK)
	log.Printf("[GraphRetriever] 发现 %d 个实体, %d 个关系, 返回 %d 个文档",
		len(graphData.Node), len(graphData.Relation), len(docs))
	return docs, nil
}

// scoredChunk 图谱命中的 chunk 及其得分
type scoredChunk struct {
	ID    string
	Score float64
	Hops  int
}

// rankGraphChunks 汇总实体关联的 chunk 得分并确定性排序：
// 每个实体贡献 weight/(1+hops)（直接命中得 1，一跳邻居得 1/2 ……），
// 同一 chunk 被多个实体引用时得分累加；按得分降序、chunk ID 升序排列后截取 topK。
func rankGraphChunks(nodes []*GraphNode, topK int) []scoredChunk {
	byID := make(map[string]*scoredChunk)
	for _, node := range nodes {
		if node == nil {
			continue
		}
		weight := node.Weight
		if weight <= 0 {
			weight = 1.0
		}
		contribution := weight / float64(1+node.Hops)
		seen := make(map[string]bool, len(node.Chunks))
		for _, chunkID := range node.Chunks {
			if chunkID == "" || seen[chunkID] {
				continue
			}
			seen[chunkID] = true
			sc, ok := byID[chunkID]
			if !ok {
				byID[chunkID] = &scoredChunk{ID: chunkID, Score: contribution, Hops: node.Hops}
				continue
			}
			sc.Score += contribution
			if node.Hops < sc.Hops {
				sc.Hops = node.Hops
			}
		}
	}

	ranked := make([]scoredChunk, 0, len(byID))
	for _, sc := range byID {
		ranked = append(ranked, *sc)
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].Score != ranked[j].Score {
			return ranked[i].Score > ranked[j].Score
		}
		return ranked[i].ID < ranked[j].ID
	})
	if topK > 0 && len(ranked) > topK {
		ranked = ranked[:topK]
	}
	return ranked
}

// buildGraphDocuments 把检索到的子图转换为 chunk 文档 + 实体关系上下文文档
func buildGraphDocuments(graphData *GraphData, topK int) []*schema.Document {
	docs := make([]*schema.Document, 0)
	for _, sc := range rankGraphChunks(graphData.Node, topK) {
		doc := &schema.Document{
			ID:      sc.ID,
			Content: "", // chunk 内容由下游补充
			MetaData: map[string]any{
				"match_type": "graph",
				"source":     "graphrag",
				"graph_hops": sc.Hops,
			},
		}
		docs = append(docs, doc.WithScore(sc.Score))
	}

	entityInfo := make([]string, 0, len(graphData.Node))
	for _, node := range graphData.Node {
		if node == nil {
			continue
		}
		desc := node.Name
		if node.Type != "" && node.Type != "Other" {
			desc = fmt.Sprintf("[%s]%s", node.Type, node.Name)
		}
		if len(node.Attributes) > 0 {
			desc = fmt.Sprintf("%s(%s)", desc, strings.Join(node.Attributes, ", "))
		}
		entityInfo = append(entityInfo, desc)
	}

	relationInfo := make([]string, 0, len(graphData.Relation))
	for _, rel := range graphData.Relation {
		if rel == nil {
			continue
		}
		relationInfo = append(relationInfo, fmt.Sprintf("%s -[%s]-> %s", rel.Node1, rel.Type, rel.Node2))
	}

	if len(entityInfo) > 0 || len(relationInfo) > 0 {
		var contextParts []string
		if len(entityInfo) > 0 {
			contextParts = append(contextParts, "【相关实体】"+strings.Join(entityInfo, "; "))
		}
		if len(relationInfo) > 0 {
			contextParts = append(contextParts, "【实体关系】"+strings.Join(relationInfo, "; "))
		}
		docs = append(docs, &schema.Document{
			ID:      "graph-context",
			Content: strings.Join(contextParts, "\n"),
			MetaData: map[string]any{
				"match_type": "graph_context",
				"source":     "graphrag",
				"entities":   len(entityInfo),
				"relations":  len(relationInfo),
			},
		})
	}
	return docs
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
