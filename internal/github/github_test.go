package github

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-github/v77/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Stars(t *testing.T) {
	client := NewGitHubClient("")
	client.Repositories = fakeRepositories{}
	client.Activity = fakeActivity{}

	stars, err := client.Stargazers(context.Background(), "bar", true)
	require.NoError(t, err)

	want := []Stargazer{
		{RepoName: "foo/foo", Login: "user1", StarredAt: time.Date(2024, time.November, 19, 21, 30, 0, 0, time.UTC)},
		{RepoName: "foo/foo", Login: "user2", StarredAt: time.Date(2024, time.November, 19, 21, 30, 0, 0, time.UTC)},
	}

	assert.Equal(t, want, stars)
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

var _ Repositories = &fakeRepositories{}

type fakeRepositories struct{}

func (f fakeRepositories) ListByUser(_ context.Context, _ string, opts *github.RepositoryListByUserOptions) ([]*github.Repository, *github.Response, error) {
	resp, ok := listResponses[opts.Page]
	if !ok {
		return nil, nil, errors.New("page not found")
	}

	return resp.repos, resp.resp, nil
}

type repoResponsePage struct {
	repos []*github.Repository
	resp  *github.Response
}

var listResponses = map[int]repoResponsePage{
	0: {
		repos: []*github.Repository{{FullName: varP("foo/foo"), Name: varP("foo")}},
		resp:  &github.Response{NextPage: 1},
	},
	1: {
		repos: []*github.Repository{{FullName: varP("foo/bar"), Name: varP("bar")}},
		resp:  &github.Response{NextPage: 0},
	},
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

var _ Activity = &fakeActivity{}

type fakeActivity struct{}

func (f fakeActivity) ListStargazers(_ context.Context, _ string, repo string, opts *github.ListOptions) ([]*github.Stargazer, *github.Response, error) {
	repoResps, ok := listStargazersResponses[repo]
	if !ok {
		return nil, nil, fmt.Errorf("repo not found: %s", repo)
	}
	repoResp, ok := repoResps[opts.Page]
	if !ok {
		return nil, nil, fmt.Errorf("page not found: %s/%d", repo, opts.Page)
	}
	return repoResp.gazers, repoResp.resp, nil
}

var listStargazersResponses = map[string]map[int]stargazerResponse{
	"foo": {
		0: {
			gazers: []*github.Stargazer{{
				StarredAt: &github.Timestamp{Time: time.Date(2024, time.November, 19, 21, 30, 0, 0, time.UTC)},
				User:      &github.User{Login: varP("user1")},
			}},
			resp: &github.Response{NextPage: 1},
		},
		1: {
			gazers: []*github.Stargazer{{
				StarredAt: &github.Timestamp{Time: time.Date(2024, time.November, 19, 21, 30, 0, 0, time.UTC)},
				User:      &github.User{Login: varP("user2")},
			}},
			resp: &github.Response{NextPage: 0},
		},
	},
	"bar": {
		0: {resp: &github.Response{NextPage: 0}},
	},
}

type stargazerResponse struct {
	gazers []*github.Stargazer
	resp   *github.Response
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func varP[T any](t T) *T {
	return &t
}
