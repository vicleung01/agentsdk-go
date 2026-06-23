package a2ui

import "strings"

const defaultPlainTextSurfaceID = "main"

// MessagesFromPlainText builds createSurface + updateComponents for a simple text reply.
func MessagesFromPlainText(text string) []*ServerMessage {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}
	return []*ServerMessage{
		{
			Version: Version,
			CreateSurface: &CreateSurface{
				SurfaceID: defaultPlainTextSurfaceID,
				CatalogID: BasicCatalogID,
			},
		},
		{
			Version: Version,
			UpdateComponents: &UpdateComponents{
				SurfaceID: defaultPlainTextSurfaceID,
				Components: []map[string]any{
					{
						"id":        "root",
						"component": "Column",
						"children":  []any{"body"},
					},
					{
						"id":        "body",
						"component": "Text",
						"text":      trimmed,
					},
				},
			},
		},
	}
}
