package stars

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/clambin/github-stars/internal/github"
	"github.com/clambin/github-stars/slogctx"
)

type Client interface {
	Stargazers(context.Context, string, bool) ([]github.Stargazer, error)
}

// Scan retrieves all repositories for the user, gets the stars for each repository and adds new ones to the Store.
func Scan(ctx context.Context, user string, c Client, s *NotifyingStore, includeArchived bool) error {
	stargazers, err := c.Stargazers(ctx, user, includeArchived)
	if err != nil {
		return fmt.Errorf("stars: %w", err)
	}

	if err = s.Set(ctx, stargazers); err != nil {
		return fmt.Errorf("add: %w", err)
	}
	return nil
}

// Handler returns a webhook handler for GitHub star events.
func Handler(store *NotifyingStore) func(ctx context.Context, stargazer github.Stargazer) error {
	return func(ctx context.Context, stargazer github.Stargazer) (err error) {
		// Get logger
		logger := slogctx.FromContext(ctx).With(
			slog.String("repo", stargazer.RepoName),
			slog.String("user", stargazer.Login),
		)

		// Handle the "star" event
		switch stargazer.Action {
		case "created":
			logger.Debug("adding new stargazer")
			if err = store.Add(ctx, stargazer); err != nil {
				err = fmt.Errorf("add: %w", err)
			}
		case "deleted":
			logger.Debug("removing a stargazer")
			if err = store.Delete(ctx, stargazer); err != nil {
				err = fmt.Errorf("delete: %w", err)
			}
		default:
			err = fmt.Errorf("unsupported action: %s", stargazer.Action)
		}
		if err != nil {
			logger.Error("failed to handle event", "err", err)
		}
		return err
	}
}
