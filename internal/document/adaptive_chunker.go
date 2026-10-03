package document

import (
	"context"
	"fmt"
	"strings"

	"eino_agent/internal/chunking"
	"eino_agent/internal/container"
)

// AdaptiveChunker splits a document along its own structure: Markdown
// headings or chapter markers when it has them, paragraphs and sentences when
// it does not. Code blocks, tables, math and links are never cut, and each
// chunk under a heading starts with that heading's path.
type AdaptiveChunker struct {
	ChunkSize    int
	ChunkOverlap int
}

// NewAdaptiveChunker creates an adaptive chunker. An overlap of 0 means none.
func NewAdaptiveChunker(chunkSize, chunkOverlap int) *AdaptiveChunker {
	return &AdaptiveChunker{ChunkSize: chunkSize, ChunkOverlap: chunkOverlap}
}

// Chunk implements Chunker.
func (c *AdaptiveChunker) Chunk(_ context.Context, doc *RawDocument) ([]*container.Document, error) {
	text := chunking.NormalizeLineEndings(strings.TrimSpace(doc.Content))
	pieces := chunking.Split(text, chunking.SplitterConfig{
		ChunkSize:    c.ChunkSize,
		ChunkOverlap: max(c.ChunkOverlap, 0),
		Separators:   chunking.TextSeparators,
		Strategy:     chunking.StrategyAuto,
	})

	contents := make([]string, 0, len(pieces))
	for _, p := range pieces {
		if content := p.EmbeddingContent(); content != "" {
			contents = append(contents, content)
		}
	}

	documents := make([]*container.Document, len(contents))
	for i, content := range contents {
		documents[i] = &container.Document{
			ID:      fmt.Sprintf("%s_chunk_%d", doc.ID, i),
			Content: content,
			Metadata: map[string]interface{}{
				"source":       doc.Source,
				"chunk_index":  i,
				"total_chunks": len(contents),
				"doc_id":       doc.ID,
			},
		}
	}
	return documents, nil
}
