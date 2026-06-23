package toolbuiltin

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/stellarlinkco/agentsdk-go/pkg/tool"
)

const currentTimeDescription = `Get the current date and time in any IANA timezone.
Use for time-aware behavior without assuming the user's timezone in the system prompt.`

var currentTimeSchema = &tool.JSONSchema{
	Type: "object",
	Properties: map[string]interface{}{
		"timezone": map[string]interface{}{
			"type":        "string",
			"description": "IANA timezone (e.g. Asia/Shanghai, America/New_York). Default: UTC",
		},
		"format": map[string]interface{}{
			"type":        "string",
			"description": "Go time layout (default: RFC3339). Use \"unix\" for Unix seconds.",
		},
	},
}

// CurrentTimeTool returns the current time in a requested timezone.
type CurrentTimeTool struct{}

func (CurrentTimeTool) Name() string { return "current_time" }

func (CurrentTimeTool) Description() string { return currentTimeDescription }

func (CurrentTimeTool) Schema() *tool.JSONSchema { return currentTimeSchema }

func (CurrentTimeTool) Execute(ctx context.Context, params map[string]interface{}) (*tool.ToolResult, error) {
	_ = ctx
	tzName := strings.TrimSpace(stringParam(params, "timezone"))
	if tzName == "" {
		tzName = "UTC"
	}
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		return toolErrorResult(fmt.Sprintf("invalid timezone %q: %v", tzName, err)), nil
	}
	now := time.Now().In(loc)
	format := strings.TrimSpace(stringParam(params, "format"))
	var formatted string
	switch strings.ToLower(format) {
	case "unix":
		formatted = fmt.Sprintf("%d", now.Unix())
	case "":
		formatted = now.Format(time.RFC3339)
	default:
		formatted = now.Format(format)
	}
	out := fmt.Sprintf("timezone: %s\nlocal: %s\nutc: %s", tzName, formatted, now.UTC().Format(time.RFC3339))
	return &tool.ToolResult{Success: true, Output: out}, nil
}
