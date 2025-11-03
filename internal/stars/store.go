package stars

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/go-github/v76/github"
)

// RepoStar is a single star for a repository
type RepoStar struct {
	StarredAt time.Time `json:"starred_at"`
}

// RepoStars contains all stars for a repository
type RepoStars map[string]RepoStar

// Store contains all stars for all repositories
type Store struct {
	DatabasePath string
	Repos        map[string]RepoStars
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
	User      string
	StarredAt time.Time
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

func (s NotifyingStore) Add(ctx context.Context, repo *github.Repository, stargazers ...*github.Stargazer) error {
	added, err := s.Store.Add(repo, stargazers...)
	if err == nil && len(added) > 0 {
		s.Notify(ctx, true, repo, added...)
	}
	return err
}

func (s NotifyingStore) Delete(ctx context.Context, repo *github.Repository, stargazers ...*github.Stargazer) error {
	deleted, err := s.Store.Delete(repo, stargazers...)
	if err == nil && len(deleted) > 0 {
		s.Notify(ctx, false, repo, stargazers...)
	}
	return err
}
