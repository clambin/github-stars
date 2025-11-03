package main

import (
	"context"
	"fmt"
	"iter"
	"net/http"
	"testing"
	"time"

	"github.com/clambin/github-stars/internal/stars"
	"github.com/google/go-github/v76/github"
	"github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	client := fakeClient{
		repos: map[string][]*github.Repository{
			"user1": {
				{FullName: varPtr("user1/foo")},
				{FullName: varPtr("user1/bar")},
			},
		},
		stargazers: map[string][]*github.Stargazer{
			"user1/foo": {
				{
					StarredAt: &github.Timestamp{Time: time.Date(2024, time.November, 20, 8, 0, 0, 0, time.UTC)},
					User:      &github.User{Login: varPtr("user1")},
				},
			},
			"user1/bar": {
				{
					StarredAt: &github.Timestamp{Time: time.Date(2024, time.November, 20, 8, 0, 0, 0, time.UTC)},
					User:      &github.User{Login: varPtr("user1")},
				},
			},
		},
	}
	cfg := configuration{
		User:      "user1",
		Directory: t.TempDir(),
		GitHub:    githubConfiguration{WebHook: webhookConfiguration{Addr: ":8080"}},
	}

	// start the handler
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	errCh := make(chan error)
	go func() {
		errCh <- runWithClient(ctx, &client, cfg)
	}()

	// wait for the handler to perform the scan and start serving the webhook
	for {
		time.Sleep(10 * time.Millisecond)
		if resp, err := http.Get(fmt.Sprintf("http://localhost%s/readyz", cfg.GitHub.WebHook.Addr)); err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
	}

	// stop the handler
	cancel()
	require.NoError(t, <-errCh)
}

var _ stars.Client = fakeClient{}

type fakeClient struct {
	repos      map[string][]*github.Repository
	stargazers map[string][]*github.Stargazer
}

func (f fakeClient) GetUserRepos(_ context.Context, user string) iter.Seq2[*github.Repository, error] {
	r, ok := f.repos[user]
	if !ok {
		return func(yield func(*github.Repository, error) bool) {
			yield(nil, fmt.Errorf("user %q not found", user))
		}
	}
	return func(yield func(*github.Repository, error) bool) {
		for _, repo := range r {
			if !yield(repo, nil) {
				return
			}
		}
	}
}

func (f fakeClient) GetStarGazers(_ context.Context, repository *github.Repository) ([]*github.Stargazer, error) {
	if stargazers, ok := f.stargazers[repository.GetFullName()]; ok {
		return stargazers, nil
	}
	return nil, fmt.Errorf("repo %q not found", repository.GetFullName())
}

func varPtr[T any](v T) *T {
	return &v
}
