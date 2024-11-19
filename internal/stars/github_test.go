package stars

import (
	"context"
	"errors"
	ggh "github.com/google/go-github/v65/github"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestClient_GetUserRepoNames(t *testing.T) {
	client := Client{Repositories: fakeRepositories{}}

	var count int
	for _, err := range client.GetUserRepoNames(context.Background(), "") {
		if err != nil {
			t.Fatal(err)
		}
		count++
	}
	assert.Equal(t, 2, count)
}

func TestClient_GetUserStars(t *testing.T) {
	client := Client{Activity: fakeActivity{}}

	var count int
	for _, err := range client.GetStarGazers(context.Background(), "", "foo") {
		if err != nil {
			t.Fatal(err)
		}
		count++
	}
	assert.Equal(t, 2, count)
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

var _ Repositories = &fakeRepositories{}

type fakeRepositories struct{}

func (f fakeRepositories) ListByUser(_ context.Context, _ string, opts *ggh.RepositoryListByUserOptions) ([]*ggh.Repository, *ggh.Response, error) {
	resp, ok := listResponses[opts.Page]
	if !ok {
		return nil, nil, errors.New("page not found")
	}

	return resp.repos, resp.resp, nil
}

type listResponse struct {
	repos []*ggh.Repository
	resp  *ggh.Response
}

var listResponses = map[int]listResponse{
	0: {
		repos: []*ggh.Repository{
			{
				FullName: ConstP("foo/foo"),
				Name:     ConstP("foo"),
			},
		},
		resp: &ggh.Response{NextPage: 1},
	},
	1: {
		repos: []*ggh.Repository{
			{
				FullName: ConstP("foo/bar"),
				Name:     ConstP("bar"),
			},
		},
		resp: &ggh.Response{NextPage: 0},
	},
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

var _ Activity = &fakeActivity{}

type fakeActivity struct{}

func (f fakeActivity) ListStargazers(_ context.Context, _ string, repo string, opts *ggh.ListOptions) ([]*ggh.Stargazer, *ggh.Response, error) {
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
			gazers: []*ggh.Stargazer{{
				StarredAt: &ggh.Timestamp{Time: time.Date(2024, time.November, 19, 21, 30, 0, 0, time.UTC)},
				User:      &ggh.User{Login: ConstP("user1")},
			}},
			resp: &ggh.Response{NextPage: 1},
		},
		1: {
			gazers: []*ggh.Stargazer{{
				StarredAt: &ggh.Timestamp{Time: time.Date(2024, time.November, 19, 21, 30, 0, 0, time.UTC)},
				User:      &ggh.User{Login: ConstP("user2")},
			}},
			resp: &ggh.Response{NextPage: 0},
		},
	},
}

type stargazerResponse struct {
	gazers []*ggh.Stargazer
	resp   *ggh.Response
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func ConstP[T any](t T) *T {
	return &t
}
