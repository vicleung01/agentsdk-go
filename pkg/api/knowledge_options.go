package api

import (
	"github.com/tmc/langchaingo/embeddings"
)

const defaultKnowledgeSearchLimit = 8

// KnowledgeOptions configures Obsidian-compatible vault indexing and retrieval tools.
type KnowledgeOptions struct {
	Enabled bool
	// VaultDir is the user-visible markdown vault (Obsidian can open this folder).
	VaultDir string
	// IndexDir stores Bleve index, corpus.json, vectors.json (derived, not for manual edit).
	IndexDir string
	// DisableEmbedding forces Bleve-only search when true or when no embedder is available.
	DisableEmbedding bool
	Embedder         embeddings.Embedder `json:"-"`
}
