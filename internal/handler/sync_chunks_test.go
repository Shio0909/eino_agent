package handler

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"sync"
	"testing"

	einoembedding "github.com/cloudwego/eino/components/embedding"

	"eino_agent/internal/config"
	"eino_agent/internal/container"
	"eino_agent/internal/database/repository"
	"eino_agent/internal/document"
)

// statefulChunkRepo is an in-memory chunk table that behaves like the real one
// where it matters here: upsert by id, and a stable order by chunk_index.
type statefulChunkRepo struct {
	mu      sync.Mutex
	rows    map[string]*repository.Chunk
	failFn  map[string]error // method name -> error to return once
	created int
}

func newStatefulChunkRepo() *statefulChunkRepo {
	return &statefulChunkRepo{rows: map[string]*repository.Chunk{}, failFn: map[string]error{}}
}

func (r *statefulChunkRepo) take(name string) error {
	if err := r.failFn[name]; err != nil {
		delete(r.failFn, name)
		return err
	}
	return nil
}

func (r *statefulChunkRepo) BatchCreate(_ context.Context, chunks []*repository.Chunk) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.take("BatchCreate"); err != nil {
		return err
	}
	for _, c := range chunks {
		cp := *c
		r.rows[c.ID] = &cp
		r.created++
	}
	return nil
}

func (r *statefulChunkRepo) GetByKnowledgeID(_ context.Context, knowledgeID string) ([]*repository.Chunk, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.take("GetByKnowledgeID"); err != nil {
		return nil, err
	}
	var out []*repository.Chunk
	for _, c := range r.rows {
		if c.KnowledgeID == knowledgeID {
			cp := *c
			out = append(out, &cp)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ChunkIndex != out[j].ChunkIndex {
			return out[i].ChunkIndex < out[j].ChunkIndex
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (r *statefulChunkRepo) GetHashesByKnowledgeID(ctx context.Context, knowledgeID string) (map[string]string, error) {
	chunks, _ := r.GetByKnowledgeID(ctx, knowledgeID)
	out := map[string]string{}
	for _, c := range chunks {
		out[c.ContentHash] = c.ID
	}
	return out, nil
}

func (r *statefulChunkRepo) DeleteByIDs(_ context.Context, ids []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.take("DeleteByIDs"); err != nil {
		return err
	}
	for _, id := range ids {
		delete(r.rows, id)
	}
	return nil
}

func (r *statefulChunkRepo) DeleteByKnowledgeID(_ context.Context, knowledgeID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, c := range r.rows {
		if c.KnowledgeID == knowledgeID {
			delete(r.rows, id)
		}
	}
	return nil
}

func (r *statefulChunkRepo) UpdateChunkIndexes(_ context.Context, indexes map[string]int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.take("UpdateChunkIndexes"); err != nil {
		return err
	}
	for id, idx := range indexes {
		if c, ok := r.rows[id]; ok {
			c.ChunkIndex = idx
		}
	}
	return nil
}

// statefulVectorDB keeps every document it is given, so a test can see exactly
// what a search would be able to return.
type statefulVectorDB struct {
	mu     sync.Mutex
	docs   map[string]*container.Document
	failFn map[string]error
}

func newStatefulVectorDB() *statefulVectorDB {
	return &statefulVectorDB{docs: map[string]*container.Document{}, failFn: map[string]error{}}
}

func (v *statefulVectorDB) take(name string) error {
	if err := v.failFn[name]; err != nil {
		delete(v.failFn, name)
		return err
	}
	return nil
}

func (v *statefulVectorDB) Upsert(_ context.Context, docs []*container.Document) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if err := v.take("Upsert"); err != nil {
		return err
	}
	for _, d := range docs {
		cp := *d
		cp.Metadata = map[string]interface{}{}
		for k, val := range d.Metadata {
			cp.Metadata[k] = val
		}
		v.docs[d.ID] = &cp
	}
	return nil
}
func (v *statefulVectorDB) Delete(_ context.Context, ids []string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if err := v.take("Delete"); err != nil {
		return err
	}
	for _, id := range ids {
		delete(v.docs, id)
	}
	return nil
}
func (v *statefulVectorDB) DeleteByKnowledgeID(_ context.Context, knowledgeID string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	for id, d := range v.docs {
		if d.Metadata["knowledge_id"] == knowledgeID {
			delete(v.docs, id)
		}
	}
	return nil
}
func (v *statefulVectorDB) UpdateChunkIndexes(_ context.Context, indexes map[string]int) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	for id, idx := range indexes {
		if d, ok := v.docs[id]; ok {
			d.Metadata["chunk_index"] = idx
		}
	}
	return nil
}
func (v *statefulVectorDB) DeleteByKnowledgeBaseID(context.Context, string) error { return nil }
func (v *statefulVectorDB) Ping(context.Context) error                            { return nil }
func (v *statefulVectorDB) Close() error                                          { return nil }
func (v *statefulVectorDB) Search(context.Context, []float32, int, container.SearchFilter) ([]*container.Document, error) {
	return nil, nil
}
func (v *statefulVectorDB) SearchKeyword(context.Context, string, int, container.SearchFilter) ([]*container.Document, error) {
	return nil, nil
}

func (v *statefulVectorDB) forKnowledge(knowledgeID string) map[string]*container.Document {
	v.mu.Lock()
	defer v.mu.Unlock()
	out := map[string]*container.Document{}
	for id, d := range v.docs {
		if d.Metadata["knowledge_id"] == knowledgeID {
			out[id] = d
		}
	}
	return out
}

type countingEmbedder struct {
	mu    sync.Mutex
	texts int
	fail  error
}

func (e *countingEmbedder) EmbedStrings(_ context.Context, texts []string, _ ...einoembedding.Option) ([][]float64, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.fail != nil {
		return nil, e.fail
	}
	e.texts += len(texts)
	out := make([][]float64, len(texts))
	for i, t := range texts {
		hs := fnv.New32a()
		_, _ = hs.Write([]byte(t))
		out[i] = []float64{float64(hs.Sum32()%97) + 1, 1, 0}
	}
	return out, nil
}

func (e *countingEmbedder) reset() {
	e.mu.Lock()
	e.texts = 0
	e.mu.Unlock()
}

type syncEnv struct {
	h        *Handler
	chunks   *statefulChunkRepo
	vectors  *statefulVectorDB
	embedder *countingEmbedder
}

func newSyncEnv() *syncEnv {
	env := &syncEnv{chunks: newStatefulChunkRepo(), vectors: newStatefulVectorDB(), embedder: &countingEmbedder{}}
	env.h = &Handler{
		cfg:       &config.Config{RAG: config.RAGConfig{ChunkStrategy: "markdown", ChunkSize: 400, ChunkOverlap: 0}},
		embedding: env.embedder,
		vectorDB:  env.vectors,
		chunkRepo: env.chunks,
		knowledgeRepo: newFakeKnowledgeRepo(map[string]*repository.Knowledge{
			syncDoc: {ID: syncDoc, KnowledgeBaseID: syncKB, SourceType: "file", FileName: syncName},
		}),
	}
	return env
}

const (
	syncKB   = "kb-1"
	syncDoc  = "doc-1"
	syncName = "ops.md"
)

func sectionText(i int) string {
	return fmt.Sprintf("## Section %02d\n\nProcedure %02d: inspect the subsystem, record the readings, and escalate when a reading drifts.\n\n", i, i)
}

func syncTestDoc(sections []string) string {
	return "# Operations\n\n" + strings.Join(sections, "")
}

func numberedSections(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = sectionText(i)
	}
	return out
}

// expectedChunks is what the handler's chunker makes of a document, so tests
// compare the index against the parser's own output instead of a hand count.
func (e *syncEnv) expectedChunks(t *testing.T, content string) []string {
	t.Helper()
	chunker := document.NewChunker(e.h.cfg.RAG.ChunkStrategy, e.h.cfg.RAG.ChunkSize, e.h.cfg.RAG.ChunkOverlap, syncName)
	docs, err := chunker.Chunk(context.Background(), &document.RawDocument{ID: syncDoc, Source: syncName, Content: content})
	if err != nil {
		t.Fatalf("chunk: %v", err)
	}
	out := make([]string, 0, len(docs))
	for _, d := range docs {
		out = append(out, d.Content)
	}
	return out
}

func (e *syncEnv) firstImport(t *testing.T, content string) {
	t.Helper()
	if _, err := e.h.processPlainTextDocument(context.Background(), syncKB, syncDoc, syncName, []byte(content)); err != nil {
		t.Fatalf("first import: %v", err)
	}
	e.embedder.reset()
}

func (e *syncEnv) sync(content string) (*syncResult, error) {
	chunks, err := e.h.chunkPlainText(context.Background(), syncDoc, syncName, []byte(content))
	if err != nil {
		return nil, err
	}
	return e.h.applySync(context.Background(), syncKB, syncDoc, syncName, chunks, contentHash(content), syncOptions{})
}

// assertIndex checks that what the document preview shows (Postgres chunks in
// order) and what search can find (vector documents) are the same chunks, in
// the document's order, numbered 0..n-1.
func (e *syncEnv) assertIndex(t *testing.T, want []string) {
	t.Helper()
	rows, err := e.chunks.GetByKnowledgeID(context.Background(), syncDoc)
	if err != nil {
		t.Fatalf("read chunks: %v", err)
	}
	if len(rows) != len(want) {
		t.Fatalf("stored chunks = %d, want %d", len(rows), len(want))
	}
	vecs := e.vectors.forKnowledge(syncDoc)
	if len(vecs) != len(want) {
		t.Fatalf("searchable vectors = %d, want %d", len(vecs), len(want))
	}
	for i, row := range rows {
		if row.ChunkIndex != i {
			t.Fatalf("chunk at position %d has chunk_index %d", i, row.ChunkIndex)
		}
		if row.Content != want[i] {
			t.Fatalf("chunk %d content = %q, want %q", i, row.Content, want[i])
		}
		vec, ok := vecs[row.ID]
		if !ok {
			t.Fatalf("chunk %d (%s) has no vector", i, row.ID)
		}
		if got, _ := vec.Metadata["chunk_index"].(int); got != i {
			t.Fatalf("vector for chunk %d carries chunk_index %v", i, vec.Metadata["chunk_index"])
		}
	}
}

func TestSyncInsertAtStartEmbedsOnlyTheNewChunkAndRenumbersTheRest(t *testing.T) {
	env := newSyncEnv()
	sections := numberedSections(12)
	env.firstImport(t, syncTestDoc(sections))

	edited := syncTestDoc(append([]string{"## Preface\n\nRead this before anything else; it changes the order of everything below.\n\n"}, sections...))
	res, err := env.sync(edited)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if env.embedder.texts != res.Added || res.Added != 1 {
		t.Fatalf("embedded %d texts, result added %d; want exactly the 1 new chunk", env.embedder.texts, res.Added)
	}
	env.assertIndex(t, env.expectedChunks(t, edited))
}

func TestSyncInsertAndDeleteKeepsPreviewOrderMatchingTheDocument(t *testing.T) {
	env := newSyncEnv()
	sections := numberedSections(20)
	env.firstImport(t, syncTestDoc(sections))

	var edited []string
	for i := 0; i < 3; i++ {
		edited = append(edited, fmt.Sprintf("## Inserted %d\n\nA paragraph added in front of the existing material, number %d.\n\n", i, i))
	}
	for i, s := range sections {
		if i == 7 || i == 8 || i == 15 {
			continue
		}
		edited = append(edited, s)
	}
	doc := syncTestDoc(edited)
	if _, err := env.sync(doc); err != nil {
		t.Fatalf("sync: %v", err)
	}
	env.assertIndex(t, env.expectedChunks(t, doc))
}

func TestSyncEmbeddingFailureLeavesTheCurrentIndexUntouched(t *testing.T) {
	env := newSyncEnv()
	sections := numberedSections(10)
	original := syncTestDoc(sections)
	env.firstImport(t, original)

	sections[4] = "## Section 04\n\nRewritten so that it has to be embedded again.\n\n"
	env.embedder.fail = errors.New("embedding service down")
	if _, err := env.sync(syncTestDoc(sections)); err == nil {
		t.Fatal("sync succeeded although embedding failed")
	}
	env.embedder.fail = nil

	env.assertIndex(t, env.expectedChunks(t, original))
}

func TestSyncChunkStoreFailureIsReportedAndLeavesNoStrayVectors(t *testing.T) {
	env := newSyncEnv()
	sections := numberedSections(10)
	original := syncTestDoc(sections)
	env.firstImport(t, original)

	sections[4] = "## Section 04\n\nRewritten so that it has to be stored again.\n\n"
	env.chunks.failFn["BatchCreate"] = errors.New("chunk insert failed")
	if _, err := env.sync(syncTestDoc(sections)); err == nil {
		t.Fatal("sync reported success although the chunk rows were not written")
	}

	env.assertIndex(t, env.expectedChunks(t, original))
}

func TestSyncVectorDeleteFailureIsReportedAndARetryConverges(t *testing.T) {
	env := newSyncEnv()
	sections := numberedSections(10)
	env.firstImport(t, syncTestDoc(sections))

	sections[4] = "## Section 04\n\nRewritten; the old text must stop being searchable.\n\n"
	edited := syncTestDoc(sections)
	env.vectors.failFn["Delete"] = errors.New("vector delete failed")
	if _, err := env.sync(edited); err == nil {
		t.Fatal("sync reported success although the old vectors were not removed")
	}

	if _, err := env.sync(edited); err != nil {
		t.Fatalf("retry: %v", err)
	}
	env.assertIndex(t, env.expectedChunks(t, edited))
	for _, d := range env.vectors.forKnowledge(syncDoc) {
		if strings.Contains(d.Content, "Procedure 04") {
			t.Fatalf("stale content is still searchable: %q", d.Content)
		}
	}
}

func TestSyncIsIdempotentWhenNothingChanged(t *testing.T) {
	env := newSyncEnv()
	doc := syncTestDoc(numberedSections(8))
	env.firstImport(t, doc)

	res, err := env.sync(doc)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if res.Added != 0 || res.Removed != 0 || env.embedder.texts != 0 {
		t.Fatalf("unchanged document: added=%d removed=%d embedded=%d", res.Added, res.Removed, env.embedder.texts)
	}
	env.assertIndex(t, env.expectedChunks(t, doc))
}
