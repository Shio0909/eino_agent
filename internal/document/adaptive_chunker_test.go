package document

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestNewChunkerPicksAdaptiveForAutoAndAdaptive(t *testing.T) {
	for _, strategy := range []string{"auto", "adaptive", ""} {
		for _, name := range []string{"a.md", "a.txt", ""} {
			if _, ok := NewChunker(strategy, 300, 30, name).(*AdaptiveChunker); !ok {
				t.Errorf("strategy %q file %q: got %T, want *AdaptiveChunker", strategy, name, NewChunker(strategy, 300, 30, name))
			}
		}
	}
}

func TestNewChunkerKeepsExplicitLegacyStrategies(t *testing.T) {
	if _, ok := NewChunker("markdown", 300, 30, "a.txt").(*MarkdownChunker); !ok {
		t.Errorf("markdown: got %T", NewChunker("markdown", 300, 30, "a.txt"))
	}
	if _, ok := NewChunker("recursive", 300, 30, "a.md").(*RecursiveCharacterChunker); !ok {
		t.Errorf("recursive: got %T", NewChunker("recursive", 300, 30, "a.md"))
	}
}

func TestSemanticWithoutEmbedderFallsBackToAdaptive(t *testing.T) {
	if _, ok := NewChunker("semantic", 300, 30, "a.md").(*AdaptiveChunker); !ok {
		t.Fatalf("got %T", NewChunker("semantic", 300, 30, "a.md"))
	}
}

func chunkContents(t *testing.T, strategy, name, text string, size, overlap int) []string {
	t.Helper()
	docs, err := NewChunker(strategy, size, overlap, name).Chunk(context.Background(), &RawDocument{ID: "d1", Source: name, Content: text})
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, len(docs))
	for i, d := range docs {
		out[i] = d.Content
	}
	return out
}

func TestAdaptiveChunkerKeepsHeadingPathAndCodeWhole(t *testing.T) {
	var b strings.Builder
	b.WriteString("# 运维手册\n\n")
	for i := 1; i <= 3; i++ {
		fmt.Fprintf(&b, "## 步骤%d\n\n", i)
		b.WriteString(strings.Repeat("先检查服务状态再继续下一步操作。", 10))
		b.WriteString("\n\n```bash\nsystemctl status app\njournalctl -u app -n 50\n```\n\n")
	}
	chunks := chunkContents(t, "auto", "ops.md", b.String(), 300, 30)

	if len(chunks) < 3 {
		t.Fatalf("expected a chunk per step, got %d", len(chunks))
	}
	for i, c := range chunks {
		if strings.Count(c, "```")%2 != 0 {
			t.Errorf("chunk %d cuts the code block:\n%s", i, c)
		}
		if !strings.Contains(c, "# 运维手册") {
			t.Errorf("chunk %d lost the heading path:\n%s", i, c)
		}
	}
}

func TestAdaptiveChunkerHonorsSizeAndMetadata(t *testing.T) {
	text := strings.Repeat("今天天气很好，适合出门散步。我们一起去公园看看花。", 60)
	docs, err := NewChunker("auto", 150, 10, "n.txt").Chunk(context.Background(), &RawDocument{ID: "doc", Source: "n.txt", Content: text})
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) < 5 {
		t.Fatalf("got %d chunks", len(docs))
	}
	for i, d := range docs {
		if n := utf8.RuneCountInString(d.Content); n > 150 {
			t.Errorf("chunk %d has %d runes", i, n)
		}
		if d.ID != fmt.Sprintf("doc_chunk_%d", i) {
			t.Errorf("chunk %d id %q", i, d.ID)
		}
		if d.Metadata["chunk_index"] != i || d.Metadata["total_chunks"] != len(docs) || d.Metadata["source"] != "n.txt" {
			t.Errorf("chunk %d metadata %v", i, d.Metadata)
		}
	}
}

func TestAdaptiveChunkerZeroOverlapMeansNone(t *testing.T) {
	text := strings.Repeat("今天天气很好，适合出门散步。我们一起去公园看看花。", 60)
	chunks := chunkContents(t, "auto", "n.txt", text, 150, 0)
	if got := strings.Join(chunks, ""); got != text {
		t.Fatalf("zero overlap should reassemble the text exactly: %d runes vs %d", utf8.RuneCountInString(got), utf8.RuneCountInString(text))
	}
}

func TestAdaptiveChunkerEmptyDocument(t *testing.T) {
	docs, err := NewChunker("auto", 150, 10, "e.txt").Chunk(context.Background(), &RawDocument{ID: "e", Content: "  \n\n "})
	if err != nil || len(docs) != 0 {
		t.Fatalf("got %d docs, err %v", len(docs), err)
	}
}

func TestAdaptiveChunkerNormalizesLineEndings(t *testing.T) {
	var b strings.Builder
	b.WriteString("# 手册\r\n\r\n")
	for i := 1; i <= 3; i++ {
		fmt.Fprintf(&b, "## 章节%d\r\n\r\n%s\r\n\r\n", i, strings.Repeat("这一章说明具体的操作步骤。", 15))
	}
	chunks := chunkContents(t, "auto", "m.md", b.String(), 300, 30)
	if len(chunks) < 3 {
		t.Fatalf("got %d chunks", len(chunks))
	}
	for i, c := range chunks {
		if strings.Contains(c, "\r") {
			t.Errorf("chunk %d keeps a carriage return", i)
		}
		if !strings.Contains(c, "# 手册") {
			t.Errorf("chunk %d lost its heading path: headings were not recognised in CRLF text:\n%s", i, c)
		}
	}
}

func TestAdaptiveChunkerSplitsAnInlineHTMLTableBetweenRows(t *testing.T) {
	var table strings.Builder
	table.WriteString(`<table><tr><td colspan="3">汇总</td></tr>`)
	for i := 0; i < 150; i++ {
		fmt.Fprintf(&table, "<tr><td>项目%03d</td><td>数值%03d</td><td>备注说明%03d</td></tr>", i, i, i)
	}
	table.WriteString("</table>")

	const size = 256
	contents := chunkContents(t, "auto", "ocr.md", "# 数据\n\n"+table.String(), size, 0)

	if len(contents) < 4 {
		t.Fatalf("want the table split into several chunks, got %d", len(contents))
	}
	for i, c := range contents {
		if n := utf8.RuneCountInString(c); n > 2*size {
			t.Fatalf("chunk %d has %d runes, over 2*%d", i, n, size)
		}
		for _, line := range strings.Split(c, "\n") {
			line = strings.TrimSuffix(strings.TrimSpace(line), "</table>")
			if strings.HasPrefix(line, "<tr") && !strings.HasSuffix(line, "</tr>") {
				t.Fatalf("chunk %d cuts a row: %q", i, line)
			}
		}
	}
}

func TestAdaptiveChunkerTurnsAnInlineHTMLTableIntoMarkdown(t *testing.T) {
	text := "# 报告\n\n<table><tr><td>指标</td><td>数值</td></tr><tr><td>营收</td><td>10亿</td></tr></table>\n\n结尾。"
	joined := strings.Join(chunkContents(t, "auto", "ocr.md", text, 400, 0), "\n")
	if strings.Contains(joined, "<table") || !strings.Contains(joined, "| 营收 | 10亿 |") {
		t.Fatalf("expected a Markdown table in the chunks, got:\n%s", joined)
	}
}
