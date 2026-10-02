// Package usage counts the LLM tokens a process spends and how much work it
// has in flight. The totals are cumulative across restarts when a Store is
// attached; a controller that meters or budgets the process reads them from the
// heartbeat endpoint.
package usage

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// Totals is cumulative token consumption.
type Totals struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}

func (t Totals) plus(o Totals) Totals {
	return Totals{t.PromptTokens + o.PromptTokens, t.CompletionTokens + o.CompletionTokens, t.TotalTokens + o.TotalTokens}
}

func (t Totals) zero() bool { return t == Totals{} }

// Store persists the totals. Add must apply delta atomically to the stored
// value and return the result, so that a second process, or a crash between
// flushes, can lose at most the unflushed delta and never overwrite another
// writer's tokens.
type Store interface {
	Add(ctx context.Context, delta Totals) (Totals, error)
}

// Meter is safe for concurrent use. The zero value is ready to use and counts
// in memory only.
type Meter struct {
	mu      sync.Mutex
	base    Totals        // last total known to be persisted
	pending Totals        // recorded since, not yet persisted
	wake    chan struct{} // set by Attach: work ended, flush now

	inflight atomic.Int64
}

// Default is the process-wide meter that NewLLMProvider's models report to, so
// that models swapped in at runtime are counted too.
var Default = &Meter{}

// Record adds one call's usage. Providers sometimes report only the parts or
// only the total; the total never ends up below their sum.
func (m *Meter) Record(prompt, completion, total int64) {
	prompt, completion = max(prompt, 0), max(completion, 0)
	total = max(total, prompt+completion)
	if prompt == 0 && completion == 0 && total == 0 {
		return
	}
	m.mu.Lock()
	m.pending = m.pending.plus(Totals{prompt, completion, total})
	m.mu.Unlock()
}

// Totals is everything recorded so far, including what earlier runs persisted.
func (m *Meter) Totals() Totals {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.base.plus(m.pending)
}

// Begin marks one unit of work as in flight until the returned func is called.
// Ending it asks an attached meter to flush, so a crash after a turn has
// finished does not lose that turn's tokens; only work still running at the
// time can be lost.
func (m *Meter) Begin() (end func()) {
	m.inflight.Add(1)
	var once sync.Once
	return func() {
		once.Do(func() {
			m.inflight.Add(-1)
			m.mu.Lock()
			wake := m.wake
			m.mu.Unlock()
			if wake != nil {
				select {
				case wake <- struct{}{}:
				default: // a flush is already requested
				}
			}
		})
	}
}

// InFlight is the number of units of work that have begun and not ended.
func (m *Meter) InFlight() int64 { return m.inflight.Load() }

// Flush persists what has been recorded since the last flush. On failure the
// delta is kept for the next attempt.
func (m *Meter) Flush(ctx context.Context, s Store) error {
	m.mu.Lock()
	delta := m.pending
	m.pending = Totals{}
	m.mu.Unlock()

	// An empty delta still reads the stored total, which is how Attach learns
	// what earlier runs spent.
	stored, err := s.Add(ctx, delta)

	m.mu.Lock()
	defer m.mu.Unlock()
	if err != nil {
		m.pending = m.pending.plus(delta)
		return err
	}
	m.base = stored
	return nil
}

// Attach loads the persisted totals and then flushes every interval until ctx
// ends, with a last flush on the way out. It returns once the first load has
// finished so that the first heartbeat already carries the history.
func (m *Meter) Attach(ctx context.Context, s Store, interval time.Duration) (stopped <-chan struct{}) {
	if err := m.Flush(ctx, s); err != nil {
		log.Printf("[Usage] load persisted totals failed, counting from zero: %v", err)
	}
	wake := make(chan struct{}, 1)
	m.mu.Lock()
	m.wake = wake
	m.mu.Unlock()
	done := make(chan struct{})
	go func() {
		defer close(done)
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-wake:
				// Work that spent no tokens, which is most requests that are not
				// a chat turn, costs no database write.
				if m.unflushed() == 0 {
					continue
				}
				if err := m.Flush(ctx, s); err != nil {
					log.Printf("[Usage] persist totals failed, will retry: %v", err)
				}
			case <-t.C:
				if err := m.Flush(ctx, s); err != nil {
					log.Printf("[Usage] persist totals failed, will retry: %v", err)
				}
			case <-ctx.Done():
				// ctx is done, so the final flush needs one of its own.
				final, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := m.Flush(final, s); err != nil {
					log.Printf("[Usage] final persist failed, %d tokens unrecorded: %v", m.unflushed(), err)
				}
				return
			}
		}
	}()
	return done
}

func (m *Meter) unflushed() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.pending.TotalTokens
}
