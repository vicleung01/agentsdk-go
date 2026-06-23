package a2ui

import (
	"strings"
	"testing"
)

func TestStripCommandConfirmationButtons(t *testing.T) {
	raw := []map[string]any{
		{
			"id":        "root",
			"component": "Column",
			"children":  []any{"title", "desc", "confirmBtn"},
		},
		{"id": "title", "component": "Text", "text": "需要确认"},
		{
			"id":        "desc",
			"component": "Text",
			"text":      "你要求执行 `ls` 命令。按照安全规则，需要你先确认。",
		},
		{
			"id":        "confirmBtn",
			"component": "Button",
			"label":     "确认执行 ls",
			"action":    map[string]any{"name": "confirm_ls"},
		},
	}
	got := stripCommandConfirmationButtons(repairButtonLabels(raw))
	for _, comp := range got {
		if comp["component"] == "Button" {
			t.Fatalf("expected no buttons, got %#v", got)
		}
	}
	desc := findComponentByID(got, "desc")
	if desc == nil {
		t.Fatal("missing desc")
	}
	text, _ := desc["text"].(string)
	if !strings.Contains(text, "聊天输入框") {
		t.Fatalf("expected chat input hint, got %q", text)
	}
	root := findComponentByID(got, "root")
	children, _ := root["children"].([]any)
	if len(children) != 2 {
		t.Fatalf("expected title+desc only, children=%v", children)
	}
}

func findComponentByID(components []map[string]any, id string) map[string]any {
	for _, comp := range components {
		if compID, _ := comp["id"].(string); compID == id {
			return comp
		}
	}
	return nil
}
