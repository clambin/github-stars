package slogctx_test

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/clambin/github-stars/slogctx"
	"github.com/stretchr/testify/assert"
)

func TestSlogCtx(t *testing.T) {
	var buf bytes.Buffer
	l := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
		if a.Key == "time" {
			return slog.Attr{}
		}
		return a
	}}))
	ctx := slogctx.New(l)
	slogctx.FromContext(ctx).Info("test")
	assert.Equal(t, "level=INFO msg=test\n", buf.String())
}
