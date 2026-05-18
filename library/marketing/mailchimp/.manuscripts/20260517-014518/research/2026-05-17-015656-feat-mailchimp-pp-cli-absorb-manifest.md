# Mailchimp CLI Absorb Manifest

## Spec source
- **Spec**: `https://raw.githubusercontent.com/mailchimp/mailchimp-client-lib-codegen/main/spec/marketing.json` (Swagger 2.0, converted to OpenAPI 3.0 at `research/mailchimp-oas3.json`)
- **Resources**: 67, **Endpoints**: 291
- **Auth**: HTTP Basic `anystring:{API_KEY}`; datacenter encoded in key suffix
- **MCP shape**: 291 typed + ~15 framework + ~7 novel ≈ 313 total tools — recommend Cloudflare pattern (transport: [stdio, http], orchestration: code, endpoint_tools: hidden)

## Absorbed (match or beat everything that exists)

Every endpoint of the Mailchimp Marketing API is reachable through the generator's typed Cobra commands and the runtime cobratree-walked MCP surface. Highlights below; the long tail of CRUD endpoints (verified domains, file manager, batch webhooks, etc.) is exposed identically. Added value: `--json`, `--select`, `--csv`, `--dry-run` (mutations), `--limit`, local FTS5 + SQL over synced data.

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | All ~291 endpoints reachable | Official Node/Python SDKs, 6 third-party MCP servers, 5 stale CLI repos | Generator-emitted typed Cobra commands + cobratree-walked MCP tools | Identical reach + offline FTS over synced data + token-efficient MCP (Cloudflare pattern) |
| 2 | Audience/list CRUD | Official SDKs | `mailchimp lists [get/create/update/delete]` | --json, --select, --csv |
| 3 | Member CRUD | Official SDKs, 4 MCP servers | `mailchimp lists members [get/create/update/upsert/delete]` | --dry-run, auto-MD5 hash |
| 4 | Tag management | Official SDKs, 3 MCP servers | `mailchimp lists members tags [add/remove/list]` | --dry-run, multi-tag |
| 5 | Segments CRUD + members | Official SDKs | `mailchimp lists segments [...]` | endpoint mirrors |
| 6 | Merge fields | Official SDKs | `mailchimp lists merge-fields [...]` | endpoint mirrors |
| 7 | Campaign CRUD | Official SDKs, 6 MCP servers | `mailchimp campaigns [...]` | --dry-run |
| 8 | Campaign content + send actions | Official SDKs, 3 MCP servers | `mailchimp campaigns content/send/send-test/schedule/pause/resume/replicate` | --dry-run, --file <html> |
| 9 | Send checklist | Official SDKs | `mailchimp campaigns checklist <id>` | endpoint mirror |
| 10 | Reports (all 9 sub-resources) | Official SDKs, 6 MCP servers | `mailchimp reports [opens/clicks/email-activity/sent-to/unsubscribed/abuse-reports/locations/domain-performance/ecommerce-product-activity]` | --json, --select |
| 11 | Automations (classic) | Official SDKs, 3 MCP servers | `mailchimp automations [list/get/pause/start/archive/emails/queue]` | endpoint mirrors |
| 12 | Customer Journeys trigger | Mailchimp dev portal | `mailchimp customer-journeys trigger` | endpoint mirror |
| 13 | E-commerce stores + customers/products/orders/carts | Official SDKs, 4 MCP servers | `mailchimp ecommerce stores [...]` | endpoint mirrors |
| 14 | Promo rules + codes | Official SDKs | `mailchimp ecommerce stores promo-rules/promo-codes` | endpoint mirrors |
| 15 | Templates + template folders | Official SDKs | `mailchimp templates`, `template-folders` | endpoint mirrors |
| 16 | Campaign folders | Official SDKs | `mailchimp campaign-folders` | endpoint mirrors |
| 17 | File manager (files + folders) | Official SDKs | `mailchimp file-manager [files/folders]` | endpoint mirrors |
| 18 | SMS campaigns | Mailchimp dev portal | `mailchimp sms-campaigns` | endpoint mirrors |
| 19 | Landing pages | Official SDKs | `mailchimp landing-pages` | endpoint mirrors |
| 20 | Surveys | Official SDKs | `mailchimp surveys` | endpoint mirrors |
| 21 | Conversations / replies | Official SDKs | `mailchimp conversations` | endpoint mirrors |
| 22 | Connected sites | Official SDKs | `mailchimp connected-sites` | endpoint mirrors |
| 23 | Verified domains | Official SDKs | `mailchimp verified-domains` | endpoint mirrors |
| 24 | Batches (start/list/get/delete) | Official SDKs, 2 MCP servers | `mailchimp batches` | endpoint mirrors |
| 25 | Batch webhooks | Official SDKs | `mailchimp batch-webhooks` | endpoint mirrors |
| 26 | Search members + campaigns | Official SDKs (`searchMembers`, `searchCampaigns`), 3 MCP servers | `mailchimp search-members`, `search-campaigns` | endpoint mirrors |
| 27 | Ping / health | Official SDKs | `mailchimp ping` + `mailchimp doctor` | doctor adds dc validation + auth probe |
| 28 | OAuth dc lookup | Mailchimp dev portal | `mailchimp auth dc-lookup` | endpoint mirror |
| 29 | Activity feed | Mailchimp dev portal | `mailchimp activity` | endpoint mirror |
| 30 | Member notes/events/goals/activity | Official SDKs | `mailchimp lists members [notes/events/goals/activity]` | endpoint mirrors |
| 31 | Local FTS over synced data | None | Built-in `search` framework command over members/campaigns/reports/tags | New capability — no competitor has this |
| 32 | Local SQL over synced data | None | Built-in `sql` framework command over local SQLite | New capability — no competitor has this |
| 33 | Incremental sync with cursors | None | Built-in `sync` command using `since_*`/`before_*` ISO timestamps | New capability |
| 34 | Mock-mode + golden fixtures | None | Generator-emitted `--mock` for every command | New capability — agent dev workflow |

No competing tool ships items 31-34. These are generator framework guarantees, not novel features — listed here to establish the baseline.

## Transcendence (only possible with our approach)

Hand-code commitment: **8 features, ~930 lines**. Persona-validated. Score ≥ 7/10.

| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|--------------|--------------|----------|
| 1 | One-shot subscribe-tag with auto-MD5 | `mailchimp subscribe <email> --list <id> --tags <t1,t2> [--merge FNAME=...]` | **10/10** | hand-code | Calls `PUT /lists/{id}/members/{MD5(lower(email))}` then `POST /lists/{id}/members/{hash}/tags` in one transaction; computes the hash so the user never sees it. | Brief Workflow #1 ("single most-used workflow"); Pain Points #3 (MD5) + #4 (PUT-vs-POST); no incumbent CLI composes it. |
| 2 | CSV bulk subscribe with batch decode | `mailchimp bulk-subscribe --csv <file> --list <id> [--tags ...] [--watch]` | **9/10** | hand-code | Reads CSV, posts operations array to `POST /batches`, polls `GET /batches/{id}` until finished, fetches `response_body_url` within the 10-minute window, decompresses tar.gz of JSONL, prints per-row success/error CSV. | Brief Workflow #2 + Pain Point #6 (10-min URL expiry, tar.gz JSONL decoding "rarely made easy"). |
| 3 | Campaign performance digest (single + rollup) | `mailchimp digest <campaign-id> [--md]` OR `mailchimp digest --last <N> [--md]` | **10/10** | hand-code | **Single-campaign mode** (`digest <id>`): joins `GET /campaigns/{id}` + `GET /reports/{id}` + `GET /reports/{id}/email-activity` + `GET /reports/{id}/ecommerce-product-activity` into one summary with opens/clicks/bounces/revenue + top-clicked links + top-converted products. **Rollup mode** (`digest --last N` or `digest --week`): fetches reports for the last N campaigns in parallel, renders one row per campaign with subject/sends/opens/CTR/revenue + aggregate stats at top. `--md` renders either shape as paste-ready markdown for Notion / Slack / client weekly docs. | Brief Workflow #4 ("dashboard groups these poorly"); Build Priority 3d names it explicitly; 6+ third-party MCP servers exist, none ship a digest. Two personas at Phase Gate 1.5: **Marcus's Monday founder doc** (rollup) and **Priya's Friday client reports** (single-campaign). |
| 4 | Segment health audit | `mailchimp segments audit --list <id>` | **8/10** | hand-code | Reads `/lists/{id}/segments`, joins synced member counts in local SQLite, flags segments with 0 members, segments that haven't grown in 90 days, segments with no campaign reference in the last N sends. | Brief Workflow #5 + Build Priority 3e ("find empty/stale segments"). |
| 5 | Send-checklist CI gate | `mailchimp campaigns send <id> --gate` (flag on the spec-emitted send command) | **8/10** | hand-code | Calls `GET /campaigns/{id}/send-checklist` before `POST /campaigns/{id}/actions/send`; if `is_ready=false` or any item has `type=error`, exits 2 with the failing items printed. | Brief Table Stakes + Build Priority 3c; agency persona needs "fail the script if checklist isn't clean." |
| 6 | E-commerce attribution per campaign | `mailchimp attribution <campaign-id> [--store <id>]` | **8/10** | hand-code | Joins `GET /reports/{id}/ecommerce-product-activity` with synced `/ecommerce/stores/{id}/orders` from local SQLite, computes attributed revenue + top products + conversion rate (orders / opens). | Brief Workflow #6; Product Thesis singles out attribution as locked behind the dashboard. |
| 7 | Per-domain deliverability rollup | `mailchimp deliverability [--last <N>] [--domain <gmail.com>]` | **7/10** | hand-code | Pulls `GET /reports/{id}/domain-performance` for the last N campaigns, rolls up per-recipient-domain bounce/spam/open/click rates, surfaces domains performing below the account average. | Brief Workflow #4 fan-out; agency persona triage pain. |
| 8 | Head-to-head campaign comparison | `mailchimp compare <campaign-a> <campaign-b> [--md]` | **9/10** | hand-code | Parallel fetch of both campaigns' `/reports/{id}` + `/reports/{id}/email-activity`. Renders side-by-side metric diff (opens, CTR, click-to-open, bounces, unsubscribes, revenue), picks a winner per metric with delta, surfaces subject-line / send-time / audience-size differences. `--md` renders as paste-ready markdown for Notion / Slack. | User-added at Phase Gate 1.5: "things for the personas that help them easily compare campaigns" — lifecycle marketer's "did the subject line work?", agency's "which variant won?" Marcus & Priya personas. |

### Hand-code scope summary

| Feature | LoC | Type |
|---------|-----|------|
| `subscribe` | ~80 | Composes 2 spec-emitted endpoints + MD5 hashing |
| `bulk-subscribe` | ~200 | CSV reader + `/batches` + tar.gz/JSONL decoder |
| `digest` | ~200 | Single-campaign join (4 endpoints) + multi-campaign rollup (last N) + `--md` markdown render |
| `segments audit` | ~100 | Local SQL aggregation over synced segments + members |
| `campaigns send --gate` | ~30 | Flag on spec-emitted send command |
| `attribution` | ~120 | Spec-emitted endpoint + local orders join |
| `deliverability` | ~100 | Parallel report fetches + per-domain rollup |
| `compare` | ~140 | Parallel report fetches for two campaigns + side-by-side render + `--md` |
| **Total** | **~970** | 7 new commands + 2 flags |

## Reachability + Risk

- **Reachability**: probed `https://us6.api.mailchimp.com/3.0/ping` — `mode: standard_http`, 401 without auth (expected). No CDN block, no challenge.
- **Rate limits**: 10 concurrent connections per key. Mitigation: serialize default; bulk paths use `/batches`.
- **Datacenter routing**: required runtime behavior. Phase 3 will parse `-dc` from `MAILCHIMP_API_KEY` and override base URL at request time.
- **Spec version drift**: codegen repo on 3.0.91, official SDKs on 3.0.80. We pin the codegen snapshot.

## Stubs

None. All 8 novel features are shipping scope. No `(stub)` rows.
