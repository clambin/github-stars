package stars

import (
	ggh "github.com/google/go-github/v65/github"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSlackWebHookNotifier_makeMessage(t *testing.T) {
	repo := &ggh.Repository{
		FullName: ConstP("foo/bar"),
		HTMLURL:  ConstP("https://example.com/foo/bar"),
	}
	gazers := []*ggh.Stargazer{{
		User: &ggh.User{
			Login:   ConstP("user1"),
			HTMLURL: ConstP("https://example.com/users/user1"),
		},
	}}

	var s SlackWebHookNotifier
	text := s.makeMessage(repo, gazers)
	assert.Equal(t, `Repo <https://example.com/foo/bar|foo/bar> received a star from <https://example.com/users/user1|@user1>`, text)

	gazers = append(gazers, &ggh.Stargazer{
		User: &ggh.User{
			Login:   ConstP("user2"),
			HTMLURL: ConstP("https://example.com/users/user2"),
		},
	})
	text = s.makeMessage(repo, gazers)
	assert.Equal(t, `Repo <https://example.com/foo/bar|foo/bar> received a star from 2 users: <https://example.com/users/user1|@user1>, <https://example.com/users/user2|@user2>`, text)
}
