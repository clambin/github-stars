package stars

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/clambin/github-stars/internal/github"
	"github.com/clambin/github-stars/slogctx"
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
	Repos        map[string]map[string]RepoStar
	DatabasePath string
	lock         sync.RWMutex
}

// NewStore creates a new Store
func NewStore(databasePath string) (*Store, error) {
	store := Store{
		DatabasePath: databasePath,
		Repos:        make(map[string]map[string]RepoStar),
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
func (s *Store) Add(stargazers ...github.Stargazer) ([]github.Stargazer, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	added := make([]github.Stargazer, 0, len(stargazers))
	for _, star := range stargazers {
		if _, ok := s.Repos[star.RepoName]; !ok {
			s.Repos[star.RepoName] = make(RepoStars)
		}
		if _, ok := s.Repos[star.RepoName][star.Login]; !ok {
			s.Repos[star.RepoName][star.Login] = RepoStar{StarredAt: star.StarredAt}
			added = append(added, star)
		}
	}
	return added, s.save()
}

// Delete removes stargazers from a repository.
// Returns the removed stargazers and an error if there was a problem saving the store to disk.
func (s *Store) Delete(stargazer ...github.Stargazer) ([]github.Stargazer, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	removed := make([]github.Stargazer, 0, len(stargazer))
	for _, star := range stargazer {
		if _, ok := s.Repos[star.RepoName]; !ok {
			continue
		}
		if _, ok := s.Repos[star.RepoName][star.Login]; !ok {
			continue
		}
		delete(s.Repos[star.RepoName], star.Login)
		removed = append(removed, star)
	}
	return removed, s.save()
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
func (s NotifyingStore) Add(ctx context.Context, stars ...github.Stargazer) error {
	added, err := s.Store.Add(stars...)
	if err == nil && len(added) > 0 {
		s.Notify(ctx, true, added)
	}
	return err
}

// Delete removes stargazers from a repository.
// Notifies the Notifier if there were any removed stargazers.
func (s NotifyingStore) Delete(ctx context.Context, stars ...github.Stargazer) error {
	deleted, err := s.Store.Delete(stars...)
	if err == nil && len(deleted) > 0 {
		s.Notify(ctx, false, deleted)
	}
	return err
}

func (s NotifyingStore) Set(ctx context.Context, stars []github.Stargazer) error {
	// set the store to the given star gazers
	// generate notifications added/removed vs current store
	panic("not implemented")
}

// Notifier notifies about added/removed stargazers.
type Notifier interface {
	Notify(ctx context.Context, added bool, stars []github.Stargazer)
}

// Notifiers is a collection of Notifiers.
type Notifiers []Notifier

// Notify notifies all Notifiers.
func (n Notifiers) Notify(ctx context.Context, added bool, stars []github.Stargazer) {
	for _, notifier := range n {
		notifier.Notify(ctx, added, stars)
	}
}

// SlogNotifier is a Notifier that logs the added/removed stargazers to a slog.Logger stored in the context.
type SlogNotifier struct{}

var _ Notifier = SlogNotifier{}

func (s SlogNotifier) Notify(ctx context.Context, added bool, stars []github.Stargazer) {
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

func (s SlackNotifier) Notify(ctx context.Context, added bool, stars []github.Stargazer) {
	for _, repoStars := range stargazersByRepo(stars) {
		err := slack.PostWebhook(s.WebHookURL, &slack.WebhookMessage{
			Text:        s.makeMessage(repoStars, added),
			UnfurlLinks: false,
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
	var userList string
	if len(gazers) > 1 {
		userList = strconv.Itoa(len(gazers)) + " users"
	}
	var repoName, repoHTMLURL string
	users := make([]string, 0, len(gazers))
	for _, star := range gazers {
		users = append(users, "<"+star.UserHTMLURL+"|@"+star.Login+">")
		repoName = star.RepoName
		repoHTMLURL = star.RepoHTMLURL
	}

	if len(gazers) <= cmp.Or(s.MaximumUsers, defaultMaximumUsers) {
		if len(gazers) > 1 {
			userList += ": "
		}
		userList += strings.Join(users, ", ")
	}
	return "Repo <" + repoHTMLURL + "|" + repoName + "> " + action[added] + " a star from " + userList
}

func stargazersByRepo(stargazer []github.Stargazer) map[string][]github.Stargazer {
	out := make(map[string][]github.Stargazer)
	for _, stargazers := range stargazer {
		out[stargazers.RepoName] = append(out[stargazers.RepoName], stargazers)
	}
	return out
}
