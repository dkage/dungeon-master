package server

import (
	"net/http"

	"github.com/dkage/dungeon-master/internal/appearance"
)

type appearanceResp struct {
	Settings appearance.Settings `json:"settings"`
	// Stored is false until something was saved; the page then offers what
	// the browser held before, once.
	Stored bool `json:"stored"`
}

func (s *Server) handleAppearance(w http.ResponseWriter, _ *http.Request) {
	cur, stored := s.cfg.Appearance.Get()
	writeJSON(w, http.StatusOK, appearanceResp{Settings: cur, Stored: stored})
}

// handleSaveAppearance applies the fields in the body over the current
// settings; a missing field keeps its value and an invalid one refuses the
// whole write.
func (s *Server) handleSaveAppearance(w http.ResponseWriter, r *http.Request) {
	cur, _ := s.cfg.Appearance.Get()
	in := cur
	if !decode(w, r, &in) {
		return
	}
	if err := appearance.Validate(in); err != nil {
		httpErr(w, http.StatusBadRequest, err.Error())
		return
	}
	saved, err := s.cfg.Appearance.Update(func(s *appearance.Settings) { *s = in })
	if err != nil {
		httpErr(w, http.StatusInternalServerError, "could not save the settings")
		return
	}
	writeJSON(w, http.StatusOK, appearanceResp{Settings: saved, Stored: true})
}
