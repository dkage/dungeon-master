// Package appearance keeps the user's look-and-feel choices (reading font,
// text sizes, theme, terminal font, line height and colours) in a local state
// file, so every browser, port and window of this machine shows the same look.
// It lives in the user config dir like the other state; nothing is written to
// a repository or to GitHub.
package appearance

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"sync"

	"github.com/dkage/dungeon-master/internal/fsutil"
)

// Settings is every appearance choice. The allowed values are the ones in
// the lists below; anything else is refused on write and replaced by the
// default on load.
type Settings struct {
	Font          string `json:"font"`
	SidebarSize   int    `json:"sidebarSize"`
	TerminalSize  int    `json:"terminalSize"`
	QuestsSize    int    `json:"questsSize"`
	WordMode      string `json:"wordMode"`
	Theme         string `json:"theme"`
	Notifications bool   `json:"notifications"`

	// TerminalFont is "stack" (Nerd Font names, bundled Fira Code, system
	// monospace), "bundled", "system" or "custom"; custom uses TerminalFontName.
	TerminalFont       string  `json:"terminalFont"`
	TerminalFontName   string  `json:"terminalFontName"`
	TerminalLineHeight float64 `json:"terminalLineHeight"`
	// TerminalColors is "default", "high-contrast" or "light".
	TerminalColors string `json:"terminalColors"`
}

// MinSize and MaxSize bound every text size.
const (
	MinSize = 10
	MaxSize = 24
)

var (
	fonts         = []string{"almendra", "eb-garamond", "alegreya"}
	wordModes     = []string{"hints", "themed", "plain"}
	themes        = []string{"dungeon", "neutral"}
	terminalFonts = []string{"stack", "bundled", "system", "custom"}
	lineHeights   = []float64{1.15, 1.3, 1.5}
	colourSchemes = []string{"default", "high-contrast", "light"}

	// A plain font name: letters, digits, spaces, dots, dashes, underscores.
	// No quotes, commas, slashes or backslashes, so it cannot break out of the
	// font-family list it is placed in.
	fontName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 ._-]{0,63}$`)
)

// Defaults is what a fresh install shows.
func Defaults() Settings {
	return Settings{
		Font: "almendra", SidebarSize: 14, TerminalSize: 13, QuestsSize: 14,
		WordMode: "hints", Theme: "dungeon",
		TerminalFont: "stack", TerminalLineHeight: 1.3, TerminalColors: "default",
	}
}

// Validate says what is wrong with s, or returns nil.
func Validate(s Settings) error {
	switch {
	case !slices.Contains(fonts, s.Font):
		return fmt.Errorf("font must be one of %v", fonts)
	case !sizeOK(s.SidebarSize):
		return errors.New("sidebarSize is out of range")
	case !sizeOK(s.TerminalSize):
		return errors.New("terminalSize is out of range")
	case !sizeOK(s.QuestsSize):
		return errors.New("questsSize is out of range")
	case !slices.Contains(wordModes, s.WordMode):
		return fmt.Errorf("wordMode must be one of %v", wordModes)
	case !slices.Contains(themes, s.Theme):
		return fmt.Errorf("theme must be one of %v", themes)
	case !slices.Contains(terminalFonts, s.TerminalFont):
		return fmt.Errorf("terminalFont must be one of %v", terminalFonts)
	case s.TerminalFontName != "" && !fontName.MatchString(s.TerminalFontName):
		return errors.New("terminalFontName must be a plain font name")
	case s.TerminalFont == "custom" && s.TerminalFontName == "":
		return errors.New("terminalFontName is required for a custom font")
	case !slices.Contains(lineHeights, s.TerminalLineHeight):
		return fmt.Errorf("terminalLineHeight must be one of %v", lineHeights)
	case !slices.Contains(colourSchemes, s.TerminalColors):
		return fmt.Errorf("terminalColors must be one of %v", colourSchemes)
	}
	return nil
}

func sizeOK(n int) bool { return n >= MinSize && n <= MaxSize }

// repair replaces each invalid field of s with its default, so one bad value
// in a hand-edited file does not discard the rest.
func repair(s Settings) Settings {
	d := Defaults()
	if !slices.Contains(fonts, s.Font) {
		s.Font = d.Font
	}
	if !sizeOK(s.SidebarSize) {
		s.SidebarSize = d.SidebarSize
	}
	if !sizeOK(s.TerminalSize) {
		s.TerminalSize = d.TerminalSize
	}
	if !sizeOK(s.QuestsSize) {
		s.QuestsSize = d.QuestsSize
	}
	if !slices.Contains(wordModes, s.WordMode) {
		s.WordMode = d.WordMode
	}
	if !slices.Contains(themes, s.Theme) {
		s.Theme = d.Theme
	}
	if !slices.Contains(terminalFonts, s.TerminalFont) {
		s.TerminalFont = d.TerminalFont
	}
	if s.TerminalFontName != "" && !fontName.MatchString(s.TerminalFontName) {
		s.TerminalFontName = ""
	}
	if s.TerminalFont == "custom" && s.TerminalFontName == "" {
		s.TerminalFont = d.TerminalFont
	}
	if !slices.Contains(lineHeights, s.TerminalLineHeight) {
		s.TerminalLineHeight = d.TerminalLineHeight
	}
	if !slices.Contains(colourSchemes, s.TerminalColors) {
		s.TerminalColors = d.TerminalColors
	}
	return s
}

type fileFormat struct {
	Version  int      `json:"version"`
	Settings Settings `json:"settings"`
}

// Store is the persisted settings. The zero path keeps them in memory only.
type Store struct {
	path string

	mu     sync.Mutex
	cur    Settings
	stored bool
}

// Open loads the file at path. A missing, unreadable or malformed file gives
// the defaults and stored == false, which is how the page knows it may import
// what the browser held before.
func Open(path string) *Store {
	s := &Store{path: path, cur: Defaults()}
	if path == "" {
		return s
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	f := fileFormat{Settings: Defaults()}
	if json.Unmarshal(data, &f) != nil {
		return s
	}
	s.cur, s.stored = repair(f.Settings), true
	return s
}

// Get returns the current settings and whether they were ever saved.
func (s *Store) Get() (Settings, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cur, s.stored
}

// Update lets change adjust a copy of the current settings, validates the
// result and writes it atomically (0600, in a 0700 directory).
func (s *Store) Update(change func(*Settings)) (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.cur
	change(&next)
	if err := Validate(next); err != nil {
		return s.cur, err
	}
	if s.path != "" {
		data, err := json.MarshalIndent(fileFormat{Version: 1, Settings: next}, "", "  ")
		if err != nil {
			return s.cur, err
		}
		if err := fsutil.WriteFileAtomic(s.path, append(data, '\n')); err != nil {
			return s.cur, err
		}
	}
	s.cur, s.stored = next, true
	return next, nil
}
