package service

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/skill"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"

	"eino_agent/internal/approval"
	internalTool "eino_agent/internal/tool"
)

// newStreamToolCallChecker decides, from the start of a model turn's stream,
// whether the turn is tool calls (the agent loops) or the answer (the agent
// ends). Eino routes the stream only after this returns, so the answer cannot
// reach the caller before it does.
//
// Models differ in where the call sits: some send it first, some write a
// sentence of text before it (the default Eino checker, which stops at the
// first text, would then drop the call). So the checker reads until a tool
// call shows up, or until peekRunes of text have gone by without one. A
// peekRunes of zero or less reads the whole turn, which is always right and
// delays the answer until the model has finished writing it.
func newStreamToolCallChecker(peekRunes int) func(context.Context, *schema.StreamReader[*schema.Message]) (bool, error) {
	return func(_ context.Context, sr *schema.StreamReader[*schema.Message]) (bool, error) {
		defer sr.Close()
		seen := 0
		for {
			msg, err := sr.Recv()
			if err == io.EOF {
				return false, nil
			}
			if err != nil {
				return false, err
			}
			if msg == nil {
				continue
			}
			if len(msg.ToolCalls) > 0 {
				return true, nil
			}
			if peekRunes > 0 {
				seen += utf8.RuneCountInString(msg.Content)
				if seen >= peekRunes {
					return false, nil
				}
			}
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
) (*agentRun, *internalTool.KnowledgeTool, error) {
	toolCallingModel, ok := any(s.chatModel).(model.ToolCallingChatModel)
	if !ok {
		return nil, nil, fmt.Errorf("agent mode requires ToolCallingChatModel, current model type: %T", s.chatModel)
	}

	toolCallingModel = s.wrapModelForCompaction(toolCallingModel, eventSink)

	tools, kt := s.buildToolsWithRetriever(runtimeRetriever)
	tools = append(tools, s.mcpTools...)

	runtimeSkill, err := s.buildSkillMiddlewareForRequest(ctx, req)
	if err != nil {
		return nil, nil, fmt.Errorf("build skill middleware: %w", err)
	}
	if runtimeSkill != nil {
		tools = append(tools, runtimeSkill.AdditionalTools...)
	}

	toolsConfig := compose.ToolsNodeConfig{Tools: tools, ToolCallMiddlewares: s.toolCallMiddlewares(req, eventSink)}
	systemInstruction := s.buildAgentInstructionForRequest(req, runtimeSkill)
	agent, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolsConfig:           toolsConfig,
		MaxStep:               s.config.Agent.MaxSteps,
		ToolCallingModel:      toolCallingModel,
		StreamToolCallChecker: newStreamToolCallChecker(streamPeekRunes(s.config.Agent.StreamPeekChars)),
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

	return &agentRun{agent: agent, toolsConfig: toolsConfig}, kt, nil
}

// agentRun is a built ReAct agent together with the configuration of its tools
// node, which a streamed run needs again when a model calls a tool after it
// already started writing its answer (see streamAgent). The configuration
// carries the tool middlewares, so a tool run that way is reported, approved
// and checked like any other.
type agentRun struct {
	agent       *react.Agent
	toolsConfig compose.ToolsNodeConfig
}

// streamPeekRunes maps AgentConfig.StreamPeekChars to the checker's window:
// unset means the default, negative means read the whole model turn.
func streamPeekRunes(configured int) int {
	switch {
	case configured < 0:
		return 0
	case configured == 0:
		return 256
	default:
		return configured
	}
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

func humanDecisionObservation(status, toolName, reason string) string {
	if strings.TrimSpace(reason) == "" {
		return fmt.Sprintf("human approval %s for tool %s; do not execute this tool call, explain the result and ask for an alternative if needed", status, toolName)
	}
	return fmt.Sprintf("human approval %s for tool %s: %s; do not execute this tool call, explain the result and ask for an alternative if needed", status, toolName, reason)
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
