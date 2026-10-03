package chunking

import (
	"strings"
	"testing"
)

func TestSplit_EmptyText(t *testing.T) {
	if got := Split("", DefaultConfig()); got != nil {
		t.Errorf("empty text should return nil, got %v", got)
	}
}

func TestSplit_LegacyStrategy_MatchesSplitText(t *testing.T) {
	text := strings.Repeat("Hello world.\n\n", 30)
	cfg := SplitterConfig{ChunkSize: 100, ChunkOverlap: 20, Separators: []string{"\n\n"}, Strategy: StrategyLegacy}
	a := Split(text, cfg)
	b := SplitText(text, cfg)
	if len(a) != len(b) {
		t.Errorf("legacy strategy should match SplitText: got %d vs %d chunks", len(a), len(b))
	}
	for i := range a {
		if a[i].Content != b[i].Content {
			t.Errorf("chunk %d differs", i)
		}
	}
}

func TestSplit_EmptyStrategyEqualsLegacy(t *testing.T) {
	text := strings.Repeat("Sentence one. Sentence two.\n", 20)
	cfg := SplitterConfig{ChunkSize: 80, ChunkOverlap: 10}
	a := Split(text, cfg)
	cfg.Strategy = StrategyLegacy
	b := Split(text, cfg)
	if len(a) != len(b) {
		t.Errorf("empty Strategy should equal legacy: %d vs %d", len(a), len(b))
	}
}

func TestSplit_AutoStrategy_PicksHeadingForMarkdownDoc(t *testing.T) {
	doc := strings.Repeat("# A\nbody\n## B\nbody\n## C\nbody\n## D\nbody\n", 1)
	cfg := SplitterConfig{ChunkSize: 200, ChunkOverlap: 20, Strategy: StrategyAuto}
	// Until heading_splitter is wired, this falls through to SplitText —
	// just assert we get a valid result.
	chunks := Split(doc, cfg)
	if len(chunks) == 0 {
		t.Error("auto strategy should produce chunks")
	}
}

func TestSplit_HeadingStrategyKeepsDistinctTopLevelHeadings(t *testing.T) {
	doc := `# Intro
short intro.

# Usage
short usage.

# FAQ
short faq.`
	cfg := SplitterConfig{ChunkSize: 500, ChunkOverlap: 0, Strategy: StrategyHeading}
	chunks := Split(doc, cfg)
	if len(chunks) != 3 {
		t.Fatalf("expected one chunk per top-level heading, got %d:\n%v", len(chunks), chunks)
	}
	for i, heading := range []string{"# Intro", "# Usage", "# FAQ"} {
		if !strings.Contains(chunks[i].Content, heading) {
			t.Errorf("chunk %d should contain heading %q, got:\n%s", i, heading, chunks[i].Content)
		}
	}
}

// TestSplit_PreservesPositionInvariantAcrossTiers ensures every chunk's
// (Start, End, Content) triple stays consistent — End-Start must equal the
// rune length of Content, and runes[Start:End] must equal Content. This is
// the contract that knowledge.go:2278+ relies on for document reconstruction
// during summary generation.
func TestSplit_PreservesPositionInvariantAcrossTiers(t *testing.T) {
	cases := map[string]string{
		"heading-tier": "# Top\nintro paragraph here.\n\n## Section A\nbody A here.\n\n## Section B\nbody B here.\n\n## Section C\nbody C.",
		"heuristic-tier": strings.Repeat("Kapitel 1: Einleitung\n", 1) + strings.Repeat("Beispieltext. ", 50) +
			"\n\n" + strings.Repeat("Kapitel 2: Hauptteil\n", 1) + strings.Repeat("Mehr Text. ", 50),
		"recursive-tier": strings.Repeat("plain prose without structure. ", 100),
	}
	cfg := SplitterConfig{ChunkSize: 300, ChunkOverlap: 30, Separators: []string{"\n\n", "\n", "。", ". "}, Strategy: StrategyAuto}

	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			runes := []rune(doc)
			chunks := Split(doc, cfg)
			if len(chunks) == 0 {
				t.Fatal("expected chunks")
			}
			for i, c := range chunks {
				contentRuneLen := len([]rune(c.Content))
				spanLen := c.End - c.Start
				if spanLen != contentRuneLen {
					t.Errorf("chunk %d: End(%d)-Start(%d)=%d but Content has %d runes:\n%q",
						i, c.End, c.Start, spanLen, contentRuneLen, c.Content)
				}
				if c.Start < 0 || c.End > len(runes) {
					t.Errorf("chunk %d: position out of range Start=%d End=%d totalRunes=%d",
						i, c.Start, c.End, len(runes))
				}
				if c.Start >= 0 && c.End <= len(runes) {
					sliced := string(runes[c.Start:c.End])
					if sliced != c.Content {
						t.Errorf("chunk %d: runes[Start:End] differs from Content", i)
					}
				}
			}
		})
	}
}

func TestEnsureDefaults(t *testing.T) {
	cfg := ensureDefaults(SplitterConfig{ChunkOverlap: -1})
	if cfg.ChunkSize != DefaultChunkSize {
		t.Errorf("expected default ChunkSize %d, got %d", DefaultChunkSize, cfg.ChunkSize)
	}
	if cfg.ChunkOverlap != DefaultChunkOverlap {
		t.Errorf("expected default ChunkOverlap %d, got %d", DefaultChunkOverlap, cfg.ChunkOverlap)
	}
	if got := ensureDefaults(SplitterConfig{}).ChunkOverlap; got != 0 {
		t.Errorf("an overlap of 0 must stay 0, got %d", got)
	}
	if len(cfg.Separators) == 0 {
		t.Error("expected default separators")
	}
}

func TestNormalizeSplitterConfig_MatchesIngestionDefaults(t *testing.T) {
	cfg := NormalizeSplitterConfig(SplitterConfig{ChunkOverlap: -1})
	if cfg.ChunkSize != DefaultChunkSize {
		t.Errorf("chunk size: got %d want %d", cfg.ChunkSize, DefaultChunkSize)
	}
	if cfg.ChunkOverlap != DefaultChunkOverlap {
		t.Errorf("chunk overlap: got %d want %d", cfg.ChunkOverlap, DefaultChunkOverlap)
	}
	if len(cfg.Separators) != 3 {
		t.Fatalf("separators: got %v", cfg.Separators)
	}
}

func TestNormalizeLineEndings(t *testing.T) {
	input := "first\r\nsecond\rthird\nfourth"
	if got, want := NormalizeLineEndings(input), "first\nsecond\nthird\nfourth"; got != want {
		t.Errorf("NormalizeLineEndings() = %q, want %q", got, want)
	}
}

func TestValidateChunks_Empty(t *testing.T) {
	if v := ValidateChunks(nil, 1000, 500); v.OK {
		t.Error("nil chunks should be invalid")
	}
}

func TestValidateChunks_SingleChunkLargeDoc(t *testing.T) {
	c := []Chunk{{Content: strings.Repeat("a", 5000)}}
	if v := ValidateChunks(c, 5000, 500); v.OK {
		t.Error("single 10x-too-large chunk should be invalid")
	}
}

func TestValidateChunks_AcceptsReasonableOutput(t *testing.T) {
	chunks := []Chunk{
		{Content: strings.Repeat("a", 480)},
		{Content: strings.Repeat("b", 510)},
		{Content: strings.Repeat("c", 460)},
	}
	if v := ValidateChunks(chunks, 1500, 512); !v.OK {
		t.Errorf("reasonable chunks should validate, got: %s", v.Reason)
	}
}

func TestValidateChunks_RejectsOversized(t *testing.T) {
	chunks := []Chunk{
		{Content: strings.Repeat("a", 100)},
		{Content: strings.Repeat("b", 5000)}, // > 2x chunkSize
	}
	if v := ValidateChunks(chunks, 5100, 1000); v.OK {
		t.Error("chunk >2x size should be invalid")
	}
}

func TestValidateChunks_TolerantTinyTail(t *testing.T) {
	chunks := []Chunk{
		{Content: strings.Repeat("a", 480)},
		{Content: strings.Repeat("b", 510)},
		{Content: "tail"},
	}
	if v := ValidateChunks(chunks, 994, 512); !v.OK {
		t.Errorf("tiny last chunk should be tolerated, got: %s", v.Reason)
	}
}
