package github

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-github/v77/github"
	"github.com/stretchr/testify/require"
)

func TestNewStarEventWebhook(t *testing.T) {
	const secret = "secret"
	tests := []struct {
		name           string
		eventType      string
		event          any
		secret         string
		want           Stargazer
		wantStatusCode int
	}{
		{
			name:           "invalid signature",
			eventType:      "star",
			event:          github.StarEvent{},
			secret:         "invalid-secret",
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name:           "unsupported event type",
			eventType:      "fork",
			event:          github.ForkEvent{},
			secret:         secret,
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name:           "invalid event",
			eventType:      "star",
			event:          "invalid",
			secret:         secret,
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name:      "add",
			eventType: "star",
			event: github.StarEvent{
				Action:    github.Ptr("created"),
				StarredAt: &github.Timestamp{Time: time.Date(2025, time.November, 7, 21, 30, 0, 0, time.UTC)},
				Repo:      &github.Repository{FullName: github.Ptr("foo/bar")},
				Sender:    &github.User{Login: github.Ptr("user1")},
			},
			secret: secret,
			want: Stargazer{
				Action:    "created",
				RepoName:  "foo/bar",
				Login:     "user1",
				StarredAt: time.Date(2025, time.November, 7, 21, 30, 0, 0, time.UTC),
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name:      "delete",
			eventType: "star",
			event: github.StarEvent{
				Action: github.Ptr("deleted"),
				Repo:   &github.Repository{FullName: github.Ptr("foo/bar")},
				Sender: &github.User{Login: github.Ptr("user1")},
			},
			secret: secret,
			want: Stargazer{
				Action:   "deleted",
				RepoName: "foo/bar",
				Login:    "user1",
			},
			wantStatusCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlers := WebhookHandlers{
				StarEvent: func(_ context.Context, stargazer Stargazer) error {
					if stargazer != tt.want {
						return fmt.Errorf("got %v, want %v", stargazer, tt.want)
					}
					return nil
				},
			}
			h := WebhookHandler(handlers, secret, slog.New(slog.DiscardHandler))

			var buf bytes.Buffer
			_ = json.NewEncoder(&buf).Encode(tt.event)
			mac := calculateHMAC(buf.Bytes(), tt.secret)

			req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, "/", &buf)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("User-Agent", "GitHub-Hookshot/1.0")
			req.Header.Set("X-Hub-Signature-256", mac)
			req.Header.Set("X-GitHub-Event", tt.eventType)
			resp := httptest.NewRecorder()
			h.ServeHTTP(resp, req)
			require.Equal(t, tt.wantStatusCode, resp.Code)
		})
	}
}

func calculateHMAC(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
