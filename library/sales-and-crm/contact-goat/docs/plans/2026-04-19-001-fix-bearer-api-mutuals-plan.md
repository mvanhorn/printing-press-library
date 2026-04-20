---
title: "fix: Preserve Happenstance bearer API mutuals + affinity on all bearer-path commands"
type: fix
status: active
date: 2026-04-19
---

# fix: Preserve Happenstance bearer API mutuals + affinity on all bearer-path commands

## Overview

The Happenstance bearer API (`POST /v1/search` + `GET /v1/search/{id}`) returns a top-level `mutuals[]` array naming the user's 1st-degree bridges, plus per-result `mutuals[].index + affinity_score` pointing into that array. contact-goat currently unmarshals none of it. Every bearer result surfaces as a bare `{name, title, company}` with a generic "Happenstance bearer" rationale and a score derived only from `weighted_traits_score`. Users cannot tell which friend bridges to which person, nor how strong the signal actually is.

This plan captures that data in the API types, carries it through the normalizer, and renders it on every bearer-surface command.

## Problem Frame

The contact-goat CLI is positioned as a warm-intro tool. On the bearer path it fails at that positioning: the API gives us a graph (bridges + affinity), we throw the graph away and print a list of names. A real-session trace on 2026-04-19 (Tesla coverage query) returned 25 hits with no bridge info via the CLI; the same query against the raw bearer API returned full `mutuals` + `affinity_score` data. The disconnect is a pure client-side bug, not an API limitation.

This is discovered in the Tesla session documented in the conversation that produced this plan: LinkedIn's cookie path surfaces mutuals well ("Charly Mwangi + 3 others"), Happenstance's cookie path surfaces referrer rationale, but the bearer fallback silently drops the equivalent metadata.

## Requirements Trace

- R1. Every bearer-surface command response MUST include, for each result, an enumerated list of bridges with name, UUID, and affinity_score.
- R2. The rationale string MUST name the top bridge and affinity score (e.g., `via Jeff Clavier (affinity 104.4)`) instead of the generic `Happenstance bearer`.
- R3. Results MUST be sortable / rankable by max affinity across bridges, with stable fallback to `weighted_traits_score` on zero-affinity hits.
- R4. Zero-affinity hits MUST be distinguishable from strong-affinity hits in both JSON and human-friendly output (tag + sort position).
- R5. Existing cookie-path behavior MUST NOT regress. Only the bearer-path projection changes.
- R6. Commands affected: `coverage`, `hp people`, `prospect`, `warm-intro`, `api hpn search`.

## Scope Boundaries

- Not changing the cookie-surface output shape or rationale text.
- Not redesigning warm-intro's scoring algorithm. That belongs in a separate plan; here we only ensure warm-intro sees affinity data when it exists.
- Not adding new bearer endpoints or pagination controls. `has_more` / `next_page` remain as they are.
- Not modeling every bearer field (e.g., `traits[].evidence` prose, `socials.twitter_url`). Only bridge metadata and the minimum socials needed to render a result cleanly (linkedin_url).
- Not changing `flagshipPerson.MutualCount` semantics. A new field holds bridge detail.

## Context & Research

### Relevant Code and Patterns

- `internal/happenstance/api/types.go` lines 62-83: `SearchEnvelope` + `SearchResult` structs. Neither captures `mutuals` today.
- `internal/happenstance/api/search.go` lines 87-107: `POST /v1/search` + `PollSearch` pipeline. Unmarshaling uses the types above; no request shape change needed.
- `internal/happenstance/api/normalize.go` lines 50-57: `ToClientPerson` maps SearchResult to `client.Person`. Currently drops anything not in the 4-field struct.
- `internal/cli/source_selection.go` lines 361-423 (`ExecuteWithSourceFallback`), 453-491 (`BearerSearchAdapter`): bearer fallback + result normalization plumbing. Adapter calls `ToClientPerson` on each result (line 482).
- `internal/cli/flagship_helpers.go` lines 32-47: `flagshipPerson` struct used by coverage/prospect/warm-intro. Has `MutualCount` (cookie path only) and `Raw` escape hatch but no bridge-detail field.
- `internal/cli/coverage.go` lines 275-290: bearer-row mapping block. The site of the visible bug.
- `internal/cli/prospect.go` lines 128-148: same bug pattern.
- `internal/cli/hp_people.go`: `buildHPPeopleJSON` projection, same bug pattern.
- `internal/cli/warm_intro.go`: bearer row projection, same bug pattern.
- `internal/cli/api_hpn_search.go` lines 290-300: emits a thin `hpnSearchResult` (no bridge fields).

### Institutional Learnings

- None found in `docs/solutions/` for this CLI (directory does not exist yet for contact-goat).

### External References

- Happenstance bearer API response shape verified empirically 2026-04-19 against production: `https://api.happenstance.ai/v1/search` + `GET /v1/search/{id}`. Response includes top-level `mutuals[]`, per-result `mutuals[].index + affinity_score`, `traits[]`, and `socials{linkedin_url, twitter_url, instagram_url, happenstance_url}`.
- No public Happenstance OpenAPI at time of writing; shape is treated as empirical.

## Key Technical Decisions

- **Field placement, API layer:** Add `Mutuals []SearchMutual` at the envelope level and `Mutuals []ResultMutual` + `Socials *SearchSocials` + `Traits []SearchTrait` at the result level. Rationale: the envelope-level list is the source of truth for names; per-result entries only carry `index + affinity_score` and must dereference against it. Colocating both in types.go matches the API shape exactly and keeps the struct grep-able.

- **Field placement, canonical person:** Add `Bridges []client.Bridge` to `client.Person`. Each Bridge holds `Name, HappenstanceUUID, AffinityScore`. Rationale: canonical Person is what every renderer consumes; putting bridges here means a single change propagates across coverage/prospect/warm-intro/hp-people/api-hpn-search.

- **Self-bridge handling:** The API's top-level mutuals include the user themselves (e.g., `"Matt Van Horn"` at index 3). Results with `mutuals[].index` pointing at the self-entry represent 1st-degree contacts in the user's own synced graph. Tag these as `self_graph` in Bridge; do not present "via Matt Van Horn" as a bridge string. Rationale: the CLI already knows who the current user is (`currentUUID` is plumbed through coverage.go). Filtering self-bridges at the normalizer keeps renderers simple.

- **Rationale string format:** `via <top bridge name> (affinity X.X)` when at least one non-zero affinity bridge exists; `in your synced graph` when only self-bridge hits; `Happenstance bearer (weak signal)` when all bridge affinities are zero. Rationale: matches how the user would describe the relationship aloud and is greppable.

- **Sorting:** Sort bearer-path results by `max(affinity_score)` desc, with `weighted_traits_score` as secondary key. Zero-affinity entries sink below any positive-affinity entry regardless of traits score. Rationale: a score-1.0 Tesla employee with zero graph connection is less useful than a score-0.5 ex-Tesla founder via your strongest bridge.

- **Backwards compat:** `flagshipPerson.MutualCount` stays. New `Bridges []BridgeRef` is additive. JSON consumers that ignore unknown fields are unaffected. Rationale: CLI is user-owned, no external consumers of JSON output that we know of, but additive change is still cheaper than migration.

- **SKILL.md + plugin regen:** Per `printing-press-library/AGENTS.md`, changes under `library/**/internal/cli/**` require running `go run ./tools/generate-skills/main.go` and bumping `plugin/.claude-plugin/plugin.json` patch version. Rationale: repo convention, CI depends on it.

## Open Questions

### Resolved During Planning

- **Q: Should we change warm-intro's ranking to use affinity?** Resolved: no, this plan only ensures warm-intro has affinity available in `client.Person`. Ranking redesign is out of scope.
- **Q: Should we surface `traits[].evidence` prose?** Resolved: no for this plan. `traits` is captured in the API types so it is available to future renderers, but no command prints it in this plan. Keeps the change bounded.
- **Q: Filter out self-bridge entries, or present them as "in your contacts"?** Resolved: present as `in your synced graph` tag and do not list the user by name. The existing `currentUUID` plumbing already lets the normalizer identify the self-entry.

### Deferred to Implementation

- Exact Go type names for new structs (SearchMutual vs BridgeEntry etc.) - pick during implementation, prefer names that already appear elsewhere in the codebase.
- Whether to inline-define Bridge in `client` package or make it a sub-package - decide by looking at how client.Person is currently organized.
- Whether the human-friendly renderer should show all bridges or just the top 1 - decide once the JSON path is working and the human render can be tested.

## High-Level Technical Design

> This illustrates the intended approach and is directional guidance for review, not implementation specification. The implementing agent should treat it as context, not code to reproduce.

Data flow:

```
POST /v1/search (bearer)
       |
       v
GET /v1/search/{id}              <- poll until COMPLETED
       |
       v
SearchEnvelope {
  Mutuals: [                      <- NEW: top-level bridge list
    {index:0, id, name},          <- friends
    {index:3, id, name}           <- self-entry (identified by currentUUID)
  ],
  Results: [
    { Name, CurrentTitle, CurrentCompany,
      WeightedTraitsScore,
      Mutuals: [{index, affinity_score}],   <- NEW
      Socials: {linkedin_url, ...},          <- NEW
      Traits:  [{index, score, evidence}]    <- NEW (captured, not rendered)
    }
  ]
}
       |
       v  ToClientPerson(result, envelopeMutuals, currentUUID)
       |
       v
client.Person {
  Name, Title, Company, Score,
  Bridges: [                      <- NEW, dereferenced + filtered
    {Name:"Jeff Clavier", UUID, AffinityScore:104.38, Kind:"friend"},
  ]
  Socials: {LinkedInURL}
}
       |
       v  per-command projection (coverage/prospect/warm-intro/hp-people/api-hpn-search)
       |
       v
Output row {
  Rationale: "via Jeff Clavier (affinity 104.4)",
  Bridges:   [...],
  Score:     max(Bridges.AffinityScore) || WeightedTraitsScore
}
```

Rationale-string decision table:

| Bridges present | Max affinity | Rationale string |
|---|---|---|
| 1+ friend bridge | > 0 | `via <top bridge name> (affinity X.X)` |
| 1+ friend bridge | = 0 | `Happenstance bearer (weak signal, no graph affinity)` |
| Only self-bridge | n/a | `in your synced graph` |
| No bridges at all | n/a | `Happenstance bearer (no graph match)` |

## Implementation Units

- [ ] **Unit 1: Extend Happenstance bearer API types to capture mutuals, socials, traits**

**Goal:** `SearchEnvelope` and `SearchResult` unmarshal the full bearer response shape including bridges, affinity, socials, and traits. No behavior change yet; just the structs.

**Requirements:** R1

**Dependencies:** None.

**Files:**
- Modify: `internal/happenstance/api/types.go`
- Test: `internal/happenstance/api/search_test.go`

**Approach:**
- Add `SearchMutual` type (envelope-level): `Index int`, `Id string`, `Name string`, `HappenstanceURL string`.
- Add `ResultMutual` type (result-level): `Index int`, `AffinityScore float64`.
- Add `SearchSocials` type: `LinkedInURL`, `TwitterURL`, `InstagramURL`, `HappenstanceURL` (all omitempty).
- Add `SearchTrait` type: `Index int`, `Score float64`, `Evidence string`.
- Extend `SearchEnvelope` with `Mutuals []SearchMutual`.
- Extend `SearchResult` with `Mutuals []ResultMutual`, `Socials *SearchSocials`, `Traits []SearchTrait`, `Summary string`, `Id string`.
- Keep all new fields `omitempty` so existing cookie-path fixtures remain valid.

**Patterns to follow:**
- Existing struct definitions at `internal/happenstance/api/types.go` lines 62-83.
- Tag convention: `json:"snake_case_field,omitempty"`.

**Test scenarios:**
- Happy path: Unmarshal a fixture JSON matching the verified 2026-04-19 Tesla response (top-level `mutuals` with 4 entries, result rows with per-result `mutuals` + `affinity_score` + `socials.linkedin_url`); assert all fields populate.
- Edge case: Unmarshal a response with an empty `mutuals: []` array at both levels; assert no panic and empty slice.
- Edge case: Unmarshal a response where `mutuals` field is absent entirely (older API or cookie path); assert nil slice, no error.
- Edge case: Unmarshal a result row where `mutuals[].affinity_score` is a very small float (e.g., `4.77e-39` per observed response) and a very large float (`245.23`); assert both round-trip without precision loss.

**Verification:**
- Test fixture unmarshal round-trips every new field.
- `go vet ./...` and `go build ./...` clean.
- No existing test fails.

- [ ] **Unit 2: Extend canonical Person with Bridges + Socials; update normalizer**

**Goal:** `client.Person` carries bridge info and linkedin_url. `api.ToClientPerson` dereferences the envelope-level `Mutuals` against each result's `Mutuals[].index`, filters out the self-entry (matched by `currentUUID`), and emits `client.Bridge` entries.

**Requirements:** R1, R3, R5

**Dependencies:** Unit 1.

**Files:**
- Modify: `internal/client/person.go` (or wherever `client.Person` is defined - locate during implementation)
- Modify: `internal/happenstance/api/normalize.go`
- Modify: `internal/happenstance/api/search.go` (if `ToClientPerson` signature needs envelope mutuals + currentUUID passed in)
- Modify: `internal/cli/source_selection.go` (BearerSearchAdapter calls `ToClientPerson`; update call site at line 482)
- Test: `internal/happenstance/api/normalize_test.go`

**Approach:**
- Define `client.Bridge { Name string; HappenstanceUUID string; AffinityScore float64; Kind string }`. Kind is `"friend"` or `"self_graph"`.
- Add `Bridges []Bridge` and `LinkedInURL string` fields on `client.Person`.
- `ToClientPerson` now takes the envelope `Mutuals` slice and `currentUUID` as extra args. Build an index map (`index -> SearchMutual`) once per envelope; for each result, resolve its `Mutuals[].index` to bridge entries; skip / retag any bridge whose `Id` equals `currentUUID`.
- Propagate the extra args through BearerSearchAdapter without changing exported return shape.

**Patterns to follow:**
- Existing ToClientPerson mapping at `internal/happenstance/api/normalize.go` lines 50-57.
- `currentUUID` is already plumbed through `coverage.go`; reuse the same source (Happenstance auth layer or session context).

**Test scenarios:**
- Happy path: Envelope with 3 friend mutuals + 1 self-mutual; result row with two bridges (one friend at affinity 104, one self at affinity 0). Assert `Bridges` contains only the friend bridge with `Kind:"friend"`, and a `self_graph` marker attached separately (field TBD during impl).
- Edge case: Result with empty `Mutuals: []`; assert `Bridges` is empty and no panic.
- Edge case: Result with a single bridge whose `index` points to an entry NOT in the envelope (malformed API response); assert the bridge is dropped silently and a debug log line is emitted.
- Edge case: Envelope with `currentUUID` not matching any top-level mutual; assert no panic, all bridges treated as `friend`.
- Integration scenario: Call BearerSearchAdapter with a captured fixture via a stubbed HTTP transport; assert the returned `[]client.Person` have Bridges populated end-to-end.

**Verification:**
- Normalizer tests pass.
- `client.Person` JSON marshal produces `bridges` and `linkedin_url` fields when populated, omits them when empty.

- [ ] **Unit 3: Update flagshipPerson + coverage bearer-path rendering**

**Goal:** Coverage command surfaces bridges, affinity-aware rationale, and affinity-based sort for bearer rows. `flagshipPerson` gains a `Bridges` field.

**Requirements:** R1, R2, R3, R4, R6

**Dependencies:** Unit 2.

**Files:**
- Modify: `internal/cli/flagship_helpers.go` (lines 32-47 struct def; plus dedup/merge logic if any touches Bridges)
- Modify: `internal/cli/coverage.go` (lines 275-290 bearer mapping block; plus final sort)
- Test: `internal/cli/flagship_helpers_test.go`
- Create: `internal/cli/coverage_test.go`

**Approach:**
- Add `Bridges []BridgeRef` to `flagshipPerson` where `BridgeRef = {Name, HappenstanceUUID, AffinityScore, Kind}`.
- In coverage.go bearer block: populate `row.Bridges` from `p.Bridges`; compute `maxAff := max(p.Bridges[i].AffinityScore)`; set `row.Rationale` per the rationale decision table; set `row.Score = maxAff` when > 0, else fall back to the existing `p.Score` (WeightedTraitsScore).
- After the bearer block and before cross-source dedup, sort bearer-tagged rows by `Score` desc, zero-affinity rows last.
- Ensure cross-source dedup (cookie row + bearer row for same person) merges Bridges from the bearer row into the kept row.

**Patterns to follow:**
- Existing `graphPersonToFlagship` helper in coverage.go.
- Existing dedup/merge logic in `flagship_helpers.go` (whatever merges duplicate rows across sources).

**Test scenarios:**
- Happy path: 3 bearer results with affinities [104, 50, 0]; assert output rationale strings `via Jeff Clavier (affinity 104.4)`, `via Garry Tan (affinity 50.0)`, `Happenstance bearer (weak signal, no graph affinity)` and sort order [104, 50, 0].
- Happy path: 1 bearer result whose only bridge is self; assert rationale `in your synced graph` and sort-wise treated as weak.
- Edge case: Result with zero bridges at all; assert rationale `Happenstance bearer (no graph match)`, sort position after all bridged rows.
- Error path: `ExecuteWithSourceFallback` returns no results (all upstream failed); assert `count:0` JSON with `source_errors` still propagated.
- Integration scenario: Run coverage for a company where cookie path returns 1 row and bearer path returns 3 rows (including 1 duplicate of the cookie row); assert dedup keeps the cookie row but merges bearer Bridges onto it.

**Verification:**
- `contact-goat-pp-cli coverage Tesla --agent` against a recorded fixture reproduces the expected JSON with Bridges + affinity-aware rationale.
- Zero-affinity hits sort last.
- Existing cookie-path coverage test (if any) still passes.

- [ ] **Unit 4: Apply the same bearer-path fix to prospect, hp-people, warm-intro, api-hpn-search**

**Goal:** Bearer-surface projection is consistent across every command that calls BearerSearchAdapter or the bearer path directly. No remaining code path emits `Rationale: "Happenstance bearer"` without bridge detail.

**Requirements:** R2, R6

**Dependencies:** Unit 3 (the rationale decision table + `flagshipPerson.Bridges` shape must be locked in first).

**Files:**
- Modify: `internal/cli/prospect.go` (lines 128-148)
- Modify: `internal/cli/hp_people.go` (buildHPPeopleJSON)
- Modify: `internal/cli/warm_intro.go` (bearer row projection site)
- Modify: `internal/cli/api_hpn_search.go` (lines 290-300; extend `hpnSearchResult` with Bridges)
- Test: `internal/cli/prospect_test.go` (if exists; else create)
- Test: `internal/cli/warm_intro_test.go` (if exists; else create)
- Test: `internal/cli/api_hpn_search_test.go` (if exists; else create)

**Approach:**
- Each file gets the same change: read `p.Bridges`, compute rationale via a shared helper in `flagship_helpers.go` called e.g. `bearerRationale(bridges, selfUUID) string`, set output rationale + score identically.
- Extract the rationale formatter into one helper so the decision table has a single source of truth.
- `api_hpn_search.go`'s `hpnSearchResult` is a thinner struct (no flagshipPerson); extend it with `Bridges []BridgeRef` and `Rationale string`.

**Patterns to follow:**
- The bearer-row mapping pattern established in Unit 3.
- Shared helper placement convention: generic flagship helpers live in `flagship_helpers.go`.

**Test scenarios:**
- Happy path (prospect): Fan-out search with a bearer hit at affinity 50; assert same rationale format as coverage.
- Happy path (warm-intro): Target person with one bearer bridge at affinity 104; assert the candidate output includes the bridge + affinity (warm-intro's own ranking stays unchanged).
- Happy path (hp people): Same as coverage assertions.
- Happy path (api hpn search): hpnSearchResult JSON now includes `bridges` and `rationale` fields.
- Edge case: Each command still handles an empty results array without panicking.

**Verification:**
- No command in `internal/cli/` references the literal `"Happenstance bearer"` string for rationale anymore - grep confirms the only reference is in the shared helper.
- `go test ./...` passes.

- [ ] **Unit 5: Regenerate plugin/skills mirror + bump plugin version**

**Goal:** Repo convention (per `printing-press-library/AGENTS.md`) that any change under `library/**/internal/cli/**` must commit the regenerated `plugin/skills/pp-contact-goat/SKILL.md` plus a semver-patch bump on `plugin/.claude-plugin/plugin.json`.

**Requirements:** R5 (repo-convention compliance; CI dependency)

**Dependencies:** Units 1-4 merged (regen runs against final source).

**Files:**
- Modify: `plugin/.claude-plugin/plugin.json` (version field)
- Modify: `plugin/skills/pp-contact-goat/SKILL.md` (auto-generated)

**Approach:**
- Run `go run ./tools/generate-skills/main.go` from repo root.
- Bump `plugin/.claude-plugin/plugin.json` patch version by 1.
- Commit alongside the feature commit per AGENTS.md commit-style guidance: `chore(plugin): regenerate pp-contact-goat skill + bump to X.Y.Z`.

**Patterns to follow:**
- `printing-press-library/AGENTS.md` "Keeping plugin/skills in sync" section.

**Test scenarios:**
- Not applicable; this is a regen step.

**Verification:**
- `git diff plugin/skills/pp-contact-goat/SKILL.md` shows only changes that reflect the new flags/behavior (if SKILL.md documents bearer-path behavior) or is empty (if SKILL.md does not mention bearer details).
- CI `verify-skills.yml` passes locally via `python .github/scripts/verify-skill/verify_skill.py library/sales-and-crm/contact-goat`.

## System-Wide Impact

- **Interaction graph:** Every command consuming bearer results flows through `BearerSearchAdapter -> ToClientPerson`. Changes in Unit 2 propagate automatically; per-command projections (Units 3-4) catch the remaining surface.
- **Error propagation:** Bearer API 429s and malformed responses are already handled in `ExecuteWithSourceFallback`. New code paths must not swallow errors - e.g., a malformed `mutuals[].index` should log at debug and drop the entry, not fail the request.
- **State lifecycle risks:** None. No persistent state; each request is stateless.
- **API surface parity:** All 5 bearer-path commands must match on rationale format. Single shared helper enforces this.
- **Integration coverage:** Need one end-to-end test per command that exercises the bearer path (stubbed HTTP) and asserts bridge + rationale appear in output. Unit-level tests on the normalizer and flagship helper are insufficient alone.
- **Unchanged invariants:** Cookie-path output unchanged. JSON consumers that ignore unknown fields unaffected. `flagshipPerson.MutualCount` semantics unchanged. Existing `Rationale` field is a string (still a string, just a richer value). `Score` field is a float (still a float, new semantics documented in code).

## Risks & Dependencies

| Risk | Mitigation |
|------|------------|
| Cookie-path projections accidentally get "bearer rationale" formatting | Apply new rationale logic only inside the `if out.UsedSource == SourceAPI` branches; unit tests per command verify cookie path unchanged. |
| `currentUUID` plumbing isn't available in all bearer call sites | Pre-flight audit during Unit 2: grep every `ToClientPerson` call and confirm currentUUID source. If missing, pass `""` and skip self-filtering; add a TODO + test expecting self-bridge tagged as `friend` in that fallback. |
| Malformed bearer response (index out of range) crashes the normalizer | Unit 2 tests cover this; defensive index check in the dereference loop. |
| Score field now mixes affinity and traits-score magnitudes (affinity can be 0-300+, traits is 0-1) | Document clearly in `flagshipPerson` struct doc comment. Downstream consumers already treat Score as relative, not absolute. Cross-source ranking in coverage.go lines ~ranked section still uses source-strength tiers, not raw Score comparisons. |
| SKILL.md regen drift between library copy and plugin copy | Unit 5 is explicit step; CI verify-skills catches residual drift. |
| Plan deploys before affinity scores are validated to mean what we think they mean | Affinity observed in live 2026-04-19 Tesla response: friend-bridge hits showed 104, 71, 62, 55, 48, 24 etc.; self-bridge hits showed 0 or near-zero. Behavior matches "higher = stronger graph signal." Accepting this empirical interpretation without API docs. |

## Documentation / Operational Notes

- Update `library/sales-and-crm/contact-goat/README.md` Coverage section to describe bridge + affinity output (one paragraph).
- Update `library/sales-and-crm/contact-goat/SKILL.md` if it documents bearer-path behavior; per AGENTS.md the verifier checks flags, not field names, so this is discretionary.
- No migration needed. No feature flag. Change is additive for JSON consumers.
- Dogfood: rerun the Tesla coverage query that surfaced this bug and confirm Ira Ehrenpreis rationale reads `via Jeff Clavier (affinity 104.4)`.

## Sources & References

- Live bearer API response capture: 2026-04-19, `/v1/search` id `4d0f3992-2587-47c2-a743-1a4365d6c8f2`, query `"people at Tesla"`.
- Repo source: `/Users/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat`
- Upstream repo: https://github.com/mvanhorn/printing-press-library (via AGENTS.md reference)
- Related convention doc: `/Users/mvanhorn/printing-press-library/AGENTS.md` (SKILL.md + plugin sync section)
