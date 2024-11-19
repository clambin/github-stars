package stars

import (
	"github.com/google/go-github/v66/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStore(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	const input = `{
	"foo/foo": {
		"user1": { "starred_at": "2024-10-31T05:50:46Z"}
	}
}
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "stargazers.json"), []byte(input), 0600))

	s := Store{DatabasePath: tmpDir}

	assert.NoError(t, s.Load())

	r := github.Repository{
		Name:     ConstP("foo"),
		FullName: ConstP("foo/foo"),
	}
	g := github.Stargazer{
		User:      &github.User{Login: ConstP("user2")},
		StarredAt: &github.Timestamp{Time: time.Date(2024, time.November, 19, 22, 0, 0, 0, time.UTC)},
	}

	added, err := s.Add(&r, &g)
	assert.NoError(t, err)
	assert.True(t, added)

	added, err = s.Add(&r, &g)
	assert.NoError(t, err)
	assert.False(t, added)
}
