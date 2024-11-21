package server

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/clambin/github-stars/internal/store"
	"github.com/google/go-github/v66/github"
	"io"
	"log/slog"
	"net/http"
)

type GitHubWebhook struct {
	Notifiers Notifier
	Store     Store
	Logger    *slog.Logger
}

var _ Store = &store.Store{}

func (l *GitHubWebhook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Parse the webhook payload
	var event github.StarEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		l.Logger.Error("Unable to parse StarEvent", "err", err)
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
		l.Logger.Debug("adding new stargazer", "repo", event.GetRepo().GetName(), "user", event.GetSender().GetLogin())
		added, err := l.Store.Add(event.GetRepo(), &stargazer)
		if err != nil {
			l.Logger.Error("Unable to store star", "err", err)
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		if added && l.Notifiers != nil {
			l.Notifiers.Notify(event.GetRepo(), []*github.Stargazer{&stargazer})
		}
	case "deleted":
		l.Logger.Debug("removing a stargazer", "repo", event.GetRepo().GetName(), "user", event.GetSender().GetLogin())
		deleted, err := l.Store.Delete(event.GetRepo(), &stargazer)
		if err != nil {
			l.Logger.Error("Unable to store star", "err", err)
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		if deleted && l.Notifiers != nil {
			l.Notifiers.Notify(event.GetRepo(), []*github.Stargazer{&stargazer})
		}
	default:
		l.Logger.Warn("Unsupported action", "action", event.GetAction())
		http.Error(w, "Unsupported action: "+event.GetAction(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func GitHubAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// read the request body, but make a copy, as we'll restore r.Body later
			var bodyCopy bytes.Buffer
			body, err := io.ReadAll(io.TeeReader(r.Body, &bodyCopy))
			if err != nil {
				http.Error(w, "Failed to read body", http.StatusInternalServerError)
				return
			}
			defer func() { _ = r.Body.Close() }()

			// validate GitHub signature
			signature := r.Header.Get("X-Hub-Signature-256")
			if !validateHMAC(body, signature, secret) {
				http.Error(w, "Invalid signature", http.StatusUnauthorized)
				return
			}

			// Restore the body and call the next handler
			r.Body = io.NopCloser(bytes.NewBuffer(body))
			next.ServeHTTP(w, r)
		})
	}
}

func calculateHMAC(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func validateHMAC(body []byte, signature, secret string) bool {
	expected := calculateHMAC(body, secret)
	return hmac.Equal([]byte(expected), []byte(signature))
}
