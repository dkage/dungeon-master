package quartermaster

import (
	"encoding/json"
	"errors"
	"os"
	"time"

	"github.com/dkage/dungeon-master/internal/fsutil"
)

// State is what survives a restart, so that restarting the app cannot be
// used (by accident) to get around a pause.
type State struct {
	CampUntil time.Time `json:"campUntil"`
	Reason    string    `json:"reason,omitempty"`
	Level     int       `json:"level"`
	LastTrip  time.Time `json:"lastTrip"`
}

// Store persists State.
type Store interface {
	Load() (State, error)
	Save(State) error
}

// MemStore keeps state in memory; for tests and for running without a config
// directory.
type MemStore struct{ S State }

func (m *MemStore) Load() (State, error) { return m.S, nil }
func (m *MemStore) Save(s State) error   { m.S = s; return nil }

// FileStore keeps state in a small private JSON file.
type FileStore struct{ Path string }

// Load returns the saved state. A missing or unreadable file is an empty
// state: losing the pause is better than refusing to start.
func (f FileStore) Load() (State, error) {
	b, err := os.ReadFile(f.Path)
	if errors.Is(err, os.ErrNotExist) {
		return State{}, nil
	}
	if err != nil {
		return State{}, err
	}
	var s State
	if err := json.Unmarshal(b, &s); err != nil {
		return State{}, nil
	}
	return s, nil
}

// Save writes the state atomically with owner-only permissions.
func (f FileStore) Save(s State) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(f.Path, b)
}
