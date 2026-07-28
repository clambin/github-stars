package stars

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/clambin/github-stars/internal/github"
	"github.com/clambin/github-stars/slogctx"
	"github.com/slack-go/slack"
)

// Notifier notifies about added/removed stargazers.
type Notifier interface {
	Notify(ctx context.Context, added bool, stars ...github.Stargazer)
}

// Notifiers is a collection of Notifiers.
type Notifiers []Notifier

// Notify notifies all Notifiers.
func (n Notifiers) Notify(ctx context.Context, added bool, stars ...github.Stargazer) {
	for _, notifier := range n {
		notifier.Notify(ctx, added, stars...)
	}
}

// SlogNotifier is a Notifier that logs the added/removed stargazers to a slog.Logger stored in the context.
type SlogNotifier struct{}

var _ Notifier = SlogNotifier{}

func (s SlogNotifier) Notify(ctx context.Context, added bool, stars ...github.Stargazer) {
	logger := slogctx.FromContext(ctx)
	for repo, repoStars := range stargazersByRepo(stars) {
		var msg string
		switch added {
		case true:
			msg = fmt.Sprintf("repo has %d new stargazers", len(repoStars))
		case false:
			msg = fmt.Sprintf("repo lost %d stargazers", len(repoStars))
		}
		logger.Info(msg, slog.String("repo", repo))
	}
}

const (
	defaultMaximumUsers = 5
)

// SlackNotifier is a Notifier that posts added/removed stargazers to a Slack channel.
type SlackNotifier struct {
	// WebHookURL is the URL to the Slack webhook
	WebHookURL string
	// MaximumUsers is the maximum number of users to notify about. Default is 5.
	// If the number of stargazers is greater than this, SlackNotifier only notifies the number of users.
	MaximumUsers int
}

var _ Notifier = SlackNotifier{}

func (s SlackNotifier) Notify(ctx context.Context, added bool, stars ...github.Stargazer) {
	for _, stargazers := range stargazersByRepo(stars) {
		err := slack.PostWebhook(s.WebHookURL, &slack.WebhookMessage{
			Text:        s.makeMessage(stargazers, added),
			UnfurlLinks: new(false),
		})
		if err != nil {
			slogctx.FromContext(ctx).Warn("Failed to post message", "err", err)
		}
	}
}

var action = map[bool]string{
	true:  "received",
	false: "lost",
}

func (s SlackNotifier) makeMessage(gazers []github.Stargazer, added bool) string {
	// Guard against empty input (shouldn't normally happen)
	if len(gazers) == 0 {
		return ""
	}

	// Determine repo information from the first stargazer
	repoName := slackFormatRepo(gazers[0])

	// maximum users to list individually
	maxUsers := cmp.Or(s.MaximumUsers, defaultMaximumUsers)

	// Create the userList text depending on the number of gazers
	var userList string
	if len(gazers) > 1 {
		userList = strconv.Itoa(len(gazers)) + " users"
	}
	if len(gazers) <= maxUsers {
		if len(gazers) > 1 {
			userList += ": "
		}
		// Build list of user mentions
		users := make([]string, len(gazers))
		for i := range gazers {
			users[i] = slackFormatUser(gazers[i])
		}
		userList += strings.Join(users, ", ")
	}

	return "Repo " + repoName + " " + action[added] + " a star from " + userList
}

func stargazersByRepo(stargazer []github.Stargazer) map[string][]github.Stargazer {
	out := make(map[string][]github.Stargazer)
	for _, stargazers := range stargazer {
		out[stargazers.RepoName] = append(out[stargazers.RepoName], stargazers)
	}
	return out
}

func slackFormatRepo(stargazer github.Stargazer) string {
	if repoHTMLURL := stargazer.RepoHTMLURL; repoHTMLURL != "" {
		return "<" + repoHTMLURL + "|" + stargazer.RepoName + ">"
	}
	return stargazer.RepoName
}

func slackFormatUser(stargazer github.Stargazer) string {
	if userHTMLURL := stargazer.UserHTMLURL; userHTMLURL != "" {
		return "<" + userHTMLURL + "|@" + stargazer.Login + ">"
	}
	return stargazer.Login
}
