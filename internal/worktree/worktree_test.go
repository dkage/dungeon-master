package worktree

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// repo makes a throwaway Git repository with one commit, ignoring the user's
// own Git configuration.
func repo(t *testing.T) string {
	t.Helper()
	empty := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", empty)
	t.Setenv("GIT_CONFIG_SYSTEM", empty)
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "t@example.test")
	run("config", "user.name", "t")
	write(t, filepath.Join(dir, ".gitignore"), "node_modules/\n.env\n")
	write(t, filepath.Join(dir, "a.txt"), "one\n")
	run("add", "a.txt", ".gitignore")
	run("commit", "-q", "-m", "init")
	return dir
}

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func manager(t *testing.T) *Manager { return &Manager{Root: filepath.Join(t.TempDir(), "worktrees")} }

func TestCreateMakesACopyOutsideTheRepositoryAndLeavesTheMainTreeAlone(t *testing.T) {
	main := repo(t)
	write(t, filepath.Join(main, "a.txt"), "one\ndirty\n") // uncommitted in the main tree
	write(t, filepath.Join(main, "note.txt"), "scratch")
	before := gitOut(t, main, "status", "--short")
	m := manager(t)
	c, err := m.Create(context.Background(), main, "c1", "s1", 12)
	if err != nil {
		t.Fatal(err)
	}
	if c.Branch != "dungeon/issue-12" || c.StartName != "main" || len(c.Start) != 12 {
		t.Fatalf("copy %+v", c)
	}
	if rel, err := filepath.Rel(main, c.Path); err == nil && !strings.HasPrefix(rel, "..") {
		t.Fatalf("the copy %s is inside the repository", c.Path)
	}
	if got := gitOut(t, main, "status", "--short"); got != before {
		t.Fatalf("the main tree changed:\nbefore %q\nafter  %q", before, got)
	}
	if b, _ := os.ReadFile(filepath.Join(c.Path, "a.txt")); string(b) != "one\n" {
		t.Fatalf("the copy holds the committed file, not the main tree's edit: %q", b)
	}
	if _, err := os.Stat(filepath.Join(c.Path, "note.txt")); err == nil {
		t.Fatal("an untracked file reached the copy")
	}
}

func TestTwoIssuesGetSeparateCopiesAndTheSameIssueIsRefused(t *testing.T) {
	main := repo(t)
	m := manager(t)
	ctx := context.Background()
	a, err := m.Create(ctx, main, "c1", "s1", 1)
	if err != nil {
		t.Fatal(err)
	}
	b, err := m.Create(ctx, main, "c1", "s2", 2)
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(a.Path, "only-a.txt"), "a")
	if _, err := os.Stat(filepath.Join(b.Path, "only-a.txt")); err == nil {
		t.Fatal("a file written in one copy appeared in the other")
	}
	if _, err := os.Stat(filepath.Join(main, "only-a.txt")); err == nil {
		t.Fatal("a file written in a copy appeared in the main tree")
	}
	if _, err := m.Create(ctx, main, "c1", "s3", 1); !errors.Is(err, ErrBranchInUse) {
		t.Fatalf("a second session for issue 1: %v, want ErrBranchInUse", err)
	}
}

func TestIdsAndNamesCannotEscapeOrInject(t *testing.T) {
	m := manager(t)
	for _, bad := range []string{"", "..", "../x", "a/b", "a b", "C1", "a;rm", strings.Repeat("a", 41)} {
		if _, err := m.Dir(bad, "s1"); err == nil {
			t.Errorf("campaign id %q accepted", bad)
		}
		if _, err := m.Dir("c1", bad); err == nil {
			t.Errorf("session id %q accepted", bad)
		}
	}
	if Branch(7) != "dungeon/issue-7" {
		t.Fatal(Branch(7))
	}
	for _, bad := range []string{"", ".", "..", ".git", "a/b", `a\b`, "a\x00b"} {
		if SafeEntryName(bad) {
			t.Errorf("entry name %q accepted", bad)
		}
	}
}

func TestRemoveDeletesACleanCopyAndItsEmptyBranch(t *testing.T) {
	main := repo(t)
	m := manager(t)
	ctx := context.Background()
	c, _ := m.Create(ctx, main, "c1", "s1", 5)
	res, err := m.Remove(ctx, main, c)
	if err != nil || !res.Removed || !res.BranchDeleted || res.Kept {
		t.Fatalf("remove: %+v %v", res, err)
	}
	if _, err := os.Stat(c.Path); err == nil {
		t.Fatal("the copy is still there")
	}
	if out := gitOut(t, main, "branch", "--list", "dungeon/*"); out != "" {
		t.Fatalf("the empty branch was kept: %q", out)
	}
}

func TestRemoveKeepsADirtyCopyAndABranchWithCommits(t *testing.T) {
	main := repo(t)
	m := manager(t)
	ctx := context.Background()

	dirty, _ := m.Create(ctx, main, "c1", "s1", 6)
	write(t, filepath.Join(dirty.Path, "wip.txt"), "unfinished")
	res, err := m.Remove(ctx, main, dirty)
	if err != nil || !res.Kept || res.Removed || len(res.Changes) != 1 || !strings.Contains(res.Changes[0], "wip.txt") {
		t.Fatalf("dirty copy: %+v %v", res, err)
	}
	if b, _ := os.ReadFile(filepath.Join(dirty.Path, "wip.txt")); string(b) != "unfinished" {
		t.Fatal("unfinished work was lost")
	}

	committed, _ := m.Create(ctx, main, "c1", "s2", 7)
	write(t, filepath.Join(committed.Path, "done.txt"), "done")
	gitOut(t, committed.Path, "add", "done.txt")
	gitOut(t, committed.Path, "commit", "-q", "-m", "work")
	res, err = m.Remove(ctx, main, committed)
	if err != nil || !res.Removed || res.BranchDeleted {
		t.Fatalf("committed copy: %+v %v", res, err)
	}
	if out := gitOut(t, main, "branch", "--list", "dungeon/issue-7"); out == "" {
		t.Fatal("a branch with commits was deleted")
	}
	// The branch can be reused by a later session of the same issue.
	again, err := m.Create(ctx, main, "c1", "s3", 7)
	if err != nil {
		t.Fatalf("reusing the branch: %v", err)
	}
	if _, err := os.Stat(filepath.Join(again.Path, "done.txt")); err != nil {
		t.Fatal("the reused branch lost its commit")
	}
}

func TestACopyDeletedByHandIsPrunedAndRecreated(t *testing.T) {
	main := repo(t)
	m := manager(t)
	ctx := context.Background()
	c, _ := m.Create(ctx, main, "c1", "s1", 8)
	if err := os.RemoveAll(c.Path); err != nil {
		t.Fatal(err)
	}
	again, err := m.Create(ctx, main, "c1", "s2", 8)
	if err != nil {
		t.Fatalf("recreating after a hand deletion: %v", err)
	}
	if _, err := os.Stat(filepath.Join(again.Path, "a.txt")); err != nil {
		t.Fatal(err)
	}
	// Removing a copy that is already gone succeeds.
	gone := Copy{Path: filepath.Join(m.Root, "c1", "s9"), Branch: Branch(9), Start: c.Start}
	if res, err := m.Remove(ctx, main, gone); err != nil || !res.Removed {
		t.Fatalf("remove of a missing copy: %+v %v", res, err)
	}
}

func TestEntriesAndCarry(t *testing.T) {
	main := repo(t)
	write(t, filepath.Join(main, ".env"), "SECRET=1\n")
	write(t, filepath.Join(main, "node_modules", "dep.js"), "x")
	write(t, filepath.Join(main, "note.txt"), "scratch")
	m := manager(t)
	ctx := context.Background()
	entries, err := m.Entries(ctx, main)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Entry{}
	for _, e := range entries {
		got[e.Name] = e
	}
	if !got[".env"].Ignored || !got["node_modules"].Dir || !got["node_modules"].Ignored || got["note.txt"].Ignored {
		t.Fatalf("entries %+v", entries)
	}
	c, _ := m.Create(ctx, main, "c1", "s1", 3)
	if err := Carry(main, c.Path, map[string]string{".env": CarryCopy, "node_modules": CarryLink}, entries); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(c.Path, ".env")); string(b) != "SECRET=1\n" {
		t.Fatalf(".env in the copy: %q", b)
	}
	if info, err := os.Lstat(filepath.Join(c.Path, "node_modules")); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("node_modules should be a link: %v %v", info, err)
	}
	// Names that were not offered, or that climb out, are refused.
	for _, name := range []string{"../escape", ".git", "not-listed"} {
		if err := Carry(main, c.Path, map[string]string{name: CarryCopy}, entries); err == nil {
			t.Errorf("carrying %q was accepted", name)
		}
	}
}

func TestPlanReportsTheStartWithoutCreatingAnything(t *testing.T) {
	main := repo(t)
	m := manager(t)
	p, err := m.Plan(context.Background(), main, "c1", 4)
	if err != nil || p.Branch != "dungeon/issue-4" || p.StartName != "main" {
		t.Fatalf("plan %+v %v", p, err)
	}
	if _, err := os.Stat(m.Root); err == nil {
		t.Fatal("planning created the worktrees folder")
	}
}
