# Plaud CLI Research Brief

## API Identity
- **Domain:** AI voice recorder + transcript/summary platform. Plaud sells a small dedicated recorder + an app (`app.plaud.ai` / `web.plaud.ai`) that uploads audio, runs ASR with speaker diarization, generates AI summaries, and stores everything as searchable recordings.
- **Two distinct API surfaces:**
  - **User-app API** (`api.plaud.ai`, regional: `api-euc1.plaud.ai`, `api-apse1.plaud.ai`) — what individual users hit through `app.plaud.ai`. JWT auth via `/auth/access-token`. **This is our target.**
  - **Embedded SDK API** (`platform-{us,jp}.plaud.ai/developer/api/open/partner/`) — B2B partner API for white-label apps. Partner-token + user-provisioning flow. **Not our target.**
- **Users:** professionals capturing meetings, sales calls, lectures, 1:1s. Power users sync to Obsidian/Notion/Logseq. Long tail: developers building automation around action items + decisions.
- **Data profile:** recordings have `id`, `filename`, `start_time`, `end_time`, `duration`, `serial_number`, `scene`, `filetag_id_list`, plus derived `trans_result` (speaker-diarized transcript segments) and `ai_content` (Markdown summary with decisions, action items, topics). Embeddings exist server-side. Average user has hundreds of recordings, totaling thousands of transcript segments.

## Reachability Risk
- **Low for documented surface, Medium for undocumented user-app surface.** The official CLI (`@plaud-ai/cli`) and official MCP (`@plaud-ai/mcp`) ship via npm against the user-app API and use browser OAuth — no reachability concerns there.
- The undocumented user-app endpoints (`/file/simple/web`, `/ai/transsumm/`, `/file/detail/{id}`, etc.) are battle-tested across 15+ community tools that have been live for 6-18 months. applaud (46⭐, last push 2026-04-30), leonardsellem (49⭐, 2026-05-15), and openplaud (179⭐, 2026-05-15) all hit them successfully.
- **Token rotation risk:** Reddit reports periodic JWT rotations that have broken reverse-engineered clients. Mitigated by supporting both auto re-login (email+password) and Chrome LevelDB JWT extraction (applaud's path). Plus a scheduled smoke test (one of the brief's risk-mitigations) ships in our doctor command.
- **Browser-fingerprint headers required.** The Plaud gateway 5xx's without `origin: https://app.plaud.ai`, `referer`, `app-platform: web`, `x-request-id`, `x-device-id`, `x-pld-tag`. All known and reproducible in stdlib HTTP — no clearance cookie needed.

## Top Workflows
1. **"What did I say I'd do?"** — pull every commitment ("I'll send", "I'll follow up", "let me get back to you") across all transcripts, grouped by person, with date + recording link.
2. **"What did <person> say about <X>?"** — FTS over speaker-diarized transcripts. Returns every utterance by that speaker mentioning that topic, with surrounding context and recording IDs.
3. **"Themes this month."** — clustering of topics across recent recordings: emerging vs decaying mentions. The signal nobody surfaces today.
4. **"Have I been consistent?"** — cross-meeting consistency check. Given a person + topic, return what was said in each meeting so the user can see if their position drifted.
5. **Bulk export + local sync + raw SQL escape hatch** — what every power-user-style integration needs (Obsidian, Notion, n8n, custom dashboards).

## Data Layer
- **Primary entities:**
  - `recordings` — list metadata (`id`, `filename`, `start_time`, `end_time`, `duration`, `scene`, `serial_number`, `filetag_id_list`, `is_trash`, `is_trans`, `is_summary`, `version`, `timezone`)
  - `transcripts` — per recording, segment rows (`recording_id`, `idx`, `start_time`, `end_time`, `content`, `speaker`, `original_speaker`)
  - `summaries` — per recording (`recording_id`, `markdown`, `decisions`, `action_items`, `topics`, `header`)
  - `filetags` — folder/tag structure (`id`, `name`, `parent_id`)
  - `speakers` — derived dimension table (`name`, `original_speaker`, count, first_seen, last_seen)
- **Sync cursor:** No server-side delta. Strategy = walk full list per sync, dedupe by `id`, upsert. Track `last_synced_at` per recording (because `ai_content` can update after the initial capture). Use `edit_time` field on the list response to detect updates.
- **FTS:** SQLite FTS5 over `transcripts.content` joined to `speakers.name` + `recordings.start_time`. This is the foundation for the compound queries that justify the whole CLI.

## Codebase Intelligence
- **Source:** Direct read of `sergivalverde/plaud-toolkit` source via subagent — `packages/core/src/{auth,client,config,types}.ts`. Confirmed: `BASE_URLS = { us: 'https://api.plaud.ai', eu: 'https://api-euc1.plaud.ai' }`, login is `POST /auth/access-token` (form-urlencoded `username`/`password`), response is `{ status, msg?, access_token, token_type }`. JWT lifetime confirmed ~300 days via decoding `exp`. Refresh = full re-login when `Date.now() + 30d > expiresAt`. Region auto-switch on `status: -302` with `data.domains.api`.
- **Data model (from applaud + Plaud_API):** Recording row carries `id`, `filename`, `fullname`, `filesize`, `file_md5`, `start_time` (epoch seconds), `end_time`, `duration`, `version`, `version_ms`, `edit_time`, `is_trash`, `is_trans`, `is_summary`, `serial_number`, `filetype`, `timezone`, `scene`, `filetag_id_list`, `is_markmemo`, `wait_pull`.
- **Transcript shape (from `/ai/transsumm/{id}`):** `data_result: TranscriptSegment[]` where `TranscriptSegment = { start_time, end_time, content, speaker, original_speaker }`. `speaker` is the renamed display name; `original_speaker` is the raw "Speaker 1/2/3" label from ASR.
- **Summary shape:** `data_result_summ` is **inconsistent** — JSON string for normal recordings, structured object `{markdown}|{ai_content,header}|{content:str}|{content:{markdown}}` for short ones. Must normalize defensively.
- **S3 fallback:** Recordings created before March 2026 return `status:-12` empty from `/ai/transsumm/{id}`. Fallback: GET `/file/detail/{id}`, walk `content_list[]`, GET unauthenticated `data_link` S3 URLs (may be gzipped — try gunzip first).
- **Rate limiting:** No 429 observed in any community tool. applaud uses 3 attempts with 1s/2s/3s linear backoff on 5xx. 401 = re-auth required. We'll match that.
- **Architecture insight:** Plaud's user-app API is **stateless + region-routed**. No webhooks for the user-app side (webhooks exist only for the Embedded SDK partner API). Sync = poll. This is why we *need* local FTS — there's nothing on Plaud's side to query intelligently.

## User Vision
- **NOI thesis (verbatim from user brief):** Plaud isn't a transcription service — it's a passive long-term memory of every conversation. Every recording is a signal about recurring topics, unfulfilled commitments, and the gap between what you said and what anyone remembers.
- **Compound commands the user prioritized:** `commitments`, `recurring-topic <X>`, `about <person>`, `forgotten`, `themes --last 30d`, `cross-meeting <person> <topic>`, `sync + sql`.
- **Auth strategy approved by user:** "documented-endpoints path first (sergivalverde reference). Then sniff web.plaud.ai for undocumented routes — I'll provide login creds when prompted."
- **Risks user named:** TOS (official CLI exists — ship unofficial disclaimer), JWT fragility (mitigation: long-lived 300-day tokens + auto re-login), dedup vs 15 community tools (mitigation: agent-native + SQL + FTS over speaker diarization), Plaud server changes (mitigation: scheduled smoke test).

## Product Thesis
- **Name:** `plaud-pp-cli` (binary `plaud-pp-cli`, prose name "Plaud Press CLI").
- **Why it should exist:** No existing tool combines (a) local SQLite mirror of all recordings + transcripts + summaries, (b) FTS5 search across speaker-diarized transcript content, (c) compound queries over commitments / themes / consistency across meetings, (d) agent-native output (`--json`, `--select`, `--csv`, typed exit codes), and (e) a raw `sql` escape hatch. The official CLI is 11 thin commands with client-side keyword search. Every community tool either targets one consumer (Obsidian, Notion) or stops at "list + download." Our wedge: **transcripts as a queryable database, not a feed.**
- **Tagline:** "Every conversation, queryable. Plaud isn't transcription — it's the searchable history of what you said to whom, what you promised, and what nobody else remembers."

## Build Priorities
1. **Foundation (Priority 0)** — Auth (email+password → `/auth/access-token`, persist to `~/.plaud/config.json` mode 0600, auto re-login at 30d-to-expiry), region routing with `-302` retry, browser-fingerprint headers, SQLite store with full schema (recordings, transcripts, summaries, filetags, speakers), FTS5 over transcripts joined to speakers, sync (full + incremental via `edit_time`), `sql` raw query, `search` typed FTS.
2. **Parity (Priority 1 — Absorb)** — Every command in the official CLI (`files`, `recent`, `today`, `search`, `file`, `audio`, `transcript`, `summary`, `me`, `login`, `logout`) plus everything community tools added: trash/untrash/permadelete (Plaud_API), file tags listing (Plaud_API), shareable links (Plaud_API), document export (DOCX/PDF/TXT/MD via `/file/document/export`), bulk download (Chrome ext + Plaud_BulkDownloader), Chrome JWT extraction (applaud), Obsidian-shape sync (leonardsellem), MCP server mirroring CLI surface (official MCP + sergivalverde + charathram). Every absorbed feature must work offline against the local store after one sync.
3. **Transcendence (Priority 2 — the wedge)** — `commitments` (regex+keyword extraction across all transcripts), `recurring-topic <X>` (FTS + temporal clustering), `about <person>` (FTS scoped to speaker), `forgotten` (commitments with no follow-up signal), `themes --last 30d` (topic clustering with emerging/decaying buckets), `cross-meeting <person> <topic>` (consistency check), `with <person>` (every recording where speaker appears, with role/topic mix), `silence <person> --days 30` (people you haven't talked to recently), `mentioned-me` (third-party mentions of the authenticated user by name).

## Auth Strategy (concrete)
- **Primary:** `plaud-pp-cli auth login` → interactive email+password prompt → POST form-urlencoded to `/auth/access-token` → persist JWT + plaintext credentials (sergivalverde's approach, ugly but pragmatic) at `~/.plaud-pp-cli/config.json` (mode 0600).
- **Secondary:** `plaud-pp-cli auth login --chrome` → applaud's LevelDB scrape path → extract JWT from `PLADU_bearer` key in Chrome/Brave/Arc/Vivaldi LevelDB. No password stored. Re-auth requires re-login in browser.
- **Region:** `--region us|eu|ap` flag on `login`; auto-detect on first call via `-302` retry; persist in config.
- **Re-auth trigger:** 30 days before JWT `exp`, or on any 401, or when credentials change.
- **Doctor:** verifies token, hits `/user/me`, checks region, surfaces JWT expiry remaining days.

## Source Priority
This is a single-source CLI (the Plaud user-app API). The `web.plaud.ai` URL the user provided is the same surface as `api.plaud.ai` — `web.plaud.ai` is the React frontend that calls `api.plaud.ai`. Phase 1.7 browser-sniff will validate the endpoint inventory + capture any newer endpoints not in community tools (especially around the "thought partner" feature seen in `/file/detail`).

## Phase 1.7 Plan (Browser-Sniff)
The user will provide login creds when prompted. The sniff has three concrete goals beyond what's already known from community-tool source:
1. **Confirm endpoint inventory is current** — community tools' last-push dates range March-May 2026; verify no newer routes (e.g., live agent / thought partner).
2. **Discover the "thought partner" surface.** Visible in `/file/detail` as `has_thought_partner: bool`. Likely an interactive chat-over-transcript endpoint. Worth absorbing if it exists.
3. **Confirm the browser-fingerprint headers** are still the gateway requirement (not a CF/WAF JS challenge).

The sniff is **enrichment**, not the primary spec source. Even if Phase 1.7 fails for some reason (Cloudflare challenge, auth fragility), we have enough from the source-code-derived inventory to generate.
