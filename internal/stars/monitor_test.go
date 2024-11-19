package stars

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"
)

var discardLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

func TestRepoScanner(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	store := Store{DatabasePath: tmpDir}
	scanner := RepoScanner{
		User:         "foo",
		RepoInterval: time.Hour,
		StarInterval: time.Hour,
		Logger:       discardLogger,
		Client: &Client{
			Repositories: fakeRepositories{},
			Activity:     fakeActivity{},
		},
		Store:    &store,
		Notifier: SLogNotifier{Logger: discardLogger},
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error)
	go func() {
		errCh <- scanner.Run(ctx)
	}()

	assert.Eventually(t, func() bool {
		return store.Len() > 0
	}, time.Second, time.Millisecond*100)

	cancel()
	assert.NoError(t, <-errCh)

	want := map[string]repository{
		"foo/foo": {
			"user1": {StarredAt: time.Date(2024, time.November, 19, 21, 30, 0, 0, time.UTC)},
			"user2": {StarredAt: time.Date(2024, time.November, 19, 21, 30, 0, 0, time.UTC)},
		},
	}
	assert.Equal(t, want, store.Repos)
}
