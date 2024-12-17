package server

import (
	"context"
	"fmt"
	"github.com/clambin/github-stars/internal/server/mocks"
	"github.com/clambin/github-stars/internal/testutils"
	"github.com/google/go-github/v67/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"io"
	"iter"
	"log/slog"
	"testing"
	"time"
)

func TestScan(t *testing.T) {
	ctx := context.Background()

	c := mocks.NewClient(t)
	c.EXPECT().
		GetUserRepos(ctx, "user").
		Return(repoIterate([]*github.Repository{
			{FullName: testutils.Ptr("user/foo"), Name: testutils.Ptr("foo")},
			{FullName: testutils.Ptr("user/bar"), Name: testutils.Ptr("bar"), Archived: testutils.Ptr(true)},
		})).
		Once()
	c.EXPECT().
		GetStarGazers(ctx, mock.AnythingOfType("*github.Repository")).
		Return([]*github.Stargazer{
			{
				StarredAt: &github.Timestamp{Time: time.Date(2024, time.November, 23, 30, 0, 0, 0, time.UTC)},
				User:      &github.User{Login: testutils.Ptr("stargazer")},
			}}, nil).
		Once()

	s := mocks.NewStore(t)
	s.EXPECT().
		SetStargazers(mock.AnythingOfType("*github.Repository"), mock.AnythingOfType("[]*github.Stargazer")).
		RunAndReturn(func(repository *github.Repository, stargazers []*github.Stargazer) ([]*github.Stargazer, error) {
			if repository.GetFullName() != "user/foo" {
				return nil, fmt.Errorf("unexpected repository full name")
			}
			if len(stargazers) != 1 {
				return nil, fmt.Errorf("unexpected number of stargazers")
			}
			if stargazers[0].User.GetLogin() != "stargazer" {
				return nil, fmt.Errorf("unexpected stargazers")
			}
			return stargazers, nil
		}).
		Once()

	n := mocks.NewNotifier(t)
	n.EXPECT().
		Notify(mock.AnythingOfType("*github.Repository"), mock.AnythingOfType("[]*github.Stargazer"), true).Once()

	l := slog.New(slog.NewTextHandler(io.Discard, nil))

	err := Scan(ctx, "user", c, s, n, false, l)
	assert.NoError(t, err)
}

func repoIterate(repos []*github.Repository) iter.Seq2[*github.Repository, error] {
	return func(yield func(*github.Repository, error) bool) {
		for _, repo := range repos {
			if !yield(repo, nil) {
				return
			}
		}
	}
}
