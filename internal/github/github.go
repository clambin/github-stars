package github

import (
	"context"
	"strings"

	"github.com/google/go-github/v77/github"
)

type Client struct {
	Repositories
	Activity
}

type Repositories interface {
	ListByUser(ctx context.Context, user string, opts *github.RepositoryListByUserOptions) ([]*github.Repository, *github.Response, error)
}

type Activity interface {
	ListStargazers(ctx context.Context, owner string, repo string, opts *github.ListOptions) ([]*github.Stargazer, *github.Response, error)
}

func NewGitHubClient(token string) *Client {
	client := github.NewClient(nil).WithAuthToken(token)
	return &Client{
		Repositories: client.Repositories,
		Activity:     client.Activity,
	}
}

const recordsPerPage = 100

func (c Client) userRepos(ctx context.Context, user string) ([]*github.Repository, error) {
	var repos []*github.Repository
	listOptions := github.RepositoryListByUserOptions{ListOptions: github.ListOptions{PerPage: recordsPerPage}}

	for {
		var resp *github.Response
		// TODO: ListByAuthenticatedUser() could list all repos the user (token) has access to?
		repoPage, resp, err := c.ListByUser(ctx, user, &listOptions)
		if err != nil {
			return nil, err
		}
		repos = append(repos, repoPage...)
		if resp.NextPage == 0 {
			return repos, nil
		}
		listOptions.Page = resp.NextPage
	}
}

func (c Client) starGazers(ctx context.Context, repo *github.Repository) ([]*github.Stargazer, error) {
	var starGazers []*github.Stargazer
	listOptions := github.ListOptions{PerPage: recordsPerPage}

	// repo.Owner.GetLogin() ???
	user := strings.TrimSuffix(repo.GetFullName(), "/"+repo.GetName())

	for {
		page, resp, err := c.ListStargazers(ctx, user, repo.GetName(), &listOptions)
		if err != nil {
			return nil, err
		}
		starGazers = append(starGazers, page...)
		if resp.NextPage == 0 {
			return starGazers, nil
		}
		listOptions.Page = resp.NextPage
	}
}
