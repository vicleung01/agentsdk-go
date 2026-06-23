package api

import "github.com/stellarlinkco/agentsdk-go/pkg/model"

// EnabledBuiltinToolKeys returns the built-in registration keys selected by
// Options.EnabledBuiltinTools for the effective entrypoint.
func EnabledBuiltinToolKeys(opts Options) []string {
	resolved := opts.withDefaults()
	entry := effectiveEntryPoint(resolved)
	return filterBuiltinNames(resolved.EnabledBuiltinTools, builtinOrder(entry, resolved.BrowserHandler, resolved.WebTools))
}

// AvailableTools returns model-facing tool definitions from the runtime registry.
func (rt *Runtime) AvailableTools() []model.ToolDefinition {
	if rt == nil {
		return nil
	}
	schemaMode := rt.opts.ToolPromptSchema
	if schemaMode == "" {
		schemaMode = ToolPromptSchemaFull
	}
	return availableTools(rt.registry, nil, schemaMode)
}

// AvailableToolsForWhitelist returns model-facing tool definitions constrained by whitelist.
func (rt *Runtime) AvailableToolsForWhitelist(toolWhitelist []string) []model.ToolDefinition {
	if rt == nil {
		return nil
	}
	schemaMode := rt.opts.ToolPromptSchema
	if schemaMode == "" {
		schemaMode = ToolPromptSchemaFull
	}
	return availableTools(rt.registry, toLowerSet(toolWhitelist), schemaMode)
}
