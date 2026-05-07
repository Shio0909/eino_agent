package service

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/skill"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/retriever"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"

	"eino_agent/internal/approval"
	internalTool "eino_agent/internal/tool"
)

// allChunksStreamToolCallChecker reads all chunks before deciding whether the model
// issued a tool call. Models like MiniMax M2.7 emit text content BEFORE the tool_call
// object, so the default Eino checker (which bails on the first non-empty text chunk)
// would incorrectly route to END. This checker drains the full stream and returns true
// if any chunk carries a ToolCall.
func allChunksStreamToolCallChecker(_ context.Context, sr *schema.StreamReader[*schema.Message]) (bool, error) {
	defer sr.Close()
	for {
		msg, err := sr.Recv()
		if err == io.EOF {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if msg != nil && len(msg.ToolCalls) > 0 {
			return true, nil
		}
	}
}

type selectedSkillBackend struct {
	base    skill.Backend
	allowed map[string]struct{}
}

func (b *selectedSkillBackend) List(ctx context.Context) ([]skill.FrontMatter, error) {
	items, err := b.base.List(ctx)
	if err != nil {
		return nil, err
	}
	if len(b.allowed) == 0 {
		return items, nil
	}
	filtered := make([]skill.FrontMatter, 0, len(items))
	for _, item := range items {
		if _, ok := b.allowed[strings.TrimSpace(item.Name)]; ok {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

func (b *selectedSkillBackend) Get(ctx context.Context, name string) (skill.Skill, error) {
	if len(b.allowed) > 0 {
		if _, ok := b.allowed[strings.TrimSpace(name)]; !ok {
			return skill.Skill{}, fmt.Errorf("skill %q is not enabled for this request", name)
		}
	}
	return b.base.Get(ctx, name)
}

func normalizeSkillNames(names []string) []string {
	if len(names) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(names))
	out := make([]string, 0, len(names))
	for _, name := range names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func (s *ChatService) buildSkillMiddlewareForRequest(ctx context.Context, req *ChatRequest) (*adk.AgentMiddleware, error) {
	if !s.skillsEnabledForRequest(req) || s.skillBackend == nil {
		return nil, nil
	}

	selected := normalizeSkillNames(req.SelectedSkills)
	if len(selected) == 0 {
		return s.skillMiddleware, nil
	}

	allowed := make(map[string]struct{}, len(selected))
	for _, name := range selected {
		allowed[name] = struct{}{}
	}

	mw, err := skill.New(ctx, &skill.Config{
		Backend:    &selectedSkillBackend{base: s.skillBackend, allowed: allowed},
		UseChinese: true,
	})
	if err != nil {
		return nil, err
	}
	return &mw, nil
}

func (s *ChatService) buildAgentInstructionForRequest(req *ChatRequest, runtimeSkill *adk.AgentMiddleware) string {
	systemInstruction := s.renderSystemPrompt("agentic")
	if runtimeSkill != nil && runtimeSkill.AdditionalInstruction != "" {
		systemInstruction += "\n\n" + runtimeSkill.AdditionalInstruction
	}
	return systemInstruction
}

func (s *ChatService) buildRuntimeAgentForRequest(
	ctx context.Context,
	runtimeRetriever retriever.Retriever,
	req *ChatRequest,
	eventSink func(StreamEvent),
) (*react.Agent, *internalTool.KnowledgeTool, error) {
	toolCallingModel, ok := any(s.chatModel).(model.ToolCallingChatModel)
	if !ok {
		return nil, nil, fmt.Errorf("agent mode requires ToolCallingChatModel, current model type: %T", s.chatModel)
	}

	tools, kt := s.buildToolsWithRetriever(runtimeRetriever)
	tools = append(tools, s.mcpTools...)

	runtimeSkill, err := s.buildSkillMiddlewareForRequest(ctx, req)
	if err != nil {
		return nil, nil, fmt.Errorf("build skill middleware: %w", err)
	}
	if runtimeSkill != nil {
		tools = append(tools, runtimeSkill.AdditionalTools...)
	}
	if eventSink != nil {
		tools = s.wrapAgentToolsForRequest(tools, req, eventSink)
		tracker := newToolCallTracker()
		wrappedTools := make([]einotool.BaseTool, len(tools))
		for i, wt := range tools {
			if et, ok := wt.(*eventTool); ok {
				wrappedTools[i] = &trackerEventTool{eventTool: et, tracker: tracker}
			} else {
				wrappedTools[i] = wt
			}
		}
		tools = wrappedTools
	}

	systemInstruction := s.buildAgentInstructionForRequest(req, runtimeSkill)
	agent, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolsConfig:           compose.ToolsNodeConfig{Tools: tools},
		MaxStep:               s.config.Agent.MaxSteps,
		ToolCallingModel:      toolCallingModel,
		StreamToolCallChecker: allChunksStreamToolCallChecker,
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
	if err != nil {
		return nil, nil, err
	}

	return agent, kt, nil
}

type eventTool struct {
	name            string
	base            einotool.BaseTool
	sink            func(StreamEvent)
	approvalManager *approval.Manager
	approvalRequest approval.Request
}

func (s *ChatService) wrapAgentToolsForRequest(tools []einotool.BaseTool, req *ChatRequest, sink func(StreamEvent)) []einotool.BaseTool {
	if sink == nil || len(tools) == 0 {
		return tools
	}

	wrapped := make([]einotool.BaseTool, 0, len(tools))
	for _, base := range tools {
		if _, ok := base.(einotool.InvokableTool); !ok {
			if _, ok := base.(einotool.StreamableTool); !ok {
				wrapped = append(wrapped, base)
				continue
			}
		}
		name := "tool"
		if info, err := base.Info(context.Background()); err == nil && info != nil && strings.TrimSpace(info.Name) != "" {
			name = strings.TrimSpace(info.Name)
		}
		wrapped = append(wrapped, &eventTool{
			name:            name,
			base:            base,
			sink:            sink,
			approvalManager: s.approvalManager,
			approvalRequest: s.approvalRequestForTool(req, name),
		})
	}
	return wrapped
}

func (s *ChatService) approvalRequestForTool(req *ChatRequest, toolName string) approval.Request {
	if s == nil || s.config == nil || req == nil || !s.config.HITL.Enabled || !s.isHighRiskTool(toolName) {
		return approval.Request{}
	}
	return approval.Request{
		TenantID:  req.TenantID,
		UserID:    req.UserID,
		SessionID: req.SessionID,
		Source:    "chat_stream",
		Action:    toolName,
		ToolName:  toolName,
		Reason:    chatApprovalReason(toolName),
		RiskLevel: "high",
	}
}

func (s *ChatService) isHighRiskTool(toolName string) bool {
	if s == nil || s.config == nil {
		return false
	}
	for _, action := range s.config.HITL.HighRiskActions {
		if strings.EqualFold(strings.TrimSpace(action), toolName) {
			return true
		}
	}
	return false
}

func chatApprovalReason(action string) string {
	switch action {
	case "create_knowledge_base":
		return "创建知识库会写入租户级知识库配置"
	case "import_url":
		return "导入 URL 会访问外部地址并写入知识库内容"
	case "delete_knowledge_base":
		return "删除知识库会移除知识库及其关联内容"
	case "delete_document":
		return "删除文档会移除知识库中的已有内容"
	case "clone_code_repo":
		return "克隆代码仓库会访问外部仓库并写入本地索引目录"
	case "index_code_repo":
		return "索引代码仓库会更新代码知识图谱和检索索引"
	default:
		return "该工具调用具有外部副作用或写入风险，需要人工审批"
	}
}

func wrapAgentTools(tools []einotool.BaseTool, sink func(StreamEvent)) []einotool.BaseTool {
	if sink == nil || len(tools) == 0 {
		return tools
	}

	wrapped := make([]einotool.BaseTool, 0, len(tools))
	for _, base := range tools {
		if _, ok := base.(einotool.InvokableTool); !ok {
			if _, ok := base.(einotool.StreamableTool); !ok {
				wrapped = append(wrapped, base)
				continue
			}
		}
		name := "tool"
		if info, err := base.Info(context.Background()); err == nil && info != nil && strings.TrimSpace(info.Name) != "" {
			name = strings.TrimSpace(info.Name)
		}
		wrapped = append(wrapped, &eventTool{name: name, base: base, sink: sink})
	}
	return wrapped
}

func (t *eventTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return t.base.Info(ctx)
}

func (t *eventTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...einotool.Option) (string, error) {
	it, ok := t.base.(einotool.InvokableTool)
	if !ok {
		return "", fmt.Errorf("tool %s is not invokable", t.name)
	}

	started := time.Now()
	t.emit("action", argumentsInJSON, "", 0, nil, map[string]any{"args_chars": len(argumentsInJSON)})
	if proceed, observation, waitErr := t.waitForApproval(ctx, argumentsInJSON, started); !proceed {
		metadata := map[string]any{"args_chars": len(argumentsInJSON), "output_chars": len(observation), "approval_blocked": true}
		if waitErr != nil {
			t.emit("observation", argumentsInJSON, summarizeToolOutput("error: "+waitErr.Error()), time.Since(started).Milliseconds(), waitErr, metadata)
			return "", waitErr
		}
		t.emit("observation", argumentsInJSON, summarizeToolOutput(observation), time.Since(started).Milliseconds(), nil, metadata)
		return observation, nil
	}
	result, err := it.InvokableRun(ctx, argumentsInJSON, opts...)
	metadata := map[string]any{"args_chars": len(argumentsInJSON), "output_chars": len(result)}
	if err != nil {
		t.emit("observation", argumentsInJSON, summarizeToolOutput("error: "+err.Error()), time.Since(started).Milliseconds(), err, metadata)
		return "", err
	}
	t.emit("observation", argumentsInJSON, summarizeToolOutput(result), time.Since(started).Milliseconds(), nil, metadata)
	return result, nil
}

func (t *eventTool) StreamableRun(ctx context.Context, argumentsInJSON string, opts ...einotool.Option) (*schema.StreamReader[string], error) {
	st, ok := t.base.(einotool.StreamableTool)
	if !ok {
		return nil, fmt.Errorf("tool %s is not streamable", t.name)
	}

	started := time.Now()
	t.emit("action", argumentsInJSON, "", 0, nil, map[string]any{"args_chars": len(argumentsInJSON), "streamable": true})
	if proceed, observation, waitErr := t.waitForApproval(ctx, argumentsInJSON, started); !proceed {
		metadata := map[string]any{"args_chars": len(argumentsInJSON), "streamable": true, "approval_blocked": true}
		if waitErr != nil {
			t.emit("observation", argumentsInJSON, summarizeToolOutput("error: "+waitErr.Error()), time.Since(started).Milliseconds(), waitErr, metadata)
			return nil, waitErr
		}
		t.emit("observation", argumentsInJSON, summarizeToolOutput(observation), time.Since(started).Milliseconds(), nil, metadata)
		return schema.StreamReaderFromArray([]string{observation}), nil
	}
	reader, err := st.StreamableRun(ctx, argumentsInJSON, opts...)
	metadata := map[string]any{"args_chars": len(argumentsInJSON), "streamable": true}
	if err != nil {
		t.emit("observation", argumentsInJSON, summarizeToolOutput("error: "+err.Error()), time.Since(started).Milliseconds(), err, metadata)
		return nil, err
	}
	t.emit("observation", argumentsInJSON, "stream started", time.Since(started).Milliseconds(), nil, metadata)
	return reader, nil
}

func (t *eventTool) waitForApproval(ctx context.Context, argumentsInJSON string, started time.Time) (bool, string, error) {
	if t.approvalManager == nil || t.approvalRequest.Action == "" {
		return true, "", nil
	}
	req := t.approvalRequest
	req.ToolInput = argumentsInJSON
	item, err := t.approvalManager.Create(ctx, req)
	if err != nil {
		return false, "", fmt.Errorf("create approval request: %w", err)
	}
	metadata := map[string]any{
		"args_chars":  len(argumentsInJSON),
		"action_hash": item.ActionHash,
		"source":      item.Source,
	}
	t.emitApproval("approval_required", argumentsInJSON, item, metadata, time.Since(started).Milliseconds(), "")
	decision, err := t.approvalManager.Wait(ctx, item.ID)
	if err != nil {
		return false, "", err
	}
	current, _ := t.approvalManager.Get(ctx, item.ID)
	if current.ID == "" {
		current = item
	}
	switch decision.Decision {
	case approval.DecisionApprove:
		t.emitApproval("approval_resolved", argumentsInJSON, current, metadata, time.Since(started).Milliseconds(), decision.Reason)
		return true, "", nil
	case approval.DecisionReject:
		t.emitApproval("approval_rejected", argumentsInJSON, current, metadata, time.Since(started).Milliseconds(), decision.Reason)
		return false, humanDecisionObservation("rejected", t.name, decision.Reason), nil
	case approval.DecisionExpire:
		t.emitApproval("approval_expired", argumentsInJSON, current, metadata, time.Since(started).Milliseconds(), decision.Reason)
		return false, humanDecisionObservation("expired", t.name, decision.Reason), nil
	default:
		return false, humanDecisionObservation("rejected", t.name, decision.Reason), nil
	}
}

func (t *eventTool) emitApproval(eventType, input string, item approval.Approval, metadata map[string]any, latencyMs int64, decisionReason string) {
	if t.sink == nil {
		return
	}
	content := item.Reason
	if decisionReason != "" {
		content = decisionReason
	}
	step := &TraceStep{
		Type:      eventType,
		Stage:     eventType,
		Content:   content,
		ToolName:  t.name,
		ToolInput: input,
		LatencyMs: latencyMs,
		Metadata:  metadata,
	}
	t.sink(StreamEvent{
		Type:           eventType,
		Content:        content,
		ToolName:       t.name,
		ToolInput:      input,
		ApprovalID:     item.ID,
		ApprovalStatus: string(item.Status),
		Action:         item.Action,
		RiskLevel:      item.RiskLevel,
		Reason:         item.Reason,
		ExpiresAt:      &item.ExpiresAt,
		Metadata:       metadata,
		LatencyMs:      latencyMs,
		TraceStep:      step,
	})
}

func humanDecisionObservation(status, toolName, reason string) string {
	if strings.TrimSpace(reason) == "" {
		return fmt.Sprintf("human approval %s for tool %s; do not execute this tool call, explain the result and ask for an alternative if needed", status, toolName)
	}
	return fmt.Sprintf("human approval %s for tool %s: %s; do not execute this tool call, explain the result and ask for an alternative if needed", status, toolName, reason)
}

func (t *eventTool) emit(eventType, input, content string, latencyMs int64, err error, metadata map[string]any) {
	if t.sink == nil {
		return
	}
	step := &TraceStep{
		Type:      eventType,
		Stage:     eventType,
		Content:   content,
		ToolName:  t.name,
		ToolInput: input,
		LatencyMs: latencyMs,
		Metadata:  metadata,
	}
	if err != nil {
		step.Level = "error"
		step.Error = err.Error()
	}
	t.sink(StreamEvent{
		Type:      eventType,
		Content:   content,
		ToolName:  t.name,
		ToolInput: input,
		Metadata:  metadata,
		LatencyMs: latencyMs,
		Error:     step.Error,
		TraceStep: step,
	})
}

func summarizeToolOutput(output string) string {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return "(empty tool result)"
	}
	runes := []rune(trimmed)
	if len(runes) > 300 {
		return string(runes[:300]) + "..."
	}
	return trimmed
}

var _ einotool.InvokableTool = (*eventTool)(nil)
var _ einotool.StreamableTool = (*eventTool)(nil)

// toolCallRecord 记录单次工具调用
type toolCallRecord struct {
	name string
	args string
	time time.Time
}

// toolCallTracker 追踪 Agent 工具调用，检测重复模式
type toolCallTracker struct {
	mu      sync.Mutex
	history []toolCallRecord
	warned  bool
}

func newToolCallTracker() *toolCallTracker {
	return &toolCallTracker{}
}

// record 记录一次调用，返回注入给 Agent 的警告消息（如有）。
func (t *toolCallTracker) record(name, args string) string {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.history = append(t.history, toolCallRecord{name: name, args: args, time: time.Now()})
	if len(t.history) > 10 {
		t.history = t.history[len(t.history)-10:]
	}

	// 检测：同一 tool + 相同参数 >= 3 次
	sameCount := 0
	normalized := strings.TrimSpace(args)
	for i := len(t.history) - 1; i >= 0; i-- {
		if t.history[i].name == name && strings.TrimSpace(t.history[i].args) == normalized {
			sameCount++
		} else {
			break
		}
	}
	if sameCount >= 3 && !t.warned {
		t.warned = true
		return fmt.Sprintf(
			"⚠️ 工具 %q 已被连续调用 %d 次且结果无新发现。你已在决策阶段第五条——必须停止调用此工具，立刻基于已有证据作答。",
			name, sameCount)
	}

	// 检测：同一 tool 连续 >= 5 次
	consecutive := 0
	for i := len(t.history) - 1; i >= 0; i-- {
		if t.history[i].name == name {
			consecutive++
		} else {
			break
		}
	}
	if consecutive >= 5 && !t.warned {
		t.warned = true
		return fmt.Sprintf(
			"⚠️ 工具 %q 已被连续调用 %d 次。强烈建议你停止调用该工具，使用其他工具或基于已有信息作答。",
			name, consecutive)
	}

	return ""
}

func (t *toolCallTracker) reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.history = nil
	t.warned = false
}

// trackerEventTool 带有调用追踪的 eventTool
type trackerEventTool struct {
	*eventTool
	tracker *toolCallTracker
}

func (t *trackerEventTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...einotool.Option) (string, error) {
	warning := t.tracker.record(t.name, argumentsInJSON)
	if warning != "" && t.eventTool.sink != nil {
		t.eventTool.sink(StreamEvent{Type: "warning", Content: warning})
	}
	return t.eventTool.InvokableRun(ctx, argumentsInJSON, opts...)
}

func (t *trackerEventTool) StreamableRun(ctx context.Context, argumentsInJSON string, opts ...einotool.Option) (*schema.StreamReader[string], error) {
	warning := t.tracker.record(t.name, argumentsInJSON)
	if warning != "" && t.eventTool.sink != nil {
		t.eventTool.sink(StreamEvent{Type: "warning", Content: warning})
	}
	return t.eventTool.StreamableRun(ctx, argumentsInJSON, opts...)
}
