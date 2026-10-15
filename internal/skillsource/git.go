package skillsource

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// GitFetcher fetches with the git binary, run with direct argv and no
// prompts. It checks the pin out detached, so no hook or script from the
// repository runs, and removes the .git directory afterwards.
type GitFetcher struct{}

func (GitFetcher) Fetch(ctx context.Context, location, pin, dest string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	env := gitEnv(os.Environ(), location, configuredSSH)
	run := func(dir string, args ...string) (string, error) {
		cmd := exec.CommandContext(ctx, "git", append([]string{
			"-c", "protocol.ext.allow=never", "-c", "core.hooksPath=" + os.DevNull,
		}, args...)...)
		cmd.Dir = dir
		cmd.Env = env
		var out, errb bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errb
		if err := cmd.Run(); err != nil {
			msg := strings.TrimSpace(errb.String())
			if msg == "" {
				msg = err.Error()
			}
			return "", fmt.Errorf("git %s: %s", args[0], firstLine(msg))
		}
		return strings.TrimSpace(out.String()), nil
	}
	if pin == "" {
		pin = "HEAD"
	}
	if err := os.MkdirAll(dest, 0o700); err != nil {
		return "", err
	}
	if _, err := run(dest, "init", "--quiet"); err != nil {
		return "", err
	}
	if _, err := run(dest, "fetch", "--quiet", "--depth", "1", "--no-tags", "--", location, pin); err != nil {
		return "", err
	}
	if _, err := run(dest, "-c", "advice.detachedHead=false", "checkout", "--quiet", "--detach", "FETCH_HEAD"); err != nil {
		return "", err
	}
	rev, err := run(dest, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	if err := os.RemoveAll(dest + string(os.PathSeparator) + ".git"); err != nil {
		return "", err
	}
	return rev, nil
}

// configuredSSH reads the ssh command the user set in their Git config, so
// batch mode is added to it rather than replacing it.
func configuredSSH() string {
	out, err := exec.Command("git", "config", "--get", "core.sshCommand").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// scpStyle matches git@host:path remotes, which Git sends over ssh.
var scpStyle = regexp.MustCompile(`^[A-Za-z0-9._-]+@[A-Za-z0-9.-]+:`)

func usesSSH(location string) bool {
	return strings.HasPrefix(location, "ssh://") || scpStyle.MatchString(location)
}

// gitEnv is the environment of every git command the fetcher runs. Nothing
// may ask a question, because nobody is there to answer: no terminal prompt,
// no askpass helper and, over ssh, no passphrase, password or host-key
// question (batch mode makes ssh fail instead). The ssh command the user
// already uses (GIT_SSH_COMMAND, GIT_SSH or core.sshCommand) is kept, with
// batch mode added. configured reads core.sshCommand and is only called for
// ssh remotes.
func gitEnv(environ []string, location string, configured func() string) []string {
	env := append([]string(nil), environ...)
	env = append(env, "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=true", "SSH_ASKPASS=true", "SSH_ASKPASS_REQUIRE=never")
	if !usesSSH(location) {
		return env
	}
	get := func(key string) string {
		for i := len(environ) - 1; i >= 0; i-- {
			if v, ok := strings.CutPrefix(environ[i], key+"="); ok {
				return v
			}
		}
		return ""
	}
	base := get("GIT_SSH_COMMAND")
	if base == "" {
		if prog := get("GIT_SSH"); prog != "" {
			base = "'" + strings.ReplaceAll(prog, "'", `'\''`) + "'"
		}
	}
	if base == "" {
		base = configured()
	}
	if base == "" {
		base = "ssh"
	}
	return append(env, "GIT_SSH_COMMAND="+base+" -o BatchMode=yes")
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
