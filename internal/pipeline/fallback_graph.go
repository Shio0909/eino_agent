package pipeline

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

const (
	fallbackNodeAssessKnowledge = "kb_assess"
	fallbackNodeKnowledgeReady  = "kb_context_build"
	fallbackNodeExternalPlan    = "external_plan"
	fallbackNodeExternalSearch  = "external_search"
	fallbackNodeExternalAssess  = "external_assess"
	fallbackNodeRefuse          = "refuse_or_clarify"
)

type fallbackGraphInput struct {
	Query string
	Docs  []*schema.Document
}

type fallbackGraphOutput struct {
	Docs     []*schema.Document
	Decision FallbackDecision
	Trace    []FallbackGraphStep
	Err      error
}

type FallbackGraphStep struct {
	State     AnswerState    `json:"state"`
	Reason    FallbackReason `json:"reason,omitempty"`
	Providers []string       `json:"providers,omitempty"`
	DocCount  int            `json:"doc_count"`
	Error     string         `json:"error,omitempty"`
}

type fallbackGraphState struct {
	Query    string
	Docs     []*schema.Document
	Decision FallbackDecision
	Trace    []FallbackGraphStep
	Err      error
}

func (p *RAGPipeline) runFallbackGraph(ctx context.Context, query string, docs []*schema.Document) (fallbackGraphOutput, error) {
	graph := compose.NewGraph[fallbackGraphInput, fallbackGraphOutput](
		compose.WithGenLocalState(func(context.Context) *fallbackGraphState {
			return &fallbackGraphState{}
		}),
	)

	if err := graph.AddLambdaNode(fallbackNodeAssessKnowledge, compose.InvokableLambda(func(ctx context.Context, input fallbackGraphInput) (fallbackGraphInput, error) {
		decision := p.assessKnowledgeResults(input.Query, input.Docs)
		return input, compose.ProcessState[*fallbackGraphState](ctx, func(_ context.Context, state *fallbackGraphState) error {
			state.Query = input.Query
			state.Docs = input.Docs
			state.Decision = decision
			state.Trace = append(state.Trace, fallbackGraphStep(decision, len(input.Docs), ""))
			return nil
		})
	})); err != nil {
		return fallbackGraphOutput{}, err
	}

	if err := graph.AddLambdaNode(fallbackNodeKnowledgeReady, compose.InvokableLambda(func(ctx context.Context, input fallbackGraphInput) (fallbackGraphOutput, error) {
		var output fallbackGraphOutput
		return output, compose.ProcessState[*fallbackGraphState](ctx, func(_ context.Context, state *fallbackGraphState) error {
			state.Decision.State = StateKBContextBuild
			state.Trace = append(state.Trace, fallbackGraphStep(state.Decision, len(state.Docs), ""))
			output = fallbackGraphOutput{Docs: state.Docs, Decision: state.Decision, Trace: state.Trace}
			return nil
		})
	})); err != nil {
		return fallbackGraphOutput{}, err
	}

	if err := graph.AddLambdaNode(fallbackNodeExternalPlan, compose.InvokableLambda(func(ctx context.Context, input fallbackGraphInput) (fallbackGraphInput, error) {
		return input, compose.ProcessState[*fallbackGraphState](ctx, func(_ context.Context, state *fallbackGraphState) error {
			state.Decision.State = StateExternalPlan
			state.Trace = append(state.Trace, fallbackGraphStep(state.Decision, len(state.Docs), ""))
			return nil
		})
	})); err != nil {
		return fallbackGraphOutput{}, err
	}

	if err := graph.AddLambdaNode(fallbackNodeExternalSearch, compose.InvokableLambda(func(ctx context.Context, input fallbackGraphInput) (fallbackGraphInput, error) {
		return input, compose.ProcessState[*fallbackGraphState](ctx, func(_ context.Context, state *fallbackGraphState) error {
			state.Decision.State = StateExternalSearch
			state.Trace = append(state.Trace, fallbackGraphStep(state.Decision, len(state.Docs), ""))
			results, err := p.fallback.Search(ctx, ExternalSearchRequest{Query: state.Query, Providers: state.Decision.Providers, MaxResults: p.fallbackConfig().MaxExternalResults})
			if err != nil {
				state.Decision.State = StateRefuseOrClarify
				state.Decision.Reason = FallbackReasonProviderError
				state.Err = err
				state.Trace = append(state.Trace, fallbackGraphStep(state.Decision, len(state.Docs), err.Error()))
				return nil
			}
			state.Docs = externalResultsToDocuments(results, p.fallbackConfig().MaxExternalContext)
			return nil
		})
	})); err != nil {
		return fallbackGraphOutput{}, err
	}

	if err := graph.AddLambdaNode(fallbackNodeExternalAssess, compose.InvokableLambda(func(ctx context.Context, input fallbackGraphInput) (fallbackGraphOutput, error) {
		var output fallbackGraphOutput
		return output, compose.ProcessState[*fallbackGraphState](ctx, func(_ context.Context, state *fallbackGraphState) error {
			if len(state.Docs) == 0 {
				state.Decision.State = StateRefuseOrClarify
				state.Decision.Reason = FallbackReasonNoExternal
				state.Trace = append(state.Trace, fallbackGraphStep(state.Decision, 0, ""))
			} else {
				state.Decision.State = StateExternalContextBuild
				state.Trace = append(state.Trace, fallbackGraphStep(state.Decision, len(state.Docs), ""))
			}
			state.Decision.DocsCount = len(state.Docs)
			state.Decision.ContextChars = totalDocumentChars(state.Docs)
			output = fallbackGraphOutput{Docs: state.Docs, Decision: state.Decision, Trace: state.Trace, Err: state.Err}
			return nil
		})
	})); err != nil {
		return fallbackGraphOutput{}, err
	}

	if err := graph.AddLambdaNode(fallbackNodeRefuse, compose.InvokableLambda(func(ctx context.Context, input fallbackGraphInput) (fallbackGraphOutput, error) {
		var output fallbackGraphOutput
		return output, compose.ProcessState[*fallbackGraphState](ctx, func(_ context.Context, state *fallbackGraphState) error {
			output = fallbackGraphOutput{Docs: state.Docs, Decision: state.Decision, Trace: state.Trace, Err: state.Err}
			return nil
		})
	})); err != nil {
		return fallbackGraphOutput{}, err
	}

	if err := graph.AddEdge(compose.START, fallbackNodeAssessKnowledge); err != nil {
		return fallbackGraphOutput{}, err
	}
	if err := graph.AddBranch(fallbackNodeAssessKnowledge, compose.NewGraphBranch(func(branchCtx context.Context, _ fallbackGraphInput) (string, error) {
		var target string
		err := compose.ProcessState[*fallbackGraphState](branchCtx, func(_ context.Context, state *fallbackGraphState) error {
			if state.Decision.AllowExternal {
				target = fallbackNodeExternalPlan
			} else {
				target = fallbackNodeKnowledgeReady
			}
			return nil
		})
		return target, err
	}, map[string]bool{fallbackNodeKnowledgeReady: true, fallbackNodeExternalPlan: true})); err != nil {
		return fallbackGraphOutput{}, err
	}
	if err := graph.AddEdge(fallbackNodeKnowledgeReady, compose.END); err != nil {
		return fallbackGraphOutput{}, err
	}
	if err := graph.AddEdge(fallbackNodeExternalPlan, fallbackNodeExternalSearch); err != nil {
		return fallbackGraphOutput{}, err
	}
	if err := graph.AddBranch(fallbackNodeExternalSearch, compose.NewGraphBranch(func(branchCtx context.Context, _ fallbackGraphInput) (string, error) {
		var target string
		err := compose.ProcessState[*fallbackGraphState](branchCtx, func(_ context.Context, state *fallbackGraphState) error {
			if state.Err != nil {
				target = fallbackNodeRefuse
			} else {
				target = fallbackNodeExternalAssess
			}
			return nil
		})
		return target, err
	}, map[string]bool{fallbackNodeExternalAssess: true, fallbackNodeRefuse: true})); err != nil {
		return fallbackGraphOutput{}, err
	}
	if err := graph.AddEdge(fallbackNodeExternalAssess, compose.END); err != nil {
		return fallbackGraphOutput{}, err
	}
	if err := graph.AddEdge(fallbackNodeRefuse, compose.END); err != nil {
		return fallbackGraphOutput{}, err
	}

	runnable, err := graph.Compile(ctx, compose.WithGraphName("rag_fallback_state_graph"), compose.WithMaxRunSteps(10))
	if err != nil {
		return fallbackGraphOutput{}, err
	}
	output, err := runnable.Invoke(ctx, fallbackGraphInput{Query: query, Docs: docs})
	if err != nil {
		return fallbackGraphOutput{}, fmt.Errorf("fallback graph: %w", err)
	}
	return output, nil
}

func fallbackGraphStep(decision FallbackDecision, docCount int, err string) FallbackGraphStep {
	return FallbackGraphStep{State: decision.State, Reason: decision.Reason, Providers: decision.Providers, DocCount: docCount, Error: err}
}
