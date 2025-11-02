package github

import (
	"context"
	"errors"
	"github.com/google/go-github/v76/github"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestClient_GetUserRepoNames(t *testing.T) {
	client := NewGitHubClient("")
	client.Repositories = fakeRepositories{}

	var count int
	for _, err := range client.GetUserRepos(context.Background(), "") {
		if err != nil {
			t.Fatal(err)
		}
		count++
	}
	assert.Equal(t, 2, count)
}

func TestClient_GetUserStars(t *testing.T) {
	client := NewGitHubClient("")
	client.Activity = fakeActivity{}

	repo := github.Repository{FullName: ConstP("user/foo"), Name: ConstP("foo")}

	starGazers, err := client.GetStarGazers(context.Background(), &repo)
	assert.NoError(t, err)
	assert.Len(t, starGazers, 2)
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

type listResponse struct {
	repos []*github.Repository
	resp  *github.Response
}

var listResponses = map[int]listResponse{
	0: {
		repos: []*github.Repository{
			{
				FullName: ConstP("foo/foo"),
				Name:     ConstP("foo"),
			},
		},
		resp: &github.Response{NextPage: 1},
	},
	1: {
		repos: []*github.Repository{
			{
				FullName: ConstP("foo/bar"),
				Name:     ConstP("bar"),
			},
		},
		resp: &github.Response{NextPage: 0},
	},
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

var _ Activity = &fakeActivity{}

type fakeActivity struct{}

func (f fakeActivity) ListStargazers(_ context.Context, _ string, repo string, opts *github.ListOptions) ([]*github.Stargazer, *github.Response, error) {
	repoResps, ok := stargazerResponses[repo]
	if !ok {
		return nil, nil, errors.New("repo not found")
	}
	repoResp, ok := repoResps[opts.Page]
	if !ok {
		return nil, nil, errors.New("page not found")
	}
	return repoResp.gazers, repoResp.resp, nil
}

var stargazerResponses = map[string]map[int]stargazerResponse{
	"foo": {
		0: {
			gazers: []*github.Stargazer{{
				StarredAt: &github.Timestamp{Time: time.Date(2024, time.November, 19, 21, 30, 0, 0, time.UTC)},
				User:      &github.User{Login: ConstP("user1")},
			}},
			resp: &github.Response{NextPage: 1},
		},
		1: {
			gazers: []*github.Stargazer{{
				StarredAt: &github.Timestamp{Time: time.Date(2024, time.November, 19, 21, 30, 0, 0, time.UTC)},
				User:      &github.User{Login: ConstP("user2")},
			}},
			resp: &github.Response{NextPage: 0},
		},
	},
}

type stargazerResponse struct {
	gazers []*github.Stargazer
	resp   *github.Response
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func ConstP[T any](t T) *T {
	return &t
}
