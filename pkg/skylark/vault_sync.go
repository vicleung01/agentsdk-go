package skylark

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultChunkMaxChars = 2400
	DefaultChunkOverlap  = 2
)

type vaultChunk struct {
	startLine, endLine int
	text               string
}

// SyncVault scans vaultDir for markdown files and returns searchable documents.
// Ignores .obsidian/, .trash/, and hidden path segments.
func SyncVault(vaultDir string) ([]Document, error) {
	vaultDir = strings.TrimSpace(vaultDir)
	if vaultDir == "" {
		return nil, nil
	}
	info, err := os.Stat(vaultDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("skylark: vault path is not a directory: %s", vaultDir)
	}

	var docs []Document
	err = filepath.Walk(vaultDir, func(absPath string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if fi.IsDir() {
			base := filepath.Base(absPath)
			if strings.HasPrefix(base, ".") {
				return filepath.SkipDir
			}
			if base == ".obsidian" || base == ".trash" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(absPath), ".md") {
			return nil
		}
		rel, err := filepath.Rel(vaultDir, absPath)
		if err != nil {
			rel = absPath
		}
		rel = filepath.ToSlash(rel)
		if shouldSkipVaultRel(rel) {
			return nil
		}

		content, err := os.ReadFile(absPath)
		if err != nil {
			return nil
		}
		text := string(content)
		fileHash := hashBytes(content)
		title := vaultTitle(rel, text)

		for i, c := range chunkVaultText(text) {
			id := fmt.Sprintf("chunk:%s:%s:%d", rel, fileHash[:8], i)
			meta := map[string]string{
				"path":       rel,
				"start_line": fmt.Sprintf("%d", c.startLine),
				"end_line":   fmt.Sprintf("%d", c.endLine),
			}
			docs = append(docs, Document{
				ID:    id,
				Kind:  KindDocument,
				Title: title,
				Text:  c.text,
				Meta:  meta,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return docs, nil
}

func shouldSkipVaultRel(rel string) bool {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for _, p := range parts {
		if strings.HasPrefix(p, ".") {
			return true
		}
	}
	return false
}

func vaultTitle(relPath, body string) string {
	base := strings.TrimSuffix(filepath.Base(relPath), filepath.Ext(relPath))
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return base
}

func chunkVaultText(text string) []vaultChunk {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return nil
	}
	var chunks []vaultChunk
	start := 0
	for start < len(lines) {
		n := 0
		chars := 0
		for i := start; i < len(lines) && chars < DefaultChunkMaxChars; i++ {
			chars += len(lines[i]) + 1
			n = i - start + 1
		}
		if n == 0 {
			n = 1
		}
		block := strings.Join(lines[start:start+n], "\n")
		chunks = append(chunks, vaultChunk{
			startLine: start + 1,
			endLine:   start + n,
			text:      block,
		})
		start += n
		if DefaultChunkOverlap > 0 && start < len(lines) {
			start -= DefaultChunkOverlap
			if start < 0 {
				start = 0
			}
		}
	}
	return chunks
}

func hashBytes(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
