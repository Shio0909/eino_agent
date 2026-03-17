// Package memory 实现对话记忆管理
//
// 【差异化亮点】参考 AGGO 的 MemoryManager 设计，提供：
// - 短期记忆（会话内的最近 N 轮对话）
// - 长期记忆（跨会话的关键信息持久化）
// - 自动摘要（超过窗口长度时 LLM 自动总结）
//
// 与 WeKnora 手动管理对话历史不同，Memory 系统自动管理
// 上下文窗口，避免 token 溢出，并通过 LLM 提取长期记忆。
package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// Manager 记忆管理器
// 管理短期记忆（滑动窗口）和长期记忆（LLM 提取摘要）
type Manager struct {
	mu        sync.RWMutex
	chatModel model.ChatModel

	// 短期记忆：每个 session 的最近对话
	sessions map[string]*SessionMemory

	// 长期记忆：跨 session 的用户偏好/事实
	longTerm map[string]*LongTermMemory

	config ManagerConfig
}

// ManagerConfig 记忆管理器配置
type ManagerConfig struct {
	// 短期记忆：滑动窗口大小（保留最近 N 轮对话）
	WindowSize int `yaml:"window_size"`

	// 触发自动摘要的消息数阈值
	SummaryThreshold int `yaml:"summary_threshold"`

	// 长期记忆：最大条目数
	MaxLongTermEntries int `yaml:"max_long_term_entries"`

	// 是否启用自动摘要
	EnableAutoSummary bool `yaml:"enable_auto_summary"`

	// 是否启用长期记忆提取
	EnableLongTerm bool `yaml:"enable_long_term"`
}

// DefaultConfig 默认配置
func DefaultConfig() ManagerConfig {
	return ManagerConfig{
		WindowSize:         20,
		SummaryThreshold:   30,
		MaxLongTermEntries: 100,
		EnableAutoSummary:  true,
		EnableLongTerm:     true,
	}
}

// SessionMemory 会话记忆（短期）
type SessionMemory struct {
	SessionID string
	Messages  []*schema.Message // 完整消息记录
	Summary   string            // 超出窗口的历史摘要
	CreatedAt time.Time
	UpdatedAt time.Time
}

// LongTermMemory 长期记忆
type LongTermMemory struct {
	UserID  string
	Entries []MemoryEntry
}

// MemoryEntry 长期记忆条目
type MemoryEntry struct {
	Key       string    `json:"key"`       // 记忆键（如 "用户偏好"、"重要事实"）
	Content   string    `json:"content"`   // 记忆内容
	Source    string    `json:"source"`    // 来源 session
	CreatedAt time.Time `json:"created_at"`
}

// NewManager 创建记忆管理器
func NewManager(chatModel model.ChatModel, config ManagerConfig) *Manager {
	return &Manager{
		chatModel: chatModel,
		sessions:  make(map[string]*SessionMemory),
		longTerm:  make(map[string]*LongTermMemory),
		config:    config,
	}
}

// GetMessages 获取会话的上下文消息（包含摘要 + 最近消息）
//
// 返回值可直接作为 LLM 的输入消息列表：
// [system_prompt, summary(if any), recent_messages...]
func (m *Manager) GetMessages(sessionID string, systemPrompt string) []*schema.Message {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		// 新会话，只返回 system prompt
		return []*schema.Message{
			{Role: schema.System, Content: systemPrompt},
		}
	}

	messages := make([]*schema.Message, 0, m.config.WindowSize+2)

	// 1. System prompt
	messages = append(messages, &schema.Message{
		Role:    schema.System,
		Content: systemPrompt,
	})

	// 2. 历史摘要（如果有）
	if session.Summary != "" {
		messages = append(messages, &schema.Message{
			Role:    schema.System,
			Content: fmt.Sprintf("[历史对话摘要]\n%s", session.Summary),
		})
	}

	// 3. 最近 N 轮消息（滑动窗口）
	start := 0
	if len(session.Messages) > m.config.WindowSize*2 { // *2 因为一问一答是两条消息
		start = len(session.Messages) - m.config.WindowSize*2
	}
	messages = append(messages, session.Messages[start:]...)

	return messages
}

// AddMessage 添加消息到会话记忆
func (m *Manager) AddMessage(ctx context.Context, sessionID string, msg *schema.Message) error {
	m.mu.Lock()

	session, exists := m.sessions[sessionID]
	if !exists {
		session = &SessionMemory{
			SessionID: sessionID,
			Messages:  make([]*schema.Message, 0),
			CreatedAt: time.Now(),
		}
		m.sessions[sessionID] = session
	}

	session.Messages = append(session.Messages, msg)
	session.UpdatedAt = time.Now()

	// 检查是否需要自动摘要
	needSummary := m.config.EnableAutoSummary &&
		len(session.Messages) > m.config.SummaryThreshold*2

	m.mu.Unlock()

	// 触发自动摘要（异步，不阻塞主流程）
	if needSummary {
		go func() {
			if err := m.summarizeSession(ctx, sessionID); err != nil {
				// 摘要失败不影响主流程
				fmt.Printf("[Memory] 自动摘要失败: %v\n", err)
			}
		}()
	}

	return nil
}

// AddUserAssistantPair 便捷方法：同时添加用户消息和助手回复
func (m *Manager) AddUserAssistantPair(ctx context.Context, sessionID, userMsg, assistantMsg string) error {
	if err := m.AddMessage(ctx, sessionID, &schema.Message{
		Role:    schema.User,
		Content: userMsg,
	}); err != nil {
		return err
	}
	return m.AddMessage(ctx, sessionID, &schema.Message{
		Role:    schema.Assistant,
		Content: assistantMsg,
	})
}

// summarizeSession 对会话历史进行 LLM 摘要
func (m *Manager) summarizeSession(ctx context.Context, sessionID string) error {
	m.mu.RLock()
	session, exists := m.sessions[sessionID]
	if !exists {
		m.mu.RUnlock()
		return nil
	}

	// 获取需要摘要的消息（当前窗口之外的）
	windowStart := len(session.Messages) - m.config.WindowSize*2
	if windowStart <= 0 {
		m.mu.RUnlock()
		return nil
	}

	toSummarize := make([]*schema.Message, windowStart)
	copy(toSummarize, session.Messages[:windowStart])
	existingSummary := session.Summary
	m.mu.RUnlock()

	// 构建摘要 prompt
	summaryPrompt := "请将以下对话历史浓缩为简洁的摘要，保留关键信息点、用户偏好和重要决策。用第三人称描述。"
	if existingSummary != "" {
		summaryPrompt += fmt.Sprintf("\n\n之前的对话摘要：\n%s", existingSummary)
	}

	messages := []*schema.Message{
		{Role: schema.System, Content: summaryPrompt},
	}

	// 添加待摘要的消息
	for _, msg := range toSummarize {
		messages = append(messages, msg)
	}

	messages = append(messages, &schema.Message{
		Role:    schema.User,
		Content: "请输出摘要：",
	})

	// 调用 LLM 生成摘要
	resp, err := m.chatModel.Generate(ctx, messages)
	if err != nil {
		return fmt.Errorf("生成摘要失败: %w", err)
	}

	// 更新摘要并裁剪消息
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists = m.sessions[sessionID]
	if !exists {
		return nil
	}

	session.Summary = resp.Content
	// 只保留窗口内的消息
	if len(session.Messages) > m.config.WindowSize*2 {
		session.Messages = session.Messages[len(session.Messages)-m.config.WindowSize*2:]
	}

	return nil
}

// ExtractLongTermMemory 从对话中提取长期记忆
func (m *Manager) ExtractLongTermMemory(ctx context.Context, sessionID, userID string) error {
	if !m.config.EnableLongTerm || m.chatModel == nil {
		return nil
	}

	m.mu.RLock()
	session, exists := m.sessions[sessionID]
	if !exists || len(session.Messages) < 4 {
		m.mu.RUnlock()
		return nil // 对话太短，不提取
	}
	msgCopy := make([]*schema.Message, len(session.Messages))
	copy(msgCopy, session.Messages)
	m.mu.RUnlock()

	// 构建提取 prompt
	messages := []*schema.Message{
		{
			Role: schema.System,
			Content: `分析以下对话，提取用户的关键偏好、事实和重要信息。
输出格式（每行一条）：
- 偏好: <内容>
- 事实: <内容>
- 习惯: <内容>

如果没有值得记录的长期信息，输出"无"。`,
		},
	}
	messages = append(messages, msgCopy...)
	messages = append(messages, &schema.Message{
		Role:    schema.User,
		Content: "请分析并提取：",
	})

	resp, err := m.chatModel.Generate(ctx, messages)
	if err != nil {
		return fmt.Errorf("提取长期记忆失败: %w", err)
	}

	if resp.Content == "无" || resp.Content == "" {
		return nil
	}

	// 存储长期记忆
	m.mu.Lock()
	defer m.mu.Unlock()

	ltm, exists := m.longTerm[userID]
	if !exists {
		ltm = &LongTermMemory{UserID: userID}
		m.longTerm[userID] = ltm
	}

	ltm.Entries = append(ltm.Entries, MemoryEntry{
		Key:       "auto_extract",
		Content:   resp.Content,
		Source:    sessionID,
		CreatedAt: time.Now(),
	})

	// 限制长期记忆数量
	if len(ltm.Entries) > m.config.MaxLongTermEntries {
		ltm.Entries = ltm.Entries[len(ltm.Entries)-m.config.MaxLongTermEntries:]
	}

	return nil
}

// GetLongTermContext 获取用户的长期记忆上下文（用于拼接到 system prompt）
func (m *Manager) GetLongTermContext(userID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ltm, exists := m.longTerm[userID]
	if !exists || len(ltm.Entries) == 0 {
		return ""
	}

	result := "[用户长期记忆]\n"
	for _, entry := range ltm.Entries {
		result += fmt.Sprintf("- %s\n", entry.Content)
	}
	return result
}

// ClearSession 清除会话记忆
func (m *Manager) ClearSession(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, sessionID)
}

// GetSessionCount 获取活跃会话数
func (m *Manager) GetSessionCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}
