package server

import (
	"context"
	"errors"
	"flag"
	ghc "github.com/clambin/github-stars/internal/github"
	"github.com/clambin/github-stars/internal/store"
	"github.com/google/go-github/v66/github"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"iter"
	"log/slog"
	"net/http"
	"time"
)

var (
	githubToken     = flag.String("github.token", "", "GitHub API githubToken")
	webHookAddr     = flag.String("github.webhook.addr", ":8080", "Address for the webhook server")
	webHookSecret   = flag.String("github.webhook.secret", "todo", "Secret for the webhook server")
	addr            = flag.String("addr", ":9091", "Prometheus handler address")
	slackWebHook    = flag.String("slack.webHook", "", "Slack WebHook URL")
	directory       = flag.String("directory", ".", "Database directory")
	user            = flag.String("user", "", "GitHub username")
	includeArchived = flag.Bool("include_archived", false, "Include archived repositories")
)

type Client interface {
	GetUserRepos(ctx context.Context, user string) iter.Seq2[*github.Repository, error]
	GetStarGazers(ctx context.Context, repository *github.Repository) ([]*github.Stargazer, error)
}

type Store interface {
	SetStargazers(repository *github.Repository, stargazers []*github.Stargazer) ([]*github.Stargazer, error)
	Add(repo *github.Repository, stargazer *github.Stargazer) (bool, error)
	Delete(repo *github.Repository, stargazer *github.Stargazer) (bool, error)
}

type Notifier interface {
	Notify(repository *github.Repository, gazers []*github.Stargazer)
}

func Run(ctx context.Context, version string, l *slog.Logger) error {
	client := ghc.NewGitHubClient(*githubToken)
	return runWithClient(ctx, client, version, l)
}

func runWithClient(ctx context.Context, client Client, version string, l *slog.Logger) error {
	l.Info("github-stars is starting", "version", version)

	http.Handle("/metrics", promhttp.Handler())
	go func() {
		if err := http.ListenAndServe(*addr, nil); !errors.Is(err, http.ErrServerClosed) {
			l.Warn("failed to start Prometheus handler", "err", err)
		}
	}()

	notifiers := Notifiers{
		SLogNotifier{Logger: l.With("component", "slogNotifier")},
	}
	if *slackWebHook != "" {
		notifiers = append(notifiers, &SlackNotifier{WebHookURL: *slackWebHook, Logger: l.With("component", "slackNotifier")})
	}

	repoStore := store.New(*directory)
	if err := repoStore.Load(); err != nil {
		l.Warn("database not found. possibly this is a new installation", "err", err)
	}

	l.Info("scanning repositories")

	// Scan repos and populate the database. This catches up any stars given while we were down.
	err := Scan(ctx, *user, client, repoStore, notifiers, *includeArchived, l.With("component", "scanner"))
	if err != nil {
		l.Error("failed to scan github repositories", "err", err)
	}

	l.Info("starting webhook handler")

	httpServer := &http.Server{
		Addr: *webHookAddr,
		Handler: GitHubAuth(*webHookSecret)(
			&GitHubWebhook{
				Store:     repoStore,
				Notifiers: notifiers,
				Logger:    l.With("component", "webhook"),
			},
		),
	}

	errCh := make(chan error)
	go func() {
		err = httpServer.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		errCh <- err
	}()

	select {
	case err = <-errCh:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	return httpServer.Shutdown(shutdownCtx)
}
