package api

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stellarlinkco/agentsdk-go/pkg/message"
	"github.com/stellarlinkco/agentsdk-go/pkg/skylark"
)

func TestMemorySearchToolHitsVault(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	vault := filepath.Join(dir, "vault")
	index := filepath.Join(dir, "index")
	if err := os.MkdirAll(vault, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "# Deploy\n\nUse kubectl apply for production rollout.\n"
	if err := os.WriteFile(filepath.Join(vault, "deploy.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	docs, err := skylark.SyncVault(vault)
	if err != nil {
		t.Fatal(err)
	}
	eng, err := skylark.NewEngine(index, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = eng.Close() })
	if err := eng.Rebuild(ctx, docs); err != nil {
		t.Fatal(err)
	}

	tool := &memorySearchTool{engine: eng}
	res, err := tool.Execute(ctx, map[string]any{"query": "kubectl production"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success {
		t.Fatalf("tool failed: %s", res.Output)
	}
	var payload struct {
		Hits []map[string]any `json:"hits"`
	}
	if err := json.Unmarshal([]byte(res.Output), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Hits) == 0 {
		t.Fatal("expected hits")
	}
	path, _ := payload.Hits[0]["path"].(string)
	if path != "deploy.md" {
		t.Fatalf("path: %q", path)
	}
}

func TestSessionSearchToolHitsHistory(t *testing.T) {
	h := message.NewHistory()
	h.Append(message.Message{Role: "user", Content: "remember the deploy checklist"})
	h.Append(message.Message{Role: "assistant", Content: "Step one: run kubectl apply"})

	ctx := withKnowledgeRun(context.Background(), &knowledgeRunBundle{History: h})
	tool := &sessionSearchTool{}
	res, err := tool.Execute(ctx, map[string]any{"query": "kubectl deploy"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success {
		t.Fatalf("tool failed: %s", res.Output)
	}
	if !strings.Contains(res.Output, "kubectl") {
		t.Fatalf("expected kubectl in output: %s", res.Output)
	}
}
