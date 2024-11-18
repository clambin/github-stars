package stars

import (
	"context"
	"github.com/google/go-github/v65/github"
	"iter"
)

type Repositories interface {
	ListByUser(ctx context.Context, user string, opts *github.RepositoryListByUserOptions) ([]*github.Repository, *github.Response, error)
}

type Activity interface {
	ListStargazers(ctx context.Context, owner string, repo string, opts *github.ListOptions) ([]*github.Stargazer, *github.Response, error)
}

type Client struct {
	Repositories
	Activity
}

func New(client *github.Client) *Client {
	return &Client{
		Repositories: client.Repositories,
		Activity:     client.Activity,
	}
}

const recordsPerPage = 100

func (c Client) GetUserRepoNames(ctx context.Context, user string) iter.Seq2[*github.Repository, error] {
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

func (c Client) GetStarGazers(ctx context.Context, user string, repo string) iter.Seq2[*github.Stargazer, error] {
	return func(yield func(*github.Stargazer, error) bool) {
		listOptions := github.ListOptions{PerPage: recordsPerPage}
		for {
			page, resp, err := c.Activity.ListStargazers(ctx, user, repo, &listOptions)
			if err != nil {
				yield(nil, err)
				return
			}
			for _, gazer := range page {
				if !yield(gazer, err) {
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
