package a2ui

import (
	"regexp"
	"strings"
)

var markdownFenceRE = regexp.MustCompile("(?s)`{3}(text)?\\s*([\\s\\S]*?)\\s*`{3}")
var lsEntryStartRE = regexp.MustCompile(`total \d+|[d-][rwx-]{9,10}@?\s`)
var lsListingHintRE = regexp.MustCompile(`(?:^|\s)total \d+\s|[d-][rwx-]{9,10}@?\s`)
var lsLineRE = regexp.MustCompile(`^(?:total \d+|[d-][rwx-])`)

// repairTextPresentation fixes collapsed command output (e.g. ls) in A2UI Text markdown.
func repairTextPresentation(text string) string {
	if strings.TrimSpace(text) == "" {
		return text
	}
	return repairMarkdownFencedOutput(text)
}

func repairMarkdownFencedOutput(text string) string {
	return markdownFenceRE.ReplaceAllStringFunc(text, func(match string) string {
		parts := markdownFenceRE.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		lang := parts[1]
		body := strings.TrimSpace(parts[2])
		fixed := repairListingBody(body)
		if fixed == body && strings.Contains(match, "\n") {
			return match
		}
		return formatCodeFence(lang, fixed)
	})
}

func formatCodeFence(lang, body string) string {
	if lang == "text" {
		return "```text\n" + body + "\n```"
	}
	if lang != "" {
		return "```" + lang + "\n" + body + "\n```"
	}
	return "```\n" + body + "\n```"
}

func repairListingBody(body string) string {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return body
	}
	if looksLikeMultilineLsListing(trimmed) {
		return trimmed
	}
	if split := splitLsListing(trimmed); split != trimmed {
		return split
	}
	if !strings.Contains(trimmed, "\n") {
		return collapseSpacesToLines(trimmed)
	}
	return trimmed
}

func looksLikeMultilineLsListing(body string) bool {
	lines := strings.Split(body, "\n")
	nonEmpty := 0
	lsLike := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		nonEmpty++
		if lsLineRE.MatchString(line) {
			lsLike++
		}
	}
	return nonEmpty >= 2 && lsLike >= 2
}

func splitLsListing(body string) string {
	trimmed := strings.TrimSpace(body)
	if !lsListingHintRE.MatchString(trimmed) {
		return body
	}
	locs := lsEntryStartRE.FindAllStringIndex(trimmed, -1)
	if len(locs) < 2 {
		return body
	}
	parts := make([]string, 0, len(locs))
	for i, loc := range locs {
		start := loc[0]
		end := len(trimmed)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		part := strings.TrimSpace(trimmed[start:end])
		if part != "" {
			parts = append(parts, part)
		}
	}
	if len(parts) < 2 {
		return body
	}
	return strings.Join(parts, "\n")
}

// collapseSpacesToLines turns single-line ls-style listings into newline-separated lines.
func collapseSpacesToLines(content string) string {
	content = strings.TrimSpace(content)
	if content == "" || strings.Contains(content, "\n") {
		return content
	}
	fields := strings.Fields(content)
	if len(fields) < 4 {
		return content
	}
	if !mostlyPathEntries(fields) {
		return content
	}
	return strings.Join(fields, "\n")
}

func mostlyPathEntries(fields []string) bool {
	if len(fields) < 4 {
		return false
	}
	strong := 0
	weakDirs := 0
	for _, f := range fields {
		if strings.Contains(f, ".") || strings.Contains(f, "-") || strings.Contains(f, "_") {
			strong++
			continue
		}
		if f == "attachments" || f == "prompt" || strings.HasSuffix(f, "-report") {
			weakDirs++
			continue
		}
		for _, r := range f {
			if r > 127 {
				strong++
				break
			}
		}
	}
	if strong >= 2 {
		return true
	}
	return len(fields) >= 6 && strong+weakDirs >= 3
}

func repairTextComponents(components []map[string]any) []map[string]any {
	out := make([]map[string]any, len(components))
	for i, comp := range components {
		out[i] = comp
		typeName, _ := comp["component"].(string)
		if typeName != "Text" {
			continue
		}
		text, ok := comp["text"].(string)
		if !ok || text == "" {
			continue
		}
		repaired := repairTextPresentation(text)
		if repaired == text {
			continue
		}
		clone := cloneComponentMap(comp)
		clone["text"] = repaired
		out[i] = clone
	}
	return out
}
