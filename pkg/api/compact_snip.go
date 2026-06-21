package api

import (
	"github.com/stellarlinkco/agentsdk-go/pkg/message"
)

// ---------------------------------------------------------------------------
// Snip Compression — Layer 1 (deterministic, model-free)
// ---------------------------------------------------------------------------
// Snip deletes the oldest messages when the conversation approaches the token
// limit. Unlike Auto compression (Layer 3), Snip does NOT call the model — it
// simply removes messages. This makes it fast and reliable, at the cost of
// losing the oldest context.
//
// Safety: Protected by circuit breaker (consecutiveCompactFailures) and
// toolTransactionSpans (never cuts inside a tool call sequence).
// ---------------------------------------------------------------------------

const defaultSnipPreserveCount = 5

// maybeSnip performs Snip compression: deterministically deletes the oldest
// messages without calling the model. Protected by the circuit breaker.
// Returns true if messages were snipped, false otherwise.
func maybeSnip(hist *message.History, cb *compactCircuitBreaker, cfg SnipConfig, tokenLimit int) bool {
	if !cfg.Enabled {
		return false
	}
	if hist == nil {
		return false
	}

	// Circuit breaker: skip Snip if too many recent compaction failures.
	if cb.isOpen() {
		return false
	}

	// Check trigger condition: token usage exceeds trigger ratio.
	tokenCount := hist.TokenCount()
	if tokenCount <= 0 || tokenLimit <= 0 {
		return false
	}
	ratio := float64(tokenCount) / float64(tokenLimit)
	if ratio < cfg.TriggerRatio {
		return false
	}

	snapshot := hist.All()
	preserve := defaultSnipPreserveCount
	msgCount := len(snapshot)
	if msgCount <= preserve {
		return false
	}

	// Calculate how many oldest messages to cut.
	// Cut at most MaxSnipCount messages from the front.
	cut := cfg.MaxSnipCount
	if cut >= msgCount-preserve {
		cut = msgCount - preserve
	}
	if cut <= 0 {
		return false
	}

	// Protect tool transaction spans: never cut inside a tool call sequence.
	for _, span := range toolTransactionSpans(snapshot) {
		if span.start < cut && cut < span.end {
			// Adjust cut to before the span starts.
			cut = span.start
			break
		}
	}
	if cut <= 0 {
		return false
	}

	// Perform the snip: replace history with messages from cut index onwards.
	remaining := make([]message.Message, len(snapshot[cut:]))
	copy(remaining, snapshot[cut:])
	hist.Replace(remaining)

	cb.recordSuccess()
	return true
}
