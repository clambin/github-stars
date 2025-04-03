package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/clambin/github-stars/internal/server/mocks"
	"github.com/clambin/github-stars/internal/testutils"
	"github.com/google/go-github/v70/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWebhook(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		event github.StarEvent
		mocks func(s *mocks.Store, n *mocks.Notifier)
		want  int
	}{
		{
			name: "create (new)",
			event: github.StarEvent{
				Action:    testutils.Ptr("created"),
				StarredAt: &github.Timestamp{Time: time.Date(2024, time.November, 20, 21, 0, 0, 0, time.UTC)},
				Repo:      &github.Repository{Name: testutils.Ptr("foo")},
				Sender:    &github.User{Login: testutils.Ptr("snafu")},
			},
			mocks: func(s *mocks.Store, n *mocks.Notifier) {
				s.EXPECT().Add(mock.AnythingOfType("*github.Repository"), mock.AnythingOfType("*github.Stargazer")).Return(true, nil).Once()
				n.EXPECT().Notify(mock.AnythingOfType("*github.Repository"), mock.AnythingOfType("[]*github.Stargazer"), true).Return().Once()
			},
			want: http.StatusOK,
		},
		{
			name: "create (new) fails",
			event: github.StarEvent{
				Action:    testutils.Ptr("created"),
				StarredAt: &github.Timestamp{Time: time.Date(2024, time.November, 20, 21, 0, 0, 0, time.UTC)},
				Repo:      &github.Repository{Name: testutils.Ptr("foo")},
				Sender:    &github.User{Login: testutils.Ptr("snafu")},
			},
			mocks: func(s *mocks.Store, n *mocks.Notifier) {
				s.EXPECT().Add(mock.AnythingOfType("*github.Repository"), mock.AnythingOfType("*github.Stargazer")).Return(false, errors.New("failed")).Once()
			},
			want: http.StatusInternalServerError,
		},
		{
			name: "create (not new)",
			event: github.StarEvent{
				Action:    testutils.Ptr("created"),
				StarredAt: &github.Timestamp{Time: time.Date(2024, time.November, 20, 21, 0, 0, 0, time.UTC)},
				Repo:      &github.Repository{Name: testutils.Ptr("foo")},
				Sender:    &github.User{Login: testutils.Ptr("snafu")},
			},
			mocks: func(s *mocks.Store, n *mocks.Notifier) {
				s.EXPECT().Add(mock.AnythingOfType("*github.Repository"), mock.AnythingOfType("*github.Stargazer")).Return(false, nil).Once()
			},
			want: http.StatusOK,
		},
		{
			name: "delete (new)",
			event: github.StarEvent{
				Action: testutils.Ptr("deleted"),
				//StarredAt: &github.Timestamp{Time: time.Date(2024, time.November, 20, 21, 0, 0, 0, time.UTC)},
				Repo:   &github.Repository{Name: testutils.Ptr("foo")},
				Sender: &github.User{Login: testutils.Ptr("snafu")},
			},
			mocks: func(s *mocks.Store, n *mocks.Notifier) {
				s.EXPECT().Delete(mock.AnythingOfType("*github.Repository"), mock.AnythingOfType("*github.Stargazer")).Return(true, nil).Once()
				n.EXPECT().Notify(mock.AnythingOfType("*github.Repository"), mock.AnythingOfType("[]*github.Stargazer"), false).Return().Once()
			},
			want: http.StatusOK,
		},
		{
			name: "delete (new) fails",
			event: github.StarEvent{
				Action: testutils.Ptr("deleted"),
				//StarredAt: &github.Timestamp{Time: time.Date(2024, time.November, 20, 21, 0, 0, 0, time.UTC)},
				Repo:   &github.Repository{Name: testutils.Ptr("foo")},
				Sender: &github.User{Login: testutils.Ptr("snafu")},
			},
			mocks: func(s *mocks.Store, n *mocks.Notifier) {
				s.EXPECT().Delete(mock.AnythingOfType("*github.Repository"), mock.AnythingOfType("*github.Stargazer")).Return(false, errors.New("failed")).Once()
			},
			want: http.StatusInternalServerError,
		},
		{
			name: "delete (not new)",
			event: github.StarEvent{
				Action: testutils.Ptr("deleted"),
				//StarredAt: &github.Timestamp{Time: time.Date(2024, time.November, 20, 21, 0, 0, 0, time.UTC)},
				Repo:   &github.Repository{Name: testutils.Ptr("foo")},
				Sender: &github.User{Login: testutils.Ptr("snafu")},
			},
			mocks: func(s *mocks.Store, n *mocks.Notifier) {
				s.EXPECT().Delete(mock.AnythingOfType("*github.Repository"), mock.AnythingOfType("*github.Stargazer")).Return(false, nil).Once()
			},
			want: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifier := mocks.NewNotifier(t)
			store := mocks.NewStore(t)
			l := slog.New(slog.DiscardHandler)

			testServer := httptest.NewServer(GithubWebHookHandler(notifier, store, l))
			t.Cleanup(testServer.Close)

			if tt.mocks != nil {
				tt.mocks(store, notifier)
			}
			body, _ := json.Marshal(&tt.event)
			req, _ := http.NewRequest(http.MethodPost, testServer.URL+tt.path, bytes.NewReader(body))

			resp, err := http.DefaultClient.Do(req)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, resp.StatusCode)

		})
	}
}

func TestGitHubAuth(t *testing.T) {
	const validSecret = "valid-secret"

	tests := []struct {
		name   string
		secret string
		want   int
	}{
		{
			name:   "valid secret",
			secret: validSecret,
			want:   http.StatusOK,
		},
		{
			name:   "invalid secret",
			secret: "invalid-secret",
			want:   http.StatusUnauthorized,
		},
		{
			name: "no signature",
			want: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const sentBody = "hello world"
			testServer := httptest.NewServer(GitHubAuth(validSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				if bytes.Equal(body, []byte(sentBody)) {
					w.WriteHeader(http.StatusOK)
					return
				}
				w.WriteHeader(http.StatusBadRequest)
			})))
			t.Cleanup(testServer.Close)

			req, _ := http.NewRequest(http.MethodPost, testServer.URL, strings.NewReader(sentBody))
			if tt.secret != "" {
				req.Header.Set("X-Hub-Signature-256", calculateHMAC([]byte(sentBody), tt.secret))
			}

			resp, err := http.DefaultClient.Do(req)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, resp.StatusCode)
		})
	}
}
