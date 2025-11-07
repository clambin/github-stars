package github

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/clambin/github-stars/slogctx"
	"github.com/stretchr/testify/assert"
)

func TestWebHook(t *testing.T) {
	const authSecret = "auth-secret"
	tests := []struct {
		name   string
		method string
		path   string
		body   string
		secret string
		want   int
	}{
		{"valid signature", http.MethodPost, "/", "hello world", authSecret, http.StatusOK},
		{"invalid signature", http.MethodPost, "/", "hello world", "invalid-secret", http.StatusUnauthorized},
		{"invalid method", http.MethodGet, "/", "hello world", authSecret, http.StatusMethodNotAllowed},
		{"readyz", http.MethodGet, "/readyz", "", "secret-not-used-so-this-can-be-false", http.StatusOK},
		{"readyz invalid method", http.MethodPost, "/readyz", "", "secret-not-used-so-this-can-be-false", http.StatusUnauthorized}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if logger := slogctx.FromContext(r.Context()); logger == nil {
					http.Error(w, "logger should not have been set", http.StatusInternalServerError)
					return
				}
				if userAgent := r.Header.Get("User-Agent"); userAgent != "GitHub-Hookshot/1.0" {
					http.Error(w, "unexpected user agent", http.StatusBadRequest)
					return
				}
				body, _ := io.ReadAll(r.Body)
				if string(body) != tt.body {
					http.Error(w, "unexpected body", http.StatusBadRequest)
					return
				}
				w.WriteHeader(http.StatusOK)
			})
			h := NewWebhook(authSecret, handler, slog.New(slog.DiscardHandler))
			req, _ := http.NewRequestWithContext(t.Context(), tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("X-Hub-Signature-256", calculateHMAC([]byte(tt.body), tt.secret))
			req.Header.Set("User-Agent", "GitHub-Hookshot/1.0")
			resp := httptest.NewRecorder()
			h.ServeHTTP(resp, req)
			assert.Equal(t, tt.want, resp.Code)
		})
	}
}
