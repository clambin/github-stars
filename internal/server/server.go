package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/clambin/github-stars/internal/github"
	"github.com/clambin/github-stars/internal/stars"
	"github.com/clambin/github-stars/slogctx"
)

type StarStore interface {
	Add(context.Context, ...github.Stargazer) error
	Delete(context.Context, ...github.Stargazer) error
}

var _ StarStore = (*stars.NotifyingStore)(nil)

func New(
	store StarStore,
	secret string,
	logger *slog.Logger,
) http.Handler {
	return github.WebhookHandler{
		StarEvent: starEventHandler(store),
		Secret:    secret,
		Logger:    logger,
	}.Handler()
}

func starEventHandler(store StarStore) github.StarEventFunc {
	return func(ctx context.Context, stargazer github.Stargazer) (err error) {
		// Get logger
		logger := slogctx.FromContext(ctx).With(
			slog.String("repo", stargazer.RepoName),
			slog.String("user", stargazer.Login),
		)

		defer func() {
			if err != nil {
				logger.Error("failed to handled event", "err", err)
			}
		}()

		// Handle the "star" event
		switch stargazer.Action {
		case "created":
			logger.Debug("adding new stargazer")
			if err = store.Add(ctx, stargazer); err != nil {
				return fmt.Errorf("add: %w", err)
			}
			return nil
		case "deleted":
			logger.Debug("removing a stargazer")
			if err = store.Delete(ctx, stargazer); err != nil {
				return fmt.Errorf("delete: %w", err)
			}
			return nil
		default:
			return fmt.Errorf("unsupported action: %s", stargazer.Action)
		}
	}
}
