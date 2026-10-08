package campaign

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// Git answers the two questions Dungeon asks of a repository. It exists so
// tests can fake it; ExecGit is the real implementation.
type Git interface {
	// Toplevel returns the root of the repository containing dir.
	Toplevel(ctx context.Context, dir string) (string, error)
	// RemoteURL returns the URL of a named remote, with insteadOf rewrites
	// applied.
	RemoteURL(ctx context.Context, root, remote string) (string, error)
}

// ExecGit runs the git binary. It never touches the network.
type ExecGit struct{}

func (ExecGit) run(ctx context.Context, dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// core.fsmonitor is disabled so that inspecting an untrusted repository
	// cannot run a program named by its config.
	full := append([]string{"-C", dir, "-c", "core.fsmonitor=false"}, args...)
	cmd := exec.CommandContext(ctx, "git", full...)
	cmd.Env = append(cmd.Environ(), "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (g ExecGit) Toplevel(ctx context.Context, dir string) (string, error) {
	return g.run(ctx, dir, "rev-parse", "--show-toplevel")
}

func (g ExecGit) RemoteURL(ctx context.Context, root, remote string) (string, error) {
	return g.run(ctx, root, "remote", "get-url", remote)
}

var nameSegment = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

// ParseGitHubRemote extracts owner and repository from a github.com remote
// URL. It understands https, ssh://, and scp-style (git@github.com:o/r.git)
// forms. Other hosts, extra path segments and malformed names are rejected.
func ParseGitHubRemote(raw string) (owner, repo string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", fmt.Errorf("empty remote url")
	}
	var host, path string
	if strings.Contains(raw, "://") {
		u, perr := url.Parse(raw)
		if perr != nil {
			return "", "", fmt.Errorf("unparseable remote url")
		}
		host, path = u.Hostname(), u.Path
	} else if at := strings.Index(raw, "@"); at >= 0 || strings.Contains(raw, ":") {
		// scp-like: [user@]host:path
		rest := raw
		if at >= 0 {
			rest = raw[at+1:]
		}
		h, p, ok := strings.Cut(rest, ":")
		if !ok {
			return "", "", fmt.Errorf("unrecognised remote url")
		}
		host, path = h, p
	} else {
		return "", "", fmt.Errorf("unrecognised remote url")
	}
	if host != "github.com" && host != "www.github.com" {
		return "", "", fmt.Errorf("not a github.com remote")
	}
	path = strings.Trim(path, "/")
	path = strings.TrimSuffix(path, ".git")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || !nameSegment.MatchString(parts[0]) || !nameSegment.MatchString(parts[1]) ||
		parts[0] == "." || parts[0] == ".." || parts[1] == "." || parts[1] == ".." {
		return "", "", fmt.Errorf("remote path is not owner/repo")
	}
	return parts[0], parts[1], nil
}
