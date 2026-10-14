// Package skillsource keeps the user's registered skill sources: local
// folders and pinned Git repositories, and which of them each campaign uses
// and in what order. Everything lives in one private state file in the user
// config dir; Git checkouts live under the app's own state directory. Nothing
// is written into a user's repository, and registering a source never runs
// anything from it.
package skillsource

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
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/dkage/dungeon-master/internal/fsutil"
)

var (
	// ErrNotFound means no source has the given id.
	ErrNotFound = errors.New("skillsource: not found")
	// ErrDuplicate means the location is already registered.
	ErrDuplicate = errors.New("skillsource: location already registered")
)

// ValidationError is a problem with user input, safe to show to the user.
type ValidationError string

func (e ValidationError) Error() string { return string(e) }

// Kind is where a source's files come from.
type Kind string

const (
	Folder Kind = "folder"
	Git    Kind = "git"
)

// Source is one registered source.
type Source struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Kind     Kind   `json:"kind"`
	Location string `json:"location"` // folder path, or Git remote
	// Pin is the ref (tag, branch or commit) the user asked for; Git only.
	Pin     string `json:"pin,omitempty"`
	Enabled bool   `json:"enabled"`
	// License and Notice keep the credit of an external source visible.
	License string `json:"license,omitempty"`
	Notice  string `json:"notice,omitempty"`

	// Verified is the last checkout that succeeded; Git only. A failed
	// refresh leaves it alone and records Error instead.
	Verified *Checkout `json:"verified,omitempty"`
	Error    *Failure  `json:"error,omitempty"`
}

// Checkout is a resolved, verified copy of a Git source.
type Checkout struct {
	Revision string    `json:"revision"` // the full commit id the pin resolved to
	At       time.Time `json:"at"`
	// Dir is where the files are; derived from the state dir, not persisted.
	Dir string `json:"-"`
}

// Failure is the last failed refresh.
type Failure struct {
	Message string    `json:"message"`
	At      time.Time `json:"at"`
}

// Mapping is one entry of a campaign's own source list.
type Mapping struct {
	SourceID string `json:"sourceId"`
	Enabled  bool   `json:"enabled"`
}

// Effective is a source as one campaign sees it: its place in the
// precedence order and whether it applies there.
type Effective struct {
	Source
	Enabled bool `json:"enabled"`
	// Override is true when the campaign lists the source itself rather than
	// inheriting the global default.
	Override bool `json:"override"`
}

// Fetcher gets a Git source's files at a pin into dest, which does not exist
// yet, and returns the full commit id. Tests replace it; the real one runs
// git with direct argv.
type Fetcher interface {
	Fetch(ctx context.Context, location, pin, dest string) (revision string, err error)
}

// Suggested is a well-known source the user may add explicitly. Offering one
// registers nothing and downloads nothing.
type Suggested struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Location string `json:"location"`
	License  string `json:"license"`
	Notice   string `json:"notice"`
}

// Suggestions are the opt-in sources. The pin is always the user's choice.
func Suggestions() []Suggested {
	return []Suggested{{
		Key:      "mattpocock",
		Name:     "Matt Pocock's skills",
		Location: "https://github.com/mattpocock/skills",
		License:  "MIT",
		Notice:   "Skills by Matt Pocock, MIT licensed. Fetched from the upstream repository at the revision you pin; nothing is bundled with this app.",
	}}
}

// Discovery turns on the optional supplementary places to look for skills:
// a project's own skill folders, the user's, and installed plugins. All are
// off until the user chooses; registered sources stay the primary model.
type Discovery struct {
	Project bool `json:"project"`
	User    bool `json:"user"`
	Plugins bool `json:"plugins"`
}

type file struct {
	Version   int       `json:"version"`
	Discovery Discovery `json:"discovery"`
	Sources   []Source  `json:"sources"`
	// Campaigns maps a campaign id to its own ordered source list.
	Campaigns map[string][]Mapping `json:"campaigns,omitempty"`
}

// Registry is the persisted set of sources. An empty path keeps it in memory.
type Registry struct {
	path    string
	root    string // where Git checkouts live
	fetcher Fetcher
	now     func() time.Time

	mu   sync.Mutex
	data file
}

// Options configure Open.
type Options struct {
	// Root is the directory for Git checkouts; Git sources are refused when empty.
	Root    string
	Fetcher Fetcher
	Now     func() time.Time
}

// Open loads the file at path. A missing file is an empty registry; a
// malformed one is an error, so a hand-edit is never silently discarded.
func Open(path string, opt Options) (*Registry, error) {
	r := &Registry{path: path, root: opt.Root, fetcher: opt.Fetcher, now: opt.Now}
	if r.now == nil {
		r.now = time.Now
	}
	if r.fetcher == nil {
		r.fetcher = GitFetcher{}
	}
	if path != "" {
		data, err := os.ReadFile(path)
		switch {
		case errors.Is(err, os.ErrNotExist):
		case err != nil:
			return nil, err
		default:
			if err := json.Unmarshal(data, &r.data); err != nil {
				return nil, fmt.Errorf("%s: %w", path, err)
			}
		}
	}
	return r, nil
}

// Input is a new source.
type Input struct {
	Name     string
	Kind     Kind
	Location string
	Pin      string
	License  string
	Notice   string
}

var (
	idPattern  = regexp.MustCompile(`^[a-f0-9]{16}$`)
	pinPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,127}$`)
	urlPattern = regexp.MustCompile(`^(https://|ssh://|git@)[A-Za-z0-9][^\s]{0,500}$`)
)

// Add registers a source. A folder must exist and be absolute; a Git source
// needs a pin and is fetched before it is stored, so a bad URL or ref is
// refused up front rather than registered broken.
func (r *Registry) Add(ctx context.Context, in Input) (Source, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" || len(name) > 80 || strings.ContainsAny(name, "\r\n\x00") {
		return Source{}, ValidationError("name must be 1-80 characters on one line")
	}
	src := Source{Name: name, Kind: in.Kind, Enabled: true, License: in.License, Notice: in.Notice}
	switch in.Kind {
	case Folder:
		loc, err := cleanFolder(in.Location)
		if err != nil {
			return Source{}, err
		}
		src.Location = loc
	case Git:
		if r.root == "" {
			return Source{}, ValidationError("Git sources are not available")
		}
		if err := checkRemote(in.Location); err != nil {
			return Source{}, err
		}
		if err := checkPin(in.Pin); err != nil {
			return Source{}, err
		}
		src.Location, src.Pin = in.Location, in.Pin // a blank pin follows the remote's default branch
	default:
		return Source{}, ValidationError(`kind must be "folder" or "git"`)
	}
	id, err := newID()
	if err != nil {
		return Source{}, err
	}
	src.ID = id

	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.data.Sources {
		if s.Kind == src.Kind && s.Location == src.Location && s.Pin == src.Pin {
			return Source{}, ErrDuplicate
		}
	}
	if src.Kind == Git {
		co, err := r.fetchLocked(ctx, src)
		if err != nil {
			return Source{}, ValidationError("could not fetch the source: " + err.Error())
		}
		src.Verified = &co
	}
	r.data.Sources = append(r.data.Sources, src)
	if err := r.saveLocked(); err != nil {
		r.data.Sources = r.data.Sources[:len(r.data.Sources)-1]
		return Source{}, err
	}
	return r.viewLocked(src), nil
}

// Patch changes a source; nil fields stay. Changing the pin of a Git source
// fetches the new revision first and keeps the old one if that fails.
type Patch struct {
	Name     *string
	Enabled  *bool
	Pin      *string
	Location *string
}

func (r *Registry) Update(ctx context.Context, id string, p Patch) (Source, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	i := r.indexLocked(id)
	if i < 0 {
		return Source{}, ErrNotFound
	}
	next := r.data.Sources[i]
	if p.Name != nil {
		n := strings.TrimSpace(*p.Name)
		if n == "" || len(n) > 80 || strings.ContainsAny(n, "\r\n\x00") {
			return Source{}, ValidationError("name must be 1-80 characters on one line")
		}
		next.Name = n
	}
	if p.Enabled != nil {
		next.Enabled = *p.Enabled
	}
	if p.Location != nil && *p.Location != next.Location {
		switch next.Kind {
		case Folder:
			loc, err := cleanFolder(*p.Location)
			if err != nil {
				return Source{}, err
			}
			next.Location = loc
		case Git:
			if err := checkRemote(*p.Location); err != nil {
				return Source{}, err
			}
			next.Location = *p.Location
		}
	}
	if p.Pin != nil && *p.Pin != next.Pin && next.Kind != Git {
		return Source{}, ValidationError("only a Git source has a pin")
	}
	if next.Kind == Git && ((p.Pin != nil && *p.Pin != next.Pin) || next.Location != r.data.Sources[i].Location) {
		if p.Pin != nil {
			if err := checkPin(*p.Pin); err != nil {
				return Source{}, err
			}
			next.Pin = *p.Pin
		}
		co, err := r.fetchLocked(ctx, next)
		if err != nil {
			return Source{}, ValidationError("could not fetch the source: " + err.Error())
		}
		next.Verified, next.Error = &co, nil
	}
	prev := r.data.Sources[i]
	r.data.Sources[i] = next
	if err := r.saveLocked(); err != nil {
		r.data.Sources[i] = prev
		return Source{}, err
	}
	return r.viewLocked(next), nil
}

// Refresh fetches a Git source again. On failure the previous verified
// checkout stays in place and the error is recorded with its time.
func (r *Registry) Refresh(ctx context.Context, id string) (Source, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	i := r.indexLocked(id)
	if i < 0 {
		return Source{}, ErrNotFound
	}
	src := r.data.Sources[i]
	if src.Kind != Git {
		return Source{}, ValidationError("only a Git source can be refreshed")
	}
	co, err := r.fetchLocked(ctx, src)
	if err != nil {
		src.Error = &Failure{Message: err.Error(), At: r.now().UTC()}
	} else {
		old := src.Verified
		src.Verified, src.Error = &co, nil
		if old != nil && old.Revision != co.Revision {
			_ = os.RemoveAll(r.checkoutDir(src.ID, old.Revision))
		}
	}
	r.data.Sources[i] = src
	if serr := r.saveLocked(); serr != nil {
		return Source{}, serr
	}
	return r.viewLocked(src), nil
}

// Remove forgets a source and its mappings. Files the user owns are never
// touched; only an app-owned checkout is deleted.
func (r *Registry) Remove(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	i := r.indexLocked(id)
	if i < 0 {
		return ErrNotFound
	}
	old := r.data
	r.data.Sources = slices.Delete(slices.Clone(r.data.Sources), i, i+1)
	camps := map[string][]Mapping{}
	for c, ms := range old.Campaigns {
		if kept := slices.DeleteFunc(slices.Clone(ms), func(m Mapping) bool { return m.SourceID == id }); len(kept) > 0 {
			camps[c] = kept
		}
	}
	r.data.Campaigns = camps
	if err := r.saveLocked(); err != nil {
		r.data = old
		return err
	}
	if old.Sources[i].Kind == Git && r.root != "" {
		_ = os.RemoveAll(filepath.Join(r.root, id))
	}
	return nil
}

// Reorder sets the global precedence. ids must be exactly the registered set.
func (r *Registry) Reorder(ids []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !sameSet(ids, sourceIDs(r.data.Sources)) {
		return ValidationError("the order must list every source exactly once")
	}
	old := r.data.Sources
	next := make([]Source, 0, len(old))
	for _, id := range ids {
		next = append(next, old[r.indexLocked(id)])
	}
	r.data.Sources = next
	if err := r.saveLocked(); err != nil {
		r.data.Sources = old
		return err
	}
	return nil
}

// Discovery returns which supplementary places are turned on.
func (r *Registry) Discovery() Discovery {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.data.Discovery
}

// SetDiscovery saves the supplementary choices.
func (r *Registry) SetDiscovery(d Discovery) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	old := r.data.Discovery
	r.data.Discovery = d
	if err := r.saveLocked(); err != nil {
		r.data.Discovery = old
		return err
	}
	return nil
}

// List returns the global sources in precedence order.
func (r *Registry) List() []Source {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Source, 0, len(r.data.Sources))
	for _, s := range r.data.Sources {
		out = append(out, r.viewLocked(s))
	}
	return out
}

// Get returns one source.
func (r *Registry) Get(id string) (Source, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if i := r.indexLocked(id); i >= 0 {
		return r.viewLocked(r.data.Sources[i]), true
	}
	return Source{}, false
}

// ForCampaign is the ordered list a campaign sees. A campaign with no list of
// its own inherits the global order and enabled flags. With one, its entries
// come first in its order with its flags, and sources it does not mention
// follow in global order with their global flag, so a newly registered source
// appears without rewriting every campaign.
func (r *Registry) ForCampaign(campaignID string) []Effective {
	r.mu.Lock()
	defer r.mu.Unlock()
	mine := r.data.Campaigns[campaignID]
	out := []Effective{} // never nil: it is sent as a JSON list
	seen := map[string]bool{}
	for _, m := range mine {
		i := r.indexLocked(m.SourceID)
		if i < 0 {
			continue
		}
		s := r.viewLocked(r.data.Sources[i])
		out = append(out, Effective{Source: s, Enabled: m.Enabled && s.Enabled, Override: true})
		seen[m.SourceID] = true
	}
	for _, s := range r.data.Sources {
		if !seen[s.ID] {
			v := r.viewLocked(s)
			out = append(out, Effective{Source: v, Enabled: s.Enabled})
		}
	}
	return out
}

// SetCampaign stores a campaign's own list. An empty list clears the
// override, returning the campaign to the inherited defaults.
func (r *Registry) SetCampaign(campaignID string, list []Mapping) error {
	if campaignID == "" {
		return ValidationError("campaign is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	seen := map[string]bool{}
	for _, m := range list {
		if r.indexLocked(m.SourceID) < 0 {
			return ValidationError("unknown source " + m.SourceID)
		}
		if seen[m.SourceID] {
			return ValidationError("a source may be listed once")
		}
		seen[m.SourceID] = true
	}
	old := r.data.Campaigns
	next := make(map[string][]Mapping, len(old)+1)
	for k, v := range old {
		next[k] = v
	}
	if len(list) == 0 {
		delete(next, campaignID)
	} else {
		next[campaignID] = slices.Clone(list)
	}
	r.data.Campaigns = next
	if err := r.saveLocked(); err != nil {
		r.data.Campaigns = old
		return err
	}
	return nil
}

// ForgetCampaign drops a removed campaign's mapping.
func (r *Registry) ForgetCampaign(campaignID string) error {
	r.mu.Lock()
	_, ok := r.data.Campaigns[campaignID]
	r.mu.Unlock()
	if !ok {
		return nil
	}
	return r.SetCampaign(campaignID, nil)
}

func (r *Registry) fetchLocked(ctx context.Context, src Source) (Checkout, error) {
	dest := filepath.Join(r.root, src.ID, ".incoming")
	if err := os.RemoveAll(dest); err != nil {
		return Checkout{}, err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return Checkout{}, err
	}
	rev, err := r.fetcher.Fetch(ctx, src.Location, src.Pin, dest)
	if err != nil {
		_ = os.RemoveAll(dest)
		return Checkout{}, err
	}
	if !revPattern.MatchString(rev) {
		_ = os.RemoveAll(dest)
		return Checkout{}, errors.New("the fetcher returned no commit id")
	}
	final := r.checkoutDir(src.ID, rev)
	if _, err := os.Stat(final); err == nil {
		_ = os.RemoveAll(dest) // the same revision is already here
	} else if err := os.Rename(dest, final); err != nil {
		_ = os.RemoveAll(dest)
		return Checkout{}, err
	}
	return Checkout{Revision: rev, At: r.now().UTC(), Dir: final}, nil
}

var revPattern = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)

func (r *Registry) checkoutDir(id, rev string) string { return filepath.Join(r.root, id, rev) }

// viewLocked fills in the derived checkout directory.
func (r *Registry) viewLocked(s Source) Source {
	if s.Verified != nil {
		v := *s.Verified
		v.Dir = r.checkoutDir(s.ID, v.Revision)
		s.Verified = &v
	}
	return s
}

func (r *Registry) indexLocked(id string) int {
	return slices.IndexFunc(r.data.Sources, func(s Source) bool { return s.ID == id })
}

func (r *Registry) saveLocked() error {
	if r.path == "" {
		return nil
	}
	r.data.Version = 1
	b, err := json.MarshalIndent(r.data, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(r.path, append(b, '\n'))
}

func sourceIDs(ss []Source) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = s.ID
	}
	return out
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	x, y := slices.Clone(a), slices.Clone(b)
	slices.Sort(x)
	slices.Sort(y)
	return slices.Equal(x, y)
}

func cleanFolder(dir string) (string, error) {
	if dir == "" || !filepath.IsAbs(dir) {
		return "", ValidationError("location must be an absolute folder path")
	}
	dir = filepath.Clean(dir)
	fi, err := os.Stat(dir)
	if err != nil || !fi.IsDir() {
		return "", ValidationError("location is not a folder")
	}
	return dir, nil
}

// checkRemote accepts https, ssh and scp-style remotes, or an absolute local
// path (a local clone). Nothing starting with "-", so it can never be read as
// an option; "ext::" and other transport helpers are not accepted.
func checkRemote(loc string) error {
	if urlPattern.MatchString(loc) || (filepath.IsAbs(loc) && !strings.ContainsAny(loc, "\r\n\x00")) {
		return nil
	}
	return ValidationError("location must be an https or ssh remote, or an absolute path")
}

// checkPin accepts a blank pin, meaning the remote's default branch; either
// way the exact commit it resolved to is recorded and shown.
func checkPin(pin string) error {
	if pin == "" {
		return nil
	}
	if !pinPattern.MatchString(pin) || strings.Contains(pin, "..") {
		return ValidationError("pin must be a tag, branch or commit id")
	}
	return nil
}

func newID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	id := hex.EncodeToString(b)
	if !idPattern.MatchString(id) {
		return "", errors.New("bad id")
	}
	return id, nil
}
