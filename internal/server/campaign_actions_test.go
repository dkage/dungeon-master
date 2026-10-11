package server

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dkage/dungeon-master/internal/campaign"
)

func TestPickFolderReturnsPickedPathCancellationAndFallback(t *testing.T) {
	selected := t.TempDir()
	e := startServer(t, func(cfg *Config) {
		cfg.PickFolder = func(context.Context) (string, error) { return selected, nil }
	})
	resp, out := e.req(http.MethodPost, "/api/pick-folder", map[string]any{}, nil)
	var picked map[string]any
	if err := json.Unmarshal(out, &picked); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || picked["path"] != selected {
		t.Fatalf("picked path: %d %s", resp.StatusCode, out)
	}

	e = startServer(t, func(cfg *Config) {
		cfg.PickFolder = func(context.Context) (string, error) { return "", errPickerCancelled }
	})
	resp, out = e.req(http.MethodPost, "/api/pick-folder", map[string]any{}, nil)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(out), `"cancelled":true`) {
		t.Fatalf("cancelled picker: %d %s", resp.StatusCode, out)
	}

	e = startServer(t, func(cfg *Config) {
		cfg.PickFolder = func(context.Context) (string, error) { return "", errPickerUnavailable }
	})
	resp, out = e.req(http.MethodPost, "/api/pick-folder", map[string]any{}, nil)
	if resp.StatusCode != http.StatusNotImplemented || !strings.Contains(string(out), "picker_unavailable") {
		t.Fatalf("unavailable picker: %d %s", resp.StatusCode, out)
	}
}

func TestPickFolderSerializesDialogsAndTimesOut(t *testing.T) {
	selected := t.TempDir()
	started, release := make(chan struct{}), make(chan struct{})
	e := startServer(t, func(cfg *Config) {
		cfg.PickFolder = func(ctx context.Context) (string, error) {
			close(started)
			select {
			case <-release:
				return selected, nil
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
	})
	first := make(chan *http.Response, 1)
	go func() {
		resp, _ := e.req(http.MethodPost, "/api/pick-folder", map[string]any{}, nil)
		first <- resp
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("picker did not start")
	}
	resp, out := e.req(http.MethodPost, "/api/pick-folder", map[string]any{}, nil)
	if resp.StatusCode != http.StatusConflict || !strings.Contains(string(out), "picker_busy") {
		t.Fatalf("second picker: %d %s", resp.StatusCode, out)
	}
	close(release)
	select {
	case resp := <-first:
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("first picker: %d", resp.StatusCode)
		}
	case <-time.After(time.Second):
		t.Fatal("first picker did not return")
	}

	e = startServer(t, func(cfg *Config) {
		cfg.PickFolder = func(context.Context) (string, error) { return "", context.DeadlineExceeded }
	})
	resp, out = e.req(http.MethodPost, "/api/pick-folder", map[string]any{}, nil)
	if resp.StatusCode != http.StatusGatewayTimeout || !strings.Contains(string(out), "picker_timeout") {
		t.Fatalf("picker timeout: %d %s", resp.StatusCode, out)
	}
}

func TestCampaignActionsOpenFolderAndReuseDuplicate(t *testing.T) {
	var opened string
	e := startServer(t, func(cfg *Config) {
		cfg.OpenFolder = func(_ context.Context, path string) error { opened = path; return nil }
	})
	resp, _ := e.req(http.MethodPost, "/api/campaigns/"+e.campaign+"/open-folder", map[string]any{}, nil)
	if resp.StatusCode != http.StatusNoContent || filepath.Clean(opened) != filepath.Clean(e.dir) {
		t.Fatalf("open folder: %d %q", resp.StatusCode, opened)
	}

	resp, out := e.req(http.MethodPost, "/api/campaigns", map[string]any{"dir": e.dir, "name": "duplicate"}, nil)
	var reused campaign.Campaign
	if err := json.Unmarshal(out, &reused); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || reused.ID != e.campaign || len(e.cfg.Campaigns.List()) != 1 {
		t.Fatalf("duplicate campaign should return the existing entry: %d %s", resp.StatusCode, out)
	}
}

func TestFolderActionsRejectForeignOriginsAndMissingTokens(t *testing.T) {
	calls := 0
	e := startServer(t, func(cfg *Config) {
		cfg.PickFolder = func(context.Context) (string, error) { calls++; return t.TempDir(), nil }
	})
	for _, tc := range []struct {
		name   string
		mutate func(*http.Request)
		status int
	}{
		{"foreign origin", func(r *http.Request) { r.Header.Set("Origin", "https://foreign.test") }, http.StatusForbidden},
		{"missing token", func(r *http.Request) { r.Header.Del("Authorization") }, http.StatusUnauthorized},
	} {
		resp, _ := e.req(http.MethodPost, "/api/pick-folder", map[string]any{}, tc.mutate)
		if resp.StatusCode != tc.status {
			t.Errorf("%s: got %d, want %d", tc.name, resp.StatusCode, tc.status)
		}
	}
	if calls != 0 {
		t.Fatalf("hostile requests reached the native picker %d times", calls)
	}
}
