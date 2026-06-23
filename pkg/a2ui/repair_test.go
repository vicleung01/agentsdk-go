package a2ui

import "testing"

func findUpdateComponents(repaired []*ServerMessage) *UpdateComponents {
	for _, msg := range repaired {
		if msg != nil && msg.UpdateComponents != nil {
			return msg.UpdateComponents
		}
	}
	return nil
}

func TestRepairMessages_createSurfaceOnly(t *testing.T) {
	msgs, err := ParseLine(`{"createSurface":{"surfaceId":"confirm_ls","catalogId":"basic"}}`)
	if err != nil {
		t.Fatal(err)
	}
	repaired := RepairMessages([]*ServerMessage{msgs})
	if len(repaired) != 2 {
		t.Fatalf("got %d messages, want createSurface + updateComponents", len(repaired))
	}
	if repaired[1].UpdateComponents == nil || !updateComponentsHasRoot(repaired[1].UpdateComponents) {
		t.Fatal("expected synthesized updateComponents with root")
	}
}

func TestRepairMessages_nestedComponent(t *testing.T) {
	raw := `{"updateComponents":{"surfaceId":"main","components":[{"id":"title","component":{"Text":{"text":{"literalString":"Hi"}}}}]}}`
	msg, err := ParseLine(raw)
	if err != nil {
		t.Fatal(err)
	}
	repaired := RepairMessages([]*ServerMessage{msg})
	uc := findUpdateComponents(repaired)
	if uc == nil {
		t.Fatal("expected updateComponents")
	}
	comp := uc.Components
	var rootComp, titleComp map[string]any
	for _, c := range comp {
		switch c["id"] {
		case "root":
			rootComp = c
		case "title":
			titleComp = c
		}
	}
	if rootComp == nil || titleComp == nil {
		t.Fatalf("components=%v", comp)
	}
	if rootComp["component"] != "Column" {
		t.Fatalf("root component=%v", rootComp["component"])
	}
}

func TestRepairMessages_missingChildReference(t *testing.T) {
	raw := `{"updateComponents":{"surfaceId":"main","components":[
		{"id":"root","component":"Column","children":["title","btn"]}
	]}}`
	msg, err := ParseLine(raw)
	if err != nil {
		t.Fatal(err)
	}
	repaired := RepairMessages([]*ServerMessage{msg})
	uc := findUpdateComponents(repaired)
	if uc == nil {
		t.Fatal("expected updateComponents")
	}
	comp := uc.Components
	ids := map[string]bool{}
	for _, c := range comp {
		if id, ok := c["id"].(string); ok {
			ids[id] = true
		}
	}
	for _, want := range []string{"root", "title", "btn", "btn_label"} {
		if !ids[want] {
			t.Fatalf("missing synthesized component %q in %v", want, comp)
		}
	}
}

func TestRepairMessages_updateComponentsOnly(t *testing.T) {
	raw := `{"updateComponents":{"surfaceId":"main","components":[
		{"id":"root","component":"Column","children":["title"]},
		{"id":"title","component":"Text","text":"Hi"}
	]}}`
	msg, err := ParseLine(raw)
	if err != nil {
		t.Fatal(err)
	}
	repaired := RepairMessages([]*ServerMessage{msg})
	if len(repaired) < 2 {
		t.Fatalf("expected createSurface + updateComponents, got %d messages", len(repaired))
	}
	if repaired[0].CreateSurface == nil || repaired[0].CreateSurface.SurfaceID != "main" {
		t.Fatalf("expected injected createSurface, got %#v", repaired[0])
	}
}

func TestRepairMessages_coalesceSplitUpdates(t *testing.T) {
	msgs, err := ParseMessages([]any{
		map[string]any{"createSurface": map[string]any{"surfaceId": "main", "catalogId": "basic"}},
		map[string]any{"updateComponents": map[string]any{
			"surfaceId": "main",
			"components": []any{
				map[string]any{"id": "root", "component": "Column", "children": []any{"title"}},
			},
		}},
		map[string]any{"updateComponents": map[string]any{
			"surfaceId": "main",
			"components": []any{
				map[string]any{"id": "title", "component": "Text", "text": "Hello"},
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	repaired := RepairMessages(msgs)
	var uc *UpdateComponents
	for _, msg := range repaired {
		if msg.UpdateComponents != nil && msg.UpdateComponents.SurfaceID == "main" {
			uc = msg.UpdateComponents
			break
		}
	}
	if uc == nil {
		t.Fatal("expected merged updateComponents")
	}
	ids := map[string]bool{}
	for _, c := range uc.Components {
		if id, ok := c["id"].(string); ok {
			ids[id] = true
		}
	}
	if !ids["root"] || !ids["title"] {
		t.Fatalf("expected root+title after coalesce, got %v", uc.Components)
	}
}

func TestRepairMessages_layoutRenamedToRoot(t *testing.T) {
	raw := `{"updateComponents":{"surfaceId":"main","components":[
		{"id":"dialog","component":"Column","children":["title"]},
		{"id":"title","component":"Text","text":"Hi"}
	]}}`
	msg, err := ParseLine(raw)
	if err != nil {
		t.Fatal(err)
	}
	repaired := RepairMessages([]*ServerMessage{msg})
	uc := findUpdateComponents(repaired)
	if uc == nil {
		t.Fatal("expected updateComponents")
	}
	comp := uc.Components
	if comp[0]["id"] != "root" {
		t.Fatalf("expected dialog renamed to root, got %v", comp[0]["id"])
	}
}
