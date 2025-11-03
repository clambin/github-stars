package stars

import (
	"context"
	"fmt"
	"iter"
	"time"

	"github.com/clambin/github-stars/slogctx"
	"github.com/google/go-github/v76/github"
	"golang.org/x/sync/errgroup"
)

type Client interface {
	GetUserRepos(ctx context.Context, user string) iter.Seq2[*github.Repository, error]
	GetStarGazers(ctx context.Context, repository *github.Repository) ([]*github.Stargazer, error)
}

// Scan retrieves all repositories for the user, gets the stars for each repository and adds new ones to the Store.
func Scan(
	ctx context.Context,
	user string,
	c Client,
	s *NotifyingStore,
	includeArchived bool,
) error {
	logger := slogctx.FromContext(ctx)
	logger.Debug("scanning all user repos")

	var reposFound, reposScanned int
	start := time.Now()

	var g errgroup.Group
	for repo, err := range c.GetUserRepos(ctx, user) {
		if err != nil {
			return fmt.Errorf("GetUserRepos: %w", err)
		}
		reposFound++
		logger.Debug("repo found", "repo", repo.GetFullName(), "archived", repo.GetArchived())
		if !includeArchived && repo.GetArchived() {
			continue
		}
		reposScanned++
		g.Go(func() error {
			return scanRepo(ctx, c, s, repo)
		})
	}
	err := g.Wait()
	logger.Debug("all user repos scanned", "err", err, "found", reposFound, "scanned", reposScanned, "elapsed", time.Since(start))
	return err
}

func scanRepo(
	ctx context.Context,
	c Client,
	s *NotifyingStore,
	r *github.Repository,
) error {
	stargazers, err := c.GetStarGazers(ctx, r)
	if err == nil {
		err = s.Add(ctx, r, stargazers...)
	}
	// TODO: this only adds stars to an existing store.  if someone unstarred a repo while we were down, it won't be removed.
	// to fix that, we need to iterate over stargazers in the store and remove any that are not in the new list.
	return err
}
