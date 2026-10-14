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
	// Skill is a source-qualified skill id from the project's catalog; empty
	// means no skill. It is never a bare name and never falls back.
	Skill string `json:"skill,omitempty"`
	// Direction is the operator's own text for this launch.
	Direction string `json:"direction,omitempty"`
}

type launchPreviewJSON struct {
	briefing.Result
	CanStart        bool         `json:"canStart"`
	Reason          string       `json:"reason,omitempty"`
	Hero            string       `json:"hero,omitempty"`
	HeroLabel       string       `json:"heroLabel,omitempty"`
	Directory       string       `json:"directory,omitempty"`
	BriefingID      string       `json:"briefingId,omitempty"`
	BriefingFile    string       `json:"briefingFile,omitempty"`
	Opener          string       `json:"opener,omitempty"`
	SystemPrompt    string       `json:"systemPrompt,omitempty"`
	Command         []string     `json:"command,omitempty"`
	ExistingSession *sessionJSON `json:"existingSession,omitempty"`
	// WorkingCopy says where the session would run and what a copy lacks.
	WorkingCopy *workingCopyJSON `json:"workingCopy,omitempty"`
	// Method is the chosen skill with its pin and delivery; absent for none.
	Method *methodJSON `json:"method,omitempty"`
	// MethodCode names why a chosen skill cannot be used, so the page can
	// ask the operator to choose again rather than launch something else.
	MethodCode string `json:"methodCode,omitempty"`
	Direction  string `json:"direction,omitempty"`
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
	if existing := s.activeIssueSession(c, n); existing != nil {
		out.ExistingSession = existing
		out.Reason = existing.Title + " already has a session."
		writeJSON(w, http.StatusOK, out)
		return
	}
	wc := s.workingCopyPlan(r.Context(), c, n)
	out.WorkingCopy = &wc
	if !wc.Available && wc.MainBusy != nil && !c.ParallelQuests {
		out.ExistingSession = wc.MainBusy
		out.Reason = wc.MainBusy.Title + " is already working in this Git working tree."
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
	if len([]rune(req.Direction)) > briefing.MaxDirectionRunes*2 {
		httpErr(w, http.StatusBadRequest, "the additional direction is too long")
		return
	}
	plan, err := s.questLaunchPlan(c, d, h, "", launchChoices{Skill: req.Skill, Direction: req.Direction})
	switch {
	case errors.Is(err, errMethodUnavailable), errors.Is(err, errMethodChanged):
		out.Reason, out.MethodCode = err.Error()+". Choose a skill again.", "method_unavailable"
		writeJSON(w, http.StatusOK, out)
		return
	case err != nil:
		httpErr(w, http.StatusInternalServerError, "could not prepare the private task briefing")
		return
	}
	out.Result = plan.result
	out.Method, out.Direction = plan.method, strings.TrimSpace(req.Direction)
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
	addDirs      []string
	result       briefing.Result
	method       *methodJSON
}

// launchChoices are what the operator added to a launch. They only matter
// when a preview is made; Start reuses the reviewed briefing and its record.
type launchChoices struct {
	Skill     string
	Direction string
}

func (s *Server) questLaunchPlan(c campaign.Campaign, d quests.Detail, h party.Hero, previewID string, choice launchChoices) (questLaunchPlan, error) {
	var id, path string
	var content string
	var result briefing.Result
	var method *resolvedMethod
	var err error
	if previewID == "" {
		opt := briefing.Options{Direction: choice.Direction}
		if choice.Skill != "" {
			m, merr := s.resolveMethod(c.ID, choice.Skill)
			if merr != nil {
				return questLaunchPlan{}, merr
			}
			method, opt.Method = &m, &m.method
		}
		result = briefing.BuildWith(c.Repo, d, opt)
		content = result.Content
		id, path, err = briefing.SavePreview(s.cfg.StateDir, c.ID, d.Number, h.ID, content)
		if err == nil && method != nil {
			err = briefing.SaveMeta(path, briefing.PlanMeta{SkillID: method.entry.ID, SourceID: method.entry.SourceID, Revision: method.entry.Revision, SHA256: method.sum})
		}
	} else {
		path, err = briefing.PreviewPath(s.cfg.StateDir, c.ID, d.Number, h.ID, previewID)
		if err == nil {
			contentBytes, readErr := os.ReadFile(path)
			err = readErr
			content = string(contentBytes)
			id = previewID
		}
		// The reviewed method must still be exactly what was reviewed.
		if err == nil {
			meta, ok, metaErr := briefing.ReadMeta(path)
			if metaErr != nil {
				err = metaErr
			} else if ok {
				m, rerr := s.revalidate(c.ID, meta)
				if rerr != nil {
					return questLaunchPlan{}, rerr
				}
				method = &m
			}
		}
	}
	if err != nil {
		return questLaunchPlan{}, err
	}
	opener := fmt.Sprintf("Read the task briefing file at %q in full, then work on GitHub issue #%d: %s.", path, d.Number, d.URL)
	addDirs := []string{filepath.Dir(path)}
	var methodOut *methodJSON
	if method != nil {
		opener = fmt.Sprintf("Read the task briefing file at %q in full, follow the method it names, then work on GitHub issue #%d: %s.", path, d.Number, d.URL)
		addDirs = append(addDirs, method.entry.Dir)
		methodOut = method.json(result.MethodTruncated)
	}
	systemPrompt := s.systemPrompt(briefing.Instructions)
	opts := launcher.Opts{
		Opener: opener, AppendSystemPrompt: briefing.Instructions,
		AddDirs: addDirs,
	}
	opts = s.withGatekeeperRule(opts)
	return questLaunchPlan{
		id: id, path: path, content: content, opener: opener,
		systemPrompt: systemPrompt, addDirs: addDirs, result: result, method: methodOut,
		command: launcher.Command(s.cfg.Claude, h, opts),
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
