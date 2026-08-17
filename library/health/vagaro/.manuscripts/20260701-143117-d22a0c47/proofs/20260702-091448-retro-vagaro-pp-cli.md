# Printing Press Retro: Vagaro

## Session Stats
- API: vagaro (consumer marketplace; first-of-kind for the beauty/wellness booking category)
- Spec source: browser-sniffed (authenticated), internal YAML spec hand-authored from capture
- Scorecard: 83/100 (Grade A)
- Verify pass rate: 96.4%
- Fix loops: ~2 (flagship perf tuning; error-path annotations)
- Manual code edits: high (all-hand-authored client + 7 novel commands; expected for an undocumented HTML/websiteapi surface)
- Features built from scratch: 7 novel + slots + book + favorites + a sibling websiteapi client

## Findings

### 1. Generator emits `partialFailureErr` unconditionally, but its only call sites are in conditionally-emitted endpoint commands (Default gap / Bug)
- **What happened:** The generated CLI's `dogfood` flags `partialFailureErr` as a dead function (`dead_functions: {dead: 1, items: ["partialFailureErr"]}`). This drives a dogfood WARN/FAIL and contributed to polish returning `hold` on an otherwise Grade-A CLI.
- **Scorer correct?** Partially. The function IS genuinely unreferenced in this CLI, so the dead-func detector is factually correct. But the root cause is the generator emitting a helper whose only callers are conditionally emitted — so the durable fix is in the generator, not the scorer.
- **Root cause:** `internal/generator/templates/helpers.go.tmpl:334` emits `func partialFailureErr(err error) error` **unconditionally**. Its only call sites are in `internal/generator/templates/command_endpoint.go.tmpl` (5 sites). A CLI with **zero generated endpoint-mirror commands** — all-hand-authored / HTML-extraction / novel-only, as vagaro is (business commands were rewired to a sibling websiteapi client; everything else is novel/framework) — emits the helper with no caller. `dogfood.go` liveness-seeding explicitly excludes `helpers.go` from the seed set (line ~2134), so a helper referenced only by other helpers or by (absent) endpoint files reads as dead.
- **Cross-API check:** Recurs on any CLI with no generated endpoint commands. This is provable from the template structure, independent of API — the unconditional emit + conditional-only call sites guarantee it.
- **Frequency:** subclass:zero-endpoint-command CLIs (browser-sniffed HTML surfaces, pure-novel CLIs). Not "every API," but a real and growing shape as browser-sniffed/HTML CLIs increase.
- **Fallback if the Printing Press doesn't fix it:** The agent must recognize the dead-func WARN as a generator false-signal (not a real defect) and manually discount it in the ship decision. Polish did NOT — it returned `hold` partly on this. So the fallback is unreliable: it misleads the automated ship gate.
- **Worth a Printing Press fix?** Yes. Trivial, provably-correct fix; removes a misleading dogfood signal for a real subclass.
- **Inherent or fixable:** Fixable.
- **Durable fix:** Gate the `partialFailureErr` emit in `helpers.go.tmpl` on the same condition that emits endpoint commands (e.g. `{{if .HasEndpointCommands}}` or whatever flag the generator already uses to decide endpoint-command emission). Since the helper is *only* called from `command_endpoint.go.tmpl`, emitting it exactly when ≥1 endpoint command is emitted keeps them in lockstep. **Alternative (weaker):** teach `dogfood.go`'s dead-func detector to treat known framework helpers in `helpers.go` as conditionally-live. The generator-side gate is cleaner (don't emit dead code) and more targeted; note the uncertainty so the implementer can pick.
- **Test:** positive — generate a CLI with ≥1 endpoint command, assert `partialFailureErr` is emitted and dogfood reports 0 dead funcs. Negative — generate a CLI with zero endpoint commands (all-novel/HTML spec), assert `partialFailureErr` is NOT emitted and dogfood reports 0 dead funcs.
- **Evidence:** `dogfood --dir .../library/vagaro --json` → `dead_functions.items: ["partialFailureErr"]`. `grep partialFailureErr(` across the generated tree returns only the definition (no call sites). Template grep confirms call sites live exclusively in `command_endpoint.go.tmpl`.
- **Related prior retros:** None (no prior retros in manuscripts).

## Prioritized Improvements

### P3 — Low priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| 1 | Gate `partialFailureErr` emit on endpoint-command presence | generator | subclass:zero-endpoint CLIs | Low (misleads ship gate) | small | Emit iff ≥1 endpoint command emitted |

### Skip
| Finding | Title | Why it didn't make it |
|---------|-------|------------------------|
| A | `auth_protocol` scores composed/cookie (`s_utkn`+cookie) auth low (2/10) | Step B: only 1 clean composed-auth CLI in the local library (vagaro); scrape-creators uses agentcookie+env-key, not the same pattern. Real bug, but can't name 3 shipped CLIs with direct evidence. Composed/cookie auth is documented first-class (auth-companion.md, browser-sniff Step 2a.1.5) — re-file if 2+ more composed-auth CLIs ship and reproduce the low score. |
| B | scorecard "Data Pipeline PARTIAL" penalizes single-table discovery CLIs (generic Upsert) | Step B: structural for lookup/discovery CLIs with one domain table; couldn't name 3 with evidence. Borderline WAI. |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| promoted `classes` Example says `classes list --page-size 20` | The bad example came from the spec endpoint's `example:` string I authored, not generator logic | printed-CLI (spec authoring) |
| Performance-API needed to capture Angular SPA XHR | `browser-sniff-capture.md` already says prefer Performance API over fetch/XHR monkey-patching | already-documented |
| `--rate-limit` default 2 made fan-out commands time out the 10s live probe | Fan-out commands are hand-built; their rate tuning is per-CLI | printed-CLI |
| `html_extract.mode: page` produced generic metadata for business detail | Hand-writing structured extraction for an undocumented surface is expected iteration | iteration-normal |
| IP-based geo scoping (URL/cookie don't override) surprised discovery | Marketplace/geo-specific; not generalizable across the library | API-quirk / narrow |

## Work Units

### WU-1: Emit `partialFailureErr` only when endpoint commands are emitted (from F1)
- **Priority:** P3
- **Component:** generator
- **Goal:** Stop emitting the `partialFailureErr` helper into `helpers.go` for CLIs that generate zero endpoint-mirror commands, so dogfood's dead-func check doesn't false-positive on a provably-unreachable framework helper.
- **Target:** `internal/generator/templates/helpers.go.tmpl` (the `partialFailureErr` definition ~line 334) gated on the same condition that governs `internal/generator/templates/command_endpoint.go.tmpl` emission.
- **Acceptance criteria:**
  - positive test: a spec with ≥1 endpoint command emits `partialFailureErr` and dogfood reports 0 dead funcs.
  - negative test: an all-novel / HTML-extraction spec with zero endpoint commands does NOT emit `partialFailureErr`, dogfood reports 0 dead funcs, and the CLI still builds (`--allow-partial-failure` flag remains valid; it only downgrades exit codes, it doesn't require the helper).
- **Scope boundary:** Only `partialFailureErr`. Do not sweep other helpers in this WU; if the implementer finds sibling helpers with the same conditional-only-caller shape, note them but keep this WU focused.
- **Dependencies:** none
- **Complexity:** small

## Anti-patterns
- (none load-bearing this run)

## What the Printing Press Got Right
- The internal-YAML spec + `response_format: html` / `html_extract` + composed cookie auth (`auth login --chrome`) + sibling-client extension pattern all composed cleanly for a fully-undocumented, JS-heavy marketplace — the generator emitted a working framework (client, store, auth, sync, MCP walker, output flags) that the hand-authored layer built on without fighting it.
- `regen-merge` / hand-edit durability, `cliutil` helpers (`ParseODataDate`, `AdaptiveLimiter`, `FanoutRun`, `CleanText`), and the `pp:no-error-path-probe` / `pp:data-source` / `pp:client-call` annotation surface all did exactly what the docs said — the annotations cleanly resolved the dogfood false-positives on empty-result and reimplementation checks.
- The scorecard live-probe caught the flagship `find`/`price-check` timeouts (20s) that structural checks missed — a genuine behavioral catch, not a false signal.
