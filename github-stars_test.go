package main

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/clambin/github-stars/internal/github"
	"github.com/clambin/github-stars/internal/stars"
	"github.com/stretchr/testify/assert"
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
	ctx, cancel := context.WithCancel(t.Context())
	errCh := make(chan error)
	go func() {
		errCh <- runWithClient(ctx, &client, cfg)
	}()

	// wait for the handler to perform the scan and start serving the webhook
	assert.Eventually(t, func() bool {
		resp, err := http.Get(fmt.Sprintf("http://localhost%s/readyz", cfg.GitHub.WebHook.Addr))
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, 5*time.Second, 1000*time.Millisecond)

	// stop the handler
	cancel()
	require.NoError(t, <-errCh)
}

var _ stars.Client = fakeClient{}

type fakeClient struct {
	stargazers []github.Stargazer
}

func (f fakeClient) Stargazers(_ context.Context, _ string) ([]github.Stargazer, error) {
	return f.stargazers, nil
}
