// Package service 业务服务层
package service

import (
	"context"
	"errors"
	"fmt"
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

	"eino_agent/internal/approval"
	cachepkg "eino_agent/internal/cache"
	"eino_agent/internal/codegraph"
	"eino_agent/internal/config"
	"eino_agent/internal/container"
	"eino_agent/internal/database/repository"
	"eino_agent/internal/filter"
	"eino_agent/internal/graphrag"
	"eino_agent/internal/logger"
	"eino_agent/internal/metrics"
	"eino_agent/internal/pipeline"
	promptmgr "eino_agent/internal/prompt"
	"eino_agent/internal/retrieval"
	internalTool "eino_agent/internal/tool"
)

// ChatService 聊天服务
// 【Eino 特点】整合 Eino 的各组件，提供统一的聊天接口
// 支持两种模式：
// 1. Pipeline 模式：线性 RAG 流水线
// 2. Agentic 模式：ReAct Agent + 工具调用（knowledge_search + query_decompose + web_search）
type ChatService struct {
	config          *config.Config
	chatModel       model.ChatModel
	lightModel      model.ChatModel // 轻量模型，用于 query_decompose 等辅助任务
	retriever       retriever.Retriever
	reranker        pipeline.Reranker // 重排序器（注入到 Pipeline）
	pipeline        *pipeline.RAGPipeline
	agent           *react.Agent
	knowledgeTool   *internalTool.KnowledgeTool   // Agent 模式的知识库工具引用
	graphRAGService *graphrag.Service             // GraphRAG 服务（可选，用于按需创建图谱检索器）
	codeGraphRepo   codegraph.CodeGraphRepository // 代码知识图谱存储（可选）
	codeIndexer     *codegraph.Indexer            // 代码索引器（可选）
	skillBackend    skill.Backend
	mcpTools        []tool.BaseTool      // MCP 远程工具
	skillMiddleware *adk.AgentMiddleware // Eino 原生 skill 中间件
	promptManager   *promptmgr.Manager
	approvalManager *approval.Manager

	// 持久化
	sessionRepo      repository.SessionRepository
	messageRepo      repository.MessageRepository
	summaryRepo      repository.SessionSummaryRepository // 跨轮会话摘要（可选，nil 则回退到窗口截断）
	sessionCache     cachepkg.SessionCache
	auditRepo        repository.LLMAuditRepository // LLM 调用审计日志（可选）
	requestTraceRepo repository.RequestTraceRepository
}

// NewChatService 创建聊天服务
func NewChatService(cfg *config.Config) (*ChatService, error) {
	svc := &ChatService{
		config:        cfg,
		sessionCache:  cachepkg.NewNoopSessionCache(),
		promptManager: promptmgr.NewManager(),
	}

	// 初始化 skill 中间件（渐进式披露）
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
		svc.skillBackend = backend
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

func (s *ChatService) SetApprovalManager(manager *approval.Manager) {
	s.approvalManager = manager
}

// SetReranker 设置重排序器（在 InitWithComponents 之前调用）
func (s *ChatService) SetReranker(r pipeline.Reranker) {
	s.reranker = r
}

// SetChatModel 替换 ChatModel（用于运行时热重载）
func (s *ChatService) SetChatModel(m model.ChatModel) {
	s.chatModel = m
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

// SetAuditRepo 设置 LLM 调用审计日志仓储（可选，nil 则不记录）
func (s *ChatService) SetAuditRepo(repo repository.LLMAuditRepository) {
	s.auditRepo = repo
}

func (s *ChatService) SetRequestTraceRepo(repo repository.RequestTraceRepository) {
	s.requestTraceRepo = repo
}

func (s *ChatService) recordRequestTrace(traceID string, req *ChatRequest, sessionID, messageID, mode, status string, latencyMs int64, steps []TraceStep, summary repository.JSON, errText string) {
	if s.requestTraceRepo == nil || traceID == "" {
		return
	}
	trace := &repository.RequestTrace{
		TraceID:   traceID,
		TenantID:  req.TenantID,
		UserID:    req.UserID,
		SessionID: sessionID,
		MessageID: messageID,
		Mode:      mode,
		Status:    status,
		LatencyMs: int(latencyMs),
		Steps:     steps,
		Summary:   summary,
		Error:     errText,
	}
	if trace.TenantID <= 0 {
		trace.TenantID = 1
	}
	go func() {
		writeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := s.requestTraceRepo.Create(writeCtx, trace); err != nil {
			log.Printf("[Trace] 写入请求 trace 失败: %v", err)
		}
	}()
}

// recordLLMAudit 异步写入 LLM 审计日志，不阻塞主流程。
func (s *ChatService) recordLLMAudit(ctx context.Context, entry *repository.LLMAuditLog) {
	if s.auditRepo == nil || entry == nil {
		return
	}
	go func() {
		writeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := s.auditRepo.Create(writeCtx, entry); err != nil {
			log.Printf("[Audit] 写入 LLM 审计日志失败: %v", err)
		}
	}()
}

// SetGraphRAGService 设置 GraphRAG 服务（按需创建图谱检索器）
func (s *ChatService) SetGraphRAGService(svc *graphrag.Service) {
	s.graphRAGService = svc
}

// SetCodeGraph 设置代码知识图谱存储和索引器
func (s *ChatService) SetCodeGraph(repo codegraph.CodeGraphRepository, indexer *codegraph.Indexer) {
	s.codeGraphRepo = repo
	s.codeIndexer = indexer
}

// InitWithComponents 使用组件初始化服务
func (s *ChatService) InitWithComponents(
	chatModel model.ChatModel,
	retriever retriever.Retriever,
) error {
	ctx := context.Background()
	s.chatModel = chatModel
	s.retriever = retriever

	// 初始化轻量模型（用于 query_decompose 等辅助工具）
	if lm := s.config.Agent.LightLLM; lm != nil && lm.ModelID != "" {
		lightModel, _, err := container.NewLLMProvider(ctx, lm)
		if err != nil {
			log.Printf("[ChatService] 轻量模型初始化失败，降级使用主模型: %v", err)
			s.lightModel = chatModel
		} else {
			log.Printf("[ChatService] 轻量模型已启用: %s/%s", lm.Provider, lm.ModelID)
			s.lightModel = lightModel
		}
	} else {
		s.lightModel = chatModel
	}

	// 初始化 Pipeline
	pipeOpts := []pipeline.Option{
		pipeline.WithRetriever(retriever),
		pipeline.WithGenerator(pipeline.NewLLMGenerator(chatModel, s.config.Agent.SystemPrompt)),
	}
	if s.reranker != nil {
		pipeOpts = append(pipeOpts, pipeline.WithReranker(s.reranker))
	}
	// 注入查询重写器
	pipeOpts = append(pipeOpts, pipeline.WithRewriter(pipeline.NewLLMRewriter(chatModel)))
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
		systemInstruction := s.renderSystemPrompt("agentic")
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
		kt = internalTool.NewKnowledgeTool(runtimeRetriever, s.config.RAG.TopK, s.config.Agent.MaxContentPerDoc, s.config.Agent.MaxTotalContent)
		if s.config.Agent.EnableConflictDetection && s.lightModel != nil {
			kt.SetLightModel(s.lightModel)
		}
		tools = append(tools, kt)
	}

	// query_decompose 工具（使用轻量模型）
	if s.lightModel != nil {
		tools = append(tools, internalTool.NewQueryDecomposeTool(s.lightModel))
	}

	if s.config.Agent.EnableWebSearch {
		tools = append(tools, internalTool.NewWebSearchTool(&internalTool.WebSearchConfig{
			TavilyAPIKey: s.config.Agent.TavilyAPIKey,
			SerpAPIKey:   s.config.Agent.SerpAPIKey,
			MaxResults:   5,
		}))
	}

	// code_search 工具（代码仓库检索）
	if s.config.Agent.EnableCodeSearch && s.config.Agent.CodeSearchReposDir != "" {
		tools = append(tools, internalTool.NewCodeSearchTool(s.config.Agent.CodeSearchReposDir))
	}

	// repo_manager 工具（仓库 clone/pull/list）
	if s.config.Agent.EnableCodeSearch && s.config.Agent.CodeSearchReposDir != "" {
		tools = append(tools, internalTool.NewRepoManagerTool(s.config.Agent.CodeSearchReposDir))
	}

	// code_graph 工具（代码知识图谱查询）
	if s.config.Agent.EnableCodeGraph && s.codeGraphRepo != nil {
		tools = append(tools, internalTool.NewCodeGraphTool(s.codeGraphRepo, s.codeIndexer))
	}

	// 为所有工具包装参数 schema 自动校验
	// kt 变量已在包装前赋值，LastDocs() 等方法不受影响
	for i, t := range tools {
		if invokable, ok := t.(tool.InvokableTool); ok {
			tools[i] = internalTool.WrapWithValidation(invokable)
		}
	}

	return tools, kt
}

func (s *ChatService) buildRuntimeInstruction(ctx context.Context, req *ChatRequest, sessionID string) string {
	parts := make([]string, 0, 4)

	if req.ForceCitation {
		parts = append(parts,
			"【引用强制模式】必须优先且只能基于选中的知识库/检索结果回答。每个关键结论必须附带来源标注（如 [来源1]）。如果检索结果不足或没有可验证来源，不要输出普通科普答案，必须明确写出：⚠️ 未在当前知识库中找到足够依据，以下内容不是知识库证据支持的回答。",
		)
	}

	if memoryInst := s.buildMemoryInstruction(ctx, req, sessionID); memoryInst != "" {
		parts = append(parts, memoryInst)
	}

	if codeQueryInstruction(req.Message) != "" && req.UseAgent {
		parts = append(parts, codeQueryInstruction(req.Message))
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, "\n\n")
}

func codeQueryInstruction(message string) string {
	lower := strings.ToLower(message)
	keywords := []string{"code", "function", "method", "tool_call", "function_call", "源码", "代码", "函数", "方法", "实现", "调用链", "工具调用"}
	matched := false
	for _, keyword := range keywords {
		if strings.Contains(lower, strings.ToLower(keyword)) {
			matched = true
			break
		}
	}
	if !matched {
		return ""
	}
	return "【代码查询约束】这是代码实现类问题。必须优先使用 code_search，最多执行 4 次 code_search 调用：第 1 次 grep 精确运行时/函数关键词，第 2-3 次 read 最相关文件，第 4 次仅在路径错误时 find 或 grep 纠错。repo 参数填写目标仓库目录名，问题未指明仓库时留空使用默认仓库。除非用户明确问构建或格式化配置，不要优先读取 Makefile、go.mod、package.json、prettier、eslint、配置文件。已有足够文件证据后必须停止调用工具并直接回答。"
}

func (s *ChatService) agenticProjectContextRecoveryInstruction(ctx context.Context, req *ChatRequest) string {
	if decision, ok := s.agenticProjectContextRecoveryDecision(ctx, req); ok {
		return agenticProjectContextRecoveryText(decision.WebSearchEnabled)
	}
	return ""
}

type agenticProjectContextRecoveryDecision struct {
	InitialStatus     string
	InitialCandidates int
	WebSearchEnabled  bool
}

func (s *ChatService) agenticProjectContextRecoveryDecision(ctx context.Context, req *ChatRequest) (agenticProjectContextRecoveryDecision, bool) {
	if req == nil || !req.UseAgent || !pipeline.IsProjectOverviewQuery(req.Message) || s.retriever == nil {
		return agenticProjectContextRecoveryDecision{}, false
	}
	docs, err := s.getRuntimeRetriever(req).Retrieve(ctx, req.Message)
	if err != nil {
		return agenticProjectContextRecoveryDecision{}, false
	}
	gate := pipeline.EvaluateEvidenceGate(req.Message, docs)
	if gate.Status != "insufficient_project_context" {
		return agenticProjectContextRecoveryDecision{}, false
	}
	webSearchEnabled := false
	if s.config != nil {
		webSearchEnabled = s.config.Agent.EnableWebSearch
	}
	return agenticProjectContextRecoveryDecision{InitialStatus: gate.Status, InitialCandidates: gate.CandidateCount, WebSearchEnabled: webSearchEnabled}, true
}

func agenticProjectContextRecoveryText(webSearchEnabled bool) string {
	steps := "1. 先用 knowledge_search 对原问题执行 auto 检索；2. 改用 exact/semantic 模式检索 `README`、`架构`、`项目介绍`、`overview`、`architecture` 以及问题中出现的项目名等关键词；3. 如果仍缺证据，使用 query_decompose 拆成项目定位、核心模块、整体架构等子问题后分别检索；4. 再尝试 HyDE 风格检索：把用户问题改写成一段可能存在于项目说明中的假设文档摘要，并用该摘要关键词检索。"
	if webSearchEnabled {
		steps += "5. 知识库策略全部失败后，才允许使用 web_search 或其他外部工具补充，并在答案中明确标注外部来源不是知识库证据。"
	} else {
		steps += "5. 若未配置外部搜索工具，知识库策略全部失败后进入最终 insufficient_project_context，回答无法基于当前知识库说明项目整体情况。"
	}
	return "【Agentic 项目上下文恢复状态】预检发现这是项目概述/整体架构类问题，且首轮召回缺少有效项目上下文证据。不要直接套用 pipeline 的固定拒答；这在 agentic 模式只是中间状态。必须按顺序尝试恢复：" + steps + " 只有拿到能说明项目定位、模块或架构的证据后才作答；否则输出：⚠️ 当前知识库没有可用于说明项目整体情况的有效项目描述。"
}

func isAgenticProjectContextRecoveryActive(instruction string) bool {
	return strings.Contains(instruction, "【Agentic 项目上下文恢复状态】")
}

func (s *ChatService) runAgenticProjectContextRecovery(ctx context.Context, req *ChatRequest, sessionID, traceID string, trace *traceCollector, decision agenticProjectContextRecoveryDecision) (*ChatResponse, bool) {
	if req == nil || s.retriever == nil {
		return nil, false
	}
	runtimeRetriever := s.getRuntimeRetriever(req)
	if runtimeRetriever == nil {
		return nil, false
	}
	trace.add(TraceStep{Type: "guardrail", Stage: "agentic_project_context_recovery", Summary: "initial retrieval lacks project context; deterministic recovery started", Metadata: map[string]any{"status": decision.InitialStatus, "initial_candidates": decision.InitialCandidates, "terminal": false}})
	queries := agenticProjectContextRecoveryQueries(req.Message)
	seen := make(map[string]struct{}, len(queries))
	for _, attempt := range queries {
		key := attempt.Mode + "\x00" + attempt.Query
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		started := time.Now()
		docs, err := retrieveForRecoveryMode(ctx, runtimeRetriever, attempt.Query, attempt.Mode)
		metadata := map[string]any{"query": attempt.Query, "mode": attempt.Mode, "doc_count": len(docs)}
		if err != nil {
			metadata["error"] = err.Error()
			trace.add(TraceStep{Type: "error", Stage: "agentic_project_context_recovery", Error: err.Error(), LatencyMs: time.Since(started).Milliseconds(), Metadata: metadata})
			continue
		}
		gate := pipeline.EvaluateEvidenceGate(req.Message, docs)
		metadata["gate_status"] = gate.Status
		trace.add(TraceStep{Type: "retrieval", Stage: "agentic_project_context_recovery", LatencyMs: time.Since(started).Milliseconds(), Metadata: metadata})
		if gate.Status == "ok" {
			sources := sourcesFromDocuments(docs, s.config.RAG.TopK)
			answer := summarizeRecoveredProjectContext(sources)
			return &ChatResponse{Answer: answer, Sources: sources, SessionID: sessionID, TraceID: traceID, Trace: trace.snapshot()}, true
		}
	}
	return nil, false
}

type projectContextRecoveryQuery struct {
	Query string
	Mode  string
}

func agenticProjectContextRecoveryQueries(original string) []projectContextRecoveryQuery {
	return []projectContextRecoveryQuery{
		{Query: original, Mode: "auto"},
		{Query: "README 项目介绍 项目概述 overview", Mode: "exact"},
		{Query: "architecture 整体架构 系统架构 核心模块", Mode: "semantic"},
		{Query: "项目定位 核心功能 主要模块 技术栈", Mode: "auto"},
		{Query: "本项目是一个系统，主要功能、核心模块和整体架构如下。", Mode: "semantic"},
	}
}

func retrieveForRecoveryMode(ctx context.Context, runtimeRetriever retriever.Retriever, query, mode string) ([]*schema.Document, error) {
	if mode == "" || mode == retrieval.ModeAuto {
		return runtimeRetriever.Retrieve(ctx, query)
	}
	return runtimeRetriever.Retrieve(ctx, query, retrieval.WithMode(mode))
}

func summarizeRecoveredProjectContext(sources []Source) string {
	if len(sources) == 0 {
		return "⚠️ 当前知识库没有可用于说明项目整体情况的有效项目描述。"
	}
	var sb strings.Builder
	sb.WriteString("基于恢复检索找到的项目上下文证据：")
	for i, source := range sources {
		label := fmt.Sprintf("来源%d", i+1)
		sourceName := metadataStringValue(source.Metadata, "source")
		if sourceName == "" {
			sourceName = metadataStringValue(source.Metadata, "source_filename")
		}
		if sourceName != "" {
			label += " " + sourceName
		}
		sb.WriteString(fmt.Sprintf("\n- [%s] %s", label, truncateText(source.Content, 220)))
	}
	return sb.String()
}

type retrievalScope struct {
	kbSet      map[string]struct{}
	docSet     map[string]struct{}
	restricted bool
}

func (s *ChatService) buildRetrievalScope(req *ChatRequest) retrievalScope {
	scope := retrievalScope{
		kbSet:      make(map[string]struct{}),
		docSet:     make(map[string]struct{}),
		restricted: req.RestrictRetrieval,
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
	return !s.restricted && len(s.kbSet) == 0 && len(s.docSet) == 0
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
	if r.scope.restricted && len(r.scope.kbSet) == 0 && len(r.scope.docSet) == 0 {
		return false
	}
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
	// 注入查询重写器
	pipeOpts = append(pipeOpts, pipeline.WithRewriter(pipeline.NewLLMRewriter(s.chatModel)))
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

func (s *ChatService) buildMemoryInstruction(ctx context.Context, req *ChatRequest, sessionID string) string {
	if !s.config.Memory.Enabled || sessionID == "" || s.messageRepo == nil {
		return ""
	}

	maxChars := s.config.Memory.MaxContextChars
	if maxChars <= 0 {
		maxChars = 5000
	}

	parts := make([]string, 0, 2)

	window := s.config.Memory.WindowSize
	if window <= 0 {
		window = 8
	}

	compacted := false
	if s.crossTurnCompactionEnabled() {
		h, hErr := s.loadSessionHistory(ctx, sessionID)
		if hErr != nil {
			log.Printf("[ChatService] session history unavailable, using window memory (session=%s): %v", sessionID, hErr)
		} else {
			compacted = true
			emitHistoryCompaction(ctx, sessionID, h.Compact)
			if hist := formatSessionHistory(h); hist != "" {
				parts = append(parts, hist)
			}
		}
	}
	if !compacted {
		shortMsgs, err := s.getShortTermMessages(ctx, sessionID, window*2)
		if err == nil && len(shortMsgs) > 0 {
			shortCtx := formatMemoryMessages(shortMsgs, maxChars/2)
			if shortCtx != "" {
				parts = append(parts, "【会话短期记忆】\n"+shortCtx)
			}
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

// buildQueryContext 从近期会话消息提取简短上文摘要，供 Pipeline 查询重写使用。
// 只取最近 2-3 条用户消息的前 ~40 字符连接，不做 LLM 调用。
func (s *ChatService) buildQueryContext(ctx context.Context, req *ChatRequest, sessionID string) string {
	if sessionID == "" || s.messageRepo == nil {
		return ""
	}
	turns := s.config.Agent.QueryContextTurns
	if turns <= 0 {
		turns = 3
	}
	msgs, err := s.messageRepo.ListBySession(ctx, sessionID, turns*2)
	if err != nil || len(msgs) == 0 {
		return ""
	}

	var keywords []string
	seen := map[string]struct{}{}
	recent := msgs
	if len(recent) > turns*2 {
		recent = recent[len(recent)-turns*2:]
	}
	for _, m := range recent {
		if m.Role != "user" {
			continue
		}
		snippet := strings.TrimSpace(m.Content)
		if len([]rune(snippet)) > 40 {
			snippet = string([]rune(snippet)[:40])
		}
		if _, ok := seen[snippet]; ok || snippet == "" {
			continue
		}
		seen[snippet] = struct{}{}
		keywords = append(keywords, snippet)
	}
	if len(keywords) == 0 {
		return ""
	}

	result := "用户刚才在讨论: " + strings.Join(keywords, "、")
	if len([]rune(result)) > 120 {
		result = string([]rune(result)[:120])
	}
	return result
}

// formatMemoryMessages renders the most recent turns oldest-first within a
// character budget. The budget is spent from the newest message backwards, so
// when it runs out it is the oldest turns that are dropped: the latest turn
// is the one a follow-up question refers to.
func formatMemoryMessages(messages []*repository.Message, maxChars int) string {
	const (
		maxMessages     = 12
		perMessageChars = 400
	)
	if maxChars <= 0 {
		maxChars = 1000
	}

	var lines []string // newest first
	used := 0
	for i := len(messages) - 1; i >= 0 && len(lines) < maxMessages; i-- {
		msg := messages[i]
		if msg == nil {
			continue
		}
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		role := "用户"
		if strings.EqualFold(msg.Role, "assistant") {
			role = "助手"
		}
		line := role + ": " + truncateText(content, perMessageChars)
		cost := len([]rune(line)) + 1 // the newline
		if used+cost > maxChars {
			if len(lines) == 0 {
				// Even the newest message alone is over budget: keep its start.
				lines = append(lines, truncateText(line, maxChars))
			}
			break
		}
		lines = append(lines, line)
		used += cost
	}

	for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
		lines[i], lines[j] = lines[j], lines[i]
	}
	return strings.Join(lines, "\n")
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

func appendInstructionPart(instruction, part string) string {
	instruction = strings.TrimSpace(instruction)
	part = strings.TrimSpace(part)
	if part == "" {
		return instruction
	}
	if instruction == "" {
		return part
	}
	return instruction + "\n\n" + part
}

// Source 来源信息
type Source struct {
	Content  string                 `json:"content"`
	DocID    string                 `json:"doc_id"`
	Score    float64                `json:"score,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

func sourceFromDocument(doc *schema.Document) Source {
	if doc == nil {
		return Source{}
	}
	return Source{Content: doc.Content, DocID: doc.ID, Metadata: doc.MetaData}
}

func sourcesFromDocuments(docs []*schema.Document, maxN int) []Source {
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
		sources = append(sources, sourceFromDocument(doc))
		seen[docID] = struct{}{}
		if len(sources) >= maxN {
			break
		}
	}

	return sources
}

// Retrieve returns retrieved documents/pages only; it does not call the chat model, rewrite queries, or synthesize answers.
func (s *ChatService) Retrieve(ctx context.Context, req *RetrieveRequest) (*RetrieveResponse, error) {
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return &RetrieveResponse{Query: req.Query}, nil
	}

	chatReq := &ChatRequest{
		Message:           query,
		KnowledgeBaseIDs:  req.KnowledgeBaseIDs,
		DocumentIDs:       req.DocumentIDs,
		RestrictRetrieval: true,
	}
	runtimeRetriever := s.getRuntimeRetriever(chatReq)
	if runtimeRetriever == nil {
		return &RetrieveResponse{Query: query}, nil
	}

	docs, err := runtimeRetriever.Retrieve(ctx, query)
	if err != nil {
		return nil, err
	}

	maxN := req.TopK
	if maxN <= 0 {
		maxN = s.config.RAG.TopK
	}
	if maxN <= 0 {
		maxN = 5
	}

	results := retrieveResultsFromDocuments(docs, maxN)
	return &RetrieveResponse{Query: query, Total: len(results), Results: results}, nil
}

func retrieveResultsFromDocuments(docs []*schema.Document, maxN int) []RetrieveResult {
	if maxN <= 0 {
		maxN = 5
	}
	results := make([]RetrieveResult, 0, maxN)
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
		results = append(results, retrieveResultFromDocument(doc))
		seen[docID] = struct{}{}
		if len(results) >= maxN {
			break
		}
	}
	return results
}

func retrieveResultFromDocument(doc *schema.Document) RetrieveResult {
	metadata := doc.MetaData
	return RetrieveResult{
		ID:              doc.ID,
		Type:            inferRetrieveResultType(metadata),
		Title:           firstMetadataString(metadata, "wiki_title", "title", "document_name", "filename"),
		Content:         doc.Content,
		Snippet:         truncateText(strings.TrimSpace(doc.Content), 500),
		KnowledgeBaseID: metadataStringValue(metadata, "knowledge_base_id"),
		KnowledgeID:     firstMetadataString(metadata, "knowledge_id", "document_id", "doc_id"),
		WikiPath:        metadataStringValue(metadata, "wiki_path"),
		WikiPageType:    metadataStringValue(metadata, "wiki_page_type"),
		MatchType:       metadataStringValue(metadata, "match_type"),
		Metadata:        metadata,
	}
}

func inferRetrieveResultType(metadata map[string]interface{}) string {
	if metadataStringValue(metadata, "wiki_path") != "" {
		return "wiki_page"
	}
	if metadataStringValue(metadata, "chunk_id") != "" {
		return "document_chunk"
	}
	return "document"
}

func firstMetadataString(metadata map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value := metadataStringValue(metadata, key); value != "" {
			return value
		}
	}
	return ""
}

func metadataStringValue(metadata map[string]interface{}, key string) string {
	if metadata == nil {
		return ""
	}
	value, ok := metadata[key]
	if !ok || value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(fmt.Sprint(value))
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

	return sourcesFromDocuments(docs, maxN)
}

// skillsEnabledForRequest 判断本次请求是否启用 Eino skill 功能。
// 优先采用请求中的 EnableSkills 覆盖值；未设置时回退到全局配置。
func (s *ChatService) skillsEnabledForRequest(req *ChatRequest) bool {
	if req.EnableSkills != nil {
		return *req.EnableSkills
	}
	return s.config.Agent.EnableSkills
}

// renderSystemPrompt 通过 promptManager 渲染指定模式的系统提示词，
// 失败时回退到配置文件中的静态 SystemPrompt。
func (s *ChatService) renderSystemPrompt(mode string) string {
	if s.promptManager == nil {
		return s.config.Agent.SystemPrompt
	}
	result, err := s.promptManager.RenderSystemPrompt(mode, nil)
	if err != nil || strings.TrimSpace(result) == "" {
		return s.config.Agent.SystemPrompt
	}
	return result
}

// chatContext 封装 Chat/ChatStream 共用的请求预处理结果
type chatContext struct {
	sessionID             string
	runtimeInstruction    string
	messageWithInst       string
	runtimeRetriever      retriever.Retriever
	followUpEvidenceCount int
}

// prepareChatContext 执行聊天请求的公共预处理：会话管理、指令构建、检索器构建、保存用户消息
func (s *ChatService) prepareChatContext(ctx context.Context, req *ChatRequest) *chatContext {
	sessionID, _ := s.ensureSession(ctx, req)
	runtimeInstruction := s.buildRuntimeInstruction(ctx, req, sessionID)
	followUpEvidenceCount := 0
	if evidenceInstruction, evidenceSources := s.buildFollowUpEvidenceInstruction(ctx, req, sessionID); evidenceInstruction != "" {
		runtimeInstruction = appendInstructionPart(runtimeInstruction, evidenceInstruction)
		followUpEvidenceCount = len(evidenceSources)
	}
	messageWithInst := appendInstructionToMessage(req.Message, runtimeInstruction)
	runtimeRetriever := s.getRuntimeRetriever(req)
	s.saveUserMessage(ctx, sessionID, req.Message)
	return &chatContext{
		sessionID:             sessionID,
		runtimeInstruction:    runtimeInstruction,
		messageWithInst:       messageWithInst,
		runtimeRetriever:      runtimeRetriever,
		followUpEvidenceCount: followUpEvidenceCount,
	}
}

// agentMaxStepsNotice is what the user gets, in place of an error, when the
// agent has used its tool-step budget.
const agentMaxStepsNotice = "⚠️ Agentic 查询超过最大工具步数，已停止继续调用工具。请缩小问题范围、指定 repo 和目标文件，或先用 code_search 获取候选证据。"

func isAgentMaxStepsError(err error) bool {
	return errors.Is(err, compose.ErrExceedMaxSteps)
}

func addPipelineRetrievalTrace(trace *traceCollector, details pipeline.RetrievalTrace) {
	if trace == nil {
		return
	}
	if len(details.Retrieved) > 0 {
		trace.add(TraceStep{Type: "retrieval", Stage: "retrieved_candidates", Metadata: map[string]any{"chunks": details.Retrieved, "count": len(details.Retrieved)}})
	}
	if len(details.RerankBefore) > 0 || len(details.RerankAfter) > 0 {
		trace.add(TraceStep{Type: "rerank", Stage: "rerank", Metadata: map[string]any{"before": details.RerankBefore, "after": details.RerankAfter}})
	}
	if len(details.Context) > 0 {
		trace.add(TraceStep{Type: "context", Stage: "context_build", Metadata: map[string]any{"chunks": details.Context, "count": len(details.Context)}})
	}
}

func traceStepFromPipelineMetadata(metadata map[string]any) *TraceStep {
	if metadata == nil {
		return nil
	}
	stage, _ := metadata["stage"].(string)
	if stage == "" {
		stage = "trace"
	}
	traceType := "status"
	switch stage {
	case "retrieved_candidates":
		traceType = "retrieval"
	case "rerank":
		traceType = "rerank"
	case "context_build":
		traceType = "context"
	}
	return &TraceStep{Type: traceType, Stage: stage, Metadata: metadata}
}

// Chat 执行聊天
func (s *ChatService) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	startTime := time.Now()
	traceID := logger.TraceIDFrom(ctx)
	trace := newTraceCollector(traceID)
	ctx = trace.context(ctx)
	trace.add(TraceStep{
		Type:  "status",
		Stage: "request",
		Metadata: map[string]any{
			"query":                req.Message,
			"mode":                 req.Mode,
			"use_agent":            req.UseAgent,
			"knowledge_base_ids":   req.KnowledgeBaseIDs,
			"document_ids":         req.DocumentIDs,
			"restrict_retrieval":   req.RestrictRetrieval,
			"enable_long_term_set": req.EnableLongTerm != nil,
		},
	})
	cc := s.prepareChatContext(ctx, req)
	if cc.followUpEvidenceCount > 0 {
		trace.add(TraceStep{Type: "context", Stage: "followup_evidence", Summary: "reused last-turn evidence for follow-up question", Metadata: map[string]any{"source_count": cc.followUpEvidenceCount}})
	}
	cc.runtimeRetriever = newTracedRetriever(cc.runtimeRetriever, trace)

	var resp *ChatResponse
	mode := "pipeline"

	// Agentic 模式
	if req.UseAgent && s.config.Agent.Enabled {
		if recoveryDecision, ok := s.agenticProjectContextRecoveryDecision(ctx, req); ok {
			if recoveryResp, recovered := s.runAgenticProjectContextRecovery(ctx, req, cc.sessionID, traceID, trace, recoveryDecision); recovered {
				resp = recoveryResp
				mode = "agentic_project_context_recovery"
				goto finalizeChat
			}
			cc.runtimeInstruction = appendInstructionPart(cc.runtimeInstruction, agenticProjectContextRecoveryText(recoveryDecision.WebSearchEnabled))
			cc.messageWithInst = appendInstructionToMessage(req.Message, cc.runtimeInstruction)
		}
		mode = "agentic"
		modeStart := time.Now()
		trace.add(TraceStep{Type: "status", Stage: "agent_start", Metadata: map[string]any{"max_steps": s.config.Agent.MaxSteps}})
		runtimeAgent, kt, createErr := s.buildRuntimeAgentForRequest(ctx, cc.runtimeRetriever, req, trace.addEvent)
		if createErr != nil {
			return nil, fmt.Errorf("build runtime agent: %w", createErr)
		}

		messages := []*schema.Message{
			{Role: schema.User, Content: cc.messageWithInst},
		}
		llmCtx, llmCancel := context.WithTimeout(ctx, time.Duration(s.config.Agent.LLMTimeout)*time.Second)
		defer llmCancel()
		respMsg, agentErr := runtimeAgent.agent.Generate(llmCtx, messages)
		if agentErr != nil {
			if isAgentMaxStepsError(agentErr) {
				trace.add(TraceStep{Type: "guardrail", Stage: "agent_max_steps", Error: agentErr.Error(), Metadata: map[string]any{"max_steps": s.config.Agent.MaxSteps}})
				respMsg = &schema.Message{Content: agentMaxStepsNotice}
			} else {
				return nil, fmt.Errorf("agent chat: %w", agentErr)
			}
		}

		answer := ""
		var promptTokens, completionTokens int
		if respMsg != nil {
			answer = respMsg.Content
			if respMsg.ResponseMeta != nil && respMsg.ResponseMeta.Usage != nil {
				promptTokens = respMsg.ResponseMeta.Usage.PromptTokens
				completionTokens = respMsg.ResponseMeta.Usage.CompletionTokens
			}
		}
		answer = filter.StripThinkTags(answer)
		trace.add(TraceStep{
			Type:       "status",
			Stage:      "agent_generate",
			LatencyMs:  time.Since(modeStart).Milliseconds(),
			TokenCount: promptTokens + completionTokens,
			Metadata: map[string]any{
				"prompt_tokens":     promptTokens,
				"completion_tokens": completionTokens,
			},
		})

		// 从 KnowledgeTool 缓存中提取 sources，避免冗余二次检索
		var sources []Source
		if kt != nil {
			for _, doc := range kt.LastDocs() {
				if doc == nil || strings.TrimSpace(doc.ID) == "" {
					continue
				}
				sources = append(sources, sourceFromDocument(doc))
				if len(sources) >= s.config.RAG.TopK {
					break
				}
			}
		}
		logger.FromContext(ctx).Info("chat_agentic",
			"sources", len(sources),
			"duration_ms", time.Since(modeStart).Milliseconds(),
			"prompt_tokens", promptTokens,
			"completion_tokens", completionTokens,
		)

		resp = &ChatResponse{
			Answer:    answer,
			Sources:   sources,
			SessionID: cc.sessionID,
			TraceID:   traceID,
			Trace:     trace.snapshot(),
		}

		// 记录 Prometheus 指标 & 审计日志
		duration := time.Since(modeStart)
		metrics.RecordLLMCall(s.config.LLM.Provider, s.config.LLM.ModelID, mode, duration, promptTokens, completionTokens)
		s.recordLLMAudit(ctx, &repository.LLMAuditLog{
			TraceID:          logger.TraceIDFrom(ctx),
			UserID:           req.UserID,
			SessionID:        cc.sessionID,
			Provider:         s.config.LLM.Provider,
			Model:            s.config.LLM.ModelID,
			Mode:             mode,
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
			LatencyMs:        int(duration.Milliseconds()),
		})
	} else if s.pipeline != nil {
		// Pipeline 模式
		pipelineStart := time.Now()
		trace.add(TraceStep{Type: "status", Stage: "pipeline_start"})
		runtimePipeline := s.pipeline
		if cc.runtimeRetriever != s.retriever {
			runtimePipeline = s.buildRuntimePipeline(cc.runtimeRetriever)
		}

		rewriteCtx := ctx
		if qc := s.buildQueryContext(ctx, req, cc.sessionID); qc != "" {
			rewriteCtx = pipeline.WithQueryHistory(ctx, qc)
		}

		pipeResp, pipeErr := runtimePipeline.Run(rewriteCtx, &pipeline.RAGRequest{
			Query:                 req.Message,
			SessionID:             cc.sessionID,
			GenerationInstruction: cc.runtimeInstruction,
		})
		if pipeErr != nil {
			return nil, fmt.Errorf("pipeline run: %w", pipeErr)
		}

		if pipeResp.RewriteQ != "" {
			trace.add(TraceStep{Type: "rewrite", Stage: "rewrite", Content: pipeResp.RewriteQ})
		}
		trace.add(TraceStep{Type: "status", Stage: "pipeline_run", LatencyMs: time.Since(pipelineStart).Milliseconds(), Metadata: map[string]any{"source_count": len(pipeResp.Sources)}})

		addPipelineRetrievalTrace(trace, pipeResp.Trace)

		pipeResp.Answer = filter.StripThinkTags(pipeResp.Answer)

		sources := make([]Source, len(pipeResp.Sources))
		for i, src := range pipeResp.Sources {
			sources[i] = Source{
				Content:  src.Content,
				DocID:    src.DocID,
				Score:    src.Score,
				Metadata: src.Metadata,
			}
			trace.add(TraceStep{Type: "source", Stage: "source", Content: src.Content, DocID: src.DocID, Metadata: src.Metadata})
		}

		resp = &ChatResponse{
			Answer:          pipeResp.Answer,
			Sources:         sources,
			SessionID:       cc.sessionID,
			TraceID:         traceID,
			Trace:           trace.snapshot(),
			GroundingStatus: pipeResp.Metadata[pipeline.MetadataKeyGroundingStatus],
		}

		// 记录 Prometheus 指标 & 审计日志（pipeline 无 token usage，记 0）
		duration := time.Since(startTime)
		metrics.RecordLLMCall(s.config.LLM.Provider, s.config.LLM.ModelID, mode, duration, 0, 0)
		s.recordLLMAudit(ctx, &repository.LLMAuditLog{
			TraceID:   logger.TraceIDFrom(ctx),
			UserID:    req.UserID,
			SessionID: cc.sessionID,
			Provider:  s.config.LLM.Provider,
			Model:     s.config.LLM.ModelID,
			Mode:      mode,
			LatencyMs: int(duration.Milliseconds()),
		})
	} else {
		return nil, fmt.Errorf("no chat handler available")
	}

finalizeChat:
	// 保存助手消息
	latencyMs := time.Since(startTime).Milliseconds()
	trace.add(TraceStep{Type: "status", Stage: "complete", LatencyMs: latencyMs, Metadata: map[string]any{"mode": mode, "source_count": len(resp.Sources)}})
	resp.Trace = trace.snapshot()
	messageID := s.saveAssistantMessageWithTrace(ctx, cc.sessionID, resp.Answer, 0, latencyMs, resp.Trace, resp.Sources)
	s.recordRequestTrace(traceID, req, cc.sessionID, messageID, mode, "completed", latencyMs, resp.Trace, trace.summary(mode, "completed", latencyMs, len(resp.Sources), ""), "")

	return resp, nil
}

// ChatStream 流式聊天
func (s *ChatService) ChatStream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent, 100)
	traceID := logger.TraceIDFrom(ctx)
	trace := newTraceCollector(traceID)
	ctx = trace.context(ctx)
	cc := s.prepareChatContext(ctx, req)
	if cc.followUpEvidenceCount > 0 {
		trace.add(TraceStep{Type: "context", Stage: "followup_evidence", Summary: "reused last-turn evidence for follow-up question", Metadata: map[string]any{"source_count": cc.followUpEvidenceCount}})
	}
	trace.add(TraceStep{
		Type:  "status",
		Stage: "request",
		Metadata: map[string]any{
			"query":                req.Message,
			"mode":                 req.Mode,
			"use_agent":            req.UseAgent,
			"knowledge_base_ids":   req.KnowledgeBaseIDs,
			"document_ids":         req.DocumentIDs,
			"restrict_retrieval":   req.RestrictRetrieval,
			"enable_long_term_set": req.EnableLongTerm != nil,
		},
	})
	cc.runtimeRetriever = newTracedRetriever(cc.runtimeRetriever, trace)

	// trySend 向 channel 发送事件，如果 ctx 已取消则放弃，防止 goroutine 泄露
	trySend := func(ev StreamEvent) bool {
		if ev.TraceID == "" {
			ev.TraceID = traceID
		}
		if ev.TraceStep == nil && (ev.Type == "status" || ev.Type == "rewrite" || ev.Type == "source" || ev.Type == "action" || ev.Type == "observation" || ev.Type == "error") {
			step := TraceStep{Type: ev.Type, Stage: ev.Type, Content: ev.Content, ToolName: ev.ToolName, ToolInput: ev.ToolInput, DocID: ev.DocID, LatencyMs: ev.LatencyMs, Error: ev.Error}
			ev.TraceStep = &step
		}
		if ev.TraceStep != nil {
			step := trace.add(*ev.TraceStep)
			ev.TraceStep = &step
		}
		select {
		case ch <- ev:
			return true
		case <-ctx.Done():
			return false
		}
	}

	// 发送 session_id 事件（首个事件）
	if cc.sessionID != "" {
		ch <- StreamEvent{Type: "session_id", SessionID: cc.sessionID, TraceID: traceID}
	}

	go func() {
		defer close(ch)

		startTime := time.Now()
		firstTokenLogged := false
		var fullResponse strings.Builder

		// 创建流式 think 标签过滤器
		thinkFilter := filter.NewThinkTagStreamFilter()
		finishError := func(mode, stage, errText string) {
			latencyMs := time.Since(startTime).Milliseconds()
			trySend(StreamEvent{Type: "error", Error: errText, TraceStep: &TraceStep{Type: "error", Stage: stage, Error: errText, Summary: errText, LatencyMs: latencyMs}})
			snapshot := trace.snapshot()
			s.recordRequestTrace(traceID, req, cc.sessionID, "", mode, "error", latencyMs, snapshot, trace.summary(mode, "error", latencyMs, 0, errText), errText)
			trySend(StreamEvent{Type: "trace_snapshot", TraceSnapshot: snapshot})
		}

		// Agentic 模式
		if req.UseAgent && s.config.Agent.Enabled {
			mode := "agentic"
			if recoveryDecision, ok := s.agenticProjectContextRecoveryDecision(ctx, req); ok {
				recoveryInstruction := agenticProjectContextRecoveryText(recoveryDecision.WebSearchEnabled)
				trace.add(TraceStep{Type: "guardrail", Stage: "agentic_project_context_recovery", Summary: "initial retrieval lacks project context; agentic recovery enabled", Metadata: map[string]any{"status": recoveryDecision.InitialStatus, "initial_candidates": recoveryDecision.InitialCandidates, "terminal": false}})
				cc.runtimeInstruction = appendInstructionPart(cc.runtimeInstruction, recoveryInstruction)
				cc.messageWithInst = appendInstructionToMessage(req.Message, cc.runtimeInstruction)
			}
			trace.add(TraceStep{Type: "status", Stage: "agent_start", Metadata: map[string]any{"max_steps": s.config.Agent.MaxSteps}})
			eventSink := func(ev StreamEvent) {
				trace.addEvent(ev)
				trySend(ev)
			}
			runtimeAgent, kt, createErr := s.buildRuntimeAgentForRequest(ctx, cc.runtimeRetriever, req, eventSink)
			if createErr != nil {
				finishError(mode, "agent_create", createErr.Error())
				return
			}

			messages := []*schema.Message{
				{Role: schema.User, Content: cc.messageWithInst},
			}
			// Stream the run. Tools that only implement InvokableTool are fine: the
			// event wrappers declare only what their base can do, and Eino turns an
			// invokable tool into a streaming one itself. Action/observation events
			// fire via eventSink during tool execution, the answer text as it is written.
			llmCtx, llmCancel := context.WithTimeout(ctx, time.Duration(s.config.Agent.LLMTimeout)*time.Second)
			var sources []Source
			sourcesSent := -1
			sendSources := func() bool {
				if kt == nil {
					return true
				}
				sources = sourcesFromDocuments(kt.LastDocs(), s.config.RAG.TopK)
				if len(sources) == sourcesSent || (len(sources) == 0 && sourcesSent < 0) {
					return true
				}
				sourcesSent = len(sources)
				return trySend(StreamEvent{Type: "sources", Sources: sources, SourceCount: len(sources)})
			}
			started := false
			emit := func(delta string) bool {
				out := thinkFilter.Filter(delta)
				if !started {
					out = strings.TrimLeft(out, " \t\r\n")
				}
				if out == "" {
					return true
				}
				if !started {
					started = true
					// Sources go out before the first words of the answer, as in pipeline mode.
					if !sendSources() {
						return false
					}
					log.Printf("[Timing][ChatService] mode=agentic stage=first_token duration_ms=%d", time.Since(startTime).Milliseconds())
				}
				fullResponse.WriteString(out)
				return trySend(StreamEvent{Type: "content", Content: out})
			}
			respMsg, agentErr := streamAgent(llmCtx, runtimeAgent, messages, emit)
			llmCancel()
			if errors.Is(agentErr, errStreamConsumerGone) {
				return
			}
			maxSteps := isAgentMaxStepsError(agentErr)
			if agentErr != nil && !maxSteps {
				finishError(mode, "agent_stream", agentErr.Error())
				return
			}
			// Text that bypasses emit's per-delta path still goes out sources-first.
			writeTail := func(out string) bool {
				if !started {
					if !sendSources() {
						return false
					}
					started = true
				}
				fullResponse.WriteString(out)
				return trySend(StreamEvent{Type: "content", Content: out})
			}
			if rest := thinkFilter.Flush(); rest != "" {
				if !started {
					rest = strings.TrimLeft(rest, " \t\r\n")
				}
				if rest != "" && !writeTail(rest) {
					return
				}
			}
			if maxSteps {
				// Same outcome as the non-streaming path: what was written stays and
				// the notice follows it, instead of an error that discards both.
				trace.add(TraceStep{Type: "guardrail", Stage: "agent_max_steps", Error: agentErr.Error(), Metadata: map[string]any{"max_steps": s.config.Agent.MaxSteps}})
				notice := agentMaxStepsNotice
				if started {
					notice = "\n\n" + notice
				}
				if !writeTail(notice) {
					return
				}
				respMsg = nil
			}
			// Tools that ran after the answer had started add sources the early event lacked.
			if !sendSources() {
				return
			}
			trace.add(TraceStep{Type: "status", Stage: "agent_generate", LatencyMs: time.Since(startTime).Milliseconds()})

			// 记录流式 agentic 模式的指标和审计日志
			duration := time.Since(startTime)
			var promptTokens, completionTokens int
			if respMsg != nil && respMsg.ResponseMeta != nil && respMsg.ResponseMeta.Usage != nil {
				promptTokens = respMsg.ResponseMeta.Usage.PromptTokens
				completionTokens = respMsg.ResponseMeta.Usage.CompletionTokens
			}
			metrics.RecordLLMCall(s.config.LLM.Provider, s.config.LLM.ModelID, "agentic", duration, promptTokens, completionTokens)
			s.recordLLMAudit(ctx, &repository.LLMAuditLog{
				TraceID:          logger.TraceIDFrom(ctx),
				UserID:           req.UserID,
				SessionID:        cc.sessionID,
				Provider:         s.config.LLM.Provider,
				Model:            s.config.LLM.ModelID,
				Mode:             "agentic",
				PromptTokens:     promptTokens,
				CompletionTokens: completionTokens,
				TotalTokens:      promptTokens + completionTokens,
				LatencyMs:        int(duration.Milliseconds()),
			})

			latencyMs := time.Since(startTime).Milliseconds()
			trace.add(TraceStep{Type: "status", Stage: "complete", LatencyMs: latencyMs, Metadata: map[string]any{"mode": mode}})
			snapshot := trace.snapshot()
			messageID := s.saveAssistantMessageWithTrace(ctx, cc.sessionID, strings.TrimSpace(fullResponse.String()), 0, latencyMs, snapshot, sources)
			s.recordRequestTrace(traceID, req, cc.sessionID, messageID, mode, "completed", latencyMs, snapshot, trace.summary(mode, "completed", latencyMs, len(sources), ""), "")
			trySend(StreamEvent{Type: "trace_snapshot", TraceSnapshot: snapshot})
			trySend(StreamEvent{Type: "done"})
			return
		}

		// Pipeline 模式
		if s.pipeline != nil {
			trace.add(TraceStep{Type: "status", Stage: "pipeline_start"})
			runtimePipeline := s.pipeline
			if cc.runtimeRetriever != s.retriever {
				runtimePipeline = s.buildRuntimePipeline(cc.runtimeRetriever)
			}

			rewriteCtx := ctx
			if qc := s.buildQueryContext(ctx, req, cc.sessionID); qc != "" {
				rewriteCtx = pipeline.WithQueryHistory(ctx, qc)
			}

			stream, err := runtimePipeline.RunStream(rewriteCtx, &pipeline.RAGRequest{
				Query:                 req.Message,
				SessionID:             cc.sessionID,
				GenerationInstruction: cc.runtimeInstruction,
			})
			if err != nil {
				finishError("pipeline", "pipeline_stream", err.Error())
				return
			}

			sources := make([]Source, 0, s.config.RAG.TopK)
			pipelineDone := false
			for chunk := range stream {
				if chunk.Type == pipeline.ChunkTypeDone {
					pipelineDone = true
					continue
				}
				if chunk.Type == pipeline.ChunkTypeContent {
					filtered := thinkFilter.Filter(chunk.Content)
					if filtered != "" {
						if !firstTokenLogged {
							log.Printf("[Timing][ChatService] mode=pipeline stage=first_token duration_ms=%d", time.Since(startTime).Milliseconds())
							firstTokenLogged = true
						}
						fullResponse.WriteString(filtered)
						if !trySend(StreamEvent{
							Type:    string(chunk.Type),
							Content: filtered,
							DocID:   chunk.DocID,
						}) {
							return
						}
					}
				} else {
					if chunk.Type == pipeline.ChunkTypeError {
						finishError("pipeline", "pipeline_run", chunk.Content)
						return
					}
					if chunk.Type == pipeline.ChunkTypeSource {
						sources = append(sources, Source{Content: chunk.Content, DocID: chunk.DocID, Score: chunk.Score, Metadata: chunk.Metadata})
					}
					event := StreamEvent{
						Type:     string(chunk.Type),
						Content:  chunk.Content,
						DocID:    chunk.DocID,
						Metadata: chunk.Metadata,
					}
					if chunk.Type == pipeline.ChunkTypeTrace {
						event.TraceStep = traceStepFromPipelineMetadata(chunk.Metadata)
					}
					if !trySend(event) {
						return
					}
				}
			}
			// 刷新过滤器缓冲区
			if remaining := thinkFilter.Flush(); remaining != "" {
				fullResponse.WriteString(remaining)
				trySend(StreamEvent{Type: "content", Content: remaining})
			}

			latencyMs := time.Since(startTime).Milliseconds()
			trace.add(TraceStep{Type: "status", Stage: "complete", LatencyMs: latencyMs, Metadata: map[string]any{"mode": "pipeline"}})
			snapshot := trace.snapshot()
			messageID := s.saveAssistantMessageWithTrace(ctx, cc.sessionID, fullResponse.String(), 0, latencyMs, snapshot, sources)
			s.recordRequestTrace(traceID, req, cc.sessionID, messageID, "pipeline", "completed", latencyMs, snapshot, trace.summary("pipeline", "completed", latencyMs, 0, ""), "")
			trySend(StreamEvent{Type: "trace_snapshot", TraceSnapshot: snapshot})
			if pipelineDone {
				trySend(StreamEvent{Type: string(pipeline.ChunkTypeDone)})
			}
			return
		}

		finishError("pipeline", "handler", "no handler available")
	}()

	return ch, nil
}
