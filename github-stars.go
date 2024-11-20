package main

import (
	"context"
	"errors"
	"flag"
	"github.com/clambin/github-stars/internal/stars"
	"github.com/clambin/github-stars/internal/store"
	"github.com/clambin/github-stars/internal/webhook"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	version = "change-me"

	debug           = flag.Bool("debug", false, "Enable debug mode")
	webHookAddr     = flag.String("webhook.addr", ":8080", "Address for the webhook server")
	webHookSecret   = flag.String("webhook.secret", "todo", "Secret for the webhook server")
	addr            = flag.String("addr", ":9091", "Prometheus handler address")
	githubToken     = flag.String("github.token", "", "GitHub API githubToken")
	slackWebHook    = flag.String("slack.webHook", "", "Slack WebHook URL")
	directory       = flag.String("directory", ".", "Database directory")
	user            = flag.String("user", "", "GitHub username")
	repoInterval    = flag.Duration("repo_interval", 24*time.Hour, "Repo scanning interval")
	starInterval    = flag.Duration("star_interval", 15*time.Minute, "Stargazer count interval")
	includeArchived = flag.Bool("include_archived", false, "Include archived repositories")
)

func main() {
	flag.Parse()

	var opts slog.HandlerOptions
	if *debug {
		opts.Level = slog.LevelDebug
	}
	l := slog.New(slog.NewTextHandler(os.Stderr, &opts))

	http.Handle("/metrics", promhttp.Handler())
	go func() {
		if err := http.ListenAndServe(*addr, nil); !errors.Is(err, http.ErrServerClosed) {
			l.Warn("failed to start Prometheus handler", "err", err)
		}
	}()

	notifiers := stars.Notifiers{
		stars.SLogNotifier{Logger: l},
	}
	if *slackWebHook != "" {
		notifiers = append(notifiers, &stars.SlackWebHookNotifier{WebHookURL: *slackWebHook, Logger: l})
	}

	repoStore := store.New(*directory)

	// TODO: run Scanner first. After that, start the Webhook to listen for new stars (in the main goroutine).
	go func() {
		lst := webhook.Webhook{
			Store:  repoStore,
			Logger: l.With("component", "webhook"),
		}
		if err := http.ListenAndServe(*webHookAddr, webhook.GitHubAuth(*webHookSecret)(&lst)); !errors.Is(err, http.ErrServerClosed) {
			l.Warn("failed to start WebHook handler", "err", err)
		}
	}()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	m := stars.RepoScanner{
		User:            *user,
		RepoInterval:    *repoInterval,
		StarInterval:    *starInterval,
		Logger:          l.With("component", "scanner"),
		Client:          stars.NewGitHubClient(*githubToken),
		Store:           repoStore,
		Notifier:        notifiers,
		IncludeArchived: *includeArchived,
	}

	l.Info("github-start starting", "version", version)

	if err := m.Run(ctx); err != nil {
		l.Error("github-stars failed", "err", err)
	}
}
