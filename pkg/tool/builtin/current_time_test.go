package toolbuiltin

import (
	"context"
	"strings"
	"testing"
)

func TestCurrentTimeUTC(t *testing.T) {
	res, err := (CurrentTimeTool{}).Execute(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !res.Success || !strings.Contains(res.Output, "timezone: UTC") {
		t.Fatalf("unexpected output: %s", res.Output)
	}
}
