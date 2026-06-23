package toolbuiltin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stellarlinkco/agentsdk-go/pkg/tool"
)

// BrowserHandler executes browser automation requests. Inject from the host runtime
// (e.g. OpenOcta rod/Chromium service). When nil, the browser tool returns a clear error.
type BrowserHandler func(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error)

const browserDescription = `Control a built-in Chromium browser when a BrowserHandler is configured by the host runtime.
Actions: status, start, stop, tabs, open, close, navigate, snapshot, screenshot, act.
For multi-step UI flows: open once, snapshot to get refs, then act with request.kind click/type/press/wait.`

var browserSchema = &tool.JSONSchema{
	Type: "object",
	Properties: map[string]interface{}{
		"action": map[string]interface{}{
			"type":        "string",
			"description": "status | start | stop | tabs | open | close | navigate | snapshot | screenshot | act",
		},
		"url": map[string]interface{}{
			"type":        "string",
			"description": "URL for open/navigate",
		},
		"targetUrl": map[string]interface{}{
			"type":        "string",
			"description": "Alias for url",
		},
		"targetId": map[string]interface{}{
			"type":        "string",
			"description": "Tab id (t1) or label from open",
		},
		"label": map[string]interface{}{
			"type":        "string",
			"description": "Stable tab label for open",
		},
		"request": map[string]interface{}{
			"type":        "object",
			"description": "act payload: kind (click|type|press|wait), ref, text, key",
		},
	},
	Required: []string{"action"},
}

// BrowserTool exposes browser automation via an injected handler.
type BrowserTool struct {
	Handler BrowserHandler
}

// NewBrowserTool builds a browser tool with the given handler.
func NewBrowserTool(handler BrowserHandler) *BrowserTool {
	return &BrowserTool{Handler: handler}
}

func (BrowserTool) Name() string { return "browser" }

func (BrowserTool) Description() string { return browserDescription }

func (BrowserTool) Schema() *tool.JSONSchema { return browserSchema }

func (t *BrowserTool) Execute(ctx context.Context, params map[string]interface{}) (*tool.ToolResult, error) {
	action := strings.TrimSpace(stringParam(params, "action"))
	if action == "" {
		return toolErrorResult("action is required"), nil
	}
	if t == nil || t.Handler == nil {
		return toolErrorResult("browser tool is not configured; inject a BrowserHandler in runtime Options"), nil
	}
	payload, err := t.Handler(ctx, params)
	if err != nil {
		return toolErrorResult(err.Error()), nil
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return &tool.ToolResult{Success: true, Output: fmt.Sprintf("%v", payload)}, nil
	}
	return &tool.ToolResult{Success: true, Output: string(raw)}, nil
}
