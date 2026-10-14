package skillcatalog

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Compatibility notes are what a quick, read-only look at a skill can say
// about how it fits this app's workflow: GitHub issues are the tracker, and a
// method never creates files or issues on its own. They come from patterns in
// the skill's text and from the files beside it. They are hints, not proof:
// they never block a launch and never replace reading the skill.
var (
	writesGitHub = regexp.MustCompile(`(?i)\bgh\s+(issue\s+(create|edit|comment|close|reopen|transfer|delete)|pr\s+(create|merge|edit|comment|close))\b|\bgh\s+api\b[^\n]*(-X|--method)[ =]*(POST|PATCH|PUT|DELETE)`)
	fileTracker  = regexp.MustCompile(`(?i)\.scratch/|\b(tickets?|issues?)\s+(as|in)\s+(local\s+)?(markdown\s+)?(files?|markdown)\b|\blocal\s+(markdown\s+)?(issue|ticket)\s+tracker|\bfile[- ]based\s+(issue\s+)?tracker`)
	setupFiles   = regexp.MustCompile(`(?i)\b(create|write|add|generate|scaffold)\b[^\n.]{0,60}\b(CONTEXT\.md|AGENTS\.md|CLAUDE\.md|docs/agents)`)
)

// Notes returned, in a stable order, for the text of a skill and its directory.
const (
	NoteWritesGitHub = "Mentions creating or changing GitHub issues or pull requests. A launch never grants that; the agent still needs your say-so."
	NoteFileTracker  = "Appears to keep issues or tickets in local files. GitHub issues are the tracker here, so this skill may not fit this workflow."
	NoteSetupFiles   = "Asks to create setup files in the repository. Nothing is written for you; check it before using it."
	NoteScripts      = "Ships scripts. This app never runs them; the agent might, if you allow it."
)

// maxInspectEntries bounds the look at a skill's directory.
const maxInspectEntries = 200

// inspect says what is worth knowing about a skill before using it.
func inspect(dir, text string) []string {
	notes := []string{} // a list in JSON even when empty
	if writesGitHub.MatchString(text) {
		notes = append(notes, NoteWritesGitHub)
	}
	if fileTracker.MatchString(text) {
		notes = append(notes, NoteFileTracker)
	}
	if setupFiles.MatchString(text) {
		notes = append(notes, NoteSetupFiles)
	}
	if hasScripts(dir) {
		notes = append(notes, NoteScripts)
	}
	return notes
}

// hasScripts reports an executable file, or any file under a scripts folder,
// within two levels of the skill's directory. Symlinks are not followed.
func hasScripts(dir string) bool {
	seen := 0
	found := false
	var walk func(d string, depth int)
	walk = func(d string, depth int) {
		ents, err := os.ReadDir(d)
		if err != nil {
			return
		}
		for _, e := range ents {
			seen++
			if found || seen > maxInspectEntries {
				return
			}
			if e.Type()&fs.ModeSymlink != 0 || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			path := filepath.Join(d, e.Name())
			if e.IsDir() {
				if depth < 2 {
					if strings.EqualFold(e.Name(), "scripts") && hasAnyFile(path) {
						found = true
						return
					}
					walk(path, depth+1)
				}
				continue
			}
			if info, err := e.Info(); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 && !strings.EqualFold(e.Name(), SkillFile) {
				found = true
				return
			}
		}
	}
	walk(dir, 1)
	return found
}

func hasAnyFile(dir string) bool {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range ents {
		if !e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			return true
		}
	}
	return false
}
