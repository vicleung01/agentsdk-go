package api

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/stellarlinkco/agentsdk-go/pkg/a2ui"
	"github.com/stellarlinkco/agentsdk-go/pkg/middleware"
	"github.com/stellarlinkco/agentsdk-go/pkg/model"
	"github.com/stellarlinkco/agentsdk-go/pkg/tool"
)

type a2uiStreamMiddleware struct {
	emitter   progressEmitter
	lineBuf   a2ui.LineBuffer
	textBlock int
}

func newA2UIStreamMiddleware(events chan<- StreamEvent) middleware.Middleware {
	return &a2uiStreamMiddleware{emitter: progressEmitter{ch: events}}
}

func (m *a2uiStreamMiddleware) Name() string { return "a2ui-stream" }

func (m *a2uiStreamMiddleware) BeforeAgent(ctx context.Context, st *middleware.State) error {
	m.lineBuf = a2ui.LineBuffer{}
	if st != nil {
		if st.Values == nil {
			st.Values = make(map[string]any)
		}
		st.Values[a2uiLineBufferKey] = &m.lineBuf
	}
	return nil
}

func (m *a2uiStreamMiddleware) BeforeTool(_ context.Context, _ *middleware.State) error {
	return nil
}

func (m *a2uiStreamMiddleware) AfterAgent(ctx context.Context, st *middleware.State) error {
	msgs, err := m.lineBuf.Flush()
	if err == nil {
		m.emitMessages(ctx, msgs)
	}
	resp, ok := st.ModelOutput.(*model.Response)
	if !ok || resp == nil {
		return nil
	}
	content := strings.TrimSpace(resp.Message.Content)
	if content == "" {
		return nil
	}
	for _, line := range a2ui.ExtractFromFencedBlock(content) {
		msg, err := a2ui.ParseLine(line)
		if err == nil && msg != nil {
			m.emitMessages(ctx, []*a2ui.ServerMessage{msg})
		}
	}
	if jsonlMsgs, err := a2ui.ParseJSONL(content); err == nil {
		m.emitMessages(ctx, jsonlMsgs)
	}
	return nil
}

func (m *a2uiStreamMiddleware) AfterTool(ctx context.Context, st *middleware.State) error {
	call, ok := st.ToolCall.(model.ToolCall)
	if !ok {
		return nil
	}
	if call.Name != "a2ui_push" && call.Name != "a2ui_reset" {
		return nil
	}

	var parsed []*a2ui.ServerMessage
	if cr, ok := st.ToolResult.(*tool.CallResult); ok && cr != nil && cr.Result != nil && cr.Result.Data != nil {
		if dataMap, ok := cr.Result.Data.(map[string]any); ok {
			for _, raw := range a2uiMessagesFromData(dataMap["a2ui_messages"]) {
				if msg, err := a2ui.ParseLine(string(raw)); err == nil && msg != nil {
					parsed = append(parsed, msg)
				}
			}
		}
	}
	if len(parsed) == 0 && len(call.Arguments) > 0 {
		if msgs, err := a2ui.ExtractMessagesFromToolParams(call.Arguments); err == nil {
			parsed = msgs
		}
	}
	for _, msg := range a2ui.RepairMessages(parsed) {
		m.emitNormalized(ctx, msg)
	}
	return nil
}

func a2uiMessagesFromData(raw any) []json.RawMessage {
	switch arr := raw.(type) {
	case []json.RawMessage:
		return arr
	case []any:
		var out []json.RawMessage
		for _, item := range arr {
			b, err := json.Marshal(item)
			if err == nil {
				out = append(out, b)
			}
		}
		return out
	default:
		return nil
	}
}

func (m *a2uiStreamMiddleware) emitMessages(ctx context.Context, msgs []*a2ui.ServerMessage) {
	for _, msg := range a2ui.RepairMessages(msgs) {
		m.emitNormalized(ctx, msg)
	}
}

func (m *a2uiStreamMiddleware) emitNormalized(ctx context.Context, msg *a2ui.ServerMessage) {
	if msg == nil {
		return
	}
	raw, err := msg.RawJSON()
	if err != nil || len(raw) == 0 {
		return
	}
	m.emitter.emit(ctx, StreamEvent{Type: EventA2UI, A2UI: raw})
}

func (m *a2uiStreamMiddleware) emitRaw(ctx context.Context, raw json.RawMessage) {
	if len(raw) == 0 {
		return
	}
	msg, err := a2ui.ParseLine(string(raw))
	if err != nil || msg == nil {
		m.emitter.emit(ctx, StreamEvent{Type: EventA2UI, A2UI: raw})
		return
	}
	m.emitNormalized(ctx, a2ui.NormalizeServerMessage(msg))
}

// emitA2UIFromStreamDelta scans streaming text for JSONL A2UI lines and fenced blocks.
func emitA2UIFromStreamDelta(ctx context.Context, emit streamEmitFunc, buf *a2ui.LineBuffer, text string) {
	if text == "" || emit == nil || buf == nil {
		return
	}
	msgs, err := buf.Push(text)
	if err == nil {
		for _, msg := range msgs {
			emit(ctx, StreamEvent{Type: EventA2UI, A2UI: mustRawJSON(a2ui.NormalizeServerMessage(msg))})
		}
	}
	for _, line := range a2ui.ExtractFromFencedBlock(text) {
		msg, err := a2ui.ParseLine(line)
		if err == nil && msg != nil {
			emit(ctx, StreamEvent{Type: EventA2UI, A2UI: mustRawJSON(a2ui.NormalizeServerMessage(msg))})
		}
	}
}

func mustRawJSON(msg *a2ui.ServerMessage) json.RawMessage {
	if msg == nil {
		return nil
	}
	raw, err := msg.RawJSON()
	if err != nil {
		return nil
	}
	return raw
}

// emitA2UIFromTextDelta scans streaming text for JSONL A2UI lines and fenced blocks.
func emitA2UIFromTextDelta(ctx context.Context, emitter progressEmitter, buf *a2ui.LineBuffer, text string) {
	if text == "" {
		return
	}
	emit := func(ctx context.Context, evt StreamEvent) {
		emitter.emit(ctx, evt)
	}
	emitA2UIFromStreamDelta(ctx, emit, buf, text)
}
