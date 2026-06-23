// Package a2ui implements the A2UI (Agent-to-UI) v0.9 server-to-client protocol.
// See https://a2ui.org/specification/v0.9-a2ui/
package a2ui

import (
	"encoding/json"
	"fmt"
)

const Version = "v0.9"

// BasicCatalogID is the canonical catalogId for @a2ui/lit basicCatalog on the client.
const BasicCatalogID = "https://a2ui.org/specification/v0_9/basic_catalog.json"

// ServerMessage is a single A2UI server-to-client message. Exactly one of the
// action keys must be present per message.
type ServerMessage struct {
	Version          string            `json:"version,omitempty"`
	CreateSurface    *CreateSurface    `json:"createSurface,omitempty"`
	UpdateComponents *UpdateComponents `json:"updateComponents,omitempty"`
	UpdateDataModel  *UpdateDataModel  `json:"updateDataModel,omitempty"`
	DeleteSurface    *DeleteSurface    `json:"deleteSurface,omitempty"`
	// v0.8 legacy aliases (accepted on parse, normalized on emit)
	SurfaceUpdate   *UpdateComponents `json:"surfaceUpdate,omitempty"`
	DataModelUpdate *UpdateDataModel  `json:"dataModelUpdate,omitempty"`
	BeginRendering  *BeginRendering   `json:"beginRendering,omitempty"`
}

type CreateSurface struct {
	SurfaceID string `json:"surfaceId"`
	CatalogID string `json:"catalogId,omitempty"`
}

type BeginRendering struct {
	SurfaceID string `json:"surfaceId"`
	Root      string `json:"root,omitempty"`
	CatalogID string `json:"catalogId,omitempty"`
}

type UpdateComponents struct {
	SurfaceID  string           `json:"surfaceId"`
	Components []map[string]any `json:"components,omitempty"`
}

// UnmarshalJSON accepts components as an array or an id-keyed object map.
func (uc *UpdateComponents) UnmarshalJSON(data []byte) error {
	type alias UpdateComponents
	var aux struct {
		alias
		Components json.RawMessage `json:"components"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*uc = UpdateComponents(aux.alias)
	if len(aux.Components) == 0 || string(aux.Components) == "null" {
		return nil
	}
	var itemWrapper struct {
		Item json.RawMessage `json:"item"`
	}
	if err := json.Unmarshal(aux.Components, &itemWrapper); err == nil && len(itemWrapper.Item) > 0 {
		var arr []map[string]any
		if err := json.Unmarshal(itemWrapper.Item, &arr); err == nil {
			uc.Components = arr
			return nil
		}
	}
	var arr []map[string]any
	if err := json.Unmarshal(aux.Components, &arr); err == nil {
		uc.Components = arr
		return nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(aux.Components, &obj); err != nil {
		return err
	}
	for id, raw := range obj {
		var comp map[string]any
		if err := json.Unmarshal(raw, &comp); err != nil {
			continue
		}
		if comp == nil {
			comp = map[string]any{}
		}
		if _, ok := comp["id"]; !ok {
			comp["id"] = id
		}
		uc.Components = append(uc.Components, comp)
	}
	return nil
}

type UpdateDataModel struct {
	SurfaceID string `json:"surfaceId"`
	Path      string `json:"path,omitempty"`
	Value     any    `json:"value,omitempty"`
}

type DeleteSurface struct {
	SurfaceID string `json:"surfaceId"`
}

// ClientMessage wraps client-to-server A2UI events (e.g. userAction).
type ClientMessage struct {
	UserAction *UserAction  `json:"userAction,omitempty"`
	Error      *ClientError `json:"error,omitempty"`
}

type UserAction struct {
	Name              string         `json:"name"`
	SurfaceID         string         `json:"surfaceId"`
	SourceComponentID string         `json:"sourceComponentId,omitempty"`
	Timestamp         string         `json:"timestamp,omitempty"`
	Context           map[string]any `json:"context,omitempty"`
}

type ClientError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// Kind returns the primary message type key.
func (m *ServerMessage) Kind() string {
	if m == nil {
		return ""
	}
	switch {
	case m.CreateSurface != nil:
		return "createSurface"
	case m.UpdateComponents != nil:
		return "updateComponents"
	case m.UpdateDataModel != nil:
		return "updateDataModel"
	case m.DeleteSurface != nil:
		return "deleteSurface"
	case m.SurfaceUpdate != nil:
		return "surfaceUpdate"
	case m.DataModelUpdate != nil:
		return "dataModelUpdate"
	case m.BeginRendering != nil:
		return "beginRendering"
	default:
		return ""
	}
}

// Normalize converts v0.8 messages to v0.9 equivalents where possible.
func (m *ServerMessage) Normalize() *ServerMessage {
	if m == nil {
		return nil
	}
	out := *m
	if out.Version == "" {
		out.Version = Version
	}
	if out.SurfaceUpdate != nil && out.UpdateComponents == nil {
		out.UpdateComponents = out.SurfaceUpdate
		out.SurfaceUpdate = nil
	}
	if out.DataModelUpdate != nil && out.UpdateDataModel == nil {
		out.UpdateDataModel = out.DataModelUpdate
		out.DataModelUpdate = nil
	}
	if out.BeginRendering != nil && out.CreateSurface == nil {
		out.CreateSurface = &CreateSurface{
			SurfaceID: out.BeginRendering.SurfaceID,
			CatalogID: NormalizeCatalogID(out.BeginRendering.CatalogID),
		}
		out.BeginRendering = nil
	}
	if out.CreateSurface != nil {
		cs := *out.CreateSurface
		cs.CatalogID = NormalizeCatalogID(cs.CatalogID)
		out.CreateSurface = &cs
	}
	return &out
}

// NormalizeCatalogID maps common agent shorthand (e.g. "basic") to the client catalog id.
func NormalizeCatalogID(catalogID string) string {
	switch catalogID {
	case "", "basic", "basic_catalog", "basicCatalog", "standard", "default":
		return BasicCatalogID
	default:
		return catalogID
	}
}

// Validate checks that the message has exactly one action key.
func (m *ServerMessage) Validate() error {
	if m == nil {
		return fmt.Errorf("a2ui: nil message")
	}
	kind := m.Kind()
	if kind == "" {
		return fmt.Errorf("a2ui: message must contain one of createSurface, updateComponents, updateDataModel, deleteSurface")
	}
	return nil
}

// RawJSON returns the message as a JSON object suitable for streaming.
func (m *ServerMessage) RawJSON() (json.RawMessage, error) {
	n := m.Normalize()
	data, err := json.Marshal(n)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}
