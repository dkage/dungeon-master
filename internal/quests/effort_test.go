package quests

import (
	"slices"
	"testing"

	"github.com/dkage/dungeon-master/internal/campaign"
)

func labels(names ...string) []Label {
	out := make([]Label, len(names))
	for i, n := range names {
		out[i] = Label{Name: n}
	}
	return out
}

func TestResolveEffort(t *testing.T) {
	mapping := map[string]string{"size/s": EffortLow, "size/xl": EffortExtreme, "bogus": "huge"}
	cases := []struct {
		name     string
		labels   []Label
		override string
		tier     string
		source   string
		conflict []string
	}{
		{"default label", labels("bug", "effort:moderate"), "", EffortModerate, "label", nil},
		{"case and spaces", labels(" Effort:HIGH "), "", EffortHigh, "label", nil},
		{"mapped label", labels("Size/S"), "", EffortLow, "mapping", nil},
		{"override wins", labels("effort:trivial", "effort:high"), EffortExtreme, EffortExtreme, "override", nil},
		{"no label is unknown", labels("bug", "feature"), "", "", "", nil},
		{"unknown tier word ignored", labels("effort:huge"), "", "", "", nil},
		{"mapping to a bad tier ignored", labels("bogus"), "", "", "", nil},
		{"bad override ignored", labels("effort:low"), "massive", EffortLow, "label", nil},
		{"agreeing labels are fine", labels("effort:low", "size/s"), "", EffortLow, "label", nil},
		{"two tiers conflict", labels("effort:low", "effort:high"), "", "", "", []string{"effort:low", "effort:high"}},
		{"label and mapping conflict", labels("size/xl", "effort:trivial"), "", "", "", []string{"size/xl", "effort:trivial"}},
		{"title is never read", nil, "", "", "", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := ResolveEffort(c.labels, mapping, c.override)
			if e.Tier != c.tier || e.Source != c.source || !slices.Equal(e.Conflict, c.conflict) {
				t.Fatalf("got %+v, want tier %q source %q conflict %v", e, c.tier, c.source, c.conflict)
			}
		})
	}
}

func TestResolvePriority(t *testing.T) {
	if p, c := ResolvePriority(labels("bug")); p != "normal" || c != nil {
		t.Errorf("no label: %s %v", p, c)
	}
	if p, _ := ResolvePriority(labels("Priority:Urgent")); p != "urgent" {
		t.Errorf("urgent: %s", p)
	}
	if p, _ := ResolvePriority(labels("priority:critical")); p != "normal" {
		t.Errorf("unknown level: %s", p)
	}
	if p, c := ResolvePriority(labels("priority:low", "priority:high")); p != "normal" || len(c) != 2 {
		t.Errorf("conflict: %s %v", p, c)
	}
}

func TestCreatureForIsStableAndInTier(t *testing.T) {
	for _, tier := range EffortTiers {
		seen := map[string]bool{}
		for n := 1; n <= 200; n++ {
			c := CreatureFor(tier, "acme/app", n)
			if CreatureTier(c) != tier {
				t.Fatalf("%s #%d got %q from another tier", tier, n, c)
			}
			if again := CreatureFor(tier, "acme/app", n); again != c {
				t.Fatalf("not stable: %s then %s", c, again)
			}
			seen[c] = true
		}
		if len(seen) != len(Roster[tier]) {
			t.Errorf("%s: 200 issues used only %v", tier, seen)
		}
	}
	if CreatureFor("", "acme/app", 1) != "" || CreatureFor("huge", "acme/app", 1) != "" {
		t.Error("unknown tier must give no creature")
	}
}

// The seed is fixed by the roster document, so known inputs must keep their
// creature across releases. The expected values were computed outside Go from
// the documented formula.
func TestCreatureForMatchesTheDocumentedSeed(t *testing.T) {
	cases := []struct {
		tier, repo string
		n          int
		want       string
	}{
		{EffortTrivial, "acme/app", 1, "cave-bat"},
		{EffortModerate, "acme/app", 42, "crypt-ghoul"},
		{EffortModerate, "acme/other", 42, "skeleton-guard"},
		{EffortExtreme, "dkage/dungeon-master", 28, "crypt-lich"},
	}
	for _, c := range cases {
		if got := CreatureFor(c.tier, c.repo, c.n); got != c.want {
			t.Errorf("%s %s#%d: got %s want %s", c.tier, c.repo, c.n, got, c.want)
		}
	}
}

func TestAnnotate(t *testing.T) {
	c := campaign.Campaign{
		ID: "c1", Repo: "acme/app",
		EffortLabels:    map[string]string{"size/s": EffortLow},
		EffortOverrides: map[int]string{3: EffortHigh},
	}
	in := []Quest{
		{Number: 1, Labels: labels("effort:trivial")},
		{Number: 2, Labels: labels("size/s")},
		{Number: 3, Labels: labels("effort:trivial")},
		{Number: 4, Labels: labels("bug")},
		{Number: 5, Labels: labels("effort:low", "effort:high", "priority:urgent")},
	}
	out := Annotate(in, c)
	if in[0].Effort != "" || in[0].Creature != "" {
		t.Fatal("Annotate changed its input, which may be cached")
	}
	if want := CreatureFor(EffortTrivial, "acme/app", 1); out[0].Creature != want {
		t.Errorf("#1 creature %s, want the stable choice %s", out[0].Creature, want)
	}
	if out[1].Effort != EffortLow || out[1].EffortSource != "mapping" || CreatureTier(out[1].Creature) != EffortLow {
		t.Errorf("#2 mapped to low, so its creature is from the low tier: %+v", out[1])
	}
	if out[2].Effort != EffortHigh || out[2].EffortSource != "override" || CreatureTier(out[2].Creature) != EffortHigh {
		t.Errorf("#3 override: %+v", out[2])
	}
	if out[3].Effort != "" || out[3].Creature != "" {
		t.Errorf("#4 has no effort and must get no creature: %+v", out[3])
	}
	if out[4].Effort != "" || len(out[4].EffortConflict) != 2 || out[4].Creature != "" || out[4].Priority != "urgent" {
		t.Errorf("#5 conflict: %+v", out[4])
	}
}

func TestAnnotateWithoutARepoUsesTheCampaign(t *testing.T) {
	a := Annotate([]Quest{{Number: 7, Labels: labels("effort:high")}}, campaign.Campaign{ID: "one"})
	if a[0].Creature == "" || CreatureTier(a[0].Creature) != EffortHigh {
		t.Fatalf("no creature for a campaign without a repository: %+v", a[0])
	}
}
