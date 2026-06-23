package model

import (
	"testing"
)

func TestNewAnthropicAllowsMissingAPIKey(t *testing.T) {
	if _, err := NewAnthropic(AnthropicConfig{BaseURL: "http://localhost:11434"}); err != nil {
		t.Fatalf("unexpected error for local model without api key: %v", err)
	}
}

func TestAnthropicHeaders(t *testing.T) {
	t.Setenv("ANTHROPIC_CUSTOM_HEADERS_ENABLED", "true")
	headers := newAnthropicHeaders(map[string]string{"X-Test": "1"}, map[string]string{"x-api-key": "skip"})
	if headers["x-test"] != "1" {
		t.Fatalf("expected x-test header, got %v", headers)
	}
}

func TestAnthropicRequestOptions(t *testing.T) {
	m := &anthropicModel{configuredAPIKey: "key"}
	opts := m.requestOptions()
	if len(opts) == 0 {
		t.Fatalf("expected request options")
	}
}
