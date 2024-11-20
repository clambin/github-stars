package main

import (
	"context"
	"errors"
	"flag"
	"github.com/clambin/github-stars/internal/github"
	"github.com/clambin/github-stars/internal/server"
	"github.com/clambin/github-stars/internal/store"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

var (
	version = "change-me"

	debug           = flag.Bool("debug", false, "Enable debug mode")
	githubToken     = flag.String("github.token", "", "GitHub API githubToken")
	webHookAddr     = flag.String("github.webhook.addr", ":8080", "Address for the webhook server")
	webHookSecret   = flag.String("github.webhook.secret", "todo", "Secret for the webhook server")
	addr            = flag.String("addr", ":9091", "Prometheus handler address")
	slackWebHook    = flag.String("slack.webHook", "", "Slack WebHook URL")
	directory       = flag.String("directory", ".", "Database directory")
	user            = flag.String("user", "", "GitHub username")
	includeArchived = flag.Bool("include_archived", false, "Include archived repositories")
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

	l.Info("github-stars is starting", "version", version)

	http.Handle("/metrics", promhttp.Handler())
	go func() {
		if err := http.ListenAndServe(*addr, nil); !errors.Is(err, http.ErrServerClosed) {
			l.Warn("failed to start Prometheus handler", "err", err)
		}
	}()

	notifiers := server.Notifiers{
		server.SLogNotifier{Logger: l},
	}
	if *slackWebHook != "" {
		notifiers = append(notifiers, &server.SlackWebHookNotifier{WebHookURL: *slackWebHook, Logger: l})
	}

	client := github.NewGitHubClient(*githubToken)
	repoStore := store.New(*directory)
	if err := repoStore.Load(); err != nil {
		l.Warn("database not found. possibly this is a new installation", "err", err)
	}

	l.Info("scanning repositories")

	// Scan repos and populate the database. This catches up any stars given while we were down.
	err := server.Scan(ctx, *user, client, repoStore, notifiers, *includeArchived, l.With("component", "scanner"))
	if err != nil {
		l.Error("failed to scan github repositories", "err", err)
	}

	l.Info("starting webhook handler")

	h := server.GitHubAuth(*webHookSecret)(
		&server.Webhook{
			Store:  repoStore,
			Logger: l.With("component", "webhook"),
		},
	)

	if err = http.ListenAndServe(*webHookAddr, h); !errors.Is(err, http.ErrServerClosed) {
		l.Error("failed to start WebHook handler", "err", err)
	}
}
