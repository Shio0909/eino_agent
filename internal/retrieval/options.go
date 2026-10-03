// Package retrieval defines the call options this project's retrievers
// understand.
//
// They are ordinary Eino retriever.Option values. Every retriever wrapper
// (scoping, tracing, unified routing) already forwards the options it gets, so
// a choice made by the caller reaches the retriever that acts on it, which a
// side interface asserted on the outermost retriever cannot do once a wrapper
// sits in between.
package retrieval

import (
	"strings"

	"github.com/cloudwego/eino/components/retriever"
)

// Retrieval modes. An empty mode and ModeAuto mean the retriever's default.
const (
	ModeAuto     = "auto"
	ModeSemantic = "semantic"
	ModeExact    = "exact"
	ModeGraph    = "graph"
)

type options struct{ mode string }

// WithMode selects how a retriever searches: ModeSemantic (vectors only),
// ModeExact (keywords only), ModeGraph (the knowledge graph) or ModeAuto.
func WithMode(mode string) retriever.Option {
	return retriever.WrapImplSpecificOptFn(func(o *options) {
		o.mode = strings.ToLower(strings.TrimSpace(mode))
	})
}

// ModeFromOptions returns the mode the options ask for, or "" when none is set.
func ModeFromOptions(opts ...retriever.Option) string {
	return retriever.GetImplSpecificOptions(&options{}, opts...).mode
}
