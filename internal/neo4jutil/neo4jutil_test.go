package neo4jutil

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

func TestRecordHelpers(t *testing.T) {
	rec := &neo4j.Record{
		Keys:   []string{"name", "count", "ratio", "missing", "list"},
		Values: []any{"alpha", int64(7), 2.5, nil, []any{"a", int64(1), nil}},
	}
	if got := String(rec, "name"); got != "alpha" {
		t.Fatalf("String = %q", got)
	}
	if got := String(rec, "missing"); got != "" {
		t.Fatalf("String(nil value) = %q", got)
	}
	if got := String(rec, "absent"); got != "" {
		t.Fatalf("String(absent) = %q", got)
	}
	if got := Int(rec, "count"); got != 7 {
		t.Fatalf("Int = %d", got)
	}
	if got := Int(rec, "ratio"); got != 2 {
		t.Fatalf("Int(float) = %d", got)
	}
	if got := Int(nil, "count"); got != 0 {
		t.Fatalf("Int(nil record) = %d", got)
	}
	v, _ := rec.Get("list")
	if got := ToStrings(v); !reflect.DeepEqual(got, []string{"a", "1"}) {
		t.Fatalf("ToStrings = %v", got)
	}
	if got := ToStrings("not a list"); got != nil {
		t.Fatalf("ToStrings(non-list) = %v", got)
	}
	if got := ToFloat(int64(3)); got != 3 {
		t.Fatalf("ToFloat = %v", got)
	}
}

func TestConnectConfigValidate(t *testing.T) {
	cfg := ConnectConfig{Password: "secret"}
	if err := cfg.Validate(); !errors.Is(err, ErrMissingURI) {
		t.Fatalf("expected ErrMissingURI, got %v", err)
	}

	cfg = ConnectConfig{URI: "bolt://db:7687"}
	if err := cfg.Validate(); !errors.Is(err, ErrMissingPassword) {
		t.Fatalf("expected ErrMissingPassword, got %v", err)
	}

	cfg = ConnectConfig{URI: " bolt://db:7687 ", Password: "secret"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Username != DefaultUsername || cfg.URI != "bolt://db:7687" {
		t.Fatalf("unexpected normalized config: %+v", cfg)
	}
}

func TestConnectRejectsMissingCredentialsWithoutDialing(t *testing.T) {
	if _, err := Connect(context.Background(), ConnectConfig{URI: "bolt://127.0.0.1:1"}); !errors.Is(err, ErrMissingPassword) {
		t.Fatalf("expected ErrMissingPassword, got %v", err)
	}
}

func TestQuoteIdentifier(t *testing.T) {
	if got := QuoteIdentifier("ENTITYkb-1"); got != "`ENTITYkb-1`" {
		t.Fatalf("QuoteIdentifier = %s", got)
	}
	if got := QuoteIdentifier("a`) DETACH DELETE n //"); got != "`a``) DETACH DELETE n //`" {
		t.Fatalf("QuoteIdentifier escape = %s", got)
	}
}
