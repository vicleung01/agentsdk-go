package a2ui

import (
	"fmt"
	"strings"
)

var layoutComponentTypes = map[string]bool{
	"Column": true,
	"Row":    true,
	"Modal":  true,
	"Tabs":   true,
	"Card":   true,
	"List":   true,
}

// NormalizeServerMessage applies per-message fixes without batch-only injections.
func NormalizeServerMessage(msg *ServerMessage) *ServerMessage {
	if msg == nil {
		return nil
	}
	n := msg.Normalize()
	if n.UpdateComponents != nil {
		repairUpdateComponents(n.UpdateComponents)
	}
	return n
}

// RepairMessages normalizes agent output so the Lit client can render surfaces.
func RepairMessages(msgs []*ServerMessage) []*ServerMessage {
	if len(msgs) == 0 {
		return msgs
	}

	coalesced := coalesceMessages(msgs)
	coalesced = ensureCreateSurfaceMessages(coalesced)
	out := make([]*ServerMessage, 0, len(coalesced)+2)
	created := map[string]bool{}
	hasRoot := map[string]bool{}

	for _, msg := range coalesced {
		if msg == nil {
			continue
		}
		n := NormalizeServerMessage(msg)
		if n.CreateSurface != nil {
			sid := strings.TrimSpace(n.CreateSurface.SurfaceID)
			if sid != "" {
				created[sid] = true
			}
		}
		if n.UpdateComponents != nil {
			sid := strings.TrimSpace(n.UpdateComponents.SurfaceID)
			repairUpdateComponents(n.UpdateComponents)
			if sid != "" && updateComponentsHasRoot(n.UpdateComponents) {
				hasRoot[sid] = true
			}
		}
		out = append(out, n)
	}

	for sid := range created {
		if hasRoot[sid] {
			continue
		}
		out = append(out, defaultRootUpdateMessage(sid))
	}
	return out
}

func coalesceMessages(msgs []*ServerMessage) []*ServerMessage {
	var withoutUC []*ServerMessage
	ucParts := map[string][]map[string]any{}

	for _, msg := range msgs {
		if msg == nil {
			continue
		}
		n := msg.Normalize()
		if n.UpdateComponents != nil {
			sid := strings.TrimSpace(n.UpdateComponents.SurfaceID)
			if sid != "" {
				ucParts[sid] = append(ucParts[sid], componentsArrayFromRaw(n.UpdateComponents.Components)...)
			}
			continue
		}
		withoutUC = append(withoutUC, n)
	}
	if len(ucParts) == 0 {
		return msgs
	}

	out := make([]*ServerMessage, 0, len(withoutUC)+len(ucParts))
	inserted := map[string]bool{}
	for _, msg := range withoutUC {
		out = append(out, msg)
		if msg.CreateSurface != nil {
			sid := strings.TrimSpace(msg.CreateSurface.SurfaceID)
			if sid != "" && ucParts[sid] != nil && !inserted[sid] {
				inserted[sid] = true
				out = append(out, mergedUpdateMessage(sid, ucParts[sid]))
			}
		}
	}
	for sid, parts := range ucParts {
		if inserted[sid] {
			continue
		}
		out = append(out, mergedUpdateMessage(sid, parts))
	}
	return out
}

func mergedUpdateMessage(surfaceID string, raw []map[string]any) *ServerMessage {
	uc := &UpdateComponents{
		SurfaceID:  surfaceID,
		Components: repairComponentList(raw),
	}
	return (&ServerMessage{
		Version:          Version,
		UpdateComponents: uc,
	}).Normalize()
}

func defaultRootUpdateMessage(surfaceID string) *ServerMessage {
	return (&ServerMessage{
		Version: Version,
		UpdateComponents: &UpdateComponents{
			SurfaceID: surfaceID,
			Components: []map[string]any{
				{
					"id":        "root",
					"component": "Text",
					"text":      "",
				},
			},
		},
	}).Normalize()
}

func updateComponentsHasRoot(uc *UpdateComponents) bool {
	for _, comp := range uc.Components {
		if id, ok := comp["id"].(string); ok && id == "root" {
			return true
		}
	}
	return false
}

func repairUpdateComponents(uc *UpdateComponents) {
	if uc == nil {
		return
	}
	uc.Components = repairComponentList(uc.Components)
}

func repairComponentList(raw []map[string]any) []map[string]any {
	components := componentsArrayFromRaw(raw)
	if len(components) == 0 {
		return components
	}
	components = flattenInlineComponents(components)
	components = repairButtonLabels(components)
	components = stripCommandConfirmationButtons(components)
	components = repairTextComponents(components)
	components = synthesizeMissingReferencedComponents(ensureRootComponent(components))
	components = wrapTrailingButtonsInRow(components)
	return components
}

func normalizeButtonAction(action map[string]any) map[string]any {
	if action == nil {
		return nil
	}
	if event, ok := action["event"].(map[string]any); ok && event != nil {
		return action
	}
	if name, ok := action["name"].(string); ok && strings.TrimSpace(name) != "" {
		out := map[string]any{
			"event": map[string]any{"name": strings.TrimSpace(name)},
		}
		if ctx, ok := action["context"].(map[string]any); ok && ctx != nil {
			out["event"].(map[string]any)["context"] = ctx
		}
		return out
	}
	return action
}

func wrapTrailingButtonsInRow(components []map[string]any) []map[string]any {
	var root map[string]any
	rootIdx := -1
	for i, comp := range components {
		if id, _ := comp["id"].(string); id == "root" {
			root = comp
			rootIdx = i
			break
		}
	}
	if root == nil {
		return components
	}
	childRaw, ok := anySliceFromRaw(root["children"])
	if !ok {
		return components
	}
	childIDs := make([]string, 0, len(childRaw))
	for _, item := range childRaw {
		if id, ok := item.(string); ok && id != "" {
			childIDs = append(childIDs, id)
		}
	}
	byID := map[string]map[string]any{}
	for _, comp := range components {
		if id, ok := comp["id"].(string); ok && id != "" {
			byID[id] = comp
		}
	}
	isButton := func(id string) bool {
		comp := byID[id]
		if comp == nil {
			return false
		}
		typeName, _ := comp["component"].(string)
		return typeName == "Button"
	}
	start := len(childIDs)
	for start > 0 && isButton(childIDs[start-1]) {
		start--
	}
	buttonIDs := childIDs[start:]
	if len(buttonIDs) < 2 {
		return components
	}
	rowID := "_actions_row"
	nextChildren := append(append([]string{}, childIDs[:start]...), rowID)
	updatedRoot := cloneComponentMap(root)
	updatedRoot["children"] = nextChildren
	out := make([]map[string]any, len(components))
	copy(out, components)
	out[rootIdx] = updatedRoot
	out = append(out, map[string]any{
		"id":        rowID,
		"component": "Row",
		"children":  buttonIDs,
		"justify":   "end",
		"align":     "center",
	})
	return out
}

func repairButtonLabels(components []map[string]any) []map[string]any {
	out := make([]map[string]any, len(components))
	for i, comp := range components {
		out[i] = cloneComponentMap(comp)
	}
	ids := map[string]bool{}
	for _, comp := range out {
		if id, ok := comp["id"].(string); ok && id != "" {
			ids[id] = true
		}
	}
	for _, comp := range out {
		typeName, _ := comp["component"].(string)
		if typeName != "Button" {
			continue
		}
		if child, ok := comp["child"].(string); ok && child != "" {
			continue
		}
		btnID, _ := comp["id"].(string)
		if btnID == "" {
			btnID = "btn"
		}
		labelText := btnID
		if label, ok := comp["label"].(string); ok && label != "" {
			labelText = label
		} else if text, ok := comp["text"].(string); ok && text != "" {
			labelText = text
		}
		labelID := btnID + "_label"
		if !ids[labelID] {
			out = append(out, map[string]any{
				"id":        labelID,
				"component": "Text",
				"text":      labelText,
			})
			ids[labelID] = true
		}
		comp["child"] = labelID
		delete(comp, "label")
		if action, ok := comp["action"].(map[string]any); ok {
			comp["action"] = normalizeButtonAction(action)
		}
	}
	return out
}

func ensureCreateSurfaceMessages(msgs []*ServerMessage) []*ServerMessage {
	haveCreate := map[string]bool{}
	needSurface := map[string]bool{}
	for _, msg := range msgs {
		if msg == nil {
			continue
		}
		if msg.CreateSurface != nil {
			if sid := strings.TrimSpace(msg.CreateSurface.SurfaceID); sid != "" {
				haveCreate[sid] = true
			}
		}
		if msg.UpdateComponents != nil {
			if sid := strings.TrimSpace(msg.UpdateComponents.SurfaceID); sid != "" {
				needSurface[sid] = true
			}
		}
		if msg.UpdateDataModel != nil {
			if sid := strings.TrimSpace(msg.UpdateDataModel.SurfaceID); sid != "" {
				needSurface[sid] = true
			}
		}
	}
	var prefix []*ServerMessage
	for sid := range needSurface {
		if haveCreate[sid] {
			continue
		}
		haveCreate[sid] = true
		prefix = append(prefix, (&ServerMessage{
			Version: Version,
			CreateSurface: &CreateSurface{
				SurfaceID: sid,
				CatalogID: BasicCatalogID,
			},
		}).Normalize())
	}
	if len(prefix) == 0 {
		return msgs
	}
	return append(prefix, msgs...)
}

func componentsArrayFromRaw(raw []map[string]any) []map[string]any {
	if len(raw) == 0 {
		return nil
	}
	out := make([]map[string]any, len(raw))
	for i, comp := range raw {
		out[i] = cloneComponentMap(comp)
	}
	return out
}

func flattenInlineComponents(components []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(components))
	byID := map[string]map[string]any{}

	var ingest func(comp map[string]any)
	ingest = func(comp map[string]any) {
		normalized := normalizeComponentMap(comp)
		id, _ := normalized["id"].(string)
		if id == "" {
			out = append(out, normalized)
			return
		}

		if rawChildren, ok := anySliceFromRaw(normalized["children"]); ok {
			childIDs := make([]any, 0, len(rawChildren))
			for _, item := range rawChildren {
				if childID, ok := item.(string); ok && childID != "" {
					childIDs = append(childIDs, childID)
					continue
				}
				inline, ok := item.(map[string]any)
				if !ok {
					continue
				}
				inlineNorm := normalizeComponentMap(inline)
				inlineID, _ := inlineNorm["id"].(string)
				if inlineID == "" {
					inlineID = fmt.Sprintf("_inline_%s_%d", id, len(childIDs))
					inlineNorm["id"] = inlineID
				}
				ingest(inlineNorm)
				childIDs = append(childIDs, inlineID)
			}
			normalized["children"] = childIDs
		}

		if existing, ok := byID[id]; ok {
			for k, v := range normalized {
				existing[k] = v
			}
			return
		}
		byID[id] = normalized
		out = append(out, normalized)
	}

	for _, comp := range components {
		ingest(comp)
	}
	return out
}

func ensureRootComponent(components []map[string]any) []map[string]any {
	if updateComponentsHasRoot(&UpdateComponents{Components: components}) {
		return components
	}
	if len(components) == 0 {
		return components
	}

	referenced := collectReferencedComponentIDs(components)
	topLevel := topLevelComponentIDs(components, referenced)

	if len(topLevel) == 1 {
		for i, comp := range components {
			id, _ := comp["id"].(string)
			if id != topLevel[0] {
				continue
			}
			if typeName, _ := comp["component"].(string); layoutComponentTypes[typeName] {
				updated := cloneComponentMap(comp)
				updated["id"] = "root"
				components[i] = updated
				return components
			}
		}
	}

	children := topLevel
	if len(children) == 0 {
		children = allComponentIDs(components)
	}

	root := map[string]any{
		"id":        "root",
		"component": "Column",
		"children":  children,
	}
	return append([]map[string]any{root}, components...)
}

func collectReferencedComponentIDs(components []map[string]any) map[string]bool {
	referenced := make(map[string]bool)
	for _, comp := range components {
		addReferencedIDs(referenced, comp["children"])
		if child, ok := comp["child"].(string); ok && child != "" {
			referenced[child] = true
		}
		for _, field := range []string{"trigger", "content"} {
			if id, ok := comp[field].(string); ok && id != "" {
				referenced[id] = true
			}
		}
		addTabsChildRefs(referenced, comp["tabs"])
		if childList, ok := comp["children"].(map[string]any); ok {
			if templateID, ok := childList["componentId"].(string); ok && templateID != "" {
				referenced[templateID] = true
			}
		}
	}
	return referenced
}

func addTabsChildRefs(referenced map[string]bool, raw any) {
	items, ok := raw.([]any)
	if !ok {
		return
	}
	for _, item := range items {
		tab, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if child, ok := tab["child"].(string); ok && child != "" {
			referenced[child] = true
		}
	}
}

func synthesizeMissingReferencedComponents(components []map[string]any) []map[string]any {
	current := components
	for pass := 0; pass < 4; pass++ {
		defined := map[string]bool{}
		for _, comp := range current {
			if id, ok := comp["id"].(string); ok && id != "" {
				defined[id] = true
			}
		}
		referenced := collectReferencedComponentIDs(current)
		var placeholders []map[string]any
		for id := range referenced {
			if defined[id] {
				continue
			}
			for _, ph := range placeholderComponents(id) {
				phID, _ := ph["id"].(string)
				if phID != "" && !defined[phID] {
					defined[phID] = true
					placeholders = append(placeholders, ph)
				}
			}
		}
		if len(placeholders) == 0 {
			return current
		}
		current = append(current, placeholders...)
	}
	return current
}

func placeholderComponents(id string) []map[string]any {
	lower := strings.ToLower(id)
	if strings.Contains(lower, "btn") || strings.Contains(lower, "button") {
		labelID := id + "_label"
		return []map[string]any{
			{"id": labelID, "component": "Text", "text": id},
			{
				"id":        id,
				"component": "Button",
				"child":     labelID,
				"action":    map[string]any{"event": map[string]any{"name": "noop"}},
			},
		}
	}
	return []map[string]any{
		{"id": id, "component": "Text", "text": ""},
	}
}

func addReferencedIDs(referenced map[string]bool, raw any) {
	if items, ok := anySliceFromRaw(raw); ok {
		for _, item := range items {
			if id, ok := item.(string); ok && id != "" {
				referenced[id] = true
			}
		}
		return
	}
	switch v := raw.(type) {
	case string:
		if v != "" {
			referenced[v] = true
		}
	}
}

// anySliceFromRaw accepts []any, []string, or {"item":[...]} array wrappers from some tool serializers.
func anySliceFromRaw(raw any) ([]any, bool) {
	if raw == nil {
		return nil, false
	}
	switch v := raw.(type) {
	case []any:
		return v, true
	case []string:
		out := make([]any, len(v))
		for i, s := range v {
			out[i] = s
		}
		return out, true
	case map[string]any:
		if items, ok := v["item"].([]any); ok {
			return items, true
		}
	}
	return nil, false
}

func topLevelComponentIDs(components []map[string]any, referenced map[string]bool) []string {
	var topLevel []string
	seen := map[string]bool{}
	for _, comp := range components {
		id, ok := comp["id"].(string)
		if !ok || id == "" || seen[id] {
			continue
		}
		seen[id] = true
		if !referenced[id] {
			topLevel = append(topLevel, id)
		}
	}
	return topLevel
}

func allComponentIDs(components []map[string]any) []string {
	var ids []string
	for _, comp := range components {
		if id, ok := comp["id"].(string); ok && id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func cloneComponentMap(comp map[string]any) map[string]any {
	out := make(map[string]any, len(comp))
	for k, v := range comp {
		out[k] = v
	}
	return out
}

func normalizeComponentMap(comp map[string]any) map[string]any {
	if comp == nil {
		return map[string]any{"id": "root", "component": "Text", "text": ""}
	}
	out := cloneComponentMap(comp)

	switch c := comp["component"].(type) {
	case string:
	case map[string]any:
		for typeName, props := range c {
			out["component"] = typeName
			if propsMap, ok := props.(map[string]any); ok {
				for pk, pv := range propsMap {
					if _, exists := out[pk]; !exists {
						out[pk] = pv
					}
				}
			}
			break
		}
	}

	if _, ok := out["component"]; !ok {
		out["component"] = "Text"
	}
	return out
}
