package main

import (
	"context"
	"flag"
	"github.com/clambin/github-stars/internal/server"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

var (
	version = "change-me"

	debug = flag.Bool("debug", false, "Enable debug mode")
)

func main() {
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var opts slog.HandlerOptions
	if *debug {
		opts.Level = slog.LevelDebug
	}
	l := slog.New(slog.NewTextHandler(os.Stderr, &opts))

	if err := server.Run(ctx, version, l); err != nil {
		l.Error("server failed", "err", err)
		os.Exit(1)
	}
}
