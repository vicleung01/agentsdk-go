package toolbuiltin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stellarlinkco/agentsdk-go/pkg/a2ui"
	"github.com/stellarlinkco/agentsdk-go/pkg/tool"
)

const (
	toolA2UIPush  = "a2ui_push"
	toolA2UIReset = "a2ui_reset"
)

var a2uiPushSchema = &tool.JSONSchema{
	Type: "object",
	Properties: map[string]interface{}{
		"messages": map[string]interface{}{
			"type":        "array",
			"description": "Required. A2UI v0.9 server messages — each object has exactly one action key: createSurface, updateComponents, updateDataModel, or deleteSurface",
		},
		"jsonl": map[string]interface{}{
			"type":        "string",
			"description": "Alternative to messages: JSONL string of A2UI messages (one JSON object per line)",
		},
	},
}

// A2UIPushTool pushes A2UI messages to the connected client via stream events.
type A2UIPushTool struct{}

func NewA2UIPushTool() *A2UIPushTool { return &A2UIPushTool{} }

func (t *A2UIPushTool) Name() string { return toolA2UIPush }

func (t *A2UIPushTool) Description() string {
	return fmt.Sprintf(`Push A2UI v0.9 messages to render the user-visible reply in the chat UI.
Always use this tool for user-facing responses instead of plain markdown text.

Pass a "messages" array. Each item is one A2UI server message with exactly one action key:
createSurface, updateComponents, updateDataModel, or deleteSurface.

Typical flow:
1. createSurface — {"surfaceId":"main","catalogId":%q}
2. updateComponents — add UI components
3. updateDataModel — bind data (optional)

Example call:
{"messages":[
  {"createSurface":{"surfaceId":"main","catalogId":%q}},
  {"updateComponents":{"surfaceId":"main","components":[
    {"id":"root","component":"Text","text":"Hello"},
    {"id":"btn","component":"Button","label":"OK","action":{"name":"confirm"}}
  ]}}
]}`, a2ui.BasicCatalogID, a2ui.BasicCatalogID)
}

func (t *A2UIPushTool) Schema() *tool.JSONSchema { return a2uiPushSchema }

func (t *A2UIPushTool) Execute(_ context.Context, params map[string]interface{}) (*tool.ToolResult, error) {
	msgs, err := a2ui.ExtractMessagesFromToolParams(params)
	if err != nil {
		return &tool.ToolResult{Success: false, Output: fmt.Sprintf("invalid A2UI messages: %v", err)}, nil
	}
	if len(msgs) == 0 {
		return &tool.ToolResult{
			Success: false,
			Output:  "no A2UI messages provided: pass {\"messages\":[...]} with createSurface/updateComponents/updateDataModel/deleteSurface objects",
		}, nil
	}
	arr := make([]json.RawMessage, 0, len(msgs))
	for _, m := range msgs {
		rawJSON, err := m.RawJSON()
		if err != nil {
			return &tool.ToolResult{Success: false, Output: err.Error()}, nil
		}
		arr = append(arr, rawJSON)
	}
	return &tool.ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Pushed %d A2UI message(s) to client", len(msgs)),
		Data:    map[string]any{"a2ui_messages": arr, "count": len(msgs)},
	}, nil
}

// A2UIResetTool deletes an A2UI surface on the client.
type A2UIResetTool struct{}

func NewA2UIResetTool() *A2UIResetTool { return &A2UIResetTool{} }

func (t *A2UIResetTool) Name() string { return toolA2UIReset }

func (t *A2UIResetTool) Description() string {
	return "Reset/delete an A2UI surface on the client. Provide surfaceId (default: main)."
}

func (t *A2UIResetTool) Schema() *tool.JSONSchema {
	return &tool.JSONSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"surfaceId": map[string]interface{}{
				"type":        "string",
				"description": "Surface ID to delete (default: main)",
			},
		},
	}
}

func (t *A2UIResetTool) Execute(_ context.Context, params map[string]interface{}) (*tool.ToolResult, error) {
	surfaceID := "main"
	if params != nil {
		if v, ok := params["surfaceId"].(string); ok && strings.TrimSpace(v) != "" {
			surfaceID = strings.TrimSpace(v)
		}
	}
	msg := (&a2ui.ServerMessage{
		Version:       a2ui.Version,
		DeleteSurface: &a2ui.DeleteSurface{SurfaceID: surfaceID},
	}).Normalize()
	rawJSON, err := msg.RawJSON()
	if err != nil {
		return &tool.ToolResult{Success: false, Output: err.Error()}, nil
	}
	return &tool.ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Reset A2UI surface %q", surfaceID),
		Data:    map[string]any{"a2ui_messages": []json.RawMessage{rawJSON}},
	}, nil
}

// A2UITools returns built-in A2UI protocol tools.
func A2UITools() []tool.Tool {
	return []tool.Tool{NewA2UIPushTool(), NewA2UIResetTool()}
}
