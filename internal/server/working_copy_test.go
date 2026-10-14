package server

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dkage/dungeon-master/internal/quests"
)

// gitRepo makes a real repository (the user's own Git configuration is off)
// and registers it as a second campaign of the test server.
func (e *env) gitRepo(t *testing.T) (dir, campaignID string) {
	t.Helper()
	empty := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", empty)
	t.Setenv("GIT_CONFIG_SYSTEM", empty)
	dir = filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "main")
	git("config", "user.email", "t@example.test")
	git("config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".env\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "a.txt", ".gitignore")
	git("commit", "-q", "-m", "init")
	// The repository's real top level (macOS reports /private/var for /var).
	e.gh.mu.Lock()
	e.gh.detail = quests.Detail{Quest: quests.Quest{Title: "An open issue", State: quests.StateOpen}}
	e.gh.mu.Unlock()
	real, _ := filepath.EvalSymlinks(dir)
	resp, out := e.req("POST", "/api/campaigns", map[string]any{"dir": real, "name": "repo"}, nil)
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		t.Fatalf("add campaign: %d %s", resp.StatusCode, out)
	}
	var c struct{ ID string }
	_ = json.Unmarshal(out, &c)
	return real, c.ID
}

func (e *env) launch(campaign string, quest int, extra map[string]any) (*http.Response, []byte) {
	body := map[string]any{"campaign": campaign, "hero": "opus", "quest": quest}
	for k, v := range extra {
		body[k] = v
	}
	return e.req("POST", "/api/sessions", body, nil)
}

func TestTwoIssueSessionsRunInSeparateWorkingCopies(t *testing.T) {
	e := startServer(t, nil)
	repo, camp := e.gitRepo(t)

	var views []sessionJSON
	for _, n := range []int{1, 2} {
		resp, out := e.launch(camp, n, map[string]any{"workingCopy": "copy"})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("issue %d in a copy: %d %s", n, resp.StatusCode, out)
		}
		var sj sessionJSON
		_ = json.Unmarshal(out, &sj)
		views = append(views, sj)
	}
	a, b := views[0], views[1]
	if a.Copy == "" || b.Copy == "" || a.Copy == b.Copy || a.Branch != "dungeon/issue-1" || b.Branch != "dungeon/issue-2" {
		t.Fatalf("copies: %+v %+v", a, b)
	}
	for _, v := range views {
		if strings.HasPrefix(v.Copy, repo) {
			t.Errorf("the copy %s is inside the repository", v.Copy)
		}
		if v.Dir != v.Copy {
			t.Errorf("the session starts in %s, want its copy %s", v.Dir, v.Copy)
		}
	}
	// Each agent really runs in its own copy, and a write there stays there.
	for _, v := range views {
		c, _, err := e.dial(v.ID, nil)
		if err != nil {
			t.Fatal(err)
		}
		out := readUntil(t, c, "cwd=")
		c.CloseNow()
		// Windows prints the long form of a short temp path, so only compare elsewhere.
		if runtime.GOOS != "windows" && !strings.Contains(out, "cwd="+realPathOf(t, v.Copy)) && !strings.Contains(out, v.Copy) {
			t.Errorf("session %s did not start in %s:\n%s", v.ID, v.Copy, out)
		}
		if err := os.WriteFile(filepath.Join(v.Copy, "from-"+v.ID+".txt"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, other := range []struct{ dir, name string }{{a.Copy, "from-" + b.ID + ".txt"}, {b.Copy, "from-" + a.ID + ".txt"}, {repo, "from-" + a.ID + ".txt"}} {
		if _, err := os.Stat(filepath.Join(other.dir, other.name)); err == nil {
			t.Errorf("%s appeared in %s", other.name, other.dir)
		}
	}

	// The same issue again is refused, in a copy or in the main tree.
	for _, mode := range []string{"copy", "main"} {
		if resp, out := e.launch(camp, 1, map[string]any{"workingCopy": mode}); resp.StatusCode != http.StatusConflict || !strings.Contains(string(out), "already has a session") {
			t.Errorf("issue 1 again (%s): %d %s", mode, resp.StatusCode, out)
		}
	}
	// The main tree is still free for one more issue, but not for two.
	if resp, out := e.launch(camp, 3, nil); resp.StatusCode != http.StatusCreated {
		t.Fatalf("issue 3 in the main tree: %d %s", resp.StatusCode, out)
	}
	if resp, out := e.launch(camp, 4, nil); resp.StatusCode != http.StatusConflict {
		t.Fatalf("a second issue in the main tree: %d %s", resp.StatusCode, out)
	}
	if resp, out := e.launch(camp, 4, map[string]any{"workingCopy": "copy"}); resp.StatusCode != http.StatusCreated {
		t.Fatalf("a fourth issue in its own copy: %d %s", resp.StatusCode, out)
	}
	// Git itself left no trace in the main tree's working files.
	if out, _ := exec.Command("git", "-C", repo, "status", "--short").Output(); strings.TrimSpace(string(out)) != "" {
		t.Errorf("the main tree changed: %q", out)
	}
}

func realPathOf(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		return p
	}
	return r
}

func TestClosingASessionRemovesACleanCopyAndKeepsADirtyOne(t *testing.T) {
	e := startServer(t, nil)
	_, camp := e.gitRepo(t)
	mk := func(n int) sessionJSON {
		resp, out := e.launch(camp, n, map[string]any{"workingCopy": "copy"})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("launch %d: %d %s", n, resp.StatusCode, out)
		}
		var sj sessionJSON
		_ = json.Unmarshal(out, &sj)
		return sj
	}
	clean, dirty := mk(1), mk(2)
	if err := os.WriteFile(filepath.Join(dirty.Copy, "wip.txt"), []byte("unfinished"), 0o644); err != nil {
		t.Fatal(err)
	}

	resp, _ := e.req("DELETE", "/api/sessions/"+clean.ID, nil, nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("close clean: %d", resp.StatusCode)
	}
	if _, err := os.Stat(clean.Copy); err == nil {
		t.Error("a clean copy was left behind")
	}

	resp, out := e.req("DELETE", "/api/sessions/"+dirty.ID, nil, nil)
	var kept struct {
		CopyKept string
		Branch   string
		Changes  int
	}
	_ = json.Unmarshal(out, &kept)
	if resp.StatusCode != http.StatusOK || kept.CopyKept != dirty.Copy || kept.Changes != 1 || kept.Branch != "dungeon/issue-2" {
		t.Fatalf("close dirty: %d %s", resp.StatusCode, out)
	}
	if b, _ := os.ReadFile(filepath.Join(dirty.Copy, "wip.txt")); string(b) != "unfinished" {
		t.Fatal("unfinished work was discarded")
	}
	// The issue can be started again; the kept copy's branch is still checked out, so it says so.
	if resp, out := e.launch(camp, 2, map[string]any{"workingCopy": "copy"}); resp.StatusCode != http.StatusConflict {
		t.Fatalf("restarting an issue whose copy was kept: %d %s", resp.StatusCode, out)
	}
}

func TestLaunchPreviewDescribesTheWorkingCopy(t *testing.T) {
	e := startServer(t, nil)
	repo, camp := e.gitRepo(t)
	if err := os.WriteFile(filepath.Join(repo, ".env"), []byte("K=1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "scratch.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	resp, out := e.req("POST", "/api/campaigns/"+camp+"/quests/9/launch-preview", map[string]any{"hero": "opus"}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("preview: %d %s", resp.StatusCode, out)
	}
	var p struct {
		CanStart    bool
		WorkingCopy workingCopyJSON
	}
	if err := json.Unmarshal(out, &p); err != nil {
		t.Fatal(err)
	}
	w := p.WorkingCopy
	if !p.CanStart || !w.Available || w.Branch != "dungeon/issue-9" || w.StartName != "main" || w.Start == "" || w.MainChanges != 1 {
		t.Fatalf("working copy plan: %+v", w)
	}
	names := map[string]bool{}
	for _, en := range w.Entries {
		names[en.Name] = en.Ignored
	}
	if ignored, ok := names[".env"]; !ok || !ignored {
		t.Errorf("entries %+v lack the ignored .env", w.Entries)
	}
	if ignored, ok := names["scratch.txt"]; !ok || ignored {
		t.Errorf("entries %+v lack the untracked scratch.txt", w.Entries)
	}
	if strings.Contains(w.Location, "<session>") || !strings.HasPrefix(w.Location, e.cfg.StateDir) {
		t.Errorf("location %q should be a folder in the state dir", w.Location)
	}
	if _, err := os.Stat(w.Location); err == nil {
		t.Error("previewing created the folder")
	}
}

func TestWorkingCopyRefusals(t *testing.T) {
	e := startServer(t, nil)
	_, camp := e.gitRepo(t)
	for name, body := range map[string]map[string]any{
		"unknown mode":          {"workingCopy": "elsewhere"},
		"carry without a copy":  {"carry": map[string]string{".env": "copy"}},
		"carry escaping":        {"workingCopy": "copy", "carry": map[string]string{"../x": "copy"}},
		"carry the git folder":  {"workingCopy": "copy", "carry": map[string]string{".git": "link"}},
		"carry an unlisted one": {"workingCopy": "copy", "carry": map[string]string{"nothing-here": "copy"}},
		"carry in a bad way":    {"workingCopy": "copy", "carry": map[string]string{".gitignore": "move"}},
	} {
		if resp, out := e.launch(camp, 5, body); resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: %d %s", name, resp.StatusCode, out)
		}
	}
	// Nothing was left behind by the refused launches.
	if entries, _ := os.ReadDir(filepath.Join(e.cfg.StateDir, "worktrees", camp)); len(entries) != 0 {
		t.Errorf("refused launches left copies: %v", entries)
	}
	// A shell never gets a copy.
	if resp, _ := e.req("POST", "/api/sessions", map[string]any{"campaign": camp, "shell": true, "workingCopy": "copy"}, nil); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("shell with a copy: %d", resp.StatusCode)
	}
}

func TestCarriedEntriesReachTheCopy(t *testing.T) {
	e := startServer(t, nil)
	repo, camp := e.gitRepo(t)
	if err := os.WriteFile(filepath.Join(repo, ".env"), []byte("K=1"), 0o644); err != nil {
		t.Fatal(err)
	}
	resp, out := e.launch(camp, 6, map[string]any{"workingCopy": "copy", "carry": map[string]string{".env": "copy"}})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("launch: %d %s", resp.StatusCode, out)
	}
	var sj sessionJSON
	_ = json.Unmarshal(out, &sj)
	if b, _ := os.ReadFile(filepath.Join(sj.Copy, ".env")); string(b) != "K=1" {
		t.Fatalf(".env in the copy: %q", b)
	}
}
