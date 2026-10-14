package skillsource

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// GitFetcher fetches with the git binary, run with direct argv and no
// prompts. It checks the pin out detached, so no hook or script from the
// repository runs, and removes the .git directory afterwards.
type GitFetcher struct{}

func (GitFetcher) Fetch(ctx context.Context, location, pin, dest string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	run := func(dir string, args ...string) (string, error) {
		cmd := exec.CommandContext(ctx, "git", append([]string{
			"-c", "protocol.ext.allow=never", "-c", "core.hooksPath=" + os.DevNull,
		}, args...)...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=true")
		var out, errb bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errb
		if err := cmd.Run(); err != nil {
			msg := strings.TrimSpace(errb.String())
			if msg == "" {
				msg = err.Error()
			}
			return "", fmt.Errorf("git %s: %s", args[0], firstLine(msg))
		}
		return strings.TrimSpace(out.String()), nil
	}
	if pin == "" {
		pin = "HEAD"
	}
	if err := os.MkdirAll(dest, 0o700); err != nil {
		return "", err
	}
	if _, err := run(dest, "init", "--quiet"); err != nil {
		return "", err
	}
	if _, err := run(dest, "fetch", "--quiet", "--depth", "1", "--no-tags", "--", location, pin); err != nil {
		return "", err
	}
	if _, err := run(dest, "-c", "advice.detachedHead=false", "checkout", "--quiet", "--detach", "FETCH_HEAD"); err != nil {
		return "", err
	}
	rev, err := run(dest, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	if err := os.RemoveAll(dest + string(os.PathSeparator) + ".git"); err != nil {
		return "", err
	}
	return rev, nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
