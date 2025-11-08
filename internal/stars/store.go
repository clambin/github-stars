package stars

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/clambin/github-stars/internal/github"
	"github.com/clambin/github-stars/slogctx"
	"github.com/slack-go/slack"
)

// Store contains all stars for all repositories
type Store struct {
	stargazers   map[string]map[string]github.Stargazer
	databasePath string
	lock         sync.RWMutex
}

// NewStore creates a new Store
func NewStore(databasePath string) (store *Store, err error) {
	store = &Store{databasePath: databasePath}
	f, err := os.Open(filepath.Join(databasePath, "stargazers.json"))
	switch {
	case err == nil:
		defer func() { _ = f.Close() }()
		var err2 *json.UnmarshalTypeError
		if store.stargazers, err = load(f); errors.As(err, &err2) {
			// store was created with an older version. reset it. Scan() will repopulate it.
			// (yeah, a bit hacky)
			store.stargazers = make(map[string]map[string]github.Stargazer)
			err = nil
		}
	case os.IsNotExist(err):
		store.stargazers = make(map[string]map[string]github.Stargazer)
		err = nil
	default:
		return nil, err
	}
	return store, err
}

// load loads the store from disk
func load(r io.Reader) (map[string]map[string]github.Stargazer, error) {
	var stargazers []github.Stargazer
	if err := json.NewDecoder(r).Decode(&stargazers); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return indexedStargazers(stargazers), nil
}

// save saves the store to disk
func (s *Store) save() error {
	var stargazers []github.Stargazer
	for _, users := range s.stargazers {
		for _, rs := range users {
			stargazers = append(stargazers, rs)
		}
	}

	f, err := os.Create(filepath.Join(s.databasePath, "stargazers.json"))
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer func() { _ = f.Close() }()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err = enc.Encode(stargazers); err != nil {
		_ = f.Close()
		return fmt.Errorf("encode: %w", err)
	}
	return f.Close()
}

// Add adds new stargazers to a repository.
// Returns the new stargazers and an error if there was a problem saving the store to disk.
func (s *Store) Add(stargazers ...github.Stargazer) ([]github.Stargazer, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	added := make([]github.Stargazer, 0, len(stargazers))
	for _, star := range stargazers {
		if _, ok := s.stargazers[star.RepoName]; !ok {
			s.stargazers[star.RepoName] = make(map[string]github.Stargazer)
		}
		if _, ok := s.stargazers[star.RepoName][star.Login]; !ok {
			s.stargazers[star.RepoName][star.Login] = star
			added = append(added, star)
		}
	}
	var err error
	if len(added) > 0 {
		err = s.save()
	}
	return added, err
}

// Delete removes stargazers from a repository.
// Returns the removed stargazers and an error if there was a problem saving the store to disk.
func (s *Store) Delete(stargazer ...github.Stargazer) ([]github.Stargazer, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	removed := make([]github.Stargazer, 0, len(stargazer))
	for _, star := range stargazer {
		if _, ok := s.stargazers[star.RepoName]; !ok {
			continue
		}
		if _, ok := s.stargazers[star.RepoName][star.Login]; !ok {
			continue
		}
		delete(s.stargazers[star.RepoName], star.Login)
		removed = append(removed, star)
	}
	return removed, s.save()
}

// Set updates the store to exactly match the provided stargazers per repository.
// It returns the stargazers that were added and those that were removed.
func (s *Store) Set(stargazers []github.Stargazer) ([]github.Stargazer, []github.Stargazer, error) {
	// Build desired state: repo -> login -> RepoStar
	desired := indexedStargazers(stargazers)
	added := repoDiff(desired, s.stargazers)
	removed := repoDiff(s.stargazers, desired)

	s.lock.Lock()
	defer s.lock.Unlock()
	s.stargazers = desired
	if err := s.save(); err != nil {
		return nil, nil, err
	}
	return added, removed, nil
}

// indexedStargazers indexes the stargazers by repository and user.
func indexedStargazers(stargazers []github.Stargazer) map[string]map[string]github.Stargazer {
	index := make(map[string]map[string]github.Stargazer)
	for _, star := range stargazers {
		if _, ok := index[star.RepoName]; !ok {
			index[star.RepoName] = make(map[string]github.Stargazer)
		}
		index[star.RepoName][star.Login] = star
	}
	return index
}

// repoDiff returns the stargazers from a that are not in b.
func repoDiff(a, b map[string]map[string]github.Stargazer) []github.Stargazer {
	var diff []github.Stargazer

	for repo, users := range a {
		for user, star := range users {
			if _, ok := b[repo][user]; !ok {
				diff = append(diff, star)
			}
		}
	}
	return diff
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

// Set updates the store to the provided stargazers.
// Notifies the Notifier if there were any new or removed stargazers.
func (s NotifyingStore) Set(ctx context.Context, stars []github.Stargazer) error {
	added, deleted, err := s.Store.Set(stars)
	if err == nil && len(added) > 0 {
		s.Notify(ctx, true, added)
	}
	if err == nil && len(deleted) > 0 {
		s.Notify(ctx, false, deleted)
	}
	return err
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
	for _, stargazers := range stargazersByRepo(stars) {
		err := slack.PostWebhook(s.WebHookURL, &slack.WebhookMessage{
			Text:        s.makeMessage(stargazers, added),
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
