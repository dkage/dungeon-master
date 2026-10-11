package github

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dkage/dungeon-master/internal/quests"
	"github.com/dkage/dungeon-master/internal/testutil"
)

func fakeGH(t *testing.T, options testutil.ProgramOptions) string {
	return testutil.Program(t, "gh", options)
}

func TestEnvOrGHPrecedence(t *testing.T) {
	gh := fakeGH(t, testutil.ProgramOptions{Output: "from-gh"})
	ctx := context.Background()

	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	if tok, err := EnvOrGH(gh)(ctx); err != nil || tok != "from-gh" {
		t.Fatalf("gh: %q %v", tok, err)
	}
	t.Setenv("GITHUB_TOKEN", "from-github-token")
	if tok, _ := EnvOrGH(gh)(ctx); tok != "from-github-token" {
		t.Fatalf("GITHUB_TOKEN should beat gh: %q", tok)
	}
	t.Setenv("GH_TOKEN", "  from-gh-token \n")
	if tok, _ := EnvOrGH(gh)(ctx); tok != "from-gh-token" {
		t.Fatalf("GH_TOKEN should win and be trimmed: %q", tok)
	}
}

func TestEnvOrGHFailureModes(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	ctx := context.Background()
	for name, path := range map[string]string{
		"no gh configured":  "",
		"gh missing":        "/nonexistent/gh",
		"gh fails":          fakeGH(t, testutil.ProgramOptions{Error: "not logged in", Exit: 1}),
		"gh prints nothing": fakeGH(t, testutil.ProgramOptions{}),
		"gh prints blanks":  fakeGH(t, testutil.ProgramOptions{Output: "   "}),
	} {
		if _, err := EnvOrGH(path)(ctx); !errors.Is(err, quests.ErrNoToken) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestEnvOrGHTimesOut(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	gh := fakeGH(t, testutil.ProgramOptions{SleepMS: 30000})
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := EnvOrGH(gh)(ctx); !errors.Is(err, quests.ErrNoToken) {
		t.Fatalf("err %v", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("did not honour the context")
	}
}

func TestTokenCache(t *testing.T) {
	var calls atomic.Int32
	now := time.Unix(1000, 0)
	c := &cached{
		src: func(context.Context) (string, error) { calls.Add(1); return "t", nil },
		ttl: time.Minute,
		now: func() time.Time { return now },
	}
	ctx := context.Background()
	for range 3 {
		if tok, err := c.get(ctx); err != nil || tok != "t" {
			t.Fatal(tok, err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("source called %d times, want 1", calls.Load())
	}
	now = now.Add(2 * time.Minute) // expired
	_, _ = c.get(ctx)
	if calls.Load() != 2 {
		t.Fatalf("after expiry: %d calls", calls.Load())
	}
	c.forget()
	_, _ = c.get(ctx)
	if calls.Load() != 3 {
		t.Fatalf("after forget: %d calls", calls.Load())
	}
	// Errors are not cached.
	bad := &cached{src: func(context.Context) (string, error) { return "", quests.ErrNoToken }, ttl: time.Minute, now: time.Now}
	if _, err := bad.get(ctx); err == nil {
		t.Fatal("expected error")
	}
}
