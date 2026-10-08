package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/dkage/dungeon-master/internal/campaign"
	"github.com/dkage/dungeon-master/internal/history"
	"github.com/dkage/dungeon-master/internal/launcher"
	"github.com/dkage/dungeon-master/internal/party"
	"github.com/dkage/dungeon-master/internal/quartermaster"
	"github.com/dkage/dungeon-master/internal/quests"
	"github.com/dkage/dungeon-master/internal/quests/demo"
	"github.com/dkage/dungeon-master/internal/quests/github"
	"github.com/dkage/dungeon-master/internal/server"
	"github.com/dkage/dungeon-master/web"
)

// githubEndpoint returns GitHub's GraphQL URL. DUNGEON_GITHUB_ENDPOINT can
// replace it, for tests that run against a local fake, but only with a
// loopback URL: the token travels in these requests, so an environment
// variable must never be able to point them at another host.
func githubEndpoint() (string, error) {
	v := os.Getenv("DUNGEON_GITHUB_ENDPOINT")
	if v == "" {
		return github.DefaultEndpoint, nil
	}
	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("DUNGEON_GITHUB_ENDPOINT: %q is not an http(s) URL", v)
	}
	if h := u.Hostname(); h != "localhost" && !net.ParseIP(h).IsLoopback() {
		return "", fmt.Errorf("DUNGEON_GITHUB_ENDPOINT: %q is not a loopback URL", v)
	}
	return v, nil
}

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
	endpoint, err := githubEndpoint()
	if err != nil {
		return server.Config{}, err
	}
	// Every GitHub request goes through the Quartermaster; its pause survives
	// restarts.
	qm, err := quartermaster.New(quartermaster.Options{
		Store: quartermaster.FileStore{Path: filepath.Join(dir, "quartermaster.json")},
	})
	if err != nil {
		return server.Config{}, err
	}
	qsvc := quests.NewService(quests.ServiceOptions{
		GitHub: github.New(github.Options{
			Endpoint: endpoint,
			Token:    github.EnvOrGH(ghPath),
			HTTP:     &http.Client{Timeout: 20 * time.Second, Transport: qm.Transport(http.DefaultTransport)},
			OnRateLimit: func(reset time.Time) {
				qm.Trip(reset, "GitHub reported a rate limit")
			},
		}),
		Demo: demo.New(),
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

		Quartermaster: qm,
	}
	if path, err := launcher.Resolve(ctx, p.Claude); err != nil {
		cfg.ClaudeErr = err.Error()
	} else {
		cfg.Claude = path
	}
	return cfg, nil
}
