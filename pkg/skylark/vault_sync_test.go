package skylark

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncVaultIgnoresObsidianAndChunks(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".obsidian"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".obsidian", "config.md"), []byte("hidden"), 0o644); err != nil {
		t.Fatal(err)
	}
	note := `# My Note

First paragraph line one.
Second line still in chunk.

Another section after blank line.
`
	if err := os.WriteFile(filepath.Join(dir, "note.md"), []byte(note), 0o644); err != nil {
		t.Fatal(err)
	}

	docs, err := SyncVault(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) == 0 {
		t.Fatal("expected at least one document chunk")
	}
	found := false
	for _, d := range docs {
		if d.Kind != KindDocument {
			t.Fatalf("kind: %s", d.Kind)
		}
		if d.Meta["path"] != "note.md" {
			continue
		}
		found = true
		if d.Title != "My Note" {
			t.Fatalf("title: %q", d.Title)
		}
		if d.Meta["start_line"] == "" || d.Meta["end_line"] == "" {
			t.Fatalf("missing line meta: %#v", d.Meta)
		}
		if !strings.Contains(d.Text, "First paragraph") {
			t.Fatalf("unexpected text: %q", d.Text)
		}
	}
	if !found {
		t.Fatal("expected chunk for note.md")
	}
}

func TestSyncVaultSkipsHiddenPaths(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".hidden.md"), []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	docs, err := SyncVault(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 0 {
		t.Fatalf("expected no docs, got %d", len(docs))
	}
}

func TestChunkVaultTextOverlap(t *testing.T) {
	var lines []string
	for i := 0; i < 200; i++ {
		lines = append(lines, strings.Repeat("x", 20))
	}
	chunks := chunkVaultText(strings.Join(lines, "\n"))
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	if chunks[1].startLine >= chunks[0].endLine {
		t.Fatalf("expected overlap: first end=%d second start=%d", chunks[0].endLine, chunks[1].startLine)
	}
}
