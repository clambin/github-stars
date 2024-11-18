package stars

import (
	"encoding/json"
	"fmt"
	ggh "github.com/google/go-github/v65/github"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Store struct {
	DatabasePath string
	Repos        map[string]repository
	lock         sync.RWMutex
}

type repository map[string]stargazer

type stargazer struct {
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
	return s.read(f)
}

func (s *Store) read(r io.Reader) error {
	return json.NewDecoder(r).Decode(&s.Repos)
}

func (s *Store) Save() error {
	s.lock.Lock()
	defer s.lock.Unlock()
	f, err := os.Create(filepath.Join(s.DatabasePath, "stargazers.json"))
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer func() { _ = f.Close() }()
	return s.write(f)
}

func (s *Store) write(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(s.Repos)
}

func (s *Store) Add(repo *ggh.Repository, newStargazer *ggh.Stargazer) (bool, error) {
	if !s.addIfNew(repo, newStargazer) {
		return false, nil
	}
	// TODO: other processors may have added more repos between addIfNew and Save, meaning at least one Save is unnecessary.
	// possibly add "dirty" to Store and only save if needed.
	return true, s.Save()
}

func (s *Store) addIfNew(repo *ggh.Repository, newStargazer *ggh.Stargazer) bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.Repos == nil {
		s.Repos = make(map[string]repository)
	}
	r, found := s.Repos[repo.GetFullName()]
	if !found {
		if r == nil {
			r = make(map[string]stargazer, 1)
		}
		r[newStargazer.GetUser().GetLogin()] = stargazer{StarredAt: newStargazer.GetStarredAt().Time}
		s.Repos[repo.GetFullName()] = r
	}
	return !found
}
