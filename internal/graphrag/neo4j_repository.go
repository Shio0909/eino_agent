// Package graphrag - Neo4j Repository 实现
//
// 【Cypher 操作】使用 APOC 插件的 merge 语义实现 Upsert，
// 批量删除使用 apoc.periodic.iterate 防止大事务
//
// 参考 WeKnora: internal/application/repository/retriever/neo4j/repository.go
package graphrag

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Neo4jRepository Neo4j 图存储实现
type Neo4jRepository struct {
	driver     neo4j.DriverWithContext
	nodePrefix string
}

// NewNeo4jRepository 创建 Neo4j Repository
func NewNeo4jRepository(driver neo4j.DriverWithContext) GraphRepository {
	return &Neo4jRepository{driver: driver, nodePrefix: "ENTITY"}
}

// removeHyphen 去除连字符（Neo4j 标签不支持连字符）
func removeHyphen(s string) string {
	return strings.ReplaceAll(s, "-", "_")
}

// relTypeRE 匹配 Neo4j 关系类型允许的字符（字母、数字、下划线、中文）
var relTypeRE = regexp.MustCompile(`[^a-zA-Z0-9_\p{Han}]`)

// sanitizeRelType 清理关系类型名称，确保 Neo4j Cypher 安全
// 移除反引号、斜杠等特殊字符，截断过长名称
func sanitizeRelType(relType string) string {
	relType = relTypeRE.ReplaceAllString(relType, "_")
	// 合并连续下划线
	for strings.Contains(relType, "__") {
		relType = strings.ReplaceAll(relType, "__", "_")
	}
	relType = strings.Trim(relType, "_")
	// 截断过长的关系类型（超过 50 字符的通常是抽取错误）
	runes := []rune(relType)
	if len(runes) > 50 {
		relType = string(runes[:50])
	}
	if relType == "" {
		relType = "RELATED_TO"
	}
	return relType
}

// Labels 返回命名空间对应的 Neo4j 标签列表
func (r *Neo4jRepository) Labels(namespace NameSpace) []string {
	res := make([]string, 0)
	for _, label := range namespace.Labels() {
		res = append(res, r.nodePrefix+removeHyphen(label))
	}
	return res
}

// Label 返回命名空间对应的标签表达式（用于 Cypher）
func (r *Neo4jRepository) Label(namespace NameSpace) string {
	labels := r.Labels(namespace)
	return strings.Join(labels, ":")
}

// AddGraph 将图数据写入 Neo4j
// 使用 apoc.merge.node / apoc.merge.relationship 实现 Upsert
func (r *Neo4jRepository) AddGraph(ctx context.Context, namespace NameSpace, graphs []*GraphData) error {
	if r.driver == nil {
		log.Println("[GraphRAG] Neo4j 未启用，跳过 AddGraph")
		return nil
	}
	for _, graph := range graphs {
		if err := r.addGraph(ctx, namespace, graph); err != nil {
			return err
		}
	}
	return nil
}

func (r *Neo4jRepository) addGraph(ctx context.Context, namespace NameSpace, graph *GraphData) error {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		// ── 导入节点（使用 apoc.merge.node 实现 Upsert）──
		nodeImportQuery := `
			UNWIND $data AS row
			CALL apoc.merge.node(row.labels, {name: row.name, kg: row.knowledge_id}, row.props, {}) YIELD node
			SET node.chunks = apoc.coll.union(node.chunks, row.chunks)
			RETURN distinct 'done' AS result
		`
		nodeData := make([]map[string]interface{}, 0, len(graph.Node))
		for _, node := range graph.Node {
			nodeData = append(nodeData, map[string]interface{}{
				"name":         node.Name,
				"knowledge_id": namespace.Knowledge,
				"props":        map[string][]string{"attributes": node.Attributes},
				"chunks":       node.Chunks,
				"labels":       r.Labels(namespace),
			})
		}
		if len(nodeData) > 0 {
			if _, err := tx.Run(ctx, nodeImportQuery, map[string]interface{}{"data": nodeData}); err != nil {
				return nil, fmt.Errorf("创建节点失败: %w", err)
			}
		}

		// ── 导入关系（使用 apoc.merge.relationship）──
		relImportQuery := `
			UNWIND $data AS row
			CALL apoc.merge.node(row.source_labels, {name: row.source, kg: row.knowledge_id}, {}, {}) YIELD node as source
			CALL apoc.merge.node(row.target_labels, {name: row.target, kg: row.knowledge_id}, {}, {}) YIELD node as target
			CALL apoc.merge.relationship(source, row.type, {}, row.attributes, target) YIELD rel
			RETURN distinct 'done'
		`
		relData := make([]map[string]interface{}, 0, len(graph.Relation))
		for _, rel := range graph.Relation {
			relType := rel.Type
			if relType == "" {
				relType = "RELATED_TO"
			}
			// Neo4j 关系类型只允许字母、数字、下划线（sanitize 特殊字符）
			relType = sanitizeRelType(relType)
			relData = append(relData, map[string]interface{}{
				"source":        rel.Node1,
				"target":        rel.Node2,
				"knowledge_id":  namespace.Knowledge,
				"type":          relType,
				"source_labels": r.Labels(namespace),
				"target_labels": r.Labels(namespace),
				"attributes":    map[string]interface{}{},
			})
		}
		if len(relData) > 0 {
			if _, err := tx.Run(ctx, relImportQuery, map[string]interface{}{"data": relData}); err != nil {
				return nil, fmt.Errorf("创建关系失败: %w", err)
			}
		}
		return nil, nil
	})
	if err != nil {
		log.Printf("[GraphRAG] AddGraph 失败: %v", err)
		return err
	}
	return nil
}

// DelGraph 删除指定命名空间的图数据
// 使用 apoc.periodic.iterate 批量删除，避免大事务
func (r *Neo4jRepository) DelGraph(ctx context.Context, namespaces []NameSpace) error {
	if r.driver == nil {
		return nil
	}
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		for _, namespace := range namespaces {
			labelExpr := r.Label(namespace)

			// 先删关系
			deleteRelsQuery := `
				CALL apoc.periodic.iterate(
					"MATCH (n:` + labelExpr + ` {kg: $knowledge_id})-[r]-(m:` + labelExpr + ` {kg: $knowledge_id}) RETURN r",
					"DELETE r",
					{batchSize: 1000, parallel: true, params: {knowledge_id: $knowledge_id}}
				) YIELD batches, total
				RETURN total
			`
			if _, err := tx.Run(ctx, deleteRelsQuery, map[string]interface{}{
				"knowledge_id": namespace.Knowledge,
			}); err != nil {
				return nil, fmt.Errorf("删除关系失败: %w", err)
			}

			// 再删节点
			deleteNodesQuery := `
				CALL apoc.periodic.iterate(
					"MATCH (n:` + labelExpr + ` {kg: $knowledge_id}) RETURN n",
					"DELETE n",
					{batchSize: 1000, parallel: true, params: {knowledge_id: $knowledge_id}}
				) YIELD batches, total
				RETURN total
			`
			if _, err := tx.Run(ctx, deleteNodesQuery, map[string]interface{}{
				"knowledge_id": namespace.Knowledge,
			}); err != nil {
				return nil, fmt.Errorf("删除节点失败: %w", err)
			}
		}
		return nil, nil
	})
	if err != nil {
		log.Printf("[GraphRAG] DelGraph 失败: %v", err)
		return err
	}
	return nil
}

// SearchNode 根据实体名称列表在 Neo4j 中检索
// 使用 CONTAINS 匹配，返回匹配节点及其关系和关联的 Chunk IDs
func (r *Neo4jRepository) SearchNode(
	ctx context.Context,
	namespace NameSpace,
	nodes []string,
) (*GraphData, error) {
	if r.driver == nil {
		return nil, nil
	}
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		labelExpr := r.Label(namespace)
		var nodeMatch string
		if labelExpr == "" {
			nodeMatch = "(n)"
		} else {
			nodeMatch = "(n:" + labelExpr + ")"
		}

		matchExpr := `ANY(nodeText IN $nodes WHERE size(nodeText) >= 2 AND (toLower(n.name) CONTAINS toLower(nodeText) OR (size(n.name) >= 2 AND toLower(nodeText) CONTAINS toLower(n.name))))`

		var query string
		if labelExpr == "" {
			query = `
				MATCH ` + nodeMatch + `-[r]-(m)
				WHERE ANY(lbl IN labels(n) WHERE lbl STARTS WITH 'ENTITY')
				AND ` + matchExpr + `
				RETURN n, r, m
				LIMIT $maxResults
			`
		} else {
			query = `
				MATCH ` + nodeMatch + `-[r]-(m)
				WHERE ` + matchExpr + `
				RETURN n, r, m
				LIMIT $maxResults
			`
		}
		params := map[string]interface{}{"nodes": nodes, "maxResults": 200}
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, fmt.Errorf("Cypher 查询失败: %w", err)
		}

		graphData := &GraphData{}
		nodeSeen := make(map[string]bool)
		for result.Next(ctx) {
			record := result.Record()
			node, _ := record.Get("n")
			rel, _ := record.Get("r")
			targetNode, _ := record.Get("m")

			nodeData := node.(neo4j.Node)
			targetNodeData := targetNode.(neo4j.Node)

			// 收集节点（去重）
			for _, n := range []neo4j.Node{nodeData, targetNodeData} {
				nameStr, _ := n.Props["name"].(string)
				if nameStr == "" {
					continue
				}
				if !nodeSeen[nameStr] {
					nodeSeen[nameStr] = true
					graphData.Node = append(graphData.Node, &GraphNode{
						Name:       nameStr,
						Chunks:     listToStrings(n.Props["chunks"]),
						Attributes: listToStrings(n.Props["attributes"]),
					})
				}
			}

			// 收集关系
			relData := rel.(neo4j.Relationship)
			sourceName, _ := nodeData.Props["name"].(string)
			targetName, _ := targetNodeData.Props["name"].(string)
			graphData.Relation = append(graphData.Relation, &GraphRelation{
				Node1: sourceName,
				Node2: targetName,
				Type:  relData.Type,
			})
		}
		return graphData, nil
	})
	if err != nil {
		log.Printf("[GraphRAG] SearchNode 失败: %v", err)
		return nil, err
	}
	return result.(*GraphData), nil
}

// listToStrings 将 Neo4j 返回的 []interface{} 转为 []string
func listToStrings(v interface{}) []string {
	if v == nil {
		return nil
	}
	list, ok := v.([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, len(list))
	for _, item := range list {
		result = append(result, fmt.Sprintf("%v", item))
	}
	return result
}

// GetGraphForVis 获取可视化用的子图数据
// 先查该 namespace 下的节点（按关系数排序取 top N），再查这些节点之间的关系
func (r *Neo4jRepository) GetGraphForVis(ctx context.Context, namespace NameSpace, limit int) (*VisGraph, error) {
	if r.driver == nil {
		return &VisGraph{}, nil
	}
	if limit <= 0 || limit > 500 {
		limit = 200
	}

	session := r.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		labelExpr := r.Label(namespace)
		if labelExpr == "" {
			return &VisGraph{}, nil
		}

		// Step 1: 获取节点及其 degree，按 degree 降序
		nodeQuery := `
			MATCH (n:` + labelExpr + `)
			OPTIONAL MATCH (n)-[r]-()
			WITH n, count(r) AS degree
			ORDER BY degree DESC
			LIMIT $limit
			RETURN n.name AS name, degree, n.chunks AS chunks
		`
		nodeResult, err := tx.Run(ctx, nodeQuery, map[string]interface{}{"limit": limit})
		if err != nil {
			return nil, fmt.Errorf("查询节点失败: %w", err)
		}

		vis := &VisGraph{}
		nodeSet := make(map[string]bool)

		for nodeResult.Next(ctx) {
			rec := nodeResult.Record()
			name, _ := rec.Get("name")
			degree, _ := rec.Get("degree")
			chunks, _ := rec.Get("chunks")

			nameStr, _ := name.(string)
			if nameStr == "" {
				continue
			}
			degreeInt, _ := degree.(int64)
			chunkCount := 0
			if cl, ok := chunks.([]interface{}); ok {
				chunkCount = len(cl)
			}

			nodeSet[nameStr] = true
			vis.Nodes = append(vis.Nodes, VisNode{
				ID:         nameStr,
				Label:      nameStr,
				Degree:     int(degreeInt),
				ChunkCount: chunkCount,
			})
		}

		if len(vis.Nodes) == 0 {
			return vis, nil
		}

		// Step 2: 获取这些节点之间的关系
		edgeQuery := `
			MATCH (n:` + labelExpr + `)-[r]-(m:` + labelExpr + `)
			WHERE n.name IN $names AND m.name IN $names AND n.name < m.name
			RETURN DISTINCT n.name AS source, m.name AS target, type(r) AS relType
			LIMIT $edgeLimit
		`
		names := make([]string, 0, len(nodeSet))
		for n := range nodeSet {
			names = append(names, n)
		}
		edgeLimit := limit * 2
		if edgeLimit > 1000 {
			edgeLimit = 1000
		}

		edgeResult, err := tx.Run(ctx, edgeQuery, map[string]interface{}{
			"names":     names,
			"edgeLimit": edgeLimit,
		})
		if err != nil {
			return nil, fmt.Errorf("查询关系失败: %w", err)
		}

		for edgeResult.Next(ctx) {
			rec := edgeResult.Record()
			source, _ := rec.Get("source")
			target, _ := rec.Get("target")
			relType, _ := rec.Get("relType")

			sourceStr, _ := source.(string)
			targetStr, _ := target.(string)
			relStr, _ := relType.(string)

			vis.Edges = append(vis.Edges, VisEdge{
				Source: sourceStr,
				Target: targetStr,
				Label:  relStr,
			})
		}

		return vis, nil
	})
	if err != nil {
		return nil, err
	}
	return result.(*VisGraph), nil
}

// EnsureIndexes 为 Neo4j 创建全文索引加速实体名称查询
// 索引使用 name 属性，利用 CONTAINS 匹配时显著提升性能
func (r *Neo4jRepository) EnsureIndexes(ctx context.Context) error {
	if r.driver == nil {
		return nil
	}
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	// 创建 name 属性索引（IF NOT EXISTS 保证幂等）
	queries := []string{
		`CREATE TEXT INDEX entity_name_text IF NOT EXISTS FOR (n:ENTITY) ON (n.name)`,
	}
	for _, q := range queries {
		if _, err := session.Run(ctx, q, nil); err != nil {
			log.Printf("[GraphRAG] 创建索引警告（可忽略）: %v", err)
			// 不同 Neo4j 版本可能不支持 TEXT INDEX，尝试普通索引
			fallback := `CREATE INDEX entity_name IF NOT EXISTS FOR (n:ENTITY) ON (n.name)`
			if _, err2 := session.Run(ctx, fallback, nil); err2 != nil {
				log.Printf("[GraphRAG] 创建备选索引也失败: %v", err2)
			}
		}
	}
	log.Println("[GraphRAG] Neo4j 索引已确保创建")
	return nil
}

// ── Neo4j 连接工厂 ──

// InitNeo4jDriver 初始化 Neo4j 驱动（带重试）
// 优先使用 Config 字段，环境变量作为备选
func InitNeo4jDriver(ctx context.Context, cfg *Config) (neo4j.DriverWithContext, error) {
	uri := cfg.Neo4jURI
	if uri == "" {
		uri = os.Getenv("NEO4J_URI")
	}
	username := cfg.Neo4jUser
	if username == "" {
		username = os.Getenv("NEO4J_USERNAME")
	}
	password := cfg.Neo4jPass
	if password == "" {
		password = os.Getenv("NEO4J_PASSWORD")
	}

	if uri == "" {
		uri = "bolt://localhost:7687"
	}
	if username == "" {
		username = "neo4j"
	}
	if password == "" {
		password = "password"
	}

	maxRetries := 5
	retryInterval := 2 * time.Second

	var driver neo4j.DriverWithContext
	var err error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		driver, err = neo4j.NewDriverWithContext(uri, neo4j.BasicAuth(username, password, ""))
		if err != nil {
			log.Printf("[GraphRAG] 创建 Neo4j 驱动失败 (尝试 %d/%d): %v", attempt, maxRetries, err)
			time.Sleep(retryInterval)
			continue
		}

		err = driver.VerifyConnectivity(ctx)
		if err == nil {
			if attempt > 1 {
				log.Printf("[GraphRAG] Neo4j 连接成功 (经过 %d 次尝试)", attempt)
			} else {
				log.Printf("[GraphRAG] Neo4j 连接成功: %s", uri)
			}
			return driver, nil
		}

		log.Printf("[GraphRAG] Neo4j 连接验证失败 (尝试 %d/%d): %v", attempt, maxRetries, err)
		driver.Close(ctx)
		time.Sleep(retryInterval)
	}

	return nil, fmt.Errorf("Neo4j 连接失败 (尝试 %d 次): %w", maxRetries, err)
}


