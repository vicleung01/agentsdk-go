package a2ui

import "testing"

func TestMessagesFromPlainText(t *testing.T) {
	msgs := MessagesFromPlainText("  hello  ")
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].CreateSurface == nil || msgs[0].CreateSurface.SurfaceID != "main" {
		t.Fatalf("expected createSurface main, got %#v", msgs[0].CreateSurface)
	}
	if msgs[1].UpdateComponents == nil {
		t.Fatal("expected updateComponents")
	}
	if len(msgs[1].UpdateComponents.Components) < 2 {
		t.Fatalf("expected at least 2 components, got %d", len(msgs[1].UpdateComponents.Components))
	}
}

func TestMessagesFromPlainTextEmpty(t *testing.T) {
	if msgs := MessagesFromPlainText(" \n\t"); msgs != nil {
		t.Fatalf("expected nil for empty text, got %#v", msgs)
	}
}
