package server

import (
	"github.com/google/go-github/v70/github"
	"github.com/slack-go/slack"
	"log/slog"
	"strconv"
	"strings"
)

var _ Notifier = Notifiers{}

type Notifiers []Notifier

func (n Notifiers) Notify(repository *github.Repository, gazers []*github.Stargazer, added bool) {
	for _, notifier := range n {
		notifier.Notify(repository, gazers, added)
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////

var _ Notifier = SLogNotifier{}

type SLogNotifier struct {
	Logger *slog.Logger
}

func (s SLogNotifier) Notify(repository *github.Repository, gazers []*github.Stargazer, added bool) {
	var msg string
	if added {
		msg = "repo has new stargazers"
	} else {
		msg = "repo lost stargazers"
	}
	s.Logger.Info(msg, "repo", repository.GetFullName(), "stargazers", len(gazers))
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////

var _ Notifier = SlackNotifier{}

type SlackNotifier struct {
	WebHookURL string
	Logger     *slog.Logger
}

func (s SlackNotifier) Notify(repository *github.Repository, gazers []*github.Stargazer, added bool) {
	err := slack.PostWebhook(s.WebHookURL, &slack.WebhookMessage{
		Text:        s.makeMessage(repository, gazers, added),
		UnfurlLinks: false,
	})
	if err != nil {
		s.Logger.Warn("Failed to post message", "err", err)
	}
}

var action = map[bool]string{
	true:  "received",
	false: "lost",
}

func (s SlackNotifier) makeMessage(repository *github.Repository, gazers []*github.Stargazer, added bool) string {
	repo := "<" + repository.GetHTMLURL() + "|" + repository.GetFullName() + ">"

	users := make([]string, len(gazers))
	for i, gazer := range gazers {
		users[i] = "<" + gazer.GetUser().GetHTMLURL() + "|@" + gazer.GetUser().GetLogin() + ">"
	}
	var userCount string
	if len(users) > 1 {
		userCount = strconv.Itoa(len(users)) + " users: "
	}

	return "Repo " + repo + " " + action[added] + " a star from " + userCount + strings.Join(users, ", ")
}
