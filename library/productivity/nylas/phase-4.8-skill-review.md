# Phase 4.8 — Agentic SKILL review

## Verdict
NEEDS-FIXES

## Trigger phrases

- finding: SKILL.md L3 description lists 6 trigger phrases, but only 2 of them clearly route to a novel feature. "search my Nylas inbox across all grants" -> search. "what changed in Nylas in the last 2 hours" -> since. "first-response time on my Nylas threads" -> response-time. The other three ("replay a Nylas webhook locally", "preview a Nylas send before it goes out", "use nylas", "run nylas-pp-cli") are weaker — "use nylas" and "run nylas-pp-cli" are escape hatches, not capability triggers, so they pad the list without aiming routing.

- finding: No trigger phrase for `gravity` in the description (L3). The novel-features doc explicitly calls out queries like "who actually matters to this tenant" and "who do I email most" / "rank my key contacts" / "CRM hygiene". A user asking "who do I email most across my mailboxes" will not match the current description and will route to a generic search or contacts endpoint instead of `gravity`.

- finding: No trigger phrase for `sql` (L3). A user asking "run an ad-hoc query over my Nylas data" or "count messages per provider" should land here. Absent.

- finding: No trigger phrase for `grants doctor` / `doctor` (L3). The CLI explicitly recommends `doctor` as the first command (README L118, SKILL L492), but neither "is my Nylas setup working" nor "check my grant health" are listed as triggers. An agent on first install will skip the doctor step.

- finding: No trigger phrase for `sync` (L3). Without "mirror my Nylas data locally" or "pull my Nylas inbox into a local store", `since` and `search` triggers route to commands that will silently return zero rows on a fresh install (acknowledged at SKILL L1008 as a known foot-gun for the README; not echoed in SKILL).

- finding: No trigger phrase for `export` (L3). Phrases like "dump my Nylas messages to NDJSON" or "pipe Nylas data into duckdb" are explicit value props in the novel-features doc (#17) and recipe-worthy, but absent from description.

- finding: No trigger phrase for `idempotent-send` (L3). The "preview a Nylas send before it goes out" phrase covers send-preview but not retry-safe idempotent-send. A user asking "send this email safely from a retry loop" will not match.

- finding: Over-broad trigger "use nylas" (L3) will fire on requests this CLI cannot serve directly without setup (e.g. "use nylas to send a calendar invite from my unauthenticated CLI"). Combined with the absent doctor trigger, this guarantees premature execution.

## Novel-feature descriptions

- finding: `since` vs `search` vs `sql` disambiguation is implicit, not stated (L50-63, L120-126). An agent reading the SKILL has to infer that `since` = time-window + structured field filter, `search` = FTS5 keyword, `sql` = arbitrary join. Add one explicit "When to choose" sentence per command, e.g. "Use `since` for time-bounded changes, `search` for keyword/FTS5 matches, `sql` for anything else."

- finding: The hard prerequisite that `sync` must run before `since`/`search`/`sql`/`gravity`/`response-time`/`export` produce any rows is stated only in the README troubleshooting (README L1008) and not in SKILL.md at all. The recipes (L450, L458, L466) implicitly chain sync first, but a one-line "All local-store commands require a prior `sync`" near L42 ("Local state that compounds") is the bare minimum, and it's missing. Agents that read only Unique Capabilities will skip sync.

- finding: `messages send` appears twice as a bullet (L96 and L104) for two different concerns (confirm-by-hash and idempotent-send). This reads as a documentation bug — both are the same command differentiated by flags. Combine into one bullet listing both `--confirm` and `--idempotency` flags, or split them under distinct headings ("send-preview" and "idempotent-send") so an agent can find each by name.

- finding: `webhook-replay` description (L89) says "from the local store" but does not state the prerequisite — that webhook deliveries must have been persisted by an earlier `sync` (or webhook receiver) before replay works. An agent will run this against an empty store and get zero hits.

- finding: `gravity` description (L73) does not state it requires messages AND contacts AND events to be synced. The novel-features doc (#5) says "sent + received + meeting-attended" — that is three resources. The example at L78 omits `--resource messages,contacts,events` from the prior sync step, so a sync of just messages will produce wrong gravity rankings silently.

- finding: `response-time` description (L80) does not state it requires threads to be synced AND that "first response" is computed from local message timestamps — so a partial sync window biases the metric. Worth a half-sentence caveat.

- finding: `grants doctor` (L111) is buried under "Reliability & safety" as the 4th item. Given it's the documented first command (README L118 Quick Start, SKILL L492 Auth Setup), it should also be cross-referenced from the Auth Setup section as "run this before anything else."

- finding: `--agent` global flag (L127) is documented twice — once as a "Unique Capability" bullet and again in the dedicated "Agent Mode" section (L494). The bullet description ("forces --json --select defaults") and the Agent Mode section ("--json --compact --no-input --no-color --yes") disagree on the exact expansion. The Agent Mode version is correct; reconcile or one of them is misinformation.

- finding: `mcp serve` is listed as a survivor in the novel-features doc (#10, #12) but does NOT appear anywhere in SKILL.md as a Unique Capability bullet. Only the MCP installation snippet at L599 mentions it indirectly. The skill claims 12 novel features in scope but actually documents 11 (sync, since, search, export, gravity, response-time, webhook-replay, send-preview as confirm-by-hash, idempotent-send, grants doctor, sql, --agent = 12 if you count both send variants, 11 if you don't). `mcp serve` is missing.

## Auth narrative

- finding: Auth Setup section (L488-492) leads with API key concept but does not show a worked example — no `export NYLAS_API_KEY=nyk_...` line. An agent setting up from cold needs to see the exact shell line.

- finding: Auth Setup conflates `NYLAS_API_KEY` and `NYLAS_ACCESS_TOKEN` (L490) without explaining when each is required. README L994 only lists `NYLAS_ACCESS_TOKEN` in the env-var table as required — that contradicts SKILL L490 which says `NYLAS_API_KEY` is the primary. An agent reading both will be confused about which to set. The truth (per nylas-api SKILL L19) is `NYLAS_API_KEY` for application-level, `NYLAS_ACCESS_TOKEN` for per-user OAuth.

- finding: The "doctor first" prescription is hidden — single line at L492 ("Run `nylas-pp-cli doctor` to verify setup.") with no prominence. Should be elevated to a numbered step: "1. Set NYLAS_API_KEY. 2. Run `nylas-pp-cli doctor --agent`. 3. Run `nylas-pp-cli grants get-all` to confirm at least one grant exists."

- finding: The bearer-token Authorization header pattern is mentioned (L490) but the SKILL does not state that the CLI handles header construction automatically — an agent might think it needs to pass `--header "Authorization: Bearer ..."` for each call. One sentence "The CLI attaches the bearer header automatically when NYLAS_API_KEY is set" would close this.

- finding: `grants doctor` (L111) and the global `doctor` (L492) are two distinct commands but the SKILL never disambiguates. `doctor` checks the CLI's own auth+connectivity; `grants doctor` checks every grant's health. An agent will pick one of them at random.

- finding: No mention of US vs EU base URLs. The nylas-api SKILL (L18) explicitly calls out `api.us.nylas.com` vs `api.eu.nylas.com`. The CLI presumably has a `--region` flag or `NYLAS_API_URI` env var, but neither is documented in SKILL.md. An EU-region tenant will get 401 with no obvious fix.

## Recommended edits (concrete)

- SKILL.md L3 — Add trigger phrases: `who do I email most across my Nylas grants` (gravity), `dump my Nylas messages to NDJSON` (export), `check my Nylas grant health` (grants doctor), `mirror my Nylas inbox locally` (sync), `run a SQL query over my Nylas data` (sql), `safely send this email from a retry loop` (idempotent-send). Drop `use nylas` and `run nylas-pp-cli` as low-signal.

- SKILL.md L42 (start of "Local state that compounds") — Insert one line: "**Prerequisite:** all commands in this section read from the local mirror; run `sync` once before `since`, `search`, `sql`, `gravity`, `response-time`, or `export` will return data."

- SKILL.md L96–L110 — Collapse the two duplicate `messages send` bullets into one or rename them: bullet 1 "**`messages send` (confirm-by-hash)**" with `--confirm` flag, bullet 2 "**`messages send` (idempotent)**" with `--idempotency` flag. Currently both say "messages send" with no distinguishing name.

- SKILL.md L73 (gravity example, L78) — Add note: "_Requires prior sync of messages, contacts, and events for accurate ranking._"

- SKILL.md L89 (webhook-replay) — Add: "_Requires webhook deliveries to be present in the local store (via `sync --resource webhooks` or a running webhook receiver)._"

- SKILL.md L111 (grants doctor) — Add cross-reference: "Run before anything else on a fresh install; see Auth Setup."

- SKILL.md L127 ("--agent" bullet) — Replace "forces --json --select defaults" with "expands to `--json --compact --no-input --no-color --yes`" to match the authoritative description at L496.

- SKILL.md L488 (Auth Setup heading) — Replace the current paragraph with a numbered checklist:
  ```
  1. export NYLAS_API_KEY=nyk_v0_...   (application-level API key from dashboard-v3.nylas.com)
  2. Optional: export NYLAS_API_URI=https://api.eu.nylas.com   (EU region tenants)
  3. nylas-pp-cli doctor --agent      (verifies key + reachability)
  4. nylas-pp-cli grants get-all --agent   (confirm at least one grant)
  5. nylas-pp-cli grants doctor --agent    (per-grant health, scopes, sync lag)
  ```
  Because the current single-paragraph form buries the doctor step and never shows the bearer-header is handled automatically.

- SKILL.md L490 — Resolve env-var conflict with README L994. State: "Set `NYLAS_API_KEY` for application-level access (recommended for this CLI). `NYLAS_ACCESS_TOKEN` is supported only for per-user OAuth flows on grant-scoped endpoints." Then fix README env-var table to match.

- SKILL.md (new bullet under "Agent-native plumbing", after L126) — Add `mcp serve` as the 12th novel feature: "**`mcp serve`** — Expose the local-store and cross-grant tools (since, search, sql, gravity, response-time, send-preview) as MCP tools with proper read-only/destructive annotations. _Use to give Claude Code or another agent the questions the hosted Nylas MCP cannot answer._" Otherwise the SKILL is missing a documented survivor from the novel-features brief.

- SKILL.md L50, L57, L120 — Add a single "When to choose" line under `since`, `search`, and `sql` so an agent can disambiguate: e.g. under since "_Use when you know the time window and want structured field filters._"; under search "_Use for keyword / phrase / FTS5 matches across resources._"; under sql "_Use when neither since nor search fits — arbitrary joins, aggregates, custom predicates._"
