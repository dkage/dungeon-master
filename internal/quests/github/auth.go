package github

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/dkage/dungeon-master/internal/quests"
)

// TokenFunc returns a GitHub token.
type TokenFunc func(ctx context.Context) (string, error)

// EnvOrGH looks for a token in GH_TOKEN, then GITHUB_TOKEN, then asks the
// GitHub CLI (`gh auth token`) at ghPath. An empty ghPath skips the CLI.
// Credentials are used for requests to GitHub only: they are never stored,
// logged or sent to the browser.
func EnvOrGH(ghPath string) TokenFunc {
	return func(ctx context.Context) (string, error) {
		for _, name := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
			if v := strings.TrimSpace(os.Getenv(name)); v != "" {
				return v, nil
			}
		}
		if ghPath == "" {
			return "", quests.ErrNoToken
		}
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, ghPath, "auth", "token")
		// After a timeout kills gh, do not wait on pipes that a stray
		// grandchild process may still hold open.
		cmd.WaitDelay = time.Second
		out, err := cmd.Output()
		if err != nil {
			return "", quests.ErrNoToken
		}
		if tok := strings.TrimSpace(string(out)); tok != "" {
			return tok, nil
		}
		return "", quests.ErrNoToken
	}
}

// cached remembers a token for ttl so every request does not run a process.
// Forget drops it, for when GitHub rejects it.
type cached struct {
	src TokenFunc
	ttl time.Duration
	now func() time.Time

	mu      sync.Mutex
	tok     string
	expires time.Time
}

func (c *cached) get(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.tok != "" && c.now().Before(c.expires) {
		return c.tok, nil
	}
	tok, err := c.src(ctx)
	if err != nil {
		return "", err
	}
	c.tok, c.expires = tok, c.now().Add(c.ttl)
	return tok, nil
}

func (c *cached) forget() {
	c.mu.Lock()
	c.tok = ""
	c.mu.Unlock()
}
