// Package skillcatalog finds the skills inside registered sources and builds
// the catalog a campaign sees. It only reads: nothing is executed, copied
// into a repository or installed into an agent. Every bound (depth, entries,
// file size) is a constant here, and a path that resolves outside its source
// is skipped.
package skillcatalog

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// Bounds on one scan.
const (
	MaxDepth       = 3 // directories below the source root that may hold a skill
	MaxEntries     = 20000
	MaxSkills      = 500
	MaxFileBytes   = 256 << 10
	MaxDescription = 500
)

// SkillFile is the file that makes a directory a skill.
const SkillFile = "SKILL.md"

var namePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// Skill is one discovered skill, before precedence is applied.
type Skill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	// Path is the skill's directory relative to its source root, slash separated.
	Path string `json:"path"`
	// Dir and File are absolute on this machine. They are what a briefing
	// hands the agent so it can read the skill's own templates and examples.
	Dir  string `json:"dir"`
	File string `json:"file"`
	// UserInvocable is false when the skill's metadata says only the model
	// may call it; it stays listed, with that reason.
	UserInvocable bool `json:"userInvocable"`
}

// Result is what one scan found.
type Result struct {
	Skills   []Skill  `json:"skills"`
	Warnings []string `json:"warnings"`
}

// Scan walks root for skills. Directories are visited in name order, so the
// result is stable. A directory holding SKILL.md is a skill and its contents
// are supporting files, not searched further. A missing or unreadable root
// gives an empty result with a warning.
func Scan(root string) Result {
	res := Result{Skills: []Skill{}, Warnings: []string{}}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		res.Warnings = append(res.Warnings, "the source folder cannot be read")
		return res
	}
	entries := 0
	seen := map[string]string{} // lowercase name -> path, to warn on duplicates inside one source
	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		if len(res.Skills) >= MaxSkills {
			return
		}
		if sk, ok, warn := readSkill(dir, realRoot); ok {
			rel, _ := filepath.Rel(realRoot, dir)
			sk.Path = filepath.ToSlash(rel)
			if sk.Path == "." {
				sk.Path = ""
			}
			if first, dup := seen[strings.ToLower(sk.Name)]; dup {
				res.Warnings = append(res.Warnings, "duplicate name "+sk.Name+" in "+sk.Path+"; keeping "+first)
				return
			}
			seen[strings.ToLower(sk.Name)] = sk.Path
			res.Skills = append(res.Skills, sk)
			return
		} else if warn != "" {
			res.Warnings = append(res.Warnings, warn)
			return
		}
		if depth >= MaxDepth {
			return
		}
		ents, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		sort.Slice(ents, func(i, j int) bool { return ents[i].Name() < ents[j].Name() })
		for _, e := range ents {
			entries++
			if entries > MaxEntries {
				return
			}
			name := e.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" {
				continue
			}
			child := filepath.Join(dir, name)
			if !e.IsDir() && e.Type()&fs.ModeSymlink == 0 {
				continue
			}
			real, err := filepath.EvalSymlinks(child)
			if err != nil || !within(realRoot, real) {
				if e.Type()&fs.ModeSymlink != 0 {
					res.Warnings = append(res.Warnings, "skipped "+name+": it points outside the source")
				}
				continue
			}
			if fi, err := os.Stat(real); err != nil || !fi.IsDir() {
				continue
			}
			walk(real, depth+1)
		}
	}
	walk(realRoot, 0)
	if entries > MaxEntries {
		res.Warnings = append(res.Warnings, "the source has too many entries; the scan stopped early")
	}
	if len(res.Skills) >= MaxSkills {
		res.Warnings = append(res.Warnings, "the source has too many skills; the list was cut")
	}
	return res
}

// readSkill reads dir/SKILL.md. ok is true for a usable skill; warn is set
// for a SKILL.md that exists but cannot be used.
func readSkill(dir, realRoot string) (sk Skill, ok bool, warn string) {
	file := filepath.Join(dir, SkillFile)
	fi, err := os.Lstat(file)
	if err != nil {
		return Skill{}, false, ""
	}
	label := filepath.Base(dir)
	real, err := filepath.EvalSymlinks(file)
	if err != nil || !within(realRoot, real) {
		return Skill{}, false, "skipped " + label + ": SKILL.md points outside the source"
	}
	if fi, err = os.Stat(real); err != nil || !fi.Mode().IsRegular() {
		return Skill{}, false, "skipped " + label + ": SKILL.md is not a regular file"
	}
	if fi.Size() > MaxFileBytes {
		return Skill{}, false, "skipped " + label + ": SKILL.md is too large"
	}
	f, err := os.Open(real)
	if err != nil {
		return Skill{}, false, "skipped " + label + ": SKILL.md cannot be read"
	}
	defer f.Close()
	buf := make([]byte, MaxFileBytes)
	n, _ := f.Read(buf)
	meta := frontMatter(buf[:n])
	name := meta["name"]
	if name == "" {
		name = filepath.Base(dir)
	}
	if !namePattern.MatchString(name) {
		return Skill{}, false, "skipped " + label + ": its name is not a plain identifier"
	}
	return Skill{
		Name:          name,
		Description:   clean(meta["description"]),
		Dir:           dir,
		File:          file,
		UserInvocable: meta["user-invocable"] != "false",
	}, true, ""
}

// frontMatter reads the flat key: value lines between the leading "---"
// lines. It handles quoted values and folded (>) or literal (|) blocks, and
// ignores anything nested; a file with no front matter has none.
func frontMatter(b []byte) map[string]string {
	out := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 0, 64<<10), MaxFileBytes)
	if !sc.Scan() || strings.TrimSpace(sc.Text()) != "---" {
		return out
	}
	var key string
	var block []string
	flush := func() {
		if key != "" && block != nil {
			out[key] = strings.Join(strings.Fields(strings.Join(block, " ")), " ")
		}
		key, block = "", nil
	}
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "---" {
			break
		}
		if block != nil && (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") || strings.TrimSpace(line) == "") {
			block = append(block, strings.TrimSpace(line))
			continue
		}
		flush()
		k, v, found := strings.Cut(line, ":")
		k = strings.TrimSpace(k)
		if !found || k == "" || strings.ContainsAny(k, " \t") {
			continue
		}
		v = strings.TrimSpace(v)
		if v == ">" || v == "|" || v == ">-" || v == "|-" {
			key, block = k, []string{}
			continue
		}
		out[k] = unquote(v)
	}
	flush()
	return out
}

func unquote(v string) string {
	if len(v) >= 2 && (v[0] == '"' && v[len(v)-1] == '"' || v[0] == '\'' && v[len(v)-1] == '\'') {
		return v[1 : len(v)-1]
	}
	return v
}

// clean makes a description safe to show: single line, no control
// characters, bounded.
func clean(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > MaxDescription {
		s = string(r[:MaxDescription]) + "…"
	}
	return s
}

func within(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// Read returns a skill's instructions (its front matter removed) and the
// SHA-256 of the whole file, so a launch can tell later whether the text it
// reviewed has changed. The file must still resolve inside root.
func Read(root, file string) (body, sum string, err error) {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", "", err
	}
	real, err := filepath.EvalSymlinks(file)
	if err != nil || !within(realRoot, real) {
		return "", "", fs.ErrNotExist
	}
	fi, err := os.Stat(real)
	if err != nil || !fi.Mode().IsRegular() || fi.Size() > MaxFileBytes {
		return "", "", fs.ErrNotExist
	}
	data, err := os.ReadFile(real)
	if err != nil {
		return "", "", err
	}
	h := sha256.Sum256(data)
	return stripFrontMatter(string(data)), hex.EncodeToString(h[:]), nil
}

func stripFrontMatter(s string) string {
	s = strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(s)
	if !strings.HasPrefix(s, "---\n") {
		return s
	}
	rest := s[len("---\n"):]
	i := strings.Index("\n"+rest, "\n---\n")
	if i < 0 {
		return s
	}
	return strings.TrimLeft(rest[min(len(rest), i+len("---\n")):], "\n")
}
