package server

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/clambin/github-stars/internal/server/mocks"
	"github.com/google/go-github/v76/github"
	"github.com/stretchr/testify/assert"
)

func TestRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	c := mocks.NewClient(t)
	c.EXPECT().GetUserRepos(ctx, *user).Return(func(yield func(*github.Repository, error) bool) {}).Once()

	errCh := make(chan error)
	go func() {
		errCh <- runWithClient(ctx, c, "dev", slog.New(slog.NewTextHandler(os.Stderr, nil)))
	}()

	assert.Eventually(t, func() bool {
		resp, err := http.Get("http://localhost:8080/readyz")
		return err == nil && resp.StatusCode == http.StatusOK
	}, time.Second, 10*time.Millisecond)

	cancel()
	assert.NoError(t, <-errCh)
}
