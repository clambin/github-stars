package stars

import (
	"context"
	"fmt"
	ggh "github.com/google/go-github/v65/github"
	"golang.org/x/sync/errgroup"
	"log/slog"
	"sync"
	"time"
)

type RepoScanner struct {
	User         string
	RepoInterval time.Duration
	StarInterval time.Duration
	Logger       *slog.Logger
	Client       *Client
	RepoStore
	Notifier
	lock            sync.Mutex
	processors      map[string]struct{}
	IncludeArchived bool
	children        errgroup.Group
}

type RepoStore interface {
	Add(*ggh.Repository, *ggh.Stargazer) (bool, error)
	Load() error
	Save() error
}

func (r *RepoScanner) Run(ctx context.Context) error {
	ticker := time.NewTicker(r.RepoInterval)
	defer ticker.Stop()

	if err := r.RepoStore.Load(); err != nil {
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
		reposFound++
		if err != nil {
			return fmt.Errorf("GetUserRepoNames: %w", err)
		}
		r.Logger.Debug("repo found", "repo", repo.GetFullName(), "archived", repo.GetArchived())
		if !r.IncludeArchived && repo.GetArchived() {
			continue
		}
		if _, ok := r.processors[repo.GetName()]; !ok {
			reposProcessing++
			p := Processor{
				User:       r.User,
				Repository: repo,
				Interval:   r.StarInterval,
				Client:     r.Client,
				RepoStore:  r.RepoStore,
				Notifier:   r.Notifier,
				Logger:     r.Logger.With("repo", repo.GetFullName()),
			}
			r.children.Go(func() error { p.Run(ctx); return nil })
		}
	}
	return nil
}

type Processor struct {
	User       string
	Repository *ggh.Repository
	Interval   time.Duration
	Client     *Client
	RepoStore
	Notifier
	Logger *slog.Logger
}

func (p *Processor) Run(ctx context.Context) {
	p.Logger.Debug("processor started")
	defer p.Logger.Debug("processor stopped")
	ticker := time.NewTicker(p.Interval)
	defer ticker.Stop()
	for {
		if err := p.getStargazers(ctx); err != nil {
			p.Logger.Error("failed to get star gazers", "err", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (p *Processor) getStargazers(ctx context.Context) error {
	var newStarGazers []*ggh.Stargazer
	for starGazer, err := range p.Client.GetStarGazers(ctx, p.User, p.Repository.GetName()) {
		if err != nil {
			return err
		}
		added, err := p.RepoStore.Add(p.Repository, starGazer)
		if err != nil {
			return err
		}
		if added {
			newStarGazers = append(newStarGazers, starGazer)
		}
	}
	if len(newStarGazers) == 0 {
		return nil
	}
	p.Notifier.Notify(p.Repository, newStarGazers)
	return nil
}
