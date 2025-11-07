package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
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

func TestNewStarEventWebhook(t *testing.T) {
	tests := []struct {
		name           string
		event          github.StarEvent
		want           Stargazer
		wantStatusCode int
	}{
		{
			name: "add",
			event: github.StarEvent{
				Action:    varP("created"),
				StarredAt: &github.Timestamp{Time: time.Date(2025, time.November, 7, 21, 30, 0, 0, time.UTC)},
				Repo:      &github.Repository{FullName: varP("foo/bar")},
				Sender:    &github.User{Login: varP("user1")},
			},
			want: Stargazer{
				Action:      "created",
				RepoName:    "foo/bar",
				RepoHTMLURL: "",
				Login:       "user1",
				UserHTMLURL: "",
				StarredAt:   time.Date(2025, time.November, 7, 21, 30, 0, 0, time.UTC),
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name: "delete",
			event: github.StarEvent{
				Action: varP("deleted"),
				Repo:   &github.Repository{FullName: varP("foo/bar")},
				Sender: &github.User{Login: varP("user1")},
			},
			want: Stargazer{
				Action:      "deleted",
				RepoName:    "foo/bar",
				RepoHTMLURL: "",
				Login:       "user1",
				UserHTMLURL: "",
			},
			wantStatusCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const secret = "secret"
			h := NewStarEventWebhook(secret, func(_ context.Context, stargazer Stargazer) error {
				if stargazer != tt.want {
					return fmt.Errorf("got %v, want %v", stargazer, tt.want)
				}
				return nil
			}, slog.New(slog.DiscardHandler))

			var buf bytes.Buffer
			_ = json.NewEncoder(&buf).Encode(tt.event)
			mac := calculateHMAC(buf.Bytes(), secret)

			req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, "/", &buf)
			req.Header.Set("X-Hub-Signature-256", mac)
			req.Header.Set("User-Agent", "GitHub-Hookshot/1.0")
			resp := httptest.NewRecorder()
			h.ServeHTTP(resp, req)
			require.Equal(t, tt.wantStatusCode, resp.Code)
		})
	}
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
