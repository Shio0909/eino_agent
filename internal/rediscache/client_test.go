package rediscache

import (
	"context"
	"testing"
	"time"

	"eino_agent/internal/config"
)

// A disabled client must not dial: the all-in-one workspace image has no Redis
// and every process start used to wait out a connection timeout.
func TestNewClientDisabledDoesNotDial(t *testing.T) {
	// 192.0.2.1 is reserved for documentation and does not answer, so a dial
	// would take the full DialTimeout.
	cfg := config.RedisConfig{Addr: "192.0.2.1:6379", Disabled: true}

	start := time.Now()
	c, err := NewClient(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewClient error = %v, want nil for a disabled client", err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("NewClient took %v, a disabled client must not dial", elapsed)
	}
	if c.Enabled() {
		t.Fatal("a disabled client reports Enabled")
	}
	st := c.Status(context.Background())
	if st.Configured || st.Available || st.Mode != "disabled" {
		t.Fatalf("status = %+v, want unconfigured, unavailable, mode disabled", st)
	}
	if err := c.Ping(context.Background()); err != ErrDisabled {
		t.Fatalf("Ping error = %v, want ErrDisabled", err)
	}
}
