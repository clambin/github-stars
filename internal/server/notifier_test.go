package server

import (
	"bytes"
	"github.com/clambin/github-stars/internal/testutils"
	"github.com/google/go-github/v76/github"
	"github.com/stretchr/testify/assert"
	"log/slog"
	"testing"
)

func TestSLogNotifier_Notify(t *testing.T) {
	tests := []struct {
		name   string
		action bool
		want   string
	}{
		{
			name:   "added",
			action: true,
			want:   "level=INFO msg=\"repo has new stargazers\" repo=foo/bar stargazers=1\n",
		},
		{
			name:   "removed",
			action: false,
			want:   "level=INFO msg=\"repo lost stargazers\" repo=foo/bar stargazers=1\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			l := slog.New(slog.NewTextHandler(&output, &slog.HandlerOptions{ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				if a.Key == "time" {
					return slog.Attr{}
				}
				return a
			}}))
			notifiers := Notifiers{SLogNotifier{Logger: l}}

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
			notifiers.Notify(repo, gazers, tt.action)
			assert.Equal(t, tt.want, output.String())
		})
	}
}

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
