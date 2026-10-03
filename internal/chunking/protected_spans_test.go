package chunking

import (
	"strconv"
	"strings"
	"testing"
)

// buildOCRStrayBracketDoc models an OCR failure: a full-width "【公章】" whose
// opening bracket was read as a half-width '[' and whose closing bracket was
// lost, followed much later by an unrelated "](...)" fragment. A link pattern
// that may cross lines pairs the two and turns the whole document into one
// protected span that cannot be split.
func buildOCRStrayBracketDoc() string {
	var sb strings.Builder
	sb.WriteString("[公章】\n\n")
	for i := 0; i < 60; i++ {
		sb.WriteString("这是第")
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString("段用于测试切分的中文内容，包含常见标点符号。\n\n")
	}
	sb.WriteString("第三段（北京)\n\n")
	sb.WriteString("[引用](来源)")
	return sb.String()
}

func maxChunkRunes(chunks []Chunk) int {
	best := 0
	for _, c := range chunks {
		if n := len([]rune(c.Content)); n > best {
			best = n
		}
	}
	return best
}

func TestSplitText_StrayBracketDoesNotSwallowTheDocument(t *testing.T) {
	const chunkSize = 128
	cfg := SplitterConfig{ChunkSize: chunkSize, Separators: []string{"\n\n", "\n", "。"}}

	chunks := SplitText(buildOCRStrayBracketDoc(), cfg)
	if len(chunks) < 2 {
		t.Fatalf("want several chunks, got %d (largest %d runes)", len(chunks), maxChunkRunes(chunks))
	}
	if got := maxChunkRunes(chunks); got > 2*chunkSize {
		t.Fatalf("largest chunk is %d runes, over 2*chunkSize=%d: the stray '[' swallowed the document", got, 2*chunkSize)
	}
}

func TestSplit_StrayBracketDoesNotSwallowTheDocument(t *testing.T) {
	const chunkSize = 128
	cfg := SplitterConfig{
		ChunkSize:  chunkSize,
		Separators: []string{"\n\n", "\n", "。"},
		Strategy:   StrategyLegacy,
	}

	chunks := Split(buildOCRStrayBracketDoc(), cfg)
	if len(chunks) < 2 {
		t.Fatalf("want several chunks, got %d (largest %d runes)", len(chunks), maxChunkRunes(chunks))
	}
	if got := maxChunkRunes(chunks); got > 2*chunkSize {
		t.Fatalf("largest chunk is %d runes, over 2*chunkSize=%d", got, 2*chunkSize)
	}
}

func TestProtectedSpans_LinkAndImageNeverCrossALine(t *testing.T) {
	doc := buildOCRStrayBracketDoc()
	for _, s := range protectedSpans(doc) {
		if seg := doc[s.start:s.end]; strings.Contains(seg, "\n") {
			t.Fatalf("a protected span crosses a line (%d runes, starts %q)", len([]rune(seg)), string([]rune(seg)[:12]))
		}
	}
}

func TestProtectedSpans_StrayBracketWithoutALinkIsNotProtected(t *testing.T) {
	doc := "[公章】\n\n第一段。\n\n第二段。\n\n第三段（北京)"
	if spans := protectedSpans(doc); len(spans) != 0 {
		t.Fatalf("want no protected span, got %v", spans)
	}
}

func TestProtectedSpans_LinkTextBrokenAcrossLinesStaysUnprotected(t *testing.T) {
	doc := "[link\ntext](http://example.com)"
	if spans := protectedSpans(doc); len(spans) != 0 {
		t.Fatalf("want a link broken across lines left unprotected, got %v", spans)
	}
}

func TestProtectedSpans_WellFormedLinksAndImagesStayWhole(t *testing.T) {
	for _, c := range []string{
		"![alt](http://x/y.png)",
		"[文字](http://x.com)",
		`[link](url "title")`,
		"![](resource://abc)",
	} {
		spans := protectedSpans(c)
		if len(spans) == 0 {
			t.Errorf("%q should be protected, got no span", c)
			continue
		}
		if spans[0].start != 0 || spans[0].end != len(c) {
			t.Errorf("%q: span = [%d,%d), want [0,%d)", c, spans[0].start, spans[0].end, len(c))
		}
	}
}

func TestProtectedSpans_LinkBoundsAreEnforced(t *testing.T) {
	long := func(n int) string { return strings.Repeat("a", n) }
	cases := []struct {
		name string
		text string
		want bool
	}{
		{"text at the limit", "[" + long(200) + "](u)", true},
		{"text over the limit", "[" + long(201) + "](u)", false},
		{"destination at the limit", "[t](" + long(500) + ")", true},
		{"destination over the limit", "[t](" + long(501) + ")", false},
		{"image alt over the limit", "![" + long(201) + "](u)", false},
		{"image with empty alt", "![](u)", true},
		{"link with empty text", "[](u)", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spans := protectedSpans(tc.text)
			if got := len(spans) > 0 && spans[0].start == 0 && spans[0].end == len(tc.text); got != tc.want {
				t.Fatalf("protected = %v, want %v (spans %v)", got, tc.want, spans)
			}
		})
	}
}

func TestProtectedSpans_ImageBrokenAcrossLinesStaysUnprotected(t *testing.T) {
	for _, doc := range []string{"![alt\ntext](http://x/y.png)", "![alt](http://x/\ny.png)", "[t](http://x/\ny)"} {
		if spans := protectedSpans(doc); len(spans) != 0 {
			t.Fatalf("%q: want no protected span, got %v", doc, spans)
		}
	}
}
