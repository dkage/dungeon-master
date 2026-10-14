package skillsource

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const revA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const revB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

// fakeFetcher resolves every pin to rev, or fails with err, and writes one
// file so a checkout directory has content.
type fakeFetcher struct {
	rev   string
	err   error
	calls []string
}

func (f *fakeFetcher) Fetch(_ context.Context, location, pin, dest string) (string, error) {
	f.calls = append(f.calls, location+"@"+pin)
	if f.err != nil {
		return "", f.err
	}
	if err := os.MkdirAll(dest, 0o700); err != nil {
		return "", err
	}
	return f.rev, os.WriteFile(filepath.Join(dest, "SKILL.md"), []byte(f.rev), 0o600)
}

func open(t *testing.T, f Fetcher) (*Registry, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "skill-sources.json")
	r, err := Open(path, Options{Root: filepath.Join(dir, "checkouts"), Fetcher: f})
	if err != nil {
		t.Fatal(err)
	}
	return r, path
}

func addFolder(t *testing.T, r *Registry, name string) Source {
	t.Helper()
	s, err := r.Add(context.Background(), Input{Name: name, Kind: Folder, Location: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestAddFolderPersistsPrivatelyAndSurvivesReopen(t *testing.T) {
	r, path := open(t, &fakeFetcher{})
	s := addFolder(t, r, "mine")
	if !s.Enabled || s.Kind != Folder || s.ID == "" {
		t.Fatalf("source = %+v", s)
	}
	if runtime.GOOS != "windows" {
		fi, err := os.Stat(path)
		if err != nil || fi.Mode().Perm() != 0o600 {
			t.Fatalf("state file mode = %v, %v", fi.Mode(), err)
		}
	}
	again, err := Open(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := again.List(); len(got) != 1 || got[0].ID != s.ID || got[0].Location != s.Location {
		t.Fatalf("reopened = %+v", got)
	}
}

func TestAddRefusesBadInput(t *testing.T) {
	r, _ := open(t, &fakeFetcher{rev: revA})
	file := filepath.Join(t.TempDir(), "f")
	_ = os.WriteFile(file, nil, 0o600)
	for name, in := range map[string]Input{
		"no name":         {Kind: Folder, Location: t.TempDir()},
		"multiline name":  {Name: "a\nb", Kind: Folder, Location: t.TempDir()},
		"relative folder": {Name: "x", Kind: Folder, Location: "skills"},
		"missing folder":  {Name: "x", Kind: Folder, Location: filepath.Join(t.TempDir(), "nope")},
		"a file":          {Name: "x", Kind: Folder, Location: file},
		"bad kind":        {Name: "x", Kind: "zip", Location: t.TempDir()},
		"option remote":   {Name: "x", Kind: Git, Location: "--upload-pack=evil", Pin: "main"},
		"ext remote":      {Name: "x", Kind: Git, Location: "ext::sh -c evil", Pin: "main"},
		"http remote":     {Name: "x", Kind: Git, Location: "http://example.test/a.git", Pin: "main"},
		"option pin":      {Name: "x", Kind: Git, Location: "https://example.test/a.git", Pin: "--evil"},
		"dotdot pin":      {Name: "x", Kind: Git, Location: "https://example.test/a.git", Pin: "a..b"},
	} {
		_, err := r.Add(context.Background(), in)
		var ve ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("%s: err = %v, want a ValidationError", name, err)
		}
	}
	if len(r.List()) != 0 {
		t.Fatal("a refused source was registered")
	}
}

func TestAddRefusesDuplicateLocation(t *testing.T) {
	r, _ := open(t, &fakeFetcher{})
	dir := t.TempDir()
	in := Input{Name: "a", Kind: Folder, Location: dir}
	if _, err := r.Add(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Add(context.Background(), in); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("err = %v", err)
	}
}

func TestGitSourceIsPinnedAndFetchedOnRegistration(t *testing.T) {
	f := &fakeFetcher{rev: revA}
	r, _ := open(t, f)
	s, err := r.Add(context.Background(), Input{Name: "ext", Kind: Git, Location: "https://example.test/skills.git", Pin: "v1.2.0",
		License: "MIT", Notice: "credit"})
	if err != nil {
		t.Fatal(err)
	}
	if s.Pin != "v1.2.0" || s.Verified == nil || s.Verified.Revision != revA || s.License != "MIT" || s.Notice != "credit" {
		t.Fatalf("source = %+v", s)
	}
	if _, err := os.Stat(filepath.Join(s.Verified.Dir, "SKILL.md")); err != nil {
		t.Fatalf("checkout missing: %v", err)
	}
	if len(f.calls) != 1 {
		t.Fatalf("fetched %v", f.calls)
	}
	// Listing and reading never fetch: an update happens only on request.
	r.List()
	r.Get(s.ID)
	if len(f.calls) != 1 {
		t.Fatalf("a read fetched: %v", f.calls)
	}
}

func TestGitSourceThatCannotBeFetchedIsNotRegistered(t *testing.T) {
	r, _ := open(t, &fakeFetcher{err: errors.New("repository not found")})
	_, err := r.Add(context.Background(), Input{Name: "x", Kind: Git, Location: "https://example.test/a.git", Pin: "main"})
	var ve ValidationError
	if !errors.As(err, &ve) || !strings.Contains(err.Error(), "repository not found") {
		t.Fatalf("err = %v", err)
	}
	if len(r.List()) != 0 {
		t.Fatal("registered anyway")
	}
}

func TestFailedRefreshKeepsTheLastVerifiedCheckout(t *testing.T) {
	f := &fakeFetcher{rev: revA}
	now := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	r, err := Open("", Options{Root: dir, Fetcher: f, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	s, err := r.Add(context.Background(), Input{Name: "ext", Kind: Git, Location: "https://example.test/a.git", Pin: "main"})
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(48 * time.Hour)
	f.err = errors.New("network is down")
	got, err := r.Refresh(context.Background(), s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Verified == nil || got.Verified.Revision != revA || !got.Verified.At.Equal(s.Verified.At) {
		t.Fatalf("verified = %+v", got.Verified)
	}
	if got.Error == nil || got.Error.Message != "network is down" || !got.Error.At.Equal(now) {
		t.Fatalf("error = %+v", got.Error)
	}
	if _, err := os.Stat(filepath.Join(got.Verified.Dir, "SKILL.md")); err != nil {
		t.Fatalf("last good checkout lost: %v", err)
	}
	// A later success moves to the new revision, clears the error and
	// removes the superseded checkout.
	f.err, f.rev = nil, revB
	got, err = r.Refresh(context.Background(), s.ID)
	if err != nil || got.Error != nil || got.Verified.Revision != revB {
		t.Fatalf("after success: %+v, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(dir, s.ID, revA)); !os.IsNotExist(err) {
		t.Fatalf("old checkout still there: %v", err)
	}
}

func TestChangingThePinFetchesFirstAndKeepsTheOldOneOnFailure(t *testing.T) {
	f := &fakeFetcher{rev: revA}
	r, _ := open(t, f)
	s, _ := r.Add(context.Background(), Input{Name: "ext", Kind: Git, Location: "https://example.test/a.git", Pin: "v1"})
	pin := "v2"
	f.err = errors.New("no such ref")
	if _, err := r.Update(context.Background(), s.ID, Patch{Pin: &pin}); err == nil {
		t.Fatal("expected an error")
	}
	if got, _ := r.Get(s.ID); got.Pin != "v1" || got.Verified.Revision != revA {
		t.Fatalf("after failure = %+v", got)
	}
	f.err, f.rev = nil, revB
	got, err := r.Update(context.Background(), s.ID, Patch{Pin: &pin})
	if err != nil || got.Pin != "v2" || got.Verified.Revision != revB {
		t.Fatalf("after success = %+v, %v", got, err)
	}
	bad := "--evil"
	if _, err := r.Update(context.Background(), s.ID, Patch{Pin: &bad}); err == nil {
		t.Fatal("hostile pin accepted")
	}
}

func TestFolderHasNoPinAndCannotBeRefreshed(t *testing.T) {
	r, _ := open(t, &fakeFetcher{})
	s := addFolder(t, r, "mine")
	pin := "main"
	if _, err := r.Update(context.Background(), s.ID, Patch{Pin: &pin}); err == nil {
		t.Fatal("pinned a folder")
	}
	if _, err := r.Refresh(context.Background(), s.ID); err == nil {
		t.Fatal("refreshed a folder")
	}
}

func TestReorderAndEnableChangePrecedence(t *testing.T) {
	r, _ := open(t, &fakeFetcher{})
	a, b, c := addFolder(t, r, "a"), addFolder(t, r, "b"), addFolder(t, r, "c")
	if err := r.Reorder([]string{c.ID, a.ID, b.ID}); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, s := range r.List() {
		names = append(names, s.Name)
	}
	if strings.Join(names, "") != "cab" {
		t.Fatalf("order = %v", names)
	}
	for name, ids := range map[string][]string{
		"missing":   {a.ID, b.ID},
		"duplicate": {a.ID, a.ID, b.ID},
		"unknown":   {a.ID, b.ID, "ffffffffffffffff"},
	} {
		if err := r.Reorder(ids); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	off := false
	if got, err := r.Update(context.Background(), a.ID, Patch{Enabled: &off}); err != nil || got.Enabled {
		t.Fatalf("disable: %+v, %v", got, err)
	}
}

func TestCampaignMappingInheritsThenOverrides(t *testing.T) {
	r, _ := open(t, &fakeFetcher{})
	a, b := addFolder(t, r, "a"), addFolder(t, r, "b")
	inherit := r.ForCampaign("c1")
	if len(inherit) != 2 || inherit[0].ID != a.ID || !inherit[0].Enabled || inherit[0].Override {
		t.Fatalf("inherited = %+v", inherit)
	}
	// c1 puts b first and turns a off; c2 is untouched.
	if err := r.SetCampaign("c1", []Mapping{{b.ID, true}, {a.ID, false}}); err != nil {
		t.Fatal(err)
	}
	got := r.ForCampaign("c1")
	if got[0].ID != b.ID || !got[0].Enabled || !got[0].Override || got[1].ID != a.ID || got[1].Enabled {
		t.Fatalf("c1 = %+v", got)
	}
	if other := r.ForCampaign("c2"); other[0].ID != a.ID || !other[0].Enabled {
		t.Fatalf("c2 changed: %+v", other)
	}
	// A source added later shows up for c1, after its own entries.
	c := addFolder(t, r, "c")
	if got := r.ForCampaign("c1"); len(got) != 3 || got[2].ID != c.ID || got[2].Override {
		t.Fatalf("with new source = %+v", got)
	}
	// A globally disabled source stays off for a campaign that lists it on.
	off := false
	_, _ = r.Update(context.Background(), b.ID, Patch{Enabled: &off})
	if got := r.ForCampaign("c1"); got[0].Enabled {
		t.Fatalf("global off ignored: %+v", got[0])
	}
	// Clearing returns to the defaults.
	if err := r.SetCampaign("c1", nil); err != nil {
		t.Fatal(err)
	}
	if got := r.ForCampaign("c1"); got[0].Override {
		t.Fatalf("not cleared: %+v", got)
	}
}

func TestSetCampaignRefusesUnknownAndRepeatedSources(t *testing.T) {
	r, _ := open(t, &fakeFetcher{})
	a := addFolder(t, r, "a")
	if err := r.SetCampaign("c1", []Mapping{{"ffffffffffffffff", true}}); err == nil {
		t.Error("unknown source accepted")
	}
	if err := r.SetCampaign("c1", []Mapping{{a.ID, true}, {a.ID, false}}); err == nil {
		t.Error("repeated source accepted")
	}
	if err := r.SetCampaign("", []Mapping{{a.ID, true}}); err == nil {
		t.Error("empty campaign accepted")
	}
}

func TestRemoveForgetsMappingsButNeverDeletesAUserFolder(t *testing.T) {
	f := &fakeFetcher{rev: revA}
	r, _ := open(t, f)
	folder := t.TempDir()
	marker := filepath.Join(folder, "SKILL.md")
	_ = os.WriteFile(marker, []byte("mine"), 0o600)
	a, err := r.Add(context.Background(), Input{Name: "a", Kind: Folder, Location: folder})
	if err != nil {
		t.Fatal(err)
	}
	g, _ := r.Add(context.Background(), Input{Name: "g", Kind: Git, Location: "https://example.test/a.git", Pin: "main"})
	_ = r.SetCampaign("c1", []Mapping{{a.ID, true}, {g.ID, true}})
	if err := r.Remove(a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("the user's folder was touched: %v", err)
	}
	if got := r.ForCampaign("c1"); len(got) != 1 || got[0].ID != g.ID {
		t.Fatalf("mapping after remove = %+v", got)
	}
	co := g.Verified.Dir
	if err := r.Remove(g.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(co); !os.IsNotExist(err) {
		t.Fatalf("checkout kept: %v", err)
	}
	if err := r.Remove(g.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestMalformedStateFileIsAnErrorNotAnEmptyRegistry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.json")
	_ = os.WriteFile(path, []byte("{nope"), 0o600)
	if _, err := Open(path, Options{}); err == nil {
		t.Fatal("expected an error")
	}
}

func TestFailedWriteRollsBackMemory(t *testing.T) {
	dir := t.TempDir()
	// The state "file" is a directory, so every write fails.
	path := filepath.Join(dir, "state")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	r := &Registry{path: path, fetcher: &fakeFetcher{}, now: time.Now}
	if _, err := r.Add(context.Background(), Input{Name: "a", Kind: Folder, Location: t.TempDir()}); err == nil {
		t.Fatal("expected a write error")
	}
	if len(r.List()) != 0 {
		t.Fatal("memory kept a source that was never saved")
	}
}

func TestSuggestionsNameTheLicenceAndRegisterNothing(t *testing.T) {
	r, _ := open(t, &fakeFetcher{})
	s := Suggestions()
	if len(s) == 0 || s[0].License != "MIT" || s[0].Notice == "" || !strings.HasPrefix(s[0].Location, "https://") {
		t.Fatalf("suggestions = %+v", s)
	}
	if len(r.List()) != 0 {
		t.Fatal("suggesting registered a source")
	}
}

// The real fetcher against a local repository: no network, no real remote.
func TestGitFetcherChecksOutThePinWithoutRunningRepoCode(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	repo := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo, "-c", "user.name=t", "-c", "user.email=t@example.test",
			"-c", "commit.gpgsign=false"}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "--quiet", "-b", "main")
	_ = os.MkdirAll(filepath.Join(repo, "plan"), 0o755)
	_ = os.WriteFile(filepath.Join(repo, "plan", "SKILL.md"), []byte("v1"), 0o644)
	// A hook that would leave a marker if the fetcher ever ran repo code.
	marker := filepath.Join(t.TempDir(), "ran")
	_ = os.WriteFile(filepath.Join(repo, ".git", "hooks", "post-checkout"), []byte("#!/bin/sh\ntouch "+marker+"\n"), 0o755)
	git("add", ".")
	git("commit", "--quiet", "-m", "one")
	first := git("rev-parse", "HEAD")
	git("tag", "v1")
	_ = os.WriteFile(filepath.Join(repo, "plan", "SKILL.md"), []byte("v2"), 0o644)
	git("commit", "--quiet", "-am", "two")

	dir := t.TempDir()
	r, _ := Open("", Options{Root: dir})
	s, err := r.Add(context.Background(), Input{Name: "local", Kind: Git, Location: repo, Pin: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	if s.Verified.Revision != first {
		t.Fatalf("revision = %s, want %s", s.Verified.Revision, first)
	}
	b, err := os.ReadFile(filepath.Join(s.Verified.Dir, "plan", "SKILL.md"))
	if err != nil || string(b) != "v1" {
		t.Fatalf("checkout = %q, %v", b, err)
	}
	if _, err := os.Stat(filepath.Join(s.Verified.Dir, ".git")); !os.IsNotExist(err) {
		t.Fatalf(".git left in the checkout: %v", err)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("a repository hook ran")
	}
	// A blank pin follows the default branch and records its exact commit.
	head := git("rev-parse", "HEAD")
	follow, err := r.Add(context.Background(), Input{Name: "follow", Kind: Git, Location: repo})
	if err != nil || follow.Pin != "" || follow.Verified.Revision != head {
		t.Fatalf("blank pin = %+v, %v (head %s)", follow, err, head)
	}
	if _, err := r.Add(context.Background(), Input{Name: "bad", Kind: Git, Location: repo, Pin: "no-such-ref"}); err == nil {
		t.Fatal("unknown ref accepted")
	}
}

func TestEditingALocationRefetchesAndKeepsTheOldOneOnFailure(t *testing.T) {
	f := &fakeFetcher{rev: revA}
	r, _ := open(t, f)
	g, _ := r.Add(context.Background(), Input{Name: "g", Kind: Git, Location: "https://example.test/a.git"})
	other := "https://example.test/b.git"
	f.err = errors.New("not found")
	if _, err := r.Update(context.Background(), g.ID, Patch{Location: &other}); err == nil {
		t.Fatal("expected an error")
	}
	if got, _ := r.Get(g.ID); got.Location != "https://example.test/a.git" {
		t.Fatalf("location changed despite failure: %+v", got)
	}
	f.err, f.rev = nil, revB
	got, err := r.Update(context.Background(), g.ID, Patch{Location: &other})
	if err != nil || got.Location != other || got.Verified.Revision != revB {
		t.Fatalf("after success = %+v, %v", got, err)
	}
	bad := "--x"
	if _, err := r.Update(context.Background(), g.ID, Patch{Location: &bad}); err == nil {
		t.Fatal("hostile remote accepted")
	}
	folder := addFolder(t, r, "f")
	nf := filepath.Join(t.TempDir(), "gone")
	if _, err := r.Update(context.Background(), folder.ID, Patch{Location: &nf}); err == nil {
		t.Fatal("missing folder accepted")
	}
}

func TestEmptyListsAreEmptyNotNil(t *testing.T) {
	r, _ := open(t, &fakeFetcher{})
	if got := r.ForCampaign("c1"); got == nil || len(got) != 0 {
		t.Fatalf("ForCampaign = %#v, want an empty non-nil list", got)
	}
	if got := r.List(); got == nil {
		t.Fatal("List is nil")
	}
}
