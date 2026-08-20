# Novel Features Brainstorm — Mailchimp Marketing API

> Audit trail from the Step 1.5c.5 subagent. The Survivors table flows into the
> absorb manifest; the Customer model and Killed candidates are preserved here
> for retro/dogfood debugging.

## Customer model

**1. Lifecycle marketer at a small/mid-sized DTC company (primary).**
Owns one Mailchimp account, 5-50k members, 1-5 active campaigns/month, an attached Shopify/Woo store via the e-commerce sync. Lives in the Mailchimp dashboard but increasingly drives campaigns from a CSV (re-engagement lists, win-back, post-purchase). Pains: subscriber_hash MD5 quirk burns them every time they touch the API, batch endpoint result decoding is opaque, segment health degrades silently (empty/stale segments accumulate), campaign reports are split across 4 endpoints that have to be joined by hand to answer "did this campaign actually work."

**2. Growth/RevOps engineer at a Series A-B SaaS or DTC brand (secondary).**
Mailchimp is one of 5+ marketing tools; they integrate it into automation pipelines (Airflow, Zapier, custom Node scripts). Cares about: idempotent subscribe-tag operations driven by external triggers, ad-hoc SQL over the audience for cohort analysis, bulk operations that respect the 10-concurrent-connection cap, attribution joins between campaigns and e-commerce orders. Currently writes one-off scripts against the SDK.

**3. Agency account manager running multiple Mailchimp accounts (tertiary).**
Manages 5-30 client accounts, switches between them daily. Pains: re-auth friction (every account has a different `-dc` suffix, base URL changes), deliverability triage across accounts, pre-send checklist enforcement, bulk template/segment portability.

## Candidates (pre-cut)

Eleven candidates from persona walks. Cut to seven survivors plus rationale below.

## Survivors and kills

### Killed candidates

| Feature | Kill reason | Closest sibling |
|---------|-------------|-----------------|
| `members find` (free-text member search) | Generator's `search` framework command provides this for free over the synced FTS5 index. Listing it as novel would mis-credit framework capability as transcendence. | Replaced by built-in `search`. |
| `members sql` (ad-hoc SQL) | Same as above — `sql` is a generator-emitted framework command. Brief calls FTS5 a foundation, not a novel command. | Replaced by built-in `sql`. |
| `re-engage` (auto-flag stale members) | Overlaps with `segments audit` (surfaces stale activity) and `sql` (one query). The destructive write half ("tag at-risk") is better composed via pipe. | `segments audit` + `sql` + `lists members tags add`. |
| `multi-account` (profile switcher) | Wrong layer of the system — profile management is a Printing Press machine concern, not a per-CLI novel feature. Filed mentally as a retro candidate. | None at this CLI scope. |

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|--------------|--------------|----------|
| 1 | One-shot subscribe-tag with auto-MD5 | `mailchimp subscribe <email> --list <id> --tags <t1,t2> [--merge FNAME=...]` | **10/10** | hand-code | Calls `PUT /lists/{id}/members/{MD5(lower(email))}` then `POST /lists/{id}/members/{hash}/tags` in one call; computes the hash so the user never sees it. | Brief Workflow #1 ("single most-used workflow"), Pain Points #3 (MD5) and #4 (PUT-vs-POST); no incumbent CLI composes it. |
| 2 | CSV bulk subscribe with batch decode | `mailchimp bulk-subscribe --csv <file> --list <id> [--tags ...] [--watch]` | **9/10** | hand-code | Reads CSV, posts an operations array to `POST /batches`, polls `GET /batches/{id}` until `status=finished`, fetches `response_body_url` within the 10-minute window, decompresses tar.gz of JSONL, prints per-row success/error CSV. | Brief Workflow #2 + Pain Point #6 (10-min URL expiry, tar.gz JSONL decoding "rarely made easy"). |
| 3 | Campaign performance digest | `mailchimp digest <campaign-id>` | **10/10** | hand-code | Joins `GET /campaigns/{id}` + `GET /reports/{id}` + `GET /reports/{id}/email-activity` + `GET /reports/{id}/ecommerce-product-activity` into one summary with opens/clicks/bounces/revenue, plus top-clicked links and top-converted products. | Brief Workflow #4 ("dashboard groups these poorly"), Build Priority 3d names it explicitly. |
| 4 | Segment health audit | `mailchimp segments audit --list <id>` | **8/10** | hand-code | Reads `/lists/{id}/segments` from API, joins against synced member counts in local SQLite, flags segments with 0 members, segments that haven't grown in 90 days, and segments with no campaign reference in the last N sends. | Brief Workflow #5 + Build Priority 3e ("find empty/stale segments"). |
| 5 | Send-checklist CI gate | `mailchimp campaigns send <id> --gate` (flag on the spec-emitted send command) | **8/10** | hand-code | Calls `GET /campaigns/{id}/send-checklist` before `POST /campaigns/{id}/actions/send`; if `is_ready=false` or any item has `type=error`, exits 2 with the failing items printed; otherwise proceeds to send. | Brief Table Stakes + Build Priority 3c; agency persona explicitly needs "fail the script if checklist isn't clean." |
| 6 | E-commerce attribution per campaign | `mailchimp attribution <campaign-id> [--store <id>]` | **8/10** | hand-code | Joins `GET /reports/{id}/ecommerce-product-activity` with synced `/ecommerce/stores/{id}/orders` from local SQLite, computes attributed revenue, top products by attributed revenue, and conversion rate (orders / opens). | Brief Workflow #6 names it explicitly; brief Product Thesis singles out attribution as currently locked behind the dashboard. |
| 7 | Per-domain deliverability rollup | `mailchimp deliverability [--last <N>] [--domain <gmail.com>]` | **7/10** | hand-code | Pulls `GET /reports/{id}/domain-performance` for the last N campaigns (default 10), rolls up per-recipient-domain bounce / spam / open / click rates, surfaces domains performing below the account average. | Brief Workflow #4 fan-out + agency persona triage pain. |

### Hand-code commitment

All 7 survivors are `hand-code`. ~750 lines total novel command code across 6 new commands + 1 flag.

| Feature | Approx size | Notes |
|---------|------------|-------|
| `subscribe` | ~80 lines | MD5 hashing + two API calls |
| `bulk-subscribe` | ~200 lines | CSV reader + tar.gz/JSONL decoder; biggest hand-code |
| `digest` | ~120 lines | Parallel fetches + rendering |
| `segments audit` | ~100 lines | Local SQL aggregation |
| `campaigns send --gate` | ~30 lines | Flag on existing send command |
| `attribution` | ~120 lines | Join logic |
| `deliverability` | ~100 lines | Parallel report fetches + rollup |
