package toolbuiltin

import (
	"strings"
	"time"
)

const (
	defaultWebUserAgent     = "Mozilla/5.0 (compatible; AgentSDK/1.0)"
	defaultSearchMaxResults = 5
	defaultFetchMaxChars    = 12000
	defaultHTTPTimeout      = 25 * time.Second
)

// WebToolsConfig controls built-in web_search and web_fetch tools.
type WebToolsConfig struct {
	// SearchEnabled gates web_search. Default true when nil.
	SearchEnabled *bool
	// FetchEnabled gates web_fetch. Default true when nil.
	FetchEnabled *bool
	// SearchProvider is "bing" (default) or "brave".
	SearchProvider string
	// SearchAPIKey is the Brave Search API key (falls back to BRAVE_API_KEY env).
	SearchAPIKey string
	// MaxResults caps search results (default 5, max 10).
	MaxResults int
	// FetchMaxChars caps extracted text from HTML (default 12000).
	FetchMaxChars int
	// Timeout for HTTP requests.
	Timeout time.Duration
	// UserAgent for fetch requests.
	UserAgent string
}

// IsSearchEnabled reports whether web_search is enabled.
func (c *WebToolsConfig) IsSearchEnabled() bool {
	if c == nil || c.SearchEnabled == nil {
		return true
	}
	return *c.SearchEnabled
}

// IsFetchEnabled reports whether web_fetch is enabled.
func (c *WebToolsConfig) IsFetchEnabled() bool {
	if c == nil || c.FetchEnabled == nil {
		return true
	}
	return *c.FetchEnabled
}

func (c *WebToolsConfig) searchProvider() string {
	if c != nil {
		if p := strings.TrimSpace(c.SearchProvider); p != "" {
			return strings.ToLower(p)
		}
	}
	return "bing"
}

func (c *WebToolsConfig) searchMaxResults() int {
	if c != nil && c.MaxResults > 0 {
		return c.MaxResults
	}
	return defaultSearchMaxResults
}

func (c *WebToolsConfig) fetchMaxChars() int {
	if c != nil && c.FetchMaxChars > 0 {
		return c.FetchMaxChars
	}
	return defaultFetchMaxChars
}

func (c *WebToolsConfig) httpTimeout() time.Duration {
	if c != nil && c.Timeout > 0 {
		return c.Timeout
	}
	return defaultHTTPTimeout
}

func (c *WebToolsConfig) userAgent() string {
	if c != nil {
		if ua := strings.TrimSpace(c.UserAgent); ua != "" {
			return ua
		}
	}
	return defaultWebUserAgent
}

func (c *WebToolsConfig) braveAPIKey() string {
	if c != nil {
		if k := strings.TrimSpace(c.SearchAPIKey); k != "" {
			return k
		}
	}
	return ""
}
