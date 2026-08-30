# PSX CLI — Shipcheck Proof

## Scores

| Leg | Result | Notes |
|---|---|---|
| verify | PASS | 97% pass rate, 0 critical |
| validate-narrative | PASS | strict + full-examples |
| dogfood (structural) | PASS | |
| workflow-verify | PASS | |
| apify-audit | PASS | |
| **verify-skill** | **FAIL** | 3 findings — all proven false positives (see below) |
| scorecard | PASS | **97/100 Grade A** (live mode) |

**Scorecard progression across the optimization loop:** 81 → 90 → 92 → 96 → **97**.

| Dimension | Score |
|---|---|
| Output Modes, Auth, Error Handling, README, Doctor, Agent Native | 10/10 each |
| MCP Quality, MCP Desc Quality, MCP Remote Transport | 10/10 each |
| Local Cache, Cache Freshness, Breadth, Vision, Workflows, Insight | 10/10 each |
| Data Pipeline Integrity, Sync Correctness, Live API Verification | 10/10 each |
| Type Fidelity, Dead Code | 5/5 each |
| Path Validity | 9/10 |
| Terminal UX | 9/10 |
| Agent Workflow | 9/10 |
| MCP Token Efficiency | 7/10 |

## Live dogfood (Phase 5, Full)

**217/217 tests passed, 0 failed.** Every leaf subcommand exercised for help, happy-path,
JSON-fidelity and error-path against the real portal. No credentials required (PSX is
unauthenticated); no mutating endpoints exist, so nothing was written upstream.

## Fixes applied during the loop

1. **path_validity 2 → 9.** Root cause: the spec had been narrowed to JSON-only endpoints, so
   30+ commands called paths the spec never declared. Restored the full 18-resource spec and
   switched registration to *replace* the generated HTML leaves with the hand-authored parsers
   (`replaceRootCommand`), keeping declared paths, typed MCP tools and correct data simultaneously.
   Typed MCP tools went 5 → 22.
2. **data_pipeline_integrity 7 → 10**, via the same spec restoration (typed domain tables).
3. **dead_code 3 → 5.** Removed generator-emitted dead helper `collectionItemsForOutput`;
   enriched two thin framework `Short` fields.
4. **error_handling 8 → 10.** Unknown symbols now return typed exit 3 rather than an empty
   success. `/timeseries/int` reports unknown symbols in its envelope; `/timeseries/eod` does
   **not** (it returns `{"status":1,"data":[]}`), so the empty path falls back to the instrument
   master rather than opting out of the error-path probe.
5. **live_api_verification N/A → 10/10**, by running the verify leg in live mode.
6. **MCP tool descriptions**: enriched two thin spec descriptions; `tools-audit` now clean.
7. Polish pass: gosec G104 fix, `docs search` relevance ranking (title hits scored above URL-path
   hits), and replacement of a dead example query.

## Falsified experiment (recorded so it is not retried)

Enabling `mcp.orchestration: code` + `endpoint_tools: hidden` to raise MCP Token Efficiency
**lowered** the total 96 → 93: MCP Quality fell 10 → 8 and Desc Quality/Token Efficiency became
N/A, while Tool Design/Surface Strategy became 10/10. Net negative for a 22-endpoint surface.
Reverted.

## Known gaps (documented, not blocking)

- **MCP Token Efficiency 7/10.** Endpoint-mirror is the better shape here, as the experiment
  above proves. Reducing the runtime surface further would mean hiding commands the framework's
  own classification guidance says to keep exposed.
- **Path Validity / Terminal UX / Agent Workflow at 9/10** — one point each, no specific finding
  reported by any diagnostic.
- **Three output-review findings deferred** (announcements emits anchor text rather than the
  filing href; `indices` carries an embedded as-of timestamp; `quote`/`market watch` show raw
  sector codes). All three require changing `text()` in the shared `internal/psx/table.go`
  extractor, which would alter the row schema for every table-backed command and the synced
  store. That blast radius belongs in a generator change, not a pre-promotion hand-edit.

## verify-skill FAIL — proven false positive

verify-skill reports 3 `positional-args` errors claiming `watchlist show` has
`Use: "show <name>"` and `watchlist add` has `Use: "add <name>"`.

**Ground truth:** the real commands declare `Use: "show"` (0 positionals, exits 0 with 0 args)
and `Use: "add <symbol>..."` (variadic, exits 0 with `OGDC LUCK ENGRO`). Both README and SKILL
recipes are correct as written.

**Proof, two independent methods:**
1. Renaming my commands to `prices`/`track` left the findings **byte-identical**, still citing
   `watchlist show`/`watchlist add` — so the check is not reading my commands at all.
2. Renaming the placeholder `Use` strings in `internal/cli/platform_client.go:462,528` changed
   the *reported* string verbatim while still attributing it to `watchlist`.

The checker resolves command paths by leaf token, ignoring the parent group, and cannot see
commands registered through the novel-command hook. `platform_client.go`'s colliding commands are
not even registered in the Cobra tree. Dogfood's novel-feature depth check fails the same way
(`watchlist show` → `profile show`, `docs search` → `search`).

The only available "fixes" are gaming — weakening `platform_client`'s genuinely-required
`ExactArgs(1)`, renaming manifest-approved commands, or deleting correct recipes. All are
forbidden by the skill and would make the CLI worse. **Retro target, not a CLI defect.**

## Verdict

`ship-with-gaps` — every gate is green except one firing incorrectly on a proven tooling defect.
The gaps above are documented here and the deferred output-review items are recorded at the
shared extraction point in `internal/psx/table.go`.

---

# Phase 4.8/4.9/4.95 Review Results (agent-dispatched)

## Local code review — 5 HIGH findings, all fixed

| # | Finding | Fix |
|---|---|---|
| 1 | `table.go` ignored `colspan`/`rowspan`, mis-keying every cell on two-level headers. `/trading-board` depth tables put the symbol under `volume` and bid price under `volume_2` — the exact wrong-but-plausible failure header-keying exists to prevent. `basis` would have computed spreads from bid *volumes*. | Added `expandHeaderRows` (colspan expansion + rowspan carry-forward + parent/sub label merge); extra cells now keyed positionally instead of truncated. Pinned by `TestParseTables_ColspanRowspanHeaders`. **Verified live:** `board REG main` now yields `bid_vol, bid_price, offer_vol, offer_price`. |
| 2 | `drift --metric market-cap` / `free-float` could never match a stored column (PSX bakes units into headers: `MARKET CAP. (B)` → `market_cap_b`), so it reported "no history — run snapshot take" for data the user already had. | `driftMetrics` now maps to candidate lists resolved against columns actually present; a genuine column miss reports the available columns instead of blaming missing history. |
| 3 | `rotation --top` emitted overlapping head/tail slices when `len(sectors) < 2*top` — 7 sectors at `top=5` returned 10 rows with 3 duplicated, double-counting any aggregate. | Guarded to `len > 2*top`. |
| 4 | `actions`: one symbol's failure discarded the whole feed's successful siblings, and the stderr denominator counted feeds while the real loss was per-symbol. | Feeds now return `(events, failures, err)`; per-symbol errors accumulate into `fetch_failures` and partial results survive. |
| 5 | `market performers --kind` indexed into a list already filtered of empty tables, so an empty "active" table made `--kind gainers` silently return **losers**, labelled gainers. | Indexes the unfiltered list; an empty selection returns an explicit note. |

## Local code review — MEDIUM/LOW fixed

Double-unescaped HTML entities (corrupted any title containing a literal `&amp;`); portal envelopes
with `status != 1` but an empty message read as success; `basis --board` concatenated unescaped
into the URL path; terminal-escape injection at 8 hand-written print sites (now
`cliutil.ScrubTerminal`); `--rate-limit`/`--timeout` were inert on every PSX command and each call
site built its own limiter; calendar dates skipped ISO normalisation, corrupting the merged sort;
`headers` marshalled as `null` on empty results; `snapshot take` exited 0 when every kind failed;
`watchlist` re-add desynced `added_price` from `added_at` and echoed a price it had not stored;
`parsePSXDate` accepted one format only (now multi-format, and `history` errors rather than
silently dropping every bar); byte-sliced truncation could split multi-byte runes; `buy_by` was
documented as a trading day but computed as a calendar day (wording corrected — holidays are not
modelled).

**Also found and fixed during verification:** `payouts deadline` returned nothing because
book closure is a *range* (`12/05/2026 - 13/05/2026`), and its payout amount was read from a
non-existent column. Both fixed; the command now returns 6 dated deadlines for OGDC with
`32.50% dividend`-style amounts.

**Reviewer verified clean:** SQLite discipline throughout (every `*sql.Rows` drained, `rows.Err()`
checked, closed on all paths, no nested queries, `sql.Null*` scan targets everywhere) and
**no SQL injection anywhere in scope** — every statement constant, the one dynamic clause bound as
a parameter.

## README/SKILL/AGENTS audit — 7 errors, addressed at source of truth

The most serious: **`snapshot take` appeared nowhere in the docs**, so four of the eight headline
features (`diff`, `drift`, `unusual`, `rotation`) returned empty for any user following the Quick
Start. Fixed in `research.json` (now step 5 of Quick Start, with an explicit note that these four
read snapshots rather than sync). Also corrected: the lede claimed announcements/payouts/OHLCV were
mirrored when they are read live; troubleshooting pointed at `sync` where `snapshot take` is
required; and the freshness "covered paths" list advertised generated `<resource> get|list|search`
paths that this CLI's hand-authored commands replaced.

Remaining doc warnings are generator-template boilerplate on a no-auth read-only CLI (a
`credentials.toml` reference, create/write bullets, `--stdin`, mutation/confirmation language) —
filed as retro findings rather than hand-patched, since they re-render on every generation.

## Final state

- **Scorecard 97/100 Grade A** in live mode (96 standalone; the delta is `live_api_verification`,
  which only the verify leg can mark verified).
- **Live dogfood 217/217, zero failures.**
- 13 Go test packages green, `go vet` clean, `tools-audit` no findings, zero polish-owned gosec findings.
