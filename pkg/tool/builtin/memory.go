package toolbuiltin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/stellarlinkco/agentsdk-go/pkg/evolution"
	"github.com/stellarlinkco/agentsdk-go/pkg/tool"
)

const memoryToolDescription = `Save durable information to persistent curated memory that survives across sessions.

WHEN TO SAVE (proactively):
- User corrects you or says "remember this"
- User shares preferences, habits, or personal details
- You discover environment facts, project conventions, or tool quirks
- You learn standing behavior that should change your identity or system prompt

TARGETS:
- memory: agent notes (environment, conventions, lessons learned)
- user: user profile (name, preferences, communication style)
- soul: agent identity overlay (tone, personality, standing behavior)
- prompt: evolved system prompt guidance (durable instructions beyond soul)

ACTIONS: add, replace (old_text identifies entry), remove (old_text identifies entry).

Mid-session writes persist to disk immediately but appear in the system prompt only on the next session (frozen snapshot).`

var memoryToolSchema = &tool.JSONSchema{
	Type: "object",
	Properties: map[string]interface{}{
		"action": map[string]interface{}{
			"type":        "string",
			"enum":        []string{"add", "replace", "remove"},
			"description": "The action to perform.",
		},
		"target": map[string]interface{}{
			"type":        "string",
			"enum":        []string{"memory", "user", "soul", "prompt"},
			"description": "Which store to update.",
		},
		"content": map[string]interface{}{
			"type":        "string",
			"description": "Entry content. Required for add and replace.",
		},
		"old_text": map[string]interface{}{
			"type":        "string",
			"description": "Short unique substring identifying the entry to replace or remove.",
		},
	},
	Required: []string{"action", "target"},
}

// MemoryTool exposes curated L4 evolution memory to the agent runtime.
type MemoryTool struct {
	store *evolution.Store
}

// NewMemoryTool wires the evolution store into a tool. store must be non-nil.
func NewMemoryTool(store *evolution.Store) *MemoryTool {
	return &MemoryTool{store: store}
}

func (m *MemoryTool) Name() string { return "memory" }

func (m *MemoryTool) Description() string { return memoryToolDescription }

func (m *MemoryTool) Schema() *tool.JSONSchema { return memoryToolSchema }

func (m *MemoryTool) Execute(ctx context.Context, params map[string]interface{}) (*tool.ToolResult, error) {
	if ctx == nil {
		return nil, errors.New("context is nil")
	}
	if m == nil || m.store == nil {
		return nil, errors.New("memory tool is not initialised")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	action := strings.ToLower(strings.TrimSpace(stringParam(params, "action")))
	targetRaw := stringParam(params, "target")
	if strings.TrimSpace(targetRaw) == "" {
		targetRaw = "memory"
	}
	target, err := evolution.ParseTarget(targetRaw)
	if err != nil {
		return toolErrorResult(err.Error()), nil
	}

	var result map[string]any
	switch action {
	case "add":
		content := stringParam(params, "content")
		if strings.TrimSpace(content) == "" {
			return toolErrorResult("content is required for add"), nil
		}
		result, err = m.store.Add(target, content)
	case "replace":
		oldText := stringParam(params, "old_text")
		content := stringParam(params, "content")
		if strings.TrimSpace(oldText) == "" {
			return toolErrorResult("old_text is required for replace"), nil
		}
		if strings.TrimSpace(content) == "" {
			return toolErrorResult("content is required for replace"), nil
		}
		result, err = m.store.Replace(target, oldText, content)
	case "remove":
		oldText := stringParam(params, "old_text")
		if strings.TrimSpace(oldText) == "" {
			return toolErrorResult("old_text is required for remove"), nil
		}
		result, err = m.store.Remove(target, oldText)
	default:
		return toolErrorResult("unknown action; use add, replace, or remove"), nil
	}
	if err != nil {
		return toolErrorResult(err.Error()), nil
	}
	out, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("memory tool: marshal result: %w", err)
	}
	return &tool.ToolResult{Success: true, Output: string(out)}, nil
}

func stringParam(params map[string]interface{}, key string) string {
	if params == nil {
		return ""
	}
	v, ok := params[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return fmt.Sprint(t)
	}
}

func toolErrorResult(msg string) *tool.ToolResult {
	out, _ := json.Marshal(map[string]any{"success": false, "error": msg})
	return &tool.ToolResult{Success: false, Output: string(out)}
}
