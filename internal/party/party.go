// Package party loads the user's heroes: named Claude configurations (model,
// flags, environment) that become launch buttons. The file is local to the
// user and never read from a repository.
package party

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// FileName is the party file inside the config directory.
const FileName = "party.toml"

var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)

// Hero is one launchable Claude configuration.
type Hero struct {
	ID    string   `json:"id"`
	Label string   `json:"label"`
	Args  []string `json:"-"`
	// Env holds KEY=VALUE entries applied on top of the parent environment.
	Env []string `json:"-"`
}

// Party is the validated set of heroes.
type Party struct {
	// Claude is the binary to run: a name looked up on PATH or an absolute path.
	Claude string
	Heroes []Hero
	// Defaulted is true when no file existed and built-in templates are used.
	Defaulted bool
}

type rawFile struct {
	Claude string             `toml:"claude"`
	Heroes map[string]rawHero `toml:"heroes"`
}

type rawHero struct {
	Label string   `toml:"label"`
	Args  []string `toml:"args"`
	Env   []string `toml:"env"`
}

// Defaults are the starter heroes offered before the user writes a file.
func Defaults() Party {
	return Party{
		Claude:    "claude",
		Defaulted: true,
		Heroes: []Hero{
			{ID: "opus", Label: "Opus", Args: []string{"--model", "claude-opus-5-5"}},
			{ID: "sonnet", Label: "Sonnet", Args: []string{"--model", "claude-sonnet-5-5"}},
			{ID: "haiku", Label: "Haiku", Args: []string{"--model", "claude-haiku-5-5"}},
		},
	}
}

// ConfigDir returns the directory holding Dungeon Master's local config. The
// DUNGEON_HOME environment variable overrides it.
func ConfigDir() (string, error) {
	if d := os.Getenv("DUNGEON_HOME"); d != "" {
		return d, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "dungeon"), nil
}

// LoadFile reads and validates a party file. A missing file yields Defaults.
func LoadFile(path string) (Party, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Defaults(), nil
	}
	if err != nil {
		return Party{}, err
	}
	p, err := Parse(data)
	if err != nil {
		return Party{}, fmt.Errorf("%s: %w", path, err)
	}
	return p, nil
}

// Parse validates party.toml contents.
func Parse(data []byte) (Party, error) {
	var raw rawFile
	md, err := toml.Decode(string(data), &raw)
	if err != nil {
		return Party{}, err
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		return Party{}, fmt.Errorf("unknown key %q", undecoded[0].String())
	}
	p := Party{Claude: strings.TrimSpace(raw.Claude)}
	if p.Claude == "" {
		p.Claude = "claude"
	}
	if strings.ContainsAny(p.Claude, "\x00\r\n") {
		return Party{}, errors.New("claude: invalid path")
	}
	if len(raw.Heroes) == 0 {
		return Party{}, errors.New("no heroes defined")
	}
	ids := make([]string, 0, len(raw.Heroes))
	for id := range raw.Heroes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	home, _ := os.UserHomeDir()
	for _, id := range ids {
		h := raw.Heroes[id]
		if !idPattern.MatchString(id) {
			return Party{}, fmt.Errorf("hero %q: id must match %s", id, idPattern)
		}
		for _, a := range h.Args {
			if err := checkArg(a); err != nil {
				return Party{}, fmt.Errorf("hero %q: args: %w", id, err)
			}
		}
		env := make([]string, 0, len(h.Env))
		for _, e := range h.Env {
			e, err := checkEnv(e, home)
			if err != nil {
				return Party{}, fmt.Errorf("hero %q: env: %w", id, err)
			}
			env = append(env, e)
		}
		label := strings.TrimSpace(h.Label)
		if label == "" {
			label = id
		}
		p.Heroes = append(p.Heroes, Hero{ID: id, Label: label, Args: h.Args, Env: env})
	}
	return p, nil
}

// Hero returns a hero by id.
func (p Party) Hero(id string) (Hero, bool) {
	for _, h := range p.Heroes {
		if h.ID == id {
			return h, true
		}
	}
	return Hero{}, false
}

// checkArg rejects values that cannot be a sane single argv entry. Arguments
// are executed directly, never through a shell, so metacharacters are fine;
// control characters and NUL are not.
func checkArg(a string) error {
	if strings.ContainsAny(a, "\x00\r\n") {
		return fmt.Errorf("%q contains a control character", a)
	}
	return nil
}

func checkEnv(e, home string) (string, error) {
	k, v, ok := strings.Cut(e, "=")
	if !ok || k == "" || strings.ContainsAny(k, " \t\x00") {
		return "", fmt.Errorf("%q is not KEY=VALUE", e)
	}
	if strings.ContainsAny(v, "\x00\r\n") {
		return "", fmt.Errorf("%q contains a control character", k)
	}
	if home != "" && (v == "~" || strings.HasPrefix(v, "~/")) {
		v = filepath.Join(home, strings.TrimPrefix(v, "~"))
	}
	return k + "=" + v, nil
}
