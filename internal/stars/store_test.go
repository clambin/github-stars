package stars

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/clambin/github-stars/internal/github"
	"github.com/clambin/github-stars/slogctx"
	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	require.NoError(t, err)

	toAdd := []github.Stargazer{
		{"", "foo/bar", "", "user1", "", time.Date(2024, time.November, 20, 8, 0, 0, 0, time.UTC)},
		{"", "foo/bar", "", "user2", "", time.Date(2024, time.November, 20, 8, 0, 0, 0, time.UTC)},
	}

	// Add the new stargazers. All should be added
	added, err := store.Add(toAdd...)
	require.NoError(t, err)
	require.Equal(t, toAdd, added)

	// Add the same stargazers again. Nothing should change
	added, err = store.Add(toAdd...)
	require.NoError(t, err)
	require.Empty(t, added)

	// Load a copy. Add the same stargazers again. Nothing should change
	store2, err := NewStore(tmpDir)
	require.NoError(t, err)
	added, err = store2.Add(toAdd...)
	require.NoError(t, err)
	require.Empty(t, added)

	// Delete the stargazers
	deleted, err := store.Delete(toAdd...)
	require.NoError(t, err)
	require.Equal(t, toAdd, deleted)

	// Delete the stargazers again. Nothing should change
	deleted, err = store.Delete(toAdd...)
	require.NoError(t, err)
	require.Empty(t, deleted)

	// reset the store to its initial state
	_, err = store.Add(toAdd...)
	require.NoError(t, err)

	// Set the stargazers with new records.
	added, deleted, err = store.Set([]github.Stargazer{
		{"", "foo/bar", "", "user1", "", time.Date(2024, time.November, 20, 8, 0, 0, 0, time.UTC)},
		{"", "foo/bar", "", "user3", "", time.Date(2024, time.November, 20, 8, 0, 0, 0, time.UTC)},
	})
	require.NoError(t, err)
	require.Len(t, added, 1)
	require.Len(t, deleted, 1)
	require.Equal(t, "user3", added[0].Login)
	require.Equal(t, "user2", deleted[0].Login)
}

func TestNotifyingStore(t *testing.T) {
	var s fakeSlackWebhook
	ts := httptest.NewServer(&s)
	t.Cleanup(ts.Close)

	notifiers := Notifiers{
		SlogNotifier{},
		SlackNotifier{WebHookURL: ts.URL},
	}
	store, err := NewNotifyingStore(t.TempDir(), notifiers)
	require.NoError(t, err)

	var buf bytes.Buffer
	ctx := slogctx.NewWithContext(t.Context(), slogWithoutTime(&buf, slog.LevelInfo))

	toAdd := []github.Stargazer{
		{"", "foo/bar", "https://example.com/foo/bar", "user1", "https://example.com/user1", time.Date(2024, time.November, 20, 8, 0, 0, 0, time.UTC)},
	}

	require.NoError(t, store.Add(ctx, toAdd...))
	require.NoError(t, store.Delete(ctx, toAdd...))

	wantSlog := `level=INFO msg="repo has 1 new stargazers" repo=foo/bar
level=INFO msg="repo lost 1 stargazers" repo=foo/bar
`
	assert.Equal(t, wantSlog, buf.String())

	wantSlack := []string{
		"Repo <https://example.com/foo/bar|foo/bar> received a star from <https://example.com/user1|@user1>",
		"Repo <https://example.com/foo/bar|foo/bar> lost a star from <https://example.com/user1|@user1>",
	}
	assert.Equal(t, wantSlack, s.received())
}

type fakeSlackWebhook struct {
	messages []string
	lock     sync.Mutex
}

var _ http.Handler = (*fakeSlackWebhook)(nil)

func (f *fakeSlackWebhook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "invalid content type", http.StatusBadRequest)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "invalid method", http.StatusBadRequest)
		return
	}
	var body slack.WebhookMessage
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	f.lock.Lock()
	defer f.lock.Unlock()
	f.messages = append(f.messages, body.Text)
}

func (f *fakeSlackWebhook) received() []string {
	f.lock.Lock()
	defer f.lock.Unlock()
	return slices.Clone(f.messages)
}
