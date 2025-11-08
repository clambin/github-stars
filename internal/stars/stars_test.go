package stars

import (
	"bytes"
	"context"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/clambin/github-stars/internal/github"
	"github.com/clambin/github-stars/slogctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScan(t *testing.T) {
	c := fakeClient{
		stargazers: []github.Stargazer{
			{"", "user1/foo", "", "user1", "", time.Date(2024, time.November, 20, 8, 0, 0, 0, time.UTC)},
			{"", "user1/bar", "", "user1", "", time.Date(2024, time.November, 20, 8, 0, 0, 0, time.UTC)},
		},
	}

	var buf bytes.Buffer
	ctx := slogctx.NewWithContext(t.Context(), slogWithoutTime(&buf, slog.LevelInfo))
	store, err := NewNotifyingStore(t.TempDir(), Notifiers{SlogNotifier{}})
	require.NoError(t, err)

	require.NoError(t, Scan(ctx, "user1", &c, store, false))

	assert.Contains(t, buf.String(), "level=INFO msg=\"repo has 1 new stargazers\" repo=user1/foo\n")
	assert.Contains(t, buf.String(), "level=INFO msg=\"repo has 1 new stargazers\" repo=user1/bar\n")
}

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
	event := github.Stargazer{
		Action:      "created",
		RepoName:    "foo/bar",
		RepoHTMLURL: "https://example.com/foo/bar",
		Login:       "snafu",
		UserHTMLURL: "https://example.com/snafu",
		StarredAt:   time.Now(),
	}
	require.NoError(t, h(ctx, event))

	// delete a stargazer
	event.Action = "deleted"
	require.NoError(t, h(ctx, event))

	// check slog
	const slogWant = "level=INFO msg=\"repo has 1 new stargazers\" repo=foo/bar\nlevel=INFO msg=\"repo lost 1 stargazers\" repo=foo/bar\n"
	assert.Equal(t, slogWant, logBuf.String())
	// check Slack
	slackWant := []string{
		"Repo <https://example.com/foo/bar|foo/bar> received a star from <https://example.com/snafu|@snafu>",
		"Repo <https://example.com/foo/bar|foo/bar> lost a star from <https://example.com/snafu|@snafu>",
	}
	assert.Equal(t, slackWant, s.received())
}

var _ Client = fakeClient{}

type fakeClient struct {
	stargazers []github.Stargazer
}

func (f fakeClient) Stargazers(context.Context, string, bool) ([]github.Stargazer, error) {
	return f.stargazers, nil
}
