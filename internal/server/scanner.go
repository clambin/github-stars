package server

import (
	"context"
	"fmt"
	"github.com/google/go-github/v66/github"
	"golang.org/x/sync/errgroup"
	"log/slog"
	"time"
)

// Scan retrieves all repositories for the user, gets the stars for each repository and adds new ones to the Store.
func Scan(
	ctx context.Context,
	user string,
	c Client,
	s Store,
	n Notifier,
	includeArchived bool,
	l *slog.Logger,
) error {
	l.Debug("scanning all user repos")

	var reposFound, reposScanned int
	start := time.Now()

	var g errgroup.Group
	for repo, err := range c.GetUserRepos(ctx, user) {
		if err != nil {
			return fmt.Errorf("GetUserRepos: %w", err)
		}
		reposFound++
		l.Debug("repo found", "repo", repo.GetFullName(), "archived", repo.GetArchived())
		if !includeArchived && repo.GetArchived() {
			continue
		}
		reposScanned++
		g.Go(func() error {
			return scanRepo(ctx, c, s, n, repo, l.With("repo", repo.GetFullName()))
		})
	}
	err := g.Wait()
	l.Debug("all user repos scanned", "err", err, "found", reposFound, "scanned", reposScanned, "elapsed", time.Since(start))
	return err
}

func scanRepo(
	ctx context.Context,
	c Client,
	s Store,
	n Notifier,
	r *github.Repository,
	_ *slog.Logger,
) error {
	stargazers, err := c.GetStarGazers(ctx, r)
	if err != nil {
		return err
	}
	newStargazers, err := s.SetStargazers(r, stargazers)
	if err == nil && len(newStargazers) > 0 && n != nil {
		n.Notify(r, newStargazers, true)
	}
	return err
}
