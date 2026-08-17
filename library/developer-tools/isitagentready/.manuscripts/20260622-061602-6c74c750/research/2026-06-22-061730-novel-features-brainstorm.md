# Novel-features brainstorm (audit trail) — isitagentready

_Subagent output, preserved for retro/dogfood debugging. Survivors flow into the absorb manifest transcendence table._

## Customer model

**Mara — agency front-end dev shipping client sites (the fix-advice persona, = Bobe's archetype).** Pastes client URLs into the web UI, harvests fix prompts click-by-click into her coding agent, re-pastes to recheck. Weekly Monday sweep of 6-10 client sites, levels jotted into Notion by hand. Frustration: the advice is the product but she harvests it manually, can't pipe a whole site's prompts at once, and the UI has zero memory of which fixes she already shipped.

**Diego — SEO/GEO lead with "agent discoverability" as a KPI.** Scans marketing site + 3 competitors, screenshots into slides. Thinks in deltas and us-vs-them; reports in spreadsheets (CSV matters). Frustration: stateless + single-target; no week-over-week delta, no side-by-side standards diff, no worst-to-best portfolio ranking.

**Priya — platform/DevRel engineer wiring readiness into CI.** Has a brittle curl+grep gate that can't distinguish "dropped a level" from "a check flipped," and a transient target siteError looks like a real regression so the gate flaps. Needs deterministic exit codes, machine-readable reason, and regression defined vs. the previous scan.

**Sam — standards-curious indie dev fixing one personal site.** Sees a failing check (mcpServerCard, x402) and doesn't know what it means; the SKILL.md guide opens in yet another browser tab. Wants failing checks + exact fix prompt + spec/guide all in the terminal.

## Candidates (pre-cut)

14 candidates generated; kill/keep applied inline (no LLM, no auth; only free no-auth /api/scan + public skillUrl fetches allowed).

- C1 `history` (a/c, local) — KEEP: local timeline + flip detection; API has no history endpoint.
- C2 `diff` (c, local) — KEEP: pairwise per-check transition table; distinct from history.
- C3 `compare` (b/c, auto) — KEEP: 21-check x N-site matrix across sites.
- C4 `batch` (a/c, auto) — KEEP: multi-target real scans + ranking + persistence.
- C5 `gate` (a/e, auto) — KEEP: deterministic exit + baseline regression + siteError discrimination.
- C6 `fixes <url>` (e) — CUT: re-proposes absorbed `advice --copy` + `guide`; convenience wrapper.
- C7 `open-advice` (e/c, local) — KEEP: cross-site still-failing join; embodies Bobe's "track which advice is actioned."
- C8 `watch` (a, live) — CUT: level climb is human-paced; poll just re-scans unchanged site; collapses to `check`.
- C9 `levels`/`next` (b) — CUT: absorbed by `advice`/`report`; static ladder would hardcode server levelNames.
- C10 `commerce <url>` (b) — CUT: = `report --category commerce` + isCommerce; saved flag combo.
- C11 `slowest` (b) — CUT: durationMs is Cloudflare probe latency not site speed; non-actionable.
- C12 `search "term"` (e) — CUT: already the absorbed `search` framework command.
- C13 `summary`/`stats` (c, local) — KEEP-then-CUT in drop-half: thinnest leverage; derivable from batch+open-advice.
- C14 `todo <url>` (a) — CUT: degenerate subset of `advice` (`advice --limit 1`).

## Survivors (6)

| # | Feature | Command | Score | Buildability | Persona | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|---------|--------------|----------|------------------|
| 1 | Score-over-time history + flip detection | `history <url> [--limit N] [--check <name>]` | 8/10 | hand-code | Diego, Mara | Reads local SQLite scan rows for the URL, orders by scannedAt, computes per-check pass<->fail transitions between consecutive rows | Brief Workflow #3; Data Layer (web UI stateless); Diego weekly delta + Mara fix-verification | none |
| 2 | Two-scan regression diff | `diff <a> [<b>]` | 8/10 | hand-code | Diego, Priya | Loads two scan rows (default latest two for a URL), emits per-check transition table (regressed/fixed/unchanged) + level delta | Brief Workflow #3/#5; Priya "level vs check flip" | Use `diff` for two scans of the SAME site over time. For two DIFFERENT sites at one moment, use `compare`. |
| 3 | Competitor / standards matrix | `compare <url> <url> [...]` | 8/10 | hand-code | Diego, Mara | Scans each site (or reads latest local rows per --data-source), builds 21-check x N-site matrix + each level | Brief Workflow #6; Diego "which standards did they implement" | Use `compare` for DIFFERENT sites side by side. For one site across time use `diff`/`history`. |
| 4 | Portfolio batch scan + ranking | `batch [<file>] [--rank level\|failing] [--csv]` | 8/10 | hand-code | Diego, Mara | Reads URLs from file/stdin, real scan per URL, persists, ranked leaderboard | Brief Workflow #4; Mara Monday sweep + Diego estate + CSV | none |
| 5 | CI gate with baseline regression | `gate <url> [--min-level N] [--no-regress]` | 9/10 | hand-code | Priya, Diego | Scans, compares level to --min-level and each check to last stored scan, exits nonzero on failure, treats siteError as distinct | Brief Workflow #5; siteError block; Priya flapping gate | none |
| 6 | Cross-site open-advice backlog | `open-advice [--site <url>] [--check <name>]` | 9/10 | hand-code | Mara, Diego | For each site's newest scan, selects still-failing checks, lists site + level + nextLevel fix prompt | User Vision (Bobe): track actioned advice; web UI stateless; Mara "what have I NOT fixed" | Use `open-advice` for the cross-site backlog of UNFIXED checks. For one site's full prompts use the absorbed `advice` command. |

## Killed candidates
C6 fixes (wrapper over advice+guide) | C8 watch (human-paced climb) | C9 levels (absorbed/hardcodes levelName) | C10 commerce (= report --category commerce) | C11 slowest (probe latency not site metric) | C12 search (absorbed framework) | C13 summary (thinnest leverage; derivable from batch+open-advice) | C14 todo (= advice --limit 1).
