// Eino RAG 服务入口
//
// 【Eino 特点】基于字节跳动 Eino 框架构建的 RAG 服务
//
// @title Eino RAG Agent API
// @version 1.0.0
// @description 基于字节跳动 Eino 框架的智能知识库问答系统。支持 Pipeline RAG、ReAct Agent、Agentic RAG 三种模式。
// @host localhost:8080
// @BasePath /api/v1
// @schemes http
// @contact.name Eino RAG Agent
// @license.name Apache 2.0
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	einoembedding "github.com/cloudwego/eino/components/embedding"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "eino_agent/docs" // swagger docs
	cachepkg "eino_agent/internal/cache"
	"eino_agent/internal/codegraph"
	"eino_agent/internal/config"
	"eino_agent/internal/container"
	"eino_agent/internal/database/postgres"
	"eino_agent/internal/docreader"
	"eino_agent/internal/document"
	"eino_agent/internal/graphrag"
	"eino_agent/internal/handler"
	"eino_agent/internal/importqueue"
	"eino_agent/internal/logger"
	"eino_agent/internal/metrics"
	mcpmanager "eino_agent/internal/mcp"
	"eino_agent/internal/rediscache"
	"eino_agent/internal/service"
	"eino_agent/internal/tool"
	"eino_agent/internal/wiki"
	"eino_agent/internal/database/repository"
)

func main() {
	// 命令行参数
	configPath := flag.String("config", "configs/config.yaml", "配置文件路径")
	loadDocs := flag.Bool("load-docs", false, "启动时加载文档")
	migrateDB := flag.Bool("migrate", false, "运行数据库迁移")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 初始化结构化日志（slog）
	logger.Init(cfg.Server.Mode)

	// 安全校验：release 模式下禁止使用默认 JWT 密钥
	if cfg.Auth.Enabled && cfg.Auth.JWTSecret == "change-me-in-production" && cfg.Server.Mode == "release" {
		log.Fatal("[Config] JWT 密钥未配置！请在配置文件中设置 auth.jwt_secret")
	}
	if cfg.Auth.Enabled && cfg.Auth.AdminPassword == "" && cfg.Server.Mode == "release" {
		log.Fatal("[Config] 管理员密码未配置！请设置 auth.admin_password 或 ADMIN_PASSWORD 环境变量")
	}

	log.Println("========================================")
	log.Println("    Eino RAG Service - 知识库问答系统    ")
	log.Println("    Powered by Eino + Gin + PostgreSQL  ")
	log.Println("========================================")

	// 【Eino 特点】创建依赖注入容器
	log.Println("[Container] 初始化依赖注入容器...")
	c := container.New(cfg)
	defer func() {
		log.Println("[Container] 清理资源...")
		if err := c.Cleanup(ctx); err != nil {
			log.Printf("[Container] 清理资源失败: %v", err)
		}
	}()

	// ========================================
	// 初始化 Redis（可选，失败时降级）
	// ========================================
	log.Println("[Redis] 初始化 Redis 客户端...")
	redisClient, redisErr := rediscache.NewClient(ctx, cfg.Redis)
	if redisErr != nil {
		log.Printf("[Redis] 初始化失败（将降级为无缓存模式）: %v", redisErr)
		redisClient = rediscache.NewFallbackClient(cfg.Redis, redisErr)
	} else {
		status := redisClient.Status(ctx)
		log.Printf("[Redis] 状态: mode=%s addr=%s", status.Mode, status.Addr)
	}
	defer func() {
		if redisClient != nil {
			if err := redisClient.Close(); err != nil {
				log.Printf("[Redis] 关闭失败: %v", err)
			}
		}
	}()

	// ========================================
	// 初始化数据库 (可选)
	// ========================================
	var db *postgres.DB
	if cfg.Database.Host != "" {
		log.Println("[Database] 连接 PostgreSQL...")
		dbCfg := &postgres.Config{
			Host:     cfg.Database.Host,
			Port:     cfg.Database.Port,
			User:     cfg.Database.User,
			Password: cfg.Database.Password,
			Database: cfg.Database.DBName,
			SSLMode:  cfg.Database.SSLMode,
		}
		// 带重试的数据库连接（最多重试 5 次，每次间隔递增）
		for attempt := 1; attempt <= 5; attempt++ {
			db, err = postgres.New(ctx, dbCfg)
			if err == nil {
				log.Printf("[Database] 连接成功: %s:%d/%s", cfg.Database.Host, cfg.Database.Port, cfg.Database.DBName)
				break
			}
			if attempt < 5 {
				wait := time.Duration(attempt) * 2 * time.Second
				log.Printf("[Database] 连接失败（第 %d/5 次），%v 后重试: %v", attempt, wait, err)
				time.Sleep(wait)
			} else {
				log.Printf("[Database] 连接失败（将使用内存存储）: %v", err)
			}
		}
		if db != nil {
			defer db.Close()
		}

		// 运行迁移
		if db != nil && *migrateDB {
			log.Println("[Database] 跳过自动迁移，请手动运行: psql -f migrations/000001_init.up.sql")
		}
	}

	// ========================================
	// 初始化 DocReader (可选)
	// ========================================
	var docReaderCli *docreader.Client
	if cfg.DocReader.Enabled && (cfg.DocReader.Endpoint != "" || cfg.DocReader.MinerUEndpoint != "") {
		log.Printf("[DocReader] 模式: %s", cfg.DocReader.Mode)
		docReaderCfg := &docreader.Config{
			Mode:             cfg.DocReader.Mode,
			Endpoint:         cfg.DocReader.Endpoint,
			MinerUEndpoint:   cfg.DocReader.MinerUEndpoint,
			Timeout:          5 * time.Minute,
			MaxFileSize:      cfg.DocReader.MaxFileSize * 1024 * 1024,
			ChunkSize:        cfg.RAG.ChunkSize,
			ChunkOverlap:     cfg.RAG.ChunkOverlap,
			EnableMultimodal: cfg.DocReader.EnableMultimodal,
			VLMBaseURL:       cfg.DocReader.VLMBaseURL,
			VLMAPIKey:        cfg.DocReader.VLMAPIKey,
			VLMModel:         cfg.DocReader.VLMModel,
			MinIOEndpoint:    cfg.DocReader.MinIOEndpoint,
			MinIOAccessKey:   cfg.DocReader.MinIOAccessKey,
			MinIOSecretKey:   cfg.DocReader.MinIOSecretKey,
			MinIOBucket:      cfg.DocReader.MinIOBucket,
		}
		docReaderCli, err = docreader.NewClient(docReaderCfg)
		if err != nil {
			log.Printf("[DocReader] 连接失败（将使用本地解析）: %v", err)
		} else {
			log.Println("[DocReader] 连接成功")
			defer docReaderCli.Close()
		}
	}

	// ========================================
	// 初始化 AI 组件
	// ========================================

	// 【Eino 特点】初始化 LLM 组件
	log.Println("[LLM] 初始化 Eino ChatModel...")
	chatModel, err := c.GetChatModel(ctx)
	if err != nil {
		log.Fatalf("[LLM] 初始化失败: %v", err)
	}
	log.Printf("[LLM] Provider: %s, Model: %s", cfg.LLM.Provider, cfg.LLM.ModelID)

	// 【Eino 特点】初始化 Embedding 组件
	log.Println("[Embedding] 初始化 Embedding 提供者...")
	embedding, err := c.GetEmbedding(ctx)
	if err != nil {
		log.Fatalf("[Embedding] 初始化失败: %v", err)
	}
	log.Printf("[Embedding] Provider: %s, Model: %s, Dimensions: %d",
		cfg.Embedding.Provider, cfg.Embedding.ModelID, c.GetEmbeddingDimensions())

	// 【Eino 特点】初始化向量数据库
	log.Println("[VectorDB] 初始化向量数据库...")
	vectorDB, err := c.GetVectorDB(ctx)
	if err != nil {
		log.Fatalf("[VectorDB] 初始化失败: %v", err)
	}
	log.Println("[VectorDB] 初始化完成")

	// 【Eino 特点】初始化 Reranker（可选）
	var reranker container.RerankerProvider
	if cfg.Reranker.Enabled {
		log.Println("[Reranker] 初始化 Reranker...")
		reranker, err = c.GetReranker(ctx)
		if err != nil {
			log.Printf("[Reranker] 初始化失败（将跳过重排序）: %v", err)
		} else {
			log.Printf("[Reranker] Provider: %s, Model: %s", cfg.Reranker.Provider, cfg.Reranker.ModelID)
		}
	}

	// 加载文档（如果指定）
	if *loadDocs && cfg.RAG.DocumentsPath != "" {
		log.Printf("[Documents] 加载文档目录: %s", cfg.RAG.DocumentsPath)
		if err := loadDocuments(ctx, cfg, embedding, vectorDB); err != nil {
			log.Printf("[Documents] 加载文档失败: %v", err)
		}
	}

	// ========================================
	// 获取 Eino 原生接口并注入到服务层
	// ========================================

	// ChatModel 与 Retriever 已在上面完成初始化

	// ========================================
	// 创建服务和处理器
	// ========================================
	log.Println("[Service] 初始化聊天服务...")
	chatService, err := service.NewChatService(cfg)
	if err != nil {
		log.Fatalf("[Service] 创建聊天服务失败: %v", err)
	}
	sessionCache := cachepkg.NewNoopSessionCache()
	retrievalCache := cachepkg.NewNoopRetrievalCache()
	importStateStore := cachepkg.NewNoopImportStateStore()
	if redisClient != nil {
		sessionCache = rediscache.NewSessionCache(redisClient)
		retrievalCache = rediscache.NewRetrievalCache(redisClient)
		importStateStore = rediscache.NewImportStateStore(redisClient)
	}

	// 注入 LLM 审计日志仓储（需要 DB）
	if db != nil {
		chatService.SetAuditRepo(repository.NewLLMAuditRepository(db))
		log.Println("[Audit] LLM 审计日志已启用")
	}
	c.SetRetrievalCache(retrievalCache)
	log.Println("[Retriever] 初始化检索器...")
	einoRetriever, err := c.GetRetriever(ctx)
	if err != nil {
		log.Fatalf("[Retriever] 初始化失败: %v", err)
	}
	log.Println("[Retriever] 初始化完成")
	chatService.SetSessionCache(sessionCache)

	// 注入 Reranker（如果启用）
	if reranker != nil {
		chatService.SetReranker(service.NewRerankerAdapter(reranker))
		log.Println("[Reranker] 已注入到 Pipeline")
	}

	// ========================================
	// 初始化 MCP（可选）
	// ========================================
	var mcpMgr *mcpmanager.Manager
	if cfg.MCP.Enabled && len(cfg.MCP.Servers) > 0 {
		log.Println("[MCP] 初始化 MCP 客户端...")
		mcpMgr = mcpmanager.NewManager(&cfg.MCP)
		if err := mcpMgr.Init(ctx); err != nil {
			log.Printf("[MCP] 初始化失败: %v", err)
		} else {
			defer mcpMgr.Close()
			mcpTools := mcpMgr.GetTools()
			if len(mcpTools) > 0 {
				chatService.SetMCPTools(mcpTools)
				log.Printf("[MCP] 已注册 %d 个远程工具", len(mcpTools))
			}
		}
	}

	// ========================================
	// 初始化异步导入队列（可选）
	// ========================================
	var importQueue importqueue.Queue
	if cfg.ImportQueue.Enabled {
		if cfg.ImportQueue.Provider != "rabbitmq" {
			log.Fatalf("[ImportQueue] 不支持的 provider: %s", cfg.ImportQueue.Provider)
		}

		log.Printf("[ImportQueue] 连接 RabbitMQ: %s", cfg.ImportQueue.URL)
		importQueue, err = importqueue.NewRabbitMQQueue(cfg.ImportQueue)
		if err != nil {
			log.Fatalf("[ImportQueue] 初始化失败: %v", err)
		}
		defer importQueue.Close()
	}

	// 注入 Eino 原生组件（使用统一检索器 — 延后到 UnifiedRetriever 创建之后）

	// ========================================
	// 初始化 GraphRAG（可选）
	// ========================================
	var graphRAGService *graphrag.Service
	if cfg.GraphRAG.Enabled {
		log.Println("[GraphRAG] 初始化 GraphRAG 服务...")
		graphRAGCfg := &graphrag.Config{
			Enabled:     cfg.GraphRAG.Enabled,
			Neo4jURI:    cfg.GraphRAG.Neo4jURI,
			Neo4jUser:   cfg.GraphRAG.Neo4jUsername,
			Neo4jPass:   cfg.GraphRAG.Neo4jPassword,
			ExtractTemp: cfg.GraphRAG.ExtractTemperature,
		}
		graphRAGService, err = graphrag.InitService(ctx, graphRAGCfg, chatModel)
		if err != nil {
			log.Printf("[GraphRAG] 初始化失败（将跳过图谱检索）: %v", err)
		} else {
			log.Println("[GraphRAG] 服务初始化完成")
			// 将图谱检索器注入到 CompositeRetriever
			if cr, ok := einoRetriever.(*container.CompositeRetriever); ok {
				// 使用默认命名空间（查询时会通过 WithKnowledgeBaseScope 切换）
				gr := graphRAGService.CreateGraphRetriever(&graphrag.NameSpace{}, 10)
				cr.SetGraphRetriever(gr)
				log.Println("[GraphRAG] 图谱检索器已注入 Retriever")
			}
		}
	}

	// 注入 Wiki 检索器（Wiki 模式 KB 使用 LLM 编译 + wiki 页面检索）
	var wikiCompiler *wiki.Compiler
	var wikiRetriever *container.WikiRetriever
	if db != nil {
		wikiRepo := repository.NewWikiPageRepository(db)

		// 创建 Wiki 检索器
		wikiRetriever = container.NewWikiRetriever(wikiRepo, cfg.RAG.TopK)
		log.Println("[WikiRetriever] Wiki 检索器已创建")

		// 创建 Wiki 编译器（后续注入到 Handler）
		wikiCompiler = wiki.NewCompiler(chatModel, wikiRepo)
		log.Println("[WikiCompiler] Wiki 编译器已创建")
	}

	// 创建统一检索器（聚合 vector + wiki）
	var compositeRetriever *container.CompositeRetriever
	if cr, ok := einoRetriever.(*container.CompositeRetriever); ok {
		compositeRetriever = cr
	}
	var kbRepo repository.KnowledgeBaseRepository
	if db != nil {
		kbRepo = repository.NewKnowledgeBaseRepository(db)
	}
	unifiedRetriever := container.NewUnifiedRetriever(compositeRetriever, wikiRetriever, kbRepo, cfg.RAG.TopK)
	log.Println("[UnifiedRetriever] 统一检索器已创建（vector + wiki）")

	// 注入 Eino 原生组件（使用统一检索器）
	if err := chatService.InitWithComponents(chatModel, unifiedRetriever); err != nil {
		log.Fatalf("[Service] 注入组件失败: %v", err)
	}
	log.Println("[Service] 聊天服务初始化完成（Pipeline + Agent）")

	// 创建 HTTP 处理器
	apiHandler := handler.NewHandler(cfg, *configPath, chatService, embedding, vectorDB, docReaderCli, db, importQueue)
	apiHandler.SetMCPManager(mcpMgr)
	apiHandler.SetRedisClient(redisClient)
	apiHandler.SetSessionCache(sessionCache)
	apiHandler.SetRetrievalCache(retrievalCache)
	apiHandler.SetImportStateStore(importStateStore)
	if graphRAGService != nil {
		apiHandler.SetGraphRAGService(graphRAGService)
	}
	if wikiCompiler != nil {
		apiHandler.SetWikiCompiler(wikiCompiler)
	}

	// 初始化代码知识图谱（复用 GraphRAG 的 Neo4j 连接）
	var codeGraphRepo codegraph.CodeGraphRepository
	if cfg.Agent.EnableCodeGraph && cfg.GraphRAG.Enabled {
		log.Println("[CodeGraph] 初始化代码知识图谱...")
		codeGraphCfg := &graphrag.Config{
			Enabled:  true,
			Neo4jURI: cfg.GraphRAG.Neo4jURI,
			Neo4jUser: cfg.GraphRAG.Neo4jUsername,
			Neo4jPass: cfg.GraphRAG.Neo4jPassword,
		}
		neo4jDriver, err := graphrag.InitNeo4jDriver(ctx, codeGraphCfg)
		if err != nil {
			log.Printf("[CodeGraph] Neo4j 连接失败: %v", err)
		} else {
			codeGraphRepo = codegraph.NewNeo4jCodeGraphRepo(neo4jDriver)
			reposDir := cfg.Agent.CodeSearchReposDir
			if reposDir == "" {
				reposDir = "data/test_repos"
			}
			codeIndexer := codegraph.NewIndexer(codeGraphRepo, reposDir)
			chatService.SetCodeGraph(codeGraphRepo, codeIndexer)
			apiHandler.SetCodeGraph(codeGraphRepo, codeIndexer)
			log.Println("[CodeGraph] 代码知识图谱初始化完成")
		}
	}
	if importQueue != nil {
		if err := importQueue.StartConsumer(ctx, apiHandler.ProcessImportTask); err != nil {
			log.Fatalf("[ImportQueue] 启动消费者失败: %v", err)
		}
		log.Println("[ImportQueue] 异步导入 worker 已启动")
	}

	// ========================================
	// 启动 MCP Export Server（将项目能力暴露给外部 Agent）
	// ========================================
	if cfg.MCPExport.Enabled {
		mcpExportServer := mcpmanager.NewServer(cfg, chatService, kbRepo)

		// 注入可选依赖
		if graphRAGService != nil {
			mcpExportServer.SetGraphRAGService(graphRAGService)
		}
		if codeGraphRepo != nil {
			mcpExportServer.SetCodeGraph(codeGraphRepo)
		}
		// 注入知识库写入能力（通过 apiHandler 提供）
		mcpExportServer.SetKBWriter(apiHandler)
		if cfg.Agent.EnableCodeSearch {
			reposDir := cfg.Agent.CodeSearchReposDir
			if reposDir == "" {
				reposDir = "data/test_repos"
			}
			codeTool := tool.NewCodeSearchTool(reposDir)
			mcpExportServer.SetCodeSearchTool(codeTool)
		}

		mcpExportServer.Init()

		transport := cfg.MCPExport.Transport
		address := cfg.MCPExport.Address
		if address == "" {
			address = ":19094"
		}

		go func() {
			var serverErr error
			switch transport {
			case "sse":
				serverErr = mcpExportServer.ServeSSE(address)
			case "stdio":
				serverErr = mcpExportServer.ServeStdio()
			default:
				serverErr = mcpExportServer.ServeStreamableHTTP(address)
			}
			if serverErr != nil {
				log.Printf("[MCP Export] 服务启动失败: %v", serverErr)
			}
		}()
		log.Printf("[MCP Export] 已启动 (%s) 地址: %s", transport, address)
	}

	// ========================================
	// 设置 Gin 路由
	// ========================================
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	// 中间件
	r.Use(gin.Recovery())
	r.Use(handler.TraceIDMiddleware())
	r.Use(handler.RequestLogger())
	r.Use(metrics.PrometheusMiddleware())
	r.Use(handler.RateLimitMiddleware(handler.DefaultRateLimiterConfig()))

	// CORS 配置
	corsOrigins := cfg.Server.CORSOrigins
	if len(corsOrigins) == 0 {
		corsOrigins = []string{"*"}
	}
	allowCredentials := true
	for _, o := range corsOrigins {
		if o == "*" {
			allowCredentials = false
			break
		}
	}
	r.Use(cors.New(cors.Config{
		AllowOrigins:     corsOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: allowCredentials,
		MaxAge:           12 * time.Hour,
	}))

	// 注册路由
	apiHandler.RegisterRoutes(r)

	// Prometheus metrics 端点（不经过认证中间件）
	r.GET("/metrics", metrics.Handler())

	// Swagger 文档路由
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	log.Println("[Swagger] API 文档: http://0.0.0.0:8080/swagger/index.html")

	// ========================================
	// 启动服务器
	// ========================================
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  120 * time.Second,
	}

	// 优雅关闭
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		log.Println("\n[Server] 正在关闭服务...")
		cancel() // 通知所有使用 ctx 的 goroutine（MCP Export 等）退出
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("[Server] 服务关闭失败: %v", err)
		}
	}()

	// 启动服务
	log.Println("========================================")
	log.Printf("[Server] 服务启动: http://%s", addr)
	log.Println("[Server] API 端点:")
	log.Println("  - GET  /health                        - 健康检查")
	log.Println("  - POST /api/v1/chat                   - 聊天")
	log.Println("  - POST /api/v1/chat/stream            - 流式聊天 (SSE)")
	log.Println("  - GET  /api/v1/knowledge-bases        - 知识库列表")
	log.Println("  - POST /api/v1/knowledge-bases        - 创建知识库")
	log.Println("  - POST /api/v1/knowledge-bases/:id/documents - 上传文档")
	log.Println("  - GET  /api/v1/sessions               - 会话列表")
	log.Println("========================================")
	log.Println("[Server] 组件状态:")
	log.Printf("  - Database:  %v", db != nil)
	log.Printf("  - DocReader: %v", docReaderCli != nil)
	log.Printf("  - Redis:     %v", redisClient != nil && redisClient.Status(ctx).Available)
	log.Printf("  - Reranker:  %v", reranker != nil)
	log.Printf("  - GraphRAG:  %v", graphRAGService != nil)
	log.Printf("  - CodeGraph: %v", cfg.Agent.EnableCodeGraph)
	log.Printf("  - MCP:       %v", mcpMgr != nil && len(mcpMgr.GetTools()) > 0)
	log.Printf("  - MCP Export: %v", cfg.MCPExport.Enabled)
	log.Printf("  - ImportQueue: %v", importQueue != nil)
	log.Println("========================================")

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("[Server] 服务启动失败: %v", err)
	}

	log.Println("[Server] 服务已关闭")
}

// loadDocuments 加载文档到向量数据库
func loadDocuments(
	ctx context.Context,
	cfg *config.Config,
	embedding einoembedding.Embedder,
	vectorDB container.VectorDBProvider,
) error {
	// 创建目录加载器
	loader := document.NewDirectoryLoader()

	// 加载文档
	rawDocs, err := loader.Load(ctx, cfg.RAG.DocumentsPath)
	if err != nil {
		return fmt.Errorf("加载文档失败: %w", err)
	}
	log.Printf("[Documents] 加载了 %d 个文档", len(rawDocs))

	// 分块
	chunker := document.NewChunker(cfg.RAG.ChunkStrategy, cfg.RAG.ChunkSize, cfg.RAG.ChunkOverlap, "")
	var allChunks []*container.Document

	for _, rawDoc := range rawDocs {
		chunks, err := chunker.Chunk(ctx, rawDoc)
		if err != nil {
			log.Printf("[Documents] 分块失败 %s: %v", rawDoc.Source, err)
			continue
		}
		allChunks = append(allChunks, chunks...)
	}
	log.Printf("[Documents] 生成了 %d 个文本块", len(allChunks))

	if len(allChunks) == 0 {
		return nil
	}

	// 批量处理
	batchSize := 10
	for i := 0; i < len(allChunks); i += batchSize {
		end := i + batchSize
		if end > len(allChunks) {
			end = len(allChunks)
		}
		batch := allChunks[i:end]

		// 提取内容
		contents := make([]string, len(batch))
		for j, chunk := range batch {
			contents[j] = chunk.Content
		}

		// 批量嵌入
		vectors, err := container.BatchEmbedFloat32(ctx, embedding, contents)
		if err != nil {
			log.Printf("[Documents] 批量嵌入失败: %v", err)
			continue
		}

		// 更新向量
		for j, chunk := range batch {
			chunk.Vector = vectors[j]
		}

		// 存储到向量数据库
		if err := vectorDB.Upsert(ctx, batch); err != nil {
			log.Printf("[Documents] 存储向量失败: %v", err)
			continue
		}

		log.Printf("[Documents] 处理进度: %d/%d", end, len(allChunks))
	}

	log.Println("[Documents] 文档加载完成")
	return nil
}
