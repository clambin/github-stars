package stars

import (
	"context"
	"fmt"

	"github.com/clambin/github-stars/internal/github"
	"github.com/clambin/github-stars/slogctx"
)

type Client interface {
	Stargazers(context.Context, string, bool) ([]github.Stargazer, error)
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

	stargazers, err := c.Stargazers(ctx, user, includeArchived)
	if err != nil {
		return fmt.Errorf("stars: %w", err)
	}

	// TODO: this only adds stars to an existing store.  if someone unstarred a repo while we were down, it won't be removed.
	// to fix that, we need to iterate over stargazers in the store and remove any that are not in the new list.

	if err = s.Add(ctx, stargazers...); err != nil {
		return fmt.Errorf("add: %w", err)
	}
	return nil
}
