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

	"github.com/clambin/github-stars/slogctx"
	"github.com/google/go-github/v76/github"
	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	require.NoError(t, err)

	repo := &github.Repository{FullName: varPtr("foo/bar")}
	toAdd := []*github.Stargazer{
		{
			StarredAt: &github.Timestamp{Time: time.Date(2024, time.November, 20, 8, 0, 0, 0, time.UTC)},
			User:      &github.User{Login: varPtr("user1")},
		},
		{
			StarredAt: &github.Timestamp{Time: time.Date(2024, time.November, 20, 9, 0, 0, 0, time.UTC)},
			User:      &github.User{Login: varPtr("user2")},
		},
	}

	// Add the new stargazers. All should be added
	added, err := store.Add(repo, toAdd...)
	require.NoError(t, err)
	require.Equal(t, toAdd, added)

	// Add the same stargazers again. Nothing should change
	added, err = store.Add(repo, toAdd...)
	require.NoError(t, err)
	require.Empty(t, added)

	// Load a copy. Add the same stargazers again. Nothing should change
	store2, err := NewStore(tmpDir)
	require.NoError(t, err)
	added, err = store2.Add(repo, toAdd...)
	require.NoError(t, err)
	require.Empty(t, added)

	// Delete the stargazers
	deleted, err := store.Delete(repo, toAdd...)
	require.NoError(t, err)
	require.Equal(t, toAdd, deleted)

	// Delete the stargazers again. Nothing should change
	deleted, err = store.Delete(repo, toAdd...)
	require.NoError(t, err)
	require.Empty(t, deleted)
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

	repo := &github.Repository{FullName: varPtr("foo/bar"), HTMLURL: varPtr("https://example.com/foo/bar")}
	toAdd := []*github.Stargazer{
		{
			StarredAt: &github.Timestamp{Time: time.Date(2024, time.November, 20, 8, 0, 0, 0, time.UTC)},
			User:      &github.User{HTMLURL: varPtr("https://example.com/user1"), Login: varPtr("user1")},
		},
	}

	require.NoError(t, store.Add(ctx, repo, toAdd...))
	require.NoError(t, store.Delete(ctx, repo, toAdd...))

	wantSlog := `level=INFO msg="repo has 1 new stargazers" repository=foo/bar
level=INFO msg="repo lost 1 stargazers" repository=foo/bar
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
