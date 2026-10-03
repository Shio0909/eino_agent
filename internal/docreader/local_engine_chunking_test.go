package docreader

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

func parseWith(t *testing.T, text, fileName, fileType string, opts *ParseOptions) *ParseResult {
	t.Helper()
	result, err := newLocalEngine(DefaultConfig()).ParseBytes(context.Background(), []byte(text), fileName, fileType, opts)
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	return result
}

func markdownGuide() string {
	var b strings.Builder
	b.WriteString("# 部署指南\n\n")
	for i := 1; i <= 4; i++ {
		fmt.Fprintf(&b, "## 第%d节 配置\n\n", i)
		b.WriteString(strings.Repeat("这一节说明如何完成配置并验证结果。", 14))
		b.WriteString("\n\n")
	}
	return b.String()
}

func TestLocalEngineChunkSizeFollowsOptions(t *testing.T) {
	text := strings.Repeat("今天天气很好，适合出门散步。我们一起去公园看看花。", 80)
	for _, size := range []int{120, 300} {
		opts := &ParseOptions{ChunkSize: size, ChunkOverlap: 20}
		result := parseWith(t, text, "a.txt", "text", opts)
		if len(result.Chunks) < 2 {
			t.Fatalf("size %d: expected several chunks, got %d", size, len(result.Chunks))
		}
		for i, c := range result.Chunks {
			if n := utf8.RuneCountInString(c.Content); n > size {
				t.Errorf("size %d: chunk %d has %d runes", size, i, n)
			}
		}
	}
}

func TestLocalEngineNeverCutsCodeFence(t *testing.T) {
	code := "```go\nfunc main() {\n\tfmt.Println(\"hello\")\n\tfmt.Println(\"world\")\n}\n```"
	text := strings.Repeat("说明文字用来铺垫。", 25) + "\n\n" + code + "\n\n" + strings.Repeat("收尾说明文字。", 30)
	result := parseWith(t, text, "a.md", "markdown", &ParseOptions{ChunkSize: 200, ChunkOverlap: 30})
	for i, c := range result.Chunks {
		if strings.Count(c.Content, "```")%2 != 0 {
			t.Errorf("chunk %d cuts the code fence:\n%s", i, c.Content)
		}
	}
}

func TestLocalEngineMarkdownChunksCarryHeadingPath(t *testing.T) {
	result := parseWith(t, markdownGuide(), "guide.md", "markdown", &ParseOptions{ChunkSize: 300, ChunkOverlap: 30})
	if len(result.Chunks) < 4 {
		t.Fatalf("expected at least one chunk per section, got %d", len(result.Chunks))
	}
	for i, c := range result.Chunks {
		if !strings.Contains(c.Content, "# 部署指南") {
			t.Errorf("chunk %d lost its heading path:\n%s", i, c.Content)
		}
	}
}

func TestLocalEngineChapterMarkersSplitPlainText(t *testing.T) {
	var b strings.Builder
	for i, name := range []string{"一", "二", "三", "四", "五", "六"} {
		fmt.Fprintf(&b, "第%s章 主题%d\n", name, i+1)
		b.WriteString(strings.Repeat("本章讨论的内容比较具体，需要逐条说明。", 10))
		b.WriteString("\n\n")
	}
	result := parseWith(t, b.String(), "report.txt", "text", &ParseOptions{ChunkSize: 400, ChunkOverlap: 0})
	if len(result.Chunks) < 2 {
		t.Fatalf("expected several chunks, got %d", len(result.Chunks))
	}
	for i, c := range result.Chunks {
		if !strings.HasPrefix(strings.TrimSpace(c.Content), "第") {
			t.Errorf("chunk %d does not start at a chapter marker:\n%s", i, c.Content)
		}
	}
}

func TestLocalEngineZeroOverlapMeansNoOverlap(t *testing.T) {
	text := strings.Repeat("今天天气很好，适合出门散步。我们一起去公园看看花。", 80)
	result := parseWith(t, text, "a.txt", "text", &ParseOptions{ChunkSize: 150, ChunkOverlap: 0})
	if len(result.Chunks) < 3 {
		t.Fatalf("expected several chunks, got %d", len(result.Chunks))
	}
	for i := 1; i < len(result.Chunks); i++ {
		if result.Chunks[i].Start < result.Chunks[i-1].End {
			t.Errorf("chunk %d starts at %d, inside the previous chunk that ends at %d",
				i, result.Chunks[i].Start, result.Chunks[i-1].End)
		}
	}
}

func TestLocalEngineRecursiveStrategySkipsHeadingPath(t *testing.T) {
	result := parseWith(t, markdownGuide(), "guide.md", "markdown",
		&ParseOptions{ChunkSize: 300, ChunkOverlap: 30, Strategy: "recursive"})
	prefixed := 0
	for _, c := range result.Chunks {
		if strings.HasPrefix(c.Content, "# 部署指南") {
			prefixed++
		}
	}
	if prefixed > 1 {
		t.Fatalf("recursive strategy should not add heading paths, %d chunks start with one", prefixed)
	}
}

func TestLocalEngineNormalizesLineEndings(t *testing.T) {
	text := strings.ReplaceAll(markdownGuide(), "\n", "\r\n")
	result := parseWith(t, text, "guide.md", "markdown", &ParseOptions{ChunkSize: 300, ChunkOverlap: 30})
	for i, c := range result.Chunks {
		if strings.Contains(c.Content, "\r") {
			t.Fatalf("chunk %d still has a carriage return", i)
		}
	}
}

func TestLocalEngineChunkOffsetsAreSequential(t *testing.T) {
	result := parseWith(t, markdownGuide(), "guide.md", "markdown", &ParseOptions{ChunkSize: 300, ChunkOverlap: 30})
	for i, c := range result.Chunks {
		if c.Seq != i {
			t.Errorf("chunk %d has Seq %d", i, c.Seq)
		}
		if c.End < c.Start {
			t.Errorf("chunk %d has End %d before Start %d", i, c.End, c.Start)
		}
	}
}

func TestLocalEngineSplitsAnInlineHTMLTableBetweenRows(t *testing.T) {
	var table strings.Builder
	table.WriteString(`<table><tr><td colspan="3">汇总</td></tr>`)
	for i := 0; i < 150; i++ {
		fmt.Fprintf(&table, "<tr><td>项目%03d</td><td>数值%03d</td><td>备注说明%03d</td></tr>", i, i, i)
	}
	table.WriteString("</table>")

	const size = 256
	result := parseWith(t, "# 数据\n\n"+table.String(), "ocr.md", "markdown", &ParseOptions{ChunkSize: size})

	if len(result.Chunks) < 4 {
		t.Fatalf("want the table split into several chunks, got %d", len(result.Chunks))
	}
	for i, c := range result.Chunks {
		if n := utf8.RuneCountInString(c.Content); n > 2*size {
			t.Fatalf("chunk %d has %d runes, over 2*%d", i, n, size)
		}
		for _, line := range strings.Split(c.Content, "\n") {
			line = strings.TrimSuffix(strings.TrimSpace(line), "</table>")
			if strings.HasPrefix(line, "<tr") && !strings.HasSuffix(line, "</tr>") {
				t.Fatalf("chunk %d cuts a row: %q", i, line)
			}
		}
	}
}

func TestLocalEngineTurnsAnInlineHTMLTableIntoMarkdown(t *testing.T) {
	text := "# 报告\n\n<table><tr><td>指标</td><td>数值</td></tr><tr><td>营收</td><td>10亿</td></tr></table>\n\n结尾。"
	result := parseWith(t, text, "ocr.md", "markdown", &ParseOptions{ChunkSize: 400})

	joined := ""
	for _, c := range result.Chunks {
		joined += c.Content + "\n"
	}
	if strings.Contains(joined, "<table") || !strings.Contains(joined, "| 营收 | 10亿 |") {
		t.Fatalf("expected a Markdown table in the chunks, got:\n%s", joined)
	}
}
