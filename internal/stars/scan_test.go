package stars

import (
	"bytes"
	"context"
	"fmt"
	"iter"
	"log/slog"
	"testing"
	"time"

	"github.com/clambin/github-stars/slogctx"
	"github.com/google/go-github/v76/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScan(t *testing.T) {
	c := fakeClient{
		repos: map[string][]*github.Repository{
			"user1": {
				{FullName: varPtr("user1/foo")},
				{FullName: varPtr("user1/bar")},
			},
		},
		stargazers: map[string][]*github.Stargazer{
			"user1/foo": {
				{
					StarredAt: &github.Timestamp{Time: time.Date(2024, time.November, 20, 8, 0, 0, 0, time.UTC)},
					User:      &github.User{Login: varPtr("user1")},
				},
			},
			"user1/bar": {
				{
					StarredAt: &github.Timestamp{Time: time.Date(2024, time.November, 20, 8, 0, 0, 0, time.UTC)},
					User:      &github.User{Login: varPtr("user1")},
				},
			},
		},
	}

	var buf bytes.Buffer
	ctx := slogctx.NewWithContext(t.Context(), slogWithoutTime(&buf, slog.LevelInfo))
	store, err := NewNotifyingStore(t.TempDir(), Notifiers{SlogNotifier{}})
	require.NoError(t, err)

	require.NoError(t, Scan(ctx, "user1", &c, store, false))

	assert.Contains(t, buf.String(), "level=INFO msg=\"repo has 1 new stargazers\" repository=user1/foo\n")
	assert.Contains(t, buf.String(), "level=INFO msg=\"repo has 1 new stargazers\" repository=user1/bar\n")
}

var _ Client = fakeClient{}

type fakeClient struct {
	repos      map[string][]*github.Repository
	stargazers map[string][]*github.Stargazer
}

func (f fakeClient) GetUserRepos(_ context.Context, user string) iter.Seq2[*github.Repository, error] {
	r, ok := f.repos[user]
	return func(yield func(*github.Repository, error) bool) {
		if !ok {
			yield(nil, fmt.Errorf("user %q not found", user))
			return
		}
		for _, repo := range r {
			if !yield(repo, nil) {
				return
			}
		}
	}
}

func (f fakeClient) GetStarGazers(_ context.Context, repository *github.Repository) ([]*github.Stargazer, error) {
	return f.stargazers[repository.GetFullName()], nil
}
