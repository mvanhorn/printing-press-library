# YesWeHack Novel-Features Brainstorm

## Customer model

**Persona 1: Lena, the full-time bug bounty hunter**
- **Today (without this CLI):** Lena keeps 30+ Chrome tabs open: the YesWeHack programs list, three program scope pages she's actively hunting, her dashboard of submitted reports, and the hacktivity feed. She has a janky bash script that wraps `curl` against `api.yeswehack.com` with a JWT she copy-pastes from devtools every other day. She cannot answer "which of my 12 invited programs have new scope this week" without manually opening each one.
- **Weekly ritual:** Sunday night she reviews payout status on her 6 open reports, scans for new program invitations, picks two programs to focus on, exports their scope to Burp targets, and reads the last 30 days of disclosed XSS reports in her program's industry vertical to calibrate severity.
- **Frustration:** Scope drift she misses. A program quietly adds `api-v3.example.com` to in-scope and she finds out two weeks later when a competing researcher banks the bounty.

**Persona 2: Diego, the agent-driven researcher (matches the User Vision verbatim)**
- **Today (without this CLI):** Diego pairs with a Claude/Codex agent. He drops "qualify this program" into chat and the agent... can't, because YesWeHack has no read-friendly API for agents. He pastes scope as text, the agent does its best, signal gets lost.
- **Weekly ritual:** Mornings he asks his agent "what should I work on today?" and expects a ranked list: programs by reward-to-difficulty fit, fresh scope, his unfinished drafts, reports needing his response. Then he hands the agent a hypothesis and asks it to research the platform's prior art before he goes deep.
- **Frustration:** No agent-shaped way to ask "have I or anyone publicly already reported this title/CWE/asset combination" before he writes the report. He either duplicates and gets dinged, or wastes hours validating manually.

**Persona 3: Priya, the weekend hunter with a day job**
- **Today (without this CLI):** Priya has 4-8 hours on weekends. She doesn't read hacktivity daily, doesn't catch new programs, and shows up to a program that's already saturated. She wants surgical signal: "what's worth my Saturday."
- **Weekly ritual:** Saturday morning, two coffees, asks: which 2 programs added scope this week, what categories are currently paying out fastest on hacktivity, are any of her old reports awaiting her reply.
- **Frustration:** Every tool assumes infinite time. She wants a single command that says "here's your weekend slate" and she trusts the ranking enough to go.

## Candidates (pre-cut)

(16 candidates; see survivor + kill tables for final disposition.)

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Persona | How It Works | Evidence |
| - | - | - | - | - | - | - |
| 1 | Scope drift report | `programs scope-drift [--since-days 7]` | 9/10 | Lena, Priya | Diffs `scope_snapshot` table rows across two timestamps for each invited program; emits added/removed/modified asset rows with first-seen date. Pure local SQL, no API call per invocation. | Brief Data Layer explicitly calls out scope snapshot tables; Build Priorities #3 names "scope drift detection"; absorbed competitors (YesWeBurp, ywh_program_selector) ship zero drift detection. |
| 2 | Cross-program shared assets | `scopes overlap [--min-programs 2]` | 8/10 | Lena | SQL self-join on `scopes` table grouping by normalized asset (apex domain + wildcard expansion), ranks by best payout grid across the overlapping programs. | Brief Build Priorities #3 names "scope cartography (which assets span multiple programs)"; absorbed competitors offer per-program scope only, never cross-program. |
| 3 | Weekend slate | `triage weekend [--hours 6]` | 8/10 | Priya, Lena | Local-data composition: top-N rows from `programs scope-drift` (last 7d) + `user reports` where `needs_action='you'` + top 3 CWEs from last 30d hacktivity in user's specialty (derived from their accepted reports' CWE histogram). Single mechanical ranking, no LLM. | Brief User Vision: agent must "pick up challenges... to get qualified"; Top Workflow #1 (discover and qualify) is the explicit core ritual; no competing tool composes the three signals. |
| 4 | Pre-submit dedupe | `report dedupe --title <s> --asset <s> --cwe <s>` | 9/10 | Diego, Lena | FTS5 BM25 search over local `hacktivity_items` + user's own `reports` rows. Returns ranked collisions with overlap score; exit code 2 if a high-confidence match exists. Mechanical, no LLM. | Brief User Vision: "submit BETTER reports, not MORE"; Brief Build Priorities #3 names "report dedupe against own prior reports"; CoC quality-multiplier mandate. |
| 5 | Program fit ranker | `programs fit [--specialty <list>]` | 7/10 | Diego, Priya | Joins user's accepted-report CWE histogram x per-program hacktivity CWE histogram x program reward tier; scores programs by `your_strength * their_payout * invitation_state`. Pure SQL. | Brief User Vision verbatim: "be GREAT at the program"; Top Workflow #1; cross-entity join is the unique angle competitors cannot offer. |
| 6 | Hacktivity CWE trends per program | `hacktivity trends <program> [--since-days 90]` | 7/10 | Lena, Diego | `GROUP BY vulnerable_part, cwe` over `hacktivity_items` filtered to one program; outputs count, avg_bounty, p50_severity. | Brief Top Workflow #5 names this exact query ("top XSS findings in the last 90 days on fintech programs"); absorbed competitors only return raw hacktivity feed without aggregation. |
| 7 | Calendar of program events | `events calendar --mine` | 6/10 | Lena, Priya | Joins `events` table x user-invited `programs` x user `reports.state='paid_pending'`. Emits chronological list of renewals, CTF gates, and payout dates. | Brief lists `events` as an absorbed read endpoint with no join to programs/reports; cross-entity join is non-obvious and competitor tools don't surface it. |
| 8 | CVSS sanity check | `report cvss-check <vector> [--steps <file>]` | 7/10 | Diego | Parses CVSS 3.1 vector string, recomputes base score deterministically, flags rule violations (e.g. `AV:N/PR:H` claimed alongside steps text mentioning "no auth required" via keyword regex). | Brief Build Priorities #3 names "CVSS sanity prediction (rule-based, not LLM)"; CoC anti-spam ethos rewards a pre-flight check; no competitor ships this. |
| 9 | Draft report scaffold | `report draft <program-slug>` | 7/10 | Diego, Lena | Writes a markdown file with program-specific sections derived from the program's `accepted_severity`/`scope_types`/`reward_grid`; pre-fills allowed asset picker from local `scopes`. No network call. | Brief Build Priorities #3 names "draft-report local workflow"; User Vision: submit BETTER reports; absorbed competitors are all read-only on the draft surface. |
| 10 | Submit a draft (guard-railed) | `report submit <draft-file> [--confirm]` | 6/10 | Diego | Dry-run prints what would POST. With `--confirm`, validates asset-in-scope via local `scopes`, runs `report dedupe` automatically, then `POST /reports`. No batch flag, no template-substitution. | Brief User Vision: agent must submit but with guard-rails; CoC anti-AI-slop mandate; absorbed competitors ship zero write commands. |
| 11 | Hacktivity learning brief | `hacktivity learn --program <slug> --cwe <s> [--since-days 90]` | 6/10 | Diego, Lena | FTS5 + GROUP BY on `hacktivity_items` filtered to (program, cwe, window); emits top N disclosures with bounty, severity, vulnerable-part, redacted writeup link. Pipe-friendly JSON for `| claude "summarize tactics"`. | Brief Top Workflow #5 verbatim use case; rubric LLM-dependency check passed by externalizing summarization. |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
| - | - | - |
| C8 Report status board | Thin grouping wrapper over already-absorbed `user reports`; output can be reproduced with `--format json | jq group_by`. Fails wrapper-vs-leverage check. | C3 Weekend slate. |
| C12 Specialist lookup per program | Hunter attribution on hacktivity is frequently redacted, so the join produces a sparse, misleading table. Fails verifiability check. | C5 Program fit ranker. |
| C13 Scope-to-tool export bundle | Already in absorb manifest row 4. | absorbed feature. |
| C14 Email-alias picker for outbound | Single-table filter the absorbed `user email-aliases` command covers with a flag. | absorbed feature row 12. |
| C16 Program freshness watch | Overlaps heavily with C1 scope-drift and C3 weekend slate signals; no distinct workflow. | C3 Weekend slate. |
