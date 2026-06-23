package toolbuiltin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/stellarlinkco/agentsdk-go/pkg/tool"
)

const webFetchDescription = `Fetch content from HTTP(S) URLs or local sandbox files (relative paths, absolute paths, file://).
Extracts readable text from HTML pages, articles, and documentation.`

var webFetchSchema = &tool.JSONSchema{
	Type: "object",
	Properties: map[string]interface{}{
		"url": map[string]interface{}{
			"type":        "string",
			"description": "HTTP/HTTPS URL, or a local sandbox file path (e.g. docs/README.md, file:///path)",
		},
	},
	Required: []string{"url"},
}

// WebFetchTool fetches a URL and returns clean text or metadata for images.
type WebFetchTool struct {
	Config      *WebToolsConfig
	ProjectRoot string
}

// NewWebFetchTool builds a web fetch tool rooted at projectRoot.
func NewWebFetchTool(cfg *WebToolsConfig, projectRoot string) *WebFetchTool {
	root := strings.TrimSpace(projectRoot)
	if root == "" {
		root = "."
	}
	return &WebFetchTool{Config: cfg, ProjectRoot: root}
}

func (WebFetchTool) Name() string { return "web_fetch" }

func (WebFetchTool) Description() string { return webFetchDescription }

func (WebFetchTool) Schema() *tool.JSONSchema { return webFetchSchema }

func (t *WebFetchTool) Execute(ctx context.Context, params map[string]interface{}) (*tool.ToolResult, error) {
	rawURL := stringParam(params, "url")
	if rawURL == "" {
		return toolErrorResult("url is required"), nil
	}
	if looksLikeLocalResource(rawURL) {
		return t.fetchLocal(ctx, rawURL)
	}
	body, contentType, finalURL, err := httpGetWithMeta(ctx, t.Config, rawURL)
	if err != nil {
		return toolErrorResult(fmt.Sprintf("fetch failed: %v", err)), nil
	}
	ct := strings.ToLower(contentType)
	if strings.HasPrefix(ct, "image/") {
		return &tool.ToolResult{
			Success: true,
			Output:  fmt.Sprintf("Image URL: %s\nContent-Type: %s\nSize: %d bytes", finalURL, contentType, len(body)),
		}, nil
	}
	text := extractReadableText(string(body), t.Config.fetchMaxChars())
	return &tool.ToolResult{
		Success: true,
		Output:  fmt.Sprintf("URL: %s\nContent-Type: %s\n\n%s", finalURL, contentType, text),
	}, nil
}

func (t *WebFetchTool) fetchLocal(ctx context.Context, rawURL string) (*tool.ToolResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	body, contentType, displayPath, err := readLocalSandboxFile(t.ProjectRoot, rawURL)
	if err != nil {
		return toolErrorResult(fmt.Sprintf("fetch failed: %v", err)), nil
	}
	ct := strings.ToLower(contentType)
	if strings.HasPrefix(ct, "image/") {
		return &tool.ToolResult{
			Success: true,
			Output:  fmt.Sprintf("Local file: %s\nContent-Type: %s\nSize: %d bytes", displayPath, contentType, len(body)),
		}, nil
	}
	text := string(body)
	ext := strings.ToLower(filepath.Ext(displayPath))
	if strings.HasPrefix(ct, "text/") || ext == ".json" || ext == ".md" || ext == ".markdown" || ext == ".txt" || ext == ".html" || ext == ".htm" {
		text = extractReadableText(text, t.Config.fetchMaxChars())
	}
	return &tool.ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Local file: %s\nContent-Type: %s\n\n%s", displayPath, contentType, text),
	}, nil
}

func looksLikeLocalResource(raw string) bool {
	s := strings.TrimSpace(raw)
	if s == "" {
		return false
	}
	if strings.HasPrefix(s, "file://") {
		return true
	}
	if strings.HasPrefix(s, "/") || strings.HasPrefix(s, "./") || strings.HasPrefix(s, "../") {
		return true
	}
	if len(s) >= 2 && s[1] == ':' && ((s[0] >= 'a' && s[0] <= 'z') || (s[0] >= 'A' && s[0] <= 'Z')) {
		return true
	}
	return false
}

func readLocalSandboxFile(root, rawURL string) ([]byte, string, string, error) {
	s := strings.TrimSpace(rawURL)
	if strings.HasPrefix(s, "file://") {
		s = strings.TrimPrefix(s, "file://")
	}
	candidate := s
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(root, candidate)
	}
	candidate = filepath.Clean(candidate)
	rootClean := filepath.Clean(root)
	rel, err := filepath.Rel(rootClean, candidate)
	if err != nil || strings.HasPrefix(rel, "..") {
		return nil, "", "", fmt.Errorf("path %q is outside sandbox root", rawURL)
	}
	info, err := os.Stat(candidate)
	if err != nil {
		return nil, "", "", err
	}
	if info.IsDir() {
		return nil, "", "", fmt.Errorf("%s is a directory", candidate)
	}
	body, err := os.ReadFile(candidate)
	if err != nil {
		return nil, "", "", err
	}
	ct := detectContentType(candidate, body)
	return body, ct, candidate, nil
}

func detectContentType(path string, body []byte) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".html", ".htm":
		return "text/html"
	case ".md", ".markdown":
		return "text/markdown"
	case ".json":
		return "application/json"
	case ".txt":
		return "text/plain"
	}
	if len(body) > 0 {
		sample := body
		if len(sample) > 512 {
			sample = sample[:512]
		}
		lower := strings.ToLower(string(sample))
		if strings.Contains(lower, "<html") || strings.Contains(lower, "<!doctype") {
			return "text/html"
		}
	}
	return "application/octet-stream"
}
