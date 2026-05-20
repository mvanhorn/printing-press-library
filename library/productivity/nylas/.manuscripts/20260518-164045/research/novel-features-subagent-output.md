# Nylas CLI — Novel Features Proposal

## Customer model

The primary user is a **multi-grant operator**: someone holding the application API key for a Nylas tenant that has many connected mailboxes — typically an AI-agent builder, a B2B SaaS engineer running an inbox-product on top of Nylas, or a solo founder/ops person who has connected their personal Gmail, a shared support@ Microsoft inbox, and a couple of customer-side grants under one app. A secondary user is the **on-call/integration engineer** in healthcare, finance, or legal who needs deterministic, audit-logged, locally-replayable Nylas access without N round-trips. Their daily rituals look like: "What new threads landed across all my grants in the last 2 hours?", "Find every message from this domain across every connected mailbox", "Has the webhook fired for this event yet — and if not, replay it locally", "Did smart-compose generate the same body for two different grants this morning?", and "Show me the response-time SLA on threads where I am the assigned owner." None of these questions can be answered with `nylas-cli` today because it is grant-at-a-time and stateless — every query is a fresh API call, scoped to one grant, with no history, no cross-grant join, and no SQL. They are the gap.

## Candidates (full list)

1. **sync** | Pull and persist messages/threads/events/contacts/webhooks for one or all grants into local SQLite with cursor checkpoints | Only we have a local store; incumbent has no persistence layer at all. Cursors mean re-runs are O(delta) not O(total). | `nylas-pp sync --grant all --resource messages,events --since 24h` | The atomic primitive every other transcendence feature depends on. | Local state that compounds | hand-code
2. **since** | Time-windowed query across the local mirror — "what changed in the last N" without hitting the API | Incumbent forces a full re-list per call; we answer from SQLite in milliseconds and across all grants. | `nylas-pp since 2h --resource messages --json --select id,subject,from,grant_id` | Lets an agent poll cheaply without burning rate limit. | Local state that compounds | hand-code
3. **search** | FTS5 search across messages/threads/events/contacts spanning all grants in one query | Incumbent's `email search` is single-grant and API-bound; ours joins FTS to contacts and runs offline. | `nylas-pp search "invoice overdue" --resource messages --grant all --since 30d` | One command answers "where did anyone mention X across every mailbox we hold." | Local state that compounds | hand-code
4. **sql** | Raw read-only SQL escape hatch against the local mirror, with `--json` output | No competing CLI exposes its data this way. Agents can compose arbitrary joins; humans get a power-tool. | `nylas-pp sql "SELECT from_addr, COUNT(*) FROM messages WHERE grant_id IN (SELECT id FROM grants WHERE provider='google') GROUP BY 1 ORDER BY 2 DESC LIMIT 20"` | The ultimate agent-native escape hatch — anything we forgot to expose as a verb, an LLM can still answer. | Agent-native plumbing | hand-code
5. **gravity** | Rank contacts by cross-grant interaction weight (sent + received + meeting-attended), unified by email | Incumbent can list contacts per grant; only a local cross-grant join can compute a unified gravity score. | `nylas-pp gravity --top 25 --since 90d --json` | One call surfaces "who actually matters to this tenant" — invaluable for CRM hygiene and agent prioritisation. | Cross-grant analytics | hand-code
6. **response-time** | Compute median/p90 first-response latency on threads where the grant-holder replied, sliced by grant or by counterparty domain | Requires the thread+message timeline reconstructed locally; no single API call returns this. | `nylas-pp response-time --grant all --group-by domain --since 30d` | Drops an SLA dashboard into one terminal command — impossible against the live API in <5min. | Cross-grant analytics | hand-code
7. **inbox-diff** | Snapshot a grant's inbox state, name it, then later diff against current to show added/removed/changed threads | Diff requires two persisted snapshots — incumbent has neither. | `nylas-pp inbox-diff snapshot pre-migration --grant g_abc && nylas-pp inbox-diff against pre-migration` | Lets you safely verify migrations, mailbox restores, or rule changes by exact before/after. | Reliability & safety | hand-code
8. **webhook-replay** | Persist webhook deliveries locally and replay any past event into a local handler URL, with optional payload edit | Incumbent has a local server but no persistence + replay; once an event has fired it's gone. | `nylas-pp webhook-replay --since 24h --trigger message.created --to http://localhost:3000/hook` | Lets a developer reproduce a production webhook bug in <30s without waiting for the next event. | Reliability & safety | hand-code
9. **send-preview** | Render the exact wire-payload of a send/draft/RSVP/notetaker-invite as JSON + a human diff, then require `--confirm` to actually call the API | Mirrors the Nylas MCP confirm-before-send pattern but at the CLI layer, with payload-level visibility the hosted MCP doesn't expose. | `nylas-pp email send --to alice@x.com --body-file note.md` (prints payload, exits 7) → `nylas-pp email send --confirm <hash>` | Makes every destructive action reviewable; an agent cannot silently send. | Reliability & safety | hand-code
10. **rule-audit** | Fetch all server-side rules/filters/policies/webhooks per grant, hash them, and diff against a checked-in JSON baseline | Configuration drift across many grants is invisible today; only a cross-grant local snapshot makes it tractable. | `nylas-pp rule-audit --baseline ./baselines/prod.json --grant all` | One command answers "did anything change that we didn't author?" — security & compliance gold. | Reliability & safety | hand-code
11. **agent-mode** | A single `--agent` flag that forces `--json --select` defaults, suppresses prompts, sets typed exit codes, and binds to a `NYLAS_PP_AGENT=1` env discipline | Incumbent has flags but no unified "I am an LLM, behave deterministically" mode. | `nylas-pp email list --agent --since 1h` | One flag flips the entire CLI into LLM-safe mode; eliminates a class of prompt-eats-stdin bugs. | Agent-native plumbing | hand-code
12. **mcp serve** | Expose the local-store and cross-grant tools (since, search, sql, gravity, response-time, send-preview) as MCP tools with proper read-only/destructive annotations | Nylas hosts an MCP at mcp.us.nylas.com with ~17 tools, none of which touch a local mirror or do cross-grant work. We have a clear non-overlap. | `nylas-pp mcp serve --stdio` then point Claude Code at it | Gives agents the questions the hosted MCP can't answer. | Agent-native plumbing | hand-code
13. **batch** | Execute a newline-delimited file of CLI invocations with concurrency, per-line `--dry-run` capture, and a JSON ledger of results | Incumbent has no batch primitive; agents currently shell-loop. | `nylas-pp batch --file ops.txt --concurrency 4 --dry-run` | Lets an agent stage 200 ops, review the ledger, then re-run with `--confirm`. | Agent-native plumbing | hand-code
14. **grants doctor** | Health-check every grant: token expiry, missing scopes for advertised features, recent webhook failures, sync lag | Requires joining grant state + local sync metadata + webhook history — only possible with a store. | `nylas-pp grants doctor --json` | One command tells you which mailboxes are quietly broken before users notice. | Reliability & safety | hand-code
15. **meeting-load** | Compute meeting-hours per day per grant from the events table, with timezone normalisation and conflict counts | Incumbent has `calendar analyze` but single-grant; cross-grant load (e.g. an exec assistant managing 5 calendars) is uniquely ours. | `nylas-pp meeting-load --grant all --since 7d --group-by day` | Surfaces "who on this team is meeting-overloaded right now" in one call. | Cross-grant analytics | hand-code
16. **thread-thread** | Given a message id or subject, walk the local thread+contacts graph and emit a participant timeline across every grant that touched it | Requires joined message/contact data per-grant unified — no API call returns this. | `nylas-pp thread-thread "Q3 renewal" --grant all` | Reconstructs the full conversation when half is in support@, half in the founder's personal inbox. | Cross-grant analytics | hand-code
17. **export** | Stream the full local mirror (or a filtered slice) to NDJSON / Parquet for downstream analytics | The store is the asset; export turns it into a data product. | `nylas-pp export --resource messages --since 90d --format ndjson > msgs.ndjson` | Bridges Nylas data into duckdb / a notebook in one pipe. | Local state that compounds | hand-code
18. **idempotent-send** | Compute a content-hash of any send/RSVP/draft-send payload; refuse to re-execute the same hash within a TTL window | Requires persisted send-ledger; incumbent and hosted MCP have no such guarantee. | `nylas-pp email send --to a@x --body-file b.md --idempotency 24h` | Eliminates duplicate-send bugs in agent retry loops — a real production failure mode. | Reliability & safety | hand-code

## Kill list

1. **(kept #1 sync)** — atomic primitive.
2. **(kept #2 since)** — top-of-brief workflow.
3. **(kept #3 search)** — top-of-brief workflow.
4. **(kept #4 sql)** — strongest agent escape hatch.
5. **(kept #5 gravity)** — visible in `--help`, uniquely cross-grant.
6. **(kept #6 response-time)** — strong B2B SaaS hook; impossible against live API.
7. **(killed #7 inbox-diff)** — cool but narrow; reproducible via `sql` against two snapshots once `export` exists. Doesn't earn its own command.
8. **(kept #8 webhook-replay)** — incumbent has a local server but explicitly no replay; clear gap, ~120 LoC.
9. **(kept #9 send-preview)** — mirrors Nylas's own MCP safety pattern but at the wire-payload layer; differentiating.
10. **(killed #10 rule-audit)** — directionally great but requires shipping baseline schemas + diff logic that risks >150 LoC. Punt to a future release once `export` lands; `sql` covers the ad-hoc case.
11. **(kept #11 agent-mode)** — single visible flag, big trust win for LLM users.
12. **(kept #12 mcp serve)** — required to compete with the hosted Nylas MCP on local/cross-grant tools.
13. **(killed #13 batch)** — generic "run a file of commands" is shell-script territory; doesn't read as Nylas-differentiating in `--help`. An agent can just loop.
14. **(kept #14 grants doctor)** — single-screen "is my tenant healthy?" answer no one else gives.
15. **(killed #15 meeting-load)** — solid feature but partially covered by `sql` over the events table; doesn't justify a dedicated verb when scope is tight.
16. **(killed #16 thread-thread)** — narrow use-case; `search` + `sql` reach it.
17. **(kept #17 export)** — turns the store into a data product; one command, ~60 LoC, huge leverage.
18. **(kept #18 idempotent-send)** — real production failure mode; ~80 LoC; pairs naturally with send-preview.

Kill rate: 5 of 18 (28%) — at the lower end of the target band, but each kept feature passes all four adversarial tests and the killed ones genuinely fold into survivors (`sql` + `export` cover most of them).

## Survivors

| # | Feature | Command | Buildability | Why Only We Can Do This |
|---|---------|---------|--------------|------------------------|
| 1 | sync | `nylas-pp sync --grant all --resource messages,events --since 24h` | hand-code | Local SQLite + per-(grant,resource) cursors; incumbent is stateless |
| 2 | since | `nylas-pp since 2h --resource messages --json` | hand-code | Answers from the mirror across all grants, no API hit |
| 3 | search | `nylas-pp search "invoice overdue" --grant all --since 30d` | hand-code | FTS5 over messages/threads/events/contacts spanning grants |
| 4 | sql | `nylas-pp sql "SELECT from_addr, COUNT(*) FROM messages GROUP BY 1"` | hand-code | Read-only escape hatch on the local store |
| 5 | gravity | `nylas-pp gravity --top 25 --since 90d` | hand-code | Cross-grant contact-weight scoring requires a unified store |
| 6 | response-time | `nylas-pp response-time --group-by domain --since 30d` | hand-code | Thread timeline reconstruction is local-only |
| 7 | webhook-replay | `nylas-pp webhook-replay --since 24h --trigger message.created --to http://localhost:3000/hook` | hand-code | Requires persisted webhook deliveries; incumbent's local server doesn't store |
| 8 | send-preview | `nylas-pp email send --to a@x --body-file n.md` → exit 7 with payload hash, then `--confirm <hash>` | hand-code | Wire-payload-level confirm pattern; hosted MCP confirms intent, we confirm bytes |
| 9 | agent-mode | `nylas-pp <any> --agent` | hand-code | One flag flips defaults to `--json --select`, suppresses prompts, sets typed exit codes |
| 10 | mcp serve | `nylas-pp mcp serve --stdio` | hand-code | Exposes since/search/sql/gravity/response-time/send-preview — the tools the hosted Nylas MCP doesn't have |
| 11 | grants doctor | `nylas-pp grants doctor --json` | hand-code | Joins grant state + sync metadata + webhook history; local-store only |
| 12 | export | `nylas-pp export --resource messages --since 90d --format ndjson` | hand-code | Streams the store to ndjson/parquet for downstream analytics |
| 13 | idempotent-send | `nylas-pp email send --idempotency 24h ...` | hand-code | Persisted send-ledger refuses replayed payload hash |

## Recommended scope

- **spec-emits count: 0** (zero hand-code work) — every transcendence feature is hand-code by definition; spec-emitted commands cover the absorb layer separately.
- **hand-code count: 13** (each ~50–150 LoC + root.go wiring; `sync` and `mcp serve` are the largest at ~150 LoC each, `export` and `since` the smallest at ~50 LoC).
- **Total: 13 transcendence features for the manifest.**

Suggested grouping for the manifest (themes the `--help` tree should reflect):
- **Local state that compounds:** sync, since, search, export
- **Cross-grant analytics:** gravity, response-time
- **Reliability & safety:** webhook-replay, send-preview, idempotent-send, grants doctor
- **Agent-native plumbing:** sql, agent-mode, mcp serve
