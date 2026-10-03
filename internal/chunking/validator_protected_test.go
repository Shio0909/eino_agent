package chunking

import (
	"fmt"
	"strings"
	"testing"
)

// A code block or table is kept whole on purpose, so a chunk that is large only
// because of it must not make the validator throw away the heading split.
func TestSplitWithDiagnostics_BigCodeBlockKeepsHeadingTier(t *testing.T) {
	var b strings.Builder
	b.WriteString("# 手册\n\n")
	for i := 1; i <= 4; i++ {
		fmt.Fprintf(&b, "## 步骤%d\n\n%s\n\n", i, strings.Repeat("先检查服务状态再继续下一步。", 8))
	}
	b.WriteString("## 配置示例\n\n```yaml\n")
	for i := 0; i < 80; i++ {
		fmt.Fprintf(&b, "key_%02d: value-%02d-with-some-padding\n", i, i)
	}
	b.WriteString("```\n\n## 结尾\n\n" + strings.Repeat("收尾说明。", 10) + "\n")

	chunks, diag := SplitWithDiagnostics(b.String(), SplitterConfig{ChunkSize: 300, ChunkOverlap: 30, Strategy: StrategyAuto})

	if diag.SelectedTier != TierHeading {
		t.Fatalf("selected %s, rejected %+v; want the heading tier", diag.SelectedTier, diag.Rejected)
	}
	big := 0
	for _, c := range chunks {
		if runeLen(c.Content) > 2*300 {
			big++
		}
		if c.ContextHeader == "" {
			t.Errorf("chunk %d has no heading path", c.Seq)
		}
	}
	if big == 0 {
		t.Fatal("test document should produce a chunk over twice the chunk size")
	}
}

func TestValidateChunks_AcceptsOversizeProtectedBlock(t *testing.T) {
	code := "```go\n" + strings.Repeat("fmt.Println(\"x\")\n", 80) + "```"
	chunks := []Chunk{
		{Content: strings.Repeat("正文说明内容。", 40), Seq: 0},
		{Content: "前文。\n" + code, Seq: 1},
	}
	if v := ValidateChunks(chunks, 3000, 300); !v.OK {
		t.Fatalf("rejected: %s", v.Reason)
	}
}

func TestValidateChunks_RejectsOversizePlainText(t *testing.T) {
	chunks := []Chunk{
		{Content: strings.Repeat("正文说明内容。", 40), Seq: 0},
		{Content: strings.Repeat("没有任何结构的超长文字。", 80), Seq: 1},
	}
	v := ValidateChunks(chunks, 3000, 300)
	if v.OK {
		t.Fatal("a plain-text chunk far over the size budget must still be rejected")
	}
}

func TestValidateChunks_RejectsOversizeBlockWithLongProse(t *testing.T) {
	code := "```go\nx := 1\n```"
	chunks := []Chunk{
		{Content: strings.Repeat("正文说明内容。", 40), Seq: 0},
		{Content: strings.Repeat("代码前面堆了一大段没有切开的文字。", 60) + code, Seq: 1},
	}
	if v := ValidateChunks(chunks, 3000, 300); v.OK {
		t.Fatal("a small code block must not excuse a chunk that is mostly unsplit prose")
	}
}
