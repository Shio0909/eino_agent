// Package graphrag 实现基于 Neo4j 的 GraphRAG 增强检索
//
// 【面试故事线】"向量检索对实体关系类问题（'xx 的作者是谁'）效果差
// → 引入 Neo4j 图谱增强，文档入库时 LLM 抽取实体-关系存图，
// 查询时向量检索 + 图检索并行执行，合并结果送 Reranker 排序"
//
// 参考 WeKnora 的 GraphRAG 实现，适配到 Eino 框架
package graphrag

import "context"

// ── 图存储数据模型（用于 Neo4j Cypher 操作） ──

// GraphNode 图谱节点 — 对应 Neo4j 中的 ENTITY 标签节点
type GraphNode struct {
	Name       string   `json:"name,omitempty"`        // 实体名称
	Type       string   `json:"type,omitempty"`        // 实体类型（Technology/Concept/Component/Person 等）
	Chunks     []string `json:"chunks,omitempty"`      // 关联的文档 Chunk IDs
	Attributes []string `json:"attributes,omitempty"`  // 实体属性列表
}

// GraphRelation 图谱关系
type GraphRelation struct {
	Node1 string `json:"node1,omitempty"` // 源实体名
	Node2 string `json:"node2,omitempty"` // 目标实体名
	Type  string `json:"type,omitempty"`  // 关系类型（如"作者"、"别名"）
}

// GraphData 图谱数据（一次抽取的结果）
type GraphData struct {
	Text     string           `json:"text,omitempty"`     // 原文
	Node     []*GraphNode     `json:"node,omitempty"`     // 节点列表
	Relation []*GraphRelation `json:"relation,omitempty"` // 关系列表
}

// NameSpace 命名空间（用于 Neo4j 标签隔离）
// 不同知识库的图使用不同标签前缀，实现数据隔离
type NameSpace struct {
	KnowledgeBase string `json:"knowledge_base"` // 知识库 ID
	Knowledge     string `json:"knowledge"`      // 知识文件 ID
}

// Labels 返回命名空间对应的标签列表
func (n NameSpace) Labels() []string {
	res := make([]string, 0)
	if n.KnowledgeBase != "" {
		res = append(res, n.KnowledgeBase)
	}
	if n.Knowledge != "" {
		res = append(res, n.Knowledge)
	}
	return res
}

// ── 图可视化 DTO ──

// VisNode 可视化节点
type VisNode struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Degree     int    `json:"degree"`
	ChunkCount int    `json:"chunk_count"`
}

// VisEdge 可视化边
type VisEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Label  string `json:"label"`
}

// VisGraph 可视化图数据
type VisGraph struct {
	Nodes []VisNode `json:"nodes"`
	Edges []VisEdge `json:"edges"`
}

// QueryEntity 查询时抽取的实体（带类型信息用于图谱检索过滤）
type QueryEntity struct {
	Name string `json:"entity"`      // 实体名称
	Type string `json:"entity_type"` // 实体类型
}

// ── GraphRAG 配置 ──

// Config GraphRAG 配置
type Config struct {
	Enabled     bool    `yaml:"enabled"`      // 是否启用 GraphRAG
	Neo4jURI    string  `yaml:"neo4j_uri"`    // Neo4j 连接 URI (bolt://...)
	Neo4jUser   string  `yaml:"neo4j_user"`   // Neo4j 用户名
	Neo4jPass   string  `yaml:"neo4j_pass"`   // Neo4j 密码
	ExtractTemp float64 `yaml:"extract_temp"` // 实体抽取的 LLM 温度
}

// ── Repository 接口 ──

// GraphRepository 图存储接口
// 参考 WeKnora 的 RetrieveGraphRepository
type GraphRepository interface {
	// AddGraph 将图数据写入 Neo4j
	AddGraph(ctx context.Context, namespace NameSpace, graphs []*GraphData) error
	// DelGraph 删除指定命名空间的图数据
	DelGraph(ctx context.Context, namespaces []NameSpace) error
	// SearchNode 根据实体列表在图中检索（支持类型约束）
	SearchNode(ctx context.Context, namespace NameSpace, entities []QueryEntity) (*GraphData, error)
	// GetGraphForVis 获取可视化用的子图（节点+边，带 limit）
	GetGraphForVis(ctx context.Context, namespace NameSpace, limit int) (*VisGraph, error)
}
