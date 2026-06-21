package api

import (
	"context"
	"log"
	"strings"

	"github.com/stellarlinkco/agentsdk-go/pkg/model"
)

// ---------------------------------------------------------------------------
// Stream Connection Fallback — Phase 3 (zhanbei1 adapted)
// ---------------------------------------------------------------------------
// When streaming fails due to a transport error (EOF, connection reset, DNS
// failure, etc.), retry the request as a non-streaming call.
//
// Adapted from agentsdk-go but without completeWithRecovery dependency.
// zhanbei1's v1 runLoop does not have the model_recovery framework, so this
// version directly wraps CompleteStream → Complete fallback.
// ---------------------------------------------------------------------------

// isStreamConnectionError returns true when the error is likely a transport-
// level failure during streaming, as opposed to a content or API error.
func isStreamConnectionError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())

	// Common transport-level failure indicators.
	indicators := []string{
		"eof",
		"connection reset",
		"connection refused",
		"broken pipe",
		"dial tcp",
		"i/o timeout",
		"no such host",
		"tls handshake",
		"unexpected end of",
		"stream error",
	}
	for _, ind := range indicators {
		if strings.Contains(msg, ind) {
			return true
		}
	}
	return false
}

// completeWithStreamFallback attempts a streaming model call; if it fails with
// a transport error, retries once as a non-streaming call. This provides a
// recovery layer for transient network glitches without depending on v1's
// completeWithRecovery framework (which zhanbei1 does not have).
func (rt *Runtime) completeWithStreamFallback(ctx context.Context, mdl model.Model, req model.Request) (*model.Response, error) {
	var resp *model.Response
	streamErr := mdl.CompleteStream(ctx, req, func(sr model.StreamResult) error {
		if sr.Final && sr.Response != nil {
			resp = sr.Response
		}
		return nil
	})
	if streamErr == nil {
		return resp, nil
	}

	// If the error is a stream connection error, retry once as non-streaming.
	if isStreamConnectionError(streamErr) {
		log.Printf("[v2-runtime] stream connection error, retrying as non-streaming: %v", streamErr)
		return mdl.Complete(ctx, req)
	}

	return nil, streamErr
}
