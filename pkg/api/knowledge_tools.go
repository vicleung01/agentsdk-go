package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/stellarlinkco/agentsdk-go/pkg/message"
	"github.com/stellarlinkco/agentsdk-go/pkg/skylark"
	"github.com/stellarlinkco/agentsdk-go/pkg/tool"
)

const (
	memorySearchToolName  = "memory_search"
	sessionSearchToolName = "session_search"
)

type memorySearchParams struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

type sessionSearchParams struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

type memorySearchTool struct {
	engine *skylark.Engine
}

func (t *memorySearchTool) Name() string { return memorySearchToolName }

func (t *memorySearchTool) Description() string {
	return `Search the Obsidian-compatible knowledge vault (markdown notes).

REQUIRED: Before answering questions about documentation, runbooks, architecture, processes, project conventions, or any content that may live in the vault, call this tool first. Do not guess from training data or conversation context alone.

Uses hybrid Bleve text search plus optional vector similarity when embeddings are configured. Returns path, line range, snippet, and score.`
}

func (t *memorySearchTool) Schema() *tool.JSONSchema {
	return &tool.JSONSchema{
		Type: "object",
		Properties: map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Natural language query to search vault notes.",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Max hits (default 8).",
			},
		},
		Required: []string{"query"},
	}
}

func (t *memorySearchTool) Execute(ctx context.Context, params map[string]any) (*tool.ToolResult, error) {
	if t == nil || t.engine == nil {
		return nil, errors.New("knowledge: engine not initialised")
	}
	raw, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	var p memorySearchParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	q := strings.TrimSpace(p.Query)
	if q == "" {
		return nil, errors.New("query is required")
	}
	limit := p.Limit
	if limit <= 0 {
		limit = defaultSearchLimit(ctx)
	}

	kinds := map[string]struct{}{skylark.KindDocument: {}}
	hits, err := t.engine.SearchIndex(ctx, q, kinds, limit)
	if err != nil {
		return nil, err
	}
	out, err := json.MarshalIndent(map[string]any{"hits": formatKnowledgeHits(hits)}, "", "  ")
	if err != nil {
		return nil, err
	}
	return &tool.ToolResult{Success: true, Output: string(out)}, nil
}

type sessionSearchTool struct{}

func (t *sessionSearchTool) Name() string { return sessionSearchToolName }

func (t *sessionSearchTool) Description() string {
	return `Search the current conversation history for relevant prior turns. Use when the user refers to earlier messages in this session.`
}

func (t *sessionSearchTool) Schema() *tool.JSONSchema {
	return &tool.JSONSchema{
		Type: "object",
		Properties: map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Natural language query to match against prior conversation turns.",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Max hits (default 8).",
			},
		},
		Required: []string{"query"},
	}
}

func (t *sessionSearchTool) Execute(ctx context.Context, params map[string]any) (*tool.ToolResult, error) {
	raw, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	var p sessionSearchParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	q := strings.TrimSpace(p.Query)
	if q == "" {
		return nil, errors.New("query is required")
	}
	limit := p.Limit
	if limit <= 0 {
		limit = defaultSearchLimit(ctx)
	}

	b := knowledgeRunFromContext(ctx)
	if b == nil || b.History == nil {
		out, _ := json.MarshalIndent(map[string]any{"hits": []any{}}, "", "  ")
		return &tool.ToolResult{Success: true, Output: string(out)}, nil
	}

	turns := historyToTurns(b.History)
	hits := skylark.SearchHistory(q, turns, limit)
	out, err := json.MarshalIndent(map[string]any{"hits": formatKnowledgeHits(hits)}, "", "  ")
	if err != nil {
		return nil, err
	}
	return &tool.ToolResult{Success: true, Output: string(out)}, nil
}

func defaultSearchLimit(ctx context.Context) int {
	if b := knowledgeRunFromContext(ctx); b != nil && b.DefaultSearchLimit > 0 {
		return b.DefaultSearchLimit
	}
	return defaultKnowledgeSearchLimit
}

func formatKnowledgeHits(hits []skylark.Hit) []map[string]any {
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].ID < hits[j].ID
	})
	out := make([]map[string]any, 0, len(hits))
	for _, h := range hits {
		item := map[string]any{
			"id":      h.ID,
			"kind":    h.Kind,
			"title":   h.Title,
			"snippet": h.Snippet,
			"score":   h.Score,
		}
		if h.Meta != nil {
			if p := h.Meta["path"]; p != "" {
				item["path"] = p
			}
			if s := h.Meta["start_line"]; s != "" {
				item["start_line"] = parseMetaInt(s)
			}
			if e := h.Meta["end_line"]; e != "" {
				item["end_line"] = parseMetaInt(e)
			}
		}
		if h.Kind == skylark.KindHistory {
			item["path"] = h.ID
		}
		out = append(out, item)
	}
	return out
}

func parseMetaInt(s string) int {
	var n int
	_, _ = fmt.Sscanf(s, "%d", &n)
	return n
}

func historyToTurns(h *message.History) []skylark.HistoryTurn {
	if h == nil {
		return nil
	}
	msgs := h.All()
	out := make([]skylark.HistoryTurn, 0, len(msgs))
	for _, m := range msgs {
		var b strings.Builder
		if strings.TrimSpace(m.Content) != "" {
			b.WriteString(strings.TrimSpace(m.Content))
		}
		for _, tc := range m.ToolCalls {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			fmt.Fprintf(&b, "[tool_call %s]", tc.Name)
		}
		text := strings.TrimSpace(b.String())
		if text == "" {
			continue
		}
		out = append(out, skylark.HistoryTurn{Role: m.Role, Text: text})
	}
	return out
}
