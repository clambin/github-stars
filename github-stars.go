package main

import (
	"flag"
	"net/http"
	"os"
	"os/signal"
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
	GitHub          githubConfiguration
	Slack           slackConfiguration
	Directory       string
	User            string
	IncludeArchived bool
}

type githubConfiguration struct {
	Token   string
	WebHook webhookConfiguration
}

type webhookConfiguration struct {
	Addr   string
	Secret string
}

type slackConfiguration struct {
	Webhook string
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

	// setup
	logger := cfg.Logger(os.Stderr, nil)
	logger.Info("starting github-stars", "version", version)

	notifiers := stars.Notifiers{
		stars.SlogNotifier{},
	}
	if cfg.Slack.Webhook != "" {
		notifiers = append(notifiers, stars.SlackNotifier{WebHookURL: cfg.Slack.Webhook})
	}

	store, err := stars.NewNotifyingStore(cfg.Directory, notifiers)
	if err != nil {
		logger.Error("failed to create store", "err", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(slogctx.New(logger), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// on startup, scan all repos.  this will find any stars while we weren't running.
	before := time.Now()
	logger.Info("starting scan")
	if err = stars.Scan(ctx, cfg.User, github.NewGitHubClient(cfg.GitHub.Token), store, cfg.IncludeArchived); err != nil {
		logger.Error("failed to scan", "err", err)
		os.Exit(1)
	}
	logger.Info("scan complete", "duration_msec", time.Since(before).Milliseconds())

	// start the GitHub webhook handler
	s := http.Server{
		Addr:    cfg.GitHub.WebHook.Addr,
		Handler: github.NewWebHook(cfg.GitHub.WebHook.Secret, stars.Handler(store), logger),
	}

	logger.Info("starting webhook server", "addr", cfg.GitHub.WebHook.Addr)
	if err = httputils.RunServer(ctx, &s); err != nil {
		logger.Error("failed to start webhook server", "err", err)
		os.Exit(1)
	}
}
