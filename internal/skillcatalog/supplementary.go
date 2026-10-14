package skillcatalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Discovery says which supplementary places to look in. Each is off unless
// the user turns it on: registered sources are the primary model, and these
// only add skills Claude may already find on its own.
type Discovery struct {
	Project bool `json:"project"`
	User    bool `json:"user"`
	Plugins bool `json:"plugins"`
}

// maxPlugins bounds how many installed plugins are looked at.
const maxPlugins = 50

var idUnsafe = regexp.MustCompile(`[^a-z0-9]+`)

// Supplementary returns the extra sources found for a project. projectDirs
// are the project's directory and its Git root; home is the user's home
// directory, empty when user and plugin discovery are unavailable. Only
// directories that exist are returned. Where a skill was found is separate
// from whether Claude can invoke it natively: this only says where.
func Supplementary(d Discovery, projectDirs []string, home string) []Source {
	var out []Source
	add := func(id, name, origin, root string) {
		if fi, err := os.Stat(root); err == nil && fi.IsDir() {
			out = append(out, Source{ID: id, Name: name, Origin: origin, Root: root, Enabled: true})
		}
	}
	if d.Project {
		seen := map[string]bool{}
		for i, dir := range projectDirs {
			clean := filepath.Clean(dir)
			if dir == "" || seen[clean] {
				continue
			}
			seen[clean] = true
			suffix := ""
			if i > 0 {
				suffix = "-root"
			}
			add("project-claude"+suffix, "Project skills (.claude/skills)", OriginProject, filepath.Join(clean, ".claude", "skills"))
			add("project-agents"+suffix, "Project skills (.agents/skills)", OriginProject, filepath.Join(clean, ".agents", "skills"))
		}
	}
	if home == "" {
		return out
	}
	if d.User {
		add("user-claude", "Your skills (~/.claude/skills)", OriginUser, filepath.Join(home, ".claude", "skills"))
		add("user-agents", "Your skills (~/.agents/skills)", OriginUser, filepath.Join(home, ".agents", "skills"))
	}
	if d.Plugins {
		out = append(out, pluginSources(home)...)
	}
	return out
}

// pluginSources reads Claude's list of installed plugins and returns each
// plugin's skills folder. Install paths must stay inside the plugins folder.
func pluginSources(home string) []Source {
	base := filepath.Join(home, ".claude", "plugins")
	data, err := os.ReadFile(filepath.Join(base, "installed_plugins.json"))
	if err != nil || len(data) > 1<<20 {
		return nil
	}
	var file struct {
		Plugins map[string]json.RawMessage `json:"plugins"`
	}
	if json.Unmarshal(data, &file) != nil {
		return nil
	}
	realBase, err := filepath.EvalSymlinks(base)
	if err != nil {
		return nil
	}
	var out []Source
	used := map[string]bool{}
	for key, raw := range file.Plugins {
		if len(out) >= maxPlugins {
			break
		}
		var installs []struct {
			InstallPath string `json:"installPath"`
		}
		if json.Unmarshal(raw, &installs) != nil {
			var one struct {
				InstallPath string `json:"installPath"`
			}
			if json.Unmarshal(raw, &one) != nil {
				continue
			}
			installs = append(installs, one)
		}
		for _, in := range installs {
			if in.InstallPath == "" {
				continue
			}
			skills, err := filepath.EvalSymlinks(filepath.Join(in.InstallPath, "skills"))
			if err != nil || !within(realBase, skills) {
				continue
			}
			if fi, err := os.Stat(skills); err != nil || !fi.IsDir() {
				continue
			}
			name, _, _ := strings.Cut(key, "@")
			id := "plugin-" + strings.Trim(idUnsafe.ReplaceAllString(strings.ToLower(name), "-"), "-")
			if id == "plugin-" || used[id] {
				continue
			}
			used[id] = true
			out = append(out, Source{ID: id, Name: "Plugin: " + name, Origin: OriginPlugin, Root: skills, Enabled: true})
			break // one install per plugin
		}
	}
	sortByID(out)
	return out
}

func sortByID(s []Source) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j].ID < s[j-1].ID; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
