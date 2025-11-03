package testutils

import (
	"io"
	"log/slog"
)

func Ptr[T any](v T) *T {
	return &v
}

func SLogWithoutTime(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == "time" {
				return slog.Attr{}
			}
			return a
		},
	}))
}
