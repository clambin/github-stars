// Package slogctx manages a slog.Logger in a context.Context.
//
// This is just a proof of concept. It's not production ready.
package slogctx

import (
	"context"
	"log/slog"
)

type slogCtxKey struct{}

func New(l *slog.Logger) context.Context {
	return NewWithContext(context.Background(), l)
}

func NewWithContext(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, slogCtxKey{}, l)
}

func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(slogCtxKey{}).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}
