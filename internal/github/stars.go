package github

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/go-github/v77/github"
)

// Stargazer represents a star from one user for one repository.
type Stargazer struct {
	Action      string
	RepoName    string
	RepoHTMLURL string
	Login       string
	UserHTMLURL string
	StarredAt   time.Time
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

// NewStarEventWebhook creates a new GitHub webhook handler for star events
func NewStarEventWebhook(secret string, h func(context.Context, Stargazer) error, logger *slog.Logger) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var evt github.StarEvent
		if err := json.NewDecoder(r.Body).Decode(&evt); err != nil {
			logger.Error("Unable to parse StarEvent", "err", err)
			http.Error(w, "invalid json", http.StatusBadRequest)
		}
		stargazer := Stargazer{
			Action:      evt.GetAction(),
			RepoName:    evt.Repo.GetFullName(),
			RepoHTMLURL: evt.Repo.GetHTMLURL(),
			Login:       evt.Sender.GetLogin(),
			UserHTMLURL: evt.Sender.GetHTMLURL(),
			StarredAt:   evt.GetStarredAt().Time,
		}
		if err := h(r.Context(), stargazer); err != nil {
			logger.Error("Unable to handle StarEvent", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	return NewWebhook(secret, handler, logger)
}
