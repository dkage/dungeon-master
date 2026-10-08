package main

import (
	"context"
	"os"
	"path/filepath"

	"dungeon/internal/launcher"
	"dungeon/internal/party"
	"dungeon/internal/server"
	"dungeon/web"
)

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
	cfg := server.Config{
		Addr:    addr,
		Token:   token,
		Version: version,
		Assets:  web.Assets(),
		Party:   p,
		Shell:   os.Getenv("SHELL"),
	}
	if path, err := launcher.Resolve(ctx, p.Claude); err != nil {
		cfg.ClaudeErr = err.Error()
	} else {
		cfg.Claude = path
	}
	return cfg, nil
}
