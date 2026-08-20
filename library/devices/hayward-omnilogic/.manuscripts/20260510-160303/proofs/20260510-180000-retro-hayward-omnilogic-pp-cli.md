# Printing Press Retro: hayward-omnilogic

## Session Stats
- API: hayward-omnilogic (Hayward OmniLogic — residential pool/spa cloud automation)
- Spec source: hand-authored internal YAML (no Hayward-published spec; community wrapper `djtimca/omnilogic-api` 0.5.0+ was the reference)
- Scorecard: 50/100 (Grade C) — polish flagged remaining dims as scorer-shape mismatches
- Verify pass rate: 100%
- Phase 5 live acceptance: 17/17 PASS against real Hayward account
- Fix loops: 1 verify auto-fix (3 fixes applied) + 1 polish pass
- Manual code edits: large (the entire `internal/omnilogic/` package + `internal/store/` + every command RunE was hand-built per the SKILL's GraphQL-only carve-out, applied to XML-RPC)
- Features built from scratch: 9 transcendence + 12 absorbed-but-handler-replaced + 4 framework-level (auth, sync, search, sql)

## Findings

### Candidate 1: SKILL's "scaffolder + hand-build" carve-out is GraphQL-only; needs to generalize to other non-REST shapes

- **What happened:** The OmniLogic backend is XML-RPC-over-HTTP with two-stage auth (REST JSON login → token-in-header for XML envelope ops to a single URL distinguished by `<Name>` discriminator). The Printing Press generator targets REST+JSON. The SKILL has a specific GraphQL carve-out ("GraphQL-only APIs: Generate scaffolding only in Phase 2; Build real commands in Phase 3 using a GraphQL client wrapper") but doesn't generalize this pattern to XML-RPC, SOAP, or JSON-RPC. An agent encountering OmniLogic — or any future SOAP/RPC API — has to derive the strategy fresh from research evidence and the GraphQL hint rather than reading a clear "non-REST APIs: hand-build the transport in Phase 3, scaffold everything else" rule.

- **Scorer correct?** N/A — this is a SKILL gap, not a score.

- **Root cause:** `skills/printing-press/SKILL.md` documents the scaffolder pattern only for GraphQL. The broader pattern ("declare auth.type: none, hand-build internal/<api>/, register from root.go") works equally well for any RPC-over-HTTP shape but isn't named.

- **Cross-API check:** Recurs whenever a community-wrapped API has no OpenAPI and the transport is RPC-shaped. Beyond OmniLogic, the same pattern would apply to any SOAP enterprise API, JSON-RPC blockchain or financial APIs, Hayward Salt & Swim or similar pool controllers, and the existing `wrapper-only` catalog entries that have no generation path today (covered by open issue **#870**).

- **Frequency:** subclass:non-REST RPC-over-HTTP. Maybe 1-2 future CLIs per quarter.

- **Fallback if the Printing Press doesn't fix it:** Agents derive the strategy from the GraphQL hint plus general engineering judgment. This worked for OmniLogic but cost extra reasoning time. The reliability of future agents catching the right strategy is moderate — the GraphQL hint sounds GraphQL-specific.

- **Worth a Printing Press fix?** As a doc update, yes — ~5 lines. As a generator change (native XML/RPC body emission + composed auth), no — too narrow.

- **Inherent or fixable:** Fixable via SKILL doc update.

- **Durable fix:** Extend the SKILL's existing GraphQL-only Phase 2/3 guidance into a "non-REST API" subsection covering GraphQL, XML-RPC, SOAP, and JSON-RPC. Pattern: declare `auth.type: none` (or `api_key` for env var support only), let the generator emit scaffolding (Cobra tree, store, MCP, doctor, README/SKILL), hand-build the transport in `internal/<api>/`, replace generated command RunE bodies with calls into the hand-built client, delete unused generator-emitted handler files.

- **Test:** Positive — a fresh agent reading the SKILL knows the scaffolder-plus-hand-build path applies to all non-REST shapes, not just GraphQL. Negative — the SKILL doesn't claim the generator supports native XML/SOAP/JSON-RPC emission.

- **Evidence:** This session. The Explore subagent in Phase 2 explicitly answered "the generator cannot natively handle OmniLogic's shape" and the response framed the path as a generalization of the GraphQL guidance.

- **Related prior retros:** None — first retro on this machine.

### Candidate 2: Live login surfaced JSON number → Go string boundary mismatch; converse of open issue #989

- **What happened:** Hayward's v2 login response returns `"userID": 32583` as a JSON **number**. The hand-built client declared `UserID string` and `encoding/json` failed with `cannot unmarshal number into Go struct field`. Caught only at live login — mock-mode verify and dogfood happily passed against fixtures that didn't exercise this. Fixed inline by typing as `json.Number` and calling `.String()` at the boundary.

- **Scorer correct?** N/A — no scorer flagged this; live testing was the only signal.

- **Root cause:** This is the inverse direction of open issue **#989** (which is about JSON-string-encoded **numbers** leaking through as zero). The same root cause — `encoding/json` is strict on type but silent on direction — manifests in both directions. #989's evidence is "vendor sends string, agent declared float → silent 0." This run's evidence is "vendor sends number, agent declared string → loud unmarshal error." Different failure mode, same boundary class.

- **Cross-API check:** Any time an agent or template defines a Go type from research evidence rather than from an actual response sample, this can flip the wrong way. Specifically — any auth response carrying a numeric user ID (and there are many; #989 already names Binance, Coinbase, Kraken, Stripe; OmniLogic adds Hayward).

- **Frequency:** subclass:auth-response-shapes + any hand-built JSON unmarshal target.

- **Worth a Printing Press fix?** Already filed in different direction (#989). The cheapest expansion is one paragraph + one extra example in #989's body or a comment thread.

- **Durable fix:** The `ExtractNumber` helper proposed in #989 already handles both directions (numeric AND string-encoded inputs). The evidence here is that the helper would have saved one fix loop. Comment on #989 with the converse-direction example so the implementer knows to test both.

- **Test:** Whatever #989 proposes, plus a test case where the response is a JSON number and the Go type is `string` (decode via `json.Number` or coerce-to-string helper).

- **Evidence:** This session — `internal/omnilogic/auth.go` `loginResponse.UserID` was `string`, Hayward returned a number, login failed loudly. Two-line fix once identified.

- **Related prior retros:** Issue **#989** open (P1, `comp:generator + comp:skill`). My evidence aligns with the same root cause from the opposite direction.

### Candidate 3 (dropped at triage): Dead generator-emitted commands when handlers are hand-replaced

- **What happened:** After hand-replacing 12 generated command builders (newHeaterCmd, newEquipmentCmd, etc.) with omni-aware variants, the original generator-emitted command files remained as dead code. Dogfood flagged "13 unregistered commands" until I deleted the source files manually.

- **Why dropped:** Only one named API with evidence (this run). The dead-code is only flagged in the narrow scaffolder-pattern workflow — most CLIs use generator-emitted commands as-is. Fits under Candidate 1's broader SKILL update if mentioned at all; doesn't justify its own finding.

### Candidate 4 (dropped at triage): Internal YAML `types.<name>.fields` shape

- **What happened:** Wrote `fields:` as a map keyed by field name; parser wants a list. Error message was clear; fixed in 30 seconds.

- **Why dropped:** One-time learning curve; testdata/clerk.yaml shows the correct shape. Not recurring friction.

### Candidate 5 (dropped at triage): Scorer-shape mismatches penalize non-REST CLIs

- **What happened:** scorecard's `path_validity` (0/10), `sync_correctness` (2/10), `error_handling` (4/10), `vision`/`workflows`/`insight` (4/10 each) all penalize this CLI's hand-built XML-RPC + typed-Upsert + omnilogic_bridge.go shape. Polish skill explicitly flagged these as "scorer-shape mismatches."

- **Why dropped:** Only one named API with evidence. The polish skill already documents these as known false positives. Filing a separate finding now is wishlist; the next non-REST CLI's retro will reinforce the case with stronger evidence if it's real. (Honestly even then, fixing the scorer to recognize non-REST CLIs without introducing false positives elsewhere is a deep refactor — keeping the scorer calibrated for REST is reasonable.)

### Candidate 6 (dropped at triage): SKILL warned against MinimumNArgs but not MaximumNArgs

- **What happened:** My `why-not-running` and `light show` commands used `cobra.MaximumNArgs(1)`. The verifier's `--dry-run` probe was rejected by Cobra's arg gate before my RunE's `dryRunOK(flags)` short-circuit could fire. Same failure mode the SKILL already warns about with MinimumNArgs.

- **Why dropped:** Open issues **#923** ("Hand-written novel command RunE often missing dryRunOK + IsVerifyEnv guard, tanking verify pass rate") and **#965** ("skill: verify-friendly RunE template misleads agents on multi-positional commands, breaks dogfood error_path") already cover this exact failure class. Adding "and MaximumNArgs" is a one-word edit to the SKILL once those land. Not worth a separate filing.

## Prioritized Improvements

### P2 — Medium priority

| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| 1 | Generalize the GraphQL-only scaffolder-and-hand-build guidance to all non-REST shapes | skill | subclass:non-REST RPC-over-HTTP (~1-2 CLIs/quarter) | Moderate — current GraphQL hint sounds GraphQL-specific | small (doc update, 5-10 lines) | None — doc-only change cannot hurt REST CLIs |

### P3 — Low priority (informational)

| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| 2 | Add inverse-direction evidence to JSON-encoded number/string helper (#989) | generator + skill | subclass:auth-response-shapes | Moderate — silently fails until live | small (comment on #989) | None — extends existing P1 issue |

### Skip
None — Phase 3 produced 2 Do candidates and the rest were dropped at Phase 2.5 triage.

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| Dead generator-emitted command files after handler replacement | When hand-built commands replace generated ones, dogfood flags the originals as "unregistered" | unproven-one-off |
| Internal YAML `fields` map-vs-list shape | One-off learning curve; error message was clear | iteration-noise |
| Scorer-shape mismatches on non-REST CLIs | scorecard penalizes path_validity, sync_correctness, error_handling, vision/workflows/insight on hand-built non-REST CLIs | unproven-one-off (only this CLI; polish skill already documents as known false positive) |
| `cobra.MaximumNArgs` extends SKILL's MinimumNArgs warning | Same failure mode (arg gate fires before RunE dry-run short-circuit) for MaximumNArgs | duplicate — already covered by open issues #923 and #965 |

## Work Units

### WU-1: Generalize the SKILL's non-REST scaffolder-and-hand-build guidance (from Finding 1)
- **Priority:** P2
- **Component:** skill
- **Goal:** Future agents printing a non-REST API (XML-RPC, SOAP, JSON-RPC) read the SKILL and know immediately that the pattern is "scaffold via the generator, hand-build the transport," not just for GraphQL.
- **Target:** `skills/printing-press/SKILL.md` — extend the existing GraphQL-only guidance in Phase 2 / Phase 3 to a named "non-REST APIs" subsection.
- **Acceptance criteria:**
  - positive test: a fresh read of the SKILL finds explicit guidance for XML-RPC / SOAP / JSON-RPC alongside the existing GraphQL note, including: declare `auth.type: none` or `api_key` for scaffolding; hand-build the transport in `internal/<api>/`; replace generated command RunE bodies; delete unused generator-emitted handler files.
  - negative test: the SKILL does not claim the generator natively emits XML body construction, XML response parsing, or composed multi-stage auth.
- **Scope boundary:** Does NOT propose adding XML body emission or composed-auth templates to the generator. Doc update only. The generator stays REST+JSON; the SKILL just stops sounding GraphQL-specific.
- **Dependencies:** None.
- **Complexity:** small (5-10 lines of SKILL text).
- **Comment instead of file:** Open issue **#870** ("Generator: wrapper-only catalog entries have no end-to-end generation path") covers the broader "no end-to-end path for non-REST" gap. Suggest commenting on #870 with the OmniLogic evidence rather than opening a separate issue — #870's Option B already proposes a SKILL doc update for the same class.

### WU-2: Reinforce #989 with the converse direction (from Finding 2)
- **Priority:** P3
- **Component:** generator + skill (same as #989's existing labels)
- **Goal:** Implementer of the `ExtractNumber` helper (#989) tests both directions — JSON number → Go string AND JSON string → Go number — and the helper documents both cases.
- **Target:** Comment on open issue #989.
- **Acceptance criteria:** Comment on #989 names the OmniLogic case (Hayward `userID` returned as JSON number; Go struct declared as string; loud unmarshal error rather than silent failure). Reinforces the case-for landing the helper.
- **Scope boundary:** Does NOT propose a new issue. Comment-only on #989.
- **Complexity:** small (one comment).

## Anti-patterns
- The novel-features subagent worked well; the structured table output flowed cleanly into the manifest. Keep this pattern.
- Spawning the Phase 2 Explore subagent before committing to a generation strategy paid off — it answered "can the generator do this natively" deterministically in ~600 words, which saved me from a multi-hour discovery of the same answer mid-Phase-3.
- Live dogfood with real credentials caught one real bug (`userID` JSON number/string) that mock-mode verify and structural shipcheck both missed. The Phase 5 contract of "Full dogfood is the recommended default when credentials are available" justified itself.

## What the Printing Press Got Right
- The scaffolder pattern is genuinely powerful: even for a CLI where ~80% of the code is hand-built (transport, operations, store, novel features), the generator's scaffolding (Cobra tree, MCP cobratree walker, doctor, README/SKILL templates, agent_context, helpers) was a big head start. Without it the CLI would still ship; with it the CLI ships hours faster and matches the agent-native conventions of every other printed CLI automatically.
- The polish skill's "scorer-shape mismatch" classification is honest engineering. It distinguished structural quality from heuristic-pleasing and didn't propose busywork to lift Grade D to Grade A on a CLI that was functionally excellent. The `further_polish_recommended: no` verdict was correct.
- The Phase 5.6 promotion gate + JSON marker contract worked: when live dogfood passed, replacing `phase5-skip.json` with `phase5-acceptance.json` immediately enabled `publish validate` to pass without manual intervention.
- The publish skill's dedup scan caught both relevant prior issues (#870 wrapper-only, #989 JSON number/string boundary) cleanly. The "comment instead of file" branch is the right outcome when the open issue tracker already covers the territory.
