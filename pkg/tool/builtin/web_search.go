package toolbuiltin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/stellarlinkco/agentsdk-go/pkg/tool"
)

const webSearchDescription = `Search the web for up-to-date information. Returns titles, URLs, and snippets.
Use web_fetch to read full page content from a result URL.`

var webSearchSchema = &tool.JSONSchema{
	Type: "object",
	Properties: map[string]interface{}{
		"query": map[string]interface{}{
			"type":        "string",
			"description": "Search query",
		},
		"max_results": map[string]interface{}{
			"type":        "integer",
			"description": "Maximum results (1-10)",
		},
	},
	Required: []string{"query"},
}

// WebSearchTool searches the public web (Bing by default, Brave when API key is configured).
type WebSearchTool struct {
	Config *WebToolsConfig
}

// NewWebSearchTool builds a web search tool.
func NewWebSearchTool(cfg *WebToolsConfig) *WebSearchTool {
	return &WebSearchTool{Config: cfg}
}

func (WebSearchTool) Name() string { return "web_search" }

func (WebSearchTool) Description() string { return webSearchDescription }

func (WebSearchTool) Schema() *tool.JSONSchema { return webSearchSchema }

func (t *WebSearchTool) Execute(ctx context.Context, params map[string]interface{}) (*tool.ToolResult, error) {
	query := stringParam(params, "query")
	if query == "" {
		return toolErrorResult("query is required"), nil
	}
	cfg := t.Config
	maxResults := cfg.searchMaxResults()
	if v, ok := params["max_results"].(float64); ok && int(v) > 0 {
		maxResults = int(v)
	}
	if maxResults > 10 {
		maxResults = 10
	}
	var (
		results []searchResult
		err     error
	)
	switch cfg.searchProvider() {
	case "brave":
		results, err = braveWebSearch(ctx, cfg, query, maxResults)
	default:
		results, err = bingWebSearch(ctx, cfg, query, maxResults)
	}
	if err != nil {
		return toolErrorResult(fmt.Sprintf("web search failed: %v", err)), nil
	}
	if len(results) == 0 {
		return &tool.ToolResult{Success: true, Output: fmt.Sprintf("No results for: %s", query)}, nil
	}
	var b strings.Builder
	for i, r := range results {
		b.WriteString(fmt.Sprintf("%d. %s\n   %s\n   %s\n", i+1, r.Title, r.URL, r.Snippet))
	}
	return &tool.ToolResult{Success: true, Output: strings.TrimSpace(b.String())}, nil
}

func bingWebSearch(ctx context.Context, cfg *WebToolsConfig, query string, max int) ([]searchResult, error) {
	q := url.QueryEscape(query)
	reqURL := fmt.Sprintf("https://www.bing.com/search?q=%s&count=%d", q, max)
	body, err := httpGet(ctx, cfg, reqURL)
	if err != nil {
		return nil, err
	}
	html := string(body)
	titleRe := regexp.MustCompile(`(?is)<li[^>]*class="[^"]*b_algo[^"]*"[^>]*>.*?<h2>\s*<a[^>]+href="([^"]+)"[^>]*>(.*?)</a>`)
	snippetRe := regexp.MustCompile(`(?is)<div[^>]*class="[^"]*b_caption[^"]*"[^>]*>.*?<p[^>]*>(.*?)</p>`)
	titles := titleRe.FindAllStringSubmatch(html, max)
	snippets := snippetRe.FindAllStringSubmatch(html, max)
	out := make([]searchResult, 0, len(titles))
	for i, m := range titles {
		if len(m) < 3 {
			continue
		}
		link := htmlUnescape(stripHTMLTags(m[1]))
		title := htmlUnescape(stripHTMLTags(m[2]))
		snippet := ""
		if i < len(snippets) && len(snippets[i]) > 1 {
			snippet = htmlUnescape(stripHTMLTags(snippets[i][1]))
		}
		if link == "" || title == "" {
			continue
		}
		out = append(out, searchResult{Title: title, URL: link, Snippet: snippet})
	}
	if len(out) == 0 {
		imgURL := fmt.Sprintf("https://www.bing.com/images/search?q=%s", q)
		out = append(out, searchResult{
			Title:   "Bing image search",
			URL:     imgURL,
			Snippet: "Open image results and use web_fetch on a direct image URL if needed.",
		})
	}
	return out, nil
}

func braveWebSearch(ctx context.Context, cfg *WebToolsConfig, query string, max int) ([]searchResult, error) {
	apiKey := braveAPIKeyFromEnv(cfg)
	if apiKey == "" {
		return bingWebSearch(ctx, cfg, query, max)
	}
	q := url.QueryEscape(query)
	reqURL := fmt.Sprintf("https://api.search.brave.com/res/v1/web/search?q=%s&count=%d", q, max)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", apiKey)
	client := &http.Client{Timeout: cfg.httpTimeout()}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("brave API %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var parsed struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	out := make([]searchResult, 0, len(parsed.Web.Results))
	for _, r := range parsed.Web.Results {
		if strings.TrimSpace(r.URL) == "" {
			continue
		}
		out = append(out, searchResult{Title: r.Title, URL: r.URL, Snippet: r.Description})
	}
	return out, nil
}
