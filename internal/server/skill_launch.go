package server

import (
	"errors"

	"github.com/dkage/dungeon-master/internal/briefing"
	"github.com/dkage/dungeon-master/internal/skillcatalog"
)

// Delivery says how a chosen skill reaches the agent. Only the briefing is
// offered: the skill's text and directory go into the reviewed briefing,
// which works for every source. Native slash invocation of a cached external
// source is not claimed; it needs the real-CLI probe tracked in #66.
const deliveryBriefing = "briefing"

var (
	// errMethodUnavailable: the chosen skill is no longer offered to the project.
	errMethodUnavailable = errors.New("the chosen skill is no longer available to this project")
	// errMethodChanged: the skill is there, but is not what was reviewed.
	errMethodChanged = errors.New("the chosen skill changed after it was reviewed")
)

// projectCatalog is the catalog a campaign sees: its enabled registered
// sources in its own order, then the supplementary places the user turned on
// (the project's own folders, the user's, installed plugins). Registered
// sources win a name; the others stay selectable by their qualified id.
func (s *Server) projectCatalog(campaignID string) skillcatalog.Catalog {
	eff := s.cfg.SkillSources.ForCampaign(campaignID)
	in := make([]skillcatalog.Source, 0, len(eff))
	for _, e := range eff {
		in = append(in, catalogSource(e.Source, e.Enabled))
	}
	d := s.cfg.SkillSources.Discovery()
	if c, ok := s.cfg.Campaigns.Get(campaignID); ok {
		in = append(in, skillcatalog.Supplementary(skillcatalog.Discovery{Project: d.Project, User: d.User, Plugins: d.Plugins},
			[]string{c.Dir, c.GitRoot}, s.cfg.UserHome)...)
	}
	return skillcatalog.Build(in)
}

// resolvedMethod is a skill ready to go into a briefing.
type resolvedMethod struct {
	entry  skillcatalog.Entry
	method briefing.Method
	sum    string
}

// resolveMethod finds the skill with this source-qualified id in the
// campaign's catalog. It never falls back to another source or another
// skill of the same name: a disabled source, a removed skill or a different
// source is an error.
func (s *Server) resolveMethod(campaignID, id string) (resolvedMethod, error) {
	for _, e := range s.projectCatalog(campaignID).Entries {
		if e.ID != id {
			continue
		}
		body, sum, err := skillcatalog.Read(e.Root, e.File)
		if err != nil {
			return resolvedMethod{}, errMethodUnavailable
		}
		return resolvedMethod{entry: e, sum: sum, method: briefing.Method{
			Name: e.Name, Source: e.SourceName, Revision: e.Revision, Dir: e.Dir, File: e.File, Body: body,
		}}, nil
	}
	return resolvedMethod{}, errMethodUnavailable
}

// revalidate checks that the skill is still the one that was reviewed.
func (s *Server) revalidate(campaignID string, meta briefing.PlanMeta) (resolvedMethod, error) {
	m, err := s.resolveMethod(campaignID, meta.SkillID)
	if err != nil {
		return resolvedMethod{}, err
	}
	if m.sum != meta.SHA256 || m.entry.Revision != meta.Revision || m.entry.SourceID != meta.SourceID {
		return resolvedMethod{}, errMethodChanged
	}
	return m, nil
}

type methodJSON struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	SourceName string `json:"sourceName"`
	Revision   string `json:"revision,omitempty"`
	Dir        string `json:"dir"`
	File       string `json:"file"`
	Delivery   string `json:"delivery"`
	// Origin says where the skill was found (registered, project, user or
	// plugin); Notes are compatibility hints from inspecting it.
	Origin string   `json:"origin"`
	Notes  []string `json:"notes"`
	// Truncated says only the start of the skill text was copied into the
	// briefing; the whole file stays readable at File.
	Truncated bool `json:"truncated,omitempty"`
}

func (m resolvedMethod) json(truncated bool) *methodJSON {
	return &methodJSON{ID: m.entry.ID, Name: m.entry.Name, SourceName: m.entry.SourceName, Revision: m.entry.Revision,
		Dir: m.entry.Dir, File: m.entry.File, Delivery: deliveryBriefing, Truncated: truncated,
		Origin: m.entry.Origin, Notes: m.entry.Notes}
}
