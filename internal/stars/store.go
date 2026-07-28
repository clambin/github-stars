package stars

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/clambin/github-stars/internal/github"
)

const StoreFilename = "stargazers.json"

// Store contains all stars for all repositories
type Store struct {
	stargazers   map[string]map[string]github.Stargazer
	databasePath string
	lock         sync.RWMutex
}

// NewStore creates a new Store
func NewStore(databasePath string) (store *Store, err error) {
	store = &Store{databasePath: databasePath}
	f, err := os.Open(filepath.Join(databasePath, StoreFilename))
	switch {
	case err == nil:
		defer func() { _ = f.Close() }()
		var stargazers []github.Stargazer
		if err = json.NewDecoder(f).Decode(&stargazers); err != nil {
			return nil, fmt.Errorf("decode: %w", err)
		}
		if len(stargazers) > 0 {
			store.stargazers = indexedStargazers(stargazers)
		}
	case os.IsNotExist(err):
		store.stargazers = make(map[string]map[string]github.Stargazer)
		err = nil
	default:
		return nil, err
	}
	return store, err
}

// save saves the store to disk
func (s *Store) save() error {
	var stargazers []github.Stargazer
	for _, users := range s.stargazers {
		for _, repoStargazer := range users {
			stargazers = append(stargazers, repoStargazer)
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
	store    *Store
	notifier Notifier
}

// NewNotifyingStore creates a new NotifyingStore.
func NewNotifyingStore(databaseDirectory string, notifiers Notifier) (*NotifyingStore, error) {
	store, err := NewStore(databaseDirectory)
	if err != nil {
		return nil, err
	}
	return &NotifyingStore{
		store:    store,
		notifier: notifiers,
	}, nil
}

// Add adds new stargazers to a repository.
// Notifies the Notifier if there were any new stargazers.
func (s NotifyingStore) Add(ctx context.Context, stars ...github.Stargazer) error {
	added, err := s.store.Add(stars...)
	if err == nil && len(added) > 0 {
		s.notifier.Notify(ctx, true, added...)
	}
	return err
}

// Delete removes stargazers from a repository.
// Notifies the Notifier if there were any removed stargazers.
func (s NotifyingStore) Delete(ctx context.Context, stars ...github.Stargazer) error {
	deleted, err := s.store.Delete(stars...)
	if err == nil && len(deleted) > 0 {
		s.notifier.Notify(ctx, false, deleted...)
	}
	return err
}

// Set updates the store to the provided stargazers.
// Notifies the Notifier if there were any new or removed stargazers.
func (s NotifyingStore) Set(ctx context.Context, stars []github.Stargazer) error {
	added, deleted, err := s.store.Set(stars)
	if err == nil && len(added) > 0 {
		s.notifier.Notify(ctx, true, added...)
	}
	if err == nil && len(deleted) > 0 {
		s.notifier.Notify(ctx, false, deleted...)
	}
	return err
}
