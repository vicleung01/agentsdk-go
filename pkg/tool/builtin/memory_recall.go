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

const memoryRecallDescription = `Search and list durable curated memory entries (memory tree).

Use memory tool to add/replace/remove entries. Use memory_recall to find or list existing notes before acting.

TARGETS: memory (agent notes), user (profile), soul (identity), prompt (evolved instructions).
ACTIONS: search (keyword match), list (all entries in target).`

var memoryRecallSchema = &tool.JSONSchema{
	Type: "object",
	Properties: map[string]interface{}{
		"action": map[string]interface{}{
			"type":        "string",
			"enum":        []string{"search", "list"},
			"description": "search or list",
		},
		"target": map[string]interface{}{
			"type":        "string",
			"enum":        []string{"memory", "user", "soul", "prompt"},
			"description": "Which store to query (default: memory)",
		},
		"query": map[string]interface{}{
			"type":        "string",
			"description": "Keyword for search action",
		},
		"limit": map[string]interface{}{
			"type":        "integer",
			"description": "Max results for search (default 10)",
		},
	},
	Required: []string{"action"},
}

// MemoryRecallTool searches and lists curated evolution memory entries.
type MemoryRecallTool struct {
	store *evolution.Store
}

// NewMemoryRecallTool wires the evolution store into a recall tool.
func NewMemoryRecallTool(store *evolution.Store) *MemoryRecallTool {
	return &MemoryRecallTool{store: store}
}

func (m *MemoryRecallTool) Name() string { return "memory_recall" }

func (m *MemoryRecallTool) Description() string { return memoryRecallDescription }

func (m *MemoryRecallTool) Schema() *tool.JSONSchema { return memoryRecallSchema }

func (m *MemoryRecallTool) Execute(ctx context.Context, params map[string]interface{}) (*tool.ToolResult, error) {
	if ctx == nil {
		return nil, errors.New("context is nil")
	}
	if m == nil || m.store == nil {
		return nil, errors.New("memory_recall tool is not initialised")
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

	limit := 10
	if v, ok := params["limit"].(float64); ok && int(v) > 0 {
		limit = int(v)
	}

	var entries []string
	switch action {
	case "list":
		entries, err = m.store.ListEntries(target)
	case "search":
		query := stringParam(params, "query")
		if strings.TrimSpace(query) == "" {
			return toolErrorResult("query is required for search"), nil
		}
		entries, err = m.store.SearchEntries(target, query, limit)
	default:
		return toolErrorResult("unknown action; use search or list"), nil
	}
	if err != nil {
		return toolErrorResult(err.Error()), nil
	}

	result := map[string]any{
		"success":     true,
		"action":      action,
		"target":      string(target),
		"entry_count": len(entries),
		"entries":     entries,
	}
	out, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("memory_recall: marshal result: %w", err)
	}
	return &tool.ToolResult{Success: true, Output: string(out)}, nil
}
