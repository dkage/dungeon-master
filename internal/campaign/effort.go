package campaign

import (
	"maps"
	"strconv"
	"strings"
	"unicode"
)

// maxEffortLabels bounds the label mapping; a real team has a handful.
const maxEffortLabels = 100

// copyMap returns a copy that is never nil, so callers can neither change the
// registry through it nor trip over a nil map, and it encodes as {}.
func copyMap[K comparable, V any](m map[K]V) map[K]V {
	out := make(map[K]V, len(m))
	maps.Copy(out, m)
	return out
}

// SetEffortLabels replaces the campaign's label-to-tier mapping. The caller
// checks that each value is a known tier; labels are trimmed and compared
// without regard to case.
func (r *Registry) SetEffortLabels(id string, mapping map[string]string) (Campaign, error) {
	if len(mapping) > maxEffortLabels {
		return Campaign{}, ValidationError("too many effort labels (100 at most)")
	}
	clean := make(map[string]string, len(mapping))
	for label, tier := range mapping {
		l := strings.ToLower(strings.TrimSpace(label))
		if l == "" || len([]rune(l)) > 50 || strings.IndexFunc(l, unicode.IsControl) >= 0 {
			return Campaign{}, ValidationError("effort labels must be 1 to 50 printable characters")
		}
		if _, dup := clean[l]; dup {
			return Campaign{}, ValidationError("effort labels must differ by more than case or spacing: " + strconv.Quote(l))
		}
		clean[l] = tier
	}
	return r.change(id, func(rec *record) bool {
		rec.EffortLabels = clean
		if len(clean) == 0 {
			rec.EffortLabels = nil
		}
		return true
	})
}

// SetEffortOverride sets one issue's tier locally; an empty tier removes the
// override so the labels decide again. The caller checks the tier.
func (r *Registry) SetEffortOverride(id string, number int, tier string) (Campaign, error) {
	if number <= 0 {
		return Campaign{}, ValidationError("issue number must be positive")
	}
	return r.change(id, func(rec *record) bool {
		if tier == "" {
			if _, ok := rec.EffortOverrides[number]; !ok {
				return false
			}
			delete(rec.EffortOverrides, number)
			if len(rec.EffortOverrides) == 0 {
				rec.EffortOverrides = nil
			}
			return true
		}
		if rec.EffortOverrides == nil {
			rec.EffortOverrides = map[int]string{}
		}
		rec.EffortOverrides[number] = tier
		return true
	})
}

// RememberCreatures stores creature choices for issues that have none yet.
// An issue keeps its first creature, so existing choices are never replaced.
func (r *Registry) RememberCreatures(id string, picks map[int]string) error {
	_, err := r.change(id, func(rec *record) bool {
		changed := false
		for n, c := range picks {
			if _, ok := rec.Creatures[n]; ok || c == "" {
				continue
			}
			if rec.Creatures == nil {
				rec.Creatures = map[int]string{}
			}
			rec.Creatures[n] = c
			changed = true
		}
		return changed
	})
	return err
}

// change applies fn to a copy of the campaign's record and saves it when fn
// reports a change, restoring the old record if the save fails.
func (r *Registry) change(id string, fn func(rec *record) bool) (Campaign, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, cur := range r.records {
		if cur.ID != id {
			continue
		}
		rec := cur
		rec.EffortLabels = copyMap(cur.EffortLabels)
		rec.EffortOverrides = copyMap(cur.EffortOverrides)
		rec.Creatures = copyMap(cur.Creatures)
		if !fn(&rec) {
			return r.view(cur), nil
		}
		r.records[i] = rec
		if err := r.saveLocked(); err != nil {
			r.records[i] = cur
			return Campaign{}, err
		}
		return r.view(rec), nil
	}
	return Campaign{}, ErrNotFound
}
