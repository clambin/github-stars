package stars

import (
	"context"
	"github.com/clambin/github-stars/internal/store"
	"github.com/google/go-github/v66/github"
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

	starStore := store.Store{DatabasePath: tmpDir}
	notifier := fakeNotifier{}
	scanner := RepoScanner{
		User:         "foo",
		RepoInterval: time.Hour,
		StarInterval: time.Hour,
		Logger:       discardLogger,
		Client: &Client{
			Repositories: fakeRepositories{},
			Activity:     fakeActivity{},
		},
		Store:    &starStore,
		Notifier: &notifier,
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error)
	go func() {
		errCh <- scanner.Run(ctx)
	}()

	assert.Eventually(t, func() bool {
		return starStore.Len() > 0
	}, time.Second, time.Millisecond*100)

	cancel()
	assert.NoError(t, <-errCh)

	want := map[string]store.RepoStars{
		"foo/foo": {
			"user1": {StarredAt: time.Date(2024, time.November, 19, 21, 30, 0, 0, time.UTC)},
			"user2": {StarredAt: time.Date(2024, time.November, 19, 21, 30, 0, 0, time.UTC)},
		},
	}
	assert.Equal(t, want, starStore.Repos)

	notifications, ok := notifier.rcvd["foo/foo"]
	assert.True(t, ok)
	assert.Len(t, notifications, 2)
}

var _ Notifier = &fakeNotifier{}

type fakeNotifier struct {
	rcvd map[string][]*github.Stargazer
}

func (f *fakeNotifier) Notify(repository *github.Repository, gazers []*github.Stargazer) {
	if f.rcvd == nil {
		f.rcvd = make(map[string][]*github.Stargazer)
	}
	f.rcvd[repository.GetFullName()] = gazers
}
