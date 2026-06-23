package toolbuiltin

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"
)

type searchResult struct {
	Title   string
	URL     string
	Snippet string
}

func httpGet(ctx context.Context, cfg *WebToolsConfig, rawURL string) ([]byte, error) {
	body, _, _, err := httpGetWithMeta(ctx, cfg, rawURL)
	return body, err
}

func httpGetWithMeta(ctx context.Context, cfg *WebToolsConfig, rawURL string) ([]byte, string, string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, "", "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, "", "", fmt.Errorf("only http/https URLs are supported")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", "", err
	}
	req.Header.Set("User-Agent", cfg.userAgent())
	req.Header.Set("Accept", "*/*")
	client := &http.Client{
		Timeout: cfg.httpTimeout(),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, "", "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	const maxBody = 8 << 20
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, "", "", err
	}
	ct := resp.Header.Get("Content-Type")
	if idx := strings.Index(ct, ";"); idx >= 0 {
		ct = strings.TrimSpace(ct[:idx])
	}
	finalURL := resp.Request.URL.String()
	return body, ct, finalURL, nil
}

func extractReadableText(html string, maxChars int) string {
	html = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`).ReplaceAllString(html, " ")
	html = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`).ReplaceAllString(html, " ")
	text := stripHTMLTags(html)
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)
	if maxChars > 0 && utf8.RuneCountInString(text) > maxChars {
		runes := []rune(text)
		text = string(runes[:maxChars]) + "…"
	}
	return text
}

func stripHTMLTags(s string) string {
	re := regexp.MustCompile(`(?s)<[^>]*>`)
	return strings.TrimSpace(re.ReplaceAllString(s, " "))
}

func htmlUnescape(s string) string {
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	return s
}

func braveAPIKeyFromEnv(cfg *WebToolsConfig) string {
	if k := cfg.braveAPIKey(); k != "" {
		return k
	}
	return strings.TrimSpace(os.Getenv("BRAVE_API_KEY"))
}
