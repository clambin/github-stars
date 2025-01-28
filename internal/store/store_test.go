package store

import (
	"github.com/clambin/github-stars/internal/testutils"
	"github.com/google/go-github/v68/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStore_SetStargazers(t *testing.T) {
	// current Store: user1, user2
	const input = `{
	"foo/foo": {
		"user1": { "starred_at": "2024-11-20T08:00:00Z"},
		"user2": { "starred_at": "2024-11-20T09:00:00Z"}
	}
}
`
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "stargazers.json"), []byte(input), 0600))

	s := New(tmpDir)
	require.NoError(t, s.Load())

	// fetched: user2, user3
	fetched := []*github.Stargazer{
		{User: &github.User{Login: testutils.Ptr("user2")}, StarredAt: &github.Timestamp{Time: time.Date(2024, time.November, 20, 9, 0, 0, 0, time.UTC)}},
		{User: &github.User{Login: testutils.Ptr("user3")}, StarredAt: &github.Timestamp{Time: time.Date(2024, time.November, 20, 9, 30, 0, 0, time.UTC)}},
	}
	newStargazers, err := s.SetStargazers(&github.Repository{FullName: testutils.Ptr("foo/foo")}, fetched)
	assert.NoError(t, err)
	// expected new: user3
	wantNewStargazers := []*github.Stargazer{
		{User: &github.User{Login: testutils.Ptr("user3")}, StarredAt: &github.Timestamp{Time: time.Date(2024, time.November, 20, 9, 30, 0, 0, time.UTC)}},
	}
	assert.Equal(t, wantNewStargazers, newStargazers)
	assert.Equal(t, len(wantNewStargazers), s.Len())

	body, err := os.ReadFile(filepath.Join(tmpDir, "stargazers.json"))
	assert.NoError(t, err)

	// expected Store: user2, user3
	want := `{
  "foo/foo": {
    "user2": {
      "starred_at": "2024-11-20T09:00:00Z"
    },
    "user3": {
      "starred_at": "2024-11-20T09:30:00Z"
    }
  }
}
`
	assert.Equal(t, want, string(body))

}

func TestStore_Add_Delete(t *testing.T) {
	s := New(t.TempDir())
	assert.Error(t, s.Load())

	repo := github.Repository{Name: testutils.Ptr("bar"), FullName: testutils.Ptr("foo/bar")}
	stargazer := github.Stargazer{
		User:      &github.User{Login: testutils.Ptr("user")},
		StarredAt: &github.Timestamp{Time: time.Date(2024, time.November, 20, 21, 30, 0, 0, time.UTC)},
	}

	// Add a new stargazer
	added, err := s.Add(&repo, &stargazer)
	require.NoError(t, err)
	assert.True(t, added)
	assert.Contains(t, s.Repos["foo/bar"], "user")

	// Add an existing stargazer
	added, err = s.Add(&repo, &stargazer)
	require.NoError(t, err)
	assert.False(t, added)
	assert.Contains(t, s.Repos["foo/bar"], "user")

	// Delete an existing stargazer
	deleted, err := s.Delete(&repo, &stargazer)
	require.NoError(t, err)
	assert.True(t, deleted)
	assert.NotContains(t, s.Repos["foo/bar"], "user")

	// Delete an non-existing stargazer
	deleted, err = s.Delete(&repo, &stargazer)
	require.NoError(t, err)
	assert.False(t, deleted)
	assert.NotContains(t, s.Repos["foo/bar"], "user")
}

func TestRepoStars_Equals(t *testing.T) {
	tests := []struct {
		name string
		old  RepoStars
		new  RepoStars
		want assert.BoolAssertionFunc
	}{
		{
			name: "empty",
			want: assert.True,
		},
		{
			name: "different size",
			old:  RepoStars{"foo": {StarredAt: time.Date(2024, time.November, 20, 11, 0, 0, 0, time.UTC)}},
			want: assert.False,
		},
		{
			name: "same size, different stars",
			old:  RepoStars{"foo": {StarredAt: time.Date(2024, time.November, 20, 11, 0, 0, 0, time.UTC)}},
			new:  RepoStars{"bar": {StarredAt: time.Date(2024, time.November, 20, 11, 0, 0, 0, time.UTC)}},
			want: assert.False,
		},
		{
			name: "equal",
			old:  RepoStars{"foo": {StarredAt: time.Date(2024, time.November, 20, 11, 0, 0, 0, time.UTC)}},
			new:  RepoStars{"foo": {StarredAt: time.Date(2024, time.November, 20, 11, 0, 0, 0, time.UTC)}},
			want: assert.True,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.want(t, tt.old.Equals(tt.new))
		})
	}
}
