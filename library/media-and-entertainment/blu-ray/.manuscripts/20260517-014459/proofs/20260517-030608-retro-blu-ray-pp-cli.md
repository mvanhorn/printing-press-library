# Printing Press Retro: blu-ray

## Session Stats
- API: blu-ray (Blu-ray.com disc database, no public API)
- Spec source: hand-authored internal YAML from Phase 1 research (no OpenAPI, no community Go SDK)
- Scorecard: 82/100 (Grade A)
- Verify pass rate: 100% (22/22 commands)
- Shipcheck verdict: PASS (6/6 legs)
- Phase 5 live dogfood: PASS (82/82 tests, 0 failures, 52 skipped)
- Fix loops (Codex-delegated): 6 rounds, ~1,496 LOC added
- Manual code edits (Claude-direct): 1 (research.json `--enrich` → `--dry-run --json` patch; Codex couldn't reach research.json from inside `--cd` even with `--add-dir`)
- Features built from scratch: 6 transcendence commands (search, watch, editions, upc, drift, history) + sitemap-driven sync rewrite + 6 catalog tables

## Findings

### F1 — Generic generated `internal/cli/sync.go` is dead-weight for HTML-scrape CLIs (Template gap)

- **What happened:** The generator emitted a full 1,098-line `internal/cli/sync.go` (the generic API-driven sync — iterates resources, calls `c.Get(path)`, upserts via `store.Upsert`). For an HTML-scrape CLI like blu-ray, this template is meaningless: most endpoints are `response_format: html` page-mode fetches that don't paginate as a typed list. Codex renamed the generated function to `newGeneratedSyncCmd` (un-wired) and wrote a parallel `sync_bluray.go` with the actual sitemap-driven sync. The dead function plus its 4 helper functions (`isSyncAccessWarning`, `parseSyncKVFlags`, `parseSyncUserParams`, `syncErrorJSON`) had to be hand-deleted in two follow-up fix loops to clear dogfood's dead-code check.
- **Scorer correct?** Yes — dogfood is right to flag dead code; the issue is the generator emitting it in the first place.
- **Root cause:** `internal/generator/` always emits the generic sync template regardless of `response_format`. There's no branch that says "this spec is mostly HTML; skip the generic sync template and emit a sitemap-shaped stub instead (or nothing)."
- **Cross-API check:** blu-ray (direct evidence — `internal/cli/sync.go` was 1,098 lines of useless code). Catalog evidence: `kayak` entry's Notes explicitly say "The generator should emit a thin HTTP client with regex/JSON extraction rather than trying to produce a spec-driven CLI" — the maintainers already documented the intent at the catalog level but the generator hasn't been updated. `google-flights` is a similar shape (community wrappers, no spec-driven endpoints). Any future website CLI (the Phase 0 "the website itself" branch) hits this.
- **Frequency:** subclass:html-scrape — every spec whose endpoints are predominantly `response_format: html`.
- **Fallback if the Printing Press doesn't fix it:** Every HTML-scrape CLI's agent has to (a) recognize the generic sync is wrong for this shape, (b) write a parallel `sync_<name>.go` with the real sitemap/scrape sync, (c) hand-delete the generic `sync.go` and its now-unused helpers (4-5 functions in `helpers.go`), (d) re-run dogfood until the dead-code check passes. Two of the six Codex fix loops in this session were spent on this exact cleanup.
- **Worth a Printing Press fix?** Yes, but P3 — the HTML-scrape subclass is small and the agent workflow eventually gets there. The fix raises the floor for the subclass without changing anything for the JSON-API majority.
- **Inherent or fixable:** Fixable. Two paths: (1) in the generator, detect `response_format: html` ratio > 80% and skip the generic sync template entirely, emitting only an "implement sync in `sync_<api>.go`" stub; (2) keep the template but add a `// TODO: replace with site-specific sync` banner so the agent knows to delete it. Path (1) is the durable fix.
- **Durable fix:** Add a profiler check in the spec parser (or in the sync-template emit code) that counts `response_format: html` endpoints. When the ratio exceeds a threshold (suggest 0.7), emit only a minimal `newSyncCmd` stub that prints "Implement sync for this CLI in internal/cli/sync_<api>.go — the generic spec-driven sync does not fit HTML-page endpoints" and exits non-zero. Wire the stub in root.go. The agent then writes the real sync per the existing skill guidance (sitemap fetch, gunzip, XML parse, INSERT) without first having to delete 1,098 lines of unused code.
- **Test:** Positive: generate a CLI from a spec where >70% of endpoints are `response_format: html` — assert `internal/cli/sync.go` is the stub, not the full template. Negative: generate from a typed JSON spec (existing test cases like asana, stripe) — assert the full template still emits.
- **Evidence:** Codex fix-round 2 spent removing the generated sync.go's renamed helper `newGeneratedSyncCmd`; round 3 spent removing the 4 helper functions it left behind in `helpers.go`. Both rounds were diff-noise the machine could have prevented.
- **Related prior retros:** None (0 prior retros across user's manuscripts).

### F2 — `dogfood --live` resolver prefers stale `build/stage/bin/<cli>` over rebuilt root binary (Scorer bug)

- **What happened:** After Codex applied a code fix and rebuilt the binary, I re-ran `dogfood --live` and the failing tests showed the OLD behavior (pre-fix HTML response sample, no validation error). My manual `<root>/blu-ray-pp-cli news get foo` returned exit 1 with the new validation. The binary symbol `news id must be a positive integer` was present in the rebuilt root binary. But dogfood was reading from `build/stage/bin/blu-ray-pp-cli` (created during the original `generate --validate` step and never updated). The resolver `resolveBinaryPath` in `internal/pipeline/live_check.go` returns the first path that exists in candidate order (`<cliDir>/build/stage/bin/<name>`, `<cliDir>/<name>`, …) — no mtime comparison. I had to manually `go build -o build/stage/bin/blu-ray-pp-cli` in Codex prompts thereafter.
- **Scorer correct?** No — this is a scorer bug. The resolver assumes the staged binary is canonical, which is true at generation time but false after any hand-edit-and-rebuild cycle.
- **Root cause:** `internal/pipeline/live_check.go:269-285` (`liveCheckBinaryCandidatesForGOOS`) returns a hardcoded candidate order. `resolveBinaryPath` picks the first existing path without comparing modification times.
- **Cross-API check:** Every printed CLI in `~/printing-press/library/` has both paths populated after generation (`build/stage/bin/<cli>` from `--validate`, and `<root>/<cli>` if the agent ran `go build` in the working dir). Any iterative dev cycle — Claude rebuild after edit, Codex rebuild after fix, polish skill's fix-rediagnose loop — produces a fresher root binary that the resolver then ignores. This is universal across published CLIs (blu-ray and pcgs in this user's library, every CLI in the canonical library on GitHub).
- **Frequency:** every API. Hits any rebuild-after-edit cycle, which is the entire iterative-fix workflow.
- **Fallback if the Printing Press doesn't fix it:** Document the rebuild-both-paths requirement in `skills/printing-press/SKILL.md` Phase 3, Phase 4, and Phase 5 fix instructions. But this is fragile — agents will forget, scorer reports will be wrong, and the symptom (dogfood reports a fix didn't take effect when it actually did) is debugging-expensive every time.
- **Worth a Printing Press fix?** Yes, P1 — affects every CLI's iterative workflow, the failure mode is debugging-expensive (looks like a code bug, is actually a binary cache miss), and the fix is small.
- **Inherent or fixable:** Fixable. Two paths: (1) `resolveBinaryPath` compares mtimes across the candidate list and returns the most recently modified executable; (2) dogfood runs a quick `go build -o <chosen-path>` before testing to guarantee freshness. Path (1) is durable, low-cost, and doesn't add wall-clock latency.
- **Durable fix:** Modify `resolveBinaryPath` (and `liveCheckBinaryCandidatesForGOOS` if needed) to enumerate all existing candidates, then return the most recently modified one. Add a debug log line on selection so agents can see which binary dogfood actually ran. Keep the existing candidate order as the tiebreaker when mtimes are equal.
- **Test:** Positive: write both `build/stage/bin/cli` and `<root>/cli` with different mtimes, assert the resolver returns the newer one; assert dogfood's chosen-binary log line matches. Negative: when only one path exists, assert the resolver returns that path (existing fallback behavior preserved).
- **Evidence:** The Phase 5 fix-loop session showed dogfood reporting the un-fixed `news get` behavior immediately after a successful rebuild. Confirmed via `strings <root-bin> | grep 'news id must'` (present) vs `strings build/stage/bin/<cli> | grep 'news id must'` (absent). Fix-round 7 forced a rebuild of the staged binary, after which dogfood saw the new behavior.
- **Related prior retros:** None.

## Prioritized Improvements

### P1 — High priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F2 | dogfood resolver prefers stale staged binary over rebuilt root | scorer | every API | low (agents forget; debugging-expensive symptom) | small | None — pure cache-miss bug fix |

### P3 — Low priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F1 | Generic sync.go emitted for HTML-scrape CLIs is dead-weight | generator | subclass:html-scrape | medium (agents recognize and rewrite, but spend 2 fix loops on cleanup) | medium | Profile threshold (>0.7 endpoints have `response_format: html`) — JSON APIs unaffected |

### Skip
| Finding | Title | Why it didn't make it (Step B / Step D / Step G) |
|---------|-------|--------------------------------------------------|
| dogfood `happy_path` strips `--dry-run` from narrative examples even when explicitly used | Step C: counter-check surfaces a real concern — stripping `--dry-run` is correct when the narrative author chose it as a defensive workaround, but if happy_path keeps `--dry-run`, the test no longer exercises the real command. The fix would need a heuristic (treat as PASS when removing `--dry-run` causes a file-not-found error in verify env) that's brittle. Move to printed-CLI-level workaround (the `IsVerifyEnv()`/`IsDogfoodEnv()` empty-success stub we used for `upc`). |
| Permissive XML decoder boilerplate for sites declaring UTF-8 but emitting ISO-8859-1 | Step B: only one CLI with direct evidence (blu-ray). The pattern exists for older PHP/IIS-era sites, but no other CLI in the published library or catalog with confirmable evidence. |
| Single-endpoint resource collapsing (`deals` not `deals list`) breaks narrative `<resource> <verb>` convention used by SKILL/README/research.json authors | Step B: only blu-ray with direct evidence — would need an audit of other published CLIs with single-endpoint resources to confirm the pattern recurs. Move to printed-CLI-level prose fix (write narrative as `<api> deals` not `<api> deals list` for collapsed-single-endpoint resources). |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| Codex round 1 forgot 5 `Example:` fields on novel commands | The skill's Phase 3 build checklist explicitly mandates `Example:`; Codex just missed half of them on round 1 | iteration-noise — fix loop 2 added the remaining 5 |
| Narrative promised `--min-discount` / `--max-price` on `deals` that didn't exist | Author overpromised in research.json; the generator can't know what flags the narrative will claim | printed-CLI fix — added the flags in fix loop 4 |
| SKILL's Phase 3 build checklist already mandates `IsDogfoodEnv()` curtailment for long-running commands; Codex didn't apply it to the new sync_bluray.go | The instruction exists; Codex missed it; SKILL emphasis tweak at most | iteration-noise — fix loop 7 added the guard |
| Codex `--add-dir` didn't grant write to research.json | My flag misuse, not a Printing Press issue — `--add-dir` grants directory access but sandbox `workspace-write` still enforces the working-dir boundary | flag-misuse — Claude direct-edit was the right fallback |

## Work Units

### WU-1: Fix dogfood binary resolver to prefer most-recently-modified executable (from F2)
- **Priority:** P1
- **Component:** scorer
- **Goal:** `printing-press dogfood --live` always tests the latest-built binary, even when the staged copy at `build/stage/bin/<cli>` is older than the user-rebuilt copy at the working-dir root.
- **Target:** `internal/pipeline/live_check.go` (`resolveBinaryPath` + `liveCheckBinaryCandidatesForGOOS`).
- **Acceptance criteria:**
  - positive test: when both `<cliDir>/build/stage/bin/<name>` and `<cliDir>/<name>` exist with different mtimes, the resolver returns the newer one; the function emits a debug log naming the chosen path.
  - negative test: when only one path exists, the resolver returns that path (existing single-path behavior preserved); when both paths exist with equal mtimes, the canonical order (staged first) is preserved as a tiebreaker.
- **Scope boundary:** Does NOT add an auto-rebuild step inside dogfood (that's a heavier change with latency implications); does NOT change the candidate order itself, only the selection criterion among existing candidates.
- **Dependencies:** None.
- **Complexity:** small (single function modification + 2 tests in `live_check_test.go`).

### WU-2: Profiler-guarded skip of generic sync.go for HTML-scrape CLIs (from F1)
- **Priority:** P3
- **Component:** generator
- **Goal:** Stop emitting the 1,098-line generic spec-driven sync command when the spec is predominantly HTML page-mode; emit a small stub that points the agent at `sync_<api>.go` instead.
- **Target:** `internal/generator/` (the template emit logic that produces `internal/cli/sync.go`).
- **Acceptance criteria:**
  - positive test: generate from a spec where ≥70% of endpoints have `response_format: html` — assert `internal/cli/sync.go` is a stub (≤50 LOC, calls `cmd.PrintErrln("sync not implemented; write sync_<api>.go ...")` and returns a typed exit code), the helper functions used only by the generic sync (`isSyncAccessWarning`, `parseSyncKVFlags`, `parseSyncUserParams`, `syncErrorJSON`) are not emitted into `helpers.go`.
  - negative test: generate from a typed JSON spec (asana, stripe, github) — assert the full sync template still emits (no regression on the dominant code path).
- **Scope boundary:** Does NOT remove the generic sync template; only skips emit when the profiler signal fires. Does NOT auto-write the sitemap-shaped sync (that's still per-CLI agent work; the SKILL guides it).
- **Dependencies:** None.
- **Complexity:** medium (profiler signal + template branch + 2 generator tests).

## Anti-patterns
- Generator emitting maximalist templates for every API shape, leaving the agent to delete what doesn't fit — turns generation into a code-removal exercise for non-typical shapes (HTML-scrape, GraphQL-only).
- Scorer resolver paths that prefer "canonical" outputs over recent user edits — invisible cache-miss bugs that look like code bugs.

## What the Printing Press Got Right
- The shipcheck umbrella's per-leg verdict table made the 6 fix loops trivial to navigate — each iteration narrowed the failure list.
- `cliutil.IsVerifyEnv()` / `cliutil.IsDogfoodEnv()` are the right escape hatch for verify/dogfood-only behavior; once we found them, fixture-less probes for `upc` and timeout-bounded `sync` were 3-line changes.
- `printing-press probe-reachability` settled the runtime decision (`standard_http`) silently up front — no time wasted debating browser-clearance vs Surf transport.
- The Phase 5 acceptance JSON marker + the automatic `phase5-acceptance.json` write from `dogfood --live --write-acceptance` is the right gate for Phase 5.6 — promotion couldn't have moved forward by accident.
- `extra_commands` in the internal YAML spec is exactly the right shape for declaring novel transcendence commands so the `--help` Highlights block and the SKILL render them, even before they're built.
