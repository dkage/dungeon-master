package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"path/filepath"
	"strings"

	"github.com/dkage/dungeon-master/internal/campaign"
	"github.com/dkage/dungeon-master/internal/history"
	"github.com/dkage/dungeon-master/internal/platform"
	"github.com/dkage/dungeon-master/internal/worktree"
)

// workingCopyJSON is the launch review's account of where an issue session
// would run: its own git worktree, or the project's main working tree.
type workingCopyJSON struct {
	// Available is false when the project is not in a Git repository.
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
	// Location is the folder holding every copy of this project (in local
	// state, outside the repository); Branch and Start say what a copy gets.
	Location  string `json:"location,omitempty"`
	Branch    string `json:"branch,omitempty"`
	Start     string `json:"start,omitempty"`
	StartName string `json:"startName,omitempty"`
	// MainChanges counts uncommitted or untracked paths in the main tree,
	// which a copy does not contain.
	MainChanges int `json:"mainChanges"`
	// Entries are the ignored and untracked top-level entries a copy lacks.
	Entries []worktree.Entry `json:"entries"`
	// MainBusy is the issue session already working in the main tree.
	MainBusy *sessionJSON `json:"mainBusy,omitempty"`
}

func (s *Server) workingCopyPlan(ctx context.Context, c campaign.Campaign, issue int) workingCopyJSON {
	out := workingCopyJSON{Entries: []worktree.Entry{}}
	if busy := s.activeQuestSessionFor(c); busy != nil {
		out.MainBusy = busy
	}
	if c.GitRoot == "" || s.cfg.Worktrees == nil {
		out.Reason = "This project is not in a Git repository, so it has no working copies."
		return out
	}
	plan, err := s.cfg.Worktrees.Plan(ctx, c.GitRoot, c.ID, issue)
	if err != nil {
		out.Reason = "Git could not describe this repository: " + err.Error()
		return out
	}
	out.Available = true
	out.Location = filepath.Dir(plan.Path)
	out.Branch, out.Start, out.StartName = plan.Branch, plan.Start, plan.StartName
	if changes, err := s.cfg.Worktrees.Changes(ctx, c.GitRoot); err == nil {
		out.MainChanges = len(changes)
	}
	if entries, err := s.cfg.Worktrees.Entries(ctx, c.GitRoot); err == nil && entries != nil {
		out.Entries = entries
	}
	return out
}

// makeWorkingCopy creates the copy for an issue session and brings over the
// entries the user chose; a failure leaves nothing behind.
func (s *Server) makeWorkingCopy(ctx context.Context, c campaign.Campaign, issue int, carry map[string]string) (worktree.Copy, error) {
	entries, err := s.cfg.Worktrees.Entries(ctx, c.GitRoot)
	if err != nil {
		return worktree.Copy{}, err
	}
	if err := worktree.CheckCarry(carry, entries); err != nil {
		return worktree.Copy{}, err
	}
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return worktree.Copy{}, err
	}
	cp, err := s.cfg.Worktrees.Create(ctx, c.GitRoot, c.ID, "w"+hex.EncodeToString(b), issue)
	if err != nil {
		return worktree.Copy{}, err
	}
	if len(carry) > 0 {
		if err := worktree.Carry(c.GitRoot, cp.Path, carry, entries); err != nil {
			_, _ = s.cfg.Worktrees.Remove(context.Background(), c.GitRoot, cp)
			return worktree.Copy{}, err
		}
	}
	return cp, nil
}

// copyDir is where a campaign's session starts inside a copy: the same
// subfolder of the copy as the campaign has in the repository.
func (s *Server) copyDir(c campaign.Campaign, cp worktree.Copy) string {
	root, dir := realPath(c.GitRoot), realPath(c.Dir)
	rel, err := filepath.Rel(root, dir)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return cp.Path
	}
	return filepath.Join(cp.Path, rel)
}

func realPath(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

// removeWorkingCopy cleans up the copy of a closed session.
func (s *Server) removeWorkingCopy(ctx context.Context, rec history.Record) worktree.Result {
	camp, ok := s.cfg.Campaigns.Get(rec.Campaign)
	if !ok || s.cfg.Worktrees == nil || camp.GitRoot == "" {
		return worktree.Result{Kept: true}
	}
	res, err := s.cfg.Worktrees.Remove(ctx, camp.GitRoot, worktree.Copy{Path: rec.Copy, Branch: rec.Branch, Start: rec.CopyStart})
	if err != nil {
		s.cfg.Log.Warn("could not remove a working copy", "session", rec.ID, "error", err.Error())
		res.Kept = true
	}
	return res
}

// liveQuestSessions are the running issue sessions whose project shares the
// campaign's Git working tree (campaigns added from subfolders share it).
func (s *Server) liveQuestSessions(c campaign.Campaign) []history.Record {
	tree := c.GitRoot
	if tree == "" {
		tree = c.Dir
	}
	var out []history.Record
	for _, rec := range s.cfg.History.List() {
		if rec.Quest <= 0 || rec.Kind != "hero" || rec.Ended != nil || rec.Lost {
			continue
		}
		live, ok := s.cfg.Sessions.Get(rec.ID)
		if !ok {
			continue
		}
		if exited, _ := live.Exited(); exited {
			continue
		}
		other, ok := s.cfg.Campaigns.Get(rec.Campaign)
		if !ok {
			continue
		}
		otherTree := other.GitRoot
		if otherTree == "" {
			otherTree = other.Dir
		}
		if platform.SamePath(otherTree, tree) {
			out = append(out, rec)
		}
	}
	return out
}

func recordView(rec history.Record) *sessionJSON {
	return &sessionJSON{
		ID: rec.ID, Kind: rec.Kind, Hero: rec.Hero, Quest: rec.Quest, Title: rec.Title,
		Campaign: rec.Campaign, Dir: rec.Dir, Created: rec.Created, Copy: rec.Copy, Branch: rec.Branch,
	}
}

// activeQuestSessionFor finds the running issue session working in the
// campaign's main working tree (sessions in their own copies do not count).
func (s *Server) activeQuestSessionFor(c campaign.Campaign) *sessionJSON {
	for _, rec := range s.liveQuestSessions(c) {
		if rec.Copy == "" {
			return recordView(rec)
		}
	}
	return nil
}

// activeIssueSession finds a running session for the same issue anywhere in
// the project: one active session per issue.
func (s *Server) activeIssueSession(c campaign.Campaign, issue int) *sessionJSON {
	for _, rec := range s.liveQuestSessions(c) {
		if rec.Quest == issue {
			return recordView(rec)
		}
	}
	return nil
}
