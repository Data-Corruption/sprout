package version

import (
	"context"
)

type ctxKey struct{}

func IntoContext(ctx context.Context, path string) context.Context {
	return context.WithValue(ctx, ctxKey{}, path)
}

func FromContext(ctx context.Context) string {
	if path, ok := ctx.Value(ctxKey{}).(string); ok {
		return path
	}
	return ""
}
