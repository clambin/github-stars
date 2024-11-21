package server

import (
	"github.com/clambin/github-stars/internal/testutils"
	"github.com/google/go-github/v66/github"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSlackWebHookNotifier_makeMessage(t *testing.T) {
	repo := &github.Repository{
		FullName: testutils.Ptr("foo/bar"),
		HTMLURL:  testutils.Ptr("https://example.com/foo/bar"),
	}
	gazers := []*github.Stargazer{{
		User: &github.User{
			Login:   testutils.Ptr("user1"),
			HTMLURL: testutils.Ptr("https://example.com/users/user1"),
		},
	}}

	var s SlackNotifier
	text := s.makeMessage(repo, gazers, true)
	assert.Equal(t, `Repo <https://example.com/foo/bar|foo/bar> received a star from <https://example.com/users/user1|@user1>`, text)

	gazers = append(gazers, &github.Stargazer{
		User: &github.User{
			Login:   testutils.Ptr("user2"),
			HTMLURL: testutils.Ptr("https://example.com/users/user2"),
		},
	})
	text = s.makeMessage(repo, gazers, true)
	assert.Equal(t, `Repo <https://example.com/foo/bar|foo/bar> received a star from 2 users: <https://example.com/users/user1|@user1>, <https://example.com/users/user2|@user2>`, text)
}
