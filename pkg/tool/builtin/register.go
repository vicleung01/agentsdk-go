package toolbuiltin

import (
	"strings"

	"github.com/stellarlinkco/agentsdk-go/pkg/tool"
)

// BuiltinToolNames returns the default built-in tool registration order.
func BuiltinToolNames() []string {
	return []string{
		"bash",
		"read", "write", "edit", "rollback_last_step",
		"glob", "grep",
		"web_search", "web_fetch",
		"browser",
		"current_time", "get_os_info", "probe_environment",
		"skill",
		"a2ui_push", "a2ui_reset",
	}
}

// WebToolNames lists network tools that may need sandbox network policy.
var WebToolNames = []string{"web_search", "web_fetch"}

// IsWebToolName reports whether name is a built-in web tool.
func IsWebToolName(name string) bool {
	key := strings.ToLower(strings.TrimSpace(name))
	for _, n := range WebToolNames {
		if key == n {
			return true
		}
	}
	return false
}

// NewWebTools returns web_search and web_fetch when enabled in cfg.
func NewWebTools(cfg *WebToolsConfig, projectRoot string) []tool.Tool {
	if cfg == nil {
		cfg = &WebToolsConfig{}
	}
	var out []tool.Tool
	if cfg.IsSearchEnabled() {
		out = append(out, NewWebSearchTool(cfg))
	}
	if cfg.IsFetchEnabled() {
		out = append(out, NewWebFetchTool(cfg, projectRoot))
	}
	return out
}

// NewUtilityTools returns system and utility built-ins (no external deps).
func NewUtilityTools() []tool.Tool {
	return []tool.Tool{
		CurrentTimeTool{},
		OsInfoTool{},
		EnvProbeTool{},
	}
}
