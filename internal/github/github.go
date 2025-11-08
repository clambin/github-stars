package github

import (
	"context"
	"strings"
	"time"

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

// Stargazer represents a star from one user for one repository.
type Stargazer struct {
	StarredAt   time.Time
	Action      string `json:"-"`
	RepoName    string
	RepoHTMLURL string
	Login       string
	UserHTMLURL string
}

// Stargazers returns the list of stargazers for a user's repositories.
// If includeArchived is true, archived repositories are included.
func (c Client) Stargazers(ctx context.Context, user string, includeArchived bool) ([]Stargazer, error) {
	var stargazers []Stargazer

	repos, err := c.userRepos(ctx, user)
	if err != nil {
		return nil, err
	}
	for _, repo := range repos {
		if repo.GetArchived() && !includeArchived {
			continue
		}
		gazers, err := c.starGazers(ctx, repo)
		if err != nil {
			return nil, err
		}
		for _, gazer := range gazers {
			stargazers = append(stargazers, Stargazer{
				RepoName:    repo.GetFullName(),
				RepoHTMLURL: repo.GetHTMLURL(),
				Login:       gazer.GetUser().GetLogin(),
				UserHTMLURL: gazer.GetUser().GetHTMLURL(),
				StarredAt:   gazer.GetStarredAt().Time,
			})
		}
	}
	return stargazers, nil
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
