// Package service 业务服务层
package service

import (
	"context"
	"fmt"
	"io"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/skill"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"

	cachepkg "eino_agent/internal/cache"
	"eino_agent/internal/config"
	"eino_agent/internal/container"
	"eino_agent/internal/database/repository"
	"eino_agent/internal/filter"
	"eino_agent/internal/pipeline"
	internalTool "eino_agent/internal/tool"
)

// ChatService 聊天服务
// 【Eino 特点】整合 Eino 的各组件，提供统一的聊天接口
// 支持三种模式：
// 1. Pipeline 模式：线性 RAG 流水线
// 2. Agent 模式：ReAct Agent + 工具调用
// 3. Agentic RAG 模式：Graph 有环编排，检索质量评估+自动重试
type ChatService struct {
	config          *config.Config
	chatModel       model.ChatModel
	retriever       retriever.Retriever
	reranker        pipeline.Reranker // 重排序器（注入到 Pipeline）
	pipeline        *pipeline.RAGPipeline
	agent           *react.Agent
	knowledgeTool   *internalTool.KnowledgeTool  // Agent 模式的知识库工具引用
	agenticRAG      *pipeline.AgenticRAGPipeline // Agentic RAG (Corrective RAG)
	mcpTools        []tool.BaseTool              // MCP 远程工具
	skillMiddleware *adk.AgentMiddleware         // Eino 原生 skill 中间件

	// 持久化
	sessionRepo  repository.SessionRepository
	messageRepo  repository.MessageRepository
	sessionCache cachepkg.SessionCache
}

// NewChatService 创建聊天服务
func NewChatService(cfg *config.Config) (*ChatService, error) {
	svc := &ChatService{
		config:       cfg,
		sessionCache: cachepkg.NewNoopSessionCache(),
	}

	// 初始化 Eino 原生 skill 中间件（渐进式披露）
	if cfg.Agent.EnableSkills {
		backend, err := skill.NewLocalBackend(&skill.LocalBackendConfig{
			BaseDir: cfg.Agent.SkillsDir,
		})
		if err != nil {
			return nil, fmt.Errorf("init skill backend: %w", err)
		}
		mw, err := skill.New(context.Background(), &skill.Config{
			Backend:    backend,
			UseChinese: true,
		})
		if err != nil {
			return nil, fmt.Errorf("init skill middleware: %w", err)
		}
		svc.skillMiddleware = &mw
	}

	return svc, nil
}

// SetSessionCache 设置会话短期记忆缓存。
func (s *ChatService) SetSessionCache(sessionCache cachepkg.SessionCache) {
	if sessionCache == nil {
		s.sessionCache = cachepkg.NewNoopSessionCache()
		return
	}
	s.sessionCache = sessionCache
}

// SetMCPTools 设置 MCP 工具（在 InitWithComponents 之前调用）
func (s *ChatService) SetMCPTools(tools []tool.BaseTool) {
	s.mcpTools = tools
}

// SetReranker 设置重排序器（在 InitWithComponents 之前调用）
func (s *ChatService) SetReranker(r pipeline.Reranker) {
	s.reranker = r
}

// Reload 重新初始化 Pipeline/Agent，使最新 MCP 工具配置生效
func (s *ChatService) Reload() error {
	if s.chatModel == nil || s.retriever == nil {
		return fmt.Errorf("chat service is not initialized")
	}
	return s.InitWithComponents(s.chatModel, s.retriever)
}

// SetRepositories 设置持久化仓储
func (s *ChatService) SetRepositories(sessionRepo repository.SessionRepository, messageRepo repository.MessageRepository) {
	s.sessionRepo = sessionRepo
	s.messageRepo = messageRepo
}

// InitWithComponents 使用组件初始化服务
func (s *ChatService) InitWithComponents(
	chatModel model.ChatModel,
	retriever retriever.Retriever,
) error {
	ctx := context.Background()
	s.chatModel = chatModel
	s.retriever = retriever

	// 初始化 Pipeline
	pipeOpts := []pipeline.Option{
		pipeline.WithRetriever(retriever),
		pipeline.WithGenerator(pipeline.NewLLMGenerator(chatModel, s.config.Agent.SystemPrompt)),
	}
	if s.reranker != nil {
		pipeOpts = append(pipeOpts, pipeline.WithReranker(s.reranker))
	}
	s.pipeline = pipeline.NewRAGPipeline(
		&pipeline.Config{
			EnableRewrite: s.config.RAG.EnableRewrite,
			EnableRerank:  s.config.RAG.EnableRerank,
			TopK:          s.config.RAG.TopK,
			RerankTopK:    s.config.Reranker.TopK,
			SystemPrompt:  s.config.Agent.SystemPrompt,
		},
		pipeOpts...,
	)

	// 初始化 Agentic RAG（如果启用）
	if s.config.Agent.AgenticRAG.Enabled {
		agenticRAG, err := pipeline.NewAgenticRAGPipeline(
			ctx,
			&s.config.Agent.AgenticRAG,
			pipeline.WithAgenticChatModel(chatModel),
			pipeline.WithAgenticRetriever(retriever),
		)
		if err != nil {
			return fmt.Errorf("create agentic rag: %w", err)
		}
		s.agenticRAG = agenticRAG
	}

	// 初始化 Agent（如果启用）
	if s.config.Agent.Enabled {
		tools, kt := s.buildToolsWithRetriever(retriever)
		s.knowledgeTool = kt
		// 追加 MCP 远程工具
		tools = append(tools, s.mcpTools...)

		// 追加 Eino skill 中间件提供的工具（渐进式披露）
		if s.skillMiddleware != nil {
			tools = append(tools, s.skillMiddleware.AdditionalTools...)
		}

		toolCallingModel, ok := any(chatModel).(model.ToolCallingChatModel)
		if !ok {
			return fmt.Errorf("agent mode requires ToolCallingChatModel, current model type: %T", chatModel)
		}

		// 构建系统指令（含 skill 附加指令）
		systemInstruction := s.config.Agent.SystemPrompt
		if s.skillMiddleware != nil && s.skillMiddleware.AdditionalInstruction != "" {
			systemInstruction = systemInstruction + "\n\n" + s.skillMiddleware.AdditionalInstruction
		}

		reactCfg := &react.AgentConfig{
			ToolsConfig: compose.ToolsNodeConfig{
				Tools: tools,
			},
			MaxStep:          s.config.Agent.MaxSteps,
			ToolCallingModel: toolCallingModel,
			MessageModifier: func(ctx context.Context, input []*schema.Message) []*schema.Message {
				if len(input) == 0 {
					return []*schema.Message{{Role: schema.System, Content: systemInstruction}}
				}
				out := make([]*schema.Message, 0, len(input)+1)
				out = append(out, &schema.Message{Role: schema.System, Content: systemInstruction})
				out = append(out, input...)
				return out
			},
		}

		agent, err := react.NewAgent(ctx, reactCfg)
		if err != nil {
			return fmt.Errorf("create agent: %w", err)
		}
		s.agent = agent
	}

	return nil
}

// buildToolsWithRetriever 构建工具列表，同时返回 KnowledgeTool 引用以便回填 sources
func (s *ChatService) buildToolsWithRetriever(runtimeRetriever retriever.Retriever) ([]tool.BaseTool, *internalTool.KnowledgeTool) {
	var tools []tool.BaseTool
	var kt *internalTool.KnowledgeTool

	if s.config.Agent.EnableKnowledgeTool && runtimeRetriever != nil {
		kt = internalTool.NewKnowledgeTool(runtimeRetriever, s.config.RAG.TopK)
		tools = append(tools, kt)
	}

	if s.config.Agent.EnableWebSearch {
		tools = append(tools, internalTool.NewWebSearchTool(&internalTool.WebSearchConfig{
			TavilyAPIKey: s.config.Agent.TavilyAPIKey,
			SerpAPIKey:   s.config.Agent.SerpAPIKey,
			MaxResults:   5,
		}))
	}

	return tools, kt
}

// ChatRequest 聊天请求
type ChatRequest struct {
	Message          string   `json:"message"`
	SessionID        string   `json:"session_id"`
	UseAgent         bool     `json:"use_agent"`      // 是否使用 Agent 模式
	Mode             string   `json:"mode,omitempty"` // 模式: "pipeline", "agent", "agentic_rag"
	TenantID         int      `json:"tenant_id,omitempty"`
	UserID           string   `json:"user_id,omitempty"`
	ForceCitation    bool     `json:"force_citation,omitempty"` // 强制引用模式（可选）
	KnowledgeBaseIDs []string `json:"knowledge_base_ids,omitempty"`
	DocumentIDs      []string `json:"document_ids,omitempty"`
	EnableLongTerm   *bool    `json:"enable_long_term,omitempty"`
	// EnableSkills / SelectedSkills 已废弃：skills 由 Eino 原生中间件在 Agent 初始化时注入，
	// LLM 通过渐进式披露自动选择，无需客户端指定。
}

func (s *ChatService) buildRuntimeInstruction(ctx context.Context, req *ChatRequest, sessionID string) string {
	parts := make([]string, 0, 4)

	if req.ForceCitation {
		parts = append(parts,
			"【引用强制模式】回答中的关键结论必须附带来源标注（如 [来源1]）。若无法从已知信息获得来源，请明确说明\u201c当前信息无可验证来源\u201d。",
		)
	}

	if memoryInst := s.buildMemoryInstruction(ctx, req, sessionID); memoryInst != "" {
		parts = append(parts, memoryInst)
	}

	// Skills 已迁移至 Eino 原生 skill 中间件（渐进式披露），
	// 不再在运行时指令中手动注入 skill prompt。

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, "\n\n")
}

type retrievalScope struct {
	kbSet  map[string]struct{}
	docSet map[string]struct{}
}

func (s *ChatService) buildRetrievalScope(req *ChatRequest) retrievalScope {
	scope := retrievalScope{
		kbSet:  make(map[string]struct{}),
		docSet: make(map[string]struct{}),
	}
	for _, kbID := range req.KnowledgeBaseIDs {
		id := strings.TrimSpace(kbID)
		if id != "" {
			scope.kbSet[id] = struct{}{}
		}
	}
	for _, docID := range req.DocumentIDs {
		id := strings.TrimSpace(docID)
		if id != "" {
			scope.docSet[id] = struct{}{}
		}
	}
	return scope
}

func (s retrievalScope) isEmpty() bool {
	return len(s.kbSet) == 0 && len(s.docSet) == 0
}

type scopedRetriever struct {
	base  retriever.Retriever
	scope retrievalScope
}

type knowledgeBaseScopedRetriever interface {
	WithKnowledgeBaseScope(ids []string) retriever.Retriever
}

func (r *scopedRetriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	docs, err := r.base.Retrieve(ctx, query, opts...)
	if err != nil {
		return nil, err
	}
	if r.scope.isEmpty() {
		return docs, nil
	}

	filtered := make([]*schema.Document, 0, len(docs))
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		if !r.matchDoc(doc) {
			continue
		}
		filtered = append(filtered, doc)
	}

	return filtered, nil
}

func (r *scopedRetriever) matchDoc(doc *schema.Document) bool {
	if len(r.scope.kbSet) > 0 {
		kbID := strings.TrimSpace(toString(metaValue(doc.MetaData, "knowledge_base_id")))
		if kbID == "" {
			return false
		}
		if _, ok := r.scope.kbSet[kbID]; !ok {
			return false
		}
	}

	if len(r.scope.docSet) > 0 {
		ids := []string{
			strings.TrimSpace(doc.ID),
			strings.TrimSpace(toString(metaValue(doc.MetaData, "knowledge_id"))),
			strings.TrimSpace(toString(metaValue(doc.MetaData, "document_id"))),
			strings.TrimSpace(toString(metaValue(doc.MetaData, "doc_id"))),
			strings.TrimSpace(toString(metaValue(doc.MetaData, "chunk_id"))),
		}
		for _, id := range ids {
			if id == "" {
				continue
			}
			if _, ok := r.scope.docSet[id]; ok {
				return true
			}
		}
		return false
	}

	return true
}

func metaValue(meta map[string]any, key string) any {
	if meta == nil {
		return nil
	}
	return meta[key]
}

func toString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case fmt.Stringer:
		return val.String()
	default:
		if v == nil {
			return ""
		}
		return fmt.Sprintf("%v", v)
	}
}

func (s *ChatService) getRuntimeRetriever(req *ChatRequest) retriever.Retriever {
	if s.retriever == nil {
		return nil
	}
	scope := s.buildRetrievalScope(req)
	if scope.isEmpty() {
		return s.retriever
	}
	if len(scope.kbSet) > 0 {
		ids := make([]string, 0, len(scope.kbSet))
		for id := range scope.kbSet {
			ids = append(ids, id)
		}
		if scoped, ok := s.retriever.(knowledgeBaseScopedRetriever); ok {
			return &scopedRetriever{base: scoped.WithKnowledgeBaseScope(ids), scope: scope}
		}
		if composite, ok := s.retriever.(*container.CompositeRetriever); ok {
			return &scopedRetriever{base: composite.WithKnowledgeBaseScope(ids), scope: scope}
		}
	}
	return &scopedRetriever{base: s.retriever, scope: scope}
}

func (s *ChatService) buildRuntimeAgent(ctx context.Context, runtimeRetriever retriever.Retriever) (*react.Agent, *internalTool.KnowledgeTool, error) {
	toolCallingModel, ok := any(s.chatModel).(model.ToolCallingChatModel)
	if !ok {
		return nil, nil, fmt.Errorf("agent mode requires ToolCallingChatModel, current model type: %T", s.chatModel)
	}

	tools, kt := s.buildToolsWithRetriever(runtimeRetriever)
	tools = append(tools, s.mcpTools...)
	if s.skillMiddleware != nil {
		tools = append(tools, s.skillMiddleware.AdditionalTools...)
	}

	systemInstruction := s.config.Agent.SystemPrompt
	if s.skillMiddleware != nil && s.skillMiddleware.AdditionalInstruction != "" {
		systemInstruction = systemInstruction + "\n\n" + s.skillMiddleware.AdditionalInstruction
	}

	agent, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolsConfig:      compose.ToolsNodeConfig{Tools: tools},
		MaxStep:          s.config.Agent.MaxSteps,
		ToolCallingModel: toolCallingModel,
		MessageModifier: func(ctx context.Context, input []*schema.Message) []*schema.Message {
			if len(input) == 0 {
				return []*schema.Message{{Role: schema.System, Content: systemInstruction}}
			}
			out := make([]*schema.Message, 0, len(input)+1)
			out = append(out, &schema.Message{Role: schema.System, Content: systemInstruction})
			out = append(out, input...)
			return out
		},
	})
	return agent, kt, err
}

func (s *ChatService) buildRuntimePipeline(runtimeRetriever retriever.Retriever) *pipeline.RAGPipeline {
	if runtimeRetriever == nil {
		return nil
	}

	pipeOpts := []pipeline.Option{
		pipeline.WithRetriever(runtimeRetriever),
		pipeline.WithGenerator(pipeline.NewLLMGenerator(s.chatModel, s.config.Agent.SystemPrompt)),
	}
	if s.reranker != nil {
		pipeOpts = append(pipeOpts, pipeline.WithReranker(s.reranker))
	}
	return pipeline.NewRAGPipeline(
		&pipeline.Config{
			EnableRewrite: s.config.RAG.EnableRewrite,
			EnableRerank:  s.config.RAG.EnableRerank,
			TopK:          s.config.RAG.TopK,
			RerankTopK:    s.config.Reranker.TopK,
			SystemPrompt:  s.config.Agent.SystemPrompt,
		},
		pipeOpts...,
	)
}

func (s *ChatService) buildRuntimeAgenticRAG(ctx context.Context, runtimeRetriever retriever.Retriever) (*pipeline.AgenticRAGPipeline, error) {
	if runtimeRetriever == nil {
		return nil, fmt.Errorf("retriever is nil")
	}
	return pipeline.NewAgenticRAGPipeline(
		ctx,
		&s.config.Agent.AgenticRAG,
		pipeline.WithAgenticChatModel(s.chatModel),
		pipeline.WithAgenticRetriever(runtimeRetriever),
	)
}

func (s *ChatService) buildMemoryInstruction(ctx context.Context, req *ChatRequest, sessionID string) string {
	if !s.config.Memory.Enabled || sessionID == "" || s.messageRepo == nil {
		return ""
	}

	maxChars := s.config.Memory.MaxContextChars
	if maxChars <= 0 {
		maxChars = 3000
	}

	parts := make([]string, 0, 2)

	window := s.config.Memory.WindowSize
	if window <= 0 {
		window = 8
	}

	shortMsgs, err := s.getShortTermMessages(ctx, sessionID, window*2)
	if err == nil && len(shortMsgs) > 0 {
		shortCtx := formatMemoryMessages(shortMsgs, maxChars/2)
		if shortCtx != "" {
			parts = append(parts, "【会话短期记忆】\n"+shortCtx)
		}
	}

	enableLongTerm := s.config.Memory.EnableLongTerm
	if req.EnableLongTerm != nil {
		enableLongTerm = *req.EnableLongTerm
	}

	if enableLongTerm && s.sessionRepo != nil && req.UserID != "" {
		sessionLimit := s.config.Memory.LongTermSessionLimit
		if sessionLimit <= 0 {
			sessionLimit = 5
		}
		msgPerSession := s.config.Memory.LongTermMessagesPerSession
		if msgPerSession <= 0 {
			msgPerSession = 2
		}

		sessions, sErr := s.sessionRepo.List(ctx, req.TenantID, req.UserID, 0, sessionLimit)
		if sErr == nil {
			sort.SliceStable(sessions, func(i, j int) bool {
				return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
			})

			var sb strings.Builder
			for _, session := range sessions {
				if session == nil || session.ID == "" || session.ID == sessionID {
					continue
				}
				msgs, mErr := s.messageRepo.ListBySession(ctx, session.ID, msgPerSession*2)
				if mErr != nil || len(msgs) == 0 {
					continue
				}
				title := strings.TrimSpace(session.Title)
				if title == "" {
					title = session.ID
				}
				sb.WriteString("[历史会话] ")
				sb.WriteString(title)
				sb.WriteString("\n")
				sb.WriteString(formatMemoryMessages(msgs, maxChars/4))
				sb.WriteString("\n")
				if sb.Len() >= maxChars/2 {
					break
				}
			}
			longCtx := strings.TrimSpace(sb.String())
			if longCtx != "" {
				parts = append(parts, "【跨会话长期记忆（数据库）】\n"+truncateText(longCtx, maxChars/2))
			}
		}
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, "\n\n")
}

func formatMemoryMessages(messages []*repository.Message, maxChars int) string {
	if len(messages) == 0 {
		return ""
	}
	if maxChars <= 0 {
		maxChars = 1000
	}

	start := 0
	if len(messages) > 12 {
		start = len(messages) - 12
	}

	var sb strings.Builder
	for i := start; i < len(messages); i++ {
		msg := messages[i]
		if msg == nil {
			continue
		}
		role := "用户"
		if strings.EqualFold(msg.Role, "assistant") {
			role = "助手"
		}
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		sb.WriteString(role)
		sb.WriteString(": ")
		sb.WriteString(truncateText(content, 180))
		sb.WriteString("\n")
		if sb.Len() >= maxChars {
			break
		}
	}

	return truncateText(strings.TrimSpace(sb.String()), maxChars)
}

func truncateText(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if maxLen <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

func appendInstructionToMessage(message, instruction string) string {
	if strings.TrimSpace(instruction) == "" {
		return message
	}
	return fmt.Sprintf("%s\n\n请遵循以下回答要求：\n%s", message, instruction)
}

func (s *ChatService) shortTermMessageLimit() int {
	window := s.config.Memory.WindowSize
	if window <= 0 {
		window = 8
	}
	return window * 2
}

func (s *ChatService) shortTermCacheTTL() time.Duration {
	ttlMinutes := s.config.Memory.ShortTermCacheTTLMinutes
	if ttlMinutes <= 0 {
		ttlMinutes = 60
	}
	return time.Duration(ttlMinutes) * time.Minute
}

func (s *ChatService) getShortTermMessages(ctx context.Context, sessionID string, limit int) ([]*repository.Message, error) {
	if s.messageRepo == nil || sessionID == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = s.shortTermMessageLimit()
	}

	if s.sessionCache != nil {
		cachedMessages, hit, err := s.sessionCache.GetRecentMessages(ctx, sessionID, limit)
		if err != nil {
			log.Printf("[ChatService] 读取会话缓存失败: %v", err)
		} else if hit {
			return cacheMessagesToRepository(cachedMessages), nil
		}
	}

	messages, err := s.messageRepo.ListBySession(ctx, sessionID, limit)
	if err != nil {
		return nil, err
	}

	if s.sessionCache != nil && len(messages) > 0 {
		if err := s.sessionCache.SetRecentMessages(ctx, sessionID, repositoryMessagesToCache(messages), s.shortTermCacheTTL()); err != nil {
			log.Printf("[ChatService] 回填会话缓存失败: %v", err)
		}
	}

	return messages, nil
}

func (s *ChatService) refreshSessionCache(ctx context.Context, sessionID string, fallbackMsg *repository.Message) {
	if s.sessionCache == nil || s.messageRepo == nil || sessionID == "" {
		return
	}

	limit := s.shortTermMessageLimit()
	if limit <= 0 {
		return
	}

	cachedMessages, hit, err := s.sessionCache.GetRecentMessages(ctx, sessionID, limit)
	if err != nil {
		log.Printf("[ChatService] 读取会话缓存失败: %v", err)
		hit = false
	}

	if hit {
		updated := append(cachedMessages, repositoryMessageToCache(fallbackMsg))
		if len(updated) > limit {
			updated = updated[len(updated)-limit:]
		}
		if err := s.sessionCache.SetRecentMessages(ctx, sessionID, updated, s.shortTermCacheTTL()); err != nil {
			log.Printf("[ChatService] 更新会话缓存失败: %v", err)
		}
		return
	}

	messages, listErr := s.messageRepo.ListBySession(ctx, sessionID, limit)
	if listErr != nil {
		log.Printf("[ChatService] 回源刷新会话缓存失败: %v", listErr)
		return
	}
	if len(messages) == 0 && fallbackMsg != nil {
		messages = []*repository.Message{fallbackMsg}
	}
	if len(messages) == 0 {
		return
	}
	if err := s.sessionCache.SetRecentMessages(ctx, sessionID, repositoryMessagesToCache(messages), s.shortTermCacheTTL()); err != nil {
		log.Printf("[ChatService] 刷新会话缓存失败: %v", err)
	}
}

func repositoryMessagesToCache(messages []*repository.Message) []cachepkg.SessionMessage {
	if len(messages) == 0 {
		return nil
	}
	result := make([]cachepkg.SessionMessage, 0, len(messages))
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		result = append(result, repositoryMessageToCache(msg))
	}
	return result
}

func repositoryMessageToCache(msg *repository.Message) cachepkg.SessionMessage {
	if msg == nil {
		return cachepkg.SessionMessage{}
	}
	return cachepkg.SessionMessage{
		Role:      msg.Role,
		Content:   msg.Content,
		CreatedAt: msg.CreatedAt,
	}
}

func cacheMessagesToRepository(messages []cachepkg.SessionMessage) []*repository.Message {
	if len(messages) == 0 {
		return nil
	}
	result := make([]*repository.Message, 0, len(messages))
	for _, msg := range messages {
		result = append(result, &repository.Message{
			Role:      msg.Role,
			Content:   msg.Content,
			CreatedAt: msg.CreatedAt,
		})
	}
	return result
}

// ChatResponse 聊天响应
type ChatResponse struct {
	Answer    string   `json:"answer"`
	Sources   []Source `json:"sources,omitempty"`
	SessionID string   `json:"session_id,omitempty"`
}

// Source 来源信息
type Source struct {
	Content string `json:"content"`
	DocID   string `json:"doc_id"`
}

func (s *ChatService) buildSourcesFromRetriever(ctx context.Context, runtimeRetriever retriever.Retriever, query string, maxN int) []Source {
	if runtimeRetriever == nil || strings.TrimSpace(query) == "" {
		return nil
	}

	docs, err := runtimeRetriever.Retrieve(ctx, query)
	if err != nil {
		log.Printf("[ChatService] 回填来源检索失败: %v", err)
		return nil
	}

	if maxN <= 0 {
		maxN = 5
	}

	sources := make([]Source, 0, maxN)
	seen := make(map[string]struct{}, maxN)
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		docID := strings.TrimSpace(doc.ID)
		if docID == "" {
			continue
		}
		if _, ok := seen[docID]; ok {
			continue
		}
		sources = append(sources, Source{Content: doc.Content, DocID: docID})
		seen[docID] = struct{}{}
		if len(sources) >= maxN {
			break
		}
	}

	return sources
}

// truncateTitle 截取消息前 N 个字符作为会话标题
func truncateTitle(msg string, maxLen int) string {
	msg = strings.TrimSpace(msg)
	// 去掉换行
	msg = strings.ReplaceAll(msg, "\n", " ")
	runes := []rune(msg)
	if len(runes) > maxLen {
		return string(runes[:maxLen]) + "..."
	}
	return msg
}

// ensureSession 确保会话存在，如果 SessionID 为空则自动创建
func (s *ChatService) ensureSession(ctx context.Context, req *ChatRequest) (string, error) {
	if req.SessionID != "" {
		return req.SessionID, nil
	}

	if s.sessionRepo == nil {
		return "", nil
	}

	session := &repository.Session{
		TenantID:            req.TenantID,
		UserID:              req.UserID,
		Title:               truncateTitle(req.Message, 50),
		SimilarityThreshold: 0.7,
		TopK:                s.config.RAG.TopK,
	}
	if session.TenantID <= 0 {
		session.TenantID = 1
	}
	if err := s.sessionRepo.Create(ctx, session); err != nil {
		log.Printf("[ChatService] 自动创建会话失败: %v", err)
		return "", nil // 不阻塞聊天
	}
	log.Printf("[ChatService] 自动创建会话: %s", session.ID)
	return session.ID, nil
}

// saveUserMessage 保存用户消息
func (s *ChatService) saveUserMessage(ctx context.Context, sessionID, content string) {
	if s.messageRepo == nil || sessionID == "" {
		return
	}

	msg := &repository.Message{
		SessionID: sessionID,
		Role:      "user",
		Content:   content,
	}
	if err := s.messageRepo.Create(ctx, msg); err != nil {
		log.Printf("[ChatService] 保存用户消息失败: %v", err)
		return
	}
	s.refreshSessionCache(ctx, sessionID, msg)
}

// saveAssistantMessage 保存助手消息
func (s *ChatService) saveAssistantMessage(ctx context.Context, sessionID, content string, tokensUsed int, latencyMs int64) {
	if s.messageRepo == nil || sessionID == "" {
		return
	}

	msg := &repository.Message{
		SessionID:  sessionID,
		Role:       "assistant",
		Content:    content,
		TokensUsed: tokensUsed,
		LatencyMs:  int(latencyMs),
	}
	if err := s.messageRepo.Create(ctx, msg); err != nil {
		log.Printf("[ChatService] 保存助手消息失败: %v", err)
		return
	}
	s.refreshSessionCache(ctx, sessionID, msg)
}

// Chat 执行聊天
func (s *ChatService) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	startTime := time.Now()

	// 确保会话存在
	sessionID, _ := s.ensureSession(ctx, req)
	runtimeInstruction := s.buildRuntimeInstruction(ctx, req, sessionID)
	messageWithInstruction := appendInstructionToMessage(req.Message, runtimeInstruction)
	runtimeRetriever := s.getRuntimeRetriever(req)

	// 保存用户消息
	s.saveUserMessage(ctx, sessionID, req.Message)

	var resp *ChatResponse
	var err error

	// Agentic RAG 模式
	if req.Mode == "agentic_rag" && s.agenticRAG != nil {
		modeStart := time.Now()
		runtimeAgentic := s.agenticRAG
		if runtimeRetriever != s.retriever {
			created, createErr := s.buildRuntimeAgenticRAG(ctx, runtimeRetriever)
			if createErr != nil {
				return nil, fmt.Errorf("build runtime agentic rag: %w", createErr)
			}
			runtimeAgentic = created
		}

		ragResp, ragErr := runtimeAgentic.Run(ctx, messageWithInstruction)
		if ragErr != nil {
			return nil, fmt.Errorf("agentic rag: %w", ragErr)
		}
		answer := filter.StripThinkTags(ragResp.Answer)
		sourceStart := time.Now()
		sources := s.buildSourcesFromRetriever(ctx, runtimeRetriever, req.Message, s.config.RAG.TopK)
		log.Printf("[Timing][ChatService] mode=agentic_rag stage=source_backfill duration_ms=%d sources=%d", time.Since(sourceStart).Milliseconds(), len(sources))
		resp = &ChatResponse{
			Answer:    answer,
			Sources:   sources,
			SessionID: sessionID,
		}
		log.Printf("[Timing][ChatService] mode=agentic_rag stage=total duration_ms=%d", time.Since(modeStart).Milliseconds())
		err = nil
	} else if req.UseAgent && s.agent != nil {
		// Agent 模式
		modeStart := time.Now()
		runtimeAgent := s.agent
		kt := s.knowledgeTool
		if runtimeRetriever != s.retriever {
			created, rtKt, createErr := s.buildRuntimeAgent(ctx, runtimeRetriever)
			if createErr != nil {
				return nil, fmt.Errorf("build runtime agent: %w", createErr)
			}
			runtimeAgent = created
			kt = rtKt
		}

		messages := []*schema.Message{
			{Role: schema.User, Content: messageWithInstruction},
		}
		respMsg, agentErr := runtimeAgent.Generate(ctx, messages)
		if agentErr != nil {
			return nil, fmt.Errorf("agent chat: %w", agentErr)
		}

		answer := ""
		if respMsg != nil {
			answer = respMsg.Content
		}
		answer = filter.StripThinkTags(answer)

		// 从 KnowledgeTool 缓存中提取 sources，避免冗余二次检索
		var sources []Source
		if kt != nil {
			for _, doc := range kt.LastDocs() {
				if doc == nil || strings.TrimSpace(doc.ID) == "" {
					continue
				}
				sources = append(sources, Source{Content: doc.Content, DocID: doc.ID})
				if len(sources) >= s.config.RAG.TopK {
					break
				}
			}
		}
		log.Printf("[Timing][ChatService] mode=agent stage=source_from_cache sources=%d", len(sources))

		resp = &ChatResponse{
			Answer:    answer,
			Sources:   sources,
			SessionID: sessionID,
		}
		log.Printf("[Timing][ChatService] mode=agent stage=total duration_ms=%d", time.Since(modeStart).Milliseconds())
		err = nil
	} else if s.pipeline != nil {
		// Pipeline 模式
		runtimePipeline := s.pipeline
		if runtimeRetriever != s.retriever {
			runtimePipeline = s.buildRuntimePipeline(runtimeRetriever)
		}

		pipeResp, pipeErr := runtimePipeline.Run(ctx, &pipeline.RAGRequest{
			Query:                 req.Message,
			SessionID:             sessionID,
			GenerationInstruction: runtimeInstruction,
		})
		if pipeErr != nil {
			return nil, fmt.Errorf("pipeline run: %w", pipeErr)
		}

		pipeResp.Answer = filter.StripThinkTags(pipeResp.Answer)

		sources := make([]Source, len(pipeResp.Sources))
		for i, src := range pipeResp.Sources {
			sources[i] = Source{
				Content: src.Content,
				DocID:   src.DocID,
			}
		}

		resp = &ChatResponse{
			Answer:    pipeResp.Answer,
			Sources:   sources,
			SessionID: sessionID,
		}
		err = nil
	} else {
		return nil, fmt.Errorf("no chat handler available")
	}

	if err != nil {
		return nil, err
	}

	// 保存助手消息
	latencyMs := time.Since(startTime).Milliseconds()
	s.saveAssistantMessage(ctx, sessionID, resp.Answer, 0, latencyMs)

	return resp, nil
}

// ChatStream 流式聊天
func (s *ChatService) ChatStream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent, 100)

	// 确保会话存在
	sessionID, _ := s.ensureSession(ctx, req)
	runtimeInstruction := s.buildRuntimeInstruction(ctx, req, sessionID)
	messageWithInstruction := appendInstructionToMessage(req.Message, runtimeInstruction)
	runtimeRetriever := s.getRuntimeRetriever(req)

	// 保存用户消息
	s.saveUserMessage(ctx, sessionID, req.Message)

	// 发送 session_id 事件（首个事件）
	if sessionID != "" {
		ch <- StreamEvent{Type: "session_id", SessionID: sessionID}
	}

	go func() {
		defer close(ch)

		startTime := time.Now()
		firstTokenLogged := false
		var fullResponse strings.Builder

		// 创建流式 think 标签过滤器
		thinkFilter := filter.NewThinkTagStreamFilter()

		// Agentic RAG 模式
		if req.Mode == "agentic_rag" && s.agenticRAG != nil {
			runtimeAgentic := s.agenticRAG
			if runtimeRetriever != s.retriever {
				created, createErr := s.buildRuntimeAgenticRAG(ctx, runtimeRetriever)
				if createErr != nil {
					ch <- StreamEvent{Type: "error", Error: createErr.Error()}
					return
				}
				runtimeAgentic = created
			}

			stream, err := runtimeAgentic.RunStream(ctx, messageWithInstruction)
			if err != nil {
				ch <- StreamEvent{Type: "error", Error: err.Error()}
				return
			}

			for event := range stream {
				switch event.Type {
				case pipeline.AgenticEventContent:
					filtered := thinkFilter.Filter(event.Content)
					if filtered != "" {
						if !firstTokenLogged {
							log.Printf("[Timing][ChatService] mode=agentic_rag stage=first_token duration_ms=%d", time.Since(startTime).Milliseconds())
							firstTokenLogged = true
						}
						fullResponse.WriteString(filtered)
						ch <- StreamEvent{Type: "content", Content: filtered}
					}
				case pipeline.AgenticEventStatus:
					ch <- StreamEvent{Type: "status", Content: event.Content}
				case pipeline.AgenticEventDone:
					if remaining := thinkFilter.Flush(); remaining != "" {
						fullResponse.WriteString(remaining)
						ch <- StreamEvent{Type: "content", Content: remaining}
					}
					ch <- StreamEvent{Type: "done"}
				case pipeline.AgenticEventError:
					ch <- StreamEvent{Type: "error", Error: event.Content}
				}
			}

			// 保存助手消息
			latencyMs := time.Since(startTime).Milliseconds()
			s.saveAssistantMessage(ctx, sessionID, fullResponse.String(), 0, latencyMs)
			return
		}

		// Agent 模式
		if req.UseAgent && s.agent != nil {
			runtimeAgent := s.agent
			if runtimeRetriever != s.retriever {
				created, _, createErr := s.buildRuntimeAgent(ctx, runtimeRetriever)
				if createErr != nil {
					ch <- StreamEvent{Type: "error", Error: createErr.Error()}
					return
				}
				runtimeAgent = created
			}

			messages := []*schema.Message{
				{Role: schema.User, Content: messageWithInstruction},
			}
			stream, err := runtimeAgent.Stream(ctx, messages)
			if err != nil {
				ch <- StreamEvent{Type: "error", Error: err.Error()}
				return
			}
			defer stream.Close()

			for {
				chunk, recvErr := stream.Recv()
				if recvErr != nil {
					if recvErr == io.EOF {
						break
					}
					ch <- StreamEvent{Type: "error", Error: recvErr.Error()}
					return
				}

				if chunk == nil || chunk.Content == "" {
					continue
				}

				filtered := thinkFilter.Filter(chunk.Content)
				if filtered != "" {
					if !firstTokenLogged {
						log.Printf("[Timing][ChatService] mode=agent stage=first_token duration_ms=%d", time.Since(startTime).Milliseconds())
						firstTokenLogged = true
					}
					fullResponse.WriteString(filtered)
					ch <- StreamEvent{Type: "content", Content: filtered}
				}
			}
			// 刷新过滤器缓冲区
			if remaining := thinkFilter.Flush(); remaining != "" {
				fullResponse.WriteString(remaining)
				ch <- StreamEvent{Type: "content", Content: remaining}
			}
			ch <- StreamEvent{Type: "done"}

			// 保存助手消息
			latencyMs := time.Since(startTime).Milliseconds()
			s.saveAssistantMessage(ctx, sessionID, fullResponse.String(), 0, latencyMs)
			return
		}

		// Pipeline 模式
		if s.pipeline != nil {
			runtimePipeline := s.pipeline
			if runtimeRetriever != s.retriever {
				runtimePipeline = s.buildRuntimePipeline(runtimeRetriever)
			}

			stream, err := runtimePipeline.RunStream(ctx, &pipeline.RAGRequest{
				Query:                 req.Message,
				SessionID:             sessionID,
				GenerationInstruction: runtimeInstruction,
			})
			if err != nil {
				ch <- StreamEvent{Type: "error", Error: err.Error()}
				return
			}

			for chunk := range stream {
				if chunk.Type == pipeline.ChunkTypeContent {
					// 流式过滤 认
					filtered := thinkFilter.Filter(chunk.Content)
					if filtered != "" {
						if !firstTokenLogged {
							log.Printf("[Timing][ChatService] mode=pipeline stage=first_token duration_ms=%d", time.Since(startTime).Milliseconds())
							firstTokenLogged = true
						}
						fullResponse.WriteString(filtered)
						ch <- StreamEvent{
							Type:    string(chunk.Type),
							Content: filtered,
							DocID:   chunk.DocID,
						}
					}
				} else {
					ch <- StreamEvent{
						Type:    string(chunk.Type),
						Content: chunk.Content,
						DocID:   chunk.DocID,
					}
				}
			}
			// 刷新过滤器缓冲区
			if remaining := thinkFilter.Flush(); remaining != "" {
				fullResponse.WriteString(remaining)
				ch <- StreamEvent{Type: "content", Content: remaining}
			}

			// 保存助手消息
			latencyMs := time.Since(startTime).Milliseconds()
			s.saveAssistantMessage(ctx, sessionID, fullResponse.String(), 0, latencyMs)
			return
		}

		ch <- StreamEvent{Type: "error", Error: "no handler available"}
	}()

	return ch, nil
}

// StreamEvent 流式事件
type StreamEvent struct {
	Type      string `json:"type"`
	Content   string `json:"content,omitempty"`
	DocID     string `json:"doc_id,omitempty"`
	Error     string `json:"error,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}
