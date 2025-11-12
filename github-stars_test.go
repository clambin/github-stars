package main

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/clambin/github-stars/internal/github"
	"github.com/clambin/github-stars/internal/stars"
	"github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	client := fakeClient{stargazers: []github.Stargazer{
		{StarredAt: time.Date(2024, time.November, 20, 8, 0, 0, 0, time.UTC), RepoName: "user1/foo", Login: "user1"},
		{StarredAt: time.Date(2024, time.November, 20, 8, 0, 0, 0, time.UTC), RepoName: "user1/foo", Login: "user2"},
	}}
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
	stargazers []github.Stargazer
}

func (f fakeClient) Stargazers(context.Context, string, bool) ([]github.Stargazer, error) {
	return f.stargazers, nil
}
