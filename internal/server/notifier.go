package server

import (
	"github.com/google/go-github/v66/github"
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
		msg = "stargazers left repo"
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

func (s SlackNotifier) makeMessage(repository *github.Repository, gazers []*github.Stargazer, added bool) string {
	var msg string
	if added {
		msg = "received a star from "
	} else {
		msg = "lost a star from "
	}
	list := make([]string, len(gazers))
	for i, gazer := range gazers {
		list[i] = "<" + gazer.GetUser().GetHTMLURL() + "|@" + gazer.GetUser().GetLogin() + ">"
	}
	msg = "Repo <" + repository.GetHTMLURL() + "|" + repository.GetFullName() + "> " + msg
	if len(gazers) > 1 {
		msg += strconv.Itoa(len(gazers)) + " users: "
	}
	msg += strings.Join(list, ", ")

	return msg
}
