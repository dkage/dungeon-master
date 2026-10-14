package skillcatalog

import "strings"

// Origins of a source.
const (
	OriginRegistered = "registered"
	OriginProject    = "project"
	OriginUser       = "user"
	OriginPlugin     = "plugin"
)

// Source is what the catalog needs to know about a registered source.
type Source struct {
	ID   string
	Name string
	// Origin says where the source came from; empty means registered.
	Origin   string
	Root     string // where its files are; empty when there is nothing to read yet
	Revision string // the pinned commit of a Git source; empty for a folder
	Enabled  bool
}

// Entry is a skill with its source and where it stands in the precedence.
type Entry struct {
	// ID is the source-qualified identity, "<source id>/<name>". It is what a
	// reviewed launch pins; a bare name is only ever a suggestion.
	ID         string `json:"id"`
	SourceID   string `json:"sourceId"`
	SourceName string `json:"sourceName"`
	// Origin is "registered", or where a supplementary source was found:
	// "project", "user" or "plugin". It says where, not whether Claude can
	// invoke the skill natively.
	Origin   string `json:"origin"`
	Revision string `json:"revision,omitempty"`
	Skill
	// Root is the source's directory; reads are confined to it.
	Root string `json:"-"`
	// Shadowed names the source whose same-named skill wins, when one does.
	Shadowed   bool   `json:"shadowed"`
	ShadowedBy string `json:"shadowedBy,omitempty"`
}

// SourceReport is the per-source outcome, shown next to its entry in Settings.
type SourceReport struct {
	SourceID string   `json:"sourceId"`
	Name     string   `json:"name"`
	Origin   string   `json:"origin"`
	Count    int      `json:"count"`
	Warnings []string `json:"warnings"`
	// Flagged lists skills with compatibility notes, bounded.
	Flagged []Flagged `json:"flagged"`
	// Unavailable says why nothing could be read, empty when the scan ran.
	Unavailable string `json:"unavailable,omitempty"`
}

// Flagged is a skill that has compatibility notes.
type Flagged struct {
	Name  string   `json:"name"`
	Notes []string `json:"notes"`
}

// MaxFlagged bounds the flagged list of one source.
const MaxFlagged = 50

// Catalog is every skill of the enabled sources, in precedence order, and a
// report for each source given, enabled or not.
type Catalog struct {
	Entries []Entry        `json:"entries"`
	Reports []SourceReport `json:"reports"`
}

// Build scans each source and merges the enabled ones. Sources come in
// precedence order. The first enabled source with a name wins a bare-name
// suggestion; the others stay in the list, marked shadowed and still
// addressable by their qualified ID. Disabled sources contribute no entries
// but are still reported, so Settings can show their counts.
func Build(sources []Source) Catalog {
	cat := Catalog{Entries: []Entry{}, Reports: []SourceReport{}}
	winner := map[string]string{} // lowercase name -> source name
	for _, src := range sources {
		origin := src.Origin
		if origin == "" {
			origin = OriginRegistered
		}
		rep := SourceReport{SourceID: src.ID, Name: src.Name, Origin: origin, Warnings: []string{}, Flagged: []Flagged{}}
		if src.Root == "" {
			rep.Unavailable = "nothing has been fetched for this source yet"
			cat.Reports = append(cat.Reports, rep)
			continue
		}
		res := Scan(src.Root)
		rep.Count, rep.Warnings = len(res.Skills), res.Warnings
		for _, sk := range res.Skills {
			if len(sk.Notes) > 0 && len(rep.Flagged) < MaxFlagged {
				rep.Flagged = append(rep.Flagged, Flagged{Name: sk.Name, Notes: sk.Notes})
			}
		}
		cat.Reports = append(cat.Reports, rep)
		if !src.Enabled {
			continue
		}
		for _, sk := range res.Skills {
			e := Entry{ID: src.ID + "/" + sk.Name, SourceID: src.ID, SourceName: src.Name, Origin: origin, Revision: src.Revision, Skill: sk, Root: src.Root}
			key := strings.ToLower(sk.Name)
			if by, taken := winner[key]; taken {
				e.Shadowed, e.ShadowedBy = true, by
			} else {
				winner[key] = src.Name
			}
			cat.Entries = append(cat.Entries, e)
		}
	}
	return cat
}
