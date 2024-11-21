package store

import (
	"encoding/json"
	"fmt"
	"github.com/google/go-github/v66/github"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Store struct {
	DatabasePath string
	Repos        map[string]RepoStars
	lock         sync.RWMutex
}

func New(databasePath string) *Store {
	return &Store{
		DatabasePath: databasePath,
		Repos:        make(map[string]RepoStars),
	}
}

type RepoStars map[string]RepoStar

func (r RepoStars) Equals(o RepoStars) bool {
	if len(r) != len(o) {
		return false
	}
	for login := range r {
		if _, ok := o[login]; !ok {
			return false
		}
	}
	return true
}

type RepoStar struct {
	StarredAt time.Time `json:"starred_at"`
}

func (s *Store) Load() error {
	s.lock.Lock()
	defer s.lock.Unlock()
	f, err := os.Open(filepath.Join(s.DatabasePath, "stargazers.json"))
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer func() { _ = f.Close() }()
	return json.NewDecoder(f).Decode(&s.Repos)
}

func (s *Store) save() error {
	f, err := os.Create(filepath.Join(s.DatabasePath, "stargazers.json"))
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer func() { _ = f.Close() }()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(s.Repos)
}

// Len returns the number of repos in the Store.
func (s *Store) Len() int {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return len(s.Repos)
}

// SetStargazers sets the stargazers for a repo and returns the new stargazers. It returns an error if it failed to save the store to disk.
func (s *Store) SetStargazers(repo *github.Repository, stargazers []*github.Stargazer) ([]*github.Stargazer, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	repoFullName := repo.GetFullName()
	oldRepoStars := s.Repos[repoFullName]
	newStargazers := s.getNewStargazers(oldRepoStars, stargazers)
	newRepoStars := makeRepoStars(stargazers)

	var err error
	if !oldRepoStars.Equals(newRepoStars) {
		s.Repos[repoFullName] = newRepoStars
		err = s.save()
	}
	return newStargazers, err
}

// getNewStarGazers returns all StarGazers that are not yet in the stored repoStars.
func (s *Store) getNewStargazers(stored RepoStars, found []*github.Stargazer) []*github.Stargazer {
	stargazers := make([]*github.Stargazer, 0, len(found))
	for _, stargazer := range found {
		if _, ok := stored[stargazer.GetUser().GetLogin()]; !ok {
			stargazers = append(stargazers, stargazer)
		}
	}
	return stargazers
}

// makeRepoStars converts a slice of stargazers to a RepoStars map.
func makeRepoStars(stargazers []*github.Stargazer) RepoStars {
	gazers := make(RepoStars, len(stargazers))
	for _, stargazer := range stargazers {
		gazers[stargazer.GetUser().GetLogin()] = RepoStar{StarredAt: stargazer.GetStarredAt().Time}
	}
	return gazers
}

// Add adds a stargazer to a repo. Returns true if the stargazer is new. Error indicates a problem saving the store to disk,
func (s *Store) Add(repo *github.Repository, stargazer *github.Stargazer) (bool, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	r, ok := s.Repos[repo.GetFullName()]
	if !ok {
		r = make(RepoStars)
	}
	_, ok = r[stargazer.GetUser().GetLogin()]
	if ok {
		return false, nil
	}
	r[stargazer.GetUser().GetLogin()] = RepoStar{StarredAt: time.Now()}
	s.Repos[repo.GetFullName()] = r
	return true, s.save()
}

// Delete removes a stargazer from a repo. Returns true if the stargazer was present. Error indicates a problem saving the store to disk,
func (s *Store) Delete(repo *github.Repository, stargazer *github.Stargazer) (bool, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	r, ok := s.Repos[repo.GetFullName()]
	if !ok {
		return false, nil
	}
	_, ok = r[stargazer.GetUser().GetLogin()]
	if !ok {
		return false, nil
	}
	delete(r, stargazer.GetUser().GetLogin())
	s.Repos[repo.GetFullName()] = r
	return true, s.save()
}
