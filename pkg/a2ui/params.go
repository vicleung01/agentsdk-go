package a2ui

import (
	"encoding/json"
	"strings"
)

var a2uiActionKeys = []string{
	"createSurface",
	"updateComponents",
	"updateDataModel",
	"deleteSurface",
	"surfaceUpdate",
	"dataModelUpdate",
	"beginRendering",
}

var a2uiParamWrapperKeys = []string{
	"messages",
	"message",
	"jsonl",
	"jsonlPath",
	"jsonl_path",
	"a2ui_messages",
	"a2uiMessages",
	"a2ui",
	"content",
}

// ExtractMessagesFromToolParams accepts flexible a2ui_push argument shapes:
// wrapped arrays/strings (messages, jsonl, a2ui, …), a single message object,
// or top-level A2UI action keys (createSurface, updateComponents, …).
func ExtractMessagesFromToolParams(params map[string]any) ([]*ServerMessage, error) {
	if len(params) == 0 {
		return nil, nil
	}

	for _, key := range a2uiParamWrapperKeys {
		v, ok := params[key]
		if !ok || v == nil {
			continue
		}
		msgs, err := ParseMessages(v)
		if err != nil {
			return nil, err
		}
		if len(msgs) > 0 {
			return RepairMessages(msgs), nil
		}
	}

	if msgs, err := messagesFromActionMap(params); err != nil {
		return nil, err
	} else if len(msgs) > 0 {
		return RepairMessages(msgs), nil
	}

	return nil, nil
}

func messagesFromActionMap(params map[string]any) ([]*ServerMessage, error) {
	if !hasA2UIActionKey(params) {
		return nil, nil
	}

	var out []*ServerMessage
	for _, key := range a2uiActionKeys {
		v, ok := params[key]
		if !ok || v == nil {
			continue
		}
		msgMap := map[string]any{key: v}
		if ver, ok := params["version"]; ok && ver != nil {
			msgMap["version"] = ver
		}
		b, err := json.Marshal(msgMap)
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
}

func hasA2UIActionKey(params map[string]any) bool {
	for _, key := range a2uiActionKeys {
		if v, ok := params[key]; ok && v != nil {
			return true
		}
	}
	return false
}

// IsLikelyA2UIMessage reports whether raw JSON looks like an A2UI server message.
func IsLikelyA2UIMessage(raw json.RawMessage) bool {
	raw = json.RawMessage(strings.TrimSpace(string(raw)))
	if len(raw) == 0 {
		return false
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return false
	}
	for _, key := range a2uiActionKeys {
		if _, ok := obj[key]; ok {
			return true
		}
	}
	return false
}
