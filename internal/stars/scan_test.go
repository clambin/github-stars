package stars

import (
	"bytes"
	"context"
	"log/slog"
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

var _ Client = fakeClient{}

type fakeClient struct {
	stargazers []github.Stargazer
}

func (f fakeClient) Stargazers(context.Context, string, bool) ([]github.Stargazer, error) {
	return f.stargazers, nil
}
