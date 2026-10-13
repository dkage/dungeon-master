package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/dkage/dungeon-master/internal/quests"
)

func questByNumber(t *testing.T, out []byte, n int) quests.Quest {
	t.Helper()
	for _, q := range decodeResult(t, out).Quests {
		if q.Number == n {
			return q
		}
	}
	t.Fatalf("#%d not in %s", n, out)
	return quests.Quest{}
}

func TestQuestsCarryEffortPriorityAndCreature(t *testing.T) {
	e := startServer(t, nil)
	e.gh.set([]quests.Quest{q(1, "effort:moderate", "priority:high"), q(2, "size/S"), q(3, "bug")}, nil)

	_, out := e.quests("")
	one := questByNumber(t, out, 1)
	if one.Effort != "moderate" || one.EffortSource != "label" || one.Priority != "high" || quests.CreatureTier(one.Creature) != "moderate" {
		t.Fatalf("#1: %+v", one)
	}
	if want := quests.CreatureFor("moderate", "acme/widgets", 1); one.Creature != want {
		t.Fatalf("#1 creature %s, want the stable choice %s", one.Creature, want)
	}
	if three := questByNumber(t, out, 3); three.Effort != "" || three.Creature != "" {
		t.Fatalf("#3 has no effort label and must stay unknown: %+v", three)
	}

	// Map the team's own label to a tier.
	resp, body := e.req("PUT", "/api/campaigns/"+e.campaign+"/effort-labels", map[string]any{"labels": map[string]string{"size/s": "low"}}, nil)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), `"effortLabels":{"size/s":"low"}`) {
		t.Fatalf("mapping: %d %s", resp.StatusCode, body)
	}
	_, out = e.quests("")
	if two := questByNumber(t, out, 2); two.Effort != "low" || two.EffortSource != "mapping" {
		t.Fatalf("#2 after mapping: %+v", two)
	}

	// A local override wins over the labels and is cleared with an empty tier.
	path := "/api/campaigns/" + e.campaign + "/quests/1/effort"
	if resp, body := e.req("PUT", path, map[string]any{"tier": "extreme"}, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("override: %d %s", resp.StatusCode, body)
	}
	_, out = e.quests("")
	if o := questByNumber(t, out, 1); o.Effort != "extreme" || o.EffortSource != "override" || quests.CreatureTier(o.Creature) != "extreme" {
		t.Fatalf("#1 after override: %+v", o)
	}
	e.req("PUT", path, map[string]any{"tier": ""}, nil)
	_, out = e.quests("")
	if o := questByNumber(t, out, 1); o.Effort != "moderate" || o.Creature != one.Creature {
		t.Fatalf("#1 after clearing the override should get its first creature back: %+v", o)
	}
}

func TestQuestDetailCarriesEffort(t *testing.T) {
	e := startServer(t, nil)
	e.gh.detail = quests.Detail{Quest: quests.Finish(quests.Quest{Title: "Fix it", Labels: []quests.Label{{Name: "effort:high"}, {Name: "effort:low"}}})}
	_, out := e.req("GET", "/api/campaigns/"+e.campaign+"/quests/42", nil, nil)
	var d quests.Detail
	if err := json.Unmarshal(out, &d); err != nil {
		t.Fatal(err)
	}
	if d.Effort != "" || len(d.EffortConflict) != 2 || d.Creature != "" {
		t.Fatalf("conflicting labels must leave the tier unknown and say why: %s", out)
	}
}

func TestEffortEndpointsValidate(t *testing.T) {
	e := startServer(t, nil)
	base := "/api/campaigns/" + e.campaign
	bad := []struct {
		method, path string
		body         any
		status       int
	}{
		{"PUT", base + "/effort-labels", map[string]any{"labels": map[string]string{"size/s": "huge"}}, http.StatusBadRequest},
		{"PUT", base + "/effort-labels", map[string]any{"labels": map[string]string{"": "low"}}, http.StatusBadRequest},
		{"PUT", base + "/effort-labels", map[string]any{"mapping": map[string]string{}}, http.StatusBadRequest},
		{"PUT", base + "/quests/1/effort", map[string]any{"tier": "huge"}, http.StatusBadRequest},
		{"PUT", base + "/quests/0/effort", map[string]any{"tier": "low"}, http.StatusBadRequest},
		{"PUT", base + "/quests/abc/effort", map[string]any{"tier": "low"}, http.StatusBadRequest},
		{"PUT", "/api/campaigns/cnope/effort-labels", map[string]any{"labels": map[string]string{}}, http.StatusNotFound},
		{"PUT", "/api/campaigns/cnope/quests/1/effort", map[string]any{"tier": "low"}, http.StatusNotFound},
	}
	for _, b := range bad {
		if resp, out := e.req(b.method, b.path, b.body, nil); resp.StatusCode != b.status {
			t.Errorf("%s %s %v: %d %s", b.method, b.path, b.body, resp.StatusCode, out)
		}
	}
	// Writes go through the guard like every other write.
	noAuth := func(r *http.Request) { r.Header.Del("Authorization") }
	if resp, _ := e.req("PUT", base+"/quests/1/effort", map[string]any{"tier": "low"}, noAuth); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("without auth: %d", resp.StatusCode)
	}
	textBody := func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }
	if resp, _ := e.req("PUT", base+"/effort-labels", map[string]any{"labels": map[string]string{}}, textBody); resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("non-JSON write: %d", resp.StatusCode)
	}
}
