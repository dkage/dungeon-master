package appearance

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDefaultsAreValid(t *testing.T) {
	if err := Validate(Defaults()); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRefusesEachBadField(t *testing.T) {
	bad := map[string]func(*Settings){
		"font":             func(s *Settings) { s.Font = "comic" },
		"sidebar low":      func(s *Settings) { s.SidebarSize = 9 },
		"terminal high":    func(s *Settings) { s.TerminalSize = 25 },
		"quests":           func(s *Settings) { s.QuestsSize = 0 },
		"word mode":        func(s *Settings) { s.WordMode = "loud" },
		"theme":            func(s *Settings) { s.Theme = "neon" },
		"terminal font":    func(s *Settings) { s.TerminalFont = "wingdings" },
		"quote in name":    func(s *Settings) { s.TerminalFontName = `Fira", evil` },
		"slash in name":    func(s *Settings) { s.TerminalFontName = "a/b" },
		"leading space":    func(s *Settings) { s.TerminalFontName = " Fira" },
		"name too long":    func(s *Settings) { s.TerminalFontName = strings.Repeat("a", 65) },
		"custom no name":   func(s *Settings) { s.TerminalFont = "custom" },
		"line height":      func(s *Settings) { s.TerminalLineHeight = 2 },
		"terminal colours": func(s *Settings) { s.TerminalColors = "rainbow" },
	}
	for name, mutate := range bad {
		s := Defaults()
		mutate(&s)
		if Validate(s) == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	ok := Defaults()
	ok.TerminalFont, ok.TerminalFontName = "custom", "JetBrains Mono"
	if err := Validate(ok); err != nil {
		t.Errorf("a plain custom font was refused: %v", err)
	}
}

func TestOpenWithoutAFileGivesDefaultsNotStored(t *testing.T) {
	s := Open(filepath.Join(t.TempDir(), "appearance.json"))
	got, stored := s.Get()
	if stored || got != Defaults() {
		t.Errorf("Get = %+v, %v", got, stored)
	}
}

func TestUpdatePersistsAtomicallyAndPrivately(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfg", "appearance.json")
	s := Open(path)
	if _, err := s.Update(func(x *Settings) { x.Theme, x.TerminalSize = "neutral", 17 }); err != nil {
		t.Fatal(err)
	}
	got, stored := Open(path).Get()
	if !stored || got.Theme != "neutral" || got.TerminalSize != 17 || got.Font != "almendra" {
		t.Errorf("reopened = %+v, %v", got, stored)
	}
	if runtime.GOOS != "windows" {
		for p, want := range map[string]os.FileMode{path: 0o600, filepath.Dir(path): 0o700} {
			fi, err := os.Stat(p)
			if err != nil || fi.Mode().Perm() != want {
				t.Errorf("%s mode = %v, %v; want %v", p, fi.Mode().Perm(), err, want)
			}
		}
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Errorf("leftover files: %v", entries)
	}
}

func TestUpdateRefusesInvalidAndKeepsTheOldValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "appearance.json")
	s := Open(path)
	if _, err := s.Update(func(x *Settings) { x.Theme = "nope" }); err == nil {
		t.Fatal("invalid theme accepted")
	}
	if got, stored := s.Get(); stored || got != Defaults() {
		t.Errorf("state changed: %+v %v", got, stored)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("a refused write created the file")
	}
}

func TestBadFilesFallBack(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		_ = os.WriteFile(p, []byte(body), 0o600)
		return p
	}
	if got, stored := Open(write("junk.json", "{not json")).Get(); stored || got != Defaults() {
		t.Errorf("junk: %+v %v", got, stored)
	}
	// One bad field is repaired; the good ones are kept.
	p := write("partial.json", `{"version":1,"settings":{"theme":"neutral","font":"comic","terminalSize":99,"terminalFont":"custom","terminalFontName":"x\"y"}}`)
	got, stored := Open(p).Get()
	if !stored || got.Theme != "neutral" || got.Font != "almendra" || got.TerminalSize != 13 || got.TerminalFont != "stack" || got.TerminalFontName != "" {
		t.Errorf("partial: %+v %v", got, stored)
	}
	if err := Validate(got); err != nil {
		t.Errorf("repaired settings are invalid: %v", err)
	}
}

func TestMemoryOnlyStoreWorks(t *testing.T) {
	s := Open("")
	if _, err := s.Update(func(x *Settings) { x.Theme = "neutral" }); err != nil {
		t.Fatal(err)
	}
	if got, stored := s.Get(); !stored || got.Theme != "neutral" {
		t.Errorf("%+v %v", got, stored)
	}
}
