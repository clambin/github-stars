package listener

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
)

type Listener struct {
	Secret string
	Logger *slog.Logger
}

type StarPayload struct {
	Action string `json:"action"`
	Repo   struct {
		Name  string `json:"name"`
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
	} `json:"repository"`
	Sender struct {
		Login string `json:"login"`
	} `json:"sender"`
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

	l.Logger.Debug("received request", "header", r.Header, "body", string(body))

	// Verify webhook signature
	signature := r.Header.Get("X-Hub-Signature-256")
	if !l.verifySignature(body, signature) {
		l.Logger.Error("invalid signature", "signature", signature)
		http.Error(w, "Invalid signature", http.StatusForbidden)
		return
	}

	// Parse the webhook payload
	var payload StarPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		l.Logger.Error("Unable to parse request body", "err", err)
		http.Error(w, "Unable to parse payload", http.StatusInternalServerError)
		return
	}

	// Handle the "star" event
	if payload.Action == "created" {
		l.Logger.Info("Repo received a star",
			"repo", payload.Repo.Name,
			"owner", payload.Repo.Owner.Login,
			"giver", payload.Sender.Login,
		)
	}

	w.WriteHeader(http.StatusOK)
}
