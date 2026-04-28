package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"eino_agent/internal/filter"
)

type AgentState string

const (
	AgentStatePrepare       AgentState = "agent_prepare"
	AgentStateReactGenerate AgentState = "react_generate"
	AgentStateCollectSource AgentState = "collect_sources"
	AgentStateComplete      AgentState = "complete"
	AgentStateError         AgentState = "error"
)

type agentStateGraphInput struct {
	Request          *ChatRequest
	Message          string
	RuntimeRetriever retriever.Retriever
	EventSink        func(StreamEvent)
}

type agentStateGraphOutput struct {
	Answer           string
	Sources          []Source
	PromptTokens     int
	CompletionTokens int
	Trace            []TraceStep
	Err              error
}

type agentStateGraphLocal struct {
	Input            agentStateGraphInput
	Started          time.Time
	State            AgentState
	Answer           string
	Sources          []Source
	PromptTokens     int
	CompletionTokens int
	Trace            []TraceStep
	Err              error
}

func (s *ChatService) runAgentStateGraph(ctx context.Context, input agentStateGraphInput) (agentStateGraphOutput, error) {
	graph := compose.NewGraph[agentStateGraphInput, agentStateGraphOutput](
		compose.WithGenLocalState(func(context.Context) *agentStateGraphLocal {
			return &agentStateGraphLocal{Started: time.Now()}
		}),
	)

	if err := graph.AddLambdaNode(string(AgentStatePrepare), compose.InvokableLambda(func(ctx context.Context, in agentStateGraphInput) (agentStateGraphInput, error) {
		return in, compose.ProcessState[*agentStateGraphLocal](ctx, func(_ context.Context, state *agentStateGraphLocal) error {
			state.Input = in
			state.State = AgentStatePrepare
			state.Trace = append(state.Trace, agentStateTrace(AgentStatePrepare, state.Started, map[string]any{"max_steps": s.config.Agent.MaxSteps}))
			return nil
		})
	})); err != nil {
		return agentStateGraphOutput{}, err
	}

	if err := graph.AddLambdaNode(string(AgentStateReactGenerate), compose.InvokableLambda(func(ctx context.Context, in agentStateGraphInput) (agentStateGraphInput, error) {
		return in, compose.ProcessState[*agentStateGraphLocal](ctx, func(_ context.Context, state *agentStateGraphLocal) error {
			state.State = AgentStateReactGenerate
			stepStarted := time.Now()
			runtimeAgent, kt, err := s.buildRuntimeAgentForRequest(ctx, state.Input.RuntimeRetriever, state.Input.Request, state.Input.EventSink)
			if err != nil {
				state.Err = fmt.Errorf("build runtime agent: %w", err)
				state.State = AgentStateError
				state.Trace = append(state.Trace, agentStateErrorTrace(AgentStateReactGenerate, state.Err, state.Started))
				return nil
			}
			messages := []*schema.Message{{Role: schema.User, Content: state.Input.Message}}
			llmCtx, llmCancel := context.WithTimeout(ctx, time.Duration(s.config.Agent.LLMTimeout)*time.Second)
			respMsg, err := runtimeAgent.Generate(llmCtx, messages)
			llmCancel()
			if err != nil {
				state.Err = fmt.Errorf("agent chat: %w", err)
				state.State = AgentStateError
				state.Trace = append(state.Trace, agentStateErrorTrace(AgentStateReactGenerate, state.Err, state.Started))
				return nil
			}
			if respMsg != nil {
				state.Answer = filter.StripThinkTags(respMsg.Content)
				if respMsg.ResponseMeta != nil && respMsg.ResponseMeta.Usage != nil {
					state.PromptTokens = respMsg.ResponseMeta.Usage.PromptTokens
					state.CompletionTokens = respMsg.ResponseMeta.Usage.CompletionTokens
				}
			}
			if kt != nil {
				state.Sources = sourcesFromDocuments(kt.LastDocs(), s.config.RAG.TopK)
			}
			state.Trace = append(state.Trace, TraceStep{
				Type:       "status",
				Stage:      string(AgentStateReactGenerate),
				LatencyMs:  time.Since(stepStarted).Milliseconds(),
				TokenCount: state.PromptTokens + state.CompletionTokens,
				Metadata: map[string]any{
					"prompt_tokens":     state.PromptTokens,
					"completion_tokens": state.CompletionTokens,
				},
			})
			return nil
		})
	})); err != nil {
		return agentStateGraphOutput{}, err
	}

	if err := graph.AddLambdaNode(string(AgentStateCollectSource), compose.InvokableLambda(func(ctx context.Context, in agentStateGraphInput) (agentStateGraphInput, error) {
		return in, compose.ProcessState[*agentStateGraphLocal](ctx, func(_ context.Context, state *agentStateGraphLocal) error {
			state.State = AgentStateCollectSource
			state.Trace = append(state.Trace, agentStateTrace(AgentStateCollectSource, state.Started, map[string]any{"source_count": len(state.Sources)}))
			return nil
		})
	})); err != nil {
		return agentStateGraphOutput{}, err
	}

	if err := graph.AddLambdaNode(string(AgentStateComplete), compose.InvokableLambda(func(ctx context.Context, in agentStateGraphInput) (agentStateGraphOutput, error) {
		var output agentStateGraphOutput
		return output, compose.ProcessState[*agentStateGraphLocal](ctx, func(_ context.Context, state *agentStateGraphLocal) error {
			state.State = AgentStateComplete
			state.Trace = append(state.Trace, agentStateTrace(AgentStateComplete, state.Started, map[string]any{"source_count": len(state.Sources)}))
			output = agentStateGraphOutput{
				Answer:           state.Answer,
				Sources:          state.Sources,
				PromptTokens:     state.PromptTokens,
				CompletionTokens: state.CompletionTokens,
				Trace:            state.Trace,
				Err:              state.Err,
			}
			return nil
		})
	})); err != nil {
		return agentStateGraphOutput{}, err
	}

	if err := graph.AddLambdaNode(string(AgentStateError), compose.InvokableLambda(func(ctx context.Context, in agentStateGraphInput) (agentStateGraphOutput, error) {
		var output agentStateGraphOutput
		return output, compose.ProcessState[*agentStateGraphLocal](ctx, func(_ context.Context, state *agentStateGraphLocal) error {
			output = agentStateGraphOutput{Trace: state.Trace, Err: state.Err}
			return nil
		})
	})); err != nil {
		return agentStateGraphOutput{}, err
	}

	if err := graph.AddEdge(compose.START, string(AgentStatePrepare)); err != nil {
		return agentStateGraphOutput{}, err
	}
	if err := graph.AddEdge(string(AgentStatePrepare), string(AgentStateReactGenerate)); err != nil {
		return agentStateGraphOutput{}, err
	}
	if err := graph.AddBranch(string(AgentStateReactGenerate), compose.NewGraphBranch(func(branchCtx context.Context, _ agentStateGraphInput) (string, error) {
		var target string
		err := compose.ProcessState[*agentStateGraphLocal](branchCtx, func(_ context.Context, state *agentStateGraphLocal) error {
			if state.Err != nil {
				target = string(AgentStateError)
			} else {
				target = string(AgentStateCollectSource)
			}
			return nil
		})
		return target, err
	}, map[string]bool{string(AgentStateCollectSource): true, string(AgentStateError): true})); err != nil {
		return agentStateGraphOutput{}, err
	}
	if err := graph.AddEdge(string(AgentStateCollectSource), string(AgentStateComplete)); err != nil {
		return agentStateGraphOutput{}, err
	}
	if err := graph.AddEdge(string(AgentStateComplete), compose.END); err != nil {
		return agentStateGraphOutput{}, err
	}
	if err := graph.AddEdge(string(AgentStateError), compose.END); err != nil {
		return agentStateGraphOutput{}, err
	}

	runnable, err := graph.Compile(ctx, compose.WithGraphName("react_agent_state_graph"), compose.WithMaxRunSteps(8))
	if err != nil {
		return agentStateGraphOutput{}, err
	}
	output, err := runnable.Invoke(ctx, input)
	if err != nil {
		return agentStateGraphOutput{}, err
	}
	return output, nil
}

func agentStateTrace(state AgentState, started time.Time, metadata map[string]any) TraceStep {
	return TraceStep{Type: "status", Stage: string(state), LatencyMs: time.Since(started).Milliseconds(), Metadata: metadata}
}

func agentStateErrorTrace(state AgentState, err error, started time.Time) TraceStep {
	content := ""
	if err != nil {
		content = err.Error()
	}
	return TraceStep{Type: "error", Stage: string(state), Content: content, LatencyMs: time.Since(started).Milliseconds()}
}

func appendTraceSteps(trace *traceCollector, steps []TraceStep) {
	for _, step := range steps {
		trace.add(step)
	}
}

func agentTraceHasStage(steps []TraceStep, stage AgentState) bool {
	needle := string(stage)
	for _, step := range steps {
		if strings.EqualFold(step.Stage, needle) {
			return true
		}
	}
	return false
}
