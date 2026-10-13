package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/dkage/dungeon-master/internal/appearance"
)

func appearanceServer(t *testing.T, path string) *env {
	return startServer(t, func(c *Config) { c.Appearance = appearance.Open(path) })
}

func readAppearance(t *testing.T, e *env) appearanceResp {
	t.Helper()
	resp, out := e.req("GET", "/api/appearance", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET: %d %s", resp.StatusCode, out)
	}
	var got appearanceResp
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestAppearanceStartsAsDefaultsAndIsSavedAcrossARestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "appearance.json")
	e := appearanceServer(t, path)
	if got := readAppearance(t, e); got.Stored || got.Settings != appearance.Defaults() {
		t.Fatalf("fresh = %+v", got)
	}
	resp, out := e.req("PUT", "/api/appearance", map[string]any{"theme": "neutral", "terminalSize": 18}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT: %d %s", resp.StatusCode, out)
	}
	if got := readAppearance(t, e); !got.Stored || got.Settings.Theme != "neutral" || got.Settings.TerminalSize != 18 || got.Settings.Font != "almendra" {
		t.Fatalf("after PUT = %+v", got)
	}
	// A new server on the same file, another port and origin, sees the same look.
	again := appearanceServer(t, path)
	if got := readAppearance(t, again); !got.Stored || got.Settings.Theme != "neutral" || got.Settings.TerminalSize != 18 {
		t.Fatalf("after restart = %+v", got)
	}
}

func TestAppearanceRefusesBadWritesAndLeavesTheFileAlone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "appearance.json")
	e := appearanceServer(t, path)
	for name, tc := range map[string]struct {
		body   any
		mutate func(*http.Request)
		want   int
	}{
		"bad theme":         {map[string]any{"theme": "neon"}, nil, http.StatusBadRequest},
		"size out of range": {map[string]any{"terminalSize": 99}, nil, http.StatusBadRequest},
		"unsafe font name":  {map[string]any{"terminalFont": "custom", "terminalFontName": `x"; y`}, nil, http.StatusBadRequest},
		"unknown field":     {map[string]any{"colour": "red"}, nil, http.StatusBadRequest},
		"wrong type":        {map[string]any{"theme": 3}, nil, http.StatusBadRequest},
		"foreign origin":    {map[string]any{"theme": "neutral"}, func(r *http.Request) { r.Header.Set("Origin", "http://evil.test") }, http.StatusForbidden},
		"foreign host":      {map[string]any{"theme": "neutral"}, func(r *http.Request) { r.Host = "evil.test" }, http.StatusForbidden},
		"no token":          {map[string]any{"theme": "neutral"}, func(r *http.Request) { r.Header.Del("Authorization") }, http.StatusUnauthorized},
		"form content":      {map[string]any{"theme": "neutral"}, func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }, http.StatusUnsupportedMediaType},
	} {
		if resp, out := e.req("PUT", "/api/appearance", tc.body, tc.mutate); resp.StatusCode != tc.want {
			t.Errorf("%s: %d %s, want %d", name, resp.StatusCode, out, tc.want)
		}
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("a refused write created the file")
	}
	if got := readAppearance(t, e); got.Stored || got.Settings != appearance.Defaults() {
		t.Errorf("settings changed: %+v", got)
	}
	if resp, _ := e.req("GET", "/api/appearance", nil, func(r *http.Request) { r.Header.Del("Authorization") }); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("GET without a token: %d", resp.StatusCode)
	}
}
