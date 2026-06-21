package api

import (
	"testing"

	"github.com/stellarlinkco/agentsdk-go/pkg/message"
)

// helperTokenString returns a string that occupies approximately n tokens under
// NaiveCounter (which divides Content length by 4). Each call with the same n
// returns the same string.
func helperTokenString(n int) string {
	if n <= 0 {
		return ""
	}
	// n tokens ≈ n*4 bytes of content.
	buf := make([]byte, n*4)
	for i := range buf {
		buf[i] = 'x'
	}
	return string(buf)
}

// addNMessages appends count messages to the history, each contributing ~tokensPerMsg
// tokens under NaiveCounter.
func addNMessages(hist *message.History, count, tokensPerMsg int) {
	content := helperTokenString(tokensPerMsg)
	for i := 0; i < count; i++ {
		hist.Append(message.Message{Role: "user", Content: content})
	}
}

// ---------------------------------------------------------------------------
// Test: Snip disabled → no-op
// ---------------------------------------------------------------------------

func TestSnip_Disabled(t *testing.T) {
	hist := message.NewHistory()
	addNMessages(hist, 10, 100) // ~1000 tokens
	cb := newCompactCircuitBreaker()
	cfg := SnipConfig{Enabled: false}

	snipped := maybeSnip(hist, cb, cfg, 500)
	if snipped {
		t.Fatal("expected no snip when disabled")
	}
	if hist.Len() != 10 {
		t.Fatalf("expected 10 messages, got %d", hist.Len())
	}
}

// ---------------------------------------------------------------------------
// Test: nil history → no panic, returns false
// ---------------------------------------------------------------------------

func TestSnip_NilHistory(t *testing.T) {
	cb := newCompactCircuitBreaker()
	cfg := SnipConfig{Enabled: true}

	snipped := maybeSnip(nil, cb, cfg, 1000)
	if snipped {
		t.Fatal("expected no snip with nil history")
	}
}

// ---------------------------------------------------------------------------
// Test: token ratio below trigger → no snip
// ---------------------------------------------------------------------------

func TestSnip_BelowTriggerRatio(t *testing.T) {
	hist := message.NewHistory()
	// 5 messages × 10 tokens each = ~50 tokens, limit=1000 → ratio=0.05 < 0.95
	addNMessages(hist, 5, 10)

	cb := newCompactCircuitBreaker()
	cfg := SnipConfig{Enabled: true}.withDefaults() // TriggerRatio=0.95

	snipped := maybeSnip(hist, cb, cfg, 1000)
	if snipped {
		t.Fatal("expected no snip when ratio < trigger")
	}
	if hist.Len() != 5 {
		t.Fatalf("expected 5 messages, got %d", hist.Len())
	}
}

// ---------------------------------------------------------------------------
// Test: token ratio ≥ trigger → snip deletes oldest messages
// ---------------------------------------------------------------------------

func TestSnip_TriggersAndDeletesOldest(t *testing.T) {
	hist := message.NewHistory()
	// 10 messages × 100 tokens each = ~1000 tokens, limit=1000 → ratio=1.0 ≥ 0.95
	addNMessages(hist, 10, 100)

	cb := newCompactCircuitBreaker()
	cfg := SnipConfig{Enabled: true}.withDefaults() // TriggerRatio=0.95, MaxSnipCount=2

	before := hist.Len()
	snipped := maybeSnip(hist, cb, cfg, 1000)
	if !snipped {
		t.Fatal("expected snip to trigger")
	}
	// Should have removed MaxSnipCount=2 oldest messages.
	after := hist.Len()
	if after != before-2 {
		t.Fatalf("expected %d messages after snip, got %d", before-2, after)
	}
}

// ---------------------------------------------------------------------------
// Test: preserve count respected — never snip below preserve threshold
// ---------------------------------------------------------------------------

func TestSnip_RespectsPreserveCount(t *testing.T) {
	hist := message.NewHistory()
	// 6 messages × 100 tokens = ~600 tokens, limit=600 → ratio=1.0
	// preserve=5, msgCount=6, can cut at most 1
	addNMessages(hist, 6, 100)

	cb := newCompactCircuitBreaker()
	cfg := SnipConfig{Enabled: true, TriggerRatio: 0.95, MaxSnipCount: 10}.withDefaults()

	snipped := maybeSnip(hist, cb, cfg, 600)
	if !snipped {
		t.Fatal("expected snip to trigger")
	}
	// preserve=5, so at most 1 message can be cut.
	if hist.Len() != 5 {
		t.Fatalf("expected 5 messages (preserve count), got %d", hist.Len())
	}
}

// ---------------------------------------------------------------------------
// Test: too few messages → no snip (msgCount ≤ preserve)
// ---------------------------------------------------------------------------

func TestSnip_TooFewMessages(t *testing.T) {
	hist := message.NewHistory()
	// 4 messages × 100 tokens = ~400 tokens, limit=400 → ratio=1.0
	// But 4 ≤ preserve=5, so no snip.
	addNMessages(hist, 4, 100)

	cb := newCompactCircuitBreaker()
	cfg := SnipConfig{Enabled: true}.withDefaults()

	snipped := maybeSnip(hist, cb, cfg, 400)
	if snipped {
		t.Fatal("expected no snip when msgCount <= preserve")
	}
}

// ---------------------------------------------------------------------------
// Test: tool transaction span protection
// ---------------------------------------------------------------------------

func TestSnip_ProtectsToolTransactionSpan(t *testing.T) {
	hist := message.NewHistory()

	// msg0: user (oldest, would normally be snipped)
	hist.Append(message.Message{Role: "user", Content: helperTokenString(200)})
	// msg1: assistant with tool call — start of tool transaction span
	hist.Append(message.Message{
		Role:    "assistant",
		Content: helperTokenString(200),
		ToolCalls: []message.ToolCall{{
			ID:   "tc1",
			Name: "bash",
			Arguments: map[string]any{
				"cmd": "echo hello",
			},
		}},
	})
	// msg2: tool result — part of span [1, 3)
	hist.Append(message.Message{
		Role:    "tool",
		Content: helperTokenString(200),
		ToolCalls: []message.ToolCall{{
			ID:     "tc1",
			Name:   "bash",
			Result: "hello",
		}},
	})
	// msg3-msg9: fill to exceed preserve count
	for i := 3; i < 10; i++ {
		hist.Append(message.Message{Role: "user", Content: helperTokenString(100)})
	}
	// Total ~10 messages, lots of tokens.
	// MaxSnipCount=2 would cut msg0,msg1 — but msg1 starts a tool span,
	// so cut is adjusted to span.start=1, cutting only msg0.

	cb := newCompactCircuitBreaker()
	cfg := SnipConfig{Enabled: true, TriggerRatio: 0.5, MaxSnipCount: 2}.withDefaults()

	before := hist.Len()
	snipped := maybeSnip(hist, cb, cfg, 200) // Low limit to force trigger
	if !snipped {
		t.Fatal("expected snip to trigger")
	}
	after := hist.Len()
	if after != before-1 {
		t.Fatalf("expected %d messages (cut 1, protected tool span), got %d", before-1, after)
	}

	// Verify the tool call message survived.
	msgs := hist.All()
	hasToolCall := false
	for _, m := range msgs {
		if len(m.ToolCalls) > 0 && m.ToolCalls[0].ID == "tc1" {
			hasToolCall = true
		}
	}
	if !hasToolCall {
		t.Fatal("tool call message was incorrectly snipped")
	}
}

// ---------------------------------------------------------------------------
// Test: circuit breaker open → skip snip
// ---------------------------------------------------------------------------

func TestSnip_CircuitBreakerOpen(t *testing.T) {
	hist := message.NewHistory()
	addNMessages(hist, 10, 100) // ~1000 tokens

	cb := newCompactCircuitBreaker()
	// Trip the breaker by recording 3 failures.
	cb.recordFailure()
	cb.recordFailure()
	cb.recordFailure()
	if !cb.isOpen() {
		t.Fatal("expected circuit breaker to be open")
	}

	cfg := SnipConfig{Enabled: true}.withDefaults()
	snipped := maybeSnip(hist, cb, cfg, 500) // ratio >> 0.95
	if snipped {
		t.Fatal("expected no snip when circuit breaker is open")
	}
	if hist.Len() != 10 {
		t.Fatalf("expected 10 messages (unchanged), got %d", hist.Len())
	}
}

// ---------------------------------------------------------------------------
// Test: circuit breaker resets on successful snip
// ---------------------------------------------------------------------------

func TestSnip_CircuitBreakerResetsOnSuccess(t *testing.T) {
	hist := message.NewHistory()
	addNMessages(hist, 10, 100)

	cb := newCompactCircuitBreaker()
	// 2 failures (below threshold).
	cb.recordFailure()
	cb.recordFailure()

	cfg := SnipConfig{Enabled: true}.withDefaults()
	snipped := maybeSnip(hist, cb, cfg, 500)
	if !snipped {
		t.Fatal("expected snip to trigger")
	}
	// After success, breaker should be reset.
	if cb.isOpen() {
		t.Fatal("expected circuit breaker to be closed after success")
	}
}

// ---------------------------------------------------------------------------
// Test: SnipConfig withDefaults
// ---------------------------------------------------------------------------

func TestSnipConfig_WithDefaults(t *testing.T) {
	cfg := SnipConfig{Enabled: true}
	applied := cfg.withDefaults()

	if applied.TriggerRatio != 0.95 {
		t.Fatalf("expected TriggerRatio=0.95, got %f", applied.TriggerRatio)
	}
	if applied.MaxSnipCount != 2 {
		t.Fatalf("expected MaxSnipCount=2, got %d", applied.MaxSnipCount)
	}

	// Explicit values should be preserved.
	cfg2 := SnipConfig{Enabled: true, TriggerRatio: 0.8, MaxSnipCount: 5}
	applied2 := cfg2.withDefaults()
	if applied2.TriggerRatio != 0.8 {
		t.Fatalf("expected TriggerRatio=0.8, got %f", applied2.TriggerRatio)
	}
	if applied2.MaxSnipCount != 5 {
		t.Fatalf("expected MaxSnipCount=5, got %d", applied2.MaxSnipCount)
	}
}

// ---------------------------------------------------------------------------
// Test: zero tokenLimit → no snip
// ---------------------------------------------------------------------------

func TestSnip_ZeroTokenLimit(t *testing.T) {
	hist := message.NewHistory()
	addNMessages(hist, 10, 100)

	cb := newCompactCircuitBreaker()
	cfg := SnipConfig{Enabled: true}.withDefaults()

	snipped := maybeSnip(hist, cb, cfg, 0)
	if snipped {
		t.Fatal("expected no snip with zero token limit")
	}
}

// ---------------------------------------------------------------------------
// Test: multiple snips in succession progressively reduce history
// ---------------------------------------------------------------------------

func TestSnip_MultipleSnips(t *testing.T) {
	hist := message.NewHistory()
	// 20 messages × 100 tokens = ~2000 tokens, limit=500 → ratio=4.0
	addNMessages(hist, 20, 100)

	cb := newCompactCircuitBreaker()
	cfg := SnipConfig{Enabled: true}.withDefaults() // MaxSnipCount=2

	// First snip: 20 → 18
	snipped := maybeSnip(hist, cb, cfg, 500)
	if !snipped || hist.Len() != 18 {
		t.Fatalf("first snip: expected 18, got %d, snipped=%v", hist.Len(), snipped)
	}

	// Second snip: 18 → 16
	snipped = maybeSnip(hist, cb, cfg, 500)
	if !snipped || hist.Len() != 16 {
		t.Fatalf("second snip: expected 16, got %d, snipped=%v", hist.Len(), snipped)
	}

	// Keep snipping until we hit preserve count.
	for i := 0; i < 10; i++ {
		maybeSnip(hist, cb, cfg, 500)
	}
	// preserve=5, so we should never go below 5.
	if hist.Len() < 5 {
		t.Fatalf("snipped below preserve count: %d messages remain", hist.Len())
	}
}

// ---------------------------------------------------------------------------
// Test: circuit breaker concurrency safety
// ---------------------------------------------------------------------------

func TestCircuitBreaker_ConcurrentAccess(t *testing.T) {
	cb := newCompactCircuitBreaker()
	done := make(chan struct{})

	// Writer goroutine: rapidly record failures and successes.
	go func() {
		defer func() { done <- struct{}{} }()
		for i := 0; i < 100; i++ {
			cb.recordFailure()
			cb.recordSuccess()
		}
	}()

	// Reader goroutine: check isOpen concurrently.
	go func() {
		defer func() { done <- struct{}{} }()
		for i := 0; i < 100; i++ {
			_ = cb.isOpen()
		}
	}()

	<-done
	<-done
	// No panic = success.
}
