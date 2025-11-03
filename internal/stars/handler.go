package stars

import (
	"encoding/json"
	"net/http"

	"github.com/clambin/github-stars/slogctx"
	"github.com/google/go-github/v76/github"
)

func Handler(store *NotifyingStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get logger
		ctx := r.Context()
		logger := slogctx.FromContext(ctx)

		// Parse the webhook payload
		var event github.StarEvent
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			logger.Error("Unable to parse StarEvent", "err", err)
			http.Error(w, "Unable to parse payload", http.StatusInternalServerError)
			return
		}

		stargazer := github.Stargazer{
			StarredAt: event.StarredAt,
			User:      event.Sender,
		}

		// Handle the "star" event
		switch event.GetAction() {
		case "created":
			logger.Debug("adding new stargazer", "repo", event.GetRepo().GetName(), "user", event.GetSender().GetLogin())
			if err := store.Add(ctx, event.GetRepo(), &stargazer); err != nil {
				logger.Error("Unable to store star", "err", err)
				http.Error(w, "db error", http.StatusInternalServerError)
				return
			}
		case "deleted":
			logger.Debug("removing a stargazer", "repo", event.GetRepo().GetName(), "user", event.GetSender().GetLogin())
			if err := store.Delete(ctx, event.GetRepo(), &stargazer); err != nil {
				logger.Error("Unable to store star", "err", err)
				http.Error(w, "db error", http.StatusInternalServerError)
				return
			}
		default:
			logger.Warn("Unsupported action", "action", event.GetAction())
			http.Error(w, "Unsupported action: "+event.GetAction(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}
