package quests

import (
	"crypto/sha256"
	"encoding/binary"
	"slices"
	"strconv"
	"strings"

	"github.com/dkage/dungeon-master/internal/campaign"
)

// Effort tiers, smallest first. An empty tier means the effort is unknown.
const (
	EffortTrivial  = "trivial"
	EffortLow      = "low"
	EffortModerate = "moderate"
	EffortHigh     = "high"
	EffortExtreme  = "extreme"
)

// EffortTiers lists the tiers in order.
var EffortTiers = []string{EffortTrivial, EffortLow, EffortModerate, EffortHigh, EffortExtreme}

// ValidTier reports whether t is one of the five tiers.
func ValidTier(t string) bool { return slices.Contains(EffortTiers, t) }

// Priorities, lowest first. No label means normal.
var Priorities = []string{"low", "normal", "high", "urgent"}

const (
	effortPrefix   = "effort:"
	priorityPrefix = "priority:"
)

// Effort is how an issue's tier was worked out.
type Effort struct {
	// Tier is empty when the effort is unknown or the labels disagree.
	Tier string
	// Source is "override", "label" or "mapping"; empty when unknown.
	Source string
	// Conflict lists the labels that name different tiers.
	Conflict []string
}

// ResolveEffort picks an issue's effort tier: a local override first, then
// the default `effort:<tier>` labels and the campaign's own label mapping.
// Two labels naming different tiers are a conflict and leave the tier
// unknown; the title is never used to guess.
func ResolveEffort(labels []Label, mapping map[string]string, override string) Effort {
	if ValidTier(override) {
		return Effort{Tier: override, Source: "override"}
	}
	var found []string
	tiers := map[string]bool{}
	source := ""
	for _, l := range labels {
		name := strings.ToLower(strings.TrimSpace(l.Name))
		if t, ok := strings.CutPrefix(name, effortPrefix); ok && ValidTier(t) {
			found, tiers[t], source = append(found, l.Name), true, pick(source, "label")
			continue
		}
		if t := mapping[name]; ValidTier(t) {
			found, tiers[t], source = append(found, l.Name), true, pick(source, "mapping")
		}
	}
	switch len(tiers) {
	case 0:
		return Effort{}
	case 1:
		for t := range tiers {
			return Effort{Tier: t, Source: source}
		}
	}
	return Effort{Conflict: found}
}

// pick keeps the first source seen, so a default label wins the description
// when a mapped label agrees with it.
func pick(have, next string) string {
	if have != "" {
		return have
	}
	return next
}

// ResolvePriority reads a `priority:<level>` label. No label, or labels that
// disagree, mean normal; the conflicting labels are returned for the detail.
func ResolvePriority(labels []Label) (string, []string) {
	var found []string
	levels := map[string]bool{}
	for _, l := range labels {
		name := strings.ToLower(strings.TrimSpace(l.Name))
		if p, ok := strings.CutPrefix(name, priorityPrefix); ok && slices.Contains(Priorities, p) {
			found, levels[p] = append(found, l.Name), true
		}
	}
	if len(levels) == 1 {
		for p := range levels {
			return p, nil
		}
	}
	if len(levels) > 1 {
		return "normal", found
	}
	return "normal", nil
}

// Roster is the creature IDs for each tier (docs/design/creature-roster.md).
// Adding a creature is fine: stored choices keep their creature, and only
// issues seen for the first time can get the new one.
var Roster = map[string][]string{
	EffortTrivial:  {"cave-bat", "cave-slime", "dungeon-rat"},
	EffortLow:      {"cave-beetle", "cave-spider", "kobold-scout"},
	EffortModerate: {"crypt-ghoul", "goblin-scout", "skeleton-guard"},
	EffortHigh:     {"dungeon-warlock", "orc-raider", "stone-troll"},
	EffortExtreme:  {"crypt-lich", "ember-dragon", "many-eyed-stone-watcher"},
}

// CreatureFor chooses a creature for an issue from its tier's roster: the
// first 64 bits of SHA-256("creature-assignment-v1|owner/repo#123") modulo
// the sorted IDs. The same repository and number always give the same one.
func CreatureFor(tier, repo string, number int) string {
	ids := slices.Clone(Roster[tier])
	if len(ids) == 0 {
		return ""
	}
	slices.Sort(ids)
	sum := sha256.Sum256([]byte("creature-assignment-v1|" + repo + "#" + strconv.Itoa(number)))
	return ids[binary.BigEndian.Uint64(sum[:8])%uint64(len(ids))]
}

// CreatureTier returns the tier a creature belongs to, or "".
func CreatureTier(id string) string {
	for t, ids := range Roster {
		if slices.Contains(ids, id) {
			return t
		}
	}
	return ""
}

// Annotate fills each quest's effort, priority and creature from its labels
// and the campaign's local settings. It returns new slices, never touching
// the input (which may be cached), plus the creature choices made for the
// first time, which the caller should remember.
func Annotate(qs []Quest, c campaign.Campaign) ([]Quest, map[int]string) {
	out := make([]Quest, len(qs))
	picks := map[int]string{}
	for i, q := range qs {
		out[i] = annotate(q, c, picks)
	}
	return out, picks
}

// AnnotateDetail is Annotate for one opened issue.
func AnnotateDetail(d Detail, c campaign.Campaign) (Detail, map[int]string) {
	picks := map[int]string{}
	d.Quest = annotate(d.Quest, c, picks)
	return d, picks
}

func annotate(q Quest, c campaign.Campaign, picks map[int]string) Quest {
	e := ResolveEffort(q.Labels, c.EffortLabels, c.EffortOverrides[q.Number])
	q.Effort, q.EffortSource, q.EffortConflict = e.Tier, e.Source, e.Conflict
	q.Priority, q.PriorityConflict = ResolvePriority(q.Labels)
	q.Creature = ""
	if e.Tier == "" {
		return q
	}
	stored := c.Creatures[q.Number]
	switch {
	case stored != "" && CreatureTier(stored) == e.Tier:
		q.Creature = stored
	case stored != "":
		// The tier changed since the choice was stored: follow the tier, and
		// keep the stored choice for when it changes back.
		q.Creature = CreatureFor(e.Tier, repoKey(c), q.Number)
	default:
		q.Creature = CreatureFor(e.Tier, repoKey(c), q.Number)
		picks[q.Number] = q.Creature
	}
	return q
}

// repoKey is the repository identity the creature choice is seeded with.
func repoKey(c campaign.Campaign) string {
	if c.Repo != "" {
		return c.Repo
	}
	return "local/" + c.ID
}
