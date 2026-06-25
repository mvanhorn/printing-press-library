# YesWeHack CLI Absorb Manifest

## Source Tools Catalogued

| # | Tool | Repo | Auth | Language | Audience |
| - | - | - | - | - | - |
| 1 | yeswehack-mcp | sebastianolaru3008/yeswehack-mcp | browser+JWT+PAT | Python | Claude MCP integration |
| 2 | ywh_program_selector | jdouliez/ywh_program_selector | JWT bearer + TOTP | Python | Researcher CLI |
| 3 | YesWeBurp | yeswehack/YesWeBurp (official) | email+password+TOTP | Kotlin | Researcher (Burp extension) |
| 4 | YesWeCaido | yeswehack/yeswecaido (official) | JWT paste | Vue/TS | Researcher (Caido extension) |
| 5 | ywh2bugtracker | yeswehack/ywh2bugtracker (official) | PAT (manager) | Python | Program-manager (issue-tracker sync) - out of researcher scope |

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
| - | - | - | - | - |
| 1 | List public + invited programs | yeswehack-mcp list_programs / ywh_program_selector --get-progs | `programs list` with filters (--bounty, --vdp, --search, --business-unit, --disabled, --min-reward, --max-reward) backed by sync'd SQLite | Offline, FTS5 search, agent-native --json/--select, no rate limit per query, no JWT required for public programs |
| 2 | Get program detail | yeswehack-mcp get_program | `programs get <slug>` rendering rules, reward grid, scope counts, BU, status | Read from local store first (offline), fall through to live with --live flag, --json/--select, no auth required for public programs |
| 3 | List program scopes (in + out) | YesWeBurp / YesWeCaido | `programs scopes <slug>` listing every asset, type, severity, in/out | JWT-only on YesWeHack but we cache locally so users can re-query offline; --type filter (web, mobile, api, iot); --output for Burp/Caido/proxychains exports |
| 4 | Export scopes to security-tool config | ywh_program_selector --extract-scopes / YesWeBurp / YesWeCaido | `programs scopes <slug> --export burp\|caido\|proxychains\|nuclei\|httpx` | Five export formats vs the 1-2 each competitor offers, all driven by the same SQLite-cached scope table |
| 5 | Find programs by scope URL | ywh_program_selector --find-by-scope | `scopes find <url-pattern>` with regex + glob support, returns matching programs | Works across ALL synced scopes including private, regex pattern matching, --json output, no live API call |
| 6 | Program scoring / ranking | ywh_program_selector --show | `programs rank` with default heuristics (recency, report-volume, scope-size, reward-ceiling) and per-flag tweaks (--weight-recency, --weight-reward) | Multiple ranking dimensions, exposed via SQL, fully agent-driven |
| 7 | Detect collaboration partners | ywh_program_selector --collaborations | `programs collabs` showing programs you share with other hunters (when reports are co-disclosed) | Same logic but available from CLI + SKILL recipes |
| 8 | Get current user | yeswehack-mcp get_current_user | `user get` returning username, slug, points, rank, impact, kyc_status | Cached locally, also exposes `user invitations`, `user email-aliases`, `user reports` as sibling commands |
| 9 | List my submitted reports | yeswehack-mcp list_reports | `user reports` with --status filter, --program filter, --since-days | Status filter spans the full state machine; SQLite-backed so filtering happens locally |
| 10 | Get report detail | yeswehack-mcp get_report | (Endpoint-level only — researchers can fetch their OWN report by id once user gets one; cannot fetch others') | When the user has reports, `user reports get <id>` |
| 11 | List report comments | yeswehack-mcp list_report_comments | (Bound to report scope; same auth limit as above) | Per-report SQLite cache |
| 12 | List user email aliases | yeswehack-mcp list_email_aliases | `user email-aliases` returning per-program forwarding addresses | Locally indexed by program slug |
| 13 | Get program credentials | yeswehack-mcp get_program_credentials | (Endpoint returns 404 for researcher accounts on the programs we tested, so commands gated behind real-account verification) | Will surface when the API allows; otherwise tagged "manager-only" in help |
| 14 | Browse hacktivity feed | yeswehack-mcp get_hacktivity | `hacktivity list` with --vulnerable-part, --program, --since-days filters | Full filter set, syncs entire feed to local SQLite for offline learning |
| 15 | Browse hunter's hacktivity | yeswehack-mcp `yeswehack_api_get` (raw escape hatch) | `hacktivity by-hunter <username>` typed first-class | Typed command not buried in escape hatch; supports SQL queries across all synced hunters |
| 16 | Get hunter public profile | (no competitor exposes) | `hunters get <username>` returning points, rank, impact, achievements | Brand new typed surface |
| 17 | List hunter achievements | (no competitor exposes) | `hunters achievements <username>` | Brand new typed surface |
| 18 | Browse leaderboard | (no competitor exposes) | `ranking list` (paginated, filterable, SQLite-cached) | Brand new typed surface |
| 19 | List business units | (no competitor exposes) | `business-units list` | Brand new typed surface |
| 20 | List events | (no competitor exposes) | `events list` | Brand new typed surface; calendar view via `events upcoming` |
| 21 | Vulnerability-part taxonomy lookup | yeswehack-mcp `yeswehack_api_get` (escape hatch) | `taxonomies vulnerable-parts` typed | Tag autocomplete during report drafting; SQLite-cached |
| 22 | Country / profile-URL reference | (used internally by SPA only) | `taxonomies countries`, `taxonomies profile-urls` | Useful for the report-drafting flow |
| 23 | Browser-based auth (no JWT paste) | YesWeBurp / YesWeCaido (manual JWT paste) | `auth login --chrome` reads `localStorage.access_token` from the user's Chrome profile and writes to config | No copy-paste from DevTools; one command does it. Beats every competitor whose docs tell users to "open DevTools, Application > Local Storage, copy access_token". (Status: shipping scope.) |
| 24 | Cache / sync management | ywh_program_selector --force-refresh | `sync`, `sync --full`, `sync programs`, `sync hacktivity`, with per-resource cursors | More granular than --force-refresh; resumes interrupted syncs |
| 25 | Stored credentials path | ywh_program_selector --local-auth / --auth-file | `auth status`, `auth logout`, config at `~/.config/yeswehack-pp-cli/` | Standard PP auth pattern with `doctor` for diagnostics |
| 26 | Generic API escape hatch | yeswehack-mcp `yeswehack_api_get` | (Implicitly via `sql` + `search` over the local store; no raw HTTP escape hatch ships) | Forces structured local queries instead of raw API ad-libs - aligned with the Code-of-Conduct anti-spam stance |

## Transcendence

The transcendence features will be added below after the novel-features subagent runs.

### Transcendence (only possible with our approach)

| # | Feature | Command | Score | Why Only We Can Do This |
| - | - | - | - | - |
| 1 | Scope drift report | `programs scope-drift [--since-days 7]` | 9/10 | Requires periodic snapshots of every program's scope table; pure local SQL diff. No competitor stores history. |
| 2 | Cross-program shared assets | `scopes overlap [--min-programs 2]` | 8/10 | SQL self-join across all your programs' scopes; surfaces an asset attached to 3 programs and picks the best payout. Competitors only show per-program scope. |
| 3 | Weekend slate | `triage weekend [--hours 6]` | 8/10 | Composes scope-drift + reports-needing-response + hacktivity CWE trends into a single ranked list. No single API call returns this. |
| 4 | Pre-submit dedupe | `report dedupe --title --asset --cwe` | 9/10 | FTS5 BM25 over local hacktivity_items + own reports. Returns collisions with overlap score, exit code 2 if a high-confidence match exists. Honors the Platform Code of Conduct's anti-spam rule. |
| 5 | Program fit ranker | `programs fit [--specialty xss,ssrf]` | 7/10 | Joins your-accepted-CWE histogram x program-hacktivity-CWE histogram x reward grid. Cross-entity join nobody else has. |
| 6 | Hacktivity CWE trends per program | `hacktivity trends <program> [--since-days 90]` | 7/10 | GROUP BY over hacktivity_items filtered to one program; emits count, avg_bounty, p50_severity. Aggregation a generic API client cannot do. |
| 7 | Calendar of program events | `events calendar --mine` | 6/10 | Joins events x invited programs x reports awaiting payout. Three-table join from local sync. |
| 8 | CVSS sanity check | `report cvss-check <vector> [--steps <file>]` | 7/10 | Deterministic CVSS 3.1 parser + rule-based violation detector (e.g. AV:N claimed but steps say "no remote access"). No LLM, rules only - so the result is auditable and reproducible. |
| 9 | Draft report scaffold | `report draft <program-slug>` | 7/10 | Writes a markdown file pre-filled with program-specific reward grid, accepted severity, allowed asset picker pulled from local scopes table. No network call. Quality multiplier per Platform CoC. |
| 10 | Submit a draft (guard-railed) | `report submit <draft-file> [--confirm]` | 6/10 | Dry-run default. --confirm runs asset-in-scope check, auto-runs `report dedupe`, then POSTs. No batch, no template-flood. Honors CoC anti-spam mandate. |
| 11 | Hacktivity learning brief | `hacktivity learn --program <slug> --cwe XSS [--since-days 90]` | 6/10 | FTS5 + GROUP BY filtered to (program, cwe, window). Pipe-friendly JSON for `| claude "summarize tactics"`. Externalizes any LLM step so the CLI stays deterministic. |

## Personas (from novel-features brainstorm)

- **Lena**: full-time hunter, 30+ Chrome tabs, weekly review ritual, frustrated by missed scope drift.
- **Diego**: agent-driven researcher (matches the user's stated vision), frustrated by no agent-shaped way to ask "is this a duplicate".
- **Priya**: weekend hunter, 4-8 hours, wants a single command that says "here's your weekend slate".

Full brainstorm including killed candidates: `<run>/research/2026-05-10-225249-novel-features-brainstorm.md`

