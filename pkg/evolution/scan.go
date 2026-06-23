package evolution

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var memoryThreatPatterns = []struct {
	re   *regexp.Regexp
	name string
}{
	{regexp.MustCompile(`(?i)ignore\s+(previous|all|above|prior)\s+instructions`), "prompt_injection"},
	{regexp.MustCompile(`(?i)you\s+are\s+now\s+`), "role_hijack"},
	{regexp.MustCompile(`(?i)do\s+not\s+tell\s+the\s+user`), "deception_hide"},
	{regexp.MustCompile(`(?i)system\s+prompt\s+override`), "sys_prompt_override"},
	{regexp.MustCompile(`(?i)disregard\s+(your|all|any)\s+(instructions|rules|guidelines)`), "disregard_rules"},
	{regexp.MustCompile(`(?i)act\s+as\s+(if|though)\s+you\s+(have\s+no|don't\s+have)\s+(restrictions|limits|rules)`), "bypass_restrictions"},
}

var invisibleRunes = map[rune]struct{}{
	'\u200b': {}, '\u200c': {}, '\u200d': {}, '\u2060': {}, '\ufeff': {},
	'\u202a': {}, '\u202b': {}, '\u202c': {}, '\u202d': {}, '\u202e': {},
}

func scanEvolutionContent(content string) string {
	for _, r := range content {
		if _, ok := invisibleRunes[r]; ok {
			return "blocked: content contains invisible unicode (possible injection)"
		}
	}
	for _, p := range memoryThreatPatterns {
		if p.re.MatchString(content) {
			return "blocked: content matches threat pattern " + p.name
		}
	}
	return ""
}

func normalizeEntry(content string) string {
	return strings.TrimSpace(content)
}

func charCount(entries []string) int {
	if len(entries) == 0 {
		return 0
	}
	n := 0
	for i, e := range entries {
		n += utf8.RuneCountInString(e)
		if i < len(entries)-1 {
			n += utf8.RuneCountInString(entryDelimiter)
		}
	}
	return n
}
