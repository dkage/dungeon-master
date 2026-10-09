package party

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sample = `
claude = "/opt/bin/claude"

[heroes.opus]
label = "Opus"
args  = ["--model", "claude-opus-5-5"]
env   = ["CLAUDE_CONFIG_DIR=~/.claude-work"]

[heroes.quick]
args = ["--model", "claude-haiku-5-5", "--permission-mode", "acceptEdits"]
`

func TestParse(t *testing.T) {
	p, err := Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	if p.Claude != "/opt/bin/claude" || p.Defaulted {
		t.Fatalf("claude=%q defaulted=%v", p.Claude, p.Defaulted)
	}
	if len(p.Heroes) != 2 || p.Heroes[0].ID != "opus" || p.Heroes[1].ID != "quick" {
		t.Fatalf("heroes: %+v", p.Heroes)
	}
	if p.Heroes[1].Label != "quick" {
		t.Fatalf("label should default to id, got %q", p.Heroes[1].Label)
	}
	home, _ := os.UserHomeDir()
	if got, want := p.Heroes[0].Env[0], "CLAUDE_CONFIG_DIR="+filepath.Join(home, ".claude-work"); got != want {
		t.Fatalf("env tilde: got %q want %q", got, want)
	}
	if h, ok := p.Hero("quick"); !ok || len(h.Args) != 4 {
		t.Fatalf("Hero lookup: %+v %v", h, ok)
	}
	if _, ok := p.Hero("nope"); ok {
		t.Fatal("unknown hero found")
	}
}

func TestParseDefaultsBinary(t *testing.T) {
	p, err := Parse([]byte("[heroes.a]\nargs=[]\n"))
	if err != nil || p.Claude != "claude" {
		t.Fatalf("claude=%q err=%v", p.Claude, err)
	}
}

func TestEncodeRoundTripsAndPreservesDefaultHero(t *testing.T) {
	p := Party{
		Claude:      "/opt/claude",
		DefaultHero: "quick",
		Heroes: []Hero{
			{ID: "opus", Label: "Opus", Args: []string{"--model", "claude-opus-5-5"}},
			{ID: "quick", Label: "Quick", Args: []string{"--model", "claude-haiku-5-5"}, Env: []string{"MODE=fast"}},
		},
	}
	data, err := Encode(p)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if got.Claude != p.Claude || got.DefaultHero != p.DefaultHero || len(got.Heroes) != 2 || got.Heroes[1].Env[0] != "MODE=fast" {
		t.Fatalf("round trip: %+v", got)
	}
}

func TestParseRejects(t *testing.T) {
	cases := map[string]string{
		"no heroes":       "claude = \"claude\"\n",
		"bad toml":        "[heroes.a\n",
		"unknown key":     "[heroes.a]\nargs=[]\nmodle=\"x\"\n",
		"unknown top key": "colour=\"x\"\n[heroes.a]\nargs=[]\n",
		"uppercase id":    "[heroes.Opus]\nargs=[]\n",
		"id with space":   "[heroes.\"my hero\"]\nargs=[]\n",
		"id too long":     "[heroes." + strings.Repeat("a", 33) + "]\nargs=[]\n",
		"newline in arg":  "[heroes.a]\nargs=[\"--x\\n--y\"]\n",
		"nul in arg":      "[heroes.a]\nargs=[\"a\\u0000b\"]\n",
		"env not kv":      "[heroes.a]\nenv=[\"NOEQUALS\"]\n",
		"env empty key":   "[heroes.a]\nenv=[\"=v\"]\n",
		"env newline":     "[heroes.a]\nenv=[\"K=a\\nb\"]\n",
		"claude newline":  "claude=\"a\\nb\"\n[heroes.a]\nargs=[]\n",
		"label newline":   "[heroes.a]\nlabel=\"name\\nforged\"\nargs=[]\n",
		"args wrong type": "[heroes.a]\nargs=\"--model x\"\n",
	}
	for name, src := range cases {
		if _, err := Parse([]byte(src)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestShellMetacharactersAreHarmlessData(t *testing.T) {
	p, err := Parse([]byte("[heroes.a]\nargs=[\"--x\", \"$(rm -rf ~); `id` | cat\"]\n"))
	if err != nil {
		t.Fatalf("metacharacters rejected although argv is never shell-parsed: %v", err)
	}
	if p.Heroes[0].Args[1] != "$(rm -rf ~); `id` | cat" {
		t.Fatal("argument was altered")
	}
}

func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	// Missing file: defaults.
	p, err := LoadFile(filepath.Join(dir, FileName))
	if err != nil || !p.Defaulted || len(p.Heroes) != 3 {
		t.Fatalf("missing file: %+v %v", p, err)
	}
	// Present file.
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte(sample), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err = LoadFile(path)
	if err != nil || p.Defaulted || len(p.Heroes) != 2 {
		t.Fatalf("present file: %+v %v", p, err)
	}
	// Invalid file: error names the path.
	if err := os.WriteFile(path, []byte("[heroes.A]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFile(path); err == nil || !strings.Contains(err.Error(), FileName) {
		t.Fatalf("invalid file error: %v", err)
	}
}

func TestConfigDirOverride(t *testing.T) {
	t.Setenv("DUNGEON_HOME", "/tmp/dungeon-test-home")
	d, err := ConfigDir()
	if err != nil || d != "/tmp/dungeon-test-home" {
		t.Fatalf("got %q %v", d, err)
	}
}

func TestDefaultsAreValid(t *testing.T) {
	d := Defaults()
	if d.Claude == "" || len(d.Heroes) == 0 {
		t.Fatal("empty defaults")
	}
	for _, h := range d.Heroes {
		if !idPattern.MatchString(h.ID) {
			t.Errorf("default id %q invalid", h.ID)
		}
	}
}
