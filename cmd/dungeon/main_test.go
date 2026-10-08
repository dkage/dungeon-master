package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/dkage/dungeon-master/internal/campaign"
	"github.com/dkage/dungeon-master/internal/party"
	"github.com/dkage/dungeon-master/internal/quests"
	"github.com/dkage/dungeon-master/internal/server"
	"github.com/dkage/dungeon-master/internal/testutil"
)

// TestMain keeps every test away from the developer's real config directory.
// Individual tests override DUNGEON_HOME with their own temp directory.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "dungeon-cmd-test-*")
	if err != nil {
		panic(err)
	}
	os.Setenv("DUNGEON_HOME", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func TestRunCommands(t *testing.T) {
	if err := run(nil); err == nil {
		t.Error("no command accepted")
	}
	if err := run([]string{"bogus"}); err == nil || !strings.Contains(err.Error(), "bogus") {
		t.Errorf("unknown command: %v", err)
	}
	if err := run([]string{"version"}); err != nil {
		t.Errorf("version: %v", err)
	}
	if err := run([]string{"help"}); err != nil {
		t.Errorf("help: %v", err)
	}
	if err := run([]string{"serve", "--addr", "0.0.0.0:0"}); err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Errorf("non-loopback bind: %v", err)
	}
	if err := run([]string{"serve", "--nope"}); err == nil {
		t.Error("unknown flag accepted")
	}
}

func TestBuildConfigDefaultsWithFakeClaude(t *testing.T) {
	t.Setenv("DUNGEON_HOME", t.TempDir())
	t.Setenv("PATH", filepath.Dir(testutil.FakeClaude(t)))
	t.Setenv("SHELL", "/bin/sh")

	cfg, err := buildConfig(context.Background(), "127.0.0.1:0", "tok")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Claude == "" || cfg.ClaudeErr != "" {
		t.Fatalf("claude not resolved: %q / %q", cfg.Claude, cfg.ClaudeErr)
	}
	if !cfg.Party.Defaulted || len(cfg.Party.Heroes) == 0 || cfg.Shell != "/bin/sh" || cfg.Token != "tok" {
		t.Fatalf("config: %+v", cfg)
	}
	if cfg.Quests == nil {
		t.Fatal("the quest service is not wired")
	}
}

func TestBuildConfigReadsPartyFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("DUNGEON_HOME", home)
	fake := testutil.FakeClaude(t)
	t.Setenv("PATH", "/nonexistent")
	t.Setenv("SHELL", "")
	src := "claude = \"" + fake + "\"\n[heroes.mine]\nlabel = \"Mine\"\nargs = [\"--model\", \"x\"]\n"
	if err := os.WriteFile(filepath.Join(home, "party.toml"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := buildConfig(context.Background(), "127.0.0.1:0", "tok")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Claude != fake || cfg.Party.Defaulted || len(cfg.Party.Heroes) != 1 || cfg.Party.Heroes[0].ID != "mine" {
		t.Fatalf("config: %+v", cfg)
	}
}

func TestBuildConfigMissingClaudeIsNotFatal(t *testing.T) {
	t.Setenv("DUNGEON_HOME", t.TempDir())
	t.Setenv("PATH", "/nonexistent")
	t.Setenv("SHELL", "")
	cfg, err := buildConfig(context.Background(), "127.0.0.1:0", "tok")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Claude != "" || cfg.ClaudeErr == "" {
		t.Fatalf("expected a recorded error, got %q / %q", cfg.Claude, cfg.ClaudeErr)
	}
}

func TestBuildConfigInvalidPartyFileIsFatal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("DUNGEON_HOME", home)
	if err := os.WriteFile(filepath.Join(home, "party.toml"), []byte("[heroes.BAD]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := buildConfig(context.Background(), "127.0.0.1:0", "tok"); err == nil {
		t.Fatal("invalid party file accepted")
	}
}

func TestBuildConfigPersistsCampaignsInConfigDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("DUNGEON_HOME", home)
	t.Setenv("PATH", "/nonexistent")
	t.Setenv("SHELL", "")
	ctx := context.Background()

	cfg, err := buildConfig(ctx, "127.0.0.1:0", "tok")
	if err != nil {
		t.Fatal(err)
	}
	proj := t.TempDir()
	if _, err := cfg.Campaigns.Add(ctx, campaign.Input{Dir: proj, Name: "kept"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, "campaigns.json")); err != nil {
		t.Fatalf("campaigns.json not written to the config dir: %v", err)
	}

	// A second start sees the campaign again.
	cfg2, err := buildConfig(ctx, "127.0.0.1:0", "tok")
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg2.Campaigns.List(); len(got) != 1 || got[0].Name != "kept" || got[0].Dir != proj {
		t.Fatalf("campaigns after restart: %+v", got)
	}
}

func TestBuildConfigRejectsCorruptState(t *testing.T) {
	for _, name := range []string{"campaigns.json", "sessions.json"} {
		home := t.TempDir()
		t.Setenv("DUNGEON_HOME", home)
		t.Setenv("PATH", "/nonexistent")
		if err := os.WriteFile(filepath.Join(home, name), []byte("{nope"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := buildConfig(context.Background(), "127.0.0.1:0", "tok"); err == nil {
			t.Errorf("%s: corrupt state accepted", name)
		}
	}
}

func TestBuildConfigQuestsUseTheGitHubCLIToken(t *testing.T) {
	// End to end through the real wiring: a fake `gh` supplies the token, a
	// local server stands in for GitHub and serves a response recorded from
	// the real API.
	recorded, err := os.ReadFile(filepath.Join("..", "..", "internal", "quests", "github", "testdata", "list.json"))
	if err != nil {
		t.Fatal(err)
	}
	var auth atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth.Store(r.Header.Get("Authorization"))
		var req struct{ Variables struct{ States []string } }
		_ = json.NewDecoder(r.Body).Decode(&req)
		if len(req.Variables.States) == 1 && req.Variables.States[0] == "CLOSED" {
			w.Write([]byte(`{"data":{"repository":{"issues":{"pageInfo":{"hasNextPage":false},"nodes":[]}}}}`))
			return
		}
		w.Write(recorded)
	}))
	defer srv.Close()
	t.Setenv("DUNGEON_GITHUB_ENDPOINT", srv.URL)

	t.Setenv("DUNGEON_HOME", t.TempDir())
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("SHELL", "")
	ctx := context.Background()
	camp := campaign.Campaign{ID: "c", Repo: "dkage/dungeon-master", Scope: &campaign.Scope{Mode: campaign.ScopeAll, Labels: []string{}}}

	// Without gh or an env token: a clear "no credentials", and no request.
	t.Setenv("PATH", "/nonexistent")
	cfg, err := buildConfig(ctx, "127.0.0.1:0", "tok")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cfg.Quests.List(ctx, camp, true); err == nil || !strings.Contains(err.Error(), "credentials") {
		t.Fatalf("without gh: %v", err)
	}
	if auth.Load() != nil {
		t.Fatal("a request was made without credentials")
	}

	// With gh on PATH its token is used.
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte("#!/bin/sh\necho gh-cli-token\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	cfg, err = buildConfig(ctx, "127.0.0.1:0", "tok")
	if err != nil {
		t.Fatal(err)
	}
	res, err := cfg.Quests.List(ctx, camp, true)
	if err != nil {
		t.Fatal(err)
	}
	if got := auth.Load(); got != "Bearer gh-cli-token" {
		t.Fatalf("authorization header %v", got)
	}
	if res.Source != "github" || len(res.Quests) != 5 {
		t.Fatalf("result: %+v", res)
	}
}

func TestGitHubEndpointOverrideIsLoopbackOnly(t *testing.T) {
	t.Setenv("DUNGEON_GITHUB_ENDPOINT", "")
	if got, err := githubEndpoint(); err != nil || got != "https://api.github.com/graphql" {
		t.Fatalf("default: %q %v", got, err)
	}
	for _, ok := range []string{"http://127.0.0.1:9", "http://localhost:8080/graphql", "https://127.0.0.1/x", "http://[::1]:3000"} {
		t.Setenv("DUNGEON_GITHUB_ENDPOINT", ok)
		if got, err := githubEndpoint(); err != nil || got != ok {
			t.Errorf("%s: %q %v", ok, got, err)
		}
	}
	for _, bad := range []string{
		"https://evil.test/graphql", "http://10.0.0.5:80", "http://127.0.0.1.evil.test/", "http://localhost.evil.test",
		"ftp://127.0.0.1", "not a url", "//127.0.0.1", "file:///etc/passwd", "http://0.0.0.0:80",
	} {
		t.Setenv("DUNGEON_GITHUB_ENDPOINT", bad)
		if got, err := githubEndpoint(); err == nil {
			t.Errorf("%q accepted as %q", bad, got)
		}
	}
	// An unusable override stops startup instead of silently using GitHub.
	t.Setenv("DUNGEON_HOME", t.TempDir())
	t.Setenv("DUNGEON_GITHUB_ENDPOINT", "https://evil.test/graphql")
	if _, err := buildConfig(context.Background(), "127.0.0.1:0", "tok"); err == nil {
		t.Fatal("buildConfig accepted a remote endpoint")
	}
}

func TestServeKeepsTheTokenAcrossRuns(t *testing.T) {
	// serve prints its URL and blocks, so exercise the same path it uses.
	home := t.TempDir()
	t.Setenv("DUNGEON_HOME", home)
	dir, err := party.ConfigDir()
	if err != nil || dir != home {
		t.Fatalf("config dir %q %v", dir, err)
	}
	path := filepath.Join(dir, server.TokenFile)
	a, created, err := server.LoadOrCreateToken(path, false)
	if err != nil || !created {
		t.Fatal(a, created, err)
	}
	b, created, _ := server.LoadOrCreateToken(path, false)
	if created || a != b {
		t.Fatalf("token changed between runs: %q -> %q", a, b)
	}
	if err := run([]string{"serve", "--addr", "0.0.0.0:0", "--new-token"}); err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("the flag must parse and a non-loopback bind still fail: %v", err)
	}
}

func TestServeRefusesABadAddressBeforeTouchingState(t *testing.T) {
	home := t.TempDir()
	t.Setenv("DUNGEON_HOME", home)
	for _, addr := range []string{"0.0.0.0:0", "10.1.2.3:80", "nonsense"} {
		if err := run([]string{"serve", "--addr", addr}); err == nil {
			t.Errorf("%s accepted", addr)
		}
	}
	if entries, _ := os.ReadDir(home); len(entries) != 0 {
		t.Fatalf("a refused start left files behind: %v", entries)
	}
}

func TestQuartermasterIsWiredAndItsPauseSurvivesARestart(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Retry-After", "600")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	home := t.TempDir()
	t.Setenv("DUNGEON_HOME", home)
	t.Setenv("DUNGEON_GITHUB_ENDPOINT", srv.URL)
	t.Setenv("GH_TOKEN", "tok")
	t.Setenv("PATH", "/nonexistent")
	t.Setenv("SHELL", "")
	ctx := context.Background()
	camp := campaign.Campaign{ID: "c", Repo: "acme/widgets", Scope: &campaign.Scope{Mode: campaign.ScopeAll, Labels: []string{}}}

	cfg, err := buildConfig(ctx, "127.0.0.1:0", "tok")
	if err != nil || cfg.Quartermaster == nil {
		t.Fatalf("not wired: %v", err)
	}
	// GitHub says slow down: the first request reaches it, the next ones do not.
	if _, err := cfg.Quests.List(ctx, camp, true); err == nil {
		t.Fatal("expected the rate limit")
	}
	if !cfg.Quartermaster.Status().Camp.Active {
		t.Fatal("camp not entered")
	}
	before := hits.Load()
	_, err = cfg.Quests.List(ctx, camp, true)
	var rl *quests.RateLimitError
	if !errors.As(err, &rl) || !rl.Paused || hits.Load() != before {
		t.Fatalf("a request went out during camp: %v (hits %d -> %d)", err, before, hits.Load())
	}
	if fi, err := os.Stat(filepath.Join(home, "quartermaster.json")); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("pause not stored privately: %v %v", fi, err)
	}

	// A restart does not get around it.
	cfg2, err := buildConfig(ctx, "127.0.0.1:0", "tok")
	if err != nil {
		t.Fatal(err)
	}
	if !cfg2.Quartermaster.Status().Camp.Active {
		t.Fatal("the pause was forgotten on restart")
	}
	if _, err := cfg2.Quests.List(ctx, camp, true); !errors.As(err, &rl) || !rl.Paused || hits.Load() != before {
		t.Fatalf("restart bypassed the pause: %v", err)
	}
}

func fakeGHScript(t *testing.T, exit int) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "gh")
	script := "#!/bin/sh\nexit " + strconv.Itoa(exit) + "\n"
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestTheProgramIsTheShimOnlyWhenStartedAsGh(t *testing.T) {
	for _, name := range []string{"dungeon", "/usr/bin/dungeon", "ghost", "/x/gh-extension", ""} {
		if _, ok := maybeShim([]string{name, "issue", "create"}); ok {
			t.Errorf("%q must not be the shim", name)
		}
	}
	if _, ok := maybeShim(nil); ok {
		t.Error("no argv")
	}
	// As gh it runs the real gh and passes its exit code on (read: no app needed).
	t.Setenv("DUNGEON_REAL_GH", fakeGHScript(t, 7))
	t.Setenv("DUNGEON_GATE_URL", "")
	t.Setenv("DUNGEON_GATE_TOKEN", "")
	code, ok := maybeShim([]string{"/some/where/gh", "issue", "list"})
	if !ok || code != 7 {
		t.Fatalf("handled=%v code=%d", ok, code)
	}
	// A write with no app to ask runs unpaced (fail-open) and also passes the code on.
	code, ok = maybeShim([]string{"gh", "issue", "create", "--title", "x"})
	if !ok || code != 7 {
		t.Fatalf("fail-open write: handled=%v code=%d", ok, code)
	}
}

func TestStartupPutsTheShimInFrontOfTheRealGh(t *testing.T) {
	home := t.TempDir()
	t.Setenv("DUNGEON_HOME", home)
	real := fakeGHScript(t, 0)
	t.Setenv("PATH", filepath.Dir(real))
	t.Setenv("SHELL", "")
	cfg, err := buildConfig(context.Background(), "127.0.0.1:0", "tok")
	if err != nil {
		t.Fatal(err)
	}
	g := cfg.Gatekeeper
	if g == nil || g.RealGH != real {
		t.Fatalf("gatekeeper: %+v (want real gh %s)", g, real)
	}
	exe, _ := os.Executable()
	if target, err := os.Readlink(filepath.Join(g.BinDir, "gh")); err != nil || target != exe {
		t.Fatalf("shim link: %q %v", target, err)
	}
	if !strings.HasPrefix(g.BinDir, home) {
		t.Fatalf("the shim must live in the config directory, not %q", g.BinDir)
	}
}

func TestStartedFromInsideASessionTheAppDoesNotTakeItsOwnShimForGh(t *testing.T) {
	home := t.TempDir()
	t.Setenv("DUNGEON_HOME", home)
	real := fakeGHScript(t, 0)
	// A session's PATH: the shim (a link to this very program) comes first.
	shimDir := t.TempDir()
	exe, _ := os.Executable()
	if err := os.Symlink(exe, filepath.Join(shimDir, "gh")); err != nil {
		t.Skip("symlinks unavailable")
	}
	t.Setenv("PATH", shimDir+string(os.PathListSeparator)+filepath.Dir(real))
	t.Setenv("SHELL", "")
	cfg, err := buildConfig(context.Background(), "127.0.0.1:0", "tok")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Gatekeeper == nil || cfg.Gatekeeper.RealGH != real {
		t.Fatalf("the real gh is %+v, want %q (the shim must be skipped, or every command would loop)", cfg.Gatekeeper, real)
	}
}

func TestWithoutGhThereIsNothingToGuard(t *testing.T) {
	home := t.TempDir()
	t.Setenv("DUNGEON_HOME", home)
	t.Setenv("PATH", "/nonexistent")
	t.Setenv("SHELL", "")
	cfg, err := buildConfig(context.Background(), "127.0.0.1:0", "tok")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Gatekeeper != nil {
		t.Fatalf("%+v", cfg.Gatekeeper)
	}
	if _, err := os.Stat(filepath.Join(home, "gatekeeper")); err == nil {
		t.Fatal("a shim directory was created although there is no gh")
	}
}
