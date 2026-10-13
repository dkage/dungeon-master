// Package campaign keeps the user's registered working directories. A
// campaign is a directory (a repository root or a subfolder of one); its
// GitHub issues come from the repository's remote. The registry is local to
// the user and never written into a repository.
package campaign

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/dkage/dungeon-master/internal/fsutil"
	"github.com/dkage/dungeon-master/internal/platform"
)

var (
	// ErrNotFound means no campaign has the given id.
	ErrNotFound = errors.New("campaign: not found")
	// ErrDuplicate means the directory is already registered.
	ErrDuplicate = errors.New("campaign: directory already registered")
)

// ValidationError is a problem with user input, safe to show to the user.
type ValidationError string

func (e ValidationError) Error() string { return string(e) }

// Campaign is a registered directory plus what was learned from git.
type Campaign struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Dir    string `json:"dir"`
	Remote string `json:"remote"`
	// Scope says which of the repository's issues the panel shows. Nil until
	// the user has chosen one (the first-connection question).
	Scope *Scope `json:"scope"`
	// Demo shows built-in sample quests instead of GitHub issues.
	Demo bool `json:"demo"`
	// Sample marks the built-in, throwaway onboarding campaign.
	Sample bool `json:"sample,omitempty"`
	// ParallelQuests allows a second quest session while one already runs in
	// the same Git working tree. Off by default: agents share one checkout.
	ParallelQuests bool `json:"parallelQuests"`
	// Derived from git on registration and update; empty when the directory
	// is not in a repository or the remote is not on github.com.
	GitRoot string `json:"gitRoot,omitempty"`
	Repo    string `json:"repo,omitempty"` // "owner/repo"
	// EffortLabels maps the team's own labels (for example "size/S") to an
	// effort tier, for repositories that do not use the default labels.
	EffortLabels map[string]string `json:"effortLabels"`
	// EffortOverrides sets an issue's effort tier locally, by issue number.
	EffortOverrides map[int]string `json:"effortOverrides"`
}

// Scope modes.
const (
	// ScopeLabels shows issues carrying any of the chosen labels.
	ScopeLabels = "labels"
	// ScopeAssigned shows open issues assigned to the signed-in GitHub user.
	ScopeAssigned = "assigned"
	// ScopeAll shows every issue, paged and capped.
	ScopeAll = "all"
)

// MaxScopeLabels bounds the labels of a scope: each one is a separate query.
const MaxScopeLabels = 5

// Scope is which issues a campaign shows.
type Scope struct {
	Mode   string   `json:"mode"`
	Labels []string `json:"labels"`
}

// Input registers a campaign.
type Input struct {
	Dir    string
	Name   string
	Remote string
	Demo   bool
	Sample bool
}

// Patch changes a campaign; nil fields are left alone.
type Patch struct {
	Name   *string
	Remote *string
	Scope  *Scope
	Demo   *bool
	// ParallelQuests lets several quest sessions run in one working tree.
	ParallelQuests *bool
}

type record struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Dir    string `json:"dir"`
	Remote string `json:"remote"`
	// Labels is the old campaign-level label filter, read once and turned into
	// a scope on load; never written again.
	Labels []string  `json:"labels,omitempty"`
	Scope  *Scope    `json:"scope,omitempty"`
	Demo   bool      `json:"demo,omitempty"`
	Sample bool      `json:"sample,omitempty"`
	Added  time.Time `json:"added"`

	ParallelQuests bool `json:"parallelQuests,omitempty"`

	EffortLabels    map[string]string `json:"effortLabels,omitempty"`
	EffortOverrides map[int]string    `json:"effortOverrides,omitempty"`
}

type file struct {
	Version   int      `json:"version"`
	Campaigns []record `json:"campaigns"`
}

// Registry is the persisted campaign list.
type Registry struct {
	path string
	git  Git

	mu      sync.Mutex
	records []record
	derived map[string]derived
}

type derived struct{ gitRoot, repo string }

// Open loads the registry from path. An empty path keeps everything in
// memory, which is what tests use. A nil git means ExecGit.
func Open(ctx context.Context, path string, git Git) (*Registry, error) {
	if git == nil {
		git = ExecGit{}
	}
	r := &Registry{path: path, git: git, derived: map[string]derived{}}
	if path == "" {
		return r, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return r, nil
	}
	if err != nil {
		return nil, err
	}
	var f file
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	r.records = f.Campaigns
	for i := range r.records {
		migrate(&r.records[i])
		r.derived[r.records[i].ID] = r.resolve(ctx, r.records[i])
	}
	return r, nil
}

// migrate turns the old campaign-level label filter into a scope. Nothing is
// written until the next change, and then only the scope.
func migrate(rec *record) {
	if rec.Scope == nil && len(rec.Labels) > 0 {
		if sc, err := ValidateScope(Scope{Mode: ScopeLabels, Labels: rec.Labels}); err == nil {
			rec.Scope = &sc
		}
	}
	rec.Labels = nil
}

// List returns campaigns in registration order.
func (r *Registry) List() []Campaign {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Campaign, 0, len(r.records))
	for _, rec := range r.records {
		out = append(out, r.view(rec))
	}
	return out
}

// Get returns one campaign.
func (r *Registry) Get(id string) (Campaign, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, rec := range r.records {
		if rec.ID == id {
			return r.view(rec), true
		}
	}
	return Campaign{}, false
}

// Add registers a directory.
func (r *Registry) Add(ctx context.Context, in Input) (Campaign, error) {
	dir, err := cleanDir(in.Dir)
	if err != nil {
		return Campaign{}, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = filepath.Base(dir)
	}
	if err := checkName(name); err != nil {
		return Campaign{}, err
	}
	remote, err := checkRemote(in.Remote)
	if err != nil {
		return Campaign{}, err
	}
	id, err := newID()
	if err != nil {
		return Campaign{}, err
	}
	rec := record{ID: id, Name: name, Dir: dir, Remote: remote, Demo: in.Demo, Sample: in.Sample, Added: time.Now().UTC()}
	d := r.resolve(ctx, rec)

	r.mu.Lock()
	defer r.mu.Unlock()
	for _, other := range r.records {
		if sameDir(other.Dir, dir) {
			return Campaign{}, ErrDuplicate
		}
	}
	r.records = append(r.records, rec)
	r.derived[id] = d
	if err := r.saveLocked(); err != nil {
		r.records = r.records[:len(r.records)-1]
		delete(r.derived, id)
		return Campaign{}, err
	}
	return r.view(rec), nil
}

// Update changes name, remote, scope or the demo flag, re-reading git when the remote
// changes.
func (r *Registry) Update(ctx context.Context, id string, p Patch) (Campaign, error) {
	r.mu.Lock()
	idx := -1
	for i, rec := range r.records {
		if rec.ID == id {
			idx = i
		}
	}
	if idx < 0 {
		r.mu.Unlock()
		return Campaign{}, ErrNotFound
	}
	rec := r.records[idx]
	r.mu.Unlock()

	if p.Name != nil {
		n := strings.TrimSpace(*p.Name)
		if n == "" {
			n = filepath.Base(rec.Dir)
		}
		if err := checkName(n); err != nil {
			return Campaign{}, err
		}
		rec.Name = n
	}
	if p.Remote != nil {
		rm, err := checkRemote(*p.Remote)
		if err != nil {
			return Campaign{}, err
		}
		rec.Remote = rm
	}
	if p.Scope != nil {
		sc, err := ValidateScope(*p.Scope)
		if err != nil {
			return Campaign{}, err
		}
		rec.Scope = &sc
	}
	if p.Demo != nil {
		rec.Demo = *p.Demo
	}
	if p.ParallelQuests != nil {
		rec.ParallelQuests = *p.ParallelQuests
	}
	d := r.resolve(ctx, rec)

	r.mu.Lock()
	defer r.mu.Unlock()
	for i, cur := range r.records {
		if cur.ID == id {
			old, oldD := r.records[i], r.derived[id]
			r.records[i], r.derived[id] = rec, d
			if err := r.saveLocked(); err != nil {
				r.records[i], r.derived[id] = old, oldD
				return Campaign{}, err
			}
			return r.view(rec), nil
		}
	}
	return Campaign{}, ErrNotFound
}

// Remove forgets a campaign. It does not touch the directory.
func (r *Registry) Remove(id string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, rec := range r.records {
		if rec.ID == id {
			old := append([]record(nil), r.records...)
			r.records = append(r.records[:i], r.records[i+1:]...)
			delete(r.derived, id)
			if err := r.saveLocked(); err != nil {
				r.records = old
				r.derived[id] = r.resolve(context.Background(), rec)
				return false, err
			}
			return true, nil
		}
	}
	return false, nil
}

func (r *Registry) view(rec record) Campaign {
	d := r.derived[rec.ID]
	var scope *Scope
	if rec.Scope != nil {
		sc := *rec.Scope
		sc.Labels = append([]string{}, sc.Labels...)
		scope = &sc
	}
	return Campaign{
		ID: rec.ID, Name: rec.Name, Dir: rec.Dir, Remote: rec.Remote, Scope: scope, Demo: rec.Demo, Sample: rec.Sample, ParallelQuests: rec.ParallelQuests,
		GitRoot: d.gitRoot, Repo: d.repo,
		EffortLabels:    copyMap(rec.EffortLabels),
		EffortOverrides: copyMap(rec.EffortOverrides),
	}
}

// resolve asks git where the repository root is and which GitHub repository
// the remote points at. Failure is normal (no repo, no remote, other host)
// and just leaves the fields empty.
func (r *Registry) resolve(ctx context.Context, rec record) derived {
	root, err := r.git.Toplevel(ctx, rec.Dir)
	if err != nil || root == "" {
		return derived{}
	}
	d := derived{gitRoot: root}
	raw, err := r.git.RemoteURL(ctx, root, rec.Remote)
	if err != nil {
		return d
	}
	if owner, repo, err := ParseGitHubRemote(raw); err == nil {
		d.repo = owner + "/" + repo
	}
	return d
}

func (r *Registry) saveLocked() error {
	if r.path == "" {
		return nil
	}
	data, err := json.MarshalIndent(file{Version: 1, Campaigns: r.records}, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(r.path, data)
}

func cleanDir(dir string) (string, error) {
	if dir == "" || !filepath.IsAbs(dir) {
		return "", ValidationError("dir must be an absolute path")
	}
	dir = filepath.Clean(dir)
	fi, err := os.Stat(dir)
	if err != nil || !fi.IsDir() {
		return "", ValidationError("dir is not an existing directory")
	}
	return dir, nil
}

func sameDir(a, b string) bool {
	if a == b {
		return true
	}
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	if errA != nil || errB != nil {
		return false
	}
	ai, errA := os.Stat(ra)
	bi, errB := os.Stat(rb)
	return (platform.SamePath(ra, rb) || (errA == nil && errB == nil && os.SameFile(ai, bi)))
}

func checkName(n string) error {
	if len([]rune(n)) > 60 {
		return ValidationError("name is too long (60 characters at most)")
	}
	for _, c := range n {
		if unicode.IsControl(c) {
			return ValidationError("name contains a control character")
		}
	}
	return nil
}

var remotePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,50}$`)

func checkRemote(n string) (string, error) {
	n = strings.TrimSpace(n)
	if n == "" {
		return "origin", nil
	}
	if !remotePattern.MatchString(n) || strings.HasPrefix(n, "-") {
		return "", ValidationError("remote must be a plain remote name such as origin")
	}
	return n, nil
}

// ValidateScope checks a scope and returns it cleaned: labels trimmed and
// de-duplicated, and present only in labels mode.
func ValidateScope(sc Scope) (Scope, error) {
	switch sc.Mode {
	case ScopeLabels:
		labels, err := checkLabels(sc.Labels)
		if err != nil {
			return Scope{}, err
		}
		if len(labels) == 0 {
			return Scope{}, ValidationError("choose at least one label")
		}
		if len(labels) > MaxScopeLabels {
			return Scope{}, ValidationError(fmt.Sprintf("at most %d labels", MaxScopeLabels))
		}
		return Scope{Mode: ScopeLabels, Labels: labels}, nil
	case ScopeAssigned, ScopeAll:
		if len(sc.Labels) > 0 {
			return Scope{}, ValidationError("labels only apply when showing issues by label")
		}
		return Scope{Mode: sc.Mode, Labels: []string{}}, nil
	}
	return Scope{}, ValidationError("scope mode must be labels, assigned or all")
}

func checkLabels(in []string) ([]string, error) {
	if len(in) > 10 {
		return nil, ValidationError("at most 10 labels")
	}
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, l := range in {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if len([]rune(l)) > 50 {
			return nil, ValidationError("label is too long (50 characters at most)")
		}
		for _, c := range l {
			if unicode.IsControl(c) || c == ',' {
				return nil, ValidationError("labels cannot contain commas or control characters")
			}
		}
		if !seen[strings.ToLower(l)] {
			seen[strings.ToLower(l)] = true
			out = append(out, l)
		}
	}
	return out, nil
}

func newID() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "c" + hex.EncodeToString(b), nil
}
