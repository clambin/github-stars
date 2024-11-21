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

// A GitHubWebhook receives events from a Slack App. The current implementation is limited to StarEvents.
// Additionally, GitHubWebhook supports a readiness probe (/readyz), so an orchestrator can verify that
// the server is ready to process requests.
type GitHubWebhook struct {
	Notifiers Notifier
	Store     Store
	Logger    *slog.Logger
}

var _ Store = &store.Store{}

func (g *GitHubWebhook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/readyz":
		// Readiness probe
		w.WriteHeader(http.StatusOK)
	default:
		g.handleStarEvent(w, r)
	}
}

func (g *GitHubWebhook) handleStarEvent(w http.ResponseWriter, r *http.Request) {
	// Parse the webhook payload
	var event github.StarEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		g.Logger.Error("Unable to parse StarEvent", "err", err)
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
		g.Logger.Debug("adding new stargazer", "repo", event.GetRepo().GetName(), "user", event.GetSender().GetLogin())
		added, err := g.Store.Add(event.GetRepo(), &stargazer)
		if err != nil {
			g.Logger.Error("Unable to store star", "err", err)
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		if added && g.Notifiers != nil {
			g.Notifiers.Notify(event.GetRepo(), []*github.Stargazer{&stargazer})
		}
	case "deleted":
		g.Logger.Debug("removing a stargazer", "repo", event.GetRepo().GetName(), "user", event.GetSender().GetLogin())
		deleted, err := g.Store.Delete(event.GetRepo(), &stargazer)
		if err != nil {
			g.Logger.Error("Unable to store star", "err", err)
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		if deleted && g.Notifiers != nil {
			g.Notifiers.Notify(event.GetRepo(), []*github.Stargazer{&stargazer})
		}
	default:
		g.Logger.Warn("Unsupported action", "action", event.GetAction())
		http.Error(w, "Unsupported action: "+event.GetAction(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
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
