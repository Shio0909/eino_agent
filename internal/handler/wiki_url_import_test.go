package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"

	cachepkg "eino_agent/internal/cache"
	"eino_agent/internal/config"
	"eino_agent/internal/database/repository"
	"eino_agent/internal/docreader"
	"eino_agent/internal/wiki"
)

type fakeWikiRepo struct {
	pages                  []*repository.WikiPage
	deletedSourceKnowledge []string
}

func (r *fakeWikiRepo) UpsertPage(_ context.Context, page *repository.WikiPage) error {
	if page.ID == "" {
		page.ID = page.Path
	}
	for i, existing := range r.pages {
		if existing.KnowledgeBaseID == page.KnowledgeBaseID && existing.Path == page.Path {
			r.pages[i] = page
			return nil
		}
	}
	r.pages = append(r.pages, page)
	return nil
}
func (r *fakeWikiRepo) BatchUpsertPages(ctx context.Context, pages []*repository.WikiPage) error {
	for _, page := range pages {
		if err := r.UpsertPage(ctx, page); err != nil {
			return err
		}
	}
	return nil
}
func (r *fakeWikiRepo) GetPageByPath(_ context.Context, kbID, path string) (*repository.WikiPage, error) {
	for _, page := range r.pages {
		if page.KnowledgeBaseID == kbID && page.Path == path {
			return page, nil
		}
	}
	return nil, nil
}
func (r *fakeWikiRepo) ListPages(_ context.Context, kbID string) ([]*repository.WikiPage, error) {
	result := make([]*repository.WikiPage, 0)
	for _, page := range r.pages {
		if page.KnowledgeBaseID == kbID {
			result = append(result, page)
		}
	}
	return result, nil
}
func (r *fakeWikiRepo) SearchPages(context.Context, string, string, int) ([]*repository.WikiPage, error) {
	return nil, nil
}
func (r *fakeWikiRepo) DeletePagesByKnowledgeBase(_ context.Context, kbID string) error {
	kept := r.pages[:0]
	for _, page := range r.pages {
		if page.KnowledgeBaseID != kbID {
			kept = append(kept, page)
		}
	}
	r.pages = kept
	return nil
}
func (r *fakeWikiRepo) DeletePagesBySourceKnowledge(_ context.Context, sourceKnowledgeID string) error {
	r.deletedSourceKnowledge = append(r.deletedSourceKnowledge, sourceKnowledgeID)
	kept := r.pages[:0]
	for _, page := range r.pages {
		if page.SourceKnowledgeID == nil || *page.SourceKnowledgeID != sourceKnowledgeID {
			kept = append(kept, page)
		}
	}
	r.pages = kept
	return nil
}
func (r *fakeWikiRepo) UpsertLinks(context.Context, string, []*repository.WikiLink) error { return nil }
func (r *fakeWikiRepo) GetLinkedPages(context.Context, string) ([]*repository.WikiPage, error) {
	return nil, nil
}
func (r *fakeWikiRepo) ResolveLinks(context.Context, string) error { return nil }

type fakeChatModel struct{}

func (fakeChatModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	return &schema.Message{Content: `[{"path":"topic.md","title":"Topic","content":"# Topic\n\nCompiled wiki content","type":"topic","links":[]}]`}, nil
}
func (fakeChatModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, nil
}
func (fakeChatModel) BindTools([]*schema.ToolInfo) error { return nil }

func TestUploadDocumentURLWikiModeCompilesSynchronously(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html><body><h1>Topic</h1><p>Raw URL content</p></body></html>"))
	}))
	defer server.Close()

	docReaderCfg := docreader.DefaultConfig()
	docReaderCfg.AllowPrivateNetworks = true
	docReaderCfg.RenderMode = "disabled"
	docReaderCfg.PlaywrightCommand = ""
	docReaderCli, err := docreader.NewClient(docReaderCfg)
	if err != nil {
		t.Fatalf("docreader.NewClient error = %v", err)
	}

	wikiRepo := &fakeWikiRepo{}
	knowledgeRepo := newFakeKnowledgeRepo(map[string]*repository.Knowledge{})
	h := &Handler{
		cfg: &config.Config{},
		kbRepo: &fakeKnowledgeBaseRepo{items: map[string]*repository.KnowledgeBase{
			"kb-wiki": {ID: "kb-wiki", TenantID: 1, Mode: "wiki"},
		}},
		knowledgeRepo:    knowledgeRepo,
		wikiRepo:         wikiRepo,
		wikiCompiler:     wiki.NewCompiler(fakeChatModel{}, wikiRepo),
		docReaderCli:     docReaderCli,
		importStateStore: cachepkg.NewNoopImportStateStore(),
		retrievalCache:   cachepkg.NewNoopRetrievalCache(),
	}
	h.cfg.Security.URLPolicy.AllowPrivateNetworks = true
	h.cfg.Security.URLPolicy.AllowedSchemes = []string{"http", "https"}

	body := bytes.NewBufferString(`{"url":` + strconv.Quote(server.URL) + `,"title":"URL Topic"}`)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/knowledge-bases/kb-wiki/documents/url", body)
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = gin.Params{{Key: "id", Value: "kb-wiki"}}

	h.UploadDocumentURL(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d, body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}
	if resp["mode"] != "wiki" {
		t.Fatalf("mode = %v, want wiki", resp["mode"])
	}
	if len(wikiRepo.pages) == 0 {
		t.Fatalf("expected compiled wiki pages")
	}
	if len(knowledgeRepo.items) != 1 {
		t.Fatalf("knowledge records = %d, want 1", len(knowledgeRepo.items))
	}
	for _, knowledge := range knowledgeRepo.items {
		if knowledge.ParseStatus != "completed" {
			t.Fatalf("parse status = %q, want completed", knowledge.ParseStatus)
		}
	}
}
