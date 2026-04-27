package pipeline

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/cloudwego/eino/schema"
)

type failingFallbackProvider struct{}

func (failingFallbackProvider) Search(context.Context, ExternalSearchRequest) ([]ExternalSearchResult, error) {
	return nil, errors.New("provider unavailable")
}

func TestFallbackGraphRoutesKnowledgeToContext(t *testing.T) {
	provider := &fallbackTestProvider{}
	pipeline := NewRAGPipeline(&Config{Fallback: FallbackConfig{Enabled: true, MinKnowledgeDocs: 1, MinContextChars: 5}},
		WithExternalFallbackProvider(provider),
	)

	output, err := pipeline.runFallbackGraph(context.Background(), "question", []*schema.Document{{ID: "kb-1", Content: "trusted knowledge"}})
	if err != nil {
		t.Fatalf("runFallbackGraph error = %v", err)
	}
	if provider.calls != 0 {
		t.Fatalf("provider calls = %d, want 0", provider.calls)
	}
	if output.Decision.State != StateKBContextBuild || output.Decision.AllowExternal {
		t.Fatalf("decision = %#v, want kb context", output.Decision)
	}
	if got := graphStates(output.Trace); !reflect.DeepEqual(got, []AnswerState{StateKBAssess, StateKBContextBuild}) {
		t.Fatalf("trace states = %#v", got)
	}
}

func TestFallbackGraphRoutesEmptyKnowledgeToExternal(t *testing.T) {
	provider := &fallbackTestProvider{results: []ExternalSearchResult{{Provider: "web", Title: "Web", URL: "https://example.com", Content: "web evidence", Score: 1}}}
	pipeline := NewRAGPipeline(&Config{Fallback: FallbackConfig{Enabled: true, AllowedProviders: []string{"web"}, MaxExternalContext: 2}},
		WithExternalFallbackProvider(provider),
	)

	output, err := pipeline.runFallbackGraph(context.Background(), "latest", nil)
	if err != nil {
		t.Fatalf("runFallbackGraph error = %v", err)
	}
	if provider.calls != 1 {
		t.Fatalf("provider calls = %d, want 1", provider.calls)
	}
	if output.Decision.State != StateExternalContextBuild || output.Decision.Reason != FallbackReasonNoDocuments {
		t.Fatalf("decision = %#v, want external context", output.Decision)
	}
	if len(output.Docs) != 1 || output.Docs[0].Content != "web evidence" {
		t.Fatalf("docs = %#v, want external evidence", output.Docs)
	}
	if got := graphStates(output.Trace); !reflect.DeepEqual(got, []AnswerState{StateKBAssess, StateExternalPlan, StateExternalSearch, StateExternalContextBuild}) {
		t.Fatalf("trace states = %#v", got)
	}
}

func TestFallbackGraphRoutesProviderErrorToRefuse(t *testing.T) {
	pipeline := NewRAGPipeline(&Config{Fallback: FallbackConfig{Enabled: true}},
		WithExternalFallbackProvider(failingFallbackProvider{}),
	)

	output, err := pipeline.runFallbackGraph(context.Background(), "latest", nil)
	if err != nil {
		t.Fatalf("runFallbackGraph error = %v", err)
	}
	if output.Decision.State != StateRefuseOrClarify || output.Decision.Reason != FallbackReasonProviderError {
		t.Fatalf("decision = %#v, want provider error refusal", output.Decision)
	}
	if output.Err == nil {
		t.Fatalf("output.Err = nil, want provider error")
	}
	if got := graphStates(output.Trace); !reflect.DeepEqual(got, []AnswerState{StateKBAssess, StateExternalPlan, StateExternalSearch, StateRefuseOrClarify}) {
		t.Fatalf("trace states = %#v", got)
	}
}

func graphStates(steps []FallbackGraphStep) []AnswerState {
	states := make([]AnswerState, 0, len(steps))
	for _, step := range steps {
		states = append(states, step.State)
	}
	return states
}
