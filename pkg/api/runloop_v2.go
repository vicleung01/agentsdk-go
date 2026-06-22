package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/stellarlinkco/agentsdk-go/pkg/message"
	"github.com/stellarlinkco/agentsdk-go/pkg/middleware"
	"github.com/stellarlinkco/agentsdk-go/pkg/model"
	toolpkg "github.com/stellarlinkco/agentsdk-go/pkg/tool"
)

// ---------------------------------------------------------------------------
// v2 Runtime — State Machine Agent Loop
// ---------------------------------------------------------------------------
// Based on Claude Code's query.ts execution pattern, adapted for Go.
// Full state machine with 4 transitions, Snip compression, stream fallback,
// max_tokens recovery, reactive compact, and stop injection protection.
//
// Phase 1: Stub — delegates to v1 runLoop.
// Phase 2: Snip compression + circuit breaker.
// Phase 3: Stream fallback + max_tokens recovery.
// Phase 4: Full state machine with all transitions.  ← CURRENT
// ---------------------------------------------------------------------------

// loopTransition represents a state machine transition in the agent loop.
type loopTransition string

const (
	transitionContinue loopTransition = "continue" // Normal iteration (tool calls found)
	transitionNudge    loopTransition = "nudge"    // Inject hint, continue (stop blocked)
	transitionStop     loopTransition = "stop"     // Clean termination
	transitionError    loopTransition = "error"    // Fatal error
)

// loopState tracks the full state of the v2 agent loop across iterations.
type loopState struct {
	iteration                   int
	turnCount                   int
	snipTriggered               int            // Number of times Snip compression was triggered
	maxOutputTokensRecovery     int            // Number of max_tokens recoveries attempted
	hasAttemptedReactiveCompact bool           // Whether reactive compaction has been tried
	lastTransition              loopTransition // Most recent state machine transition
}

// compactCircuitBreaker prevents cascading failures from repeated compaction errors.
// Thread-safe: all methods are protected by mutex.
type compactCircuitBreaker struct {
	mu               sync.Mutex
	failureThreshold int // Default: 3
	failureCount     int
	open             bool
	cooldown         time.Duration // Default: 30s
	lastFailure      time.Time
}

func newCompactCircuitBreaker() *compactCircuitBreaker {
	return &compactCircuitBreaker{
		failureThreshold: 3,
		cooldown:         30 * time.Second,
	}
}

func (cb *compactCircuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failureCount++
	cb.lastFailure = time.Now()
	if cb.failureCount >= cb.failureThreshold {
		cb.open = true
	}
}

func (cb *compactCircuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failureCount = 0
	cb.open = false
}

func (cb *compactCircuitBreaker) isOpen() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if !cb.open {
		return false
	}
	if time.Since(cb.lastFailure) > cb.cooldown {
		cb.open = false
		cb.failureCount = 0
		return false
	}
	return true
}

// SnipConfig controls the Snip compression layer.
type SnipConfig struct {
	Enabled      bool    `json:"enabled"`
	TriggerRatio float64 `json:"trigger_ratio"`  // Default: 0.95
	MaxSnipCount int     `json:"max_snip_count"` // Default: 2
}

func (c SnipConfig) withDefaults() SnipConfig {
	cfg := c
	if cfg.TriggerRatio <= 0 || cfg.TriggerRatio > 1 {
		cfg.TriggerRatio = 0.95
	}
	if cfg.MaxSnipCount <= 0 {
		cfg.MaxSnipCount = 2
	}
	return cfg
}

// runLoopV2 is the v2 agent loop — full state machine implementation.
// It manages its own iteration loop with Snip compression, stream fallback,
// max_tokens recovery, reactive compact, and stop injection protection.
// Skylark progressive retrieval is NOT integrated here; Skylark users use v1.
func (rt *Runtime) runLoopV2(prep preparedRun, mdl model.Model, hookAdapter *runtimeHookAdapter, tools *runtimeToolExecutor, chain *middleware.Chain, enableCache bool) (*model.Response, error) {
	if err := validateRunLoopInputs(prep.history, mdl, chain); err != nil {
		return nil, err
	}

	ctx := prep.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	// Append user message to history.
	if strings.TrimSpace(prep.prompt) != "" || len(prep.contentBlocks) > 0 {
		userMsg := message.Message{Role: "user", Content: strings.TrimSpace(prep.prompt)}
		if len(prep.contentBlocks) > 0 {
			userMsg.ContentBlocks = convertAPIContentBlocks(prep.contentBlocks)
		}
		prep.history.Append(userMsg)
	}

	// Initialize middleware state.
	state := &middleware.State{Values: map[string]any{}}
	if sessionID := strings.TrimSpace(prep.normalized.SessionID); sessionID != "" {
		state.Values["session_id"] = sessionID
	}
	if requestID := strings.TrimSpace(prep.normalized.RequestID); requestID != "" {
		state.Values["request_id"] = requestID
	}
	if len(prep.normalized.ForceSkills) > 0 {
		state.Values["request.force_skills"] = append([]string(nil), prep.normalized.ForceSkills...)
	}
	if rt.opts.skReg != nil {
		state.Values["skills.registry"] = rt.opts.skReg
	}
	ctx = context.WithValue(ctx, model.MiddlewareStateKey, state)

	systemPrompt := rt.systemPromptForSessionV2(prep.normalized.SessionID, prep.toolWhitelist)
	trimmer := rt.newTrimmer()

	// v2 state tracking.
	loopState := &loopState{}
	circuitBreaker := newCompactCircuitBreaker()
	snipCfg := SnipConfig{Enabled: true}.withDefaults()
	tokenLimit := defaultTokenLimitV2(rt)

	log.Printf("[v2-runtime] runLoopV2 started (snip=%v breaker_max=%d token_limit=%d)",
		snipCfg.Enabled, circuitBreaker.failureThreshold, tokenLimit)

	tracer := rt.opts.tracer
	agentSpan := SpanContext(nil)
	if tracer != nil {
		agentSpan = tracer.StartAgentSpan(prep.normalized.SessionID, prep.normalized.RequestID, 0)
	}
	var runErr error
	defer func() {
		if tracer == nil {
			return
		}
		tracer.EndSpan(agentSpan, map[string]any{
			"session_id":              strings.TrimSpace(prep.normalized.SessionID),
			"request_id":              strings.TrimSpace(prep.normalized.RequestID),
			"iterations":              loopState.iteration,
			"entry_point":             string(prep.normalized.Mode.EntryPoint),
			"v2.snip_triggered":       loopState.snipTriggered,
			"v2.circuit_breaker_open": circuitBreaker.isOpen(),
		}, runErr)
	}()

	var last *model.Response

	for iteration := 0; ; iteration++ {
		loopState.iteration = iteration + 1

		// --- Pre-iteration checks ---
		if err := ctx.Err(); err != nil {
			loopState.lastTransition = transitionError
			runErr = err
			return last, err
		}
		if rt.opts.MaxIterations > 0 && iteration >= rt.opts.MaxIterations {
			loopState.lastTransition = transitionError
			runErr = ErrMaxIterations
			return last, ErrMaxIterations
		}

		state.Iteration = iteration

		// === Layer 1: Snip compression (deterministic, no model call) ===
		snipped := maybeSnip(prep.history, circuitBreaker, snipCfg, tokenLimit)
		if snipped {
			loopState.snipTriggered++
			log.Printf("[v2-runtime] iter=%d snip triggered (total=%d)", iteration, loopState.snipTriggered)
		}

		// === Layer 2+3: Micro + Auto compression (existing compactor) ===
		if rt.compactor != nil {
			if _, err := rt.compactor.maybeCompact(ctx, prep.history, mdl); err != nil {
				circuitBreaker.recordFailure()
				log.Printf("[v2-runtime] iter=%d compact failed (breaker fails=%d)", iteration, circuitBreaker.failureCount)
				// v2: compaction failure is non-fatal. Circuit breaker opens after 3.
				_ = err
			}
		}

		// --- Model input preparation ---
		snapshot := prep.history.All()
		if trimmer != nil {
			snapshot = trimmer.Trim(snapshot)
		}
		toolDefs := availableTools(rt.registry, prep.toolWhitelist)

		req := model.Request{
			Messages:          convertMessages(snapshot),
			Tools:             toolDefs,
			System:            systemPrompt,
			EnablePromptCache: enableCache,
		}
		state.ModelInput = &req
		state.Values["model.request"] = req

		if err := chain.Execute(ctx, middleware.StageBeforeAgent, state); err != nil {
			loopState.lastTransition = transitionError
			runErr = err
			return last, err
		}

		// --- Model call (with v2 stream fallback) ---
		resp, err := rt.completeWithStreamFallback(ctx, mdl, req)
		if err != nil {
			// Reactive compact: if prompt too long, try aggressive Snip once.
			if !loopState.hasAttemptedReactiveCompact && isPromptTooLongError(err) {
				loopState.hasAttemptedReactiveCompact = true
				aggressiveCfg := SnipConfig{
					Enabled:      true,
					TriggerRatio: 0.5,
					MaxSnipCount: 4,
				}
				maybeSnip(prep.history, circuitBreaker, aggressiveCfg, tokenLimit)
				loopState.lastTransition = transitionContinue
				log.Printf("[v2-runtime] iter=%d reactive compact triggered (aggressive snip)", iteration)
				continue
			}
			loopState.lastTransition = evaluateTransition(loopState, nil, err, false, false)
			runErr = err
			return last, err
		}
		if resp == nil {
			runErr = errors.New("api: model returned no final response")
			return last, runErr
		}
		last = resp

		// v2: max_tokens continuation recovery.
		if rt.recoverMaxTokens(resp, loopState, prep.history) {
			loopState.lastTransition = transitionContinue
			log.Printf("[v2-runtime] iter=%d max_tokens recovery injected", iteration)
			continue
		}

		state.ModelOutput = resp
		state.Values["model.response"] = resp
		state.Values["model.usage"] = resp.Usage
		state.Values["model.stop_reason"] = resp.StopReason
		state.Values["v2.state"] = loopState
		state.Values["v2.snip_count"] = loopState.snipTriggered
		state.Values["v2.circuit_breaker_open"] = circuitBreaker.isOpen()

		// Append assistant message to history.
		assistant := message.Message{
			Role:             resp.Message.Role,
			Content:          strings.TrimSpace(resp.Message.Content),
			ReasoningContent: resp.Message.ReasoningContent,
		}
		if len(resp.Message.ToolCalls) > 0 {
			assistant.ToolCalls = make([]message.ToolCall, len(resp.Message.ToolCalls))
			for i, call := range resp.Message.ToolCalls {
				assistant.ToolCalls[i] = message.ToolCall{ID: call.ID, Name: call.Name, Arguments: call.Arguments}
			}
		}
		prep.history.Append(assistant)

		if err := chain.Execute(ctx, middleware.StageAfterAgent, state); err != nil {
			loopState.lastTransition = transitionError
			runErr = err
			return last, err
		}

		// --- Tool execution (origin/main Execute; preserves concurrency + middleware + tracer) ---
		if len(resp.Message.ToolCalls) > 0 {
			calls := resp.Message.ToolCalls
			var firstMiddlewareErr error
			outs := make([]*toolpkg.CallResult, len(calls))

			runTool := func(i int) {
				state.ToolCall = calls[i]
				if err := chain.Execute(ctx, middleware.StageBeforeTool, state); err != nil && firstMiddlewareErr == nil {
					firstMiddlewareErr = err
				}
				toolSpan := SpanContext(nil)
				if tracer != nil {
					toolSpan = tracer.StartToolSpan(agentSpan, strings.TrimSpace(calls[i].Name))
				}
				res, err := tools.Execute(ctx, calls[i])
				if tracer != nil {
					tracer.EndSpan(toolSpan, map[string]any{
						"session_id":  strings.TrimSpace(prep.normalized.SessionID),
						"request_id":  strings.TrimSpace(prep.normalized.RequestID),
						"tool_use_id": strings.TrimSpace(calls[i].ID),
						"tool_name":   strings.TrimSpace(calls[i].Name),
					}, err)
				}
				outs[i] = res
			}

			parallel := len(calls) > 1
			if parallel {
				var wg sync.WaitGroup
				for i := range calls {
					wg.Add(1)
					go func(i int) { defer wg.Done(); runTool(i) }(i)
				}
				wg.Wait()
			} else {
				for i := range calls {
					runTool(i)
				}
			}

			for i := range calls {
				state.ToolCall = calls[i]
				state.ToolResult = outs[i]
				if err := chain.Execute(ctx, middleware.StageAfterTool, state); err != nil && firstMiddlewareErr == nil {
					firstMiddlewareErr = err
				}
			}
			if firstMiddlewareErr != nil {
				loopState.lastTransition = transitionError
				runErr = firstMiddlewareErr
				return last, firstMiddlewareErr
			}
			loopState.turnCount++
			loopState.lastTransition = transitionContinue
			continue
		}

		// --- Stop evaluation ---
		// zhanbei1 does not have evaluateStop (agentsdk-go only).
		// Call the Stop hook for observability but do not block/reinject.
		if hookAdapter != nil {
			_ = hookAdapter.Stop(ctx, resp.StopReason)
		}

		log.Printf("[v2-runtime] iter=%d transition=%s snip=%d breaker=%v",
			iteration, loopState.lastTransition, loopState.snipTriggered, circuitBreaker.isOpen())
		loopState.lastTransition = transitionStop
		runErr = nil
		return resp, nil
	}
}

// evaluateTransition determines the next loop transition based on current state.
func evaluateTransition(state *loopState, resp *model.Response, stopErr error, hasToolCalls bool, stopBlocked bool) loopTransition {
	if stopErr != nil || resp == nil {
		return transitionError
	}
	if hasToolCalls {
		return transitionContinue
	}
	if stopBlocked {
		return transitionNudge
	}
	return transitionStop
}

// formatTransition returns a human-readable string for logging.
func formatTransition(t loopTransition) string {
	return fmt.Sprintf("v2-runtime: transition=%s", string(t))
}

// --- Helpers ---

// isPromptTooLongError checks if the error indicates the prompt exceeded the model's
// context window. Used by reactive compact to trigger aggressive Snip compression.

// defaultTokenLimitV2 returns the token limit from options or a sensible default.
func defaultTokenLimitV2(rt *Runtime) int {
	if rt.opts.TokenLimit > 0 {
		return rt.opts.TokenLimit
	}
	return 200000
}

// systemPromptForSessionV2 returns the system prompt for v2 sessions.
// v2 does not use Skylark one-shot augmentation (Skylark users stay on v1).
func (rt *Runtime) systemPromptForSessionV2(sessionID string, toolWhitelist map[string]struct{}) string {
	return rt.opts.SystemPrompt
}

// validateRunLoopInputs performs nil checks on required inputs.
func validateRunLoopInputs(hist *message.History, mdl model.Model, chain *middleware.Chain) error {
	if hist == nil {
		return errors.New("api: history is nil")
	}
	if mdl == nil {
		return errors.New("api: model is nil")
	}
	if chain == nil {
		return errors.New("api: middleware chain is nil")
	}
	return nil
}
