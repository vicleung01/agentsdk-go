package toolbuiltin

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stellarlinkco/agentsdk-go/pkg/evolution"
)

func TestMemoryRecallSearchAndList(t *testing.T) {
	store, err := evolution.Open(evolution.Config{Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	tool := NewMemoryRecallTool(store)
	ctx := context.Background()

	_, err = store.Add(evolution.TargetMemory, "user prefers Go for backend")
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	_, err = store.Add(evolution.TargetMemory, "project uses table-driven tests")
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	res, err := tool.Execute(ctx, map[string]interface{}{
		"action": "search",
		"target": "memory",
		"query":  "Go",
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	var searchPayload map[string]any
	if err := json.Unmarshal([]byte(res.Output), &searchPayload); err != nil {
		t.Fatalf("unmarshal search: %v", err)
	}
	if searchPayload["entry_count"].(float64) != 1 {
		t.Fatalf("expected 1 match, got %#v", searchPayload)
	}

	res, err = tool.Execute(ctx, map[string]interface{}{
		"action": "list",
		"target": "memory",
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var listPayload map[string]any
	if err := json.Unmarshal([]byte(res.Output), &listPayload); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	if listPayload["entry_count"].(float64) != 2 {
		t.Fatalf("expected 2 entries, got %#v", listPayload)
	}
}
