package server

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dkage/dungeon-master/internal/skillcatalog"
	"github.com/dkage/dungeon-master/internal/skillsource"
)

const testRev = "cccccccccccccccccccccccccccccccccccccccc"

type stubFetcher struct{ err error }

func (f stubFetcher) Fetch(_ context.Context, _, _, dest string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	if err := os.MkdirAll(dest, 0o700); err != nil {
		return "", err
	}
	return testRev, os.WriteFile(filepath.Join(dest, "SKILL.md"), []byte("x"), 0o600)
}

func skillServer(t *testing.T, f skillsource.Fetcher) (*env, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "skill-sources.json")
	reg, err := skillsource.Open(path, skillsource.Options{Root: filepath.Join(dir, "co"), Fetcher: f})
	if err != nil {
		t.Fatal(err)
	}
	return startServer(t, func(c *Config) { c.SkillSources = reg }), path
}

func decodeJSON[T any](t *testing.T, out []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(out, &v); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	return v
}

func TestSkillSourcesRegisterMapToACampaignAndLeaveProjectFilesAlone(t *testing.T) {
	e, statePath := skillServer(t, stubFetcher{})
	projectBefore := dirListing(t, e.dir)
	folder := t.TempDir()

	resp, out := e.req("POST", "/api/skill-sources", map[string]any{"name": "mine", "kind": "folder", "location": folder}, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add folder: %d %s", resp.StatusCode, out)
	}
	mine := decodeJSON[skillsource.Source](t, out)
	resp, out = e.req("POST", "/api/skill-sources", map[string]any{"suggestion": "mattpocock", "pin": "v1.0.0"}, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add suggestion: %d %s", resp.StatusCode, out)
	}
	pocock := decodeJSON[skillsource.Source](t, out)
	if pocock.License != "MIT" || pocock.Notice == "" || pocock.Verified == nil || pocock.Verified.Revision != testRev {
		t.Fatalf("suggested source = %+v", pocock)
	}

	list := decodeJSON[skillSourcesResp](t, getBody(t, e, "/api/skill-sources"))
	if len(list.Sources) != 2 || len(list.Suggestions) == 0 {
		t.Fatalf("list = %+v", list)
	}

	// Map for this campaign: the external source first, the folder off.
	resp, out = e.req("PUT", "/api/campaigns/"+e.campaign+"/skill-sources", map[string]any{"sources": []map[string]any{
		{"sourceId": pocock.ID, "enabled": true}, {"sourceId": mine.ID, "enabled": false},
	}}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("map: %d %s", resp.StatusCode, out)
	}
	got := decodeJSON[campaignSkillSourcesResp](t, getBody(t, e, "/api/campaigns/"+e.campaign+"/skill-sources"))
	if len(got.Sources) != 2 || got.Sources[0].ID != pocock.ID || !got.Sources[0].Enabled || got.Sources[1].Enabled || !got.Sources[0].Override {
		t.Fatalf("mapping = %+v", got.Sources)
	}

	// Nothing in the project changed, and the state file is private.
	if after := dirListing(t, e.dir); after != projectBefore {
		t.Fatalf("project files changed:\n%s\n---\n%s", projectBefore, after)
	}
	if fi, err := os.Stat(statePath); err != nil || (fi.Mode().Perm()&0o077 != 0 && !strings.Contains(os.Getenv("OS"), "Windows")) {
		t.Fatalf("state file: %v %v", fi, err)
	}

	// Clearing returns to inheritance.
	e.req("PUT", "/api/campaigns/"+e.campaign+"/skill-sources", map[string]any{"sources": []any{}}, nil)
	got = decodeJSON[campaignSkillSourcesResp](t, getBody(t, e, "/api/campaigns/"+e.campaign+"/skill-sources"))
	if got.Sources[0].Override {
		t.Fatalf("not cleared: %+v", got.Sources)
	}
}

func TestSkillSourcesRefusalsAndFailedRefresh(t *testing.T) {
	failing := &toggleFetcher{}
	e, _ := skillServer(t, failing)
	_, out := e.req("POST", "/api/skill-sources", map[string]any{"name": "g", "kind": "git", "location": "https://example.test/a.git", "pin": "main"}, nil)
	g := decodeJSON[skillsource.Source](t, out)

	failing.err = os.ErrDeadlineExceeded
	resp, out := e.req("POST", "/api/skill-sources/"+g.ID+"/refresh", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("refresh: %d %s", resp.StatusCode, out)
	}
	after := decodeJSON[skillsource.Source](t, out)
	if after.Verified == nil || after.Verified.Revision != testRev || after.Error == nil || after.Error.Message == "" {
		t.Fatalf("after failed refresh = %+v", after)
	}

	for name, tc := range map[string]struct {
		method, path string
		body         any
		want         int
	}{
		"hostile pin":      {"POST", "/api/skill-sources", map[string]any{"name": "x", "kind": "git", "location": "https://example.test/b.git", "pin": "--evil"}, 400},
		"hostile remote":   {"POST", "/api/skill-sources", map[string]any{"name": "x", "kind": "git", "location": "--upload-pack=x", "pin": "main"}, 400},
		"relative folder":  {"POST", "/api/skill-sources", map[string]any{"name": "x", "kind": "folder", "location": "rel"}, 400},
		"unknown field":    {"POST", "/api/skill-sources", map[string]any{"name": "x", "bogus": 1}, 400},
		"bad suggestion":   {"POST", "/api/skill-sources", map[string]any{"suggestion": "nope", "pin": "main"}, 400},
		"unknown source":   {"POST", "/api/skill-sources/ffffffffffffffff/refresh", nil, 404},
		"bad order":        {"PUT", "/api/skill-sources/order", map[string]any{"ids": []string{}}, 400},
		"unknown campaign": {"GET", "/api/campaigns/nope/skill-sources", nil, 404},
		"map unknown":      {"PUT", "/api/campaigns/" + e.campaign + "/skill-sources", map[string]any{"sources": []map[string]any{{"sourceId": "ffffffffffffffff", "enabled": true}}}, 400},
		"delete unknown":   {"DELETE", "/api/skill-sources/ffffffffffffffff", nil, 404},
	} {
		if resp, out := e.req(tc.method, tc.path, tc.body, nil); resp.StatusCode != tc.want {
			t.Errorf("%s: %d %s, want %d", name, resp.StatusCode, out, tc.want)
		}
	}

	// A write without the token is refused like every other.
	if resp, _ := e.req("POST", "/api/skill-sources", map[string]any{}, func(r *http.Request) { r.Header.Del("Authorization") }); resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusForbidden {
		t.Errorf("unauthenticated: %d", resp.StatusCode)
	}
}

func TestSkillSourcesReorderToggleAndRemove(t *testing.T) {
	e, _ := skillServer(t, stubFetcher{})
	var ids []string
	for _, n := range []string{"a", "b"} {
		_, out := e.req("POST", "/api/skill-sources", map[string]any{"name": n, "kind": "folder", "location": t.TempDir()}, nil)
		ids = append(ids, decodeJSON[skillsource.Source](t, out).ID)
	}
	resp, out := e.req("PUT", "/api/skill-sources/order", map[string]any{"ids": []string{ids[1], ids[0]}}, nil)
	if resp.StatusCode != http.StatusOK || decodeJSON[skillSourcesResp](t, out).Sources[0].ID != ids[1] {
		t.Fatalf("reorder: %d %s", resp.StatusCode, out)
	}
	resp, out = e.req("PATCH", "/api/skill-sources/"+ids[0], map[string]any{"enabled": false}, nil)
	if resp.StatusCode != http.StatusOK || decodeJSON[skillsource.Source](t, out).Enabled {
		t.Fatalf("disable: %d %s", resp.StatusCode, out)
	}
	if resp, _ := e.req("DELETE", "/api/skill-sources/"+ids[0], nil, nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("remove: %d", resp.StatusCode)
	}
	if l := decodeJSON[skillSourcesResp](t, getBody(t, e, "/api/skill-sources")); len(l.Sources) != 1 {
		t.Fatalf("list after remove = %+v", l.Sources)
	}
}

type toggleFetcher struct{ err error }

func (f *toggleFetcher) Fetch(ctx context.Context, l, p, d string) (string, error) {
	return stubFetcher{f.err}.Fetch(ctx, l, p, d)
}

func getBody(t *testing.T, e *env, path string) []byte {
	t.Helper()
	resp, out := e.req("GET", path, nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: %d %s", path, resp.StatusCode, out)
	}
	return out
}

func dirListing(t *testing.T, dir string) string {
	t.Helper()
	var sb strings.Builder
	_ = filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err == nil {
			sb.WriteString(p + " " + fi.ModTime().String() + "\n")
		}
		return nil
	})
	return sb.String()
}

func TestCampaignSkillCatalogFollowsTheMappingAndKeepsQualifiedIdentity(t *testing.T) {
	e, _ := skillServer(t, stubFetcher{})
	mk := func(name string) (skillsource.Source, string) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "tdd"), 0o755); err != nil {
			t.Fatal(err)
		}
		body := "---\nname: tdd\ndescription: from " + name + "\n---\n"
		if err := os.WriteFile(filepath.Join(dir, "tdd", "SKILL.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		_, out := e.req("POST", "/api/skill-sources", map[string]any{"name": name, "kind": "folder", "location": dir}, nil)
		return decodeJSON[skillsource.Source](t, out), dir
	}
	one, _ := mk("One")
	two, twoDir := mk("Two")

	// The source list reports each source's count.
	list := decodeJSON[skillSourcesResp](t, getBody(t, e, "/api/skill-sources"))
	if len(list.Reports) != 2 || list.Reports[0].Count != 1 {
		t.Fatalf("reports = %+v", list.Reports)
	}

	cat := decodeJSON[skillcatalog.Catalog](t, getBody(t, e, "/api/campaigns/"+e.campaign+"/skills"))
	if len(cat.Entries) != 2 || cat.Entries[0].ID != one.ID+"/tdd" || cat.Entries[0].Shadowed || !cat.Entries[1].Shadowed || cat.Entries[1].ID != two.ID+"/tdd" {
		t.Fatalf("catalog = %+v", cat.Entries)
	}
	if cat.Entries[1].Dir != filepath.Join(evalDir(t, twoDir), "tdd") {
		t.Fatalf("dir = %q", cat.Entries[1].Dir)
	}

	// Disabling a source for this campaign removes its skills; the other's
	// skill is no longer shadowed, and the disabled one is never substituted.
	e.req("PUT", "/api/campaigns/"+e.campaign+"/skill-sources", map[string]any{"sources": []map[string]any{
		{"sourceId": two.ID, "enabled": true}, {"sourceId": one.ID, "enabled": false},
	}}, nil)
	cat = decodeJSON[skillcatalog.Catalog](t, getBody(t, e, "/api/campaigns/"+e.campaign+"/skills"))
	if len(cat.Entries) != 1 || cat.Entries[0].ID != two.ID+"/tdd" || cat.Entries[0].Shadowed {
		t.Fatalf("after mapping = %+v", cat.Entries)
	}
	if resp, _ := e.req("GET", "/api/campaigns/nope/skills", nil, nil); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown campaign: %d", resp.StatusCode)
	}
}

func TestGitSourceSkillsAreReadFromItsVerifiedCheckout(t *testing.T) {
	// The stub fetcher writes a bare SKILL.md at the checkout root.
	e, _ := skillServer(t, stubFetcher{})
	_, out := e.req("POST", "/api/skill-sources", map[string]any{"name": "g", "kind": "git", "location": "https://example.test/a.git"}, nil)
	g := decodeJSON[skillsource.Source](t, out)
	cat := decodeJSON[skillcatalog.Catalog](t, getBody(t, e, "/api/campaigns/"+e.campaign+"/skills"))
	if len(cat.Entries) != 1 || cat.Entries[0].SourceID != g.ID || cat.Entries[0].Revision != testRev {
		t.Fatalf("catalog = %+v", cat)
	}
}

func evalDir(t *testing.T, d string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(d)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
