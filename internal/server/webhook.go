package server

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/google/go-github/v76/github"
)

// A GithubWebHookHandler receives events from a Slack App. The current implementation is limited to StarEvents.
func GithubWebHookHandler(notifier Notifier, store Store, l *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse the webhook payload
		var event github.StarEvent
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			l.Error("Unable to parse StarEvent", "err", err)
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
			l.Debug("adding new stargazer", "repo", event.GetRepo().GetName(), "user", event.GetSender().GetLogin())
			added, err := store.Add(event.GetRepo(), &stargazer)
			if err != nil {
				l.Error("Unable to store star", "err", err)
				http.Error(w, "db error", http.StatusInternalServerError)
				return
			}
			if added && notifier != nil {
				notifier.Notify(event.GetRepo(), []*github.Stargazer{&stargazer}, true)
			}
		case "deleted":
			l.Debug("removing a stargazer", "repo", event.GetRepo().GetName(), "user", event.GetSender().GetLogin())
			deleted, err := store.Delete(event.GetRepo(), &stargazer)
			if err != nil {
				l.Error("Unable to store star", "err", err)
				http.Error(w, "db error", http.StatusInternalServerError)
				return
			}
			if deleted && notifier != nil {
				notifier.Notify(event.GetRepo(), []*github.Stargazer{&stargazer}, false)
			}
		default:
			l.Warn("Unsupported action", "action", event.GetAction())
			http.Error(w, "Unsupported action: "+event.GetAction(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// GitHubAuth is a HTTP middleware that verifies the HMAC SHA-265 signature sent by GitHub.
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
