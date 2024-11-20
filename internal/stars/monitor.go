package stars

import (
	"context"
	"fmt"
	"github.com/clambin/github-stars/internal/store"
	"github.com/google/go-github/v66/github"
	"golang.org/x/sync/errgroup"
	"log/slog"
	"sync"
	"time"
)

type RepoScanner struct {
	User            string
	RepoInterval    time.Duration
	StarInterval    time.Duration
	Logger          *slog.Logger
	Client          *Client
	Store           *store.Store
	Notifier        Notifier
	lock            sync.Mutex
	processors      map[string]struct{}
	IncludeArchived bool
	children        errgroup.Group
}

func (r *RepoScanner) Run(ctx context.Context) error {
	ticker := time.NewTicker(r.RepoInterval)
	defer ticker.Stop()

	if err := r.Store.Load(); err != nil {
		r.Logger.Warn("failed to read stargazers store", "err", err)
	}
	for {
		if err := r.pollRepos(ctx); err != nil {
			r.Logger.Error("failed to get repos", "err", err)
		}
		select {
		case <-ctx.Done():
			return r.children.Wait()
		case <-ticker.C:
		}
	}
}

func (r *RepoScanner) pollRepos(ctx context.Context) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	start := time.Now()
	var reposFound, reposProcessing int
	r.Logger.Debug("polling repos")
	defer func() {
		r.Logger.Debug("repos polled", "found", reposFound, "processing", reposProcessing, "duration", time.Since(start))
	}()

	for repo, err := range r.Client.GetUserRepoNames(ctx, r.User) {
		if err != nil {
			return fmt.Errorf("GetUserRepoNames: %w", err)
		}
		reposFound++
		r.Logger.Debug("repo found", "repo", repo.GetFullName(), "archived", repo.GetArchived())
		if !r.IncludeArchived && repo.GetArchived() {
			continue
		}
		if _, ok := r.processors[repo.GetName()]; !ok {
			reposProcessing++
			r.startProcessor(ctx, repo)
		}
	}
	return nil
}

func (r *RepoScanner) startProcessor(ctx context.Context, repo *github.Repository) {
	p := Processor{
		User:       r.User,
		Repository: repo,
		Interval:   r.StarInterval,
		Client:     r.Client,
		Store:      r.Store,
		Notifier:   r.Notifier,
		Logger:     r.Logger.With("repo", repo.GetFullName()),
	}
	if r.processors == nil {
		r.processors = make(map[string]struct{})
	}
	r.processors[repo.GetName()] = struct{}{}
	r.children.Go(func() error { p.Run(ctx); return nil })
}

type Processor struct {
	User       string
	Repository *github.Repository
	Interval   time.Duration
	Client     *Client
	Store      *store.Store
	Notifier   Notifier
	Logger     *slog.Logger
}

func (p *Processor) Run(ctx context.Context) {
	p.Logger.Debug("processor started")
	defer p.Logger.Debug("processor stopped")
	ticker := time.NewTicker(p.Interval)
	defer ticker.Stop()
	for {
		if err := p.pollStargazers(ctx); err != nil {
			p.Logger.Error("failed to get star gazers", "err", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (p *Processor) pollStargazers(ctx context.Context) error {
	stargazers, err := p.Client.GetStarGazers(ctx, p.User, p.Repository.GetName())
	if err != nil {
		return err
	}
	fn := p.Repository.GetFullName()
	newStargazers, err := p.Store.SetStargazers(fn, stargazers)
	if err == nil && len(newStargazers) > 0 {
		p.Notifier.Notify(p.Repository, stargazers)
	}
	return err
}
