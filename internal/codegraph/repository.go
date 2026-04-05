package codegraph

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// CodeGraphRepository 代码知识图谱存储接口
type CodeGraphRepository interface {
	// UpsertFile 写入文件节点
	UpsertFile(ctx context.Context, repo string, filePath string, hash string) error
	// UpsertEntities 批量写入代码实体（函数/类/方法）
	UpsertEntities(ctx context.Context, repo string, entities []CodeEntity) error
	// UpsertRelations 批量写入代码关系
	UpsertRelations(ctx context.Context, repo string, relations []CodeRelation) error
	// DeleteFileGraph 删除某文件的所有图数据（增量更新时先删后建）
	DeleteFileGraph(ctx context.Context, repo string, filePath string) error
	// DeleteRepoGraph 删除整个仓库的图数据
	DeleteRepoGraph(ctx context.Context, repo string) error

	// FindCallers 查找调用指定函数的调用者
	FindCallers(ctx context.Context, repo string, funcName string, depth int) ([]CodeRelation, error)
	// FindCallees 查找指定函数调用的目标
	FindCallees(ctx context.Context, repo string, funcName string, depth int) ([]CodeRelation, error)
	// FindDefinition 查找符号定义
	FindDefinition(ctx context.Context, repo string, name string) ([]CodeEntity, error)
	// GetFileStructure 获取文件结构（定义了哪些实体）
	GetFileStructure(ctx context.Context, repo string, filePath string) ([]CodeEntity, error)
	// SearchSymbol 搜索符号名称（模糊匹配）
	SearchSymbol(ctx context.Context, repo string, pattern string, limit int) ([]CodeEntity, error)
	// GetRepoOverview 获取仓库图谱概览
	GetRepoOverview(ctx context.Context, repo string) (*RepoOverview, error)
}

// RepoOverview 仓库图谱概览统计
type RepoOverview struct {
	Repo       string         `json:"repo"`
	FileCount  int            `json:"file_count"`
	EntityCount int           `json:"entity_count"`
	RelationCount int         `json:"relation_count"`
	TypeCounts map[string]int `json:"type_counts"`
}

// Neo4jCodeGraphRepo Neo4j 实现
type Neo4jCodeGraphRepo struct {
	driver neo4j.DriverWithContext
}

// NewNeo4jCodeGraphRepo 创建 Neo4j 代码图谱存储
func NewNeo4jCodeGraphRepo(driver neo4j.DriverWithContext) CodeGraphRepository {
	return &Neo4jCodeGraphRepo{driver: driver}
}

// label 为节点添加仓库标签前缀，与文档图谱隔离
func codeLabel(repo string, entityType EntityType) string {
	// 使用 CODE_ 前缀 + 仓库名避免与文档图谱冲突
	safeRepo := strings.ReplaceAll(repo, "-", "_")
	safeRepo = strings.ReplaceAll(safeRepo, ".", "_")
	return fmt.Sprintf("CODE_%s_%s", safeRepo, entityType)
}

func codeFileLabel(repo string) string {
	safeRepo := strings.ReplaceAll(repo, "-", "_")
	safeRepo = strings.ReplaceAll(safeRepo, ".", "_")
	return fmt.Sprintf("CODE_%s_File", safeRepo)
}

func (r *Neo4jCodeGraphRepo) UpsertFile(ctx context.Context, repo string, filePath string, hash string) error {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	label := codeFileLabel(repo)
	cypher := fmt.Sprintf(`
		MERGE (f:%s {path: $path})
		SET f.hash = $hash, f.repo = $repo, f.updated_at = datetime()
	`, label)

	_, err := session.Run(ctx, cypher, map[string]any{
		"path": filePath,
		"hash": hash,
		"repo": repo,
	})
	return err
}

func (r *Neo4jCodeGraphRepo) UpsertEntities(ctx context.Context, repo string, entities []CodeEntity) error {
	if len(entities) == 0 {
		return nil
	}

	session := r.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	for _, e := range entities {
		label := codeLabel(repo, e.Type)
		cypher := fmt.Sprintf(`
			MERGE (n:%s {qualified_name: $qname})
			SET n.name = $name,
				n.file_path = $filePath,
				n.line_start = $lineStart,
				n.line_end = $lineEnd,
				n.params = $params,
				n.return_type = $returnType,
				n.decorators = $decorators,
				n.class_name = $className,
				n.repo = $repo
		`, label)

		_, err := session.Run(ctx, cypher, map[string]any{
			"qname":      e.QualifiedName,
			"name":       e.Name,
			"filePath":   e.FilePath,
			"lineStart":  e.LineStart,
			"lineEnd":    e.LineEnd,
			"params":     e.Params,
			"returnType": e.ReturnType,
			"decorators": e.Decorators,
			"className":  e.ClassName,
			"repo":       repo,
		})
		if err != nil {
			log.Printf("[codegraph] upsert entity %s error: %v", e.QualifiedName, err)
		}

		// 创建 File → DEFINES → Entity 关系
		fileLabel := codeFileLabel(repo)
		relCypher := fmt.Sprintf(`
			MATCH (f:%s {path: $filePath})
			MATCH (n:%s {qualified_name: $qname})
			MERGE (f)-[:DEFINES]->(n)
		`, fileLabel, label)
		_, _ = session.Run(ctx, relCypher, map[string]any{
			"filePath": e.FilePath,
			"qname":    e.QualifiedName,
		})

		// Method 属于 Class 的 CONTAINS 关系
		if e.Type == EntityMethod && e.ClassName != "" {
			classLabel := codeLabel(repo, EntityClass)
			containsCypher := fmt.Sprintf(`
				MATCH (c:%s {qualified_name: $className})
				MATCH (m:%s {qualified_name: $qname})
				MERGE (c)-[:CONTAINS]->(m)
			`, classLabel, label)
			_, _ = session.Run(ctx, containsCypher, map[string]any{
				"className": e.ClassName,
				"qname":     e.QualifiedName,
			})
		}
	}

	return nil
}

func (r *Neo4jCodeGraphRepo) UpsertRelations(ctx context.Context, repo string, relations []CodeRelation) error {
	if len(relations) == 0 {
		return nil
	}

	session := r.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	for _, rel := range relations {
		switch rel.Type {
		case RelCalls:
			// 尝试匹配已知实体
			cypher := fmt.Sprintf(`
				MATCH (a {qualified_name: $source, repo: $repo})
				MATCH (b {qualified_name: $target, repo: $repo})
				MERGE (a)-[:CALLS]->(b)
			`)
			_, _ = session.Run(ctx, cypher, map[string]any{
				"source": rel.Source,
				"target": rel.Target,
				"repo":   repo,
			})

		case RelInherits:
			classLabel := codeLabel(repo, EntityClass)
			cypher := fmt.Sprintf(`
				MATCH (a:%s {qualified_name: $source})
				MATCH (b:%s {qualified_name: $target})
				MERGE (a)-[:INHERITS]->(b)
			`, classLabel, classLabel)
			_, _ = session.Run(ctx, cypher, map[string]any{
				"source": rel.Source,
				"target": rel.Target,
			})

		case RelImports:
			fileLabel := codeFileLabel(repo)
			cypher := fmt.Sprintf(`
				MATCH (f:%s {path: $source})
				MERGE (m:CODE_Module {name: $target, repo: $repo})
				MERGE (f)-[:IMPORTS]->(m)
			`, fileLabel)
			_, _ = session.Run(ctx, cypher, map[string]any{
				"source": rel.Source,
				"target": rel.Target,
				"repo":   repo,
			})
		}
	}

	return nil
}

func (r *Neo4jCodeGraphRepo) DeleteFileGraph(ctx context.Context, repo string, filePath string) error {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	// 删除文件定义的所有实体及其关系，最后删除文件节点
	fileLabel := codeFileLabel(repo)
	cypher := fmt.Sprintf(`
		MATCH (f:%s {path: $path})-[:DEFINES]->(n)
		DETACH DELETE n
	`, fileLabel)
	_, err := session.Run(ctx, cypher, map[string]any{"path": filePath})
	if err != nil {
		return err
	}

	// 删除文件节点本身
	cypher = fmt.Sprintf(`MATCH (f:%s {path: $path}) DETACH DELETE f`, fileLabel)
	_, err = session.Run(ctx, cypher, map[string]any{"path": filePath})
	return err
}

func (r *Neo4jCodeGraphRepo) DeleteRepoGraph(ctx context.Context, repo string) error {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	// 删除所有带有该 repo 标记的节点
	cypher := `MATCH (n {repo: $repo}) DETACH DELETE n`
	_, err := session.Run(ctx, cypher, map[string]any{"repo": repo})
	return err
}

// ── 查询方法 ──

func (r *Neo4jCodeGraphRepo) FindCallers(ctx context.Context, repo string, funcName string, depth int) ([]CodeRelation, error) {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	if depth <= 0 || depth > 5 {
		depth = 2
	}

	cypher := fmt.Sprintf(`
		MATCH (caller {repo: $repo})-[:CALLS*1..%d]->(target {repo: $repo})
		WHERE target.qualified_name CONTAINS $name OR target.name CONTAINS $name
		RETURN DISTINCT caller.qualified_name AS source, target.qualified_name AS target
		LIMIT 50
	`, depth)

	result, err := session.Run(ctx, cypher, map[string]any{
		"repo": repo,
		"name": funcName,
	})
	if err != nil {
		return nil, err
	}

	var relations []CodeRelation
	for result.Next(ctx) {
		record := result.Record()
		source, _ := record.Get("source")
		target, _ := record.Get("target")
		relations = append(relations, CodeRelation{
			Type:   RelCalls,
			Source: fmt.Sprintf("%v", source),
			Target: fmt.Sprintf("%v", target),
		})
	}
	return relations, nil
}

func (r *Neo4jCodeGraphRepo) FindCallees(ctx context.Context, repo string, funcName string, depth int) ([]CodeRelation, error) {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	if depth <= 0 || depth > 5 {
		depth = 2
	}

	cypher := fmt.Sprintf(`
		MATCH (source {repo: $repo})-[:CALLS*1..%d]->(callee {repo: $repo})
		WHERE source.qualified_name CONTAINS $name OR source.name CONTAINS $name
		RETURN DISTINCT source.qualified_name AS source, callee.qualified_name AS target
		LIMIT 50
	`, depth)

	result, err := session.Run(ctx, cypher, map[string]any{
		"repo": repo,
		"name": funcName,
	})
	if err != nil {
		return nil, err
	}

	var relations []CodeRelation
	for result.Next(ctx) {
		record := result.Record()
		source, _ := record.Get("source")
		target, _ := record.Get("target")
		relations = append(relations, CodeRelation{
			Type:   RelCalls,
			Source: fmt.Sprintf("%v", source),
			Target: fmt.Sprintf("%v", target),
		})
	}
	return relations, nil
}

func (r *Neo4jCodeGraphRepo) FindDefinition(ctx context.Context, repo string, name string) ([]CodeEntity, error) {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	cypher := `
		MATCH (n {repo: $repo})
		WHERE (n.qualified_name CONTAINS $name OR n.name = $name)
		  AND n.file_path IS NOT NULL
		RETURN n.qualified_name AS qname, n.name AS name, n.file_path AS filePath,
			   n.line_start AS lineStart, n.line_end AS lineEnd,
			   n.params AS params, n.class_name AS className
		LIMIT 20
	`

	result, err := session.Run(ctx, cypher, map[string]any{
		"repo": repo,
		"name": name,
	})
	if err != nil {
		return nil, err
	}

	var entities []CodeEntity
	for result.Next(ctx) {
		r := result.Record()
		e := CodeEntity{
			QualifiedName: getStr(r, "qname"),
			Name:          getStr(r, "name"),
			FilePath:      getStr(r, "filePath"),
			LineStart:     getInt(r, "lineStart"),
			LineEnd:       getInt(r, "lineEnd"),
			Params:        getStr(r, "params"),
			ClassName:     getStr(r, "className"),
		}
		entities = append(entities, e)
	}
	return entities, nil
}

func (r *Neo4jCodeGraphRepo) GetFileStructure(ctx context.Context, repo string, filePath string) ([]CodeEntity, error) {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	fileLabel := codeFileLabel(repo)
	cypher := fmt.Sprintf(`
		MATCH (f:%s {path: $path})-[:DEFINES]->(n)
		RETURN n.qualified_name AS qname, n.name AS name,
			   n.line_start AS lineStart, n.line_end AS lineEnd,
			   n.params AS params, n.class_name AS className
		ORDER BY n.line_start
	`, fileLabel)

	result, err := session.Run(ctx, cypher, map[string]any{"path": filePath})
	if err != nil {
		return nil, err
	}

	var entities []CodeEntity
	for result.Next(ctx) {
		r := result.Record()
		entities = append(entities, CodeEntity{
			QualifiedName: getStr(r, "qname"),
			Name:          getStr(r, "name"),
			LineStart:     getInt(r, "lineStart"),
			LineEnd:       getInt(r, "lineEnd"),
			Params:        getStr(r, "params"),
			ClassName:     getStr(r, "className"),
		})
	}
	return entities, nil
}

func (r *Neo4jCodeGraphRepo) SearchSymbol(ctx context.Context, repo string, pattern string, limit int) ([]CodeEntity, error) {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	if limit <= 0 || limit > 50 {
		limit = 20
	}

	cypher := `
		MATCH (n {repo: $repo})
		WHERE n.name IS NOT NULL AND toLower(n.name) CONTAINS toLower($pattern)
		  AND n.file_path IS NOT NULL
		RETURN n.qualified_name AS qname, n.name AS name, n.file_path AS filePath,
			   n.line_start AS lineStart, n.line_end AS lineEnd
		LIMIT $limit
	`

	result, err := session.Run(ctx, cypher, map[string]any{
		"repo":    repo,
		"pattern": pattern,
		"limit":   limit,
	})
	if err != nil {
		return nil, err
	}

	var entities []CodeEntity
	for result.Next(ctx) {
		r := result.Record()
		entities = append(entities, CodeEntity{
			QualifiedName: getStr(r, "qname"),
			Name:          getStr(r, "name"),
			FilePath:      getStr(r, "filePath"),
			LineStart:     getInt(r, "lineStart"),
			LineEnd:       getInt(r, "lineEnd"),
		})
	}
	return entities, nil
}

func (r *Neo4jCodeGraphRepo) GetRepoOverview(ctx context.Context, repo string) (*RepoOverview, error) {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	overview := &RepoOverview{
		Repo:       repo,
		TypeCounts: make(map[string]int),
	}

	// 文件数
	fileLabel := codeFileLabel(repo)
	cypher := fmt.Sprintf(`MATCH (f:%s) RETURN count(f) AS cnt`, fileLabel)
	result, err := session.Run(ctx, cypher, nil)
	if err == nil && result.Next(ctx) {
		overview.FileCount = getInt(result.Record(), "cnt")
	}

	// 各类型实体数
	for _, et := range []EntityType{EntityFunction, EntityClass, EntityMethod} {
		label := codeLabel(repo, et)
		cypher = fmt.Sprintf(`MATCH (n:%s) RETURN count(n) AS cnt`, label)
		result, err = session.Run(ctx, cypher, nil)
		if err == nil && result.Next(ctx) {
			cnt := getInt(result.Record(), "cnt")
			overview.TypeCounts[string(et)] = cnt
			overview.EntityCount += cnt
		}
	}

	// 关系数
	cypher = `MATCH ({repo: $repo})-[r]->({repo: $repo}) RETURN count(r) AS cnt`
	result, err = session.Run(ctx, cypher, map[string]any{"repo": repo})
	if err == nil && result.Next(ctx) {
		overview.RelationCount = getInt(result.Record(), "cnt")
	}

	return overview, nil
}

// ── helpers ──

func getStr(record *neo4j.Record, key string) string {
	val, ok := record.Get(key)
	if !ok || val == nil {
		return ""
	}
	return fmt.Sprintf("%v", val)
}

func getInt(record *neo4j.Record, key string) int {
	val, ok := record.Get(key)
	if !ok || val == nil {
		return 0
	}
	switch v := val.(type) {
	case int64:
		return int(v)
	case int:
		return v
	case float64:
		return int(v)
	default:
		return 0
	}
}
