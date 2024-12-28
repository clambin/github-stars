package server

import (
	"context"
	"flag"
	ghc "github.com/clambin/github-stars/internal/github"
	"github.com/clambin/github-stars/internal/store"
	"github.com/clambin/go-common/httputils"
	"github.com/google/go-github/v68/github"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/sync/errgroup"
	"iter"
	"log/slog"
	"net/http"
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
	Notify(repository *github.Repository, gazers []*github.Stargazer, added bool)
}

func Run(ctx context.Context, version string, l *slog.Logger) error {
	client := ghc.NewGitHubClient(*githubToken)
	return runWithClient(ctx, client, version, l)
}

func runWithClient(ctx context.Context, client Client, version string, l *slog.Logger) error {
	l.Info("github-stars is starting", "version", version)

	var g errgroup.Group
	g.Go(func() error {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		return httputils.RunServer(ctx, &http.Server{Addr: *addr, Handler: mux})
	})

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

	mux := http.NewServeMux()
	mux.Handle("/readyz", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	mux.Handle("/", GitHubAuth(*webHookSecret)(
		GithubWebHookHandler(notifiers, repoStore, l.With("component", "webhook")),
	))
	httpServer := &http.Server{
		Addr:    *webHookAddr,
		Handler: mux,
	}
	g.Go(func() error { return httputils.RunServer(ctx, httpServer) })
	return g.Wait()
}
