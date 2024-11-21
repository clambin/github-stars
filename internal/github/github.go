package github

import (
	"context"
	"github.com/google/go-github/v66/github"
	"iter"
	"strings"
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

func (c Client) GetUserRepos(ctx context.Context, user string) iter.Seq2[*github.Repository, error] {
	return func(yield func(*github.Repository, error) bool) {
		listOptions := github.RepositoryListByUserOptions{ListOptions: github.ListOptions{PerPage: recordsPerPage}}
		for {
			var resp *github.Response
			// TODO: ListByAuthenticatedUser() could list all repos the user (token) has access to?
			repoPage, resp, err := c.Repositories.ListByUser(ctx, user, &listOptions)
			if err != nil {
				yield(nil, err)
				return
			}
			for _, repo := range repoPage {
				if !yield(repo, err) {
					return
				}
			}
			if resp.NextPage == 0 {
				return
			}
			listOptions.Page = resp.NextPage
		}
	}
}

func (c Client) GetStarGazers(ctx context.Context, repo *github.Repository) ([]*github.Stargazer, error) {
	var starGazers []*github.Stargazer
	listOptions := github.ListOptions{PerPage: recordsPerPage}

	user := strings.TrimSuffix(repo.GetFullName(), "/"+repo.GetName())

	for {
		page, resp, err := c.Activity.ListStargazers(ctx, user, repo.GetName(), &listOptions)
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
