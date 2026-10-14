package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dkage/dungeon-master/internal/briefing"
	"github.com/dkage/dungeon-master/internal/quests"
	"github.com/dkage/dungeon-master/internal/skillsource"
)

type launchPreviewResp struct {
	Content      string      `json:"content"`
	CanStart     bool        `json:"canStart"`
	Reason       string      `json:"reason"`
	BriefingID   string      `json:"briefingId"`
	BriefingFile string      `json:"briefingFile"`
	Opener       string      `json:"opener"`
	Direction    string      `json:"direction"`
	DirectionCut bool        `json:"directionTruncated"`
	Truncated    bool        `json:"truncated"`
	MethodCode   string      `json:"methodCode"`
	Method       *methodJSON `json:"method"`
	Command      []string    `json:"command"`
}

type skillLaunch struct {
	*env
	reg *skillsource.Registry
}

// skillLaunchServer is a server with one open issue and a skill registry.
func skillLaunchServer(t *testing.T) skillLaunch {
	t.Helper()
	dir := t.TempDir()
	reg, err := skillsource.Open("", skillsource.Options{Root: filepath.Join(dir, "co")})
	if err != nil {
		t.Fatal(err)
	}
	e := startServerWithQuestTTL(t, func(c *Config) { c.SkillSources = reg }, time.Minute)
	e.gh.mu.Lock()
	e.gh.detail = quests.Detail{
		Quest: quests.Quest{Number: 42, Title: "Fix signup validation", State: quests.StateOpen, URL: "https://github.com/acme/widgets/issues/42"},
		Body:  "Validate the address before saving.",
	}
	e.gh.mu.Unlock()
	return skillLaunch{env: e, reg: reg}
}

// addSkillFolder registers a folder holding one skill, with a template beside it.
func (s skillLaunch) addSkillFolder(t *testing.T, name, skill, text string) (id, dir string) {
	t.Helper()
	root := t.TempDir()
	dir = filepath.Join(root, skill)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	s.writeSkill(t, dir, skill, text)
	if err := os.WriteFile(filepath.Join(dir, "template.md"), []byte("a template"), 0o644); err != nil {
		t.Fatal(err)
	}
	src, err := s.reg.Add(t.Context(), skillsource.Input{Name: name, Kind: skillsource.Folder, Location: root})
	if err != nil {
		t.Fatal(err)
	}
	return src.ID, dir
}

func (s skillLaunch) writeSkill(t *testing.T, dir, skill, text string) {
	t.Helper()
	body := "---\nname: " + skill + "\ndescription: " + skill + " method\n---\n" + text + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (s skillLaunch) preview(t *testing.T, body map[string]any) (int, launchPreviewResp) {
	t.Helper()
	body["hero"] = "opus"
	resp, out := s.req(http.MethodPost, "/api/campaigns/"+s.campaign+"/quests/42/launch-preview", body, nil)
	return resp.StatusCode, decodeJSON[launchPreviewResp](t, out)
}

func (s skillLaunch) start(t *testing.T, briefingID string) (*http.Response, []byte) {
	t.Helper()
	return s.req(http.MethodPost, "/api/sessions", map[string]any{"campaign": s.campaign, "hero": "opus", "quest": 42, "briefingId": briefingID}, nil)
}

func TestChosenSkillAndDirectionReachTheAgentInTheReviewedBriefing(t *testing.T) {
	s := skillLaunchServer(t)
	srcID, skillDir := s.addSkillFolder(t, "Team skills", "tdd", "Write the failing test first.")
	skillDir = evalDir(t, skillDir)

	code, p := s.preview(t, map[string]any{"skill": srcID + "/tdd", "direction": "Only touch the signup form."})
	if code != http.StatusOK || !p.CanStart {
		t.Fatalf("preview: %d %+v", code, p)
	}
	m := p.Method
	if m == nil || m.ID != srcID+"/tdd" || m.SourceName != "Team skills" || m.Delivery != deliveryBriefing || m.Dir != skillDir || m.File != filepath.Join(skillDir, "SKILL.md") {
		t.Fatalf("method = %+v", m)
	}
	for _, want := range []string{
		"## Operator direction", "Only touch the signup form.", "## Method: tdd", "Write the failing test first.",
		"- Skill directory: " + skillDir, "Fix signup validation",
	} {
		if !strings.Contains(p.Content, want) {
			t.Errorf("briefing lacks %q:\n%s", want, p.Content)
		}
	}
	if strings.Contains(p.Content, "description: tdd method") {
		t.Error("front matter leaked into the instructions")
	}
	if !strings.Contains(p.Opener, "follow the method") || p.Direction != "Only touch the signup form." {
		t.Fatalf("opener/direction: %+v", p)
	}
	onDisk, err := os.ReadFile(p.BriefingFile)
	if err != nil || string(onDisk) != p.Content {
		t.Fatalf("the reviewed file differs from the preview: %v", err)
	}

	resp, out := s.start(t, p.BriefingID)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("start: %d %s", resp.StatusCode, out)
	}
	sj := decodeJSON[sessionJSON](t, out)
	c, _, err := s.dial(sj.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	output := readUntil(t, c, "cwd=")
	// The stub received the same briefing path, and read access to the skill's own directory.
	for _, want := range []string{"arg=[--add-dir]", skillDir, filepath.Dir(p.BriefingFile), strconv.Quote(p.BriefingFile), "follow the method"} {
		if !strings.Contains(output, want) {
			t.Errorf("session output missing %q:\n%s", want, output)
		}
	}
	after, _ := os.ReadFile(p.BriefingFile)
	if string(after) != p.Content {
		t.Fatal("the briefing changed between review and launch")
	}
}

func TestNoSkillLeavesTheLaunchAsItWas(t *testing.T) {
	s := skillLaunchServer(t)
	s.addSkillFolder(t, "Team skills", "tdd", "x")
	code, p := s.preview(t, map[string]any{})
	if code != http.StatusOK || !p.CanStart || p.Method != nil || strings.Contains(p.Content, "## Method") || strings.Contains(p.Opener, "method") {
		t.Fatalf("preview = %d %+v", code, p)
	}
	if _, ok, _ := briefing.ReadMeta(p.BriefingFile); ok {
		t.Fatal("a launch without a skill recorded one")
	}
	if resp, out := s.start(t, p.BriefingID); resp.StatusCode != http.StatusCreated {
		t.Fatalf("start: %d %s", resp.StatusCode, out)
	}
}

func TestUnknownOrUnqualifiedSkillsAreRefusedNotSubstituted(t *testing.T) {
	s := skillLaunchServer(t)
	srcID, _ := s.addSkillFolder(t, "Team skills", "tdd", "x")
	for name, id := range map[string]string{
		"bare name":       "tdd",
		"unknown source":  "ffffffffffffffff/tdd",
		"unknown skill":   srcID + "/nope",
		"path traversal":  srcID + "/../../etc/passwd",
		"only the source": srcID,
		"trailing slash":  srcID + "/tdd/",
		"different case":  srcID + "/TDD",
	} {
		code, p := s.preview(t, map[string]any{"skill": id})
		if code != http.StatusOK || p.CanStart || p.MethodCode != "method_unavailable" || p.Method != nil || p.BriefingFile != "" {
			t.Errorf("%s: %d %+v", name, code, p)
		}
	}
}

func TestDisablingTheSourceAfterReviewBlocksTheLaunchAndNeverPicksAnotherSource(t *testing.T) {
	s := skillLaunchServer(t)
	first, _ := s.addSkillFolder(t, "First", "tdd", "first method")
	second, _ := s.addSkillFolder(t, "Second", "tdd", "second method")
	_, p := s.preview(t, map[string]any{"skill": first + "/tdd"})
	if !p.CanStart || !strings.Contains(p.Content, "first method") {
		t.Fatalf("preview = %+v", p)
	}
	// This project stops using the first source; the second one has a skill of the same name.
	if err := s.reg.SetCampaign(s.campaign, []skillsource.Mapping{{SourceID: first, Enabled: false}, {SourceID: second, Enabled: true}}); err != nil {
		t.Fatal(err)
	}
	resp, out := s.start(t, p.BriefingID)
	if resp.StatusCode != http.StatusConflict || !strings.Contains(string(out), `"code":"method_unavailable"`) {
		t.Fatalf("start: %d %s", resp.StatusCode, out)
	}
	for _, sj := range decodeJSON[[]sessionJSON](t, getBody(t, s.env, "/api/sessions")) {
		if sj.Quest == 42 {
			t.Fatalf("a session started anyway: %+v", sj)
		}
	}
	// A new review of the second source's skill is a different, explicit choice.
	if _, p2 := s.preview(t, map[string]any{"skill": second + "/tdd"}); !p2.CanStart || !strings.Contains(p2.Content, "second method") {
		t.Fatalf("explicit second choice = %+v", p2)
	}
}

func TestEditedSkillAfterReviewInvalidatesTheReview(t *testing.T) {
	s := skillLaunchServer(t)
	id, dir := s.addSkillFolder(t, "Team skills", "tdd", "reviewed text")
	_, p := s.preview(t, map[string]any{"skill": id + "/tdd"})
	s.writeSkill(t, dir, "tdd", "text edited after the review")
	resp, out := s.start(t, p.BriefingID)
	if resp.StatusCode != http.StatusConflict || !strings.Contains(string(out), "method_unavailable") || !strings.Contains(string(out), "changed") {
		t.Fatalf("start: %d %s", resp.StatusCode, out)
	}
	// Reviewing again shows the new text, and that one can start.
	_, p2 := s.preview(t, map[string]any{"skill": id + "/tdd"})
	if !strings.Contains(p2.Content, "text edited after the review") {
		t.Fatalf("second preview = %s", p2.Content)
	}
	if resp, out := s.start(t, p2.BriefingID); resp.StatusCode != http.StatusCreated {
		t.Fatalf("start after re-review: %d %s", resp.StatusCode, out)
	}
}

func TestRemovedSkillAfterReviewBlocksTheLaunch(t *testing.T) {
	s := skillLaunchServer(t)
	id, dir := s.addSkillFolder(t, "Team skills", "tdd", "x")
	_, p := s.preview(t, map[string]any{"skill": id + "/tdd"})
	if err := os.Remove(filepath.Join(dir, "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if resp, out := s.start(t, p.BriefingID); resp.StatusCode != http.StatusConflict || !strings.Contains(string(out), "method_unavailable") {
		t.Fatalf("start: %d %s", resp.StatusCode, out)
	}
}

func TestADirectionIsBoundedAndFlagged(t *testing.T) {
	s := skillLaunchServer(t)
	_, p := s.preview(t, map[string]any{"direction": strings.Repeat("d", briefing.MaxDirectionRunes+10)})
	if !p.CanStart || !p.DirectionCut || !p.Truncated {
		t.Fatalf("preview = %+v", p)
	}
	code, _ := s.preview(t, map[string]any{"direction": strings.Repeat("d", briefing.MaxDirectionRunes*2+1)})
	if code != http.StatusBadRequest {
		t.Fatalf("absurd direction: %d", code)
	}
}

func TestDirectionAndSkillTextAreNotShellSyntax(t *testing.T) {
	s := skillLaunchServer(t)
	id, _ := s.addSkillFolder(t, "Team skills", "tdd", "run `rm -rf /` $(whoami) && echo hi")
	hostile := "'; touch /tmp/pwned; echo \"$HOME\" `id`"
	_, p := s.preview(t, map[string]any{"skill": id + "/tdd", "direction": hostile})
	if !p.CanStart {
		t.Fatalf("preview = %+v", p)
	}
	// The text lives only in the briefing file; the command line carries the path.
	joined := strings.Join(p.Command, "\x00")
	if strings.Contains(joined, "pwned") || strings.Contains(joined, "rm -rf") || strings.Contains(p.Opener, "pwned") {
		t.Fatalf("operator or skill text reached the argv: %q", p.Command)
	}
	if !strings.Contains(p.Content, hostile) {
		t.Fatal("the direction was altered on its way into the briefing")
	}
}

func TestASkillFoundInTheProjectCanBeChosenAndTheReviewShowsItsOriginAndNotes(t *testing.T) {
	s := skillLaunchServer(t)
	if err := s.reg.SetDiscovery(skillsource.Discovery{Project: true}); err != nil {
		t.Fatal(err)
	}
	skillDir := filepath.Join(s.dir, ".claude", "skills", "publish")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: publish\n---\nOpen it with gh issue create.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, p := s.preview(t, map[string]any{"skill": "project-claude/publish"})
	if code != http.StatusOK || !p.CanStart || p.Method == nil {
		t.Fatalf("preview = %d %+v", code, p)
	}
	if p.Method.Origin != "project" || len(p.Method.Notes) != 1 || !strings.Contains(p.Method.Notes[0], "GitHub") {
		t.Fatalf("method = %+v", p.Method)
	}
	if !strings.Contains(p.Content, "Open it with gh issue create.") {
		t.Fatal("the skill text is not in the briefing")
	}
	// Turning the place off after the review ends in a conflict, like a disabled source.
	if err := s.reg.SetDiscovery(skillsource.Discovery{}); err != nil {
		t.Fatal(err)
	}
	if resp, out := s.start(t, p.BriefingID); resp.StatusCode != http.StatusConflict || !strings.Contains(string(out), "method_unavailable") {
		t.Fatalf("start: %d %s", resp.StatusCode, out)
	}
}
