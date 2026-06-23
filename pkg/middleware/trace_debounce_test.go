package middleware

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTraceMiddlewareHTMLDebounceCoalescesRenders(t *testing.T) {
	dir := t.TempDir()
	tm := NewTraceMiddleware(dir, WithHTMLDebounce(80*time.Millisecond))
	t.Cleanup(tm.Close)

	sess := &traceSession{
		id:        "debounce",
		jsonPath:  filepath.Join(dir, "log.jsonl"),
		htmlPath:  filepath.Join(dir, "log.html"),
		createdAt: time.Unix(0, 0).UTC(),
		updatedAt: time.Unix(0, 0).UTC(),
		events:    []TraceEvent{{Timestamp: time.Unix(1, 0).UTC(), Stage: "before_agent", SessionID: "debounce"}},
	}
	tm.sessions["debounce"] = sess

	for i := 0; i < 5; i++ {
		tm.scheduleHTMLRender(sess)
	}
	time.Sleep(150 * time.Millisecond)
	tm.waitHTMLRender(sess)

	info, err := os.Stat(sess.htmlPath)
	if err != nil {
		t.Fatalf("expected html output: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("expected non-empty html file")
	}
}

func TestTraceMiddlewareHTMLRenderDisabled(t *testing.T) {
	dir := t.TempDir()
	tm := NewTraceMiddleware(dir, WithHTMLRender(false))
	t.Cleanup(tm.Close)

	sess := &traceSession{
		id:        "nohtml",
		jsonPath:  filepath.Join(dir, "log.jsonl"),
		htmlPath:  filepath.Join(dir, "log.html"),
		createdAt: time.Unix(0, 0).UTC(),
		updatedAt: time.Unix(0, 0).UTC(),
	}
	tm.sessions["nohtml"] = sess
	tm.scheduleHTMLRender(sess)
	time.Sleep(20 * time.Millisecond)
	if _, err := os.Stat(sess.htmlPath); !os.IsNotExist(err) {
		t.Fatalf("expected no html file, stat err=%v", err)
	}
}
