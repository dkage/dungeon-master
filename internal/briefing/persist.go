package briefing

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/dkage/dungeon-master/internal/fsutil"
)

var (
	campaignIDPattern = regexp.MustCompile(`^c[0-9a-f]{8}$`)
	heroIDPattern     = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)
	previewIDPattern  = regexp.MustCompile(`^[0-9a-f]{32}$`)
)

// SavePreview writes a launch preview under private state and returns its
// opaque id and exact path. Files and their directory have 0600/0700 modes.
func SavePreview(stateDir, campaignID string, issue int, heroID, content string) (string, string, error) {
	if stateDir == "" || !campaignIDPattern.MatchString(campaignID) || issue <= 0 || !heroIDPattern.MatchString(heroID) {
		return "", "", errors.New("briefing: invalid preview location")
	}
	for range 5 {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			return "", "", fmt.Errorf("briefing: generate preview id: %w", err)
		}
		id := hex.EncodeToString(b)
		path := filepath.Join(stateDir, "briefings", campaignID, fmt.Sprintf("%d-%s-%s.md", issue, heroID, id))
		if err := fsutil.CreateFileAtomic(path, []byte(content)); err == nil {
			return id, path, nil
		} else if !errors.Is(err, os.ErrExist) {
			return "", "", fmt.Errorf("briefing: save preview: %w", err)
		}
	}
	return "", "", errors.New("briefing: could not allocate preview file")
}

// PreviewPath resolves a preview id only inside its campaign/issue/hero path.
func PreviewPath(stateDir, campaignID string, issue int, heroID, id string) (string, error) {
	if stateDir == "" || !campaignIDPattern.MatchString(campaignID) || issue <= 0 || !heroIDPattern.MatchString(heroID) || !previewIDPattern.MatchString(id) {
		return "", errors.New("briefing: invalid preview reference")
	}
	return filepath.Join(stateDir, "briefings", campaignID, fmt.Sprintf("%d-%s-%s.md", issue, heroID, id)), nil
}

// PlanMeta is what a reviewed launch relied on besides the briefing text: the
// exact skill. Start compares it with the skill as it is now, and refuses a
// launch whose method changed instead of substituting another.
type PlanMeta struct {
	SkillID  string `json:"skillId"`
	SourceID string `json:"sourceId"`
	Revision string `json:"revision,omitempty"`
	SHA256   string `json:"sha256"`
}

func metaPath(briefingPath string) string { return briefingPath + ".plan.json" }

// SaveMeta stores the metadata beside the briefing file it describes.
func SaveMeta(briefingPath string, m PlanMeta) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(metaPath(briefingPath), append(b, '\n'))
}

// ReadMeta returns the metadata saved for a briefing; ok is false when the
// launch has no method.
func ReadMeta(briefingPath string) (m PlanMeta, ok bool, err error) {
	b, err := os.ReadFile(metaPath(briefingPath))
	if errors.Is(err, os.ErrNotExist) {
		return PlanMeta{}, false, nil
	}
	if err != nil {
		return PlanMeta{}, false, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return PlanMeta{}, false, err
	}
	return m, true, nil
}
