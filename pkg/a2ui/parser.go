package a2ui

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
)

// ParseLine parses a single JSON line into a ServerMessage.
func ParseLine(line string) (*ServerMessage, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, nil
	}
	var msg ServerMessage
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		return nil, err
	}
	n := msg.Normalize()
	if err := n.Validate(); err != nil {
		return nil, err
	}
	return n, nil
}

// ParseJSONL parses a JSONL blob into validated messages.
func ParseJSONL(raw string) ([]*ServerMessage, error) {
	scanner := bufio.NewScanner(strings.NewReader(raw))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var out []*ServerMessage
	for scanner.Scan() {
		msg, err := ParseLine(scanner.Text())
		if err != nil {
			return nil, err
		}
		if msg != nil {
			out = append(out, msg)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// LineBuffer accumulates streaming text and emits complete JSONL lines.
type LineBuffer struct {
	buf bytes.Buffer
}

// Push appends text and returns any complete lines parsed as messages.
func (lb *LineBuffer) Push(text string) ([]*ServerMessage, error) {
	if text == "" {
		return nil, nil
	}
	lb.buf.WriteString(text)
	var out []*ServerMessage
	for {
		raw := lb.buf.String()
		idx := strings.IndexByte(raw, '\n')
		if idx < 0 {
			break
		}
		line := raw[:idx]
		rest := raw[idx+1:]
		lb.buf.Reset()
		lb.buf.WriteString(rest)
		msg, err := ParseLine(line)
		if err != nil {
			// Incomplete JSON spanning lines — put line back and wait for more data.
			lb.buf.Reset()
			lb.buf.WriteString(raw)
			break
		}
		if msg != nil {
			out = append(out, msg)
		}
	}
	return out, nil
}

// Flush parses any remaining buffered content as a final message.
func (lb *LineBuffer) Flush() ([]*ServerMessage, error) {
	raw := strings.TrimSpace(lb.buf.String())
	lb.buf.Reset()
	if raw == "" {
		return nil, nil
	}
	msg, err := ParseLine(raw)
	if err != nil {
		return nil, err
	}
	if msg == nil {
		return nil, nil
	}
	return []*ServerMessage{msg}, nil
}

// ExtractFromFencedBlock pulls JSONL from markdown code fences tagged a2ui/json/jsonl.
func ExtractFromFencedBlock(text string) []string {
	var lines []string
	lower := strings.ToLower(text)
	start := 0
	for {
		i := strings.Index(lower[start:], "```")
		if i < 0 {
			break
		}
		i += start
		rest := text[i+3:]
		langEnd := strings.IndexByte(rest, '\n')
		if langEnd < 0 {
			break
		}
		lang := strings.TrimSpace(strings.ToLower(rest[:langEnd]))
		bodyStart := i + 3 + langEnd + 1
		endRel := strings.Index(text[bodyStart:], "```")
		if endRel < 0 {
			break
		}
		body := text[bodyStart : bodyStart+endRel]
		if lang == "a2ui" || lang == "jsonl" || lang == "json" || strings.HasPrefix(lang, "a2ui") {
			for _, line := range strings.Split(body, "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					lines = append(lines, line)
				}
			}
		}
		start = bodyStart + endRel + 3
	}
	return lines
}

// ParseMessages accepts JSON array, JSONL string, or []any from tool params.
func ParseMessages(raw any) ([]*ServerMessage, error) {
	switch v := raw.(type) {
	case nil:
		return nil, nil
	case string:
		v = strings.TrimSpace(v)
		if v == "" {
			return nil, nil
		}
		if strings.HasPrefix(v, "[") {
			var arr []json.RawMessage
			if err := json.Unmarshal([]byte(v), &arr); err != nil {
				return ParseJSONL(v)
			}
			var out []*ServerMessage
			for _, item := range arr {
				msg, err := ParseLine(string(item))
				if err != nil {
					return nil, err
				}
				if msg != nil {
					out = append(out, msg)
				}
			}
			return out, nil
		}
		return ParseJSONL(v)
	case []any:
		var out []*ServerMessage
		for _, item := range v {
			b, err := json.Marshal(item)
			if err != nil {
				return nil, err
			}
			msg, err := ParseLine(string(b))
			if err != nil {
				return nil, err
			}
			if msg != nil {
				out = append(out, msg)
			}
		}
		return out, nil
	case map[string]any:
		if items, ok := unwrapItemArray(v); ok {
			return ParseMessages(items)
		}
		if hasA2UIActionKey(v) {
			return messagesFromActionMap(v)
		}
		b, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		return ParseMessages(string(b))
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		return ParseMessages(string(b))
	}
}

// unwrapItemArray detects protobuf/JSON wrappers that encode arrays as {"item":[...]}.
func unwrapItemArray(v map[string]any) ([]any, bool) {
	items, ok := v["item"]
	if !ok || items == nil {
		return nil, false
	}
	arr, ok := items.([]any)
	if !ok || len(v) != 1 {
		return nil, false
	}
	return arr, true
}
