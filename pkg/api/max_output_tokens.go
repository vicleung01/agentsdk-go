package api

import (
	"github.com/stellarlinkco/agentsdk-go/pkg/message"
	"github.com/stellarlinkco/agentsdk-go/pkg/model"
)

// recoverMaxTokens checks if the response was truncated due to max_tokens.
// If so and recovery hasn't been attempted yet, it injects a continuation
// message into history and returns true (caller should re-run the model call).
// Returns false when no recovery is needed or possible.
//
// Skips recovery when the response contains tool calls -- the tools should be
// executed first, and their results provide enough context to continue.
func (rt *Runtime) recoverMaxTokens(resp *model.Response, state *loopState, hist *message.History) bool {
	if resp == nil {
		return false
	}
	if resp.StopReason != "max_tokens" {
		return false
	}
	if len(resp.Message.ToolCalls) > 0 {
		// Tool calls are more important; execute them first.
		return false
	}
	if state.maxOutputTokensRecovery >= 1 {
		return false
	}

	state.maxOutputTokensRecovery++

	hist.Append(message.Message{
		Role:    "user",
		Content: "请继续输出，上次输出被截断。",
	})
	return true
}