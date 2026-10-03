package chunking

import (
	"strings"
	"testing"
)

// The overlap carried into the next chunk is cut from the tail of the
// previous one. When that tail is the end of a fenced code block or a table,
// the next chunk used to open with a dangling "}\n```" or a headerless row.
func TestSplitText_OverlapNeverStartsInsideCodeFence(t *testing.T) {
	code := "```json\n{\n  \"alpha\": 1,\n  \"beta\": 2,\n  \"gamma\": 3\n}\n```"
	text := strings.Repeat("前言段落。", 12) + "\n\n" + code + "\n\n" +
		strings.Repeat("后续说明内容很长。", 30)

	chunks := SplitText(text, SplitterConfig{ChunkSize: 120, ChunkOverlap: 40, Separators: []string{"\n\n", "\n", "。"}})

	sawCode := false
	for i, c := range chunks {
		fences := strings.Count(c.Content, "```")
		if fences > 0 {
			sawCode = true
		}
		if fences%2 != 0 {
			t.Errorf("chunk %d cuts through the code fence:\n%q", i, c.Content)
		}
	}
	if !sawCode {
		t.Fatal("test text did not produce a chunk containing the code block")
	}
}

func TestSplitText_OverlapNeverStartsInsideTable(t *testing.T) {
	table := "| 名称 | 数值 |\n| --- | --- |\n| a | 1 |\n| b | 2 |\n| c | 3 |\n"
	text := strings.Repeat("表格前的说明文字。", 8) + "\n\n" + table + "\n" +
		strings.Repeat("表格之后的长段落内容。", 20)

	chunks := SplitText(text, SplitterConfig{ChunkSize: 140, ChunkOverlap: 40, Separators: []string{"\n\n", "\n", "。"}})

	for i, c := range chunks {
		first := strings.TrimSpace(strings.SplitN(c.Content, "\n", 2)[0])
		if !strings.HasPrefix(first, "|") {
			continue
		}
		if !strings.Contains(c.Content, "---") {
			t.Errorf("chunk %d starts with a table row but has no header:\n%q", i, c.Content)
		}
	}
}
