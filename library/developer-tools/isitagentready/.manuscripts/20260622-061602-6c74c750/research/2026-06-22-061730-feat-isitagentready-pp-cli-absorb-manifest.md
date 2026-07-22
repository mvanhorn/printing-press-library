# Is It Agent Ready - Absorb Manifest

API surface is a single no-auth endpoint (`POST /api/scan`). There is no competing CLI to out-absorb; the
"everything that exists" baseline is the isitagentready web UI plus the official `scan_site` MCP tool. We match
every web-UI capability, make it scriptable + offline, and add the local-store workflows adjacent readiness CLIs
(searchstack-aeo, geoskills) proved but the web UI lacks.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Scan a URL for agent readiness | isitagentready web scan / official MCP `scan_site` | `(generated endpoint) scan run` | offline persistence, `--json`/`--select`, scriptable, exit codes |
| 2 | Friendly scan: level + per-category summary, persisted | web UI result card | `isitagentready-pp-cli check` | one command; stores history for diff/trend; agent-native output |
| 3 | Readiness level 0-5 + levelName | web UI score card | `(behavior in isitagentready-pp-cli check)` prints level + levelName | also persisted so history/diff can use it |
| 4 | Per-check pass/fail/neutral across 5 categories + messages | web UI checks section | `isitagentready-pp-cli report` | filterable, scriptable, offline from stored scan |
| 5 | Prioritized next-level fix advice (description + prompt + shortPrompt + specUrls) | web UI "Improve the score" sheet | `isitagentready-pp-cli advice` | pasteable; reads from stored scan offline; **headline (Bobe's priority)** |
| 6 | Fetch + render the SKILL.md fix guide (skillUrl) | web UI guide links | `isitagentready-pp-cli guide` | renders markdown in-terminal, no browser tab |
| 7 | Evidence inspection (the requests/responses the scanner ran) | web UI evidence rows | `(behavior in isitagentready-pp-cli report --evidence)` | scriptable, bulk, offline |
| 8 | Filter by category / only-failing / single check | web UI Customize panel (client-side) | `(behavior in isitagentready-pp-cli report)` via `--category`/`--only-failing`/`--check` | server ignores filter params; we filter client-side cleanly |
| 9 | Site-type profile view (all / content / apiApp) | web UI profile pills | `(behavior in isitagentready-pp-cli report --profile)` | display filter over stored scan |
| 10 | Commerce signals / isCommerce surfacing | web UI commerce category | `(behavior in isitagentready-pp-cli report)` surfaces commerce checks + isCommerce | flagged in summary; feeds `compare` commerce row |
| 11 | Copy-all fix prompts as one pasteable block | web UI "Copy all instructions" | `(behavior in isitagentready-pp-cli advice --copy)` | pipe straight into a coding agent |
| 12 | Raw JSON + field selection + CSV | agent-native beat (web UI has none) | `(behavior in isitagentready-pp-cli check)` via `--json`/`--select`/`--compact`/`--csv` | machine-consumable; not possible in web UI |
| 13 | Health check / API reachability | framework | `isitagentready-pp-cli doctor` | reports endpoint reachability |
| 14 | Full-text search over stored scans (messages/advice/evidence) | framework / searchstack-aeo parity | `isitagentready-pp-cli search` | offline grep advice across all history |
| 15 | Raw SQL over local scan store | framework | `isitagentready-pp-cli sql` | composable analytics over scan history |

No stubs. Every absorbed row ships fully.

## Transcendence (only possible with our approach: local SQLite scan history)

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | CI gate with baseline-regression + siteError discrimination | `gate <url> [--min-level N] [--no-regress]` | 9/10 | hand-code | Scans, compares `level` to `--min-level` and each check to the last stored scan, exits non-zero on failure; treats target `siteError{}` as a distinct non-failing outcome unless `--strict` | Brief Workflow #5 (CI gating); Reachability `siteError` block; searchstack-aeo CI parity; Priya's flapping grep gate | none |
| 2 | Cross-site open-advice backlog | `open-advice [--site <url>] [--check <name>]` | 9/10 | hand-code | For each site's newest local scan, selects checks still `fail` and lists site + level + that check's `nextLevel` fix prompt | User Vision (Bobe: track which advice is actioned across rescans); web UI is stateless; Mara "what have I NOT fixed across clients" | Use `open-advice` for the cross-site backlog of UNFIXED checks. For one site's full prompts use `advice`. |
| 3 | Score-over-time history + flip detection | `history <url> [--limit N] [--check <name>]` | 8/10 | hand-code | Reads local scan rows for the URL ordered by `scannedAt`, computes per-check pass<->fail transitions between consecutive scans | Brief Workflow #3; web UI stateless; geoskills geo-monitor parity; Diego delta + Mara fix-verification | none |
| 4 | Two-scan regression diff | `diff <a> [<b>]` | 8/10 | hand-code | Loads two stored scans (default latest two of a URL), emits per-check transition table (regressed/fixed/unchanged) + level delta | Brief Workflow #3/#5; Priya "dropped a level vs a check flipped" | Use `diff` for two scans of the SAME site over time. For two DIFFERENT sites at one moment, use `compare`. |
| 5 | Competitor / standards matrix | `compare <url> <url> [...]` | 8/10 | hand-code | Scans each site (or reads latest local rows per `--data-source`) and builds a 21-check x N-site implemented/not matrix + each site's level | Brief Workflow #6; geoskills geo-compare parity; Diego "which standards did they implement that we didn't" | Use `compare` for DIFFERENT sites side by side. For one site across time use `diff`/`history`. |
| 6 | Portfolio batch scan + ranking | `batch [<file>] [--rank level\|failing] [--csv]` | 8/10 | hand-code | Reads URLs from a file or stdin, runs a real scan per URL, persists each, prints a leaderboard ranked by level or failing-check count | Brief Workflow #4; searchstack-aeo/geoskills batch parity; Mara Monday sweep + Diego estate + CSV reporting | none |

Transcendence hand-code count: 6 (all rows). Auto-emitted (spec): 0 transcendence rows (the single endpoint covers absorbed #1 only).

Persona audit trail and killed candidates: see `2026-06-22-061730-novel-features-brainstorm.md`.
