package gatekeeper

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

// Environment variables the app sets in the sessions it starts.
const (
	EnvGateURL  = "DUNGEON_GATE_URL"   // "unix:<socket path>", where the app answers slot requests
	EnvGateTok  = "DUNGEON_GATE_TOKEN" // a token valid for this session only
	EnvRealGH   = "DUNGEON_REAL_GH"    // the real gh, which the shim runs
	EnvGateBin  = "DUNGEON_GATE_BIN"   // the directory holding the shim, removed from gh's own PATH
	exitHeld    = 75                   // EX_TEMPFAIL: asked to try again later
	exitMissing = 127                  // the real gh could not be found
)

// Paths of the two requests the shim makes, on the gate socket.
const (
	SlotPath   = "/gatekeeper/slot"
	ReportPath = "/gatekeeper/report"
)

// ShimOptions is everything Run needs; zero values use the real process.
type ShimOptions struct {
	Args    []string // the arguments after "gh"
	Getenv  func(string) string
	Environ []string
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
	HTTP    *http.Client
	// SlotTimeout bounds the wait for a write slot. Default 150 s, a little
	// longer than the longest the app itself will make a request wait.
	SlotTimeout time.Duration
}

// IsShim reports whether the program was started under the name "gh".
func IsShim(argv0 string) bool {
	base := strings.ToLower(filepath.Base(argv0))
	return base == "gh" || base == "gh.exe"
}

var rateLimitText = regexp.MustCompile(`(?i)secondary rate limit|rate limit exceeded|abuse detection|too many requests|API rate limit`)
var retryAfterText = regexp.MustCompile(`(?i)retry-after:?\s*(\d+)`)

// Run is the whole shim. Read-only commands go straight to the real gh.
// Commands that write first ask the app for a slot, so that writes are paced
// and stop while GitHub has asked us to slow down. If the app cannot be
// reached the command runs unpaced: the shim must never be the reason work
// fails. It returns the exit code to use.
func Run(ctx context.Context, o ShimOptions) int {
	if o.Getenv == nil {
		o.Getenv = os.Getenv
	}
	if o.Environ == nil {
		o.Environ = os.Environ()
	}
	if o.Stdin == nil {
		o.Stdin = os.Stdin
	}
	if o.Stdout == nil {
		o.Stdout = os.Stdout
	}
	if o.Stderr == nil {
		o.Stderr = os.Stderr
	}
	if o.SlotTimeout <= 0 {
		o.SlotTimeout = 150 * time.Second
	}
	if o.HTTP == nil {
		o.HTTP = &http.Client{}
	}

	real, err := realGH(o)
	if err != nil {
		fmt.Fprintln(o.Stderr, "gh: command not found")
		return exitMissing
	}
	env := childEnv(o)

	if Classify(o.Args) == Read {
		return runChild(real, o.Args, env, o.Stdin, o.Stdout, o.Stderr)
	}

	gate, tok := o.Getenv(EnvGateURL), o.Getenv(EnvGateTok)
	if gate == "" || tok == "" {
		return runChild(real, o.Args, env, o.Stdin, o.Stdout, o.Stderr) // not in an app session
	}

	switch held, waited := askForSlot(ctx, o, gate, tok); {
	case held != "":
		fmt.Fprintln(o.Stderr, held)
		return exitHeld
	case waited >= time.Second:
		fmt.Fprintf(o.Stderr, "dungeon: waited %s for a GitHub write slot (limits are paced on purpose)\n", waited.Round(time.Second))
	}

	var seen limitWatcher
	code := runChild(real, o.Args, env, o.Stdin, o.Stdout, io.MultiWriter(o.Stderr, &seen))
	if code != 0 && seen.hit {
		report(ctx, o, gate, tok, seen.line, seen.retryAfter)
	}
	return code
}

type slotAnswer struct {
	Error    string `json:"error"`
	Code     string `json:"code"`
	WaitedMS int64  `json:"waitedMs"`
}

// askForSlot returns a message when the command must not run now, or how long
// it waited when it may. Any failure to ask means "go ahead".
func askForSlot(ctx context.Context, o ShimOptions, gate, tok string) (held string, waited time.Duration) {
	ctx, cancel := context.WithTimeout(ctx, o.SlotTimeout)
	defer cancel()
	body, _ := json.Marshal(map[string]string{"command": summary(o.Args)})
	client, base := Dialer(gate, o.HTTP)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+SlotPath, bytes.NewReader(body))
	if err != nil {
		return "", 0
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", 0 // the app is not there: fail open
	}
	defer resp.Body.Close()
	var ans slotAnswer
	_ = json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&ans)
	switch resp.StatusCode {
	case http.StatusOK:
		return "", time.Duration(ans.WaitedMS) * time.Millisecond
	case http.StatusTooManyRequests:
		msg := ans.Error
		if msg == "" {
			msg = "GitHub writes are paused for now"
		}
		return "dungeon: " + msg + " Try again later; do not retry in a loop.", 0
	}
	return "", 0 // unexpected answer (revoked token, old app): fail open
}

// summary is the command's first two words, for logs: never its arguments,
// which may hold issue text.
func summary(args []string) string {
	var w []string
	for i := 0; i < len(args) && len(w) < 2; i++ {
		if strings.HasPrefix(args[i], "-") {
			if valueFlags[args[i]] {
				i++
			}
			continue
		}
		w = append(w, args[i])
	}
	return strings.Join(w, " ")
}

func report(ctx context.Context, o ShimOptions, gate, tok, line string, retryAfter int) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	body, _ := json.Marshal(map[string]any{"message": line, "retryAfter": retryAfter})
	client, base := Dialer(gate, o.HTTP)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+ReportPath, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	if resp, err := client.Do(req); err == nil {
		resp.Body.Close()
	}
}

// limitWatcher notices GitHub's rate-limit message in gh's error output while
// letting it through untouched.
type limitWatcher struct {
	hit        bool
	line       string
	retryAfter int
	buf        []byte
}

func (w *limitWatcher) Write(p []byte) (int, error) {
	if len(w.buf) < 16<<10 { // only the start of the output matters
		w.buf = append(w.buf, p...)
		if rateLimitText.Match(w.buf) {
			w.hit = true
			text := string(w.buf)
			if loc := rateLimitText.FindStringIndex(text); loc != nil {
				start := strings.LastIndexByte(text[:loc[0]], '\n') + 1
				end := strings.IndexByte(text[loc[0]:], '\n')
				if end < 0 {
					end = len(text) - loc[0]
				}
				w.line = strings.TrimSpace(text[start : loc[0]+end])
			}
			if m := retryAfterText.FindStringSubmatch(text); m != nil {
				fmt.Sscanf(m[1], "%d", &w.retryAfter)
			}
		}
	}
	return len(p), nil
}

// realGH finds the gh to run: the one the app resolved, else the first gh on
// the PATH that is not this program.
func realGH(o ShimOptions) (string, error) {
	if p := o.Getenv(EnvRealGH); p != "" {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p, nil
		}
	}
	self, _ := os.Executable()
	selfReal, _ := filepath.EvalSymlinks(self)
	for _, dir := range filepath.SplitList(o.Getenv("PATH")) {
		cand := filepath.Join(dir, "gh")
		fi, err := os.Stat(cand)
		if err != nil || fi.IsDir() || fi.Mode()&0o111 == 0 {
			continue
		}
		if r, _ := filepath.EvalSymlinks(cand); r != "" && r == selfReal {
			continue
		}
		return cand, nil
	}
	return "", errors.New("gh not found")
}

// childEnv is the environment for the real gh: the shim's directory is taken
// off PATH so gh (or git calling gh as a credential helper) cannot loop back
// into the shim.
func childEnv(o ShimOptions) []string {
	bin := o.Getenv(EnvGateBin)
	out := make([]string, 0, len(o.Environ))
	for _, kv := range o.Environ {
		if bin != "" && strings.HasPrefix(kv, "PATH=") {
			var keep []string
			for _, d := range filepath.SplitList(strings.TrimPrefix(kv, "PATH=")) {
				if filepath.Clean(d) != filepath.Clean(bin) {
					keep = append(keep, d)
				}
			}
			kv = "PATH=" + strings.Join(keep, string(os.PathListSeparator))
		}
		out = append(out, kv)
	}
	return out
}

// runChild runs the real gh with the given streams and returns its exit code.
func runChild(path string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) int {
	cmd := exec.Command(path, args...)
	cmd.Env = env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, stdout, stderr
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(stderr, "gh: %v\n", err)
		return exitMissing
	}
	// Pass interrupts on to gh, as running it directly would.
	sig := make(chan os.Signal, 4)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sig)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case s := <-sig:
				_ = cmd.Process.Signal(s)
			case <-done:
				return
			}
		}
	}()
	err := cmd.Wait()
	close(done)
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if code := ee.ExitCode(); code >= 0 {
			return code
		}
		return 1 // killed by a signal
	}
	return exitMissing
}
