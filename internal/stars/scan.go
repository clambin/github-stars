package stars

import (
	"context"
	"fmt"

	"github.com/clambin/github-stars/internal/github"
)

type Client interface {
	Stargazers(context.Context, string) ([]github.Stargazer, error)
}

var _ Client = (*github.Client)(nil)

// Scan retrieves all repositories for the user, gets the stars for each repository and adds new ones to the Store.
func Scan(ctx context.Context, user string, c Client, s *NotifyingStore) error {
	stargazers, err := c.Stargazers(ctx, user)
	if err != nil {
		return fmt.Errorf("stars: %w", err)
	}

	if err = s.Set(ctx, stargazers); err != nil {
		return fmt.Errorf("add: %w", err)
	}
	return nil
}
