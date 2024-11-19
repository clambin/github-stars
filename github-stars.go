package main

import (
	"context"
	"errors"
	"flag"
	"github.com/clambin/github-stars/internal/stars"
	"github.com/google/go-github/v66/github"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/oauth2"
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

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	m := stars.RepoScanner{
		User:         *user,
		RepoInterval: *repoInterval,
		StarInterval: *starInterval,
		Logger:       l,
		Client: stars.New(github.NewClient(oauth2.NewClient(ctx, oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: *githubToken},
		)))),
		Store:           &stars.Store{DatabasePath: *directory},
		Notifier:        notifiers,
		IncludeArchived: *includeArchived,
	}

	l.Info("github-start starting", "version", version)

	if err := m.Run(ctx); err != nil {
		l.Error("github-stars failed", "err", err)
	}
}
