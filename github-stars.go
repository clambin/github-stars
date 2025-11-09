package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"codeberg.org/clambin/go-common/flagger"
	"codeberg.org/clambin/go-common/httputils"
	"github.com/clambin/github-stars/internal/github"
	"github.com/clambin/github-stars/internal/stars"
	"github.com/clambin/github-stars/slogctx"
)

var version = "(devel)"

type configuration struct {
	flagger.Log
	flagger.Prom
	GitHub    githubConfiguration
	Slack     slackConfiguration
	Directory string `flagger.usage:"database directory"`
	User      string `flagger.usage:"user to scan for repositories"`
	Archived  bool   `flagger.usage:"include archived repositories"`
}

type githubConfiguration struct {
	Token   string `flagger.usage:"GitHub API token"`
	WebHook webhookConfiguration
}

type webhookConfiguration struct {
	Addr   string `flagger.usage:"address to listen on for GitHub webhook calls"`
	Secret string `flagger.usage:"secret to verify GitHub webhook calls"`
}

type slackConfiguration struct {
	Webhook string `flagger.usage:"Slack webhook URL to post messages to"`
}

func main() {
	cfg := configuration{
		Log:  flagger.DefaultLog,
		Prom: flagger.DefaultProm,
		GitHub: githubConfiguration{
			WebHook: webhookConfiguration{Addr: ":8080"},
		},
		Slack:     slackConfiguration{},
		Directory: ".",
	}
	flagger.SetFlags(flag.CommandLine, &cfg)
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	client := github.NewGitHubClient(cfg.GitHub.Token)
	if err := runWithClient(ctx, client, cfg); err != nil {
		cfg.Logger(os.Stderr, nil).Error("failed to run", "err", err)
		os.Exit(1)
	}
}

func runWithClient(ctx context.Context, client stars.Client, cfg configuration) error {
	// setup
	logger := cfg.Logger(os.Stderr, nil)
	logger.Info("starting github-stars", "version", version)
	ctx = slogctx.NewWithContext(ctx, logger)

	notifiers := stars.Notifiers{stars.SlogNotifier{}}
	if cfg.Slack.Webhook != "" {
		notifiers = append(notifiers, stars.SlackNotifier{WebHookURL: cfg.Slack.Webhook})
	}

	store, err := stars.NewNotifyingStore(cfg.Directory, notifiers)
	var jsonErr *json.UnmarshalTypeError
	if errors.As(err, &jsonErr) {
		logger.Warn("failed to load database. reinitializing ....", "err", err)
		_ = os.Remove(filepath.Join(cfg.Directory, stars.StoreFilename))
		store, err = stars.NewNotifyingStore(cfg.Directory, notifiers)
	}
	if err != nil {
		return fmt.Errorf("failed to load database: %w", err)
	}

	// on startup, scan all repos. This will find any stars while we weren't running.
	start := time.Now()
	logger.Info("starting scan")
	if err = stars.Scan(ctx, cfg.User, client, store, cfg.Archived); err != nil {
		return fmt.Errorf("failed to scan: %w", err)
	}
	logger.Info("scan complete", "duration_msec", time.Since(start).Milliseconds())

	// start the Prometheus metrics server
	go func() {
		if err := cfg.Serve(ctx); err != nil {
			logger.Error("failed to start Prometheus server", "err", err)
		}
	}()

	// start the GitHub webhook handler
	s := http.Server{
		Addr: cfg.GitHub.WebHook.Addr,
		Handler: github.WebhookHandler(
			github.WebhookHandlers{StarEvent: stars.Handler(store)},
			cfg.GitHub.WebHook.Secret,
			logger,
		),
	}

	logger.Info("starting webhook server", "addr", cfg.GitHub.WebHook.Addr)
	if err = httputils.RunServer(ctx, &s); err != nil {
		return fmt.Errorf("failed to start webhook server: %w", err)
	}
	return nil
}
