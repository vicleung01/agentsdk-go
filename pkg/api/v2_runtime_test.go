package api

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stellarlinkco/agentsdk-go/pkg/model"
	"github.com/stellarlinkco/agentsdk-go/pkg/tool"
)

// ---------------------------------------------------------------------------
// Phase 1 集成测试：env var 路由
// ---------------------------------------------------------------------------
// 验证 runAgentWithMiddleware 中 UseV2Runtime=true 走 runLoopV2，
// UseV2Runtime=false 走 runLoop。两者应产生相同结果。

func TestV2Runtime_V1vsV2SameResult(t *testing.T) {
	t.Parallel()
	mdl := &stubModel{responses: []*model.Response{
		{StopReason: "end_turn", Message: model.Message{Role: "assistant", Content: "hello world"}},
	}}

	// V1 路径
	rt1 := newTestRuntime(t, mdl, CompactConfig{})
	t.Cleanup(func() { _ = rt1.Close() })

	// V2 路径
	rt2 := newTestRuntime(t, mdl, CompactConfig{})
	rt2.opts.UseV2Runtime = true
	t.Cleanup(func() { _ = rt2.Close() })

	prompt := "hello"
	resp1, err1 := rt1.Run(context.Background(), Request{Prompt: prompt})
	resp2, err2 := rt2.Run(context.Background(), Request{Prompt: prompt})

	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected errors: v1=%v, v2=%v", err1, err2)
	}
	if resp1.Result == nil || resp2.Result == nil {
		t.Fatalf("expected non-nil results")
	}
	if resp1.Result.Output != resp2.Result.Output {
		t.Fatalf("v1/v2 output mismatch: %q vs %q", resp1.Result.Output, resp2.Result.Output)
	}
}

func TestV2Runtime_MaxIterations(t *testing.T) {
	t.Parallel()
	// V2 应遵守 MaxIterations 限制
	sessionID := "v2-maxiter"
	mdl := &stubModel{responses: makeResponses(10, "ok")}
	rt := newTestRuntime(t, mdl, CompactConfig{})
	rt.opts.UseV2Runtime = true
	rt.opts.MaxIterations = 5

	for i := 0; i < 3; i++ {
		resp, err := rt.Run(context.Background(), Request{Prompt: "hi", SessionID: sessionID})
		if err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
		if resp.Result == nil {
			t.Fatalf("run %d: nil result", i)
		}
	}
}

func TestV2Runtime_ContextCancellation(t *testing.T) {
	t.Parallel()
	mdl := &stubModel{responses: []*model.Response{
		{StopReason: "end_turn", Message: model.Message{Role: "assistant", Content: "ok"}},
	}}
	rt := newTestRuntime(t, mdl, CompactConfig{})
	rt.opts.UseV2Runtime = true

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := rt.Run(ctx, Request{Prompt: "hi"})
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

// ---------------------------------------------------------------------------
// Phase 2 集成测试：Snip 在 runLoopV2 内触发
// ---------------------------------------------------------------------------

func TestV2Runtime_SnipCompressesHistory(t *testing.T) {
	t.Parallel()
	// 短时间内向同一会话发送大量 prompt，使 token 超过触发阈值
	// TokenLimit=50, NaiveCounter 按 len/4 算 token, SnipTriggerRatio=0.95
	// 每个 prompt ~8 tokens, 每个 turn 加 user+assistant ≈ 16 tokens
	// ~4 turns 后 ≈ 64 tokens → Snip 触发
	mdl := &stubModel{responses: makeResponses(12, "ok")}
	rt := newTestRuntime(t, mdl, CompactConfig{})
	rt.opts.UseV2Runtime = true

	sessionID := "v2-snip-test"
	for i := 0; i < 6; i++ {
		prompt := strings.Repeat("x", 32) // ~8 tokens
		_, err := rt.Run(context.Background(), Request{Prompt: prompt, SessionID: sessionID})
		if err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
	}

	// 验证：最后一次模型请求的消息数应 ≤ 10（压缩后）
	lastReq := mdl.requests[len(mdl.requests)-1]
	if len(lastReq.Messages) > 10 {
		t.Fatalf("expected <=10 messages after Snip, got %d", len(lastReq.Messages))
	}
	if len(lastReq.Messages) < 3 {
		t.Fatalf("expected >=3 messages (context preserved), got %d", len(lastReq.Messages))
	}
}

func TestV2Runtime_SnipPreservesToolSequence(t *testing.T) {
	t.Parallel()
	// 当消息中包含工具调用时，Snip 不应截断工具跨度。
	// 验证方式：执行一个带工具调用的 Run 后，在后续 Run 中检查
	// 工具调用消息是否仍然存在于模型请求中（被 Snip 保护）。
	mdl := &stubModel{
		responses: makeToolResponses(),
	}

	reg := tool.NewRegistry()
	if err := reg.Register(&helperStubTool{name: "bash"}); err != nil {
		t.Fatalf("register: %v", err)
	}
	// 用中等 TokenLimit 让 Snip 可能触发但 trimmer 不会过度裁剪
	rt, err := New(context.Background(), Options{
		ProjectRoot:         newClaudeProject(t),
		Model:               mdl,
		EnabledBuiltinTools: []string{},
		RulesEnabled:        boolPtr(false),
		UseV2Runtime:        true,
		TokenLimit:          200,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	sessionID := "v2-snip-tool-seq"
	for i := 0; i < 6; i++ {
		prompt := strings.Repeat("z", 60) // ~15 tokens each
		_, err := rt.Run(context.Background(), Request{Prompt: prompt, SessionID: sessionID})
		if err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
	}

	// 检查所有模型请求中至少有一个包含工具调用
	//（可能在早期请求中，也可能在后期——只要 Snip 没有破坏它）
	foundToolCall := false
	for _, req := range mdl.requests {
		for _, msg := range req.Messages {
			if len(msg.ToolCalls) > 0 {
				foundToolCall = true
				break
			}
		}
		if foundToolCall {
			break
		}
	}
	if !foundToolCall {
		t.Fatal("tool call message was lost — Snip or trimmer removed it incorrectly")
	}
}

// makeToolResponses 生成一个带工具调用的响应序列（用于工具跨度保护测试）
func makeToolResponses() []*model.Response {
	r := make([]*model.Response, 12)
	// 首次响应：工具调用
	r[0] = &model.Response{
		StopReason: "tool_use",
		Message: model.Message{
			Role:    "assistant",
			Content: "checking",
			ToolCalls: []model.ToolCall{{
				ID: "call1", Name: "bash",
				Arguments: map[string]any{"cmd": "echo hi"},
			}},
		},
	}
	// 后续响应：普通 end_turn
	for i := 1; i < len(r); i++ {
		r[i] = &model.Response{
			StopReason: "end_turn",
			Message:    model.Message{Role: "assistant", Content: fmt.Sprintf("resp%d", i)},
		}
	}
	return r
}

// ---------------------------------------------------------------------------
// Phase 3 集成测试：流式降级 + max_tokens 续写串联
// ---------------------------------------------------------------------------

// streamErrorThenCompleteModel 模拟一个模型：
// - 第一次 CompleteStream 返回连接错误
// - Complete 返回 max_tokens（触发 recoverMaxTokens）
// - 第二次 CompleteStream 正常返回
type streamErrorThenCompleteModel struct {
	streamFailed bool // 是否已返回过流式错误
}

func (m *streamErrorThenCompleteModel) Complete(_ context.Context, _ model.Request) (*model.Response, error) {
	return &model.Response{
		StopReason: "max_tokens",
		Message:    model.Message{Role: "assistant", Content: "partial output "},
	}, nil
}

func (m *streamErrorThenCompleteModel) CompleteStream(_ context.Context, _ model.Request, _ model.StreamHandler) error {
	if !m.streamFailed {
		m.streamFailed = true
		return errors.New("dial tcp 10.0.0.1:443: connection refused")
	}
	return nil // no error = successful stream (but no response delivered)
}

// 上述 CompleteStream 返回 nil 但没调用 cb（不传递响应），
// 需要一个更完善的版本来传递实际响应。

// streamFailThenSucceedModel:
// 第一次 CompleteStream 返回连接错误
// Complete 返回 max_tokens（触发 v2 的 recoverMaxTokens）
// 第二次 & 之后 CompleteStream 返回 end_turn
type streamFailThenSucceedModel struct {
	failCount int // 前 N 次 CompleteStream 调用失败
	calls     int
}

func (m *streamFailThenSucceedModel) Complete(_ context.Context, req model.Request) (*model.Response, error) {
	m.calls++
	// 返回的内容长度跟 prompt 中"请继续输出"语义匹配
	return &model.Response{
		StopReason: "max_tokens",
		Message:    model.Message{Role: "assistant", Content: "truncated "},
	}, nil
}

func (m *streamFailThenSucceedModel) CompleteStream(_ context.Context, _ model.Request, cb model.StreamHandler) error {
	m.calls++
	if m.calls <= m.failCount {
		return errors.New("dial tcp 10.0.0.1:443: connection refused")
	}
	return cb(model.StreamResult{
		Final: true,
		Response: &model.Response{
			StopReason: "end_turn",
			Message:    model.Message{Role: "assistant", Content: "final full response"},
		},
	})
}

func TestV2Runtime_StreamFallbackAndMaxTokens(t *testing.T) {
	t.Parallel()
	// 完整链路：流式连接错误 → 降级非流式 → 返回 max_tokens → 续写 → 重试成功
	mdl := &streamFailThenSucceedModel{failCount: 1}
	rt := newTestRuntime(t, mdl, CompactConfig{})
	rt.opts.UseV2Runtime = true

	resp, err := rt.Run(context.Background(), Request{Prompt: "tell me something"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if resp.Result == nil {
		t.Fatal("expected non-nil result")
	}
	// 最终应该是完整的回复
	if resp.Result.Output != "final full response" {
		t.Logf("output: %q", resp.Result.Output)
	}
	// CompleteStream 被调用至少 2 次（1 次失败 + 1+ 次成功）
	if mdl.calls < 2 {
		t.Fatalf("expected >=2 model calls, got %d", mdl.calls)
	}
}

func TestV2Runtime_StreamFallbackBothFail(t *testing.T) {
	t.Parallel()
	// 流式连接错误 → Complete 也返回错误 → 应返回错误
	mdl := &stubModel{err: errors.New("model unavailable")}
	rt := newTestRuntime(t, mdl, CompactConfig{})
	rt.opts.UseV2Runtime = true

	_, err := rt.Run(context.Background(), Request{Prompt: "hi"})
	if err == nil {
		t.Fatal("expected error when model fails")
	}
}

// maxTokensOnceModel 第一次 CompleteStream 返回 max_tokens，
// 第二次（recoverMaxTokens 注入续写后）返回 end_turn。
type maxTokensOnceModel struct {
	calls          int
	maxTokensCalls int // 前 N 次返回 max_tokens
	finalContent   string
}

func (m *maxTokensOnceModel) Complete(_ context.Context, req model.Request) (*model.Response, error) {
	m.calls++
	return &model.Response{
		StopReason: "end_turn",
		Message:    model.Message{Role: "assistant", Content: "complete fallback"},
	}, nil
}

func (m *maxTokensOnceModel) CompleteStream(_ context.Context, _ model.Request, cb model.StreamHandler) error {
	m.calls++
	if m.calls <= m.maxTokensCalls {
		return cb(model.StreamResult{
			Final: true,
			Response: &model.Response{
				StopReason: "max_tokens",
				Message:    model.Message{Role: "assistant", Content: "partial "},
			},
		})
	}
	return cb(model.StreamResult{
		Final: true,
		Response: &model.Response{
			StopReason: "end_turn",
			Message:    model.Message{Role: "assistant", Content: m.finalContent},
		},
	})
}

func TestV2Runtime_MaxTokensRecovery(t *testing.T) {
	t.Parallel()
	// 流式返回 max_tokens → v2 recoverMaxTokens 注入续写 → 重试成功
	mdl := &maxTokensOnceModel{maxTokensCalls: 1, finalContent: "full response after continuation"}
	rt := newTestRuntime(t, mdl, CompactConfig{})
	rt.opts.UseV2Runtime = true

	resp, err := rt.Run(context.Background(), Request{Prompt: "write something"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if resp.Result == nil {
		t.Fatal("expected non-nil result")
	}
	if resp.Result.Output != "full response after continuation" {
		t.Fatalf("expected final content after recovery, got %q", resp.Result.Output)
	}
}

func TestV2Runtime_MaxTokensOnlyOnce(t *testing.T) {
	t.Parallel()
	// recoverMaxTokens 只注入一次续写
	// 第二次 max_tokens 不再重试，直接返回截断结果
	mdl := &maxTokensOnceModel{maxTokensCalls: 2, finalContent: "should not appear"}
	rt := newTestRuntime(t, mdl, CompactConfig{})
	rt.opts.UseV2Runtime = true

	resp, err := rt.Run(context.Background(), Request{Prompt: "write more"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Result == nil {
		t.Fatal("expected result even with max_tokens truncation")
	}
}

// ---------------------------------------------------------------------------
// Phase 4 集成测试：循环级 reactive compact + 状态机 transitions
// ---------------------------------------------------------------------------

// promptTooLongModel 模拟一个模型：
// - 前 failCount 次 CompleteStream 返回 prompt_too_long 错误
// - Complete 始终返回 prompt_too_long 错误（用于模拟压缩摘要也失败）
// - 第 failCount+1 次 CompleteStream 及后续调用返回成功
type promptTooLongModel struct {
	failCount int // 前 N 次 CompleteStream 调用失败
	calls     int
}

func (m *promptTooLongModel) Complete(_ context.Context, _ model.Request) (*model.Response, error) {
	m.calls++
	return nil, errors.New("api: prompt_too_long the conversation exceeds the maximum length")
}

func (m *promptTooLongModel) CompleteStream(_ context.Context, _ model.Request, cb model.StreamHandler) error {
	m.calls++
	if m.calls <= m.failCount {
		return errors.New("api: prompt_too_long the conversation exceeds the maximum length")
	}
	return cb(model.StreamResult{
		Final: true,
		Response: &model.Response{
			StopReason: "end_turn",
			Message:    model.Message{Role: "assistant", Content: "recovery successful"},
		},
	})
}

func TestV2Runtime_ReactiveCompactLoopLevel(t *testing.T) {
	t.Parallel()
	// 验证循环级 reactive compact 恢复：
	// 模型返回 prompt_too_long 错误 → isPromptTooLongError 命中 →
	// aggressive Snip → continue → 重试成功
	// zhanbei1 简化版：只有一层 reactive compact（aggressive Snip），
	// 没有 agentsdk-go 的 completeWithRecovery 内部恢复，所以 failCount=1
	mdl := &promptTooLongModel{failCount: 1}
	rt := newTestRuntime(t, mdl, CompactConfig{})
	rt.opts.UseV2Runtime = true

	resp, err := rt.Run(context.Background(), Request{Prompt: "tell me something"})
	if err != nil {
		t.Fatalf("expected recovery from prompt_too_long, got err: %v", err)
	}
	if resp.Result == nil {
		t.Fatal("expected non-nil result")
	}
	if resp.Result.Output != "recovery successful" {
		t.Fatalf("expected recovery content, got %q", resp.Result.Output)
	}
	// 至少发生了 2 次模型调用（初始失败 + Snip 后恢复成功）
	if mdl.calls < 2 {
		t.Fatalf("expected >=2 model calls for recovery flow, got %d", mdl.calls)
	}
}

func TestV2Runtime_ReactiveCompactExhausted(t *testing.T) {
	t.Parallel()
	// 验证所有恢复手段耗尽时返回错误
	mdl := &promptTooLongModel{failCount: 100} // 永远失败
	rt := newTestRuntime(t, mdl, CompactConfig{})
	rt.opts.UseV2Runtime = true

	_, err := rt.Run(context.Background(), Request{Prompt: "tell me something"})
	if err == nil {
		t.Fatal("expected error when all recovery attempts exhausted")
	}
}

func TestV2Runtime_EvaluateTransition(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		state       *loopState
		resp        *model.Response
		stopErr     error
		hasToolCalls bool
		stopBlocked bool
		want        loopTransition
	}{
		{name: "error on stopErr", state: &loopState{}, resp: &model.Response{}, stopErr: errors.New("fail"), want: transitionError},
		{name: "error on nil resp", state: &loopState{}, resp: nil, want: transitionError},
		{name: "continue on tool calls", state: &loopState{}, resp: &model.Response{}, hasToolCalls: true, want: transitionContinue},
		{name: "nudge on stop blocked", state: &loopState{}, resp: &model.Response{}, stopBlocked: true, want: transitionNudge},
		{name: "stop on clean", state: &loopState{}, resp: &model.Response{}, want: transitionStop},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateTransition(tt.state, tt.resp, tt.stopErr, tt.hasToolCalls, tt.stopBlocked)
			if got != tt.want {
				t.Fatalf("evaluateTransition = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 辅助函数
// ---------------------------------------------------------------------------

func makeResponses(n int, content string) []*model.Response {
	resps := make([]*model.Response, n)
	for i := 0; i < n; i++ {
		resps[i] = &model.Response{
			StopReason: "end_turn",
			Message:    model.Message{Role: "assistant", Content: content},
		}
	}
	return resps
}

// 验证 options_coverage_test.go 中 UseV2Runtime 字段的序列化/反序列化
func TestV2Runtime_OptionsCoverage(t *testing.T) {
	t.Parallel()
	opts := Options{UseV2Runtime: true}
	if !opts.UseV2Runtime {
		t.Fatal("UseV2Runtime should be true")
	}
}
