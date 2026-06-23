package a2ui

import (
	"strings"
)

const chatInputConfirmationHint = "\n\n请在聊天输入框回复 **确认** 执行，或 **取消** 放弃。"

// stripCommandConfirmationButtons removes bash/command approval buttons; users confirm via chat input.
func stripCommandConfirmationButtons(components []map[string]any) []map[string]any {
	if len(components) == 0 {
		return components
	}
	byID := map[string]map[string]any{}
	for _, comp := range components {
		if id, ok := comp["id"].(string); ok && id != "" {
			byID[id] = comp
		}
	}
	remove := map[string]bool{}
	for _, comp := range components {
		if !isCommandConfirmationButton(comp, byID) {
			continue
		}
		if id, ok := comp["id"].(string); ok && id != "" {
			remove[id] = true
		}
		if child, ok := comp["child"].(string); ok && child != "" {
			remove[child] = true
		}
	}
	if len(remove) == 0 {
		return components
	}
	out := make([]map[string]any, 0, len(components))
	for _, comp := range components {
		id, _ := comp["id"].(string)
		if id != "" && remove[id] {
			continue
		}
		clone := cloneComponentMap(comp)
		pruneRemovedChildRefs(clone, remove)
		out = append(out, clone)
	}
	return appendChatInputConfirmationHint(out)
}

func buttonActionName(action map[string]any) string {
	if action == nil {
		return ""
	}
	if event, ok := action["event"].(map[string]any); ok && event != nil {
		if name, ok := event["name"].(string); ok {
			return strings.TrimSpace(name)
		}
	}
	if name, ok := action["name"].(string); ok {
		return strings.TrimSpace(name)
	}
	return ""
}

func buttonLabelText(comp map[string]any, byID map[string]map[string]any) string {
	if label, ok := comp["label"].(string); ok && strings.TrimSpace(label) != "" {
		return strings.TrimSpace(label)
	}
	if child, ok := comp["child"].(string); ok && child != "" {
		if textComp, ok := byID[child]; ok {
			if text, ok := textComp["text"].(string); ok {
				return strings.TrimSpace(text)
			}
		}
	}
	return ""
}

func isCommandConfirmationButton(comp map[string]any, byID map[string]map[string]any) bool {
	typeName, _ := comp["component"].(string)
	if typeName != "Button" {
		return false
	}
	action, _ := comp["action"].(map[string]any)
	name := strings.ToLower(buttonActionName(action))
	if strings.HasPrefix(name, "confirm") || name == "cancel" || name == "deny" {
		return true
	}
	label := buttonLabelText(comp, byID)
	if label == "" {
		return false
	}
	if strings.Contains(label, "确认") || strings.Contains(label, "取消") {
		return true
	}
	lower := strings.ToLower(label)
	return lower == "confirm" || lower == "cancel" || strings.HasPrefix(lower, "confirm ")
}

func pruneRemovedChildRefs(comp map[string]any, remove map[string]bool) {
	raw := comp["children"]
	list, ok := raw.([]any)
	if !ok || len(list) == 0 {
		return
	}
	next := make([]any, 0, len(list))
	for _, item := range list {
		id, ok := item.(string)
		if !ok || id == "" || remove[id] {
			continue
		}
		next = append(next, id)
	}
	if len(next) == 0 {
		delete(comp, "children")
		return
	}
	comp["children"] = next
}

func appendChatInputConfirmationHint(components []map[string]any) []map[string]any {
	for _, comp := range components {
		text, ok := comp["text"].(string)
		if ok && (strings.Contains(text, "聊天输入框") || strings.Contains(text, "输入框回复")) {
			return components
		}
	}
	targetIdx := -1
	targetScore := -1
	for i, comp := range components {
		typeName, _ := comp["component"].(string)
		if typeName != "Text" {
			continue
		}
		text, ok := comp["text"].(string)
		if !ok || strings.TrimSpace(text) == "" || !looksLikeCommandConfirmationCopy(text) {
			continue
		}
		score := len(text)
		if strings.Contains(text, "命令") || strings.Contains(text, "执行") {
			score += 1000
		}
		if score > targetScore {
			targetScore = score
			targetIdx = i
		}
	}
	if targetIdx < 0 {
		return components
	}
	out := make([]map[string]any, len(components))
	for j, c := range components {
		if j == targetIdx {
			clone := cloneComponentMap(c)
			text, _ := clone["text"].(string)
			clone["text"] = strings.TrimRight(text, "\n") + chatInputConfirmationHint
			out[j] = clone
		} else {
			out[j] = c
		}
	}
	return out
}

func looksLikeCommandConfirmationCopy(text string) bool {
	if strings.Contains(text, "确认") || strings.Contains(text, "安全规则") {
		return true
	}
	lower := strings.ToLower(text)
	return strings.Contains(lower, "bash") ||
		strings.Contains(text, "命令") ||
		strings.Contains(text, "执行 `") ||
		strings.Contains(text, "执行`")
}
