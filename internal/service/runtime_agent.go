package service

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/skill"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/retriever"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"

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
		tools = wrapAgentTools(tools, eventSink)
	}

	systemInstruction := s.buildAgentInstructionForRequest(req, runtimeSkill)
	agent, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolsConfig:          compose.ToolsNodeConfig{Tools: tools},
		MaxStep:              s.config.Agent.MaxSteps,
		ToolCallingModel:     toolCallingModel,
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
	name string
	base einotool.BaseTool
	sink func(StreamEvent)
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

	t.emit("action", argumentsInJSON, "")
	result, err := it.InvokableRun(ctx, argumentsInJSON, opts...)
	if err != nil {
		t.emit("observation", argumentsInJSON, "error: "+err.Error())
		return "", err
	}
	t.emit("observation", argumentsInJSON, summarizeToolOutput(result))
	return result, nil
}

func (t *eventTool) StreamableRun(ctx context.Context, argumentsInJSON string, opts ...einotool.Option) (*schema.StreamReader[string], error) {
	st, ok := t.base.(einotool.StreamableTool)
	if !ok {
		return nil, fmt.Errorf("tool %s is not streamable", t.name)
	}

	t.emit("action", argumentsInJSON, "")
	reader, err := st.StreamableRun(ctx, argumentsInJSON, opts...)
	if err != nil {
		t.emit("observation", argumentsInJSON, "error: "+err.Error())
		return nil, err
	}
	return reader, nil
}

func (t *eventTool) emit(eventType, input, content string) {
	if t.sink == nil {
		return
	}
	t.sink(StreamEvent{
		Type:      eventType,
		Content:   content,
		ToolName:  t.name,
		ToolInput: input,
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
