// Package graphrag - GraphRAG 核心服务
//
// 提供图谱构建(BuildGraph)和图谱管理功能
// 参考 WeKnora: internal/application/service/graph.go
package graphrag

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Service GraphRAG 服务
type Service struct {
	repository GraphRepository
	extractor  *Extractor
	chatModel  model.ChatModel
	config     *Config

	// driver 仅在 InitService 创建时由 Service 持有，Close 负责释放
	driver    neo4j.DriverWithContext
	closeOnce sync.Once
	closeErr  error
}

// NewService 创建 GraphRAG 服务。
// repo 由调用方拥有：通过 NewService 构造的 Service 在 Close 时不会关闭 repo 背后的驱动。
func NewService(cfg *Config, chatModel model.ChatModel, repo GraphRepository) *Service {
	extractor := NewExtractor(chatModel, cfg.ExtractTemp)
	return &Service{
		repository: repo,
		extractor:  extractor,
		chatModel:  chatModel,
		config:     cfg,
	}
}

// GetExtractor 获取实体抽取器（供外部创建 GraphRetriever 使用）
func (s *Service) GetExtractor() *Extractor {
	return s.extractor
}

// GetRepository 获取图谱仓库
func (s *Service) GetRepository() GraphRepository {
	return s.repository
}

// Driver 返回 Service 持有的 Neo4j 驱动（NewService 构造时为 nil）。
// 供其它组件（如代码图谱）复用同一连接池；驱动生命周期仍归 Service，调用方不得 Close。
func (s *Service) Driver() neo4j.DriverWithContext {
	if s == nil {
		return nil
	}
	return s.driver
}

// Close 关闭 Service 持有的 Neo4j 驱动，可重复调用。
func (s *Service) Close(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.closeOnce.Do(func() {
		if s.driver != nil {
			s.closeErr = s.driver.Close(ctx)
		}
	})
	return s.closeErr
}

// BuildGraphRequest 构建图谱请求
type BuildGraphRequest struct {
	Namespace *NameSpace       // 图谱命名空间
	Chunks    []*ChunkForGraph // 文档 chunks
}

// ChunkForGraph 用于图谱构建的 chunk
type ChunkForGraph struct {
	ID      string // chunk ID
	Content string // chunk 内容
}

// BuildGraphResult 构建图谱结果
type BuildGraphResult struct {
	TotalChunks    int           `json:"total_chunks"`
	ProcessedNodes int           `json:"processed_nodes"`
	ProcessedRels  int           `json:"processed_relations"`
	FailedChunks   int           `json:"failed_chunks"`
	Duration       time.Duration `json:"duration"`
}

// BuildGraph 从文档 chunks 构建知识图谱
// 参考 WeKnora: service/graph.go BuildGraph
// 流程：chunks → 并发 LLM 抽取实体/关系 → 存入 Neo4j
func (s *Service) BuildGraph(ctx context.Context, req *BuildGraphRequest) (*BuildGraphResult, error) {
	startTime := time.Now()

	if len(req.Chunks) == 0 {
		return &BuildGraphResult{}, nil
	}

	log.Printf("[GraphRAG] 开始构建图谱: %d 个 chunks, namespace=%+v", len(req.Chunks), req.Namespace)

	var (
		totalNodes int64
		totalRels  int64
		failCount  int64
	)

	// 并发抽取（控制并发数）
	concurrency := 5
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for _, chunk := range req.Chunks {
		wg.Add(1)
		sem <- struct{}{}

		go func(c *ChunkForGraph) {
			defer wg.Done()
			defer func() { <-sem }()

			// LLM 抽取实体和关系
			graphData, err := s.extractor.ExtractFromChunk(ctx, c.Content)
			if err != nil {
				log.Printf("[GraphRAG] 抽取失败 chunk=%s: %v", c.ID, err)
				atomic.AddInt64(&failCount, 1)
				return
			}

			if len(graphData.Node) == 0 {
				return
			}

			// 为每个节点关联 chunk ID
			for _, node := range graphData.Node {
				node.Chunks = append(node.Chunks, c.ID)
			}

			// 存入 Neo4j
			if err := s.repository.AddGraph(ctx, *req.Namespace, []*GraphData{graphData}); err != nil {
				log.Printf("[GraphRAG] 存图失败 chunk=%s: %v", c.ID, err)
				atomic.AddInt64(&failCount, 1)
				return
			}

			atomic.AddInt64(&totalNodes, int64(len(graphData.Node)))
			atomic.AddInt64(&totalRels, int64(len(graphData.Relation)))
		}(chunk)
	}

	wg.Wait()

	result := &BuildGraphResult{
		TotalChunks:    len(req.Chunks),
		ProcessedNodes: int(totalNodes),
		ProcessedRels:  int(totalRels),
		FailedChunks:   int(failCount),
		Duration:       time.Since(startTime),
	}

	log.Printf("[GraphRAG] 图谱构建完成: nodes=%d, relations=%d, failed=%d, duration=%v",
		result.ProcessedNodes, result.ProcessedRels, result.FailedChunks, result.Duration)

	return result, nil
}

// DeleteGraph 删除指定命名空间的图谱
func (s *Service) DeleteGraph(ctx context.Context, namespace *NameSpace) error {
	log.Printf("[GraphRAG] 删除图谱: namespace=%+v", namespace)
	return s.repository.DelGraph(ctx, []NameSpace{*namespace})
}

// Status GraphRAG 状态
type Status struct {
	Enabled   bool   `json:"enabled"`
	Neo4jURI  string `json:"neo4j_uri"`
	Connected bool   `json:"connected"`
}

// GetStatus 获取 GraphRAG 服务状态
func (s *Service) GetStatus() *Status {
	return &Status{
		Enabled:   s.config.Enabled,
		Neo4jURI:  s.config.Neo4jURI,
		Connected: s.repository != nil,
	}
}

// CreateGraphRetriever 创建图谱检索器
// 返回的 Retriever 可注入 container.CompositeRetriever 作为图谱信号源
func (s *Service) CreateGraphRetriever(namespace *NameSpace, topK int) *GraphRetriever {
	return NewGraphRetriever(&GraphRetrieverConfig{
		Extractor:  s.extractor,
		Repository: s.repository,
		Namespace:  namespace,
		TopK:       topK,
	})
}

// CreateScopedGraphRetriever 按知识库 ID 创建作用域图谱检索器
// 实现 container.GraphRetrieverFactory 接口
func (s *Service) CreateScopedGraphRetriever(knowledgeBaseID string, topK int) retriever.Retriever {
	ns := &NameSpace{KnowledgeBase: knowledgeBaseID}
	return s.CreateGraphRetriever(ns, topK)
}

// GetGraphForVis 获取可视化用的图数据
func (s *Service) GetGraphForVis(ctx context.Context, kbID string, limit int) (*VisGraph, error) {
	ns := NameSpace{KnowledgeBase: kbID}
	return s.repository.GetGraphForVis(ctx, ns, limit)
}

// SetLightModel 为实体抽取器设置轻量模型
func (s *Service) SetLightModel(m model.ChatModel) {
	s.extractor.SetLightModel(m)
}

// SetExtractorUseLightForBuild 设置抽取器是否使用轻量模型建图
func (s *Service) SetExtractorUseLightForBuild(use bool) {
	s.extractor.SetUseLightForBuild(use)
}

// InitService 初始化 GraphRAG 服务（工厂方法）
// 按传入配置创建 Neo4j 驱动与完整的 GraphRAG 服务；返回的 Service 拥有该驱动，调用方必须调用 Close。
func InitService(ctx context.Context, cfg *Config, chatModel model.ChatModel) (*Service, error) {
	if cfg == nil || !cfg.Enabled {
		return nil, fmt.Errorf("GraphRAG 未启用")
	}

	// 初始化 Neo4j
	driver, err := InitNeo4jDriver(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("初始化 Neo4j 失败: %w", err)
	}

	repo := NewNeo4jRepository(driver, WithMaxHops(cfg.MaxHops))
	svc := NewService(cfg, chatModel, repo)
	svc.driver = driver

	// 确保 Neo4j 索引存在（加速实体名称查询）
	if neoRepo, ok := repo.(*Neo4jRepository); ok {
		if err := neoRepo.EnsureIndexes(ctx); err != nil {
			log.Printf("[GraphRAG] 索引创建警告: %v", err)
		}
	}

	log.Println("[GraphRAG] 服务初始化完成")
	return svc, nil
}
