// Package chatpipeline Chat Pipeline 插件机制
//
// 【Eino 特点】参考 WeKnora 的插件架构
// 提供可插拔的聊天处理流水线
package chatpipeline

import (
	"context"
	"fmt"
	"sync"
	"time"

	"eino_agent/internal/container"
	"eino_agent/internal/prompt"
)

// EventType 事件类型
type EventType string

const (
	// 检索相关事件
	EventBeforeRewrite   EventType = "before_rewrite"   // 查询重写前
	EventAfterRewrite    EventType = "after_rewrite"    // 查询重写后
	EventBeforeSearch    EventType = "before_search"    // 检索前
	EventAfterSearch     EventType = "after_search"     // 检索后
	EventBeforeRerank    EventType = "before_rerank"    // 重排序前
	EventAfterRerank     EventType = "after_rerank"     // 重排序后

	// 生成相关事件
	EventBeforeGenerate  EventType = "before_generate"  // 生成前
	EventAfterGenerate   EventType = "after_generate"   // 生成后
	EventStreamChunk     EventType = "stream_chunk"     // 流式输出块

	// 错误和完成事件
	EventError           EventType = "error"            // 发生错误
	EventComplete        EventType = "complete"         // 处理完成
)

// ChatContext 聊天上下文
type ChatContext struct {
	// 请求信息
	SessionID   string
	MessageID   string
	Query       string
	RewrittenQuery string // 重写后的查询

	// 检索结果
	SearchResults []*container.Document
	RerankResults []*container.Document
	FinalContexts []prompt.DocumentContext

	// 生成结果
	SystemPrompt  string
	UserPrompt    string
	Response      string
	StreamChunks  []string

	// 元数据
	Metadata      map[string]interface{}
	StartTime     time.Time
	Duration      time.Duration

	// 错误信息
	Error         error
}

// NewChatContext 创建聊天上下文
func NewChatContext(sessionID, messageID, query string) *ChatContext {
	return &ChatContext{
		SessionID:   sessionID,
		MessageID:   messageID,
		Query:       query,
		Metadata:    make(map[string]interface{}),
		StartTime:   time.Now(),
		StreamChunks: make([]string, 0),
	}
}

// Plugin 插件接口
type Plugin interface {
	// Name 插件名称
	Name() string
	// ActivationEvents 激活事件
	ActivationEvents() []EventType
	// OnEvent 事件处理
	OnEvent(ctx context.Context, event EventType, chatCtx *ChatContext, next func() error) error
}

// PluginError 插件错误
type PluginError struct {
	Plugin      string
	Event       EventType
	Description string
	Err         error
}

func (e *PluginError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Plugin, e.Description, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Plugin, e.Description)
}

// Pipeline 聊天流水线
type Pipeline struct {
	mu       sync.RWMutex
	plugins  map[EventType][]Plugin
	handlers map[EventType]func(context.Context, *ChatContext) error
}

// NewPipeline 创建流水线
func NewPipeline() *Pipeline {
	return &Pipeline{
		plugins:  make(map[EventType][]Plugin),
		handlers: make(map[EventType]func(context.Context, *ChatContext) error),
	}
}

// Register 注册插件
func (p *Pipeline) Register(plugin Plugin) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, event := range plugin.ActivationEvents() {
		p.plugins[event] = append(p.plugins[event], plugin)
		p.handlers[event] = p.buildHandler(p.plugins[event])
	}
}

// buildHandler 构建处理链
func (p *Pipeline) buildHandler(plugins []Plugin) func(context.Context, *ChatContext) error {
	// 从最后一个插件开始构建链
	next := func(_ context.Context, _ *ChatContext) error { return nil }

	for i := len(plugins) - 1; i >= 0; i-- {
		current := plugins[i]
		prevNext := next
		next = func(ctx context.Context, chatCtx *ChatContext) error {
			event := chatCtx.Metadata["current_event"].(EventType)
			return current.OnEvent(ctx, event, chatCtx, func() error {
				return prevNext(ctx, chatCtx)
			})
		}
	}

	return next
}

// Trigger 触发事件
func (p *Pipeline) Trigger(ctx context.Context, event EventType, chatCtx *ChatContext) error {
	p.mu.RLock()
	handler, ok := p.handlers[event]
	p.mu.RUnlock()

	if !ok {
		return nil
	}

	// 设置当前事件
	chatCtx.Metadata["current_event"] = event

	return handler(ctx, chatCtx)
}

// TracingPlugin 追踪插件（记录每个阶段的执行时间）
type TracingPlugin struct {
	timings map[string]time.Duration
	mu      sync.Mutex
}

func NewTracingPlugin() *TracingPlugin {
	return &TracingPlugin{
		timings: make(map[string]time.Duration),
	}
}

func (p *TracingPlugin) Name() string {
	return "tracing"
}

func (p *TracingPlugin) ActivationEvents() []EventType {
	return []EventType{
		EventBeforeRewrite, EventAfterRewrite,
		EventBeforeSearch, EventAfterSearch,
		EventBeforeRerank, EventAfterRerank,
		EventBeforeGenerate, EventAfterGenerate,
	}
}

func (p *TracingPlugin) OnEvent(ctx context.Context, event EventType, chatCtx *ChatContext, next func() error) error {
	start := time.Now()
	err := next()
	duration := time.Since(start)

	p.mu.Lock()
	p.timings[string(event)] = duration
	p.mu.Unlock()

	// 记录到元数据
	chatCtx.Metadata[fmt.Sprintf("timing_%s", event)] = duration.String()

	return err
}

func (p *TracingPlugin) GetTimings() map[string]time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()

	result := make(map[string]time.Duration)
	for k, v := range p.timings {
		result[k] = v
	}
	return result
}

// LoggingPlugin 日志插件
type LoggingPlugin struct {
	logger func(format string, args ...interface{})
}

func NewLoggingPlugin(logger func(format string, args ...interface{})) *LoggingPlugin {
	if logger == nil {
		logger = func(format string, args ...interface{}) {
			fmt.Printf("[Pipeline] "+format+"\n", args...)
		}
	}
	return &LoggingPlugin{logger: logger}
}

func (p *LoggingPlugin) Name() string {
	return "logging"
}

func (p *LoggingPlugin) ActivationEvents() []EventType {
	return []EventType{
		EventBeforeRewrite, EventAfterRewrite,
		EventBeforeSearch, EventAfterSearch,
		EventBeforeRerank, EventAfterRerank,
		EventBeforeGenerate, EventAfterGenerate,
		EventStreamChunk, EventError, EventComplete,
	}
}

func (p *LoggingPlugin) OnEvent(ctx context.Context, event EventType, chatCtx *ChatContext, next func() error) error {
	p.logger("Event: %s, SessionID: %s, Query: %s", event, chatCtx.SessionID, truncate(chatCtx.Query, 50))

	err := next()

	if err != nil {
		p.logger("Event: %s completed with error: %v", event, err)
	}

	return err
}

// FilterTopKPlugin Top-K 过滤插件
type FilterTopKPlugin struct {
	topK int
}

func NewFilterTopKPlugin(topK int) *FilterTopKPlugin {
	return &FilterTopKPlugin{topK: topK}
}

func (p *FilterTopKPlugin) Name() string {
	return "filter_top_k"
}

func (p *FilterTopKPlugin) ActivationEvents() []EventType {
	return []EventType{EventAfterSearch, EventAfterRerank}
}

func (p *FilterTopKPlugin) OnEvent(ctx context.Context, event EventType, chatCtx *ChatContext, next func() error) error {
	// 先执行后续插件
	if err := next(); err != nil {
		return err
	}

	// 过滤结果
	switch event {
	case EventAfterSearch:
		if len(chatCtx.SearchResults) > p.topK {
			chatCtx.SearchResults = chatCtx.SearchResults[:p.topK]
		}
	case EventAfterRerank:
		if len(chatCtx.RerankResults) > p.topK {
			chatCtx.RerankResults = chatCtx.RerankResults[:p.topK]
		}
	}

	return nil
}

// ScoreFilterPlugin 分数过滤插件
type ScoreFilterPlugin struct {
	threshold float64
}

func NewScoreFilterPlugin(threshold float64) *ScoreFilterPlugin {
	return &ScoreFilterPlugin{threshold: threshold}
}

func (p *ScoreFilterPlugin) Name() string {
	return "score_filter"
}

func (p *ScoreFilterPlugin) ActivationEvents() []EventType {
	return []EventType{EventAfterSearch, EventAfterRerank}
}

func (p *ScoreFilterPlugin) OnEvent(ctx context.Context, event EventType, chatCtx *ChatContext, next func() error) error {
	if err := next(); err != nil {
		return err
	}

	// 过滤低分结果
	var filterDocs = func(docs []*container.Document) []*container.Document {
		filtered := make([]*container.Document, 0)
		for _, doc := range docs {
			if doc.Score >= p.threshold {
				filtered = append(filtered, doc)
			}
		}
		return filtered
	}

	switch event {
	case EventAfterSearch:
		chatCtx.SearchResults = filterDocs(chatCtx.SearchResults)
	case EventAfterRerank:
		chatCtx.RerankResults = filterDocs(chatCtx.RerankResults)
	}

	return nil
}

// 辅助函数
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}
