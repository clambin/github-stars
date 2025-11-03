package github

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"

	"github.com/clambin/github-stars/slogctx"
)

func NewWebHook(secret string, h http.Handler, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("POST /",
		withGitHubAuth(secret)(
			withLogger(logger)(
				h,
			),
		),
	)
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return mux
}

// withLogger returns an HTTP middleware that adds a logger to the context of the request.
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

// withGitHubAuth returns an HTTP middleware that verifies the HMAC SHA-265 signature sent by GitHub.
func withGitHubAuth(secret string) func(http.Handler) http.Handler {
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
