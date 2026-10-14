---
status: planned
milestone: M5b
github_issues: [43, 49, 50, 51, 66]
tracking: in-gh-issues
---

# Skills and launch methods

Registered sources are the primary model. A user adds a folder or Git repository globally, then enables/maps sources
per campaign in local user state. GitHub issues remain the work tracker. Skills guide a session's method; their own
tracker assumptions are compatibility questions, not an alternative source of issue truth. v1 uses Claude Code.

## 1. Sources and project mappings

A source has stable identity, a display name, kind (local folder or Git repository), location, enabled state and explicit
precedence. Git sources have an exact revision; the app keeps the checkout/cache in its own state directory, shows the
pin, and updates only on request. Registering a source does not execute its scripts or install an agent plugin.

Each campaign chooses enabled sources and order, with inherited defaults and visible overrides. Nothing is mirrored
into a user's repository or written to their GitHub merely to make discovery work. The user can remove a registration
without deleting a folder they own. Failed refresh keeps the previous verified catalog and explains its age/error.

Optional discovery of project `.claude/skills`/`.agents/skills`, user folders and installed plugins supplements the
registered catalog. Each place is off until the user turns it on in Settings; registered sources come first, so they win
a name, and every entry says where it was found. Plugins are read from Claude's list of installed plugins and confined
to its plugins folder. Distinguish where a skill was found from whether Claude can invoke it natively. Bound scanning,
file size and supporting-file access; reject symlink escapes outside registered roots and skip malformed entries.

## 2. Identity and selection

Catalog entries include source-qualified identity, name, description, source path and revision, manual invocation
metadata, and delivery availability/reason. Names collide predictably: source order chooses an automatic suggestion,
while an explicit source-qualified choice remains pinned. Disabling that source must not silently substitute another.

The picker groups Plan / Do / Check, offers None, displays source provenance, and lets the operator override the
suggestion. `type:research`, `type:prototype`, `type:decision` and `type:task` describe how to work; they never make an
issue ready. A bug category can suggest diagnosis, but only if an enabled compatible skill exists. Do not hard-code
four required role bindings or presume an upstream catalog has a particular skill/count at an unspecified revision.

## 3. Discovery and invocation are separate

The M1 probe established that a known user-invocable Claude skill can run from a positional slash opener. It did not
prove that a skill cached from a Git repository is natively registered. #66 records a real-CLI delivery probe for
project, user, plugin and app-managed external sources before #50 promises their launch behavior.

Use native invocation only where verified. Otherwise, choose and verify an explicit read-by-path briefing method for
external sources, with supporting files resolved relative to their source directory and bounded agent read access. This
is Claude-specific delivery behind the existing Launcher seam, not a generic adapter platform. The briefing for a skill
launch carries the skill's instructions and its absolute directory on this machine, so the agent can open the templates,
examples and validators that ship beside it. Unsupported sources remain visible with a reason. Do not silently
install/mirror skills, execute package scripts or discard the operator's chosen source to make a slash command succeed.

## 4. Launch review and task context

For every launch, keep the task briefing, issue/repository identity, dependency context, repository instruction
pointers, operator direction and existing safety/pace instructions. Adding a method augments that context; it does not
skip it because the skill might fetch GitHub itself. Bound context and show truncation; never claim missing blocker
results are known.

Review shows the exact method/source pin, delivery, directory, opener and briefing. Revalidate source identity and
availability, issue readiness and tree occupancy before launch. If the source, method or work facts change, invalidate
the reviewed plan and explain why. Preserve the reviewed context rather than silently composing different instructions.
Issue text is data; trusted user-selected skills are method instructions, subject to repository instructions and the
operator's authorized scope. A method never grants permission to publish issues, switch trackers or change claims.

## 5. External sourcing and ownership

Offer Matt Pocock's MIT skills as an opt-in pinned source, with license notice and existing credits; no content bundled
silently into the binary. Inspect the chosen revision for capabilities and tracker contracts: a read-only look at each
skill's text and files produces compatibility notes (it mentions creating GitHub issues or pull requests, keeps tickets
in local files, asks for setup files in the repository, ships scripts). The notes are hints shown in Settings and in the
launch review; they never block a launch and are not a verdict. Reuse unmodified methods where compatible; adapt only a
demonstrated gap, preserving notice and credits and documenting the changes.

Tracker conventions are independent of engineering methods. Provide neutral GitHub facts and explicit boundaries in
our launch context. An upstream method requiring repository setup files or a file tracker is unavailable for that
workflow until a verified non-invasive delivery or a narrowly scoped adaptation exists. User-approved issue publishing
uses #61 and the issue-shape contract, not automatic skill-driven assignment on every launch.

## 6. UI

Settings: Skill sources (add folder/repository, pin, refresh, reorder, enable and remove), and per-project source
mapping. In Review launch the default is the current prompt; a combo box adds a skill from the sources enabled for the
project, for that issue only. The footer has an Agent select (the hero) and a Method select whose default, "Automatic",
means the suggestion for this issue (or no skill when none applies) and is resolved to a named, source-qualified method
in the review, never at spawn. Below them, "Agents..." and "Skill sources..." open the matching Settings sections, and
"Review launch" is the only button that leads to a launch; an optional "Additional direction" text field sits above
them. Its text is the operator's own instruction: it is sent to the agent verbatim in the briefing, in its own labelled
section apart from the issue text (which stays untrusted data), bounded in length with any truncation shown, and part of
the reviewed plan, so editing it after review invalidates the review. Selecting or typing never launches. The detail
footer: hero configuration, skill suggestion/None, source and availability, then Review launch. Planning sessions can
select the same methods without an issue. Selecting a source, method or model never launches by itself. Presets and
user-editable suggestion rules can follow after the source and invocation contract works.

## 7. Work order

#51 registers/maps sources; #49 discovers and resolves the catalog; #66 verifies delivery and preserves context; #50
adds the picker and launch behavior. #43 is their parent. This is M5b throughout; M5a remains the map milestone. Further
workflow additions are in `docs/design/campaign-loop.md`; the milestone order lives only in `docs/roadmap.md`.

## 8. Verification

Unit tests cover source identity, mapping/precedence, pins, hostile files, unavailable delivery, invocation argv and
unchanged reviewed context. HTTP/browser tests exercise source registration, project mapping, discovery, selection,
refresh failure and launch using a stub agent. A dated manual probe establishes real-CLI delivery support and records
platform/CLI version. Automation must not call real Claude, gh or the network.
