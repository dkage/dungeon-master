package skillcatalog

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func skill(name, desc string) string {
	return "---\nname: " + name + "\ndescription: " + desc + "\n---\n# body\n"
}

func names(r Result) string {
	var n []string
	for _, s := range r.Skills {
		n = append(n, s.Name)
	}
	return strings.Join(n, ",")
}

func TestScanFindsSkillsSortedAndStopsAtASkill(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "b", "SKILL.md"), skill("beta", "second"))
	write(t, filepath.Join(root, "a", "SKILL.md"), skill("alpha", "first"))
	write(t, filepath.Join(root, "group", "deep", "SKILL.md"), skill("deep", "nested"))
	// Supporting files of a skill are not skills.
	write(t, filepath.Join(root, "a", "templates", "SKILL.md"), skill("not-a-skill", "x"))
	got := Scan(root)
	if names(got) != "alpha,beta,deep" {
		t.Fatalf("skills = %s (%v)", names(got), got.Warnings)
	}
	a := got.Skills[0]
	if a.Description != "first" || a.Path != "a" || !filepath.IsAbs(a.Dir) || filepath.Base(a.File) != "SKILL.md" || !a.UserInvocable {
		t.Fatalf("alpha = %+v", a)
	}
	if got.Skills[2].Path != "group/deep" {
		t.Fatalf("path = %q", got.Skills[2].Path)
	}
}

func TestScanSkipsHiddenDependencyAndTooDeepDirectories(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, ".git", "x", "SKILL.md"), skill("hidden", ""))
	write(t, filepath.Join(root, "node_modules", "x", "SKILL.md"), skill("dep", ""))
	write(t, filepath.Join(root, "a", "b", "c", "SKILL.md"), skill("three", ""))     // depth 3: found
	write(t, filepath.Join(root, "a", "b", "c", "d", "SKILL.md"), skill("four", "")) // depth 4: not
	if got := names(Scan(root)); got != "three" {
		t.Fatalf("skills = %s", got)
	}
}

func TestScanTakesTheDirectoryNameWhenThereIsNoFrontMatter(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "plain-skill", "SKILL.md"), "# Just a heading\n")
	got := Scan(root)
	if names(got) != "plain-skill" || got.Skills[0].Description != "" {
		t.Fatalf("skills = %+v", got.Skills)
	}
}

func TestScanSkipsMalformedOversizedAndHostileNamesWithAWarning(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "ok", "SKILL.md"), skill("ok", "fine"))
	write(t, filepath.Join(root, "bad-name", "SKILL.md"), skill("../../etc/passwd", "x"))
	write(t, filepath.Join(root, "spacey", "SKILL.md"), "---\nname: \"a b\"\n---\n")
	write(t, filepath.Join(root, "huge", "SKILL.md"), "---\nname: huge\n---\n"+strings.Repeat("x", MaxFileBytes+1))
	if err := os.MkdirAll(filepath.Join(root, "dir-skill", "SKILL.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := Scan(root)
	if names(got) != "ok" {
		t.Fatalf("skills = %s", names(got))
	}
	if len(got.Warnings) != 4 {
		t.Fatalf("warnings = %v", got.Warnings)
	}
}

func TestScanRejectsSymlinksThatEscapeTheSource(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	outside := t.TempDir()
	write(t, filepath.Join(outside, "evil", "SKILL.md"), skill("evil", "outside"))
	write(t, filepath.Join(outside, "secret.md"), skill("secret", "outside"))
	root := t.TempDir()
	write(t, filepath.Join(root, "ok", "SKILL.md"), skill("ok", ""))
	_ = os.Symlink(filepath.Join(outside, "evil"), filepath.Join(root, "linked-dir"))
	_ = os.MkdirAll(filepath.Join(root, "file-link"), 0o755)
	_ = os.Symlink(filepath.Join(outside, "secret.md"), filepath.Join(root, "file-link", "SKILL.md"))
	// A link that stays inside the source is fine.
	_ = os.Symlink(filepath.Join(root, "ok"), filepath.Join(root, "alias"))
	got := Scan(root)
	for _, s := range got.Skills {
		if s.Name == "evil" || s.Name == "secret" {
			t.Fatalf("escaped: %+v", s)
		}
	}
	if len(got.Warnings) < 2 {
		t.Fatalf("warnings = %v", got.Warnings)
	}
}

func TestScanWarnsOnDuplicateNamesInOneSourceAndKeepsTheFirst(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "a", "SKILL.md"), skill("same", "first"))
	write(t, filepath.Join(root, "b", "SKILL.md"), skill("Same", "second"))
	got := Scan(root)
	if len(got.Skills) != 1 || got.Skills[0].Description != "first" || len(got.Warnings) != 1 {
		t.Fatalf("got %+v", got)
	}
}

func TestScanOfAMissingRootIsEmptyWithAWarning(t *testing.T) {
	got := Scan(filepath.Join(t.TempDir(), "gone"))
	if len(got.Skills) != 0 || len(got.Warnings) != 1 {
		t.Fatalf("got %+v", got)
	}
}

func TestFrontMatterForms(t *testing.T) {
	for name, tc := range map[string]struct{ in, name, desc, inv string }{
		"quoted":     {"---\nname: 'q'\ndescription: \"d: with colon\"\n---\n", "q", "d: with colon", ""},
		"folded":     {"---\nname: f\ndescription: >\n  one\n  two\n---\n", "f", "one two", ""},
		"literal":    {"---\nname: l\ndescription: |-\n  a\n  b\n---\n", "l", "a b", ""},
		"model only": {"---\nname: m\ndisable-model-invocation: true\n---\n", "m", "", "true"},
		"none":       {"no front matter", "", "", ""},
		"unclosed":   {"---\nname: u\ndescription: x\n", "u", "x", ""},
	} {
		m := frontMatter([]byte(tc.in))
		if m["name"] != tc.name || m["description"] != tc.desc || m["disable-model-invocation"] != tc.inv {
			t.Errorf("%s: %v", name, m)
		}
	}
}

func TestDescriptionIsCleanedAndBounded(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "s", "SKILL.md"), "---\nname: s\ndescription: \"a\x1b[31m red\t\x00b "+strings.Repeat("z", 800)+"\"\n---\n")
	d := Scan(root).Skills[0].Description
	if strings.ContainsAny(d, "\x1b\t\x00") || len([]rune(d)) > MaxDescription+1 {
		t.Fatalf("description = %q", d)
	}
}

func TestUserInvocableReflectsMetadata(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "a", "SKILL.md"), "---\nname: a\nuser-invocable: false\n---\n")
	write(t, filepath.Join(root, "b", "SKILL.md"), "---\nname: b\ndisable-model-invocation: true\n---\n")
	for _, s := range Scan(root).Skills {
		if s.Name == "a" && s.UserInvocable {
			t.Error("a should not be user invocable")
		}
		if s.Name == "b" && !s.UserInvocable {
			t.Error("b disables only the model; the user can still invoke it")
		}
	}
}

func TestBuildAppliesPrecedenceShadowingAndDisabledSources(t *testing.T) {
	one, two, off := t.TempDir(), t.TempDir(), t.TempDir()
	write(t, filepath.Join(one, "tdd", "SKILL.md"), skill("tdd", "from one"))
	write(t, filepath.Join(two, "tdd", "SKILL.md"), skill("tdd", "from two"))
	write(t, filepath.Join(two, "only", "SKILL.md"), skill("only", "x"))
	write(t, filepath.Join(off, "hidden", "SKILL.md"), skill("hidden", "x"))
	cat := Build([]Source{
		{ID: "s1", Name: "One", Root: one, Revision: "abc", Enabled: true},
		{ID: "s2", Name: "Two", Root: two, Enabled: true},
		{ID: "s3", Name: "Off", Root: off, Enabled: false},
		{ID: "s4", Name: "Unfetched", Enabled: true},
	})
	var ids []string
	for _, e := range cat.Entries {
		ids = append(ids, e.ID)
	}
	if strings.Join(ids, " ") != "s1/tdd s2/only s2/tdd" && strings.Join(ids, " ") != "s1/tdd s2/tdd s2/only" {
		t.Fatalf("entries = %v", ids)
	}
	for _, e := range cat.Entries {
		switch e.ID {
		case "s1/tdd":
			if e.Shadowed || e.Revision != "abc" || e.SourceName != "One" {
				t.Errorf("winner = %+v", e)
			}
		case "s2/tdd":
			if !e.Shadowed || e.ShadowedBy != "One" {
				t.Errorf("shadowed = %+v", e)
			}
		}
	}
	if len(cat.Reports) != 4 || cat.Reports[2].Count != 1 || cat.Reports[3].Unavailable == "" {
		t.Fatalf("reports = %+v", cat.Reports)
	}
}
