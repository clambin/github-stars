package stars

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/clambin/github-stars/internal/github"
	"github.com/clambin/github-stars/slogctx"
)

func Handler(store *NotifyingStore) func(ctx context.Context, stargazer github.Stargazer) error {
	return func(ctx context.Context, stargazer github.Stargazer) error {
		// Get logger
		logger := slogctx.FromContext(ctx).With(
			slog.String("repo", stargazer.RepoName),
			slog.String("user", stargazer.Login),
		)

		// Handle the "star" event
		switch stargazer.Action {
		case "created":
			logger.Debug("adding new stargazer")
			if err := store.Add(ctx, stargazer); err != nil {
				logger.Error("Unable to store star", "err", err)
				return fmt.Errorf("add: %w", err)
			}
		case "deleted":
			logger.Debug("removing a stargazer")
			if err := store.Delete(ctx, stargazer); err != nil {
				logger.Error("Unable to store star", "err", err)
				return fmt.Errorf("delete: %w", err)
			}
		default:
			logger.Warn("Unsupported action", "action", stargazer.Action)
			return fmt.Errorf("unsupported action: %s", stargazer.Action)
		}
		return nil
	}
}
