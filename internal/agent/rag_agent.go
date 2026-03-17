// Package agent 实现基于 Eino 的 RAG Agent
//
// 【Eino 特点】使用 Eino 的 ReAct Agent 实现带工具调用的智能体
// 主要特性：
// - 基于 ReAct 模式的推理-行动循环
// - 支持动态工具注册
// - 内置流式输出
// - 完善的回调机制
package agent

import (
	"context"
	"fmt"
	"io"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
)

// RAGAgent RAG 智能体
// 【Eino 特点】封装 Eino ReAct Agent，支持 RAG 增强的对话
type RAGAgent struct {
	config     *AgentConfig
	chatModel  model.ChatModel
	tools      []tool.BaseTool
	reactAgent *react.Agent
}

// AgentConfig Agent 配置
type AgentConfig struct {
	// 模型配置
	ModelID     string
	Temperature float64
	MaxTokens   int

	// Agent 配置
	SystemPrompt string
	MaxSteps     int // 最大推理步数

	// RAG 配置
	EnableRAG bool
	TopK      int
}

// DefaultAgentConfig 返回默认配置
func DefaultAgentConfig() *AgentConfig {
	return &AgentConfig{
		ModelID:     "gpt-4o-mini",
		Temperature: 0.7,
		MaxTokens:   4096,
		SystemPrompt: `你是一个智能助手，可以使用工具来帮助用户解决问题。
当用户询问知识性问题时，优先使用知识库检索工具。
当需要实时信息时，使用网络搜索工具。`,
		MaxSteps:  10,
		EnableRAG: true,
		TopK:      5,
	}
}

// NewRAGAgent 创建 RAG Agent
func NewRAGAgent(cfg *AgentConfig, chatModel model.ChatModel, tools []tool.BaseTool) (*RAGAgent, error) {
	if cfg == nil {
		cfg = DefaultAgentConfig()
	}

	agent := &RAGAgent{
		config:    cfg,
		chatModel: chatModel,
		tools:     tools,
	}

	// 【Eino 特点】使用 Eino 的 react.NewAgent 创建 ReAct Agent
	if err := agent.initReactAgent(); err != nil {
		return nil, fmt.Errorf("init react agent: %w", err)
	}

	return agent, nil
}

// initReactAgent 初始化 ReAct Agent
func (a *RAGAgent) initReactAgent() error {
	if a.chatModel == nil {
		return fmt.Errorf("chat model is required")
	}

	// 【Eino 特点】配置 ReAct Agent
	// 使用 ToolsConfig 而非直接的 Tools 字段
	config := &react.AgentConfig{
		Model: a.chatModel,
		ToolsConfig: compose.ToolsNodeConfig{
			Tools: a.tools,
		},
		MaxStep: a.config.MaxSteps,
	}

	agent, err := react.NewAgent(context.Background(), config)
	if err != nil {
		return err
	}

	a.reactAgent = agent
	return nil
}

// Chat 执行对话
func (a *RAGAgent) Chat(ctx context.Context, message string) (string, error) {
	if a.reactAgent == nil {
		return "", fmt.Errorf("agent not initialized")
	}

	// 构建输入消息
	messages := []*schema.Message{
		{Role: schema.System, Content: a.config.SystemPrompt},
		{Role: schema.User, Content: message},
	}

	// 【Eino 特点】调用 Agent 的 Generate 方法
	resp, err := a.reactAgent.Generate(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("generate: %w", err)
	}

	return resp.Content, nil
}

// ChatStream 流式对话
// 【Eino 特点】使用 Eino 的流式 API
func (a *RAGAgent) ChatStream(ctx context.Context, message string) (<-chan StreamEvent, error) {
	if a.reactAgent == nil {
		return nil, fmt.Errorf("agent not initialized")
	}

	ch := make(chan StreamEvent, 100)

	go func() {
		defer close(ch)

		// 构建输入消息
		messages := []*schema.Message{
			{Role: schema.System, Content: a.config.SystemPrompt},
			{Role: schema.User, Content: message},
		}

		// 【Eino 特点】调用 Agent 的 Stream 方法
		reader, err := a.reactAgent.Stream(ctx, messages)
		if err != nil {
			ch <- StreamEvent{Type: EventTypeError, Error: err}
			return
		}
		defer reader.Close()

		for {
			chunk, err := reader.Recv()
			if err != nil {
				if err != io.EOF {
					ch <- StreamEvent{Type: EventTypeError, Error: err}
				}
				break
			}

			ch <- StreamEvent{
				Type:    EventTypeContent,
				Content: chunk.Content,
			}
		}

		ch <- StreamEvent{Type: EventTypeDone}
	}()

	return ch, nil
}

// ChatWithHistory 带历史记录的对话
func (a *RAGAgent) ChatWithHistory(ctx context.Context, message string, history []*schema.Message) (string, error) {
	if a.reactAgent == nil {
		return "", fmt.Errorf("agent not initialized")
	}

	// 构建消息列表
	messages := make([]*schema.Message, 0, len(history)+2)
	messages = append(messages, &schema.Message{Role: schema.System, Content: a.config.SystemPrompt})
	messages = append(messages, history...)
	messages = append(messages, &schema.Message{Role: schema.User, Content: message})

	resp, err := a.reactAgent.Generate(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("generate: %w", err)
	}

	return resp.Content, nil
}

// AddTool 动态添加工具
func (a *RAGAgent) AddTool(t tool.BaseTool) error {
	a.tools = append(a.tools, t)
	return a.initReactAgent() // 重新初始化以应用新工具
}

// StreamEvent 流式事件
type StreamEvent struct {
	Type    EventType
	Content string
	ToolUse *ToolUseEvent // 工具使用事件
	Error   error
}

// EventType 事件类型
type EventType string

const (
	EventTypeContent  EventType = "content"   // 内容输出
	EventTypeToolUse  EventType = "tool_use"  // 工具调用
	EventTypeToolDone EventType = "tool_done" // 工具完成
	EventTypeDone     EventType = "done"      // 完成
	EventTypeError    EventType = "error"     // 错误
)

// ToolUseEvent 工具使用事件
type ToolUseEvent struct {
	ToolName string
	Input    string
	Output   string
}

// GraphAgent 基于 Graph 的 Agent
// 【Eino 特点】使用 Eino Graph 实现更灵活的 Agent 流程
type GraphAgent struct {
	graph *compose.Graph[*AgentState, *AgentState]
}

// AgentState Agent 状态
type AgentState struct {
	Messages    []*schema.Message
	ToolResults []string
	FinalAnswer string
	Step        int
}

// NewGraphAgent 创建 Graph Agent
// 【Eino 特点】演示如何使用 Eino Graph 构建自定义 Agent 流程
func NewGraphAgent(chatModel model.ChatModel, tools []tool.BaseTool) (*GraphAgent, error) {
	// 创建 Graph
	graph := compose.NewGraph[*AgentState, *AgentState]()

	// 添加节点
	// think: 推理节点
	// act: 行动节点（工具调用）
	// respond: 响应节点

	// TODO: 完善 Graph 构建逻辑
	// 这里展示基本结构，实际实现需要根据 Eino API 调整

	if _, err := graph.Compile(context.Background()); err != nil {
		return nil, err
	}

	return &GraphAgent{graph: graph}, nil
}
