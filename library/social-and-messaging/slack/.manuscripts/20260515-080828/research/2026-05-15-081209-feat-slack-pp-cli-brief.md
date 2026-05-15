# slack-pp-cli Research Brief

> Synthesis of the 610-line scoping doc at `g:/My Drive/Cursor/workspace/evals/pp-slack-scoping-2026-05-11.md` plus Phase 1.5a/a.5 addendum at `2026-05-13-191044-feat-slack-pp-cli-addendum.md`. The 5th CLI in Erick's EOS-leadership-data suite (Attio + Notion + Asana + Fathom already shipped).

## API Identity
- **Domain**: Slack Web API — workspace messaging, channels, DMs, threads, search, files, reactions, usergroups, chat posting/scheduling. 174 endpoints across 25 resource groups in OpenAPI 3.0.0 spec (apis.guru mirror, version 1.7.0, last modified 2025-08-18).
- **Users**: CEO running EOS (Erick) + 8 internal AtomChat skills + 4 production cron jobs already consume Slack. Single workspace `T01GKHKBY7R` (Atom). No multi-workspace requirement.
- **Data profile**: ~100+ active channels Erick is in, ~50 msgs/day per active channel, ~1.8M messages/year accumulating, threaded replies + reactions + file attachments. Heavy DM use with 15 direct reports.

## Reachability Risk
- **Low** — `auth.test` passes with current xoxp token (validated 2026-05-13 19:10 UTC: `ok:true, team=Atom, user=eholmann`). API stable; no community-reported 403/block patterns affecting xoxp tokens.
- **Latent rate-limit bug surfaced by addendum**: May-2025 changelog caps `conversations.history` + `conversations.replies` at **1 req/min × 15 msgs/page** for non-Marketplace apps. Empirically validated 2026-05-11 that the cap does NOT yet apply to AtomChat's xoxp user token (`limit=999` returned 642 messages, ok=true). May tighten in the future. Build cursor-aware pagination + `Retry-After` respect as hedge.
- **Silent-truncation gotcha (community/discussions #162325)**: when the cap IS active, requests for `limit=100` silently return 15 with no error. pp-slack MUST log a warning when the returned page is smaller than requested. Class-of-bug fix, not one-off.

## Top Workflows
1. **Weekly L10 prep "what happened with customer X"** — Erick runs weekly across #churnsales + #csm + #ob-csm-cux-soporte + #sales-support + #the-wolf-of-atom + 4-5 CSM DMs (Marjorie, Adrian, Gustavo, Jorge, Camilo). Today: opens each channel in browser sequentially, ctrl-F. **The single highest-value verb** → `customer-intel`.
2. **Daily morning "did I miss anything important"** — drift on threads where @Erick was @-mentioned but didn't reply, unanswered DMs, attention-needed reactions. Today: scrolls Slack. → `attention` (composed) + `drift`.
3. **Weekly digest compilation** — 6-source weekly digest into 2 Notion pages. Slack is source #3, reads 12 channels + DMs for every direct report. Already wired into pp-notion for the write side; needs pp-slack for the read side. → `digest`.
4. **Pre-1:1 / scorecard DM scan** — for each of 15 direct reports, pull last 7-30 days of DM traffic. Vault sync is 5am-stale. → `dms-summary`.
5. **Cron-posted narrative alerts** — csm-platform-v2 posts 30 alerts/day to #csm-signals. one-on-one-prep posts 1:1 prep links to direct-report DMs 1hr before each meeting. Both hardcode the xoxp token today (`credential-safety.md` violation). → `post` + `schedule` with `--dry-run` for cron safety.

## Codebase Intelligence
- **Source: addendum read of `modelcontextprotocol/servers-archived/src/slack/index.ts` + `korotovsky/slack-mcp-server` source**
- **Auth header**: `Authorization: Bearer ${SLACK_USER_TOKEN}`. Env var should be `SLACK_USER_TOKEN` (the existing skill misnames it `SLACK_BOT_TOKEN` even though it's a user token — pp-slack will fix the canonical name).
- **Base URL**: `https://slack.com/api/<method>`. GovSlack variant `https://slack-gov.com/api/<method>` — not needed for AtomChat but cheap to support via env var.
- **Pagination**: `cursor` parameter via `URLSearchParams.append`. Slack returns `response_metadata.next_cursor`.
- **Three viable token types**: `xoxp` (user, current AtomChat token, sees everything Erick sees including DMs), `xoxb` (bot, narrow scope), `xoxc+xoxd` (browser session, "stealth" mode from korotovsky — zero admin friction, reads DMs the user reads, rotates on browser logout). pp-slack v1: support `xoxp` + `xoxb`; defer `xoxc+xoxd` to v1.1.
- **search.messages is user-token-only** — pin in contract; defaults pp-slack to xoxp mode. The single most powerful read endpoint (cross-channel FTS via Slack's own index) and unaffected by May-2025 rate-limit cap.
- **Architecture insight from rusq/slackdump v3**: SQLite backend is 5× faster than the old chunk-file (JSON+GZ) format. FTS5 over message body + thread bodies. Schema worth borrowing.

## Data Layer
- **Primary entities** (mirror tables, daily incremental sync):
  - `channels` — id, name, is_archived, is_member, is_im, is_mpim, is_private, num_members, topic, purpose
  - `users` — id, name, real_name, display_name, email, is_bot, deleted, tz
  - `messages` — channel_id, ts, thread_ts, user_id, text (body), subtype, reply_count, reply_users, reactions (JSON), files (JSON), permalink
  - `threads` — parent_ts, channel_id, last_reply_ts (derived index for `drift`)
  - `usergroups` — id (S…), handle (`@csm-emea`), name, user_ids (JSON) — currently misrendered in digest skill, fix in v1
  - `reactions` — message_ts, channel_id, emoji_name, user_ids
  - `files` — id, name, mimetype, url_private, permalink, channel_id
  - `audit_log` — append-only log of every DM-channel read (privacy guard)
- **Sync cursor**: per-channel high-water-mark `latest_ts` in `sync_state` table; `sync --since 24h` never re-fetches.
- **FTS5**: virtual table over `messages.text + messages.user_name_resolved` for offline `search`, `who-said`, `customer-intel`.
- **Storage**: `%LOCALAPPDATA%\slack-pp-cli\data.db` on Windows (NOT Google Drive — sync churn). Year-1 size budget: 1.3-1.5 GB.
- **TTL**: rolling 365-day window default; older rows archived to compressed JSONL or dropped per config.

## User Vision (from briefing context)
> "Build pp-slack with these novel verbs (rather than thin endpoint wrappers):
>
> READ-SIDE (8): digest, customer-intel (HIGHEST VALUE), drift, dms-summary, dormant, attention, who-said, thread-summary
> WRITE-SIDE (2): post, schedule
> PLUMBING (2): channel-find / user-find, sync + search"
>
> Reference: scoping doc at `g:/My Drive/Cursor/workspace/evals/pp-slack-scoping-2026-05-11.md` for consuming-skills inventory, EOS-shape rationale, privacy considerations (DM redaction, audit log).

## Privacy Posture (load-bearing)
- **Local-only by default**: SQLite mirror on Erick's machine; no Drive/Supabase upstream from pp-slack itself.
- **Channel allow/deny list** at `~/.config/slack-pp-cli/skip.yaml` — `skip_channels: ['hr-private', 'compensation-discussion']`, `redact_dms_with: ['tatiana.marin', 'german.figueroa']`.
- **Audit log** at `%LOCALAPPDATA%\slack-pp-cli\audit.log` — every DM-channel read written with timestamp + caller + channel ID + verb.
- **`--redact-sensitivity` flag** strips compensation/HR keywords (`renunc`, `comp`, `salary`, `base`, `accelerator`, `pip`) before output. Mirrors weekly-digest skill's sensitivity rules.
- **Inherited from `feedback_digest_sensitive_content.md` (2026-05-11)**: weekly digests + L10 outputs are team-shareable, must NEVER include compensation, HR sensitive processes, or exit details. Caused by Alvar comp + German "salió" leaking to Latam page. pp-slack default redaction list MUST cover these.

## Product Thesis
- **Name**: `slack-pp-cli` (matches `attio-pp-cli` / `notion-pp-cli` / `asana-pp-cli` / `fathom-pp-cli`).
- **Module**: `github.com/erick-holm/slack-pp-cli`.
- **Why it should exist**:
  1. **Compound-query gap** — MCP can't sort by `last_reply_ts`, can't FTS5 across 100 channels, can't cross-join with Attio companies. Local SQLite mirror unlocks `customer-intel`, `drift`, `dms-summary`, `who-said`, `dormant` shapes the MCP literally cannot produce.
  2. **Cron-backbone gap** — 4 hardcoded xoxp tokens across `slack-daily-sync.py`, `slack-project-sync.py`, `one-on-one-prep/scripts/run.py`, slack-workspace-api SKILL.md. pp-slack is the forcing function to centralize auth on `SLACK_USER_TOKEN` env var with file fallback.
  3. **Rate-limit hedge** — even when AtomChat's xoxp survives the new regime today, the SQLite mirror means most queries hit the mirror not Slack. Cron load distribution stays bounded.
  4. **5/5 EOS-leadership-data CLI suite** — closes the loop alongside Attio + Notion + Asana + Fathom. Same pp-* shape, same SKILL routing rule pattern, same Press scorecard target (≥83 Grade A).

## Build Priorities
1. **Sync + SQLite mirror + FTS5** (P0) — channels, users, messages, threads, usergroups tables; cursor-aware pagination with `Retry-After` respect; silent-truncation warning when page size < requested.
2. **Spec-derived endpoint commands** (P1, generator-emitted) — all 174 endpoints absorbed. `admin.*` (56) generated but documented as "unreachable on non-admin tokens." `conversations.*` (18), `users.*` (12), `chat.*` (10), `files.*` (13) actively useful.
3. **12 novel EOS-shaped verbs** (P2) — the differentiation surface, all hand-built on top of the mirror.
4. **Privacy + auth centralization** — env var precedence, audit log, redaction flag, skip-channels config.
5. **MCP server** with `mcp:` block — `transport: [stdio, http]`, `orchestration: code`, `endpoint_tools: hidden`. Total tools = 174 endpoints + 12 verbs + ~13 framework = ~199 → matches the >50-tool Cloudflare pattern.
