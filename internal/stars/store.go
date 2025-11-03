package stars

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/clambin/github-stars/slogctx"
	"github.com/google/go-github/v76/github"
	"github.com/slack-go/slack"
)

// RepoStar is a single star for a repository
type RepoStar struct {
	StarredAt time.Time `json:"starred_at"`
}

// RepoStars contains all stars for a repository
type RepoStars map[string]RepoStar

// Store contains all stars for all repositories
type Store struct {
	Repos        map[string]RepoStars
	DatabasePath string
	lock         sync.RWMutex
}

// NewStore creates a new Store
func NewStore(databasePath string) (*Store, error) {
	store := Store{
		DatabasePath: databasePath,
		Repos:        make(map[string]RepoStars),
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	return &store, nil
}

// load loads the store from disk
func (s *Store) load() error {
	s.lock.Lock()
	defer s.lock.Unlock()
	f, err := os.Open(filepath.Join(s.DatabasePath, "stargazers.json"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer func() { _ = f.Close() }()
	return json.NewDecoder(f).Decode(&s.Repos)
}

// save saves the store to disk
func (s *Store) save() error {
	f, err := os.Create(filepath.Join(s.DatabasePath, "stargazers.json"))
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer func() { _ = f.Close() }()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err = enc.Encode(s.Repos); err != nil {
		_ = f.Close()
		return fmt.Errorf("encode: %w", err)
	}
	return f.Close()
}

type Stargazer struct {
	StarredAt time.Time
	User      string
}

// Add adds new stargazers to a repository.
// Returns the new stargazers and an error if there was a problem saving the store to disk.
func (s *Store) Add(repo *github.Repository, stargazers ...*github.Stargazer) ([]*github.Stargazer, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	repoFullName := repo.GetFullName()
	r, ok := s.Repos[repoFullName]
	if !ok {
		r = make(RepoStars)
	}
	newStargazers := make([]*github.Stargazer, 0, len(stargazers))
	for _, stargazer := range stargazers {
		login := stargazer.GetUser().GetLogin()
		if _, ok = r[login]; !ok {
			newStargazers = append(newStargazers, stargazer)
		}
		r[login] = RepoStar{StarredAt: stargazer.GetStarredAt().Time}
	}
	s.Repos[repoFullName] = r
	return newStargazers, s.save()
}

// Delete removes stargazers from a repository.
// Returns the removed stargazers and an error if there was a problem saving the store to disk.
func (s *Store) Delete(repo *github.Repository, stargazers ...*github.Stargazer) ([]*github.Stargazer, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	repoFullName := repo.GetFullName()
	r, ok := s.Repos[repoFullName]
	if !ok {
		return nil, nil
	}
	removedStargazers := make([]*github.Stargazer, 0, len(stargazers))
	for _, stargazer := range stargazers {
		login := stargazer.GetUser().GetLogin()
		if _, ok = r[login]; ok {
			delete(r, login)
			removedStargazers = append(removedStargazers, stargazer)
		}
	}
	s.Repos[repoFullName] = r
	return removedStargazers, s.save()
}

// NotifyingStore is a Store that notifies a Notifier when stargazers are added or removed.
type NotifyingStore struct {
	*Store
	Notifiers
}

// NewNotifyingStore creates a new NotifyingStore.
func NewNotifyingStore(databaseDirectory string, notifiers Notifiers) (*NotifyingStore, error) {
	store, err := NewStore(databaseDirectory)
	if err != nil {
		return nil, err
	}
	return &NotifyingStore{
		Store:     store,
		Notifiers: notifiers,
	}, nil
}

// Add adds new stargazers to a repository.
// Notifies the Notifier if there were any new stargazers.
func (s NotifyingStore) Add(ctx context.Context, repo *github.Repository, stargazers ...*github.Stargazer) error {
	added, err := s.Store.Add(repo, stargazers...)
	if err == nil && len(added) > 0 {
		s.Notify(ctx, true, repo, added...)
	}
	return err
}

// Delete removes stargazers from a repository.
// Notifies the Notifier if there were any removed stargazers.
func (s NotifyingStore) Delete(ctx context.Context, repo *github.Repository, stargazers ...*github.Stargazer) error {
	deleted, err := s.Store.Delete(repo, stargazers...)
	if err == nil && len(deleted) > 0 {
		s.Notify(ctx, false, repo, stargazers...)
	}
	return err
}

// Notifier notifies about added/removed stargazers.
type Notifier interface {
	Notify(ctx context.Context, added bool, repo *github.Repository, stargazers ...*github.Stargazer)
}

// Notifiers is a collection of Notifiers.
type Notifiers []Notifier

// Notify notifies all Notifiers.
func (n Notifiers) Notify(ctx context.Context, added bool, repo *github.Repository, stargazers ...*github.Stargazer) {
	for _, notifier := range n {
		notifier.Notify(ctx, added, repo, stargazers...)
	}
}

// SlogNotifier is a Notifier that logs the added/removed stargazers to a slog.Logger stored in the context.
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

const (
	defaultMaximumUsers = 5
)

// SlackNotifier is a Notifier that posts added/removed stargazers to a Slack channel using a webhook.
type SlackNotifier struct {
	// WebHookURL is the URL to the slack webhook
	WebHookURL string
	// MaximumUsers is the maximum number of users to notify about. Default is 5.
	// If the number of stargazers is greater than this, SlackNotifier only notifies the number of users.
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
	if len(gazers) <= cmp.Or(s.MaximumUsers, defaultMaximumUsers) {
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
