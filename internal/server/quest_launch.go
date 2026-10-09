package server

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dkage/dungeon-master/internal/briefing"
	"github.com/dkage/dungeon-master/internal/campaign"
	"github.com/dkage/dungeon-master/internal/launcher"
	"github.com/dkage/dungeon-master/internal/party"
	"github.com/dkage/dungeon-master/internal/quests"
)

type launchPreviewRequest struct {
	Hero string `json:"hero"`
}

type launchPreviewJSON struct {
	briefing.Result
	CanStart     bool     `json:"canStart"`
	Reason       string   `json:"reason,omitempty"`
	Hero         string   `json:"hero,omitempty"`
	HeroLabel    string   `json:"heroLabel,omitempty"`
	Directory    string   `json:"directory,omitempty"`
	BriefingID   string   `json:"briefingId,omitempty"`
	BriefingFile string   `json:"briefingFile,omitempty"`
	Opener       string   `json:"opener,omitempty"`
	SystemPrompt string   `json:"systemPrompt,omitempty"`
	Command      []string `json:"command,omitempty"`
}

// handleQuestLaunchPreview assembles and saves the exact context that Start
// will consume. It reuses the short-lived issue detail cache populated by the
// selected-quest view, so preview does not add a GitHub request.
func (s *Server) handleQuestLaunchPreview(w http.ResponseWriter, r *http.Request) {
	var req launchPreviewRequest
	if !decode(w, r, &req) {
		return
	}
	c, ok := s.cfg.Campaigns.Get(r.PathValue("id"))
	if !ok {
		httpErr(w, http.StatusNotFound, "no such campaign")
		return
	}
	n, err := strconv.Atoi(r.PathValue("number"))
	if err != nil || n <= 0 {
		httpErr(w, http.StatusBadRequest, "quest number must be a positive integer")
		return
	}
	d, err := s.cfg.Quests.Detail(r.Context(), c, n)
	if err != nil {
		if errors.Is(err, quests.ErrNotFound) {
			httpErrCode(w, http.StatusNotFound, "not_found", "That issue was not found.")
			return
		}
		questsErr(w, err, c.Repo)
		return
	}
	out := launchPreviewJSON{Result: briefing.Build(c.Repo, d)}
	if reason := questStartBlock(d); reason != "" {
		out.Reason = reason
		writeJSON(w, http.StatusOK, out)
		return
	}
	s.partyMu.RLock()
	h, ok := s.cfg.Party.Hero(req.Hero)
	s.partyMu.RUnlock()
	if !ok {
		httpErr(w, http.StatusBadRequest, "choose a configured hero")
		return
	}
	if s.cfg.Claude == "" {
		out.Reason = "Claude Code was not found. Install it or set its path in hero settings."
		writeJSON(w, http.StatusOK, out)
		return
	}
	plan, err := s.questLaunchPlan(c, d, h, "")
	if err != nil {
		httpErr(w, http.StatusInternalServerError, "could not prepare the private task briefing")
		return
	}
	out.CanStart = true
	out.Hero, out.HeroLabel = h.ID, h.Label
	out.Directory, out.BriefingID, out.BriefingFile = c.Dir, plan.id, plan.path
	out.Opener, out.SystemPrompt, out.Command = plan.opener, plan.systemPrompt, plan.command
	writeJSON(w, http.StatusOK, out)
}

type questLaunchPlan struct {
	id           string
	path         string
	content      string
	opener       string
	systemPrompt string
	command      []string
}

func (s *Server) questLaunchPlan(c campaign.Campaign, d quests.Detail, h party.Hero, previewID string) (questLaunchPlan, error) {
	var id, path string
	var content string
	var err error
	if previewID == "" {
		built := briefing.Build(c.Repo, d)
		content = built.Content
		id, path, err = briefing.SavePreview(s.cfg.StateDir, c.ID, d.Number, h.ID, content)
	} else {
		path, err = briefing.PreviewPath(s.cfg.StateDir, c.ID, d.Number, h.ID, previewID)
		if err == nil {
			contentBytes, readErr := os.ReadFile(path)
			err = readErr
			content = string(contentBytes)
			id = previewID
		}
	}
	if err != nil {
		return questLaunchPlan{}, err
	}
	opener := fmt.Sprintf("Read the task briefing file at %q in full, then work on GitHub issue #%d: %s.", path, d.Number, d.URL)
	systemPrompt := s.systemPrompt(briefing.Instructions)
	opts := launcher.Opts{
		Opener: opener, AppendSystemPrompt: briefing.Instructions,
		AddDirs: []string{filepath.Dir(path)},
	}
	opts = s.withGatekeeperRule(opts)
	return questLaunchPlan{
		id: id, path: path, content: content, opener: opener,
		systemPrompt: systemPrompt,
		command:      launcher.Command(s.cfg.Claude, h, opts),
	}, nil
}

func questStartBlock(d quests.Detail) string {
	switch d.State {
	case quests.StateOpen, quests.StateClaimed:
		return ""
	case quests.StateLocked:
		var blockers []string
		for _, ref := range d.BlockedBy {
			if !ref.Closed {
				blockers = append(blockers, "#"+strconv.Itoa(ref.Number))
			}
		}
		if len(blockers) > 0 {
			return "Blocked by " + strings.Join(blockers, ", ") + "."
		}
		return "This issue is blocked and cannot be started yet."
	case quests.StateCleared:
		return "This issue is closed and cannot be started."
	default:
		return "This issue cannot be started yet."
	}
}
