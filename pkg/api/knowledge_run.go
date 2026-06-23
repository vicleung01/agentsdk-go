package api

import (
	"context"

	"github.com/stellarlinkco/agentsdk-go/pkg/message"
)

type knowledgeRunContextKey struct{}

// knowledgeRunBundle is injected into context so session_search can read session history.
type knowledgeRunBundle struct {
	History            *message.History
	DefaultSearchLimit int
}

func withKnowledgeRun(ctx context.Context, b *knowledgeRunBundle) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, knowledgeRunContextKey{}, b)
}

func knowledgeRunFromContext(ctx context.Context) *knowledgeRunBundle {
	if ctx == nil {
		return nil
	}
	b, _ := ctx.Value(knowledgeRunContextKey{}).(*knowledgeRunBundle)
	return b
}
