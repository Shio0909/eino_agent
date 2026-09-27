package importqueue

import (
	"errors"
	"fmt"
	"testing"
)

func TestPermanentErrorClassification(t *testing.T) {
	base := errors.New("boom")
	if IsPermanent(base) {
		t.Fatal("plain error must be retryable")
	}
	wrapped := fmt.Errorf("context: %w", Permanent(base))
	if !IsPermanent(wrapped) {
		t.Fatal("wrapped permanent error must be detected")
	}
	if !errors.Is(wrapped, base) {
		t.Fatal("permanent error must unwrap to cause")
	}
	if Permanent(nil) != nil {
		t.Fatal("Permanent(nil) must be nil")
	}
}
