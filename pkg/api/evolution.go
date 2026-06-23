package api

import (
	"strings"

	"github.com/stellarlinkco/agentsdk-go/pkg/evolution"
)

// EvolutionOptions configures L4 autonomous evolution: curated MEMORY.md /
// USER.md plus optional SOUL.md / PROMPT.md overlays. Inspired by Hermes
// Agent's frozen-snapshot memory pattern.
type EvolutionOptions struct {
	// Enabled turns on the evolution store, memory tool, and prompt injection.
	Enabled bool
	// Dir stores MEMORY.md, USER.md, SOUL.md, PROMPT.md. Default: <ProjectRoot>/.agents/evolution
	Dir string
	// MemoryCharLimit caps agent notes (default 2200 runes).
	MemoryCharLimit int
	// UserCharLimit caps user profile notes (default 1375 runes).
	UserCharLimit int
	// SoulCharLimit caps identity overlay (default 4000 runes).
	SoulCharLimit int
	// PromptCharLimit caps evolved system prompt overlay (default 2000 runes).
	PromptCharLimit int
}

func evolutionConfigFrom(opts Options) evolution.Config {
	cfg := evolution.Config{}
	if opts.Evolution != nil {
		cfg.Dir = strings.TrimSpace(opts.Evolution.Dir)
		cfg.MemoryCharLimit = opts.Evolution.MemoryCharLimit
		cfg.UserCharLimit = opts.Evolution.UserCharLimit
		cfg.SoulCharLimit = opts.Evolution.SoulCharLimit
		cfg.PromptCharLimit = opts.Evolution.PromptCharLimit
	}
	if cfg.Dir == "" {
		cfg.Dir = strings.TrimSpace(opts.ProjectRoot)
		if cfg.Dir != "" {
			cfg.Dir = strings.TrimRight(cfg.Dir, "/\\") + "/.agents/evolution"
		}
	}
	return cfg
}

func augmentSystemPromptWithEvolution(base string, snap evolution.Snapshot) string {
	out := strings.TrimSpace(base)
	if snap.Soul != "" {
		out = joinPromptBlocks(snap.Soul, out)
	}
	if snap.Prompt != "" {
		out = joinPromptBlocks(out, snap.Prompt)
	}
	if snap.Memory != "" {
		out = joinPromptBlocks(out, snap.Memory)
	}
	if snap.User != "" {
		out = joinPromptBlocks(out, snap.User)
	}
	return strings.TrimSpace(out)
}

func joinPromptBlocks(a, b string) string {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	switch {
	case a == "":
		return b
	case b == "":
		return a
	default:
		return a + "\n\n" + b
	}
}
