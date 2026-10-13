package campaign

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dkage/dungeon-master/internal/testutil"
)

func TestParseGitHubRemote(t *testing.T) {
	ok := map[string][2]string{
		"https://github.com/acme/widgets.git":      {"acme", "widgets"},
		"https://github.com/acme/widgets":          {"acme", "widgets"},
		"https://github.com/acme/widgets/":         {"acme", "widgets"},
		"http://github.com/acme/widgets.git":       {"acme", "widgets"},
		"https://user:tok@github.com/acme/w.git":   {"acme", "w"},
		"git@github.com:acme/widgets.git":          {"acme", "widgets"},
		"git@github.com:acme/widgets":              {"acme", "widgets"},
		"github.com:acme/widgets.git":              {"acme", "widgets"},
		"ssh://git@github.com/acme/widgets.git":    {"acme", "widgets"},
		"ssh://git@github.com:22/acme/widgets.git": {"acme", "widgets"},
		"https://www.github.com/a-b/c_d.e":         {"a-b", "c_d.e"},
		"  git@github.com:acme/widgets.git\n":      {"acme", "widgets"},
	}
	for in, want := range ok {
		o, r, err := ParseGitHubRemote(in)
		if err != nil || o != want[0] || r != want[1] {
			t.Errorf("%q: got %q/%q err=%v, want %v", in, o, r, err, want)
		}
	}
	bad := []string{
		"", "   ", "not a url", "/local/path/repo.git", "../relative",
		"https://gitlab.com/acme/widgets.git",
		"git@gitlab.com:acme/widgets.git",
		"https://github.com/acme",                 // no repo
		"https://github.com/acme/widgets/tree/x",  // extra segments
		"https://github.com/acme/wid gets",        // space
		"https://github.com/../widgets",           // traversal
		"https://github.com/acme/..",              // traversal
		"https://github.com.evil.test/acme/w.git", // lookalike host
		"https://evil.test/github.com/acme/w.git", // host in path
		"file:///srv/git/acme/widgets.git",
		"https://github.com//widgets",
	}
	for _, in := range bad {
		if o, r, err := ParseGitHubRemote(in); err == nil {
			t.Errorf("%q accepted as %q/%q", in, o, r)
		}
	}
}

type fakeGit struct {
	roots   map[string]string
	remotes map[string]string // root|remote → url
	calls   int
}

func (f *fakeGit) Toplevel(_ context.Context, dir string) (string, error) {
	f.calls++
	if r, ok := f.roots[dir]; ok {
		return r, nil
	}
	return "", errors.New("not a git repository")
}

func (f *fakeGit) RemoteURL(_ context.Context, root, remote string) (string, error) {
	if u, ok := f.remotes[root+"|"+remote]; ok {
		return u, nil
	}
	return "", errors.New("no such remote")
}

func newReg(t *testing.T, path string, g Git) *Registry {
	t.Helper()
	r, err := Open(context.Background(), path, g)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestAddResolvesGitAndDefaults(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "apps", "api")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	g := &fakeGit{
		roots:   map[string]string{sub: root},
		remotes: map[string]string{root + "|origin": "git@github.com:acme/mono.git"},
	}
	r := newReg(t, "", g)
	c, err := r.Add(context.Background(), Input{Dir: sub})
	if err != nil {
		t.Fatal(err)
	}
	if c.Name != "api" || c.Remote != "origin" || c.GitRoot != root || c.Repo != "acme/mono" || c.Scope != nil {
		t.Fatalf("campaign: %+v", c)
	}
	if !strings.HasPrefix(c.ID, "c") || len(c.ID) != 9 {
		t.Fatalf("id %q", c.ID)
	}
	if got, ok := r.Get(c.ID); !ok || got.ID != c.ID {
		t.Fatal("Get failed")
	}
}

func TestAddWithoutGitOrGitHubStillWorks(t *testing.T) {
	plain := t.TempDir()
	other := t.TempDir()
	g := &fakeGit{
		roots:   map[string]string{other: other},
		remotes: map[string]string{other + "|origin": "https://gitlab.com/acme/x.git"},
	}
	r := newReg(t, "", g)
	c1, err := r.Add(context.Background(), Input{Dir: plain, Name: "Scratch"})
	if err != nil || c1.GitRoot != "" || c1.Repo != "" || c1.Name != "Scratch" {
		t.Fatalf("plain: %+v %v", c1, err)
	}
	c2, err := r.Add(context.Background(), Input{Dir: other})
	if err != nil || c2.GitRoot != other || c2.Repo != "" {
		t.Fatalf("non-github: %+v %v", c2, err)
	}
}

func TestAddValidation(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f")
	_ = os.WriteFile(file, nil, 0o600)
	r := newReg(t, "", &fakeGit{})
	cases := map[string]Input{
		"empty dir":     {Dir: ""},
		"relative dir":  {Dir: "rel/dir"},
		"missing dir":   {Dir: filepath.Join(dir, "nope")},
		"file as dir":   {Dir: file},
		"long name":     {Dir: dir, Name: strings.Repeat("x", 61)},
		"control name":  {Dir: dir, Name: "a\x07b"},
		"flag remote":   {Dir: dir, Remote: "--upload-pack=x"},
		"spaced remote": {Dir: dir, Remote: "my remote"},
	}
	for name, in := range cases {
		_, err := r.Add(context.Background(), in)
		var ve ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("%s: err=%v, want ValidationError", name, err)
		}
	}
	if len(r.List()) != 0 {
		t.Fatal("invalid input registered something")
	}
}

func TestNewCampaignsHaveNoScopeYet(t *testing.T) {
	r := newReg(t, "", &fakeGit{})
	c, _ := r.Add(context.Background(), Input{Dir: t.TempDir()})
	if c.Scope != nil {
		t.Fatalf("a campaign must ask the user before showing anything: %+v", c.Scope)
	}
}

func TestValidateScope(t *testing.T) {
	good := map[string]struct {
		in   Scope
		want Scope
	}{
		"labels trimmed and de-duplicated": {Scope{Mode: ScopeLabels, Labels: []string{" tracked ", "", "Tracked", "bug"}}, Scope{Mode: ScopeLabels, Labels: []string{"tracked", "bug"}}},
		"assigned":                         {Scope{Mode: ScopeAssigned}, Scope{Mode: ScopeAssigned, Labels: []string{}}},
		"all":                              {Scope{Mode: ScopeAll, Labels: []string{}}, Scope{Mode: ScopeAll, Labels: []string{}}},
		"five labels":                      {Scope{Mode: ScopeLabels, Labels: []string{"a", "b", "c", "d", "e"}}, Scope{Mode: ScopeLabels, Labels: []string{"a", "b", "c", "d", "e"}}},
	}
	for name, tc := range good {
		got, err := ValidateScope(tc.in)
		if err != nil || got.Mode != tc.want.Mode || strings.Join(got.Labels, "|") != strings.Join(tc.want.Labels, "|") || got.Labels == nil {
			t.Errorf("%s: %+v %v", name, got, err)
		}
	}
	bad := map[string]Scope{
		"unknown mode":         {Mode: "everything"},
		"empty mode":           {},
		"labels mode, none":    {Mode: ScopeLabels},
		"labels mode, blanks":  {Mode: ScopeLabels, Labels: []string{" ", ""}},
		"too many labels":      {Mode: ScopeLabels, Labels: []string{"a", "b", "c", "d", "e", "f"}},
		"long label":           {Mode: ScopeLabels, Labels: []string{strings.Repeat("l", 51)}},
		"comma in label":       {Mode: ScopeLabels, Labels: []string{"a,b"}},
		"control char":         {Mode: ScopeLabels, Labels: []string{"a\nb"}},
		"labels with assigned": {Mode: ScopeAssigned, Labels: []string{"x"}},
		"labels with all":      {Mode: ScopeAll, Labels: []string{"x"}},
	}
	for name, sc := range bad {
		var ve ValidationError
		if _, err := ValidateScope(sc); !errors.As(err, &ve) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestScopeIsSetPersistedAndCopied(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.json")
	r := newReg(t, path, &fakeGit{})
	c, _ := r.Add(context.Background(), Input{Dir: t.TempDir()})
	sc := Scope{Mode: ScopeLabels, Labels: []string{"tracked"}}
	u, err := r.Update(context.Background(), c.ID, Patch{Scope: &sc})
	if err != nil || u.Scope == nil || u.Scope.Mode != ScopeLabels || u.Scope.Labels[0] != "tracked" {
		t.Fatalf("%+v %v", u, err)
	}
	if got, _ := newReg(t, path, &fakeGit{}).Get(c.ID); got.Scope == nil || got.Scope.Labels[0] != "tracked" {
		t.Fatalf("scope not persisted: %+v", got)
	}
	// What callers get is a copy: changing it must not change the registry.
	u.Scope.Labels[0] = "mutated"
	if again, _ := r.Get(c.ID); again.Scope.Labels[0] != "tracked" {
		t.Fatal("the registry's scope was mutated through a returned value")
	}
	// A bad scope is refused and changes nothing.
	bad := Scope{Mode: "nope"}
	if _, err := r.Update(context.Background(), c.ID, Patch{Scope: &bad}); err == nil {
		t.Fatal("bad scope accepted")
	}
	if again, _ := r.Get(c.ID); again.Scope.Mode != ScopeLabels {
		t.Fatalf("rejected update leaked: %+v", again.Scope)
	}
	// Other updates leave it alone.
	name := "x"
	if u, _ := r.Update(context.Background(), c.ID, Patch{Name: &name}); u.Scope == nil {
		t.Fatal("unrelated update cleared the scope")
	}
}

func TestOldLabelFiltersBecomeAScope(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(t.TempDir(), "campaigns.json")
	old := `{"version":1,"campaigns":[
	  {"id":"c1","name":"api","dir":"` + filepath.ToSlash(dir) + `","remote":"origin","labels":["area:api","Area:API","bug"]},
	  {"id":"c2","name":"plain","dir":"` + filepath.ToSlash(t.TempDir()) + `","remote":"origin","labels":[]},
	  {"id":"c3","name":"bad","dir":"` + filepath.ToSlash(t.TempDir()) + `","remote":"origin","labels":["a,b"]}
	]}`
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	r := newReg(t, path, &fakeGit{})
	c1, _ := r.Get("c1")
	if c1.Scope == nil || c1.Scope.Mode != ScopeLabels || strings.Join(c1.Scope.Labels, "|") != "area:api|bug" {
		t.Fatalf("c1: %+v", c1.Scope)
	}
	if c2, _ := r.Get("c2"); c2.Scope != nil {
		t.Fatalf("an empty filter must stay unconfigured: %+v", c2.Scope)
	}
	if c3, _ := r.Get("c3"); c3.Scope != nil {
		t.Fatalf("an invalid old filter must not become a scope: %+v", c3.Scope)
	}
	// The next save writes the scope and drops the old field for good.
	name := "api2"
	r.Update(context.Background(), "c1", Patch{Name: &name})
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), `"labels":["area:api"`) || !strings.Contains(string(raw), `"scope"`) {
		t.Fatalf("file after migration: %s", raw)
	}
}

func TestDuplicateDirectoryIncludingViaSymlink(t *testing.T) {
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skip("symlinks unavailable")
	}
	r := newReg(t, "", &fakeGit{})
	if _, err := r.Add(context.Background(), Input{Dir: real}); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{real, real + "/", link} {
		if _, err := r.Add(context.Background(), Input{Dir: d}); !errors.Is(err, ErrDuplicate) {
			t.Errorf("%q: err=%v, want ErrDuplicate", d, err)
		}
	}
	if len(r.List()) != 1 {
		t.Fatalf("list: %+v", r.List())
	}
}

func TestPersistenceRoundTripAndPermissions(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state", "campaigns.json")
	dir := t.TempDir()
	g := &fakeGit{
		roots:   map[string]string{dir: dir},
		remotes: map[string]string{dir + "|upstream": "https://github.com/acme/fork-parent.git"},
	}
	r := newReg(t, state, g)
	c, err := r.Add(context.Background(), Input{Dir: dir, Name: "Main", Remote: "upstream"})
	if err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(state)
	if err != nil || !testutil.PrivateMode(state, fi, 0o600) {
		t.Fatalf("state file: %v %v", fi, err)
	}
	if di, _ := os.Stat(filepath.Dir(state)); !testutil.PrivateMode(filepath.Dir(state), di, 0o700) {
		t.Fatalf("state dir mode %v", di.Mode().Perm())
	}
	raw, _ := os.ReadFile(state)
	if strings.Contains(string(raw), "gitRoot") || strings.Contains(string(raw), "acme/fork-parent") {
		t.Fatalf("derived data was persisted: %s", raw)
	}

	again := newReg(t, state, g) // reload: derived fields are re-resolved
	got, ok := again.Get(c.ID)
	if !ok || got.Name != "Main" || got.Remote != "upstream" || got.Repo != "acme/fork-parent" {
		t.Fatalf("reloaded: %+v ok=%v", got, ok)
	}
}

func TestOpenRejectsCorruptFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "campaigns.json")
	_ = os.WriteFile(p, []byte("{not json"), 0o600)
	if _, err := Open(context.Background(), p, &fakeGit{}); err == nil || !strings.Contains(err.Error(), "campaigns.json") {
		t.Fatalf("err=%v", err)
	}
}

func TestUpdate(t *testing.T) {
	dir := t.TempDir()
	g := &fakeGit{
		roots: map[string]string{dir: dir},
		remotes: map[string]string{
			dir + "|origin":   "git@github.com:me/fork.git",
			dir + "|upstream": "git@github.com:acme/real.git",
		},
	}
	r := newReg(t, filepath.Join(t.TempDir(), "c.json"), g)
	c, _ := r.Add(context.Background(), Input{Dir: dir})
	if c.Repo != "me/fork" {
		t.Fatalf("repo %q", c.Repo)
	}
	name, remote := "Renamed", "upstream"
	u, err := r.Update(context.Background(), c.ID, Patch{Name: &name, Remote: &remote})
	if err != nil || u.Name != "Renamed" || u.Repo != "acme/real" {
		t.Fatalf("update: %+v %v", u, err)
	}
	// Only the touched field changes.
	n2 := "Again"
	u, _ = r.Update(context.Background(), c.ID, Patch{Name: &n2})
	if u.Remote != "upstream" || u.Repo != "acme/real" {
		t.Fatalf("untouched fields changed: %+v", u)
	}
	// Empty name falls back to the directory name; bad input is rejected and changes nothing.
	empty := ""
	u, _ = r.Update(context.Background(), c.ID, Patch{Name: &empty})
	if u.Name != filepath.Base(dir) {
		t.Fatalf("name %q", u.Name)
	}
	bad := "--evil"
	if _, err := r.Update(context.Background(), c.ID, Patch{Remote: &bad}); err == nil {
		t.Fatal("bad remote accepted")
	}
	if got, _ := r.Get(c.ID); got.Remote != "upstream" {
		t.Fatalf("rejected update leaked: %+v", got)
	}
	if _, err := r.Update(context.Background(), "nope", Patch{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestRemove(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.json")
	r := newReg(t, path, &fakeGit{})
	dir := t.TempDir()
	c, _ := r.Add(context.Background(), Input{Dir: dir})
	if ok, err := r.Remove(c.ID); !ok || err != nil {
		t.Fatalf("remove: %v %v", ok, err)
	}
	if ok, _ := r.Remove(c.ID); ok {
		t.Fatal("removed twice")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatal("remove touched the directory")
	}
	if got := newReg(t, path, &fakeGit{}).List(); len(got) != 0 {
		t.Fatalf("persisted after remove: %+v", got)
	}
	// Re-adding after removal is allowed.
	if _, err := r.Add(context.Background(), Input{Dir: dir}); err != nil {
		t.Fatal(err)
	}
}

// --- feature test with the real git binary ---

func gitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestRealGitMonorepoSubfolderAndRewrites(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root, _ := filepath.EvalSymlinks(t.TempDir())
	gitCmd(t, root, "init", "-q")
	gitCmd(t, root, "remote", "add", "origin", "https://github.com/acme/mono.git")
	gitCmd(t, root, "remote", "add", "mirror", "gh:acme/mirror")
	gitCmd(t, root, "config", "url.https://github.com/.insteadOf", "gh:")
	sub := filepath.Join(root, "apps", "api")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	r := newReg(t, "", nil) // real git
	ctx := context.Background()

	c, err := r.Add(ctx, Input{Dir: sub})
	if err != nil {
		t.Fatal(err)
	}
	// The subfolder is the working directory; issues come from the root's remote.
	if c.Dir != sub || c.GitRoot != root || c.Repo != "acme/mono" || c.Name != "api" {
		t.Fatalf("subfolder campaign: %+v", c)
	}

	// A second campaign at the root shares the repository.
	c2, err := r.Add(ctx, Input{Dir: root, Remote: "mirror"})
	if err != nil || c2.Repo != "acme/mirror" { // insteadOf rewrite applied
		t.Fatalf("root campaign: %+v %v", c2, err)
	}

	// Missing remote: still registers, no repo.
	c3, err := r.Add(ctx, Input{Dir: t.TempDir()})
	if err != nil || c3.GitRoot != "" || c3.Repo != "" {
		t.Fatalf("plain dir: %+v %v", c3, err)
	}
	missing := "nonexistent"
	u, err := r.Update(ctx, c.ID, Patch{Remote: &missing})
	if err != nil || u.GitRoot != root || u.Repo != "" {
		t.Fatalf("missing remote: %+v %v", u, err)
	}
}

func TestDemoFlagPersistsAndUpdates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.json")
	r := newReg(t, path, &fakeGit{})
	c, _ := r.Add(context.Background(), Input{Dir: t.TempDir()})
	if c.Demo {
		t.Fatal("campaigns start with real quests")
	}
	on := true
	u, err := r.Update(context.Background(), c.ID, Patch{Demo: &on})
	if err != nil || !u.Demo {
		t.Fatalf("%+v %v", u, err)
	}
	if got, _ := newReg(t, path, &fakeGit{}).Get(c.ID); !got.Demo {
		t.Fatal("demo flag not persisted")
	}
	// Other updates leave it alone; it can be switched back off.
	name := "x"
	if u, _ := r.Update(context.Background(), c.ID, Patch{Name: &name}); !u.Demo {
		t.Fatal("unrelated update cleared the demo flag")
	}
	off := false
	if u, _ := r.Update(context.Background(), c.ID, Patch{Demo: &off}); u.Demo {
		t.Fatal("demo flag not cleared")
	}
}

func TestExecGitDoesNotHangOnStrayChildren(t *testing.T) {
	// A "git" that leaves a background process holding stdout open must not
	// make us wait for that process after the timeout kills git.
	dir := t.TempDir()
	script := "#!/bin/sh\nsleep 30 &\nsleep 30\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := (ExecGit{}).Toplevel(ctx, t.TempDir()); err == nil {
		t.Fatal("expected an error")
	}
	if time.Since(start) > 8*time.Second {
		t.Fatalf("hung for %v", time.Since(start))
	}
}

func TestParallelQuestsPersistsAndUpdates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.json")
	r := newReg(t, path, &fakeGit{})
	c, _ := r.Add(context.Background(), Input{Dir: t.TempDir()})
	if c.ParallelQuests {
		t.Fatal("one quest per working tree is the default")
	}
	on := true
	u, err := r.Update(context.Background(), c.ID, Patch{ParallelQuests: &on})
	if err != nil || !u.ParallelQuests {
		t.Fatalf("%+v %v", u, err)
	}
	if got, _ := newReg(t, path, &fakeGit{}).Get(c.ID); !got.ParallelQuests {
		t.Fatal("setting not persisted")
	}
	name := "x"
	if u, _ := r.Update(context.Background(), c.ID, Patch{Name: &name}); !u.ParallelQuests {
		t.Fatal("unrelated update cleared the setting")
	}
	off := false
	if u, _ := r.Update(context.Background(), c.ID, Patch{ParallelQuests: &off}); u.ParallelQuests {
		t.Fatal("could not switch it back off")
	}
}
