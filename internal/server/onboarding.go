package server

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/dkage/dungeon-master/internal/campaign"
)

const maxSuggestions = 40

type projectSuggestion struct {
	Name string `json:"name"`
	Dir  string `json:"dir"`
	Repo string `json:"repo,omitempty"`
}

func (s *Server) handleSuggestions(w http.ResponseWriter, r *http.Request) {
	seen := make(map[string]bool)
	var candidates []string
	add := func(dir string) bool {
		if dir == "" || seen[dir] || len(candidates) >= maxSuggestions {
			return len(candidates) < maxSuggestions
		}
		seen[dir] = true
		candidates = append(candidates, dir)
		return len(candidates) < maxSuggestions
	}
	_ = add(s.cfg.StartDir)

	home, _ := os.UserHomeDir()
	for _, rootName := range []string{"Developer", "code", "src", "dev", "projects"} {
		root := filepath.Join(home, rootName)
		_ = walkSuggestedRepos(root, 2, add)
	}

	git := campaign.ExecGit{}
	items := make([]projectSuggestion, 0, len(candidates))
	for _, dir := range candidates {
		if _, err := os.Stat(dir); err != nil {
			continue
		}
		root, err := git.Toplevel(r.Context(), dir)
		if err != nil {
			continue
		}
		name := filepath.Base(dir)
		if dir == root {
			name = filepath.Base(root)
		}
		item := projectSuggestion{Name: name, Dir: dir}
		if remote, err := git.RemoteURL(r.Context(), root, "origin"); err == nil {
			if owner, repo, err := campaign.ParseGitHubRemote(remote); err == nil {
				item.Repo = owner + "/" + repo
			}
		}
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, items)
}

func walkSuggestedRepos(root string, depth int, add func(string) bool) bool {
	if depth <= 0 {
		return true
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return true
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") || entry.Name() == "node_modules" || entry.Name() == "vendor" || entry.Name() == "dist" || entry.Name() == "build" {
			continue
		}
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			if !add(dir) {
				return false
			}
			continue
		}
		if !walkSuggestedRepos(dir, depth-1, add) {
			return false
		}
	}
	return true
}

func (s *Server) handleSampleCampaign(w http.ResponseWriter, r *http.Request) {
	for _, current := range s.cfg.Campaigns.List() {
		if current.Sample {
			writeJSON(w, http.StatusOK, current)
			return
		}
	}
	if s.cfg.SampleDir == "" || !filepath.IsAbs(s.cfg.SampleDir) {
		httpErr(w, http.StatusServiceUnavailable, "sample workspace is not configured")
		return
	}
	if err := os.MkdirAll(s.cfg.SampleDir, 0o700); err != nil {
		httpErr(w, http.StatusInternalServerError, "could not create the sample workspace")
		return
	}
	readme := filepath.Join(s.cfg.SampleDir, "README.md")
	if err := writeSampleReadme(readme); err != nil {
		httpErr(w, http.StatusInternalServerError, "could not prepare the sample workspace")
		return
	}
	created, err := s.cfg.Campaigns.Add(r.Context(), campaign.Input{
		Dir: s.cfg.SampleDir, Name: "Sample (not a real project)", Demo: true, Sample: true,
	})
	if err != nil {
		if errors.Is(err, campaign.ErrDuplicate) {
			for _, current := range s.cfg.Campaigns.List() {
				if current.Sample {
					writeJSON(w, http.StatusOK, current)
					return
				}
			}
		}
		campaignErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func writeSampleReadme(path string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString("# Sample workspace\n\nThis throwaway folder is provided so you can try Dungeon Master without changing a real project.\n")
	return err
}
