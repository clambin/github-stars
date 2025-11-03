package stars

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/clambin/github-stars/slogctx"
	"github.com/google/go-github/v76/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandler(t *testing.T) {
	var s fakeSlackWebhook
	ts := httptest.NewServer(&s)
	t.Cleanup(ts.Close)

	notifiers := Notifiers{
		SlogNotifier{},
		SlackNotifier{WebHookURL: ts.URL},
	}
	store, err := NewNotifyingStore(t.TempDir(), notifiers)
	require.NoError(t, err)
	h := Handler(store)

	var logBuf bytes.Buffer
	ctx := slogctx.New(slogWithoutTime(&logBuf, slog.LevelInfo))

	// add a stargazer
	event := github.StarEvent{
		Action:    varPtr("created"),
		StarredAt: &github.Timestamp{Time: time.Now()},
		Repo:      &github.Repository{FullName: varPtr("foo/bar"), HTMLURL: varPtr("https://example.com/foo/bar")},
		Sender:    &github.User{Login: varPtr("snafu")},
	}
	var body bytes.Buffer
	require.NoError(t, json.NewEncoder(&body).Encode(event))
	req, _ := http.NewRequestWithContext(ctx, "POST", "/", &body)
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	// delete a stargazer
	event.Action = varPtr("deleted")
	body.Reset()
	require.NoError(t, json.NewEncoder(&body).Encode(event))
	req, _ = http.NewRequestWithContext(ctx, "POST", "/", &body)
	resp = httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	// check slog
	const slogWant = "level=INFO msg=\"repo has 1 new stargazers\" repository=foo/bar\nlevel=INFO msg=\"repo lost 1 stargazers\" repository=foo/bar\n"
	assert.Equal(t, slogWant, logBuf.String())
	// check slack
	slackWant := []string{
		"Repo <https://example.com/foo/bar|foo/bar> received a star from <|@snafu>",
		"Repo <https://example.com/foo/bar|foo/bar> lost a star from <|@snafu>",
	}
	assert.Equal(t, slackWant, s.received())
}
