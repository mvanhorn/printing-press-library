## Customer model

**Persona 1 — Priya, the on-call platform engineer at a fintech**

Today: She owns the SMS pipeline that fires OTPs and fraud alerts. When a customer-success agent slacks "did this person ever get the code?", Priya pastes a curl command into a one-off terminal, copies an access key from 1Password, hits Bird's `/messages/{id}` and then `/messages/{id}/interactions`, and stitches the two responses together by eye to answer one question.

Weekly ritual: Every Friday she exports the last 7 days of delivery interactions to spot bad-carrier clusters before they bleed into the weekend. The export is a Python script she wrote in 2024 that no one else can read. She also re-runs a "did we leak any STOP-keyword opt-outs into next week's campaign list?" check by hand.

Frustration: Two API calls per audit and zero local memory of yesterday's traffic. Every question that asks "across the last week" turns into a paginated crawl over `/messages` plus a janky aggregation in jq.

**Persona 2 — Marcus, the growth/lifecycle marketer**

Today: He fires bulk transactional and reminder SMS campaigns from a CSV of recipients. He uses the legacy MessageBird Node SDK inside a small Express server he wrote himself; idempotency, retry, and "which rows failed" are all glue code he maintains.

Weekly ritual: Monday morning campaign dispatch (200–2,000 SMS). Tuesday: reconcile failures, retry the soft-failed ones, hand the hard-failed rows to a colleague for list cleanup. Wednesday: cross-check inbound STOP replies against the next campaign list so unsubscribers aren't messaged again.

Frustration: Send and reconcile are two unrelated scripts. There is no shared concept of "batch" — the failure CSV he emails to the support lead on Tuesdays is hand-built. He has no quick way to ask "every SMS we ever sent to +31612345678" because the API only paginates by channel or conversation.

**Persona 3 — Sana, the AI agent / bot vendor onboarding new Bird tenants**

Today: She ships a customer-support AI agent that lives on top of a customer's Bird workspace. Every new tenant onboarding is a 12-step manual flow: confirm channel exists, confirm channel-config is enabled, anti-spam on, compliance keywords set (HELP/STOP/START), messageability green for a test contact. Today this is a runbook in Notion and a sequence of curls.

Weekly ritual: One or two new tenants come online per week. Each one takes 30–60 minutes of stepping through the runbook, plus another half hour of "why isn't outbound flowing?" debugging when something silently fails.

Frustration: There is no single signal "this tenant is ready." Failures show up only when a real customer-support conversation hits Bird and the bot has nothing to reply with. By then she's already on a call apologizing.

**Persona 4 — Devon, the customer-support team lead**

Today: He triages open conversation threads across SMS for a mid-sized e-commerce brand. Bird's web inbox is fine for a single thread, but he can never get a clean picture of "all the threads that have been idle ≥ 14 days but are still marked active" or "every message we ever exchanged with that customer regardless of which thread it lived in."

Weekly ritual: Friday inbox grooming — close stale threads, reassign frozen ones. Spot-check the team's outbound replies via web UI. Quarterly: pull a customer's full conversation history when legal asks.

Frustration: Composite questions ("stale + active + this channel") have no single endpoint; he ends up paging through hundreds of threads in the web UI. Cross-conversation views for one customer don't exist on the Bird side at all.

## Candidates (pre-cut)

| # | Candidate | Source | Verdict (kill/keep with rubric note) |
|---|-----------|--------|--------------------------------------|
| C1 | `messages audit <id>` — fold message + interactions into a single delivery timeline, exit non-zero on terminal failure | (d) prior-keep + (a) persona Priya | **Keep.** Two-endpoint client-side join. Buildable: `/messages/{id}` + `/messages/{id}/interactions`. Persona Priya weekly. |
| C2 | `messages failures --since 24h --group-by reason` — local aggregation of recent interaction failures by reason code | (d) prior-keep + (c) cross-entity local | **Keep.** SQL over locally-synced interactions. Bird has no aggregation endpoint. Persona Priya weekly. |
| C3 | `sms search "<query>" --to <num>` — FTS5 over message bodies + recipient identifier | (d) prior-keep + (c) cross-entity local | **Keep.** FTS5 over messages joined to participants. No Bird body-search endpoint. All four personas use it. |
| C4 | `sms send-batch --csv recipients.csv --body-template "..."` — batch send with idempotency keys, persist batch locally | (d) prior-keep + (a) persona Marcus | **Keep.** Calls real `POST /channels/{id}/messages` per row, writes batch metadata to local store. Persona Marcus weekly. |
| C5 | `sms reconcile <batchId> --retry-failed` — re-fetch interactions per message in the batch, group by reason, retry | (d) prior-keep + (a) persona Marcus | **Keep.** Reads persisted batch, calls real `/interactions` per message. Persona Marcus weekly. Pair with C4. |
| C6 | `sms tail --channel-id ... --since 5m` — poll Channels list + interactions, render newest first | (d) prior-reframe + (a) persona Priya | **Reframe-keep.** Real polling against `GET /channels/{id}/messages`. Useful during incidents (Priya, Sana). Risk: drift toward "monitor" scope — descope to single-poll-with-`--watch`. |
| C7 | `conversations timeline <id>` — chronological merge of messages + participants + interactions | (d) prior-keep + (c) cross-entity local | **Keep.** SQL join over four local tables. Persona Devon weekly; Sana on tenant debugging. |
| C8 | `messages from <phone-number>` — every message exchanged with one phone across all conversations | (d) prior-keep + (c) cross-entity local | **Keep.** Joins participants → conversations → messages locally. API only paginates by channel/conversation. Persona Devon + Priya. |
| C9 | `conversations stale --older-than 14d --status active` — active conversations with no message activity in N days | (d) prior-keep + (c) cross-entity local | **Keep.** Local SQL. Persona Devon weekly. |
| C10 | `tenant doctor --test-contact <id>` — sequence channels + channel-config + antispam + compliance keywords + messageability into one checklist, single exit code | (d) prior-keep + (a) persona Sana | **Keep.** Five real API endpoints chained. Persona Sana per-onboarding (1-2/week). Highest leverage. |
| C11 | `compliance auto-block --since 7d` — scan local inbound for STOP-keyword fires, emit CSV ready for bulk-add | (d) prior-keep + (b) service-specific content pattern | **Keep.** Local FTS over inbound bodies; optional `--apply` calls real `workspace rules bulk-add`. Persona Marcus + Priya weekly. |
| C12 | `workspace rules diff <csv>` — diff local CSV against current allow/block rules, print plan, optional `--apply` | (d) prior-keep + (a) persona Marcus | **Keep.** Plan/apply pattern over real `rules list` + `rules bulk-add`. Persona Marcus weekly during campaigns. |
| C13 | `messages cost --since 7d --group-by channel` — total spend / segments sent over a window from local store | (e) user-briefing extrapolation | **Kill.** Bird does not expose per-message cost on the messages endpoint in scope (cost lives in billing/analytics surfaces outside the SMS Conversations scope). No mechanical version. Fails Kill Check "Reimplementation." |
| C14 | `sms throughput --since 1h --by-minute` — per-minute outbound throughput from local store | (c) cross-entity local | **Kill.** Useful but thin — already implied by `sms tail` and `messages failures`. Fails Sibling Kill against C2 + C6. |
| C15 | `conversations assign <id> --to <agent>` — bulk-assign stale conversations to an agent | (a) persona Devon | **Kill.** Bird Conversations API doesn't expose an assignment primitive in the in-scope surface (assignments live in the Inbox/agent product, out of scope per brief). Fails Kill Check "Auth/scope the user doesn't have." |
| C16 | `participants merge <a> <b>` — merge two participant records for the same human across channels | (a) persona Devon | **Kill.** Bird offers no merge endpoint; mechanical version (alias map in local store) doesn't transcend a CSV. Fails Build Feasibility + Reimplementation. |
| C17 | `messages export --since 7d --format csv` — bulk export local messages to CSV for offline analysis | (a) persona Marcus | **Kill / fold.** Useful but the printed CLI's generated `export` framework command already covers this. Fails Sibling Kill against built-in export. |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Persona Served | How It Works (buildability proof) | Evidence | Source |
|---|---------|---------|-------|----------------|-----------------------------------|----------|--------|
| 1 | Delivery audit | `messages audit <id>` | 9/10 | Priya, Sana | Calls real `GET /messages/{id}` + `GET /messages/{id}/interactions`, folds into chronological events list, exits non-zero on terminal failure reason code. | Brief Top Workflow #3 + prior research.json (shipped + used). | prior (kept) |
| 2 | Failure cluster | `messages failures --since 24h --group-by reason` | 8/10 | Priya, Marcus | Local SQL aggregation over the synced `messages` × `interactions` tables, grouped by interaction reason code. Bird exposes no aggregation endpoint. | Brief Top Workflow #3 + Data Layer §FTS/search note. | prior (kept) |
| 3 | SMS body search | `sms search "<query>" --to <num>` | 9/10 | All four personas | FTS5 index over `messages.body` joined to `participants.identifierValue`; filters by `--from` / `--to` phone. | Brief Data Layer §FTS/search + prior research.json. | prior (kept) |
| 4 | CSV bulk dispatch | `sms send-batch --csv recipients.csv --body-template "..."` | 9/10 | Marcus | One real `POST /channels/{channel_id}/messages` per CSV row with a deterministic per-row idempotency key; batch metadata + per-row state written to local store. | Brief Top Workflow #5 + prior research.json (shipped). | prior (kept) |
| 5 | Batch reconcile | `sms reconcile <batchId> --retry-failed` | 9/10 | Marcus | Reads persisted batch rows from local store, calls real `GET /messages/{id}/interactions` per message, folds outcomes by reason. | Brief Top Workflow #5 + prior research.json (shipped). | prior (kept) |
| 6 | Conversation timeline | `conversations timeline <id>` | 8/10 | Devon, Sana | Local SQL join over `conversations` × `messages` × `participants` × `interactions` in canonical chronological order. | Brief Top Workflow #2 + prior research.json (shipped). | prior (kept) |
| 7 | Cross-channel customer history | `messages from <phone-number>` | 8/10 | Devon, Priya | Local join: `participants` filtered by `identifierValue = <phone>` → `conversations` → `messages`. | Brief Data Layer §Primary entities + prior research.json (shipped). | prior (kept) |
| 8 | Tenant readiness check | `tenant doctor --test-contact <id>` | 10/10 | Sana, Priya | Sequences five real API calls: `GET /channels` (filtered SMS), `GET /channels/{id}` channel-config, `GET /antispam-settings`, `GET /compliance/keywords`, `GET /channels/{id}/messageability?contact=...`. | Brief Top Workflow #4 + prior research.json (shipped). | prior (kept) |
| 9 | Compliance auto-block | `compliance auto-block --since 7d` | 7/10 | Marcus, Priya | Local FTS over inbound `messages.body` for STOP-shape patterns within the window; emits a CSV; with `--apply` calls real `POST /workspace/rules/bulk-add`. | Brief Top Workflow #4 + prior research.json (shipped). | prior (kept) |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| C6 `sms tail` | Risks scope creep into "monitor" / persistent process; single-poll `--watch` value is thin once `messages failures` and `sms search` exist; fails Sibling Kill against C2 + C3 for the weekly-use question. | C2 `messages failures` for incident triage; C3 `sms search` for "did this just go through?" |
| C12 `workspace rules diff` | Persona Marcus is real but the underlying surface (`workspace rules bulk-add` already absorbed) plus C11's `--apply` path covers the same workflow; standalone `diff` is a thin wrapper that fails Wrapper-vs-Leverage. | C11 `compliance auto-block` (carries the same plan/apply pattern with real value-add). |
| C13 `messages cost` | Cost data is out of scope per brief (billing/analytics surfaces excluded); fails Reimplementation Kill Check. | C2 `messages failures` for the "what went wrong this week" question. |
| C14 `sms throughput` | Useful but redundant with C2's time-bucketing and C6's tail; fails Sibling Kill. | C2 `messages failures`. |
| C15 `conversations assign` | Assignment primitive isn't in the in-scope Conversations API surface (lives in the Inbox/agent product, excluded by brief). | C9 `conversations stale` for the underlying "clean up the inbox" job. |
| C16 `participants merge` | No Bird endpoint exists; the mechanical local-alias version would be a reimplementation that doesn't carry through to the platform. | C8 `messages from <phone>` already crosses participant identifiers. |
| C17 `messages export` | Already covered by the generator's built-in `export` framework command; fails Sibling Kill. | Generator-provided `export`. |
| C9 `conversations stale` (cut on second pass) | Survives kill checks but loses the cut against the 8 stronger survivors when targeting ~half-cut from 12 keep-candidates; weekly use is real but value is thin against `conversations timeline` + `messages from <phone>` combined. | C6 `conversations timeline` + filter on `updatedAt`. |

## Reprint verdicts

| # | Prior feature | Verdict | Justification |
|---|--------------|---------|---------------|
| 1 | Delivery audit (`messages audit`) | **Keep** | Top Workflow #3 dead-center; two real endpoint calls; Persona Priya/Sana weekly; scores 9/10. |
| 2 | Failure cluster (`messages failures`) | **Keep** | Brief explicitly calls out aggregation gap; Persona Priya weekly; 8/10. |
| 3 | SMS search (`sms search`) | **Keep** | Brief Data Layer §FTS/search names this exact use case; serves all four personas; 9/10. |
| 4 | CSV bulk dispatch (`sms send-batch`) | **Keep** | Top Workflow #5; Persona Marcus weekly; idempotency pattern is the leverage; 9/10. |
| 5 | Batch reconcile (`sms reconcile`) | **Keep** | Pairs with #4; without it #4 is half a feature; 9/10. |
| 6 | Live SMS tail (`sms tail`) | **Drop** | Single-poll value is thin once #2 and #3 ship; risks scope drift toward "monitor"; fails Sibling Kill. |
| 7 | Conversation timeline (`conversations timeline`) | **Keep** | Persona Devon weekly; Sana for tenant debugging; 8/10. |
| 8 | Cross-channel customer history (`messages from`) | **Keep** | Persona Devon + Priya; the API genuinely lacks this view; 8/10. |
| 9 | Stale conversations (`conversations stale`) | **Drop** | Survives kill checks but loses the second-pass cut to stronger siblings; the same answer is available via `conversations timeline` + a `--status active` filter on the absorbed `conversations list` command. |
| 10 | Tenant readiness check (`tenant doctor`) | **Keep** | Highest-leverage feature; Persona Sana every onboarding; 12-curl runbook collapsed to one command; 10/10. |
| 11 | Auto-block from STOP fires (`compliance auto-block`) | **Keep** | Persona Marcus + Priya weekly; carries the plan/apply pattern that absorbed `rules diff`; 7/10. |
| 12 | Bulk-rule diff (`workspace rules diff`) | **Drop** | Thin wrapper once #11's `--apply` path exists and `workspace rules bulk-add` is absorbed; fails Wrapper-vs-Leverage. |
