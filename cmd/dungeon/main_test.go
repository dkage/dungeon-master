package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/dkage/dungeon-master/internal/campaign"
	"github.com/dkage/dungeon-master/internal/testutil"
)

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
	old := githubEndpoint
	githubEndpoint = srv.URL
	defer func() { githubEndpoint = old }()

	t.Setenv("DUNGEON_HOME", t.TempDir())
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("SHELL", "")
	ctx := context.Background()
	camp := campaign.Campaign{ID: "c", Repo: "dkage/dungeon-master"}

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
