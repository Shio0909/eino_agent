package retrieval

import (
	"testing"

	"github.com/cloudwego/eino/components/retriever"
)

func TestModeFromOptions(t *testing.T) {
	cases := []struct {
		name string
		opts []retriever.Option
		want string
	}{
		{"none", nil, ""},
		{"other options only", []retriever.Option{retriever.WithTopK(3)}, ""},
		{"exact", []retriever.Option{WithMode("exact")}, ModeExact},
		{"normalised", []retriever.Option{WithMode("  Semantic ")}, ModeSemantic},
		{"last one wins", []retriever.Option{WithMode("exact"), retriever.WithTopK(3), WithMode("graph")}, ModeGraph},
	}
	for _, tc := range cases {
		if got := ModeFromOptions(tc.opts...); got != tc.want {
			t.Errorf("%s: ModeFromOptions = %q, want %q", tc.name, got, tc.want)
		}
	}
}
