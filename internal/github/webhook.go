package github

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/clambin/github-stars/slogctx"
	"github.com/google/go-github/v89/github"
)

type StarEventFunc func(context.Context, Stargazer) error

// WebhookHandler receives GitHub events through a webhook.
//
// Currently only supports the "star" event.
type WebhookHandler struct {
	Secret    string
	StarEvent StarEventFunc
	Logger    *slog.Logger
}

// Has returns true if the handler for the given event is defined.
func (h WebhookHandler) Has(evt string) bool {
	switch evt {
	case "star":
		return h.StarEvent != nil
	default:
		return false
	}
}

func (h WebhookHandler) Handler() http.Handler {
	return withLogger(h.Logger)(
		http.HandlerFunc(h.serveWebhook),
	)
}

func (h WebhookHandler) serveWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := slogctx.FromContext(ctx)
	logger.Debug("webhook call received")
	// check the signature
	payload, err := github.ValidatePayload(r, []byte(h.Secret))
	if err != nil {
		logger.Error("Unable to validate payload", "err", err)
		http.Error(w, "invalid payload", http.StatusUnauthorized)
		return
	}
	// check that we have a handler for this type of event
	webhookType := github.WebHookType(r)
	if !h.Has(webhookType) {
		logger.Error("Unsupported webhook type", "type", webhookType)
		http.Error(w, "unsupported webhook type", http.StatusBadRequest)
	}
	// parse the payload
	logger.Debug("webhook validated", "type", webhookType)
	req, err := github.ParseWebHook(webhookType, payload)
	if err != nil {
		logger.Error("Unable to parse webhook", "err", err)
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	// handle the event
	switch evt := req.(type) {
	case *github.StarEvent:
		stargazer := Stargazer{
			Action:      evt.GetAction(),
			RepoName:    evt.Repo.GetFullName(),
			RepoHTMLURL: evt.Repo.GetHTMLURL(),
			Login:       evt.Sender.GetLogin(),
			UserHTMLURL: evt.Sender.GetHTMLURL(),
			StarredAt:   evt.GetStarredAt().Time,
		}
		if err = h.StarEvent(ctx, stargazer); err != nil {
			logger.Error("Unable to handle StarEvent", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

// withLogger returns an HTTP middleware that adds a GitHub webhook-aware logger to the context of the request.
func withLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	headers := []struct {
		httpHeader string
		slogField  string
	}{
		{"X-GitHub-Hook-ID", "hook_id"},
		{"X-GitHub-Event", "event"},
		{"User-Agent", "user_agent"},
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// if any of the defined headers are set, add them to the logger
			l := logger
			for _, h := range headers {
				if v := r.Header.Get(h.httpHeader); v != "" {
					l = l.With(slog.String(h.slogField, v))
				}
			}
			// call the next handler, adding the revised logger to the context
			next.ServeHTTP(w, r.WithContext(slogctx.NewWithContext(r.Context(), l)))
		})
	}
}
