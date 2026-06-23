package model

import "testing"

func TestNormalizeAssistantContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		content      string
		hasToolCalls bool
		want         string
	}{
		{name: "placeholder with tools", content: ".", hasToolCalls: true, want: ""},
		{name: "placeholder without tools", content: ".", hasToolCalls: false, want: "."},
		{name: "real text with tools", content: "checking chrome", hasToolCalls: true, want: "checking chrome"},
		{name: "empty with tools", content: "", hasToolCalls: true, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeAssistantContent(tt.content, tt.hasToolCalls)
			if got != tt.want {
				t.Fatalf("NormalizeAssistantContent(%q, %v) = %q, want %q", tt.content, tt.hasToolCalls, got, tt.want)
			}
		})
	}
}
