package service

import (
	"context"
	"strings"
	"testing"

	"eino_agent/internal/config"
	"eino_agent/internal/database/repository"
)

func newEvidenceScopeService(t *testing.T) *ChatService {
	t.Helper()
	svc, err := NewChatService(&config.Config{Memory: config.MemoryConfig{Enabled: true, MaxContextChars: 4000}})
	if err != nil {
		t.Fatalf("NewChatService error = %v", err)
	}
	svc.messageRepo = newFakeMessageRepo()
	return svc
}

func scopedSource(chunkID, kbID, docID, content string) Source {
	meta := map[string]any{"source": docID + ".md"}
	if kbID != "" {
		meta["knowledge_base_id"] = kbID
	}
	if docID != "" {
		meta["knowledge_id"] = docID
	}
	return Source{DocID: chunkID, Content: content, Metadata: meta}
}

func rememberSources(svc *ChatService, sessionID string, sources ...Source) {
	svc.saveAssistantMessageWithTrace(context.Background(), sessionID, "上一轮答案", 0, 10, nil, sources)
}

func TestFollowUpEvidenceDropsSourcesOutsideCurrentScope(t *testing.T) {
	svc := newEvidenceScopeService(t)
	rememberSources(svc, "s", scopedSource("old-chunk", "old-kb", "old-doc", "OLD_SCOPE_EVIDENCE"))

	req := &ChatRequest{
		Message:           "继续讲这个",
		KnowledgeBaseIDs:  []string{"new-kb"},
		DocumentIDs:       []string{"new-doc"},
		RestrictRetrieval: true,
	}
	instruction, sources := svc.buildFollowUpEvidenceInstruction(context.Background(), req, "s")
	if strings.Contains(instruction, "OLD_SCOPE_EVIDENCE") {
		t.Fatalf("out-of-scope source was injected: %q", instruction)
	}
	if instruction != "" || len(sources) != 0 {
		t.Fatalf("want no follow-up evidence, got instruction=%q sources=%#v", instruction, sources)
	}
}

func TestFollowUpEvidenceKeepsOnlyTheSourcesStillInScope(t *testing.T) {
	svc := newEvidenceScopeService(t)
	rememberSources(svc, "s",
		scopedSource("c1", "kb-a", "doc-a", "KEEP_A"),
		scopedSource("c2", "kb-b", "doc-b", "DROP_B"),
	)

	req := &ChatRequest{Message: "继续讲这个", KnowledgeBaseIDs: []string{"kb-a"}, RestrictRetrieval: true}
	instruction, sources := svc.buildFollowUpEvidenceInstruction(context.Background(), req, "s")
	if len(sources) != 1 || sources[0].DocID != "c1" {
		t.Fatalf("sources = %#v, want only c1", sources)
	}
	if !strings.Contains(instruction, "KEEP_A") || strings.Contains(instruction, "DROP_B") {
		t.Fatalf("instruction should hold KEEP_A and not DROP_B: %q", instruction)
	}
}

func TestFollowUpEvidenceScopeIsAppliedBeforeTheSourceLimit(t *testing.T) {
	svc := newEvidenceScopeService(t)
	rememberSources(svc, "s",
		scopedSource("c1", "kb-x", "d1", "DROP_1"),
		scopedSource("c2", "kb-x", "d2", "DROP_2"),
		scopedSource("c3", "kb-x", "d3", "DROP_3"),
		scopedSource("c4", "kb-a", "d4", "KEEP_4"),
	)

	req := &ChatRequest{Message: "继续讲这个", KnowledgeBaseIDs: []string{"kb-a"}, RestrictRetrieval: true}
	instruction, sources := svc.buildFollowUpEvidenceInstruction(context.Background(), req, "s")
	if len(sources) != 1 || sources[0].DocID != "c4" {
		t.Fatalf("sources = %#v, want only c4 even though it ranks after the limit", sources)
	}
	if !strings.Contains(instruction, "KEEP_4") {
		t.Fatalf("instruction lost the in-scope source: %q", instruction)
	}
}

func TestFollowUpEvidenceHonorsDocumentScope(t *testing.T) {
	cases := []struct {
		name string
		req  ChatRequest
		want bool
	}{
		{"document selected by knowledge id", ChatRequest{KnowledgeBaseIDs: []string{"kb-a"}, DocumentIDs: []string{"doc-1"}}, true},
		{"document selected by chunk id", ChatRequest{DocumentIDs: []string{"chunk-1"}}, true},
		{"other document selected", ChatRequest{KnowledgeBaseIDs: []string{"kb-a"}, DocumentIDs: []string{"doc-2"}}, false},
		{"other knowledge base selected", ChatRequest{KnowledgeBaseIDs: []string{"kb-b"}, DocumentIDs: []string{"doc-1"}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newEvidenceScopeService(t)
			rememberSources(svc, "s", scopedSource("chunk-1", "kb-a", "doc-1", "THE_EVIDENCE"))

			req := tc.req
			req.Message = "继续讲这个"
			req.RestrictRetrieval = true
			instruction, sources := svc.buildFollowUpEvidenceInstruction(context.Background(), &req, "s")
			got := strings.Contains(instruction, "THE_EVIDENCE")
			if got != tc.want || (len(sources) == 1) != tc.want {
				t.Fatalf("injected=%v sources=%d, want injected=%v", got, len(sources), tc.want)
			}
		})
	}
}

func TestFollowUpEvidenceIsDroppedWhenNoKnowledgeBaseIsAccessible(t *testing.T) {
	svc := newEvidenceScopeService(t)
	rememberSources(svc, "s", scopedSource("c1", "kb-a", "doc-a", "PREVIOUSLY_VISIBLE"))

	req := &ChatRequest{Message: "继续讲这个", KnowledgeBaseIDs: []string{}, RestrictRetrieval: true}
	instruction, sources := svc.buildFollowUpEvidenceInstruction(context.Background(), req, "s")
	if instruction != "" || len(sources) != 0 {
		t.Fatalf("a restricted request with an empty scope must reuse nothing, got %q %#v", instruction, sources)
	}
}

func TestFollowUpEvidenceWithoutScopeMetadataIsNotTrustedUnderAScope(t *testing.T) {
	svc := newEvidenceScopeService(t)
	rememberSources(svc, "s", Source{DocID: "c1", Content: "NO_PROVENANCE", Metadata: map[string]any{"source": "x.md"}})

	req := &ChatRequest{Message: "继续讲这个", KnowledgeBaseIDs: []string{"kb-a"}, RestrictRetrieval: true}
	instruction, sources := svc.buildFollowUpEvidenceInstruction(context.Background(), req, "s")
	if instruction != "" || len(sources) != 0 {
		t.Fatalf("a source that cannot show its knowledge base must not pass a KB scope, got %q %#v", instruction, sources)
	}
}

func TestFollowUpEvidenceIsUnchangedWhenTheRequestHasNoScope(t *testing.T) {
	svc := newEvidenceScopeService(t)
	rememberSources(svc, "s",
		scopedSource("c1", "kb-a", "doc-a", "FROM_A"),
		Source{DocID: "c2", Content: "NO_METADATA"},
	)

	instruction, sources := svc.buildFollowUpEvidenceInstruction(context.Background(), &ChatRequest{Message: "继续讲这个"}, "s")
	if len(sources) != 2 || !strings.Contains(instruction, "FROM_A") || !strings.Contains(instruction, "NO_METADATA") {
		t.Fatalf("an unscoped request should keep all remembered evidence, got %q %#v", instruction, sources)
	}
}

func TestFollowUpEvidenceScopeAppliesToSourcesReadBackFromJSON(t *testing.T) {
	svc := newEvidenceScopeService(t)
	svc.messageRepo.(*fakeMessageRepo).messages["s"] = []*repository.Message{{
		SessionID: "s",
		Role:      "assistant",
		Content:   "上一轮答案",
		AgentSteps: repository.JSON{"sources": []any{
			map[string]any{"content": "JSON_OLD", "doc_id": "c1", "metadata": map[string]any{"knowledge_base_id": "old-kb", "knowledge_id": "old-doc"}},
			map[string]any{"content": "JSON_NEW", "doc_id": "c2", "metadata": map[string]any{"knowledge_base_id": "new-kb", "knowledge_id": "new-doc"}},
		}},
	}}

	req := &ChatRequest{Message: "继续讲这个", KnowledgeBaseIDs: []string{"new-kb"}, RestrictRetrieval: true}
	instruction, sources := svc.buildFollowUpEvidenceInstruction(context.Background(), req, "s")
	if len(sources) != 1 || sources[0].DocID != "c2" {
		t.Fatalf("sources = %#v, want only c2", sources)
	}
	if strings.Contains(instruction, "JSON_OLD") || !strings.Contains(instruction, "JSON_NEW") {
		t.Fatalf("unexpected instruction: %q", instruction)
	}
}

func TestPrepareChatContextDoesNotCarryEvidenceAcrossAScopeChange(t *testing.T) {
	svc := newEvidenceScopeService(t)
	rememberSources(svc, "s1", scopedSource("old-chunk", "old-kb", "old-doc", "OLD_SCOPE_EVIDENCE"))

	cc := svc.prepareChatContext(context.Background(), &ChatRequest{
		SessionID:         "s1",
		Message:           "继续展开这个",
		KnowledgeBaseIDs:  []string{"new-kb"},
		RestrictRetrieval: true,
	})
	if cc.followUpEvidenceCount != 0 {
		t.Fatalf("followUpEvidenceCount = %d, want 0", cc.followUpEvidenceCount)
	}
	if strings.Contains(cc.runtimeInstruction, "OLD_SCOPE_EVIDENCE") || strings.Contains(cc.messageWithInst, "OLD_SCOPE_EVIDENCE") {
		t.Fatalf("old evidence reached the model input: %q", cc.messageWithInst)
	}
}

func TestPrepareChatContextCountsOnlyTheEvidenceThatSurvivesTheScope(t *testing.T) {
	svc := newEvidenceScopeService(t)
	rememberSources(svc, "s1",
		scopedSource("c1", "kb-a", "doc-a", "STILL_IN_SCOPE"),
		scopedSource("c2", "kb-b", "doc-b", "NOW_OUT_OF_SCOPE"),
	)

	cc := svc.prepareChatContext(context.Background(), &ChatRequest{
		SessionID:         "s1",
		Message:           "继续展开这个",
		KnowledgeBaseIDs:  []string{"kb-a"},
		RestrictRetrieval: true,
	})
	if cc.followUpEvidenceCount != 1 {
		t.Fatalf("followUpEvidenceCount = %d, want 1", cc.followUpEvidenceCount)
	}
	if !strings.Contains(cc.runtimeInstruction, "STILL_IN_SCOPE") || strings.Contains(cc.runtimeInstruction, "NOW_OUT_OF_SCOPE") {
		t.Fatalf("unexpected runtimeInstruction: %q", cc.runtimeInstruction)
	}
}
