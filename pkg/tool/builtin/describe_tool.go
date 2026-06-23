package toolbuiltin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stellarlinkco/agentsdk-go/pkg/tool"
)

// DescribeToolName is the canonical name of the lazy schema lookup tool.
const DescribeToolName = "describe_tool"

// DescribeTool returns full parameter schemas for tools registered in the runtime.
type DescribeTool struct {
	registry *tool.Registry
}

// NewDescribeTool creates a tool that looks up schemas from an existing registry.
func NewDescribeTool(registry *tool.Registry) *DescribeTool {
	return &DescribeTool{registry: registry}
}

func (t *DescribeTool) Name() string { return DescribeToolName }

func (t *DescribeTool) Description() string {
	return "Return the full JSON parameter schema and description for tool name(s). " +
		"Use before calling unfamiliar tools when the model only sees tool names and short descriptions."
}

func (t *DescribeTool) Schema() *tool.JSONSchema {
	return &tool.JSONSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"tool_name": map[string]interface{}{
				"type":        "string",
				"description": "Single tool name to describe.",
			},
			"tool_names": map[string]interface{}{
				"type":        "array",
				"description": "Multiple tool names to describe.",
				"items":       map[string]interface{}{"type": "string"},
			},
		},
	}
}

func (t *DescribeTool) Execute(_ context.Context, params map[string]interface{}) (*tool.ToolResult, error) {
	if t == nil || t.registry == nil {
		return &tool.ToolResult{Success: false, Output: "describe_tool: registry not configured"}, nil
	}
	names := collectToolNames(params)
	if len(names) == 0 {
		return &tool.ToolResult{Success: false, Output: "describe_tool: provide tool_name or tool_names"}, nil
	}

	out := make(map[string]interface{}, len(names))
	var missing []string
	for _, name := range names {
		impl, err := t.registry.Get(name)
		if err != nil {
			missing = append(missing, name)
			continue
		}
		entry := map[string]interface{}{
			"name":        strings.TrimSpace(impl.Name()),
			"description": strings.TrimSpace(impl.Description()),
		}
		if schema := impl.Schema(); schema != nil {
			entry["parameters"] = schema
		}
		out[name] = entry
	}

	payload := map[string]interface{}{"tools": out}
	if len(missing) > 0 {
		payload["missing"] = missing
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return &tool.ToolResult{Success: false, Output: fmt.Sprintf("describe_tool: %v", err)}, nil
	}
	return &tool.ToolResult{Success: true, Output: string(raw), Data: payload}, nil
}

func collectToolNames(params map[string]interface{}) []string {
	if params == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var names []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		names = append(names, s)
	}
	if v, ok := params["tool_name"].(string); ok {
		add(v)
	}
	switch raw := params["tool_names"].(type) {
	case []string:
		for _, n := range raw {
			add(n)
		}
	case []interface{}:
		for _, item := range raw {
			if s, ok := item.(string); ok {
				add(s)
			}
		}
	}
	return names
}
