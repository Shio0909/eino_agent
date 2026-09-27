package container

import (
	"context"
	"math"
	"testing"
)

func TestHashEmbedderIsDeterministicAndNormalized(t *testing.T) {
	e := newHashEmbedding(32)
	first, err := e.EmbedStrings(context.Background(), []string{"退款审核"})
	if err != nil { t.Fatal(err) }
	second, err := e.EmbedStrings(context.Background(), []string{"退款审核"})
	if err != nil { t.Fatal(err) }
	if len(first) != 1 || len(first[0]) != 32 { t.Fatalf("unexpected shape: %#v", first) }
	for i, value := range first[0] {
		if value != second[0][i] { t.Fatalf("hash embedding is not deterministic at %d", i) }
	}
	var norm float64
	for _, value := range first[0] { norm += value * value }
	if math.Abs(math.Sqrt(norm)-1) > 1e-9 { t.Fatalf("norm = %f, want 1", math.Sqrt(norm)) }
}

func TestHashEmbedderUsesDefaultDimension(t *testing.T) {
	e := newHashEmbedding(0)
	result, err := e.EmbedStrings(context.Background(), []string{"x"})
	if err != nil { t.Fatal(err) }
	if len(result) != 1 || len(result[0]) != 128 { t.Fatalf("unexpected default dimension: %d", len(result[0])) }
}
