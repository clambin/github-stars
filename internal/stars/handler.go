package stars

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/clambin/github-stars/internal/github"
	"github.com/clambin/github-stars/slogctx"
)

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
