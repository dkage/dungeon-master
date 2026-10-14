package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/dkage/dungeon-master/internal/skillcatalog"
	"github.com/dkage/dungeon-master/internal/skillsource"
)

// A fetch can take a while; it must not outlive a request that gave up, nor
// run forever.
const skillFetchTimeout = 3 * time.Minute

type skillSourcesResp struct {
	Sources     []skillsource.Source    `json:"sources"`
	Suggestions []skillsource.Suggested `json:"suggestions"`
	// Reports say how many skills each source holds and why one is empty or
	// partly skipped; scanning is what the Rescan button repeats.
	Reports []skillcatalog.SourceReport `json:"reports"`
	// Discovery says which supplementary places are turned on.
	Discovery skillsource.Discovery `json:"discovery"`
}

func (s *Server) skillSourcesView() skillSourcesResp {
	srcs := s.cfg.SkillSources.List()
	in := make([]skillcatalog.Source, 0, len(srcs))
	for _, src := range srcs {
		in = append(in, catalogSource(src, src.Enabled))
	}
	return skillSourcesResp{Sources: srcs, Suggestions: skillsource.Suggestions(), Reports: skillcatalog.Build(in).Reports, Discovery: s.cfg.SkillSources.Discovery()}
}

// catalogSource says where a source's files are: the folder itself, or the
// verified checkout of a Git source (nothing until one succeeded).
func catalogSource(src skillsource.Source, enabled bool) skillcatalog.Source {
	cs := skillcatalog.Source{ID: src.ID, Name: src.Name, Enabled: enabled}
	switch {
	case src.Kind == skillsource.Folder:
		cs.Root = src.Location
	case src.Verified != nil:
		cs.Root, cs.Revision = src.Verified.Dir, src.Verified.Revision
	}
	return cs
}

func (s *Server) handleSkillSources(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.skillSourcesView())
}

// handleCampaignSkills is the catalog the campaign sees: its enabled sources
// in its own order, each skill under a source-qualified id.
func (s *Server) handleCampaignSkills(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := s.cfg.Campaigns.Get(id); !ok {
		httpErr(w, http.StatusNotFound, "no such campaign")
		return
	}
	writeJSON(w, http.StatusOK, s.projectCatalog(id))
}

type skillSourceIn struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Location string `json:"location"`
	Pin      string `json:"pin"`
	// Suggestion names an opt-in source whose licence and notice are kept
	// with it; its location is then the suggestion's.
	Suggestion string `json:"suggestion"`
}

func (s *Server) handleSkillSourceAdd(w http.ResponseWriter, r *http.Request) {
	var in skillSourceIn
	if !decode(w, r, &in) {
		return
	}
	input := skillsource.Input{Name: in.Name, Kind: skillsource.Kind(in.Kind), Location: in.Location, Pin: in.Pin}
	if in.Suggestion != "" {
		found := false
		for _, sg := range skillsource.Suggestions() {
			if sg.Key == in.Suggestion {
				found = true
				input.Kind, input.Location, input.License, input.Notice = skillsource.Git, sg.Location, sg.License, sg.Notice
				if input.Name == "" {
					input.Name = sg.Name
				}
			}
		}
		if !found {
			httpErr(w, http.StatusBadRequest, "unknown suggestion")
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), skillFetchTimeout)
	defer cancel()
	src, err := s.cfg.SkillSources.Add(ctx, input)
	if !skillSourceReply(w, http.StatusCreated, src, err) {
		return
	}
}

func (s *Server) handleSkillSourceUpdate(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name     *string `json:"name"`
		Enabled  *bool   `json:"enabled"`
		Pin      *string `json:"pin"`
		Location *string `json:"location"`
	}
	if !decode(w, r, &in) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), skillFetchTimeout)
	defer cancel()
	src, err := s.cfg.SkillSources.Update(ctx, r.PathValue("id"), skillsource.Patch{Name: in.Name, Enabled: in.Enabled, Pin: in.Pin, Location: in.Location})
	skillSourceReply(w, http.StatusOK, src, err)
}

func (s *Server) handleSkillSourceRefresh(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), skillFetchTimeout)
	defer cancel()
	src, err := s.cfg.SkillSources.Refresh(ctx, r.PathValue("id"))
	skillSourceReply(w, http.StatusOK, src, err)
}

func (s *Server) handleSkillSourceRemove(w http.ResponseWriter, r *http.Request) {
	if err := s.cfg.SkillSources.Remove(r.PathValue("id")); err != nil {
		skillSourceReply(w, 0, skillsource.Source{}, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSkillSourceOrder(w http.ResponseWriter, r *http.Request) {
	var in struct {
		IDs []string `json:"ids"`
	}
	if !decode(w, r, &in) {
		return
	}
	if err := s.cfg.SkillSources.Reorder(in.IDs); err != nil {
		skillSourceReply(w, 0, skillsource.Source{}, err)
		return
	}
	writeJSON(w, http.StatusOK, s.skillSourcesView())
}

type campaignSkillSourcesResp struct {
	Sources []skillsource.Effective `json:"sources"`
}

func (s *Server) handleCampaignSkillSources(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := s.cfg.Campaigns.Get(id); !ok {
		httpErr(w, http.StatusNotFound, "no such campaign")
		return
	}
	writeJSON(w, http.StatusOK, campaignSkillSourcesResp{Sources: s.cfg.SkillSources.ForCampaign(id)})
}

// handleCampaignSkillSourcesSet stores the campaign's own ordered list; an
// empty list returns the campaign to the inherited defaults.
func (s *Server) handleCampaignSkillSourcesSet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := s.cfg.Campaigns.Get(id); !ok {
		httpErr(w, http.StatusNotFound, "no such campaign")
		return
	}
	var in struct {
		Sources []skillsource.Mapping `json:"sources"`
	}
	if !decode(w, r, &in) {
		return
	}
	if err := s.cfg.SkillSources.SetCampaign(id, in.Sources); err != nil {
		skillSourceReply(w, 0, skillsource.Source{}, err)
		return
	}
	writeJSON(w, http.StatusOK, campaignSkillSourcesResp{Sources: s.cfg.SkillSources.ForCampaign(id)})
}

// skillSourceReply writes src, or the right error status; it reports whether
// it wrote src. Messages come from validation or the fetch, never from a path.
func skillSourceReply(w http.ResponseWriter, ok int, src skillsource.Source, err error) bool {
	var ve skillsource.ValidationError
	switch {
	case err == nil:
		writeJSON(w, ok, src)
		return true
	case errors.Is(err, skillsource.ErrNotFound):
		httpErr(w, http.StatusNotFound, "no such skill source")
	case errors.Is(err, skillsource.ErrDuplicate):
		httpErr(w, http.StatusConflict, err.Error())
	case errors.As(err, &ve):
		httpErr(w, http.StatusBadRequest, err.Error())
	default:
		httpErr(w, http.StatusInternalServerError, "could not save the skill sources")
	}
	return false
}

// handleSkillDiscovery saves which supplementary places to look in.
func (s *Server) handleSkillDiscovery(w http.ResponseWriter, r *http.Request) {
	var in skillsource.Discovery
	if !decode(w, r, &in) {
		return
	}
	if err := s.cfg.SkillSources.SetDiscovery(in); err != nil {
		httpErr(w, http.StatusInternalServerError, "could not save the skill sources")
		return
	}
	writeJSON(w, http.StatusOK, s.skillSourcesView())
}
