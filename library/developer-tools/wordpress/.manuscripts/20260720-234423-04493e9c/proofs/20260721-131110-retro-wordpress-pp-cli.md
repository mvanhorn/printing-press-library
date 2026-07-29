# Printing Press Retro: WordPress

## Session Stats
- API: wordpress
- Spec source: docs + live discovery route tables (internal YAML built from live `/wp-json/` indexes across 3 sites)
- Scorecard: 95/100 (A)
- Verify pass rate: 100% (55/55)
- Live acceptance: 547/547 (full level)
- Fix loops: 2 (foundation review, polish)
- Manual code edits: ~10 (4 foundation bugs + polish fixes)
- Features built from scratch: 10 (7 transcendence + site registry, media upload, global query flags)
- Codex delegation: 3 batches; 1 failed on broad prompt, 2 succeeded on tight single-scope prompts

## Findings

### 1. Generated credential tests fail at baseline for multi-env-var auth (Bug)
- **What happened:** With two-env-var HTTP Basic auth (`WORDPRESS_USER` + `WORDPRESS_APP_PASSWORD`), the emitted `internal/cliutil/credentials_test.go` sets only one env var and asserts a non-empty `AuthHeader()`. Four tests fail at the generated baseline, before any hand-code (verified at scratch-repo commit `491459c`).
- **Scorer correct?** N/A (go test failure, not a score penalty).
- **Root cause:** Generator — the credential-test template assumes single-token auth; it does not iterate the declared `auth_env_vars` set when arranging test env state.
- **Cross-API check:** Deterministic for every CLI whose auth format has two placeholders.
- **Frequency:** subclass: multi-env-var auth. Direct evidence in-library: **wordpress** (this run, 4 failing tests), **woocommerce** (`suspect_test: true`, `TestCorruptCredentialsFallsBackToEnvCredential`), **wpengine** (same test fn, same pattern).
- **Fallback if the Printing Press doesn't fix it:** Every agent printing a two-var CLI must diagnose a failing baseline test suite and choose between patching the printed CLI (hiding the machine bug) or shipping failing tests. This run deliberately left them failing per the template-shape escape hatch.
- **Worth a Printing Press fix?** Yes — deterministic baseline failure, three named CLIs, zero downside.
- **Inherent or fixable:** Fixable in the test template.
- **Durable fix:** Credential-test template sets **all** declared `auth_env_vars` in the happy-path arrange step, and asserts empty header when any one of them is missing (which is the emitted `AuthHeader()`'s actual documented behavior).
- **Test:** Positive — generate a two-var Basic CLI, `go test ./internal/cliutil/` passes at baseline. Negative — single-token CLIs' tests unchanged and passing.
- **Evidence:** Build log `proofs/2026-07-21-000000-fix-wordpress-pp-cli-build-log.md` §"Pre-existing generator issue"; four named failing test functions.
- **Related prior retros:**
  - `bunny` retro (20260704-143000) — `aligned`. "Multi-env-var apiKey auth generation is broken … made 5 generated credential tests FAIL." Same subclass, earlier generator version.
  - `woocommerce` retro (20260721-104027) — `aligned`. "credential tests assume single token (`internal/cliutil`)". Filed yesterday against v4.29 — same generator version as this run.

### 2. Generator silently drops API surface on name collision (Bug)
- **What happened:** The `abilities` resource vanished from the generated CLI with no warning: its endpoint names `categories`/`category` collided with the top-level `categories` resource, and the whole resource was dropped at generation. Only caught because the absorb manifest was cross-checked by hand; renaming to `ability-categories` in the spec worked around it.
- **Scorer correct?** N/A — no scorer, dogfood, or verify leg detected the missing surface; that silence is the finding.
- **Root cause:** Generator — name-keyed emission with last-writer-wins (or skip-on-conflict) semantics and no collision diagnostics. No post-emission invariant asserts that every spec resource/endpoint reached the command tree.
- **Cross-API check:** This is the fourth independent sighting of the same failure *class* (silent surface loss) with different mechanisms.
- **Frequency:** every API with name-colliding endpoints/resources; the invariant check benefits every API.
- **Fallback if the Printing Press doesn't fix it:** Agents must hand-diff the spec against `--help` output on every print — this run only caught it because the manifest listed the resource. A user who didn't cross-check would ship a CLI silently missing a namespace.
- **Worth a Printing Press fix?** Yes — silent data loss is the worst failure mode a generator can have; a loud failure is cheap.
- **Inherent or fixable:** Fixable.
- **Durable fix:** Two layers, both in the generator: (a) collision detection at emission — a name collision between endpoint and resource (or endpoint/endpoint after normalization) is a hard generation error naming both spec paths, never a silent skip; (b) post-emission parity assertion — after emit, walk the command tree and assert every spec resource/endpoint has a corresponding command, failing generation with the diff otherwise. Layer (b) also catches future unknown drop mechanisms (the linear/photos/bunny variants), not just collisions.
- **Test:** Positive — a spec with a deliberate endpoint/resource name collision fails generation with both paths named. Negative — collision-free specs generate unchanged; parity assertion passes on the full library.
- **Evidence:** mkspec.py rename workaround in this run's research pipeline; the resource silently absent from the first generation's command tree.
- **Related prior retros:**
  - `bunny` retro (20260704-143000) — `extends`. Duplicate normalized `resource\0path` key produced uncompilable Go — collision surfaced loudly there by luck (compile error), silently here.
  - `linear` retro (20260519-010809) — `extends`. "Each future GraphQL generation drops resources silently and the agent has to hand-write replacements" — 17 resources lost between generator versions.
  - `photos` retro (20260605-130643) — `extends`. Plan-declared command names not deduplicated against factory defaults → duplicate registration; same missing-diagnostics root.
  - `woocommerce` retro (20260721-104027) — `extends`. MCP cobratree walker silently prunes hidden subtrees; same class at the MCP layer.

### 3. gosec findings in generator-emitted files — narrow reframe (Bug / default gap)
- **What happened:** 20 gosec findings in generated DO-NOT-EDIT files (G201/G202 SQL string formatting in store/freshness/learnings, G304 file inclusion in config/teach/feedback/export, G119, G117, assorted G104). Polish triaged all of them as generator candidates; hand-authored files were brought to zero.
- **Scorer correct?** Mostly false positives on vetted code (SQL Sprintf sites interpolate identifiers, not values; file reads are user-supplied paths by design) — but the *noise* is real and recurs.
- **Root cause:** Generator templates emit patterns gosec flags, with no explicit discards or scoped suppressions.
- **Cross-API check:** Same templates → same findings everywhere. Verified today: `internal/store/store.go` SQL-building `fmt.Sprintf` present in **woocommerce** (line 2215), **ahrefs** (736), **linear** (540), **monday** (885), **wpengine** (2999).
- **Frequency:** every API.
- **Fallback if the Printing Press doesn't fix it:** Every polish pass re-triages the same ~20-34 findings (opensea saw 34, monologue 27, wordpress 20) and every downstream CI running gosec drowns in them.
- **Worth a Printing Press fix?** Raised in 3 prior retros without landing — per the recurrence rule this is NOT re-raised at prior scope. **Narrow reframe:** only the two mechanical, zero-risk slices — (a) explicit `_ =` discards at known intentional-ignore sites (G104), (b) scoped `// #nosec G201 -- identifier interpolation only, values are bound` annotations at the store template's SQL-builder sites. No permission changes, no behavior changes.
- **Inherent or fixable:** The narrow slice is trivially fixable; the full gosec-clean goal may be inherent tension (generated code serving many API shapes).
- **Durable fix:** Template-level edits at the specific G104/G201/G202 sites listed above.
- **Test:** Positive — freshly generated CLI's `gosec ./internal/store/... ./internal/cliutil/...` reports 0 G104/G201/G202. Negative — `go vet` and full test suite unchanged.
- **Evidence:** Polish ledger this run (20 findings routed to retro); five library CLIs' store.go verified today.
- **Related prior retros:**
  - `instagram` retro (20260607-163246) — `aligned`. Proposed exactly the `_ =` discards and scoped `#nosec` with rationale.
  - `monologue` retro (20260619-123913) — `aligned`. "27 remaining gosec findings, all in generator-emitted templated files."
  - `opensea` retro-notes — `aligned`. 34 findings across the same generated packages.
  - **Recurrence annotation:** raised 3 times without landing; this entry deliberately shrinks scope to the mechanical slice instead of re-raising the full ask.

### 4. No extension point for the local-store DB path (Missing scaffolding, subclass: multi-tenant)
- **What happened:** The generated `sync` writes to the framework `defaultDBPath` and cannot be redirected without editing generated code. This CLI's hand-built multi-site layer (site registry) needed per-site mirrors; the mismatch made `fleet` report "no local mirror" immediately after a successful 117-record sync. Fixed by hand with an existence-checked fallback resolver.
- **Scorer correct?** N/A.
- **Root cause:** Generator — `root.go` already emits `registerNovelCommand` and `registerClientHook` extension points, but the store path is hardwired; there is no equivalent hook for DB-path resolution.
- **Cross-API check:** Only 2 CLIs with direct evidence — **wordpress** (this run) and **woocommerce** (operator created per-store config files for two stores; yesterday's retro filed the per-tenant credential-scoping finding as P1). The other 20 library CLIs are single-tenant.
- **Frequency:** subclass: multi-tenant (CLIs whose API targets arbitrary per-instance endpoints — stores, sites, workspaces).
- **Fallback if the Printing Press doesn't fix it:** Each multi-tenant CLI's agent re-invents a path-resolution shim by editing generated files, which regen-merge then flags as `TEMPLATED-WITH-ADDITIONS` forever.
- **Worth a Printing Press fix?** P3 max by rule (only 2 APIs with evidence). The fix is small and floor-raising: it extends the existing hook pattern rather than adding tenancy semantics.
- **Inherent or fixable:** Fixable, small.
- **Durable fix:** Generator emits a `registerDBPathHook(func() (string, bool))` alongside the existing hooks; the generated `sync`/store-open path consults registered hooks before `defaultDBPath`. Hand-built tenancy layers then redirect the store from separate novel files, keeping regen-merge clean. No behavior change for CLIs that register nothing.
- **Test:** Positive — a CLI registering a hook syncs and reads through the hook path. Negative — hook-free CLIs byte-identical behavior.
- **Evidence:** `internal/cli/wordpress_dbpath.go` (the hand-built resolver and its apologetic comment block); build log bug #3.
- **Related prior retros:**
  - `woocommerce` retro (20260721-104027) — `extends`. "Config credential precedence … cross-tenant key transmission … scope the credentials store by config path or base_url." Same multi-tenant gap, credentials facet; this finding is the storage facet.

## Prioritized Improvements

### P1 — High priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F1 | Multi-env-var credential-test template sets all declared env vars | generator | subclass: multi-env-var auth (3 named CLIs) | Low — agents must notice failing baseline tests and resist patching the printed CLI | small | Only alters test arrange/assert for 2+ var auth |
| F2 | Collision = hard error + post-emission spec→command parity assertion | generator | every API | Very low — silent loss is undetectable without hand-diffing | medium | Assertion allows explicitly-skipped endpoints (spec `skip` markers) |

### P3 — Low priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F3 | Narrow gosec slice: explicit discards + scoped #nosec in store templates | generator | every API | Medium — polish re-triages each time | small | Only G104/G201/G202 sites; no permission/behavior changes. Raised 3× before; scope deliberately shrunk |
| F4 | `registerDBPathHook` extension point for store path | generator | subclass: multi-tenant (2 named CLIs) | Medium — agents build shims but must edit generated files | small | No-op when unregistered |

### Skip
| Finding | Title | Why it didn't make it |
|---------|-------|-----------------------|
| S1 | Codex delegation: broad prompts exhaust budget on large trees | Step B: evidence from 1 CLI only; the delegation reference already states the load-bearing rule ("paste ACTUAL CODE — never descriptions"). One violation is agent error, not an instruction gap. Re-raise if it recurs. |
| S2 | Thin generated `Short:` on `learnings list` | Step B: 2 CLIs with the string, and "List recorded learnings" is serviceable; polish accepted with ledger rationale. Cosmetic. |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| Cache-key missing global query params | Hand-added `--wp-fields`/`--embed` globals collided in the response cache | printed-CLI — all 22 library CLIs fold params into the key; none has a global-query-param mechanism (it was this CLI's hand-code) |

## Work Units

*Filed 2026-07-21: WU-1 -> [#3699](https://github.com/mvanhorn/cli-printing-press/issues/3699), WU-2 -> [#3698](https://github.com/mvanhorn/cli-printing-press/issues/3698), WU-3 -> comment on [#3428](https://github.com/mvanhorn/cli-printing-press/issues/3428), WU-4 -> [#3697](https://github.com/mvanhorn/cli-printing-press/issues/3697).*

### WU-1: Credential-test template honors multi-env-var auth (from F1)
- **Priority:** P1
- **Component:** generator
- **Goal:** Generated `internal/cliutil/credentials_test.go` passes at baseline for CLIs whose auth declares 2+ env vars.
- **Target:** Credential-test template in `internal/generator/`
- **Acceptance criteria:**
  - positive: regenerate a two-placeholder Basic-auth CLI (wordpress spec) → `go test ./internal/cliutil/` green at baseline
  - negative: single-token CLIs' emitted tests byte-stable (or equivalent) and green
- **Scope boundary:** Does not change `AuthHeader()` runtime semantics — only the test template's arrange/assert.
- **Dependencies:** none
- **Complexity:** small

### WU-2: Loud collision handling + spec→command parity assertion (from F2)
- **Priority:** P1
- **Component:** generator
- **Goal:** Generation can never silently drop a spec resource or endpoint.
- **Target:** Emission naming/registration in `internal/generator/`
- **Acceptance criteria:**
  - positive: a spec with an endpoint name colliding with a top-level resource fails generation with both spec paths named
  - positive: post-emission walk asserts every spec resource/endpoint maps to a command; injected drop → generation fails with the diff
  - negative: the existing library's specs all pass the parity assertion unchanged
- **Scope boundary:** No auto-renaming policy decision required — hard error is sufficient; renaming can be a follow-up.
- **Dependencies:** none
- **Complexity:** medium

### WU-3: Mechanical gosec slice in store/cliutil templates (from F3)
- **Priority:** P3
- **Component:** generator
- **Goal:** Freshly generated CLIs report zero G104/G201/G202 in `internal/store` and `internal/cliutil`.
- **Target:** Store and framework templates in `internal/generator/`
- **Acceptance criteria:**
  - positive: `gosec` on a fresh generation reports 0 findings for those rules in those packages
  - negative: `go vet` + full test suite unchanged
- **Scope boundary:** Explicit discards and scoped, rationale-bearing `#nosec` only. G304/G117/G119 and permission findings are out of scope (design review needed).
- **Dependencies:** none
- **Complexity:** small

### WU-4: DB-path extension hook (from F4)
- **Priority:** P3
- **Component:** generator
- **Goal:** Hand-built tenancy layers can redirect the local store without editing generated files.
- **Target:** `root.go`/store-open templates in `internal/generator/` (pattern parallel to `registerClientHook`)
- **Acceptance criteria:**
  - positive: a novel file registering a DB-path hook routes sync + reads through it
  - negative: CLIs registering nothing behave byte-identically
- **Scope boundary:** No tenancy semantics in the framework — just the resolution hook.
- **Dependencies:** none
- **Complexity:** small

## Anti-patterns
- Broad multi-item Codex prompts against a 391-file generated tree exhaust the delegation budget on exploration (18.5k log lines of file dumps, zero writes). Tight single-scope prompts with inlined signatures and a file allowlist succeeded first try. (Skipped as a finding — the reference already mandates this; recorded here as operator guidance.)
- Patching generated tests in the printed CLI to green a baseline failure hides the machine bug from the next run — the template-shape escape hatch (leave failing, file retro) is the right move and worked as designed here.

## What the Printing Press Got Right
- 53 resources / 174 endpoints emitted from an internal YAML with correct two-var Basic auth, base-URL override, learn seeds, and full MCP orchestration — baseline built clean.
- The `registerNovelCommand` / `registerClientHook` extension points made 10 hand-code items regen-durable; WU-4 just asks for one more hook in the same pattern.
- Verify's zero-input-command convention (`profile list` does work when bare) gave a clear precedent for fixing the bare-help guard on six novel commands.
- Polish caught real issues late (schema command-tree depth mismatch, rate-limit handling) and its ship logic correctly refused to inflate the scorecard.
- Live acceptance at full level (547/547) against two authenticated production sites plus behavioral verification against four unauthenticated public sites gave real correctness signal, not exit-code theater.
