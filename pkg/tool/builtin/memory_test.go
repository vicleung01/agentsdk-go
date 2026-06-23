package toolbuiltin

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stellarlinkco/agentsdk-go/pkg/evolution"
)

func TestMemoryToolAddAndReplace(t *testing.T) {
	store, err := evolution.Open(evolution.Config{Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	tool := NewMemoryTool(store)
	ctx := context.Background()

	res, err := tool.Execute(ctx, map[string]interface{}{
		"action":  "add",
		"target":  "user",
		"content": "prefers concise answers",
	})
	if err != nil {
		t.Fatalf("execute add: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success: %s", res.Output)
	}

	res, err = tool.Execute(ctx, map[string]interface{}{
		"action":   "replace",
		"target":   "user",
		"old_text": "concise",
		"content":  "prefers concise Chinese answers",
	})
	if err != nil {
		t.Fatalf("execute replace: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(res.Output), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["success"] != true {
		t.Fatalf("expected success payload: %s", res.Output)
	}
}
