package stars

import (
	"cmp"
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/clambin/github-stars/slogctx"
	"github.com/google/go-github/v76/github"
	"github.com/slack-go/slack"
)

type Notifier interface {
	Notify(ctx context.Context, added bool, repo *github.Repository, stargazers ...*github.Stargazer)
}

type Notifiers []Notifier

func (n Notifiers) Notify(ctx context.Context, added bool, repo *github.Repository, stargazers ...*github.Stargazer) {
	for _, notifier := range n {
		notifier.Notify(ctx, added, repo, stargazers...)
	}
}

// SlogNotifier is a Notifier that logs the added/removed stargazers to the slog.Logger
type SlogNotifier struct{}

var _ Notifier = SlogNotifier{}

func (s SlogNotifier) Notify(ctx context.Context, added bool, repo *github.Repository, stargazers ...*github.Stargazer) {
	l := slogctx.FromContext(ctx).With("repository", repo.GetFullName())
	var msg string
	switch added {
	case true:
		msg = fmt.Sprintf("repo has %d new stargazers", len(stargazers))
	case false:
		msg = fmt.Sprintf("repo lost %d stargazers", len(stargazers))
	}
	l.Info(msg)
}

// SlackNotifier is a notifier that sends a message to a slack channel
type SlackNotifier struct {
	// WebHookURL is the URL to the slack webhook
	WebHookURL string
	// MaximumUsers is the maximum number of users to notify about.
	// If the number of stargazers is greater than this, SlackNotifier only notifies the number of users.
	// TODO: implement this
	MaximumUsers int
}

var _ Notifier = SlackNotifier{}

func (s SlackNotifier) Notify(ctx context.Context, added bool, repo *github.Repository, stargazers ...*github.Stargazer) {
	err := slack.PostWebhook(s.WebHookURL, &slack.WebhookMessage{
		Text:        s.makeMessage(repo, stargazers, added),
		UnfurlLinks: false,
	})
	if err != nil {
		slogctx.FromContext(ctx).Warn("Failed to post message", "err", err)
	}
}

var action = map[bool]string{
	true:  "received",
	false: "lost",
}

func (s SlackNotifier) makeMessage(repo *github.Repository, gazers []*github.Stargazer, added bool) string {
	var userList string
	if len(gazers) > 1 {
		userList = strconv.Itoa(len(gazers)) + " users"
	}
	if len(gazers) < cmp.Or(s.MaximumUsers, 5) {
		if len(gazers) > 1 {
			userList += ": "
		}
		users := make([]string, len(gazers))
		for i, gazer := range gazers {
			users[i] = "<" + gazer.GetUser().GetHTMLURL() + "|@" + gazer.GetUser().GetLogin() + ">"
		}
		userList += strings.Join(users, ", ")
	}
	return "Repo <" + repo.GetHTMLURL() + "|" + repo.GetFullName() + "> " + action[added] + " a star from " + userList
}
