# Function Health CLI Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Email/password login → Firebase token | daveremy/function-health-mcp `login` + bogini `export --email --password` | `auth login` runs Firebase signInWithPassword, stores idToken + refreshToken in `~/.config/function-health-pp-cli/credentials.toml` with 0600 perms, auto-refreshes before expiry, handles `API_KEY_INVALID` by re-prompting | Refresh logic that actually works (daveremy issue #22 broken since 2026-05-01); credentials never echoed |
| 2 | Auth status check | daveremy `status` + `fh_status` MCP tool | `auth status` and `doctor` | Reports token TTL remaining + when sync last ran |
| 3 | Full data pull / sync | daveremy `sync [--force]` + bogini `export` | `sync` writes every entity into SQLite (results-report, requisitions, biomarkers catalog, categories, recommendations, bio-age, BMI, notes, visits, notifications) with adaptive 250 ms→ramped pacing | SQLite store, not 19 loose JSON files |
| 4 | Lightweight new-results probe | daveremy `check` / `fh_check` | `sync check` — only re-fetches the requisitions list and compares against local IDs to flag if a new round landed | Doesn't trigger a full pull |
| 5 | Query lab results with filters | daveremy `results` + `fh_results` | `results list --category cardiovascular --status out-of-range --round R1` | SQL composable, FTS5 search across biomarker name / clinician notes |
| 6 | Per-biomarker deep dive | daveremy `biomarker <name>` + `fh_biomarker` | `biomarker get <name>` (case-insensitive lookup) | Full history across every round, with Quest reference + Function optimal side-by-side |
| 7 | Health overview (bio age + BMI) | daveremy `summary` + `fh_summary` | `summary` | Single command pulling member, bio-age, BMI, and round counts from SQLite |
| 8 | List biomarker categories | daveremy `categories` + `fh_categories` | `categories list` | With counts and last-changed timestamps |
| 9 | Round-over-round comparison | daveremy `changes [--from --to]` + `fh_changes` | `changes --from R1 --to R2` | Improvements + declines, with significance gating |
| 10 | Health recommendations | daveremy `recommendations` + `fh_recommendations` | `recommendations list --category heart` | Category-filterable, includes resolution status |
| 11 | Change notifications | daveremy `notifications` / `fh_notifications` | `notifications list [--unread]` and `notifications ack <id>` | Local read/ack state |
| 12 | Full clinician report per visit | daveremy `report <visit>` + `fh_report` | `visits report <id>` | Full narrative, FTS-indexed |
| 13 | Multi-file JSON export | bogini `export` (19+ JSON files) | `export --format json --bundle` | One-shot bundle + manifest |
| 14 | Per-category Markdown export | bogini `markdown` (17 per-category files) | `export --format markdown --per-category` | LLM-ready, with biomarker history blocks |
| 15 | CSV export with Quest IDs | Greenband1 Chrome extension | `export --format csv --quest-ids` | Quest test code + LOINC-friendly fields |
| 16 | Clipboard export | Greenband1 Chrome extension | `--clipboard` flag (writes to pbcopy/clip) | Same shape as CSV |
| 17 | Status/direction side-by-side rendering | Greenband1 extension | All results render `value | unit | status (in/above/below) | Quest range | Function optimal range` | Generic |
| 18 | Date-range and category filtering | Greenband1 + bogini --max-biomarkers | `--from-date`, `--to-date`, `--category`, `--latest` | Generic via SQL and flags |
| 19 | Persistent settings | bogini `config` | `config get/set` | Standard pp-cli pattern |
| 20 | Rate-limit safe sync | daveremy 250ms + bogini exp-backoff | Uses `cliutil.AdaptiveLimiter` | Standard pp-cli pattern |

## Transcendence (only possible with our approach)

| # | Feature | Command | Persona | Score | Why Only We Can Do This |
|---|---------|---------|---------|-------|-------------------------|
| 1 | Branded doctor PDF | `export pdf-for-doctor --out <path>` | Marcus → Dr. Elena | 9/10 | Local SQLite (members, test_rounds, results, categories, biomarkers, notes) rendered via Go PDF generator with no external deps — every byte from the synced store. |
| 2 | Biomarker trend across all rounds | `biomarker trend <name>` | Marcus, Sam | 9/10 | `SELECT … FROM results JOIN test_rounds ORDER BY draw_date WHERE biomarker_id=?` from local SQLite — impossible in the JSON-file model the two competitors use. |
| 3 | Drift-toward-optimal "goat" | `goat` | Priya, Marcus | 8/10 | Local SQLite over `results` joined to `test_rounds`, computing slope + range-distance mechanically — no LLM, no external service. |
| 4 | Trending-worse cohort | `biomarkers trending --direction worse [--last 3]` | Priya, Marcus, Sam | 8/10 | SQLite window over `results` grouped by biomarker_id with linear-fit slope over last N draw_dates — purely local. |
| 5 | Category-level health-score timeline | `category trend <name>` | Marcus | 7/10 | `GROUP BY round_id` over `results` filtered by category_id with `CASE WHEN status='optimal'` count — pure SQL aggregate. |
| 6 | Oscillation detector | `biomarkers oscillating [--rounds 4]` | Marcus | 6/10 | SQLite window over `results` counting sign-changes of `(value - optimal_low) * (value - optimal_high)` across consecutive draws. |
| 7 | LLM-ready bundle composer | `bundle <biomarker> [--window 3rounds]` | Sam, Marcus | 7/10 | Local SQLite + FTS5 across `results`, `reports`, `recommendations`, `notes` joined by biomarker name — one query, one Markdown render. |
| 8 | Recommendation resolution tracker | `recommendations stale` | Marcus | 6/10 | SQL join between `recommendations` and the latest two `results` per biomarker — flags recs whose target biomarker hasn't moved into Function-optimal range. |

All transcendence features score ≥ 6/10. No stubs planned — every row is shipping scope.

## Compound use case (single ritual that exercises multiple features)

> **Sunday morning — Marcus's full ritual.** `function-health-pp-cli sync check` (60s if no new round, full pull if there is). Then `function-health-pp-cli goat` to see the most worrying biomarker right now. Then `function-health-pp-cli biomarker trend ApoB --window 1y` to confirm direction. Then `function-health-pp-cli bundle ApoB --window 3rounds | pbcopy` to paste into Claude with the question "given my supplement stack from these notes, what should I change?" Then once a year, `function-health-pp-cli export pdf-for-doctor --out ~/Downloads/function-2026.pdf` and email Dr. Elena.
