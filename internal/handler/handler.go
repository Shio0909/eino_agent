package handler

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	einoembedding "github.com/cloudwego/eino/components/embedding"
	"github.com/gin-gonic/gin"

	"eino_agent/internal/approval"
	cachepkg "eino_agent/internal/cache"
	"eino_agent/internal/codegraph"
	"eino_agent/internal/config"
	"eino_agent/internal/container"
	"eino_agent/internal/database/postgres"
	"eino_agent/internal/database/repository"
	"eino_agent/internal/docreader"
	"eino_agent/internal/graphrag"
	"eino_agent/internal/importqueue"
	mcpmanager "eino_agent/internal/mcp"
	"eino_agent/internal/rediscache"
	"eino_agent/internal/service"
	"eino_agent/internal/wiki"

	"github.com/cloudwego/eino/components/model"
)

// Handler HTTP 处理器
type Handler struct {
	cfg         *config.Config
	configPath  string
	chatService *service.ChatService
	// embedding may be swapped by settings updates; use currentEmbedder.
	embeddingMu sync.RWMutex
	embedding   einoembedding.Embedder
	// settingsMu serializes runtime configuration changes and their persistence.
	settingsMu       sync.Mutex
	vectorDB         container.VectorDBProvider
	fetchClientOnce  sync.Once
	fetchClient      *http.Client
	docReaderCli     *docreader.Client
	db               *postgres.DB
	mcpMgr           *mcpmanager.Manager
	importQueue      importqueue.Queue
	redisClient      *rediscache.Client
	sessionCache     cachepkg.SessionCache
	retrievalCache   cachepkg.RetrievalCache
	importStateStore cachepkg.ImportStateStore
	auditLogger      *AuditLogger
	graphRAGService  *graphrag.Service
	codeGraphRepo    codegraph.CodeGraphRepository
	codeIndexer      *codegraph.Indexer
	wikiCompiler     *wiki.Compiler
	chatModelFactory func(context.Context) (model.ChatModel, error)
	approvalManager  *approval.Manager

	// Repositories
	kbRepo           repository.KnowledgeBaseRepository
	accessRepo       repository.AccessControlRepository
	knowledgeRepo    repository.KnowledgeRepository
	chunkRepo        repository.ChunkRepository
	wikiRepo         repository.WikiPageRepository
	embeddingRepo    repository.EmbeddingRepository
	sessionRepo      repository.SessionRepository
	messageRepo      repository.MessageRepository
	requestTraceRepo repository.RequestTraceRepository

	// docLocks serializes index changes per document; addedChunks remembers
	// what the last update wrote, for enrichment.
	docLocks    keyedMutex
	addedMu     sync.Mutex
	addedChunks map[string][]string
}

func (h *Handler) currentEmbedder() einoembedding.Embedder {
	h.embeddingMu.RLock()
	defer h.embeddingMu.RUnlock()
	return h.embedding
}

func (h *Handler) setEmbedder(e einoembedding.Embedder) {
	h.embeddingMu.Lock()
	h.embedding = e
	h.embeddingMu.Unlock()
}

// SetMCPManager 设置 MCP 管理器
func (h *Handler) SetMCPManager(mgr *mcpmanager.Manager) {
	h.mcpMgr = mgr
}

func (h *Handler) SetApprovalManager(manager *approval.Manager) {
	h.approvalManager = manager
	if h.chatService != nil {
		h.chatService.SetApprovalManager(manager)
	}
}

// SetRedisClient 设置 Redis 客户端状态提供者。
func (h *Handler) SetRedisClient(client *rediscache.Client) {
	h.redisClient = client
}

// SetSessionCache 设置会话短期记忆缓存。
func (h *Handler) SetSessionCache(sessionCache cachepkg.SessionCache) {
	h.sessionCache = sessionCache
}

// SetRetrievalCache 设置检索缓存，用于知识库内容变更后的失效处理。
func (h *Handler) SetRetrievalCache(retrievalCache cachepkg.RetrievalCache) {
	h.retrievalCache = retrievalCache
}

// SetImportStateStore 设置异步导入任务状态存储。
func (h *Handler) SetImportStateStore(importStateStore cachepkg.ImportStateStore) {
	h.importStateStore = importStateStore
}

// SetGraphRAGService 设置 GraphRAG 服务。
func (h *Handler) SetGraphRAGService(svc *graphrag.Service) {
	h.graphRAGService = svc
}

func (h *Handler) contextModel(ctx context.Context) (model.ChatModel, error) {
	if h.chatModelFactory != nil {
		return h.chatModelFactory(ctx)
	}
	if h.cfg == nil || h.cfg.Agent.LightLLM == nil || h.cfg.Agent.LightLLM.BaseURL == "" {
		return nil, fmt.Errorf("light LLM 未配置")
	}
	llm, _, err := container.NewLLMProvider(ctx, h.cfg.Agent.LightLLM)
	return llm, err
}

func (h *Handler) docReaderStatus() gin.H {
	status := gin.H{
		"enabled":     h.cfg.DocReader.Enabled,
		"mode":        h.cfg.DocReader.Mode,
		"endpoint":    h.cfg.DocReader.Endpoint,
		"render_mode": h.cfg.DocReader.RenderMode,
		"active":      h.docReaderCli != nil,
	}
	mode := strings.ToLower(strings.TrimSpace(h.cfg.DocReader.Mode))
	switch mode {
	case "mineru":
		status["primary"] = "mineru"
		status["fallback"] = "none"
	case "mineru_with_fallback":
		status["primary"] = "mineru"
		status["fallback"] = "local"
	case "grpc", "auto", "grpc_with_fallback":
		status["primary"] = "grpc"
		status["fallback"] = "local"
	default:
		status["primary"] = "local"
		status["fallback"] = "none"
	}
	if h.cfg.DocReader.MinerUEndpoint != "" {
		status["mineru_endpoint"] = h.cfg.DocReader.MinerUEndpoint
	}
	return status
}

// SetCodeGraph 设置代码知识图谱组件。
func (h *Handler) SetCodeGraph(repo codegraph.CodeGraphRepository, indexer *codegraph.Indexer) {
	h.codeGraphRepo = repo
	h.codeIndexer = indexer
}

// SetWikiCompiler 设置 Wiki 编译器（Wiki 模式 KB 上传时使用）
func (h *Handler) SetWikiCompiler(compiler *wiki.Compiler) {
	h.wikiCompiler = compiler
}

// NewHandler 创建新的处理器
func NewHandler(
	cfg *config.Config,
	configPath string,
	chatService *service.ChatService,
	embedding einoembedding.Embedder,
	vectorDB container.VectorDBProvider,
	docReaderCli *docreader.Client,
	db *postgres.DB,
	importQueue importqueue.Queue,
) *Handler {
	if configPath == "" {
		configPath = "configs/config.yaml"
	}

	h := &Handler{
		cfg:              cfg,
		configPath:       configPath,
		chatService:      chatService,
		embedding:        embedding,
		vectorDB:         vectorDB,
		docReaderCli:     docReaderCli,
		db:               db,
		importQueue:      importQueue,
		retrievalCache:   cachepkg.NewNoopRetrievalCache(),
		importStateStore: cachepkg.NewNoopImportStateStore(),
	}
	if chatService != nil {
		chatService.SetApprovalManager(h.approvalManager)
	}

	if logger, err := NewAuditLogger("data/audit/audit.log"); err == nil {
		h.auditLogger = logger
	} else {
		log.Printf("[Audit] init logger failed: %v", err)
	}

	// 初始化 repositories (如果有数据库连接)
	if db != nil {
		h.kbRepo = repository.NewKnowledgeBaseRepository(db)
		h.accessRepo = repository.NewAccessControlRepository(db)
		h.knowledgeRepo = repository.NewKnowledgeRepository(db)
		h.chunkRepo = repository.NewChunkRepository(db)
		h.wikiRepo = repository.NewWikiPageRepository(db)
		h.embeddingRepo = repository.NewEmbeddingRepository(db)
		h.sessionRepo = repository.NewSessionRepository(db)
		h.messageRepo = repository.NewMessageRepository(db)
		h.requestTraceRepo = repository.NewRequestTraceRepository(db)
		h.chatService.SetRepositories(h.sessionRepo, h.messageRepo)
		h.chatService.SetSessionSummaryRepository(repository.NewSessionSummaryRepository(db))
		h.chatService.SetRequestTraceRepo(h.requestTraceRepo)
		h.chatService.SetDocNameResolver(h.knowledgeSourceName)
	}

	return h
}

func parsePositiveInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
