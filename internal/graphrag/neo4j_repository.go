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
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"

	"eino_agent/internal/neo4jutil"
)

const (
	// DefaultMaxHops 图增强检索默认遍历跳数
	DefaultMaxHops = 2
	// MaxAllowedHops 允许的最大遍历跳数（变长路径展开代价随跳数指数增长）
	MaxAllowedHops = 3

	searchSeedLimit   = 20
	searchResultLimit = 200
)

// Neo4jRepository Neo4j 图存储实现
type Neo4jRepository struct {
	driver     neo4j.DriverWithContext
	nodePrefix string
	maxHops    int
}

// RepositoryOption Neo4jRepository 可选配置
type RepositoryOption func(*Neo4jRepository)

// WithMaxHops 设置图增强检索的最大遍历跳数；<=0 使用默认值，超过上限时截断为 MaxAllowedHops。
func WithMaxHops(hops int) RepositoryOption {
	return func(r *Neo4jRepository) {
		r.maxHops = normalizeMaxHops(hops)
	}
}

// NewNeo4jRepository 创建 Neo4j Repository。
// 驱动由调用方拥有，Repository 不负责关闭。
func NewNeo4jRepository(driver neo4j.DriverWithContext, opts ...RepositoryOption) GraphRepository {
	r := &Neo4jRepository{driver: driver, nodePrefix: "ENTITY", maxHops: DefaultMaxHops}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func normalizeMaxHops(hops int) int {
	if hops <= 0 {
		return DefaultMaxHops
	}
	if hops > MaxAllowedHops {
		log.Printf("[GraphRAG] max_hops=%d 超过上限，截断为 %d", hops, MaxAllowedHops)
		return MaxAllowedHops
	}
	return hops
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

// Label 返回命名空间对应的标签表达式（用于 Cypher），每个标签都经过反引号转义
func (r *Neo4jRepository) Label(namespace NameSpace) string {
	labels := r.Labels(namespace)
	quoted := make([]string, len(labels))
	for i, label := range labels {
		quoted[i] = neo4jutil.QuoteIdentifier(label)
	}
	return strings.Join(quoted, ":")
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
	session := neo4jutil.WriteSession(ctx, r.driver)
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
				"props":        map[string]interface{}{"attributes": node.Attributes, "entity_type": node.Type},
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
	session := neo4jutil.WriteSession(ctx, r.driver)
	defer session.Close(ctx)

	// 内层语句以参数形式传给 apoc.periodic.iterate，避免在字符串字面量里拼接标签
	const batchDelete = `
		CALL apoc.periodic.iterate(
			$iterate,
			$action,
			{batchSize: 1000, parallel: true, params: {knowledge_id: $knowledge_id}}
		) YIELD batches, total
		RETURN total
	`
	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		for _, namespace := range namespaces {
			labelExpr := r.Label(namespace)
			if labelExpr == "" {
				// 无标签时 MATCH 会跨越所有命名空间，拒绝执行以免误删
				return nil, fmt.Errorf("删除图谱需要非空命名空间")
			}
			nodePattern := "(n:" + labelExpr + " {kg: $knowledge_id})"
			peerPattern := "(m:" + labelExpr + " {kg: $knowledge_id})"

			// 先删关系
			if _, err := tx.Run(ctx, batchDelete, map[string]interface{}{
				"iterate":      "MATCH " + nodePattern + "-[r]-" + peerPattern + " RETURN r",
				"action":       "DELETE r",
				"knowledge_id": namespace.Knowledge,
			}); err != nil {
				return nil, fmt.Errorf("删除关系失败: %w", err)
			}

			// 再删节点
			if _, err := tx.Run(ctx, batchDelete, map[string]interface{}{
				"iterate":      "MATCH " + nodePattern + " RETURN n",
				"action":       "DELETE n",
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

// buildSearchNodeQuery 构建图增强检索语句：
//  1. 用名称 CONTAINS 匹配出种子实体（最多 $maxSeeds 个）
//  2. 从种子出发做 1..maxHops 跳的无向变长遍历，路径上所有节点都必须在同一命名空间
//  3. 每个 (种子, 相关实体) 只保留最短、关系权重最高的一条路径
//  4. 按跳数升序、权重降序排序
//
// labelExpr 必须来自 Label()（已转义）；maxHops 只能是 1..MaxAllowedHops 的整数，
// 因为 Cypher 不支持参数化变长路径的上下界。
func buildSearchNodeQuery(labelExpr string, maxHops int) (string, error) {
	if maxHops < 1 || maxHops > MaxAllowedHops {
		return "", fmt.Errorf("max hops must be within 1..%d, got %d", MaxAllowedHops, maxHops)
	}
	inNamespace := func(v string) string {
		if labelExpr == "" {
			return "ANY(lbl IN labels(" + v + ") WHERE lbl STARTS WITH 'ENTITY')"
		}
		return v + ":" + labelExpr
	}
	return `
		MATCH (n)
		WHERE ` + inNamespace("n") + `
		  AND ANY(nodeText IN $nodes WHERE size(nodeText) >= 2 AND (toLower(n.name) CONTAINS toLower(nodeText) OR (size(n.name) >= 2 AND toLower(nodeText) CONTAINS toLower(n.name))))
		WITH n ORDER BY n.name ASC LIMIT $maxSeeds
		OPTIONAL MATCH p = (n)-[*1..` + strconv.Itoa(maxHops) + `]-(m)
		WHERE m <> n AND ALL(x IN nodes(p) WHERE ` + inNamespace("x") + `)
		WITH n, m, p,
		     CASE WHEN p IS NULL THEN 0 ELSE length(p) END AS hops,
		     CASE WHEN p IS NULL THEN 1.0 ELSE reduce(w = 1.0, rel IN relationships(p) | w * coalesce(toFloat(rel.weight), 1.0)) END AS weight
		ORDER BY hops ASC, weight DESC
		WITH n, m, head(collect({
		       hops: hops,
		       weight: weight,
		       rels: [rel IN coalesce(relationships(p), []) | {type: type(rel), source: startNode(rel).name, target: endNode(rel).name}]
		     })) AS best
		RETURN n {.name, .entity_type, .chunks, .attributes} AS seed,
		       CASE WHEN m IS NULL THEN null ELSE m {.name, .entity_type, .chunks, .attributes} END AS related,
		       best.hops AS hops, best.weight AS weight, best.rels AS rels
		ORDER BY hops ASC, weight DESC, seed.name ASC, related.name ASC
		LIMIT $maxResults
	`, nil
}

// SearchNode 根据实体列表在 Neo4j 中检索（支持类型约束过滤）
// 名称 CONTAINS 匹配种子实体后做有界变长遍历，返回相关实体（带跳数/权重）、关系及关联 Chunk IDs
func (r *Neo4jRepository) SearchNode(
	ctx context.Context,
	namespace NameSpace,
	entities []QueryEntity,
) (*GraphData, error) {
	if r.driver == nil {
		return nil, nil
	}

	entityNames := make([]string, len(entities))
	for i, e := range entities {
		entityNames[i] = e.Name
	}

	query, err := buildSearchNodeQuery(r.Label(namespace), normalizeMaxHops(r.maxHops))
	if err != nil {
		return nil, err
	}

	session := neo4jutil.ReadSession(ctx, r.driver)
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		res, err := tx.Run(ctx, query, map[string]interface{}{
			"nodes":      entityNames,
			"maxSeeds":   searchSeedLimit,
			"maxResults": searchResultLimit,
		})
		if err != nil {
			return nil, fmt.Errorf("Cypher 查询失败: %w", err)
		}
		acc := newSearchAccumulator()
		for res.Next(ctx) {
			rec := res.Record()
			seed, _ := rec.Get("seed")
			related, _ := rec.Get("related")
			hops, _ := rec.Get("hops")
			weight, _ := rec.Get("weight")
			rels, _ := rec.Get("rels")
			acc.add(seed, related, neo4jutil.ToInt(hops), neo4jutil.ToFloat(weight), rels)
		}
		if err := res.Err(); err != nil {
			return nil, fmt.Errorf("读取 Cypher 结果失败: %w", err)
		}
		return acc.graph(), nil
	})
	if err != nil {
		log.Printf("[GraphRAG] SearchNode 失败: %v", err)
		return nil, err
	}

	graphData := result.(*GraphData)
	graphData.Node = filterSeedsByType(graphData.Node, entities)
	return graphData, nil
}

// searchAccumulator 合并多条路径记录：同名实体保留最小跳数/最大权重，关系去重
type searchAccumulator struct {
	nodes    map[string]*GraphNode
	relSeen  map[string]bool
	relOrder []*GraphRelation
}

func newSearchAccumulator() *searchAccumulator {
	return &searchAccumulator{nodes: map[string]*GraphNode{}, relSeen: map[string]bool{}}
}

func (a *searchAccumulator) add(seed, related any, hops int, weight float64, rels any) {
	a.addNode(seed, 0, 1.0)
	if related != nil {
		a.addNode(related, hops, weight)
	}
	list, _ := rels.([]any)
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		rel := &GraphRelation{
			Node1: stringProp(m, "source"),
			Node2: stringProp(m, "target"),
			Type:  stringProp(m, "type"),
		}
		key := rel.Node1 + "\x00" + rel.Type + "\x00" + rel.Node2
		if a.relSeen[key] {
			continue
		}
		a.relSeen[key] = true
		a.relOrder = append(a.relOrder, rel)
	}
}

func (a *searchAccumulator) addNode(v any, hops int, weight float64) {
	props, ok := v.(map[string]any)
	if !ok {
		return
	}
	name := stringProp(props, "name")
	if name == "" {
		return
	}
	if weight <= 0 {
		weight = 1.0
	}
	if existing, ok := a.nodes[name]; ok {
		if hops < existing.Hops || (hops == existing.Hops && weight > existing.Weight) {
			existing.Hops = hops
			existing.Weight = weight
		}
		return
	}
	a.nodes[name] = &GraphNode{
		Name:       name,
		Type:       stringProp(props, "entity_type"),
		Chunks:     neo4jutil.ToStrings(props["chunks"]),
		Attributes: neo4jutil.ToStrings(props["attributes"]),
		Hops:       hops,
		Weight:     weight,
	}
}

func (a *searchAccumulator) graph() *GraphData {
	nodes := make([]*GraphNode, 0, len(a.nodes))
	for _, n := range a.nodes {
		nodes = append(nodes, n)
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Hops != nodes[j].Hops {
			return nodes[i].Hops < nodes[j].Hops
		}
		if nodes[i].Weight != nodes[j].Weight {
			return nodes[i].Weight > nodes[j].Weight
		}
		return nodes[i].Name < nodes[j].Name
	})
	return &GraphData{Node: nodes, Relation: a.relOrder}
}

func stringProp(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

// filterSeedsByType 按查询实体的类型约束过滤直接命中的种子实体。
// 种子只有在"名称命中的带类型查询实体全部类型不符，且没有其它查询实体认可它"时才被剔除；
// 通过关系遍历得到的邻居实体（Hops>0）不过滤。
func filterSeedsByType(nodes []*GraphNode, entities []QueryEntity) []*GraphNode {
	hasTyped := false
	for _, e := range entities {
		if isTypedEntity(e) {
			hasTyped = true
			break
		}
	}
	if !hasTyped {
		return nodes
	}
	filtered := make([]*GraphNode, 0, len(nodes))
	for _, node := range nodes {
		if node.Hops > 0 || keepSeed(node, entities) {
			filtered = append(filtered, node)
		}
	}
	return filtered
}

func isTypedEntity(e QueryEntity) bool {
	return e.Type != "" && e.Type != "Other"
}

func keepSeed(node *GraphNode, entities []QueryEntity) bool {
	nameLower := strings.ToLower(node.Name)
	conflict := false
	for _, e := range entities {
		queryLower := strings.ToLower(e.Name)
		if queryLower == "" || !(strings.Contains(nameLower, queryLower) || strings.Contains(queryLower, nameLower)) {
			continue
		}
		if !isTypedEntity(e) || node.Type == "" || node.Type == e.Type {
			return true
		}
		conflict = true
	}
	return !conflict
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

	session := neo4jutil.ReadSession(ctx, r.driver)
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
			ORDER BY degree DESC, n.name ASC
			LIMIT $limit
			RETURN n.name AS name, degree, n.chunks AS chunks
		`
		nodeResult, err := tx.Run(ctx, nodeQuery, map[string]interface{}{"limit": limit})
		if err != nil {
			return nil, fmt.Errorf("查询节点失败: %w", err)
		}

		vis := &VisGraph{}
		names := make([]string, 0, limit)
		nodeSet := make(map[string]bool)

		for nodeResult.Next(ctx) {
			rec := nodeResult.Record()
			nameStr := neo4jutil.String(rec, "name")
			if nameStr == "" || nodeSet[nameStr] {
				continue
			}
			chunks, _ := rec.Get("chunks")

			nodeSet[nameStr] = true
			names = append(names, nameStr)
			vis.Nodes = append(vis.Nodes, VisNode{
				ID:         nameStr,
				Label:      nameStr,
				Degree:     neo4jutil.Int(rec, "degree"),
				ChunkCount: len(neo4jutil.ToStrings(chunks)),
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
			vis.Edges = append(vis.Edges, VisEdge{
				Source: neo4jutil.String(rec, "source"),
				Target: neo4jutil.String(rec, "target"),
				Label:  neo4jutil.String(rec, "relType"),
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
	session := neo4jutil.WriteSession(ctx, r.driver)
	defer session.Close(ctx)

	// 创建 name 属性索引（IF NOT EXISTS 保证幂等）
	queries := []string{
		`CREATE TEXT INDEX entity_name_text IF NOT EXISTS FOR (n:ENTITY) ON (n.name)`,
	}
	for _, q := range queries {
		if err := neo4jutil.Exec(ctx, session, q, nil); err != nil {
			log.Printf("[GraphRAG] 创建索引警告（可忽略）: %v", err)
			// 不同 Neo4j 版本可能不支持 TEXT INDEX，尝试普通索引
			fallback := `CREATE INDEX entity_name IF NOT EXISTS FOR (n:ENTITY) ON (n.name)`
			if err2 := neo4jutil.Exec(ctx, session, fallback, nil); err2 != nil {
				log.Printf("[GraphRAG] 创建备选索引也失败: %v", err2)
			}
		}
	}
	log.Println("[GraphRAG] Neo4j 索引已确保创建")
	return nil
}

// ── Neo4j 连接工厂 ──

// InitNeo4jDriver 初始化 Neo4j 驱动（带重试）。
// 凭据只来自传入的 cfg：URI 与密码必填，用户名缺省为 "neo4j"。
// 调用方拥有返回的驱动，必须在退出时调用 driver.Close(ctx)。
func InitNeo4jDriver(ctx context.Context, cfg *Config) (neo4j.DriverWithContext, error) {
	if cfg == nil {
		return nil, fmt.Errorf("graphrag: config is nil")
	}
	driver, err := neo4jutil.Connect(ctx, neo4jutil.ConnectConfig{
		URI:       cfg.Neo4jURI,
		Username:  cfg.Neo4jUser,
		Password:  cfg.Neo4jPass,
		LogPrefix: "[GraphRAG]",
	})
	if err != nil {
		return nil, fmt.Errorf("graphrag: %w", err)
	}
	return driver, nil
}
