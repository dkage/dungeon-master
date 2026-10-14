// Package worktree gives an issue session its own working copy: a `git
// worktree` kept in Dungeon Master's local state, outside the repository and
// out of the team's view. It only runs git with argv, never through a shell.
// See docs/design/sessions.md ("Working copies for parallel issue sessions").
package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Copy is one session's working copy.
type Copy struct {
	// Path is the root of the copy; Branch is checked out in it.
	Path   string `json:"path"`
	Branch string `json:"branch"`
	// Start is the commit the copy began at, and StartName what it was taken
	// from (the branch the main tree was on).
	Start     string `json:"start"`
	StartName string `json:"startName"`
}

// ErrBranchInUse means the issue's branch is already checked out in another
// working copy, so the issue already has a session.
var ErrBranchInUse = errors.New("worktree: the issue's branch is already checked out in another working copy")

// Runner runs git in dir and returns its trimmed standard output.
type Runner func(ctx context.Context, dir string, args ...string) (string, error)

// ExecRunner runs the real git. It never touches the network.
func ExecRunner(ctx context.Context, dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	// core.fsmonitor is disabled so inspecting an untrusted repository cannot run a program its config names.
	full := append([]string{"-C", dir, "-c", "core.fsmonitor=false"}, args...)
	cmd := exec.CommandContext(ctx, "git", full...)
	cmd.Env = append(cmd.Environ(), "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")
	cmd.WaitDelay = time.Second
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return "", fmt.Errorf("git %s: %s", args[0], strings.TrimSpace(string(ee.Stderr)))
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// Manager creates and removes working copies under Root.
type Manager struct {
	// Root is <state dir>/worktrees.
	Root string
	Git  Runner
}

func (m *Manager) git() Runner {
	if m.Git != nil {
		return m.Git
	}
	return ExecRunner
}

var idPattern = regexp.MustCompile(`^[a-z0-9]{1,40}$`)

// Branch is the branch name for an issue: built from its number only, never
// from its title.
func Branch(issue int) string { return "dungeon/issue-" + strconv.Itoa(issue) }

// Dir is where a session's copy lives. Both ids are app-generated and are
// checked here too, so a hostile value cannot leave Root.
func (m *Manager) Dir(campaignID, sessionID string) (string, error) {
	if !idPattern.MatchString(campaignID) || !idPattern.MatchString(sessionID) {
		return "", fmt.Errorf("worktree: unsafe id")
	}
	return filepath.Join(m.Root, campaignID, sessionID), nil
}

// Plan describes what Create would do, for the launch review: where the copy
// goes, its branch, and the commit it starts from.
func (m *Manager) Plan(ctx context.Context, repoRoot, campaignID string, issue int) (Copy, error) {
	name, err := m.git()(ctx, repoRoot, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return Copy{}, err
	}
	sha, err := m.git()(ctx, repoRoot, "rev-parse", "--short=12", "HEAD")
	if err != nil {
		return Copy{}, err
	}
	if name == "HEAD" {
		name = "a detached commit"
	}
	if !idPattern.MatchString(campaignID) {
		return Copy{}, fmt.Errorf("worktree: unsafe id")
	}
	return Copy{
		Path:   filepath.Join(m.Root, campaignID, "<session>"),
		Branch: Branch(issue), Start: sha, StartName: name,
	}, nil
}

// Create makes the working copy for a session, reusing the issue's branch when
// it exists and is not checked out anywhere.
func (m *Manager) Create(ctx context.Context, repoRoot, campaignID, sessionID string, issue int) (Copy, error) {
	dir, err := m.Dir(campaignID, sessionID)
	if err != nil {
		return Copy{}, err
	}
	plan, err := m.Plan(ctx, repoRoot, campaignID, issue)
	if err != nil {
		return Copy{}, err
	}
	// A copy deleted by hand leaves metadata that makes Git refuse the branch.
	_, _ = m.git()(ctx, repoRoot, "worktree", "prune")
	if inUse, err := m.branchCheckedOut(ctx, repoRoot, plan.Branch); err != nil {
		return Copy{}, err
	} else if inUse {
		return Copy{}, ErrBranchInUse
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o700); err != nil {
		return Copy{}, err
	}
	exists := false
	if _, err := m.git()(ctx, repoRoot, "show-ref", "--verify", "--quiet", "refs/heads/"+plan.Branch); err == nil {
		exists = true
	}
	args := []string{"worktree", "add", "-q"}
	if exists {
		args = append(args, dir, plan.Branch)
	} else {
		args = append(args, "-b", plan.Branch, dir, "HEAD")
	}
	if _, err := m.git()(ctx, repoRoot, args...); err != nil {
		return Copy{}, err
	}
	plan.Path = dir
	return plan, nil
}

func (m *Manager) branchCheckedOut(ctx context.Context, repoRoot, branch string) (bool, error) {
	out, err := m.git()(ctx, repoRoot, "worktree", "list", "--porcelain")
	if err != nil {
		return false, err
	}
	for _, l := range strings.Split(out, "\n") {
		if strings.TrimSpace(l) == "branch refs/heads/"+branch {
			return true, nil
		}
	}
	return false, nil
}

// Changes lists what is modified or untracked in a copy.
func (m *Manager) Changes(ctx context.Context, copyPath string) ([]string, error) {
	out, err := m.git()(ctx, copyPath, "status", "--porcelain")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, l := range strings.Split(out, "\n") {
		if strings.TrimSpace(l) != "" {
			files = append(files, strings.TrimSpace(l))
		}
	}
	return files, nil
}

// Result is what Remove did.
type Result struct {
	Removed       bool     `json:"removed"`
	BranchDeleted bool     `json:"branchDeleted"`
	Kept          bool     `json:"kept"`
	Changes       []string `json:"changes,omitempty"`
}

// Remove deletes a session's copy when it is clean. A copy with uncommitted or
// untracked work is kept and reported, never discarded. The branch goes with a
// removed copy only when it has no commits of its own.
func (m *Manager) Remove(ctx context.Context, repoRoot string, c Copy) (Result, error) {
	if _, err := os.Stat(c.Path); errors.Is(err, os.ErrNotExist) {
		_, _ = m.git()(ctx, repoRoot, "worktree", "prune")
		return Result{Removed: true, BranchDeleted: m.dropEmptyBranch(ctx, repoRoot, c)}, nil
	}
	changes, err := m.Changes(ctx, c.Path)
	if err != nil {
		return Result{Kept: true}, err
	}
	if len(changes) > 0 {
		return Result{Kept: true, Changes: changes}, nil
	}
	var rmErr error
	// Windows may hold a file for a moment after the agent was killed.
	for attempt := 0; attempt < 4; attempt++ {
		if _, rmErr = m.git()(ctx, repoRoot, "worktree", "remove", c.Path); rmErr == nil {
			break
		}
		if runtime.GOOS != "windows" {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if rmErr != nil {
		return Result{Kept: true}, rmErr
	}
	return Result{Removed: true, BranchDeleted: m.dropEmptyBranch(ctx, repoRoot, c)}, nil
}

// dropEmptyBranch deletes the issue's branch when nothing was committed on it.
func (m *Manager) dropEmptyBranch(ctx context.Context, repoRoot string, c Copy) bool {
	if c.Branch == "" || c.Start == "" {
		return false
	}
	n, err := m.git()(ctx, repoRoot, "rev-list", "--count", c.Start+".."+c.Branch)
	if err != nil || n != "0" {
		return false
	}
	_, err = m.git()(ctx, repoRoot, "branch", "-d", c.Branch)
	return err == nil
}

// Entry is something the main tree holds that a fresh copy does not: an
// ignored or untracked top-level file or folder.
type Entry struct {
	Name    string `json:"name"`
	Dir     bool   `json:"dir"`
	Ignored bool   `json:"ignored"`
}

// Entries lists the ignored and untracked top-level entries of the main tree.
func (m *Manager) Entries(ctx context.Context, repoRoot string) ([]Entry, error) {
	var out []Entry
	seen := map[string]bool{}
	for _, mode := range []struct {
		args    []string
		ignored bool
	}{
		{[]string{"ls-files", "--others", "--ignored", "--exclude-standard", "--directory"}, true},
		{[]string{"ls-files", "--others", "--exclude-standard", "--directory"}, false},
	} {
		text, err := m.git()(ctx, repoRoot, mode.args...)
		if err != nil {
			return nil, err
		}
		for _, l := range strings.Split(text, "\n") {
			l = strings.TrimSpace(l)
			if l == "" {
				continue
			}
			dir := strings.HasSuffix(l, "/")
			name := strings.TrimSuffix(l, "/")
			if strings.Contains(name, "/") || seen[name] || !SafeEntryName(name) {
				continue // only top-level entries are offered
			}
			seen[name] = true
			out = append(out, Entry{Name: name, Dir: dir, Ignored: mode.ignored})
		}
	}
	return out, nil
}

// SafeEntryName accepts a single path component.
func SafeEntryName(name string) bool {
	if name == "" || name == "." || name == ".." || name == ".git" {
		return false
	}
	return !strings.ContainsAny(name, `/\`) && !strings.ContainsRune(name, 0)
}

// How an entry reaches the copy.
const (
	CarryCopy = "copy"
	CarryLink = "link"
)

// Carry puts chosen entries of the main tree into a copy: a small file is
// copied, anything else (or a requested link) is symlinked. Only names from
// Entries are accepted.
func Carry(repoRoot, copyPath string, names map[string]string, allowed []Entry) error {
	ok := map[string]Entry{}
	for _, e := range allowed {
		ok[e.Name] = e
	}
	for name, how := range names {
		e, listed := ok[name]
		if !listed || !SafeEntryName(name) {
			return fmt.Errorf("worktree: %q is not an entry that can be carried over", name)
		}
		src, dst := filepath.Join(repoRoot, name), filepath.Join(copyPath, name)
		if _, err := os.Lstat(dst); err == nil {
			continue // the copy already has it (tracked)
		}
		switch {
		case how == CarryCopy && !e.Dir:
			data, err := os.ReadFile(src)
			if err != nil {
				return err
			}
			if err := os.WriteFile(dst, data, 0o600); err != nil {
				return err
			}
		case how == CarryLink || how == CarryCopy:
			if err := os.Symlink(src, dst); err != nil {
				return err
			}
		default:
			return fmt.Errorf("worktree: unknown way %q to carry %q", how, name)
		}
	}
	return nil
}
