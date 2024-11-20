package listener

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/google/go-github/v66/github"
	"io"
	"log/slog"
	"net/http"
)

type Listener struct {
	Secret string
	Logger *slog.Logger
}

func (l *Listener) verifySignature(payload []byte, signature string) bool {
	h := hmac.New(sha256.New, []byte(l.Secret))
	h.Write(payload)
	expected := "sha256=" + hex.EncodeToString(h.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

func (l *Listener) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Unable to read request body", http.StatusInternalServerError)
		return
	}

	l.Logger.Debug("received request", "req", r.URL.String(), "header", r.Header, "body", string(body))

	// Verify webhook signature
	signature := r.Header.Get("X-Hub-Signature-256")
	if !l.verifySignature(body, signature) {
		l.Logger.Error("invalid signature", "signature", signature)
		http.Error(w, "Invalid signature", http.StatusForbidden)
		return
	}

	// Parse the webhook payload
	var event github.StarEvent
	if err := json.Unmarshal(body, &event); err != nil {
		l.Logger.Error("Unable to parse request body", "err", err)
		http.Error(w, "Unable to parse payload", http.StatusInternalServerError)
		return
	}

	// Handle the "star" event
	if event.GetAction() == "created" {
		l.Logger.Info("Repo received a star",
			"repo", event.GetRepo().GetFullName(),
			"owner", event.GetRepo().GetOwner().GetLogin(),
			"giver", event.GetSender().GetLogin(),
			"date", event.GetStarredAt().Time,
		)
	}

	w.WriteHeader(http.StatusOK)
}
