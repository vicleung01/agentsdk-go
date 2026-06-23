package a2ui

import "testing"

func TestParseLine_v09(t *testing.T) {
	msg, err := ParseLine(`{"version":"v0.9","createSurface":{"surfaceId":"main","catalogId":"basic"}}`)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Kind() != "createSurface" {
		t.Fatalf("kind=%q", msg.Kind())
	}
}

func TestParseLine_v08SurfaceUpdate(t *testing.T) {
	msg, err := ParseLine(`{"surfaceUpdate":{"surfaceId":"main","components":[{"id":"root","component":{"Column":{}}}]}}`)
	if err != nil {
		t.Fatal(err)
	}
	n := msg.Normalize()
	if n.UpdateComponents == nil {
		t.Fatal("expected normalized updateComponents")
	}
}

func TestLineBufferPush(t *testing.T) {
	var lb LineBuffer
	msgs, err := lb.Push("{\"createSurface\":{\"surfaceId\":\"a\"}}\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages after first line", len(msgs))
	}
	msgs2, err := lb.Push("{\"updateDataModel\":{\"surfaceId\":\"a\",\"path\":\"/x\",\"value\":1}}\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs2) != 1 {
		t.Fatalf("got %d messages after second line", len(msgs2))
	}
}

func TestExtractFromFencedBlock(t *testing.T) {
	text := "Here is UI:\n```a2ui\n{\"createSurface\":{\"surfaceId\":\"main\"}}\n```\n"
	lines := ExtractFromFencedBlock(text)
	if len(lines) != 1 {
		t.Fatalf("lines=%v", lines)
	}
}
