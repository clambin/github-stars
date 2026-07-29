package server

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/clambin/github-stars/internal/github"
	ggh "github.com/google/go-github/v89/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer(t *testing.T) {
	const mySecret = "mySecret"
	store := fakeStore{
		stars: make(map[string]github.Stargazer),
	}
	h := New(&store, mySecret, slog.New(slog.DiscardHandler))

	// add
	req := buildRequest(t, ggh.StarEvent{
		Action:    new("created"),
		StarredAt: &ggh.Timestamp{Time: time.Date(2025, time.November, 7, 21, 30, 0, 0, time.UTC)},
		Repo:      &ggh.Repository{FullName: new("foo/bar")},
		Sender:    &ggh.User{Login: new("user1")},
	}, mySecret)
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)

	store.mu.Lock()
	assert.Len(t, store.stars, 1)
	assert.Equal(t, "foo/bar", store.stars["user1"].RepoName)
	store.mu.Unlock()

	req = buildRequest(t, ggh.StarEvent{
		Action: new("deleted"),
		Repo:   &ggh.Repository{FullName: new("foo/bar")},
		Sender: &ggh.User{Login: new("user1")},
	}, mySecret)
	resp = httptest.NewRecorder()
	h.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)

	store.mu.Lock()
	assert.Empty(t, store.stars)
	store.mu.Unlock()
}

func buildRequest(t *testing.T, evt ggh.StarEvent, secret string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, json.NewEncoder(&buf).Encode(evt))

	mac := calculateHMAC(buf.Bytes(), secret)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "/", &buf)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "GitHub-Hookshot/1.0")
	req.Header.Set("X-Hub-Signature-256", "sha256="+mac)
	req.Header.Set("X-GitHub-Event", "star")

	return req
}

func calculateHMAC(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

var _ StarStore = (*fakeStore)(nil)

type fakeStore struct {
	stars map[string]github.Stargazer
	mu    sync.Mutex
}

func (f *fakeStore) Add(_ context.Context, stargazer ...github.Stargazer) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range stargazer {
		f.stars[s.Login] = s
	}
	return nil
}

func (f *fakeStore) Delete(_ context.Context, stargazer ...github.Stargazer) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range stargazer {
		delete(f.stars, s.Login)
	}
	return nil
}
