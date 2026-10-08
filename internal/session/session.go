// Package session runs processes in pseudo-terminals and lets any number of
// viewers attach to them. It knows nothing about heroes, quests or HTTP.
package session

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	pty "github.com/aymanbagabas/go-pty"
)

const (
	// ScrollbackBytes is how much recent output a session keeps for replay.
	ScrollbackBytes = 1 << 20
	subBuffer       = 256
	killGrace       = 3 * time.Second
)

// Spec describes what to run.
type Spec struct {
	// Argv is the program followed by its arguments. Run directly, never
	// through a shell.
	Argv []string
	Dir  string
	// Env is the full environment of the child.
	Env        []string
	Cols, Rows int
	// Meta is caller-owned labelling (hero, kind, ...), echoed back on the
	// session. The package never interprets it.
	Meta map[string]string
}

// Session is one running (or finished) process attached to a PTY.
type Session struct {
	ID      string
	Created time.Time
	Dir     string
	Meta    map[string]string

	p   pty.Pty
	cmd *pty.Cmd

	mu       sync.Mutex
	scroll   *ring
	subs     map[*Subscription]struct{}
	exited   bool
	exitCode int

	done chan struct{}
}

// Subscription is one viewer's live feed. Output arrives on C; C closes when
// the session ends or the viewer fell too far behind (reattach to resync).
type Subscription struct {
	C      <-chan []byte
	c      chan []byte
	closed bool
}

func start(id string, spec Spec) (*Session, error) {
	if len(spec.Argv) == 0 {
		return nil, errors.New("session: empty argv")
	}
	p, err := pty.New()
	if err != nil {
		return nil, fmt.Errorf("session: open pty: %w", err)
	}
	if spec.Cols > 0 && spec.Rows > 0 {
		_ = p.Resize(spec.Cols, spec.Rows)
	}
	cmd := p.Command(spec.Argv[0], spec.Argv[1:]...)
	cmd.Dir = spec.Dir
	cmd.Env = spec.Env
	if err := cmd.Start(); err != nil {
		_ = p.Close()
		return nil, fmt.Errorf("session: start %s: %w", spec.Argv[0], err)
	}
	// The child has its own copy of the slave end. Keeping ours open means
	// that on Linux reading the master never reports end-of-file after the
	// child exits, so the session would never finish. macOS reports it
	// anyway, which is why this only shows up on Linux.
	if up, ok := p.(pty.UnixPty); ok {
		_ = up.Slave().Close()
	}
	s := &Session{
		ID:      id,
		Created: time.Now(),
		Dir:     spec.Dir,
		Meta:    spec.Meta,
		p:       p,
		cmd:     cmd,
		scroll:  newRing(ScrollbackBytes),
		subs:    map[*Subscription]struct{}{},
		done:    make(chan struct{}),
	}
	go s.pump()
	return s, nil
}

// pump copies PTY output into the scrollback and to subscribers until the
// process exits.
func (s *Session) pump() {
	buf := make([]byte, 32*1024)
	for {
		n, err := s.p.Read(buf)
		if n > 0 {
			chunk := append([]byte(nil), buf[:n]...)
			s.mu.Lock()
			s.scroll.write(chunk)
			for sub := range s.subs {
				select {
				case sub.c <- chunk:
				default:
					// Too slow: drop the viewer rather than corrupt its
					// terminal with a gap. It can reattach and replay.
					s.dropLocked(sub)
				}
			}
			s.mu.Unlock()
		}
		if err != nil {
			break
		}
	}

	werr := s.cmd.Wait()
	code := 0
	if werr != nil {
		code = -1
	}
	if st := s.cmd.ProcessState; st != nil {
		code = st.ExitCode()
	}
	s.mu.Lock()
	s.exited, s.exitCode = true, code
	for sub := range s.subs {
		s.dropLocked(sub)
	}
	s.mu.Unlock()
	_ = s.p.Close()
	close(s.done)
}

func (s *Session) dropLocked(sub *Subscription) {
	if !sub.closed {
		sub.closed = true
		close(sub.c)
	}
	delete(s.subs, sub)
}

// Attach returns the retained scrollback and a live subscription that
// continues exactly where the replay ends.
func (s *Session) Attach() (replay []byte, sub *Subscription) {
	s.mu.Lock()
	defer s.mu.Unlock()
	replay = s.scroll.snapshot()
	c := make(chan []byte, subBuffer)
	sub = &Subscription{C: c, c: c}
	if s.exited {
		sub.closed = true
		close(c)
		return replay, sub
	}
	s.subs[sub] = struct{}{}
	return replay, sub
}

// Detach stops a subscription early.
func (s *Session) Detach(sub *Subscription) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dropLocked(sub)
}

// Write sends keystrokes to the process.
func (s *Session) Write(p []byte) (int, error) {
	select {
	case <-s.done:
		return 0, io.ErrClosedPipe
	default:
	}
	return s.p.Write(p)
}

// Resize changes the terminal size.
func (s *Session) Resize(cols, rows int) error {
	if cols <= 0 || rows <= 0 {
		return errors.New("session: bad size")
	}
	return s.p.Resize(cols, rows)
}

// Done is closed once the process has exited and output is drained.
func (s *Session) Done() <-chan struct{} { return s.done }

// Exited reports whether the process ended, and its exit code.
func (s *Session) Exited() (bool, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.exited, s.exitCode
}

// Kill asks the whole process group to stop, then forces it after a grace
// period. Safe to call more than once.
func (s *Session) Kill() {
	select {
	case <-s.done:
		return
	default:
	}
	signalGroup(s.cmd.Process, false)
	select {
	case <-s.done:
	case <-time.After(killGrace):
		signalGroup(s.cmd.Process, true)
		<-s.done
	}
}

// Manager owns a set of sessions.
type Manager struct {
	mu       sync.Mutex
	sessions map[string]*Session
	order    []string
}

// newID returns a short random id, unique among live sessions. Random rather
// than sequential so ids stay unique across restarts, when old records are
// reloaded from disk.
func (m *Manager) newID() (string, error) {
	for range 8 {
		b := make([]byte, 4)
		if _, err := rand.Read(b); err != nil {
			return "", fmt.Errorf("session: id: %w", err)
		}
		id := "s" + hex.EncodeToString(b)
		m.mu.Lock()
		_, taken := m.sessions[id]
		m.mu.Unlock()
		if !taken {
			return id, nil
		}
	}
	return "", errors.New("session: could not allocate an id")
}

// NewManager returns an empty manager.
func NewManager() *Manager {
	return &Manager{sessions: map[string]*Session{}}
}

// Spawn starts a process and registers it.
func (m *Manager) Spawn(spec Spec) (*Session, error) {
	id, err := m.newID()
	if err != nil {
		return nil, err
	}
	s, err := start(id, spec)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	m.sessions[id] = s
	m.order = append(m.order, id)
	m.mu.Unlock()
	return s, nil
}

// Get returns a session by id.
func (m *Manager) Get(id string) (*Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	return s, ok
}

// List returns sessions in creation order.
func (m *Manager) List() []*Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Session, 0, len(m.order))
	for _, id := range m.order {
		out = append(out, m.sessions[id])
	}
	return out
}

// Remove kills a session if it is running and forgets it.
func (m *Manager) Remove(id string) bool {
	m.mu.Lock()
	s, ok := m.sessions[id]
	if ok {
		delete(m.sessions, id)
		for i, v := range m.order {
			if v == id {
				m.order = append(m.order[:i], m.order[i+1:]...)
				break
			}
		}
	}
	m.mu.Unlock()
	if ok {
		s.Kill()
	}
	return ok
}

// Shutdown kills every session.
func (m *Manager) Shutdown() {
	for _, s := range m.List() {
		s.Kill()
	}
}

// Environ returns the parent environment with TERM set for a capable
// terminal, plus extra KEY=VALUE entries (later entries win).
func Environ(extra ...string) []string {
	env := append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor")
	return append(env, extra...)
}
