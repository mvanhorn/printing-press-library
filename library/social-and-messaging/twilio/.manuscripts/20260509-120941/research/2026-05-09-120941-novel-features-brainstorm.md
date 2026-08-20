# Novel Features Brainstorm — Twilio CLI (audit trail)

> Full subagent output. Customer model and killed-candidates table preserved
> for retro/dogfood debugging per the novel-features-subagent contract.

## Customer model

**Diego — SMS-first product engineer at a 50-person fintech**
- Today: Ships transactional SMS (OTP, payment confirmations, fraud alerts) from a Node service. Twilio is one of four vendors he integrates. Sends ~40k SMS/month across one main account and a "staging" subaccount.
- Weekly ritual: Monday morning, he pulls the prior week's `messages:list` for `Status=failed` and `Status=undelivered`, joins it against his app's user table, and files tickets for users who didn't receive their OTP. He also spot-checks unit cost — engineering leadership keeps asking "are we still at 0.79¢ per SMS or did the carrier surcharges drift?"
- Frustration: `twilio-cli` paginates 50 records per page with a 1–3s cold start each call; pulling a 7-day failed-message slice means 20+ paginated calls and a 2-minute wait. The official CLI has no offline cache and no way to ask "for these 47 failures, what was the carrier breakdown?" without writing a shell loop. He's tried Steampipe but doesn't want a Postgres FDW running on his laptop.

**Priya — Ops lead at a 6-person agency running 32 client subaccounts**
- Today: Each of Priya's clients is a Twilio subaccount. End of every month she invoices them for SMS + voice usage at her markup. She also rotates webhook URLs on ~120 IncomingPhoneNumbers when clients move from staging to prod.
- Weekly ritual: Friday afternoon, she runs `usage/records/lastMonth` for each subaccount across SMS/MMS/Voice/Recording categories. The official CLI returns one row per category per subaccount; she pastes them into a spreadsheet manually. She also audits each subaccount's IncomingPhoneNumbers for "numbers we're paying $1/month for but haven't sent a message from in 30 days" — pure manual cross-reference today.
- Frustration: There is no "list every subaccount and total their last-month spend by category in one CSV" command anywhere in the Twilio ecosystem. Steampipe's plugin can do it with SQL but the FDW setup makes it a no-go for her teammates. Idle-number reclamation is back-of-the-napkin guesswork.

**Marc — On-call SRE for a contact-center product**
- Today: Owns a Twilio Voice deployment with ~800 concurrent agents on Conferences. When a customer reports "my call dropped," he needs to find the CallSid, pull the recording, and check whether the conference participant left abnormally — usually within 10 minutes of the page.
- Weekly ritual: Pages happen at random hours. He keeps a terminal open and `grep`s through Slack for the CallSid, then runs `twilio api:core:calls:fetch --sid CAxxxx`, then `twilio api:core:recordings:list --call-sid CAxxxx`, then `curl` the media URL — three round trips and a manual stitch. On Wednesdays he reviews the prior week's call-status distribution to catch carrier issues early.
- Frustration: The Call→Recording→Transcription chain is three separate API calls every time, and there's no single command that gives him the full "everything that happened on this call" view. He also has no way to ask "of all calls last hour, which got `status=failed` with `answered_by=machine_start`?" without paginating client-side.

**Lena — Compliance auditor at a US healthcare SaaS**
- Today: Quarterly, Lena exports 90 days of SMS to prove HIPAA/TCPA compliance (no PHI in body, opt-outs honored within 24h). She also reconciles UsageRecords against the finance team's invoice from Twilio.
- Weekly ritual: Pulls `messages:list` with `DateSent>=` and `DateSent<=`, dumps to CSV, runs grep for forbidden keywords. Cross-references opt-out STOP messages against subsequent sends to the same `To` number.
- Frustration: The 90-day export is 40k rows across 800 paginated requests; the official CLI takes ~25 minutes and silently drops on rate-limit. She has no FTS over `Body` and no way to ask "did we send anything to a number after they replied STOP?" without writing custom Python.

## Candidates (pre-cut)

| # | Source | Candidate | Verdict |
|---|---|---|---|
| C1 | (a) Diego | `delivery-failures --since 7d` — local groupby over Messages where Status IN (failed,undelivered) by ErrorCode + To country prefix, with cost-of-failures total | KEEP |
| C2 | (a) Priya | `subaccount-spend --period last-month --csv` — fan-out usage/records per subaccount, pivot to wide CSV | KEEP |
| C3 | (a) Marc | `call-trace <CallSid>` — Call + Recordings + Transcriptions + Conference + Participants in one structured output | KEEP |
| C4 | (a) Lena | `opt-out-violations` — local query: STOP-style inbound joined to subsequent outbounds to same number | KEEP |
| C5 | (b) status state machine | `message-status-funnel --since 24h` — terminal status distribution + median time-to-delivery | KEEP |
| C6 | (b) status state machine | `call-disposition --since 24h` — Status × AnsweredBy cross-tab with $ cost | KEEP |
| C7 | (b) per-resource cost | `cost-by-category --period this-month --by subaccount` — pivots usage records to subaccount × category matrix | KILL — sibling of C2 |
| C8 | (b) phone-number scarcity | `number-shop --pattern "555*MAIL"` — multi-area-code fan-out with dedupe | KILL — wrapper |
| C9 | (b) idle From-pool | `idle-numbers --since 30d` — IncomingPhoneNumbers LEFT JOIN Messages/Calls, flag idle | KEEP |
| C10 | (b) webhook orphans | `webhook-audit [--probe]` — group IncomingPhoneNumbers by VoiceUrl/SmsUrl, flag unique-use, optional HEAD probe | KEEP |
| C11 | (c) cross-entity | `recording-pair <Sid>` — Call + Recording + Transcription stitch | KILL — subset of C3 |
| C12 | (c) cross-entity | `conversation <number>` — Messages ∪ Calls timeline for a phone number | KEEP |
| C13 | (c) cross-entity | `error-code-explain --since 7d` — groupby ErrorCode + curated top-50 explanation table | KEEP |
| C14 | (f) DeepWiki — page-token cost | `sync-status` — last sync watermark per resource | KEEP (descoped: local-only, no API probe) |
| C15 | (f) DeepWiki — credential scoping | `which-account` — credential introspection + reachable subaccounts | KILL — covered by doctor |
| C16 | (a) Marc — operational | `tail-messages --status failed --follow` — poll loop streaming new failures, respects PRINTING_PRESS_VERIFY | KEEP |

## Survivors and kills

### Survivors

12 features survived all four force-answers (weekly use, wrapper-vs-leverage, transcendence proof, sibling kill).

| # | Feature | Command | Score | Persona | Buildability proof |
|---|---|---|---|---|---|
| 1 | Failed-message breakdown | `delivery-failures --since 7d` | 9/10 | Diego | Local SQLite groupby on synced Messages by ErrorCode + To country prefix, summing Price |
| 2 | Subaccount spend matrix | `subaccount-spend --period last-month --csv` | 10/10 | Priya | Walks synced Subaccounts, calls usage/records per subaccount via // pp:client-call, pivots categories to columns |
| 3 | Call timeline stitch | `call-trace <Sid>` | 9/10 | Marc | Reads Calls + Recordings + Transcriptions + Conferences/Participants from local store keyed by CallSid; live fallback when not synced |
| 4 | TCPA opt-out check | `opt-out-violations` | 9/10 | Lena | Local query: inbound Messages with Body ~ /^(STOP\|UNSUBSCRIBE\|END\|QUIT)/i joined to subsequent outbound Messages to same pair |
| 5 | Message status funnel | `message-status-funnel --since 24h` | 8/10 | Diego | Groupby on synced Messages over Status enum + median DateUpdated - DateCreated |
| 6 | Call disposition cross-tab | `call-disposition --since 24h` | 7/10 | Marc | Local cross-tab on synced Calls over Status × AnsweredBy, summing Price |
| 7 | Idle number reclamation | `idle-numbers --since 30d` | 9/10 | Priya | Three-way local LEFT JOIN flagging idle IncomingPhoneNumbers with $ wasted/month |
| 8 | Webhook orphan audit | `webhook-audit [--probe]` | 7/10 | Priya | Local groupby over IncomingPhoneNumbers VoiceUrl/SmsUrl + optional HEAD probe per unique URL |
| 9 | Number conversation feed | `conversation <number>` | 8/10 | Lena, Marc | UNION over synced Messages and Calls where From=<num> OR To=<num>, sorted by timestamp |
| 10 | Error code explainer | `error-code-explain --since 7d` | 7/10 | Diego, Marc | Local groupby on synced Messages+Calls by ErrorCode + curated // pp:novel-static-reference table of top-50 codes |
| 11 | Sync watermark inspector | `sync-status` | 6/10 | All | Local-only watermark/row-count read from synced store |
| 12 | Live failure tail | `tail-messages --status failed --follow` | 7/10 | Diego, Marc | Polling loop with DateUpdated>=last_seen; short-circuits when cliutil.IsVerifyEnv() |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---|---|---|
| C7 — cost-by-category | Same dataset and fan-out as C2; subaccount-spend already pivots categories to columns. | C2 subaccount-spend |
| C8 — number-shop | Thin wrapper over absorbed available-phone-numbers list; no leverage beyond a loop. | absorbed available-phone-numbers list |
| C11 — recording-pair | Strict subset of call-trace; bifurcates the entry point. | C3 call-trace |
| C15 — which-account | Brief commits doctor to credential introspection; duplicate. | absorbed doctor |
