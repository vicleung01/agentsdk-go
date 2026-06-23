package toolbuiltin

import (
	"context"
	"testing"

	"github.com/stellarlinkco/agentsdk-go/pkg/tool"
)

func TestDescribeToolReturnsSchema(t *testing.T) {
	reg := tool.NewRegistry()
	if err := reg.Register(&helperEchoTool{}); err != nil {
		t.Fatalf("register: %v", err)
	}
	dt := NewDescribeTool(reg)
	res, err := dt.Execute(context.Background(), map[string]interface{}{
		"tool_names": []interface{}{"echo"},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if res == nil || !res.Success {
		t.Fatalf("result=%v", res)
	}
	if !contains(res.Output, "parameters") {
		t.Fatalf("expected parameters in output: %s", res.Output)
	}
}

type helperEchoTool struct{}

func (helperEchoTool) Name() string { return "echo" }
func (helperEchoTool) Description() string {
	return "echo tool"
}
func (helperEchoTool) Schema() *tool.JSONSchema {
	return &tool.JSONSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"text": map[string]interface{}{"type": "string"},
		},
		Required: []string{"text"},
	}
}
func (helperEchoTool) Execute(context.Context, map[string]interface{}) (*tool.ToolResult, error) {
	return &tool.ToolResult{Success: true, Output: "ok"}, nil
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
