package server

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/dkage/dungeon-master/internal/campaign"
	"github.com/dkage/dungeon-master/internal/quartermaster"
	"github.com/dkage/dungeon-master/internal/quests"
)

func (s *Server) handleQuests(w http.ResponseWriter, r *http.Request) {
	c, ok := s.cfg.Campaigns.Get(r.PathValue("id"))
	if !ok {
		httpErr(w, http.StatusNotFound, "no such campaign")
		return
	}
	ctx := r.Context()
	if r.URL.Query().Get("background") == "1" {
		// Polling nobody is waiting for keeps more of the GitHub budget in reserve.
		ctx = quartermaster.WithClass(ctx, quartermaster.Background)
	}
	res, err := s.cfg.Quests.List(ctx, c, r.URL.Query().Get("refresh") == "1")
	if err != nil {
		questsErr(w, err, c.Repo)
		return
	}
	if res.Quests == nil {
		res.Quests = []quests.Quest{}
	}
	writeJSON(w, http.StatusOK, res)
}

// handleScopeSet stores which issues a campaign shows.
func (s *Server) handleScopeSet(w http.ResponseWriter, r *http.Request) {
	var sc campaign.Scope
	if !decode(w, r, &sc) {
		return
	}
	c, err := s.cfg.Campaigns.Update(r.Context(), r.PathValue("id"), campaign.Patch{Scope: &sc})
	if err != nil {
		campaignErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// handleScopeOptions returns what each choice of scope would show, for the
// question asked when a campaign first connects to a repository.
func (s *Server) handleScopeOptions(w http.ResponseWriter, r *http.Request) {
	c, ok := s.cfg.Campaigns.Get(r.PathValue("id"))
	if !ok {
		httpErr(w, http.StatusNotFound, "no such campaign")
		return
	}
	counts, err := s.cfg.Quests.Counts(r.Context(), c)
	if err != nil {
		questsErr(w, err, c.Repo)
		return
	}
	if counts.Labels == nil {
		counts.Labels = []quests.LabelCount{}
	}
	writeJSON(w, http.StatusOK, counts)
}

func (s *Server) handleQuest(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, http.StatusOK, d)
}

// questsErr turns a source failure into a status, a code the UI can act on
// and a message that says what to do next. Raw upstream details never reach
// the browser beyond what the sources chose to put in their error text.
func questsErr(w http.ResponseWriter, err error, repo string) {
	var rl *quests.RateLimitError
	switch {
	case errors.Is(err, quests.ErrNoRepo):
		httpErrCode(w, http.StatusConflict, "no_repo",
			"This campaign has no GitHub repository. Give the project a remote named origin that points at github.com, or load the demo quests.")
	case errors.Is(err, quests.ErrScopeNeeded):
		httpErrCode(w, http.StatusConflict, "scope_needed", "Choose which of this repository's issues to show.")
	case errors.Is(err, quests.ErrNoToken):
		httpErrCode(w, http.StatusServiceUnavailable, "no_token",
			"No GitHub credentials found. Run `gh auth login` (or set GH_TOKEN), then refresh.")
	case errors.Is(err, quests.ErrUnauthorized):
		httpErrCode(w, http.StatusBadGateway, "github_unauthorized",
			"GitHub rejected the credentials. Run `gh auth login` again, or check GH_TOKEN.")
	case errors.As(err, &rl) && rl.Paused:
		until := "a while"
		if !rl.Reset.IsZero() {
			until = rl.Reset.Local().Format("15:04")
		}
		httpErrCode(w, http.StatusTooManyRequests, "camp",
			"To stay inside GitHub's limits the app is not sending requests until "+until+
				". Anything already loaded stays available.")
	case errors.As(err, &rl):
		httpErrCode(w, http.StatusTooManyRequests, "rate_limited", rl.Error()+". Try again later.")
	case errors.Is(err, quests.ErrNotFound):
		httpErrCode(w, http.StatusNotFound, "repo_not_found",
			"GitHub could not find "+repo+", or the credentials cannot see it.")
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		httpErrCode(w, http.StatusGatewayTimeout, "timeout", "GitHub took too long to answer.")
	default:
		httpErrCode(w, http.StatusBadGateway, "upstream", err.Error())
	}
}

// handleGitHub reports the GitHub budget and whether requests are paused.
func (s *Server) handleGitHub(w http.ResponseWriter, _ *http.Request) {
	if s.cfg.Quartermaster == nil {
		writeJSON(w, http.StatusOK, quartermaster.Status{Rations: []quartermaster.Ration{}})
		return
	}
	writeJSON(w, http.StatusOK, s.cfg.Quartermaster.Status())
}
