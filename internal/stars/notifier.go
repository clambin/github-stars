package stars

import (
	ggh "github.com/google/go-github/v65/github"
	"github.com/slack-go/slack"
	"log/slog"
	"strconv"
	"strings"
)

type Notifier interface {
	Notify(repository *ggh.Repository, gazers []*ggh.Stargazer)
}

var _ Notifier = Notifiers{}

type Notifiers []Notifier

func (n Notifiers) Notify(repository *ggh.Repository, gazers []*ggh.Stargazer) {
	for _, notifier := range n {
		notifier.Notify(repository, gazers)
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////

var _ Notifier = SLogNotifier{}

type SLogNotifier struct {
	Logger *slog.Logger
}

func (s SLogNotifier) Notify(repository *ggh.Repository, gazers []*ggh.Stargazer) {
	s.Logger.Info("repo has new stargazers",
		"repo", repository.GetFullName(),
		"stargazers", len(gazers))
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type SlackWebHookNotifier struct {
	WebHookURL string
	Logger     *slog.Logger
}

func (s *SlackWebHookNotifier) Notify(repository *ggh.Repository, gazers []*ggh.Stargazer) {
	err := slack.PostWebhook(s.WebHookURL, &slack.WebhookMessage{
		Text:        s.makeMessage(repository, gazers),
		UnfurlLinks: false,
	})
	if err != nil {
		s.Logger.Warn("Failed to post message", "err", err)
	}
}

func (s *SlackWebHookNotifier) makeMessage(repository *ggh.Repository, gazers []*ggh.Stargazer) string {
	list := make([]string, len(gazers))
	for i, gazer := range gazers {
		list[i] = "<" + gazer.GetUser().GetHTMLURL() + "|@" + gazer.GetUser().GetLogin() + ">"
	}
	msg := "Repo <" + repository.GetHTMLURL() + "|" + repository.GetFullName() + "> received a star from "
	if len(gazers) > 1 {
		msg += strconv.Itoa(len(gazers)) + " users: "
	}
	msg += strings.Join(list, ", ")

	return msg
}
