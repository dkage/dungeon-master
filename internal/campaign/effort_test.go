package campaign

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEffortSettingsPersistAndAreCopied(t *testing.T) {
	state := filepath.Join(t.TempDir(), "campaigns.json")
	r := newReg(t, state, &fakeGit{})
	c, err := r.Add(context.Background(), Input{Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if c.EffortLabels == nil || c.EffortOverrides == nil || len(c.EffortLabels) != 0 {
		t.Fatalf("a new campaign has empty, non-nil maps: %+v", c)
	}
	if _, err := r.SetEffortLabels(c.ID, map[string]string{" Size/S ": "low", "size/xl": "extreme"}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.SetEffortOverride(c.ID, 7, "high"); err != nil {
		t.Fatal(err)
	}
	if err := r.RememberCreatures(c.ID, map[int]string{7: "orc-raider"}); err != nil {
		t.Fatal(err)
	}

	got, _ := r.Get(c.ID)
	got.EffortLabels["size/s"] = "extreme" // a copy: must not reach the registry
	got.Creatures[7] = "crypt-lich"

	again := newReg(t, state, &fakeGit{})
	re, ok := again.Get(c.ID)
	if !ok || re.EffortLabels["size/s"] != "low" || re.EffortLabels["size/xl"] != "extreme" ||
		re.EffortOverrides[7] != "high" || re.Creatures[7] != "orc-raider" {
		t.Fatalf("reloaded: %+v", re)
	}
}

func TestRememberCreaturesKeepsTheFirstChoice(t *testing.T) {
	r := newReg(t, "", &fakeGit{})
	c, _ := r.Add(context.Background(), Input{Dir: t.TempDir()})
	_ = r.RememberCreatures(c.ID, map[int]string{1: "cave-bat"})
	_ = r.RememberCreatures(c.ID, map[int]string{1: "dungeon-rat", 2: "cave-slime", 3: ""})
	got, _ := r.Get(c.ID)
	if got.Creatures[1] != "cave-bat" || got.Creatures[2] != "cave-slime" || len(got.Creatures) != 2 {
		t.Fatalf("creatures: %v", got.Creatures)
	}
}

func TestEffortOverrideClears(t *testing.T) {
	r := newReg(t, "", &fakeGit{})
	c, _ := r.Add(context.Background(), Input{Dir: t.TempDir()})
	_, _ = r.SetEffortOverride(c.ID, 3, "low")
	got, err := r.SetEffortOverride(c.ID, 3, "")
	if err != nil || len(got.EffortOverrides) != 0 {
		t.Fatalf("cleared: %+v %v", got.EffortOverrides, err)
	}
	if _, err := r.SetEffortOverride(c.ID, 3, ""); err != nil {
		t.Fatalf("clearing twice is fine: %v", err)
	}
}

func TestEffortSettingsValidation(t *testing.T) {
	r := newReg(t, "", &fakeGit{})
	c, _ := r.Add(context.Background(), Input{Dir: t.TempDir()})
	var ve ValidationError
	for name, m := range map[string]map[string]string{
		"empty label": {"  ": "low"},
		"control":     {"size\x00s": "low"},
		"too long":    {strings.Repeat("x", 51): "low"},
		"too many":    many(101),
		"collision":   {"Size/S": "low", " size/s ": "high"},
	} {
		if _, err := r.SetEffortLabels(c.ID, m); !errors.As(err, &ve) {
			t.Errorf("%s: want a validation error, got %v", name, err)
		}
	}
	if _, err := r.SetEffortOverride(c.ID, 0, "low"); !errors.As(err, &ve) {
		t.Errorf("number 0: %v", err)
	}
	if _, err := r.SetEffortLabels("nope", nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown campaign: %v", err)
	}
	if err := r.RememberCreatures("nope", map[int]string{1: "cave-bat"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown campaign: %v", err)
	}
}

func TestEffortSaveFailureKeepsTheOldValue(t *testing.T) {
	dir := t.TempDir()
	state := filepath.Join(dir, "campaigns.json")
	r := newReg(t, state, &fakeGit{})
	c, _ := r.Add(context.Background(), Input{Dir: t.TempDir()})
	_, _ = r.SetEffortOverride(c.ID, 1, "low")
	// Replace the state file with a directory so the atomic write fails.
	if err := os.Remove(state); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(state, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := r.SetEffortOverride(c.ID, 1, "high"); err == nil {
		t.Fatal("expected the save to fail")
	}
	if got, _ := r.Get(c.ID); got.EffortOverrides[1] != "low" {
		t.Fatalf("a failed save must not change the campaign: %v", got.EffortOverrides)
	}
}

func many(n int) map[string]string {
	m := map[string]string{}
	for i := range n {
		m[strings.Repeat("a", 1+i%40)+string(rune('a'+i%26))+string(rune('a'+i/26))] = "low"
	}
	return m
}
