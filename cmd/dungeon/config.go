package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/dkage/dungeon-master/internal/campaign"
	"github.com/dkage/dungeon-master/internal/history"
	"github.com/dkage/dungeon-master/internal/launcher"
	"github.com/dkage/dungeon-master/internal/party"
	"github.com/dkage/dungeon-master/internal/quests"
	"github.com/dkage/dungeon-master/internal/quests/demo"
	"github.com/dkage/dungeon-master/internal/quests/github"
	"github.com/dkage/dungeon-master/internal/server"
	"github.com/dkage/dungeon-master/web"
)

// githubEndpoint is GitHub's GraphQL URL; tests point it at a local server.
var githubEndpoint = github.DefaultEndpoint

// buildConfig assembles the server configuration from the user's local
// state: the party file, the claude binary and the shell. A missing claude is
// not fatal, because the app still starts and explains the problem in its UI.
func buildConfig(ctx context.Context, addr, token string) (server.Config, error) {
	dir, err := party.ConfigDir()
	if err != nil {
		return server.Config{}, err
	}
	p, err := party.LoadFile(filepath.Join(dir, party.FileName))
	if err != nil {
		return server.Config{}, err
	}
	campaigns, err := campaign.Open(ctx, filepath.Join(dir, "campaigns.json"), nil)
	if err != nil {
		return server.Config{}, err
	}
	hist, err := history.Open(filepath.Join(dir, "sessions.json"))
	if err != nil {
		return server.Config{}, err
	}
	// The GitHub CLI is optional: without it, quests need GH_TOKEN or
	// GITHUB_TOKEN. Resolved through the login shell like claude, since a
	// GUI launch lacks the user's PATH.
	ghPath, _ := launcher.Resolve(ctx, "gh")
	qsvc := quests.NewService(quests.ServiceOptions{
		GitHub: github.New(github.Options{Endpoint: githubEndpoint, Token: github.EnvOrGH(ghPath)}),
		Demo:   demo.New(),
	})
	cfg := server.Config{
		Addr:    addr,
		Token:   token,
		Version: version,
		Assets:  web.Assets(),
		Party:   p,
		Shell:   os.Getenv("SHELL"),

		Campaigns: campaigns,
		History:   hist,
		Quests:    qsvc,
	}
	if path, err := launcher.Resolve(ctx, p.Claude); err != nil {
		cfg.ClaudeErr = err.Error()
	} else {
		cfg.Claude = path
	}
	return cfg, nil
}
