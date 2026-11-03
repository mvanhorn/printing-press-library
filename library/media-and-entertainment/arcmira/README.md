# YouTube Transcript Search CLI by Arcmira

[Arcmira](https://arcmira.com) · [API docs](https://arcmira.com/docs)

Search indexed YouTube and podcast transcripts for timestamped quotes and clip boundaries. Find mentions of people, organizations, products and topics; research channel sponsors and recommendations; retrieve creator captions or Premium transcripts with speaker identification; and monitor entities for new mentions. An explicit Premium read uses credits from the account's plan, then its on-demand budget.


## Build a quote sheet

Combine saved transcript searches for a fact-check or clip shortlist. Repeated passages keep every matching query. Each row includes the exact words and a timestamped source link. The merge runs locally.

```sh
arcmira-pp-cli quote-sheet first-search.json second-search.json --json
```

## Install

The recommended path installs both the `arcmira-pp-cli` binary and the `pp-arcmira` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install arcmira
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install arcmira --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install arcmira --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install arcmira --agent claude-code
npx -y @mvanhorn/printing-press-library install arcmira --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.10 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/arcmira/cmd/arcmira-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/arcmira-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install arcmira --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-arcmira --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-arcmira --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install arcmira --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/arcmira-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `ARCMIRA_BEARER_AUTH` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/arcmira/cmd/arcmira-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS, `%APPDATA%\Claude\claude_desktop_config.json` on Windows):

```json
{
  "mcpServers": {
    "arcmira": {
      "command": "arcmira-pp-mcp",
      "env": {
        "ARCMIRA_BEARER_AUTH": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your access token from your API provider's developer portal, then store it:

```bash
echo "$TOKEN" | arcmira-pp-cli auth set-token
```

Or set it via environment variable:

```bash
export ARCMIRA_BEARER_AUTH="your-token-here"
```

### 3. Verify Setup

```bash
arcmira-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
arcmira-pp-cli health
```

## Usage

Run `arcmira-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `ARCMIRA_CONFIG_DIR`, `ARCMIRA_DATA_DIR`, `ARCMIRA_STATE_DIR`, or `ARCMIRA_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `ARCMIRA_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export ARCMIRA_HOME=/srv/arcmira
arcmira-pp-cli doctor
```

Under `ARCMIRA_HOME=/srv/arcmira`, the four dirs resolve to `/srv/arcmira/config`, `/srv/arcmira/data`, `/srv/arcmira/state`, and `/srv/arcmira/cache`.

On Windows, set the same variable in PowerShell:

```powershell
$env:ARCMIRA_HOME = "C:\srv\arcmira"
arcmira-pp-cli doctor
```

Under `$env:ARCMIRA_HOME = "C:\srv\arcmira"`, the four dirs resolve to `C:\srv\arcmira\config`, `C:\srv\arcmira\data`, `C:\srv\arcmira\state`, and `C:\srv\arcmira\cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "arcmira": {
      "command": "arcmira-pp-mcp",
      "env": {
        "ARCMIRA_HOME": "/srv/arcmira"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `ARCMIRA_DATA_DIR` overrides an explicit `--home` for that kind. Use `ARCMIRA_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `ARCMIRA_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `arcmira-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

### Local data

Reads are saved locally by default: in `--data-source auto` mode every successful read is also written to the SQLite store (`data.db`, or `data-<hash>.db` per credential) in the data directory. GET responses are cached for five minutes under `http/` in the cache directory. Credentials live in `credentials.toml` in the data directory. The learn journal lives under `learn/` in the state directory. `arcmira-pp-cli doctor` prints the resolved paths, every store file, and the response-cache size.

None of it is encrypted. New files are created owner-only; beyond that the data relies on OS account permissions and full-disk encryption (FileVault, BitLocker, LUKS).

- **Erase it:** `arcmira-pp-cli local-data purge` lists the store files, response cache, and learn journal it would delete; add `--yes` to delete them, or `--only store|cache|journal` to narrow it. Config and credentials are kept.
- **Run without a local copy:** set `ARCMIRA_NO_LOCAL_DATA=1`. Reads go to the API (`--data-source live`; `--data-source local` still reads an existing store), the response cache is off (`--no-cache`), nothing new is written to the store, and `sync` and `workflow archive` refuse to run. Set it in an MCP host's `env` block and every command the server runs inherits it.
- **Choose the read source without flags:** `ARCMIRA_DATA_SOURCE=auto|live|local` sets the `--data-source` default; an explicit flag wins.

## Commands

### api_feedback

Manage api feedback

- **`arcmira-pp-cli api-feedback get-feedback`** - Returns the submission (id, type, query, notes, created_at) plus its corrections, each with a review status in the public vocabulary: pending_review, needs_information, accepted, accepted_with_changes, rejected, withdrawn, applied, reverted (accepted means a reviewer agreed; applied means the change is live in the index). Only the submitting user's keys can read a submission; unknown ids and other users' submissions both return 404 (never 403).
- **`arcmira-pp-cli api-feedback submit-feedback`** - Help shape Arcmira. Report a wrong result, request sources or capabilities, or tell us what would make your workflow more useful. To correct a result, attach corrections to the exact query you ran: pass the feedback type, the query object you sent, and optional per-item corrections. Public submissions are recorded for human review (status "logged"); nothing is auto-applied. Read the review status back later via GET /v1/feedback/{feedback_id}. recommendations and channel_sponsors feedback types require a Pro+ plan; every other type needs read. monitor_alert feedback targets fired alerts: query carries monitor_id and/or tracker_id and/or alert_id, corrections target the alert id, and every referenced alert must belong to the caller (otherwise 404 alert_not_found). missed_alert corrections are expectations with nothing to target: omit the correction id and put { source_url, approximate_timestamp_seconds?, entity_id? } in suggested_change. delivery_issue corrections may carry { channel } in suggested_change. experience feedback says how a task went as a whole rather than correcting a result: it requires category and notes, refuses corrections (400 invalid_feedback_request), and needs no query. category and mcp_call_id, when sent, are recorded in the stored query.

### channels

Manage channels


### entities

Manage entities

- **`arcmira-pp-cli entities get-entity`** - Returns the canonical entity envelope for an ent_{n} or numeric id, following merge redirects. For organization and product entities, callers with Recommendations API access (a Pro+ plan) also receive a recommendations_summary commercial-intelligence rollup when a brand profile exists.
- **`arcmira-pp-cli entities resolve-entity`** - Call this before passing an id to about, by, entity_ids, channel_ids or channel; those filters refuse names with id_required. Pass context with the user's own words about the name ("the startup bank", "on My First Million"). The answer is one of three: best (the name means one entity: use it and name it), suggested (no entity is certain but one stands out, with reason and evidence: use it and tell the user you assumed it), or ask (several entities fit: show ask.options, or check every option id and answer per entity). For a show pass type=channel and use the youtube_channel_id; for a brand or a person use the id. Free; uses no credits.

### health

Manage health

- **`arcmira-pp-cli health`** - Health check

### integrations

Manage integrations

- **`arcmira-pp-cli integrations`** - The account's active Slack workspaces with the ids a monitor needs for Slack delivery: create or PATCH a monitor with notify_slack: true, slack_integration_id set to an id here, and optionally slack_channel_id (default_channel_id when omitted). Slack is connected in the dashboard, never through the API; an empty list means the user must connect it there first. Reads stored data only. Single page, no pagination.

### me

Manage me

- **`arcmira-pp-cli me get`** - Available even when the account has exhausted its usage allowance. Returns the credential making the request (key_id, key_label, credential_kind), the masked email of the person, the account whose credits it spends (account: id, name, kind, plan) and the role the person holds on it, the tier, scopes, rate limit, freshness (where results stop on a plan that reads media after a delay, or null), usage with period_resets_at, and account settings. usage.credits is the primary measure: credits from the plan, then the on-demand budget. The row fields restate it at 4 credits a row. settings.transcripts is what a transcript request that names no parameter of its own receives: every key of the account resolves against it.
- **`arcmira-pp-cli me update-my-settings`** - Sets the account defaults every key of the account resolves against. Send only the fields you are changing; an omitted field keeps the value it has. Resolution order for every transcript request is the explicit parameter, then these settings, then the platform default, so a default never overrides a parameter the caller sent. Defaults live on the account, never on a key: two keys of one account answer the same request the same way. The response echoes the resolved settings.

### mentions

Manage mentions

- **`arcmira-pp-cli mentions count`** - A small ranked table of entity and channel counts, all-time unless after is set. Pass channel_ids for what shows talk about and entity_types to match the question (topic for subjects, person for guests, organization,product for brands). Pass video_ids with one id from GET /v1/channels/{channel_id}/videos for what a single episode mentions. Two or more channel_ids also return shared, the entities on more than one of them ranked by the smallest per-channel count, which is true overlap. An after later than the plan's freshness gate is refused with freshness_requires_paid rather than widened. A window the gate cut carries freshness (delay_days, served_through), unlock, and a note that says where results on this plan stop. Uses 4 credits per ranked entry returned past the first 5. Every gate is a typed error whose error.unlock.url names the plan that lifts it; pass src=mcp-tool only from the Arcmira MCP server.
- **`arcmira-pp-cli mentions list`** - Find where a person, company, product, or topic is mentioned in indexed videos and podcasts. For a person, set `is_appearance` to true to find media appearances, false for mentions by others, or omit it for both. Results include a description, source video, and available timestamps. Cursor-paginated, filtered by entity (`entity_id` is required; resolve a name first with GET /v1/entities/resolve, or the call answers 400 id_required naming the parameter), channel (`channel_id`), text query, sentiment, appearance flag, and publication window [after, before). The signed continuation binds the route, filters, caller and visibility; invalid or old cursors return invalid_cursor. A first-page ID fence excludes later insertions, including old-date backfills. Edits and deletions to existing mentions remain live. Positions are start_seconds and end_seconds (integer seconds; 0 means full episode). is_appearance filtering applies to person entities only; passing is_appearance=true for any other type returns a 400 (appearances_person_only). details=full attaches per-mention commercial recommendations and requires a Pro+ plan.

### monitors

Manage monitors

- **`arcmira-pp-cli monitors create`** - Creating with notify_webhook: true and a webhook_url enables HMAC-signed webhook delivery and returns the signing secret (monitor.webhook_secret) in this response. Store it securely. A retry with the original Idempotency-Key recovers the same secret for up to 24 hours while it remains the current secret or the valid previous secret. An expired or displaced secret returns 409 idempotency_result_expired without rotating again. Reads do not expose the secret. All subsequent reads expose only webhook_secret_set and webhook_secret_hint. Creating monitors and trackers is free. Each alert uses 25 credits once, however many channels and recipients deliver it: credits from your plan, then your on-demand budget. When the account is out of credits, alerts wait instead of sending.
- **`arcmira-pp-cli monitors delete`** - Deletes the monitor AND every tracker inside it (trackers_deleted reports how many). Anyone in the account can restore it, with its trackers and recipients, from Recently deleted on the dashboard Monitors page for 30 days. Retrying with the original Idempotency-Key returns the original deleted count without deleting again.
- **`arcmira-pp-cli monitors list`** - All monitors for the account with tracker counts, alert counts for the current calendar month, and Slack display metadata. Single page, no pagination.
- **`arcmira-pp-cli monitors update`** - A PATCH that newly enables webhook signing (turns notify_webhook on, or sets a webhook_url where no secret existed before) returns the signing secret (monitor.webhook_secret) in this response. Store it securely. A retry with the original Idempotency-Key recovers the same secret for up to 24 hours while it remains the current secret or the valid previous secret. An expired or displaced secret returns 409 idempotency_result_expired without rotating again. Reads do not expose the secret. Unrelated PATCHes expose only webhook_secret_set and webhook_secret_hint. PATCHing notify_webhook: true also re-enables a webhook that was auto-disabled after repeated failures and resets its failure counter.

### openapi-json

Manage openapi json

- **`arcmira-pp-cli openapi-json`** - OpenAPI document

### recommendations

Manage recommendations

- **`arcmira-pp-cli recommendations`** - Cursor-paginated commercial mentions (sponsored, organic and neutral mentions) filtered by entity (entity_id is required; resolve a name first with GET /v1/entities/resolve, or the call answers 400 id_required naming the parameter), channel (channel_id), class, confidence, and publication window [after, before). The signed continuation binds the route, filters, caller and visibility; invalid or old cursors return invalid_cursor. A first-page ID fence excludes later insertions, including old-date backfills. Edits and deletions to existing recommendations remain live. Requires a Pro+ plan. Positions are start_seconds and end_seconds (integer seconds).

### signups

Manage signups

- **`arcmira-pp-cli signups create`** - Starts the signup that ends in an account key, with no key and no login. Sends a 6 digit code to the address, valid for 10 minutes and 5 attempts, and answers 202 with the verify call. Sends are capped at 3 per address per hour, 10 per IP per hour, and 25 per client fingerprint per day; past a cap the response is 429 signup_send_limited with retry_after_seconds and an unlock whose action is this call. An address that already has an account gets a code too; verifying it mints a key on that account. Send { "email": "agent@example.com" }, with an optional "src" naming the surface that sent you.
- **`arcmira-pp-cli signups verify`** - Consumes the code POST /v1/signups sent, creates the account when the address has none, and mints an arc_sk_ key on it: the read scope, the free plan's 1,000 credits a month, no expiry. A wrong, expired, or spent code is 400 signup_code_invalid on param code, naming the attempts left; its unlock action is a new send. Send { "email": "agent@example.com", "code": "482913" }.

### trackers

Manage trackers

- **`arcmira-pp-cli trackers delete`** - Deletes the tracker. Cannot be undone. Its monitor stays, with its delivery settings, even when this was its last tracker.
- **`arcmira-pp-cli trackers list`** - All trackers for the account, newest first, with per-channel delivery counts for the current billing period. Single page, no pagination.
- **`arcmira-pp-cli trackers update`** - Partial update: send only the fields to change. The tracked entity itself (entity_name/entity_type) is immutable; delete and recreate to watch a different entity. Delivery is the monitor's: change it with PATCH /v1/monitors/{id}.

### transcript_search

Manage transcript search

- **`arcmira-pp-cli transcript-search`** - Search indexed YouTube and podcast transcripts for passages matching a word or phrase. Returns passage text, source metadata, publication dates, and timestamped watch links. Narrow to passages about an entity with `about`, or passages where a person speaks with `by`. Scope with `channel_ids` (or `channel`) and `entity_ids` (a person id filters to that person's appearances). Filter to sponsored, organic or mention passages with `kind`. Every filter takes ids, never names: resolve a name first with GET /v1/entities/resolve, or the call answers 400 id_required naming the parameter. Results carry names beside ids (filters.about, filters.by, chunk about and speakers_by). Use one topic per call. Search results include text on every plan within the plan's publication-date window. Explicitly requesting source=arcmira_premium on a plan without Premium transcripts is refused with filter_requires_paid. An after later than the plan's freshness gate is refused with freshness_requires_paid rather than widened. A window the gate cut carries freshness (delay_days, served_through), unlock, and a note that says where results on this plan stop. Uses 4 credits per chunk returned past the first 5. Every gate is a typed error whose error.unlock.url names the plan that lifts it; pass src=mcp-tool only from the Arcmira MCP server.

### transcriptions

Manage transcriptions

- **`arcmira-pp-cli transcriptions`** - Your transcription requests in descending creation time and id order. limit defaults to 20 and accepts 1–100. Follow next_cursor with the same video_id, limit and credential; has_more is false and next_cursor is null on the last page. A traversal excludes requests inserted after its first page. Each entry is a Premium transcript job with its processing and billing state, a `status_url` for the Premium transcript GET, and a `title` field (the video title, null when unknown). The scheduled reconciler advances requests; reading this list never dispatches work or changes billing. In-flight entries carry `eta_seconds` and `next_poll_seconds`.

### transcripts

Manage transcripts

- **`arcmira-pp-cli transcripts <video_id>`** - Caption reads use 4 credits per started 15 minutes. quality=premium is one read. An owned transcript answers 200 ready at no charge. Otherwise this call starts a Premium transcript of the whole video, 300 credits per started 15 minutes, using credits from the account's plan first and then the account's on-demand budget up to its limit, and answers 202 pending with the job and Retry-After until the transcript is ready. Read again after Retry-After; repeated reads join the same job and never charge twice. When the last Premium transcript for the video failed, the read answers 200 state failed with the job and last_attempt and charges nothing; retry=true starts a new one. When the plan or the budget blocks, 403 paid_plan_required (with unlock) or 402 quota_exceeded or spend_limit_exceeded carries the price in quote and the limit it met in limit, and nothing is charged. A default-premium account with nothing owned reads captions with a note. On captions, start and end set the billed window: 4 credits per started 15 minutes of end minus start. On Premium they only trim the returned content, and the charge is the whole video. language selects caption tracks, timestamps=false returns paragraphs. Premium lines carry speaker and index, and the body carries speakers and revision. language picks the language: language= returns the base transcript (base_language), and a language the transcript is not in returns it translated, line for line with the same timestamps and speakers. Captions offer only the languages YouTube makes available for the video (translation_languages), billed as a caption read; any other language is Premium: an owned Premium transcript in another language costs the Premium rate once per account and language.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`arcmira-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`arcmira-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`arcmira-pp-cli learnings list`** - Inspect taught rows
- **`arcmira-pp-cli learnings forget <query>`** - Undo a teach
- **`arcmira-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`arcmira-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`arcmira-pp-cli teach-pattern`** - Install a query/resource template up front
- **`arcmira-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `ARCMIRA_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `arcmira-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
arcmira-pp-cli health

# JSON for scripting and agents
arcmira-pp-cli health --json
# Filter to specific fields
arcmira-pp-cli health --json --select status,time,version

# Dry run — show the request without sending
arcmira-pp-cli health --dry-run

# Agent mode — JSON + compact + no prompts in one flag
arcmira-pp-cli health --agent
```

`--csv` prefixes a leading `'` on cells and header keys that start with `=`, `+`, `-`, `@`, a tab, or a line break (including fullwidth `＝＋－＠`) so a spreadsheet does not run them as formulas. Plain signed numbers such as `-5` and `+2.5e3` are left unchanged.

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `6` partial failure, `7` rate limited, `10` config error.

## Health Check

```bash
arcmira-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `arcmira-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/arcmira-pp-cli/config.toml` on macOS/Linux and `%USERPROFILE%\.config\arcmira-pp-cli\config.toml` on Windows; `--home`, `ARCMIRA_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

`--ca-cert`, `ARCMIRA_CA_CERT`, or `ca_cert` adds a PEM CA bundle to the system trust roots. `--tls-server-name`, `ARCMIRA_TLS_SERVER_NAME`, or `tls_server_name` is the name checked against the certificate when the dialed address does not match it. Precedence is flag, then environment, then the config file. A config file other users can change does not supply these settings. Certificate verification failures are not retried.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `ARCMIRA_BEARER_AUTH` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `arcmira-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `arcmira-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $ARCMIRA_BEARER_AUTH`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
