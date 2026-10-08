// Package launcher turns a hero into the command line and environment that
// start Claude Code. It is the one place that knows Claude's CLI shape.
package launcher

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/dkage/dungeon-master/internal/party"
)

// Opts are the per-launch inputs that are not part of a hero.
type Opts struct {
	// Name labels the session inside Claude (--name). Optional.
	Name string
	// AppendSystemPrompt is added to Claude's system prompt. Optional.
	AppendSystemPrompt string
	// AddDirs grants Claude access to extra directories. Optional.
	AddDirs []string
	// Opener is the first message, submitted on start. A leading "/" runs a
	// user-invocable skill. Optional: empty starts Claude idle.
	Opener string
}

// Command builds argv for a hero. binary is the resolved claude path.
//
// Order matters: the hero's own args first, then the launcher's flags, then
// "--" and the opener. The separator keeps variadic flags such as --add-dir
// from swallowing the opener and keeps an opener that starts with "-" from
// being read as a flag.
func Command(binary string, h party.Hero, o Opts) []string {
	argv := []string{binary}
	argv = append(argv, h.Args...)
	if o.Name != "" {
		argv = append(argv, "--name", o.Name)
	}
	if o.AppendSystemPrompt != "" {
		argv = append(argv, "--append-system-prompt", o.AppendSystemPrompt)
	}
	for _, d := range o.AddDirs {
		argv = append(argv, "--add-dir", d)
	}
	if o.Opener != "" {
		argv = append(argv, "--", o.Opener)
	}
	return argv
}

// parentSessionMarkers are set by a running Claude Code process for its
// children. Inherited by a new Claude they make it think it is a nested
// session (for example it stops saving transcripts), so they are removed.
var parentSessionMarkers = map[string]bool{
	"CLAUDECODE":                   true,
	"CLAUDE_CODE_CHILD_SESSION":    true,
	"CLAUDE_CODE_SESSION_ID":       true,
	"CLAUDE_CODE_SESSION_ATTENDED": true,
	"CLAUDE_CODE_ENTRYPOINT":       true,
	"CLAUDE_CODE_EXECPATH":         true,
	"CLAUDE_CODE_SSE_PORT":         true,
	"CLAUDE_CODE_MESSAGING_SOCKET": true,
	"CLAUDE_CODE_MESSAGING_TOKEN":  true,
	"CLAUDE_PID":                   true,
}

// Environ builds a child environment: base without parent-session markers,
// TERM set for a capable terminal, then the hero's entries (which win).
func Environ(base []string, h party.Hero) []string {
	out := make([]string, 0, len(base)+len(h.Env)+2)
	for _, kv := range base {
		k, _, _ := strings.Cut(kv, "=")
		if parentSessionMarkers[k] || k == "TERM" || k == "COLORTERM" {
			continue
		}
		out = append(out, kv)
	}
	out = append(out, "TERM=xterm-256color", "COLORTERM=truecolor")
	return append(out, h.Env...)
}

// ShellEnviron is Environ for a plain shell (no hero).
func ShellEnviron(base []string) []string {
	return Environ(base, party.Hero{})
}

// Resolve finds the claude binary. A path containing a separator is checked
// directly. A bare name is searched on PATH, then on the PATH a login shell
// would have, because a GUI app launched from Finder lacks the user's shell
// PATH (nvm, Homebrew and asdf installs would otherwise be invisible).
func Resolve(ctx context.Context, name string) (string, error) {
	return resolve(ctx, name, os.Getenv("PATH"), os.Getenv("SHELL"))
}

func resolve(ctx context.Context, name, pathEnv, shell string) (string, error) {
	if name == "" {
		return "", errors.New("launcher: empty binary name")
	}
	if strings.ContainsRune(name, filepath.Separator) {
		return checkExecutable(name)
	}
	if p, ok := lookIn(name, pathEnv); ok {
		return p, nil
	}
	if shell != "" {
		if lp, err := loginPath(ctx, shell); err == nil {
			if p, ok := lookIn(name, lp); ok {
				return p, nil
			}
		}
	}
	return "", fmt.Errorf("launcher: %q not found on PATH or in the login shell PATH", name)
}

func lookIn(name, pathList string) (string, bool) {
	for _, dir := range filepath.SplitList(pathList) {
		if dir == "" {
			continue
		}
		if p, err := checkExecutable(filepath.Join(dir, name)); err == nil {
			return p, true
		}
	}
	return "", false
}

func checkExecutable(p string) (string, error) {
	fi, err := os.Stat(p)
	if err != nil {
		return "", err
	}
	if fi.IsDir() || fi.Mode()&0o111 == 0 {
		return "", fmt.Errorf("launcher: %s is not an executable file", p)
	}
	return p, nil
}

// loginPath asks an interactive login shell for its PATH, with a short
// timeout so a slow rc file cannot hang startup.
func loginPath(ctx context.Context, shell string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, shell, "-ilc", `printf '%s' "$PATH"`)
	// An rc file may leave background children holding the pipe open.
	cmd.WaitDelay = time.Second
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	// Interactive shells may print banners; the PATH is the last line.
	s := strings.TrimSpace(string(out))
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		s = s[i+1:]
	}
	return s, nil
}
