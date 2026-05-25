# Printing Press Retro: art-goat

## Session Stats
- API: art-goat (multi-source aggregator — AIC + NASA APOD)
- Spec source: hand-authored minimal OpenAPI 3.0 for APOD (after AIC OpenAPI 3.1 caused generator pathology)
- Scorecard: 81/100 (Grade A)
- Verify pass rate: 100% (15/15)
- Phase 5 live dogfood: 10/10 pass
- Fix loops: 1 (validate-narrative + verify-skill round)
- Manual code edits: ~15 (Phase 3 hand-authored ~1,500 lines of Go for source layer, contemplative spine, journal)
- Features built from scratch: 7 transcendence commands (sit, today, journal write/stats/search, sources list/sync); 6 deferred to v1.1 via Known Gaps

## Findings

### 1. Generator hangs indefinitely on a valid OpenAPI 3.1 spec (AIC, 103 endpoints, 428 $refs) (template gap / openapi-parser)
- **What happened:** `printing-press generate --spec https://api.artic.edu/api/v1/openapi.json --name art-goat --output ... --force --lenient --validate` consumed 61+ minutes of CPU at 99.8% utilization and produced ZERO files in the output directory. Process was actively running, not blocked on I/O — pure CPU. Killed and pivoted.
- **Scorer correct?** N/A (no scorer involved; generator never reached scoring phase).
- **Root cause:** Unknown without profiler attachment. Hypothesis: recursive `$ref` expansion or N² algorithm triggered by the spec's structure (428 `$ref`s in 164KB). The spec is valid OpenAPI 3.1, downloadable in 200ms, and passes external validators.
- **Cross-API check:** Any large vendor OpenAPI 3.1 spec with deep $ref nesting is a candidate. Met Museum, GitHub (api.github.com/openapi), Stripe (full spec), Linear (full GraphQL-derived spec), Twilio. Direct evidence: AIC (this session). Probable based on size/$ref-density: GitHub OpenAPI, Stripe full spec. Inferred from structural similarity, not directly tested.
- **Frequency:** Plausibly affects ~10-20% of large vendor specs based on $ref-density distribution; rate not measured. Pathology is silent — no progress indicators, no timeout, no log — so an agent doesn't know whether to wait 10 more minutes or kill.
- **Fallback if the Printing Press doesn't fix it:** Agent has to detect the hang and pivot, which is what happened here. Pivot cost in this session: ~1.5 hours wasted + write a minimal hand-authored OpenAPI spec for the primary scaffold. Without familiarity with the Printing Press's expected timing, an agent could plausibly wait the full 10-min Phase 1.7 budget × 6 rounds and never recover.
- **Worth a Printing Press fix?** Yes. The silent-hang failure mode is the worst class of bug — no signal, no recovery, no diagnosis. Even if the underlying parsing is genuinely expensive, the generator should emit progress markers (resources parsed, refs resolved, files written) so an agent can detect "no progress for N seconds" and act.
- **Inherent or fixable:** Fixable. Either the $ref resolver has a real bug (cycle, exponential expansion) or it's working correctly but with extreme inputs. Either way, progress logging + a watchdog timeout would catch the symptom.
- **Durable fix:** Two-layer fix:
  1. Generator emits progress markers to stderr at every phase boundary (`parsing spec`, `resolving refs: N/M`, `emitting resource: <name>`, `writing files: N`). Lets the agent skill add a heartbeat detector with a configurable kill threshold.
  2. Investigate $ref resolution for cycle detection / memoization gaps using the AIC spec as the repro case. The 164KB / 428-$ref spec is small enough to profile end-to-end.
- **Test:**
  - positive: run `printing-press generate --spec aic-openapi.json --name test --output /tmp/test` and verify it either completes within 5 minutes OR emits at least one progress marker every 30 seconds.
  - negative: run against the existing happy-path Petstore/Stripe specs — progress markers should be unobtrusive and not break dry-run/JSON output.
- **Evidence:** Session moment 06:32-07:33 PDT 2026-05-21. Spec preserved at `~/printing-press/.runstate/cli-printing-press-8f659eb0/runs/20260521-062440/research/aic-openapi.json` (164KB) for reproducible profiling.
- **Related prior retros:** None (first art-goat run; no aligned/contradicting matches in `~/printing-press/manuscripts/*/proofs/*retro*.md`).
- **Step G case-against:** "AIC's spec is a one-off pathological input — maybe it actually has malformed $refs or a cycle. The user could have curated the spec manually." Counter: AIC's spec passes external OpenAPI 3.1 validators, the file downloads cleanly, and is the canonical published vendor spec. A generator that hangs silently on a valid published spec is a real bug. Case-against is weaker.

### 2. Multi-source aggregator generation path is undocumented in the skill (skill instruction gap / skill)
- **What happened:** The locked design called for an 8-source contemplative aggregator. The skill explicitly says wrapper-only catalog entries "cannot be generated directly" and that the only fallback is "hand-write a Go module that imports the wrapper" — stopping the press. But `coffee-goat` (in the public library) is the existence proof that multi-source aggregators *can* be built: one primary spec scaffolds the spine; additional sources are hand-authored in `internal/source/<slug>/`. There's no skill reference for this pattern. I had to infer the layout from coffee-goat's source tree.
- **Scorer correct?** N/A.
- **Root cause:** Skill instruction binary — "we generate from a spec" XOR "you stop and hand-write a Go module" — doesn't cover the realistic third path: scaffold from one spec and hand-author additional sources alongside. The pattern exists in the public library but isn't documented.
- **Cross-API check:** Named cases with evidence: coffee-goat (public library, 3 source clients in `internal/source/{coffeereview,shopify,youtube}/`), art-goat (just built, 2 source clients in `internal/source/{aic,apod}/`). Adjacent shape: any "goat"-style aggregator the user has dreamed up since (per the brainstorm, the user explicitly named this as a workflow).
- **Frequency:** subclass:`multi-source-aggregator`. Estimated 5-10% of future prints based on the goat-pattern brainstorm and the locked design.
- **Fallback if the Printing Press doesn't fix it:** Agents read coffee-goat's source tree by hand to discover the pattern. Slow and error-prone — there's no canonical structure documented; I had to guess at the registry shape, the Source interface, where to register, etc.
- **Worth a Printing Press fix?** Yes, but cheap: a single reference doc page in `skills/printing-press/references/` would suffice.
- **Inherent or fixable:** Fixable.
- **Durable fix:** Add `skills/printing-press/references/aggregator-pattern.md` documenting the layout:
  - Pick one source as the primary spec; run `generate` against it.
  - Hand-author additional source clients in `internal/source/<slug>/client.go`.
  - Define a Source interface in `internal/source/source.go` and a registry in `internal/source/registry.go` that source packages register via `init()`.
  - Add a `sources` command tree (`sources list`, `sources sync`) that walks the registry.
  - The unified data layer goes in `internal/store/<entity>.go` alongside (not replacing) the generated `store.go`.
  - Anti-reimplementation note: each source client must call the real external API; only the registry/orchestration is hand-authored without an API call.
- **Test:** Reference doc passes review checklist for coffee-goat conformance + art-goat conformance. No code change required; documentation-only.
- **Evidence:** Session moment 06:25 PDT 2026-05-21 — I had to grep `~/printing-press/library/coffee-goat/internal/source/` to figure out where source clients live, then read `internal/roasters/registry.go` to understand the registry pattern.
- **Related prior retros:** None.
- **Step G case-against:** "The skill says 'hand-write a Go module that imports the wrapper' — agents can extrapolate the multi-source pattern from that. coffee-goat existed without explicit docs and was built successfully." Counter: extrapolation cost is real, took ~10 min of exploration to find the pattern, and the agent in this session built something with subtle deviations (no init()-based registration in some places). A single reference page eliminates the extrapolation cost forever. The cost of the fix is one doc page; the benefit compounds. Case-against is weaker.

## Prioritized Improvements

### P1 — High priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---|---|---|---|---|---|---|
| F1 | Generator hangs on large OpenAPI 3.1 spec (AIC) | openapi-parser | ~10-20% of large vendor specs (subclass:large-spec-deep-refs) | Low — silent hang has no signal | medium (progress markers small; underlying parser fix large) | Profile against AIC spec; gate fix behind feature flag if needed |

### P3 — Low priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---|---|---|---|---|---|---|
| F2 | Multi-source aggregator pattern undocumented | skill | subclass:multi-source-aggregator (~5-10%) | Medium — agents extrapolate but with deviations | small (one reference doc) | None |

### Skip
| Finding | Title | Why it didn't make it (Step B / Step D / Step G) |
|---|---|---|
| C3 | Manifest re-approval for context pressure | Step G: case-against equal — the skill already says "return to Phase 1.5 with revised manifest" if infeasible. Context-pressure IS infeasibility from the agent's view; I just didn't invoke the mechanism. Fix is agent discipline, not skill change. |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|---|---|---|
| C2 | FTS5 contentful-table DELETE fails | printed-CLI — investigation showed the generator's `store.go.tmpl` uses contentful FTS5 correctly with INSERT/UPDATE/DELETE triggers. My bug was hand-authored extension code that omitted triggers. Could be a SKILL recipe but very narrow scope (only goat-class aggregators with hand-authored FTS5). |
| C5 | Default DB path holds stale FTS schema across iterations | printed-CLI — only matters for hand-authored novel-feature schema work, where iteration on local DB is part of dev loop. Migration code I wrote (`columnExists` probe + DROP/CREATE) is reasonable per-CLI logic, not generator territory. |

## Work Units

### WU-1: Add progress markers + diagnostic timeout to generator's spec parsing (from F1)
- **Priority:** P1
- **Component:** openapi-parser
- **Goal:** Make generator's spec-parsing phase observable so agents can detect hangs and recover.
- **Target:** `internal/openapi/parser.go` (spec parsing entry point), `internal/generator/generator.go` (orchestration)
- **Acceptance criteria:**
  - positive test: running `printing-press generate --spec aic-openapi.json --name test` emits at least one progress line to stderr every 30 seconds (phase, item count, current operation).
  - positive test: same invocation against Petstore spec completes silently (progress markers only emit if elapsed > 5s OR a phase boundary is crossed).
  - negative test: progress markers go to stderr only — stdout remains JSON-parseable when `--json` is set.
- **Scope boundary:** Does NOT include fixing the underlying $ref resolution pathology — that's a separate investigation that the progress markers will help diagnose. Scope is observability + watchdog.
- **Dependencies:** None.
- **Complexity:** small (progress markers) + medium (watchdog with configurable timeout).

### WU-2: Investigate $ref resolution algorithm on AIC OpenAPI 3.1 spec (from F1)
- **Priority:** P1
- **Component:** openapi-parser
- **Goal:** Understand why a valid 164KB/428-$ref OpenAPI 3.1 spec causes 60+ min CPU hang with zero output.
- **Target:** `internal/openapi/parser.go` and any $ref resolution code path.
- **Acceptance criteria:**
  - positive: `go test -bench BenchmarkAICSpec ./internal/openapi/` completes in <10 seconds on a developer machine.
  - positive: a regression test loads the AIC spec and asserts the parser completes within a 60-second deadline.
  - negative: existing parser tests pass unchanged.
- **Scope boundary:** Investigation + targeted fix. NOT a rewrite of the parser.
- **Dependencies:** WU-1 (progress markers help isolate which phase hangs).
- **Complexity:** medium — likely a memoization gap or cycle in $ref resolution; targeted fix should be small once root cause identified.

### WU-3: Document the multi-source aggregator pattern in skill references (from F2)
- **Priority:** P3
- **Component:** skill
- **Goal:** Make the coffee-goat / art-goat aggregator pattern discoverable from the skill without requiring agents to grep library source.
- **Target:** `skills/printing-press/references/aggregator-pattern.md` (new file), `skills/printing-press/SKILL.md` (one-line reference under Phase 1's wrapper-library decision tree).
- **Acceptance criteria:**
  - positive: reference doc names the Source interface contract, registry pattern (init()-based), recommended directory layout (`internal/source/<slug>/`), and anti-reimplementation requirement that each source client makes real API calls.
  - positive: SKILL.md mentions the reference in the existing wrapper-only paragraph as a third path option ("multi-source aggregator: pick one source as primary spec, hand-author other source clients per `references/aggregator-pattern.md`").
- **Scope boundary:** Documentation only. NO code change to generator templates.
- **Dependencies:** None.
- **Complexity:** small.

## Anti-patterns
- Hard-coding context-pressure scope cuts in the absorb manifest (downgrading 13→7 features mid-build) without explicitly returning to Phase 1.5 and getting re-approval. The existing rule covers it; I deviated. Discipline finding, not skill change.

## What the Printing Press Got Right
- The minimal hand-authored APOD OpenAPI spec (~50 lines) generated cleanly in well under a minute and produced a fully-working CLI scaffold (binary, store, MCP, doctor, auth) that all 8 quality gates passed on first try. The generator is genuinely productive when the spec is reasonable.
- The pivot from "use AIC as primary spec" to "use APOD as primary spec, hand-author AIC alongside" was smooth — the source-client extension pattern from coffee-goat slotted in cleanly.
- Shipcheck's 6-leg matrix caught real issues (verify-skill found a SKILL.md reference to an unbuilt `similar` command; validate-narrative found a SQL bug from a stale-schema test DB) and gave actionable diagnostics with file paths.
- The lock + heartbeat mechanism worked smoothly across a session that included a 1-hour generator hang, a lock release, and a fresh re-acquire.
- The journal-aware diversity logic for `today` works end-to-end with both AIC and APOD content — the unified `works` schema is a clean abstraction.
