package api

import (
	"strings"
	"testing"

	"github.com/stellarlinkco/agentsdk-go/pkg/evolution"
)

func TestAugmentSystemPromptWithEvolution_Order(t *testing.T) {
	base := "Base instructions."
	snap := evolution.Snapshot{
		Soul:   "SOUL BLOCK",
		Prompt: "PROMPT BLOCK",
		Memory: "MEMORY BLOCK",
		User:   "USER BLOCK",
	}
	got := augmentSystemPromptWithEvolution(base, snap)
	for _, want := range []string{"SOUL BLOCK", "Base instructions.", "PROMPT BLOCK", "MEMORY BLOCK", "USER BLOCK"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
	if strings.Index(got, "SOUL BLOCK") > strings.Index(got, "Base instructions.") {
		t.Fatal("soul should precede base prompt")
	}
}
