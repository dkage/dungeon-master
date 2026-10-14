// Package history remembers sessions across restarts. Pseudo-terminals do not
// survive a restart, so what is kept is a record: which hero ran where, when,
// and how it ended. Records of sessions that were still running when Dungeon Master
// stopped come back marked as lost.
package history

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/dkage/dungeon-master/internal/fsutil"
)

// MaxRecords bounds the file. When exceeded, the oldest finished records go.
const MaxRecords = 200

// Record is one session's metadata.
type Record struct {
	ID       string     `json:"id"`
	Campaign string     `json:"campaign"`
	Kind     string     `json:"kind"` // "hero" or "shell"
	Hero     string     `json:"hero,omitempty"`
	Quest    int        `json:"quest,omitempty"`
	Title    string     `json:"title"`
	Dir      string     `json:"dir"`
	Created  time.Time  `json:"created"`
	Ended    *time.Time `json:"ended,omitempty"`
	ExitCode int        `json:"exitCode"`
	// Lost marks a session that was still running when Dungeon Master stopped.
	Lost bool `json:"lost,omitempty"`
	// Stopped marks a session the user closed while it was running; its
	// outcome is "stopped by the user", not a completed run. Hidden records
	// stay in the history but are no longer listed as sessions.
	Stopped bool `json:"stopped,omitempty"`
	// Copy, Branch and CopyStart describe the git worktree an issue session
	// ran in; empty for a session that worked in the main tree.
	Copy      string `json:"copy,omitempty"`
	Branch    string `json:"branch,omitempty"`
	CopyStart string `json:"copyStart,omitempty"`
	Hidden    bool   `json:"hidden,omitempty"`
}

type file struct {
	Version int      `json:"version"`
	Records []Record `json:"records"`
}

// Store is the persisted record list.
type Store struct {
	path string
	now  func() time.Time

	mu   sync.Mutex
	recs []Record
}

// Open loads records from path (empty path: memory only). Records that never
// ended are marked lost.
func Open(path string) (*Store, error) {
	s := &Store{path: path, now: func() time.Time { return time.Now().UTC() }}
	if path == "" {
		return s, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	var f file
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	s.recs = f.Records
	dirty := false
	for i := range s.recs {
		if s.recs[i].Ended == nil {
			t := s.now()
			s.recs[i].Ended, s.recs[i].ExitCode, s.recs[i].Lost = &t, -1, true
			dirty = true
		}
	}
	if dirty {
		if err := s.saveLocked(); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// List returns records oldest first.
func (s *Store) List() []Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Record(nil), s.recs...)
}

// Get returns one record.
func (s *Store) Get(id string) (Record, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.recs {
		if r.ID == id {
			return r, true
		}
	}
	return Record{}, false
}

// Add appends a record for a newly started session.
func (s *Store) Add(r Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recs = append(s.recs, r)
	s.trimLocked()
	return s.saveLocked()
}

// Finish marks a record as ended. Unknown ids are ignored: the session may
// have been removed already.
func (s *Store) Finish(id string, code int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.recs {
		if s.recs[i].ID == id {
			t := s.now()
			s.recs[i].Ended, s.recs[i].ExitCode = &t, code
			return s.saveLocked()
		}
	}
	return nil
}

// MarkStopped records that the user stopped a running session by closing it,
// and hides it from the session list. It reports whether the record existed.
func (s *Store) MarkStopped(id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.recs {
		if s.recs[i].ID == id {
			if s.recs[i].Ended == nil {
				t := s.now()
				s.recs[i].Ended = &t
			}
			s.recs[i].Stopped, s.recs[i].Hidden = true, true
			return true, s.saveLocked()
		}
	}
	return false, nil
}

// Remove deletes one record and reports whether it existed.
func (s *Store) Remove(id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.recs {
		if s.recs[i].ID == id {
			s.recs = append(s.recs[:i], s.recs[i+1:]...)
			return true, s.saveLocked()
		}
	}
	return false, nil
}

// RemoveCampaign deletes every record of a campaign.
func (s *Store) RemoveCampaign(campaign string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.recs[:0]
	for _, r := range s.recs {
		if r.Campaign != campaign {
			kept = append(kept, r)
		}
	}
	s.recs = kept
	return s.saveLocked()
}

func (s *Store) trimLocked() {
	for len(s.recs) > MaxRecords {
		dropped := false
		for i, r := range s.recs {
			if r.Ended != nil {
				s.recs = append(s.recs[:i], s.recs[i+1:]...)
				dropped = true
				break
			}
		}
		if !dropped {
			return // everything left is still running
		}
	}
}

func (s *Store) saveLocked() error {
	if s.path == "" {
		return nil
	}
	data, err := json.MarshalIndent(file{Version: 1, Records: s.recs}, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(s.path, data)
}
