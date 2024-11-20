package server

import (
	"context"
	"github.com/google/go-github/v66/github"
	"iter"
)

type Client interface {
	GetUserRepos(ctx context.Context, user string) iter.Seq2[*github.Repository, error]
	GetStarGazers(ctx context.Context, repository *github.Repository) ([]*github.Stargazer, error)
}

type Store interface {
	SetStargazers(repository *github.Repository, stargazers []*github.Stargazer) ([]*github.Stargazer, error)
	Add(repo *github.Repository, stargazer *github.Stargazer) (bool, error)
	Delete(repo *github.Repository, stargazer *github.Stargazer) (bool, error)
}

type Notifier interface {
	Notify(repository *github.Repository, gazers []*github.Stargazer)
}
