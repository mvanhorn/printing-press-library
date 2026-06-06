# Novel Features Brainstorm — function-health

## Customer model

### Persona 1: Marcus, 47, biohacker / quantified-self practitioner
**Today (without this CLI):** Logs into my.functionhealth.com every few weeks, scrolls his results-report page, screenshots biomarker tiles into Apple Notes, maintains a clumsy Google Sheet with ApoB / hs-CRP / fasting insulin columns he hand-types after each draw. Has the daveremy CLI installed but it's been broken since 2026-05-01 on token refresh. Pipes the JSON exports into ChatGPT with prompts like "tell me what changed." Cannot answer: "is my ApoB actually trending toward Function's optimal range, or just oscillating?"

**Weekly ritual:** Sunday morning. Coffee, opens the Function dashboard, looks for new results, re-reads the most recent clinician report, scribbles a note about whether his last supplement stack moved any markers. Re-exports the same JSON every time because he doesn't trust himself to remember which version he last fed Claude.

**Frustration:** No across-rounds view. The web UI shows one round at a time. To compare four rounds of ApoB he opens four tabs.

### Persona 2: Dr. Elena, 52, Marcus's primary-care MD (recipient persona)
**Today (without this CLI):** Marcus emails her a 19-file ZIP from bogini once a year. She opens two of them, gets confused, asks him to "just send the PDF from Function" — which doesn't exist for the full history, only for the single most-recent visit. Ends up squinting at the web report Marcus screenshotted.

**Weekly ritual:** Sees 4-5 patients/week who bring outside lab data. Her workflow is: glance at a printable PDF, dictate one line into her EMR, move on. She does not log in to anything for outside data.

**Frustration:** No printable, branded, name+DOB header'd multi-round PDF exists. She gets unbranded JSON or single-visit PDFs and can't trust provenance.

### Persona 3: Priya, 38, longevity podcast listener / casual member
**Today (without this CLI):** Pays the $499/yr, gets two draws, mostly forgets to look at them. Vaguely remembers her thyroid was "a bit low" last time. Opens the app once a quarter, sees a notification she didn't read, dismisses it. Has never downloaded her data.

**Weekly ritual:** None. Reactive. When her trainer or doctor asks "what's your last hs-CRP," she fumbles in the app for 10 minutes.

**Frustration:** Cannot get a one-line answer to "is anything getting worse?" without scrolling a 100-biomarker grid.

### Persona 4: Sam, 41, Claude/ChatGPT power user, software engineer-adjacent
**Today (without this CLI):** Runs daveremy's MCP server in Claude Desktop. Asks "what's my ferritin trend?" — gets back a JSON blob with one round in it because daveremy stores per-round JSON files and the MCP tool reads the latest. Token refresh is broken so half his sessions die mid-query.

**Weekly ritual:** Adds Function data to his "personal context" Claude project once a month, asks deep questions, gets shallow answers because the agent only sees one slice.

**Frustration:** The agent has no cross-round view. He's the user who actually needs `--json --select` + MCP read-only annotations across the full history.

## Candidates (pre-cut)

| # | Name | Command | Description | Persona | Source |
|---|------|---------|-------------|---------|--------|
| C1 | Branded doctor PDF | `export pdf-for-doctor --out <path>` | Function-branded multi-round PDF with member name + DOB header, per-category sections, history sparklines per biomarker, suitable for emailing to a personal MD | Marcus → Dr. Elena | (e) user vision |
| C2 | Biomarker trend across all rounds | `biomarker trend <name>` | Every value of a biomarker across every round with delta vs Function-optimal range; ASCII sparkline in TTY, structured JSON for agents | Marcus, Sam | (c)(f) SQLite cross-round joins |
| C3 | Drift-toward-optimal score | `goat` | Ranks every biomarker by (distance from optimal) × (slope away across last 3 rounds); returns top 1 with reasoning fields | Priya, Marcus | (c) brief P2; rubric "goat" pattern |
| C4 | Oscillation detector | `biomarkers oscillating [--rounds 4]` | Finds biomarkers that crossed in/out of optimal-range boundary >= 2 times in last N rounds | Marcus | (c) SQL window over rounds |
| C5 | Trending-worse cohort | `biomarkers trending --direction worse [--last 3]` | Every biomarker whose slope across last N rounds points away from optimal, sorted by magnitude | Priya, Marcus | (c) brief P2 |
| C6 | Category-level health-score timeline | `category trend <name>` | For one of the ~13 categories, per-round aggregate (% biomarkers in Function-optimal range) over time | Marcus | (c) cross-entity aggregate |
| C7 | Reorder cadence reminder | `cadence` | User's typical inter-draw interval + days since last + overdue flag | Priya | (c) draw_date arithmetic |
| C8 | LLM-ready bundle composer | `bundle <biomarker> [--window 3rounds]` | Single Markdown with biomarker's full history + clinician notes (FTS5) + Function-optimal range + recommendation text — ready to paste into Claude/ChatGPT | Sam, Marcus | (c) FTS5 across entities |
| C9 | Auth refresh that actually works | (folded into `auth login`/`auth refresh`) | Fixes daveremy #22; not a new command but a competitive feature | All | (f) daveremy #22 |
| C10 | Cross-round biomarker diff one-liner | `biomarker diff <name> --from R1 --to R2` | Single-biomarker version of `changes` | Marcus, Sam | (c) on top of absorbed `changes` |
| C11 | MD-facing visit briefing | `brief-for-doctor --visit <id>` | One-page Markdown summary of most recent visit: top 5 out-of-optimal biomarkers, their trend, clinician note excerpt | Dr. Elena, Marcus | (e)(c) |
| C12 | Biomarker-mention search across reports | `search-reports "ApoB"` | FTS5 over clinician report narratives | Sam, Marcus | (c) FTS5 over reports |
| C13 | Biomarker UUID stability scan | `biomarkers schema-drift` | Uses persisted biomarker UUIDs to detect when Function renames/replaces a biomarker mid-history | Marcus | (f) UUID persistence |
| C14 | Recommendation resolution tracker | `recommendations stale` | Recs issued ≥ 1 round ago whose associated biomarker has NOT moved into optimal range | Marcus | (c) joins recommendations + results |
| C15 | Full-history JSON for any biomarker | (folded into existing `biomarker <name> --json`) | Duplicate of absorbed #6 | — | absorbed |
| C16 | "What's new since last sync" digest | `digest --since last-sync` | One-screen Markdown: new round? new recs? unread notifications? biomarkers that crossed optimal threshold? | Priya, Marcus | (c) sync cursor + change detection |

## Survivors and kills

### Survivors

| # | Feature | Command | Description | Persona | Score | Buildability proof |
|---|---------|---------|-------------|---------|-------|---------------------|
| 1 | Branded doctor PDF | `export pdf-for-doctor --out <path>` | Function-branded multi-round PDF with name + DOB header, per-category sections, history sparklines, suitable for emailing a personal MD | Marcus → Dr. Elena | 9/10 | Local SQLite (members, test_rounds, results, categories, biomarkers, notes) rendered via Go PDF generator with no external deps — every byte from the synced store. |
| 2 | Biomarker trend across all rounds | `biomarker trend <name>` | Every value across every round with delta vs Function-optimal range, ASCII sparkline + JSON | Marcus, Sam | 9/10 | `SELECT … FROM results JOIN test_rounds ORDER BY draw_date WHERE biomarker_id=?` from local SQLite — impossible in the JSON-file model. |
| 3 | Drift-toward-optimal "goat" | `goat` | Ranks every biomarker by (distance from optimal) × (slope away across last 3 rounds); returns top 1 | Priya, Marcus | 8/10 | Local SQLite over `results` joined to `test_rounds`, computing slope + range-distance mechanically — no LLM, no external service. |
| 4 | Trending-worse cohort | `biomarkers trending --direction worse [--last 3]` | Every biomarker whose slope across last N rounds points away from optimal, sorted by magnitude | Priya, Marcus, Sam | 8/10 | SQLite window over `results` grouped by biomarker_id with linear-fit slope over last N draw_dates — purely local. |
| 5 | Category-level health-score timeline | `category trend <name>` | Per-round aggregate (% biomarkers in Function-optimal range) over time for one of ~13 categories | Marcus | 7/10 | `GROUP BY round_id` over `results` filtered by category_id with `CASE WHEN status='optimal'` count — pure SQL aggregate. |
| 6 | Oscillation detector | `biomarkers oscillating [--rounds 4]` | Biomarkers that crossed optimal-range boundary ≥ 2 times in last N rounds | Marcus | 6/10 | SQLite window over `results` counting sign-changes of `(value - optimal_low) * (value - optimal_high)` across consecutive draws. |
| 7 | LLM-ready bundle composer | `bundle <biomarker> [--window 3rounds]` | Single Markdown with biomarker history + clinician notes mentioning it (FTS5) + Function-optimal range + recommendations | Sam, Marcus | 7/10 | Local SQLite + FTS5 across `results`, `reports`, `recommendations`, `notes` joined by biomarker name — one query, one Markdown render. |
| 8 | Recommendation resolution tracker | `recommendations stale` | Recs issued ≥ 1 round ago whose associated biomarker has not moved into optimal — recs the user didn't act on or that didn't work | Marcus | 6/10 | SQL join between `recommendations` and the latest two `results` per biomarker. |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| C7 Reorder cadence reminder | Thin: one `MAX(draw_date) - prev draw_date`; Function's app already nags about scheduling; not weekly use for any persona | C5 trending-worse cohort |
| C9 Auth refresh fix | Not a novel feature — folded into absorbed table-stakes `auth login` / `auth refresh` | (absorbed feature #1) |
| C10 Single-biomarker cross-round diff | Wrapper-shaped: one flag away from absorbed `changes`; doesn't transcend | absorbed `changes` |
| C11 MD-facing visit briefing | Sibling of doctor PDF (C1) without the PDF's authority artifact; Dr. Elena won't read another Markdown file | C1 branded doctor PDF |
| C12 Search-reports FTS | Wrapper-shaped: absorbed feature #5 already exposes FTS5 via `results list` | absorbed `results` + FTS5 from #5 |
| C13 Biomarker schema-drift scan | Verifiability fails — speculative until we see drift in the wild | C2 biomarker trend |
| C15 (duplicate) | Duplicate of absorbed #6 | absorbed feature #6 |
| C16 Sync digest | Overlaps absorbed `notifications` + `sync check`; novel part better expressed as a flag on survivors | absorbed `notifications` + C5 trending |
