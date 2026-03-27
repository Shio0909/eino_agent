// build_graph_tool: 为指定知识库批量构建图谱
// 直接读取 PostgreSQL 中的 chunks，调用 GraphRAG BuildGraph
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"

	"eino_agent/internal/config"
	"eino_agent/internal/container"
	"eino_agent/internal/graphrag"
)

func main() {
	kbFlag := flag.String("kb", "3971338d-649d-43c4-91b7-12f7543b7660", "知识库 ID")
	maxFlag := flag.Int("max", 50, "最大处理 chunk 数")
	batchFlag := flag.Int("batch", 5, "每批并发 chunk 数")
	lightFlag := flag.Bool("light", false, "使用轻量模型建图（快 10x 但质量可能略降）")
	offsetFlag := flag.Int("offset", 0, "跳过前 N 个 chunks（断点续传）")
	flag.Parse()

	kbID := *kbFlag
	maxChunks := *maxFlag
	batchSize := *batchFlag

	ctx := context.Background()

	// 加载配置
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 连接 PostgreSQL
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.User, cfg.Database.Password, cfg.Database.DBName, cfg.Database.SSLMode)
	fmt.Printf("连接 PG: host=%s port=%d user=%s db=%s\n", cfg.Database.Host, cfg.Database.Port, cfg.Database.User, cfg.Database.DBName)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("连接 PG 失败: %v", err)
	}
	defer db.Close()

	// 先检查 rag_vectors 表行数 (chunks 存储在 PgVectorDB 的 rag_vectors 表)
	var totalCount int
	err = db.QueryRowContext(ctx, `SELECT count(*) FROM rag_vectors WHERE id LIKE $1`, kbID+"%").Scan(&totalCount)
	if err != nil {
		log.Fatalf("查询 rag_vectors 数量失败: %v", err)
	}
	fmt.Printf("KB=%s 在 rag_vectors 表中有 %d 行\n", kbID, totalCount)

	// 获取 chunks — rag_vectors 表存储了 id, content, metadata, embedding
	rows, err := db.QueryContext(ctx,
		`SELECT id, content FROM rag_vectors WHERE id LIKE $1 AND content IS NOT NULL AND content != '' ORDER BY id OFFSET $2 LIMIT $3`,
		kbID+"%", *offsetFlag, maxChunks)
	if err != nil {
		log.Fatalf("查询 chunks 失败: %v", err)
	}
	var chunks []*graphrag.ChunkForGraph
	for rows.Next() {
		var id, content string
		if err := rows.Scan(&id, &content); err != nil {
			continue
		}
		if len(content) > 2000 {
			content = content[:2000]
		}
		chunks = append(chunks, &graphrag.ChunkForGraph{ID: id, Content: content})
	}
	rows.Close()
	fmt.Printf("KB=%s 获取到 %d 个 chunks\n", kbID, len(chunks))

	if len(chunks) == 0 {
		fmt.Println("无 chunks，退出")
		return
	}

	// 初始化 LLM（根据 --light 标志选择模型）
	var llmCfg *config.LLMConfig
	if *lightFlag && cfg.GraphRAG.LightLLM != nil {
		llmCfg = cfg.GraphRAG.LightLLM
		fmt.Printf("使用轻量模型建图: %s (%s)\n", llmCfg.ModelID, llmCfg.BaseURL)
	} else {
		llmCfg = &cfg.LLM
		fmt.Printf("使用主模型建图: %s\n", llmCfg.ModelID)
	}
	chatModel, cleanup, err := container.NewLLMProvider(ctx, llmCfg)
	if err != nil {
		log.Fatalf("创建 LLM 失败: %v", err)
	}
	if cleanup != nil {
		defer cleanup(ctx)
	}

	// 初始化 GraphRAG
	graphCfg := &graphrag.Config{
		Enabled:     cfg.GraphRAG.Enabled,
		Neo4jURI:    cfg.GraphRAG.Neo4jURI,
		Neo4jUser:   cfg.GraphRAG.Neo4jUsername,
		Neo4jPass:   cfg.GraphRAG.Neo4jPassword,
		ExtractTemp: cfg.GraphRAG.ExtractTemperature,
	}
	svc, err := graphrag.InitService(ctx, graphCfg, chatModel)
	if err != nil {
		log.Fatalf("初始化 GraphRAG 失败: %v", err)
	}

	// 如果使用轻量模型，设置 Extractor 使用轻量模型建图
	if *lightFlag {
		svc.SetExtractorUseLightForBuild(true)
	}

	// 批量构建
	totalNodes, totalRels, totalFailed := 0, 0, 0
	for i := 0; i < len(chunks); i += batchSize {
		end := i + batchSize
		if end > len(chunks) {
			end = len(chunks)
		}
		batch := chunks[i:end]

		fmt.Printf("\n--- Batch %d (%d chunks) ---\n", i/batchSize+1, len(batch))
		start := time.Now()

		result, err := svc.BuildGraph(ctx, &graphrag.BuildGraphRequest{
			Namespace: &graphrag.NameSpace{KnowledgeBase: kbID},
			Chunks:    batch,
		})
		elapsed := time.Since(start)

		if err != nil {
			fmt.Printf("  构建失败: %v\n", err)
			continue
		}

		totalNodes += result.ProcessedNodes
		totalRels += result.ProcessedRels
		totalFailed += result.FailedChunks
		fmt.Printf("  nodes=%d rels=%d failed=%d time=%v\n",
			result.ProcessedNodes, result.ProcessedRels, result.FailedChunks, elapsed.Round(time.Second))
	}

	fmt.Printf("\n=== 构建完成 ===\n")
	fmt.Printf("总节点: %d, 总关系: %d, 失败 chunks: %d\n", totalNodes, totalRels, totalFailed)
}
