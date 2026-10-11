package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dkage/dungeon-master/internal/campaign"
)

func onboardingRequest(t *testing.T, h http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Host = "127.0.0.1:7878"
	req.AddCookie(&http.Cookie{Name: cookieName, Value: testToken})
	req.Header.Set("Origin", "http://127.0.0.1:7878")
	if method != http.MethodGet {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestSampleCampaignUsesPrivateScratchWorkspaceAndIsIdempotent(t *testing.T) {
	root := t.TempDir()
	sampleDir := filepath.Join(root, "sample-workspace")
	registry, err := campaign.Open(t.Context(), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	h := Handler(Config{Token: testToken, Campaigns: registry, SampleDir: sampleDir}, testPort)

	first := onboardingRequest(t, h, http.MethodPost, "/api/onboarding/sample", "{}")
	if first.Code != http.StatusCreated {
		t.Fatalf("create: got %d %s", first.Code, first.Body)
	}
	var got campaign.Campaign
	if err := json.Unmarshal(first.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.Demo || !got.Sample || got.Name != "Sample (not a real project)" || got.Dir != sampleDir {
		t.Fatalf("unexpected sample campaign: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(sampleDir, "README.md")); err != nil {
		t.Fatalf("workspace was not prepared: %v", err)
	}

	second := onboardingRequest(t, h, http.MethodPost, "/api/onboarding/sample", "{}")
	if second.Code != http.StatusOK {
		t.Fatalf("second request: got %d %s", second.Code, second.Body)
	}
	var again campaign.Campaign
	if err := json.Unmarshal(second.Body.Bytes(), &again); err != nil {
		t.Fatal(err)
	}
	if again.ID != got.ID || len(registry.List()) != 1 {
		t.Fatalf("sample creation was not idempotent: first=%+v second=%+v campaigns=%d", got, again, len(registry.List()))
	}
	removed := onboardingRequest(t, h, http.MethodDelete, "/api/campaigns/"+got.ID, "")
	if removed.Code != http.StatusNoContent {
		t.Fatalf("remove sample: got %d %s", removed.Code, removed.Body)
	}
	if _, err := os.Stat(sampleDir); !os.IsNotExist(err) {
		t.Fatalf("sample scratch workspace was not removed: %v", err)
	}
}

func TestOnboardingSuggestionsOnlyReturnsGitProjectsAndRequiresAuth(t *testing.T) {
	plain := t.TempDir()
	h := Handler(Config{Token: testToken, StartDir: plain}, testPort)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/onboarding/suggestions", nil)
	req.Host = "127.0.0.1:7878"
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated request: got %d", rec.Code)
	}

	rec = onboardingRequest(t, h, http.MethodGet, "/api/onboarding/suggestions", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("suggestions: got %d %s", rec.Code, rec.Body)
	}
	var suggestions []projectSuggestion
	if err := json.Unmarshal(rec.Body.Bytes(), &suggestions); err != nil {
		t.Fatal(err)
	}
	for _, suggestion := range suggestions {
		if suggestion.Dir == plain {
			t.Fatal("plain directory was suggested")
		}
	}
}

func TestOnboardingSuggestionsIncludeLaunchDirectoryFirst(t *testing.T) {
	root := t.TempDir()
	cmd := exec.Command("git", "-C", root, "init", "-q")
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	cmd = exec.Command("git", "-C", root, "remote", "add", "origin", "git@github.com:example/project.git")
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("add remote: %v: %s", err, out)
	}
	h := Handler(Config{Token: testToken, StartDir: root}, testPort)
	rec := onboardingRequest(t, h, http.MethodGet, "/api/onboarding/suggestions", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("suggestions: got %d %s", rec.Code, rec.Body)
	}
	var suggestions []projectSuggestion
	if err := json.Unmarshal(rec.Body.Bytes(), &suggestions); err != nil {
		t.Fatal(err)
	}
	if len(suggestions) == 0 || suggestions[0].Dir != root || suggestions[0].Repo != "example/project" {
		t.Fatalf("launch directory was not suggested first: %+v", suggestions)
	}
}
