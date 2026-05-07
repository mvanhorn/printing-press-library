# Customer.io novel-features brainstorm (audit trail)

## Customer model

### Persona 1: Maya — Lifecycle Marketing Manager at a 200-employee Series C SaaS

**Today (without this CLI):** Maya lives in the Customer.io web UI. She has six tabs open: Campaigns, Broadcasts, Segments, Deliveries, the data warehouse Looker tab, and a Google Sheet where she copy-pastes campaign metrics every Monday for the marketing standup. When her CMO asks "what fraction of the Q2 onboarding segment opened the welcome journey?", she pulls the segment export, downloads a CSV of campaign deliveries, and stitches them in a pivot table. The journey funnel UI doesn't expose the cross-section she needs.

**Weekly ritual:** Monday morning, pull metrics for ~12 active campaigns + 4 active broadcasts. Mid-week, send 1-3 one-off broadcasts to special segments. Friday, review the week's transactional delivery health (bounce rate spike? new ESP complaints?). End of month, export segment members for finance + an attribution audit.

**Frustration:** The reporting UI is the #1 cited pain point. Every cross-resource question — "which customers in segment A also opened campaign B in the last 30 days?" — requires two exports and a spreadsheet. Journey funnel data is locked in a UI chart she can't pivot.

### Persona 2: Devon — Lifecycle Ops / Marketing Engineer

**Today (without this CLI):** Devon owns the integration plumbing. He maintains a Reverse-ETL job from Snowflake that syncs identified users nightly, debugs `cio api` calls in his terminal, and gets paged when transactional bounce rate spikes. He keeps a folder of curl scripts named `trigger-broadcast.sh`, `unsuppress-batch.sh`, `dump-segment-members.sh`. When a teammate asks "did the welcome SMS go out to the 04-12 cohort?", he hits the deliveries endpoint via curl, jq's the result, and pastes a screenshot into Slack.

**Weekly ritual:** Daily — check Reverse-ETL sync status and re-trigger failed syncs. 2-3x per week — bulk-suppress a list of complained or churned users from a CSV. Once a week — answer an ad-hoc "did this delivery actually fire?" question. Monthly — rotate SA tokens and verify auth across workspaces.

**Frustration:** Bulk suppress/unsuppress is a chore — the App API takes one customer at a time, and he has no audit log of what he suppressed last Tuesday. The 1 req/10s broadcast throttle has bitten him with naive retry loops twice.

### Persona 3: Priya — Product Engineer integrating identify/track from app code, on-call for delivery health

**Today (without this CLI):** Priya wrote the `customerio-node` integration in the product. When a delivery alert fires (Pagerduty: "transactional bounce rate > 5%"), she opens the Customer.io web UI, filters deliveries by the affected template, scrolls through 50+ rows, copies a few delivery IDs into a Notion incident doc, and tries to correlate timestamps with a recent code deploy. She does this maybe twice a month, but each incident eats 30+ minutes just gathering data.

**Weekly ritual:** Mostly hands-off; the integration runs itself. But on-call rotations bring her into Customer.io 2-4 times a month for triage. She also runs `cio api` occasionally to verify a single customer's `attributes` after a code change.

**Frustration:** Incident triage in the UI is slow. She wants `customer-io deliveries list --template <id> --since 1h --status bounced` piped into `claude "summarize the failure pattern"`, not a UI scroll-fest.

## Candidates (pre-cut)

(Full Pass 2 output — see archived subagent response for the original wording. 16 candidates generated; verdicts inline.)

C1 funnel · C2 overlap · C3 bulk suppress audit · C4 delivery triage bundle · C5 transactional test-matrix · C6 customer 360 timeline · C7 segment diff · C8 broadcast preflight · C9 export auto-resume · C10 reverse-etl health · C11 suppression audit · C12 recipient fatigue · C13 schema diff · C14 identify-from-jsonl · C15 engagement matrix · C16 doctor --deep

## Survivors and kills

### Survivors (8, all scored >= 6/10)

| # | Feature | Command | Score | How It Works | Persona |
|---|---|---|---|---|---|
| 1 | Journey funnel × segment | `customer-io campaigns funnel <id> [--segment <id>] [--since 7d]` | 9/10 | Joins local `deliveries` + `segment_members` tables to compute step-by-step (sent→delivered→opened→clicked→converted) cross-cut by segment. The API exposes per-campaign journey_metrics but no per-segment breakdown. | Maya |
| 2 | Segment overlap | `customer-io segments overlap <id-a> <id-b> [<id-c>...]` | 8/10 | Pure SQL over local `segment_members` table; emits Venn region counts and (with `--show-ids`) overlapping customer IDs. | Maya, Devon |
| 3 | Customer 360 timeline | `customer-io customers timeline <email-or-id> [--since 30d]` | 8/10 | Joins local `customers` + `deliveries` + `suppressions` + `segment_members` for one customer; chronological event stream. | Devon, Priya |
| 4 | Broadcast pre-flight | `customer-io broadcasts preflight <id> [--segment <id>]` | 8/10 | Calls live `segments/{id}/customer_count` + `suppressions list` + reads local `deliveries` for last-sent recency; emits green/yellow/red verdict with structured reasons. | Maya, Devon |
| 5 | Suppression audit | `customer-io suppressions audit [--since 30d] [--reason ...]` | 7/10 | Joins local `suppressions` + `deliveries` to attribute each suppression to a preceding bounce/complaint or "manual." | Devon |
| 6 | Reverse-ETL health | `customer-io cdp reverse-etl health [--since 24h] [--watch]` | 7/10 | Calls live RETL endpoints + joins synced run history; status + row counts + error reasons per job. | Devon |
| 7 | Bulk suppress with audit log | `customer-io suppressions bulk add --from-csv <file>` (also `bulk remove`) | 6/10 | Reads CSV/stdin, fans out real suppress API calls with adaptive throttle, appends every call to local JSONL audit log. | Devon |
| 8 | Delivery triage bundle | `customer-io deliveries triage --template <id> --status bounced --since 1h --bundle <dir>` | 6/10 | Filters live + local deliveries; writes `bundle/{summary.md, deliveries.jsonl, recipients.txt}` for incident handoff. | Priya |

### Killed candidates

| Feature | Kill reason | Closest sibling |
|---|---|---|
| C5 transactional test-matrix | Speculative weekly use; QA happens 1-2x per launch, not weekly. | C4 delivery triage |
| C7 segment diff | Requires snapshot history table that `sync` doesn't currently maintain. Real value but data-model addition pushes feasibility down. | C2 segment overlap |
| C12 recipient fatigue | Strong overlap with C8 broadcast preflight (which uses last-sent recency). As standalone, it's a thin wrapper over `SELECT recipient, COUNT(*)`. | C4 broadcast preflight |
| C13 schema diff | Monthly-at-most use. `diff` between two cached YAMLs is one shell command. | manifest #2 schema list/show |
| C14 identify-from-jsonl | Covered by manifest #4 (batch via stdin). | manifest #4 |
| C15 engagement matrix | Overlaps with C1 funnel; once `sql` ships, matrix is one query away. | C1 funnel |
| C16 doctor --deep | Enhancement of manifest #37, not a separate command. Fold into existing doctor. | manifest #37 doctor |

## Reprint verdicts

N/A (first print).
