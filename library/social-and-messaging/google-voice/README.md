# Google Voice CLI

**Read Google Voice through ordinary HTTP and recover evidence from a local message mirror.**

Collect account-scoped message history with resumable pagination, then search it locally. Inspect surrounding conversation, observed activity and collection coverage before answering from that history.

Learn more at [Google Voice](https://voice.google.com).

Created by [@mikemarqq](https://github.com/mikemarqq).

## Install

Install from the Printing Press catalog after this CLI is merged and released:

```bash
npx -y @mvanhorn/printing-press-library install google-voice
```

When testing a pull-request branch, build both binaries from this source directory:

```bash
go build -o build/stage/bin/google-voice-pp-cli ./cmd/google-voice-pp-cli
go build -o build/stage/bin/google-voice-pp-mcp ./cmd/google-voice-pp-mcp
```

Run the staged binaries by their absolute paths, or add the containing directory to your PATH.
The companion agent instructions are in SKILL.md; installing them into an agent is a separate local setup action.

## Use with an MCP agent

Run `google-voice-pp-cli mcp --json` for the sibling stdio server configuration.
The server is `build/stage/bin/google-voice-pp-mcp`; configure your agent to launch that absolute path.
It inherits the selected `GOOGLE_VOICE_PROFILE` and private local session.
No HTTP MCP listener is compiled into this build.
Import and verify the session with the CLI before using the MCP server.

This reader is verified on macOS. Voice session and mirror storage use Unix ownership and permission checks; Windows storage fails closed until directory ACL validation is implemented.

## Authentication

Google Voice uses an existing signed-in Google web session, not a consumer API key. Import credentials from a private local HAR with auth import-har; the CLI keeps only allowlisted session material in a machine-local profile and computes SAPISIDHASH for each fixed-host read request. Expired sessions require a fresh private import; a running browser is not required for ordinary reads.

## Quick Start

```bash
# Check command configuration without credentials or network.
google-voice-pp-cli doctor --dry-run

# Verify the imported profile identifies the expected Voice account.
google-voice-pp-cli account get --agent

# Collect a bounded amount of message history and persist continuation work.
google-voice-pp-cli mirror sync --max-pages 2 --agent

# Find literal matches in the local message mirror.
google-voice-pp-cli search appointment --limit 20 --agent

# Inspect observation freshness and unfinished collection before using the result.
google-voice-pp-cli mirror coverage --agent

```

## Recipes

### Find inbound appointment messages

```bash
google-voice-pp-cli history query --text appointment --direction inbound --limit 20 --agent --select messages.id,messages.thread_id,messages.timestamp,messages.text
```

Narrow literal evidence while preserving source identifiers.

### Inspect observed communication

```bash
google-voice-pp-cli history activity --since 30d --limit 50 --agent
```

Return counts and last-observed source times without assigning social meaning.

### Check collection coverage

```bash
google-voice-pp-cli mirror coverage --agent
```

Inspect freshness, inventory, older-history cursors and failed work.

### Inspect newly collected records

```bash
google-voice-pp-cli history arrivals --limit 100 --agent
```

Separate first-seen collection time from original message time.

## Unique Features

### Recover message evidence
- **`history query`** — Find messages using participant, dates, direction, literal text and attachment evidence together.

  _Use when a concrete question needs a bounded intersection of message facts._

  ```bash
  google-voice-pp-cli history query --text appointment --direction inbound --limit 20 --agent
  ```
- **`history context`** — Retrieve neighboring messages around a known message, including collection boundaries.

  _Use after finding an exact message when its surrounding exchange matters._

  ```bash
  google-voice-pp-cli history context --message-id sms-example --before 4 --after 4 --agent
  ```
- **`history activity`** — See observed inbound and outbound counts and latest message times per conversation.

  _Use to establish recent communication facts before a separate skill interprets them._

  ```bash
  google-voice-pp-cli history activity --since 30d --limit 50 --agent
  ```

### Understand collection state
- **`mirror coverage`** — See freshness, unfinished pagination and collection failures before relying on a lookup.

  _Use before treating an empty search as evidence that no communication occurred._

  ```bash
  google-voice-pp-cli mirror coverage --agent
  ```
- **`history arrivals`** — Inspect which records collection added, keeping old backfill distinct from recent communication.

  _Use to review collection growth and confirm repeated imports do not invent new arrivals._

  ```bash
  google-voice-pp-cli history arrivals --limit 100 --agent
  ```

## Usage

Run `google-voice-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | Settings, saved run profiles and private Voice `voice-profiles/<profile>/session.json` |
| `data` | Voice `voice-profiles/<profile>/messages.sqlite3`, plus separate framework learning/credential files when used |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `GOOGLE_VOICE_CONFIG_DIR`, `GOOGLE_VOICE_DATA_DIR`, `GOOGLE_VOICE_STATE_DIR`, or `GOOGLE_VOICE_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `GOOGLE_VOICE_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export GOOGLE_VOICE_HOME=/srv/google-voice
google-voice-pp-cli doctor
```

Under `GOOGLE_VOICE_HOME=/srv/google-voice`, the four dirs resolve to `/srv/google-voice/config`, `/srv/google-voice/data`, `/srv/google-voice/state`, and `/srv/google-voice/cache`.

On Windows, set the same variable in PowerShell:

```powershell
$env:GOOGLE_VOICE_HOME = "C:\srv\google-voice"
google-voice-pp-cli doctor
```

Under `$env:GOOGLE_VOICE_HOME = "C:\srv\google-voice"`, the four dirs resolve to `C:\srv\google-voice\config`, `C:\srv\google-voice\data`, `C:\srv\google-voice\state`, and `C:\srv\google-voice\cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "google-voice": {
      "command": "google-voice-pp-mcp",
      "env": {
        "GOOGLE_VOICE_HOME": "/srv/google-voice"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `GOOGLE_VOICE_DATA_DIR` overrides an explicit `--home` for that kind. Use `GOOGLE_VOICE_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `GOOGLE_VOICE_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Voice cookies are stored only in the private per-profile `session.json`; HAR import does not migrate framework credentials.
Run `google-voice-pp-cli doctor --fail-on warn --json` for local path/session warnings in automation; use `auth status --live` for remote verification.

### Local data

Voice live reads bypass the HTTP response cache and do not automatically save messages.
`mirror sync` explicitly collects pages into the selected account profile's private SQLite mirror.
The separate framework learning store and journal remain available for structural questions without message bodies or person identifiers.
The doctor command reports local profile, session and mirror paths; it does not measure remote connectivity or enumerate every database file.

Local data relies on OS account permissions and full-disk encryption; the CLI does not encrypt database contents.

- **Framework cleanup:** `google-voice-pp-cli local-data purge` previews framework store/cache/journal files; `--yes` opts into deletion, and `--only store`, `--only cache` or `--only journal` narrows it. Voice mirrors, custom database paths and credentials are retained, as stated in runtime help.
- **Disable collection:** `GOOGLE_VOICE_NO_LOCAL_DATA=1` disables framework response caching and blocks `mirror sync` before session/database access. Existing local mirrors remain readable.
- **Read mode:** `GOOGLE_VOICE_DATA_SOURCE=auto|live|local` supplies the global default, while Voice's live-only and local-only command boundaries remain explicit.

## Commands

### account

Verify the imported Google Voice account

- **`google-voice-pp-cli account get`** - Read the Google Voice account bound to the private imported session

### threads

Read Google Voice conversation inventory and messages

- **`google-voice-pp-cli threads get`** - Read one bounded message page without changing conversation read state
- **`google-voice-pp-cli threads list`** - List a bounded conversation page with explicit continuation and folder evidence


### history


- **`google-voice-pp-cli history query`** - Find messages using participant, dates, direction, literal text and attachment evidence together.

- **`google-voice-pp-cli history context`** - Retrieve neighboring messages around a known message, including collection boundaries.

- **`google-voice-pp-cli history activity`** - See observed inbound and outbound counts and latest message times per conversation.

- **`google-voice-pp-cli history arrivals`** - Inspect which records collection added, keeping old backfill distinct from recent communication.


### mirror

- **`google-voice-pp-cli mirror sync`** - Commit bounded source pages and resumable work to the private mirror.
- **`google-voice-pp-cli mirror export`** - Export bounded local records with source identifiers.


- **`google-voice-pp-cli mirror coverage`** - See freshness, unfinished pagination and collection failures before relying on a lookup.




### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`google-voice-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`google-voice-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`google-voice-pp-cli learnings list`** - Inspect taught rows
- **`google-voice-pp-cli learnings forget <query>`** - Undo a teach
- **`google-voice-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`google-voice-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`google-voice-pp-cli teach-pattern`** - Install a query/resource template up front
- **`google-voice-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `GOOGLE_VOICE_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `google-voice-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Account output (structured JSON; use --agent for provenance)
google-voice-pp-cli account get

# JSON for scripting and agents
google-voice-pp-cli account get --json
# Filter to specific fields
google-voice-pp-cli account get --json --select id,version

# Dry run — show the request without sending
google-voice-pp-cli account get --dry-run

# Agent mode — JSON + compact + no prompts in one flag
google-voice-pp-cli account get --agent
```

`--csv` prefixes a leading `'` on cells and header keys that start with `=`, `+`, `-`, `@`, a tab, or a line break (including fullwidth `＝＋－＠`) so a spreadsheet does not run them as formulas. Plain signed numbers such as `-5` and `+2.5e3` are left unchanged.

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
google-voice-pp-cli doctor
```

Reports local profile, session and mirror paths.
Run `google-voice-pp-cli auth status --live --json` to verify remote authentication.

## Configuration

Use `google-voice-pp-cli doctor --json` for the Voice session and mirror paths, or `agent-context --pretty` for the framework path-resolver metadata. The platform-default config path is `~/.config/google-voice-pp-cli/config.toml` on macOS/Linux and `%USERPROFILE%\.config\google-voice-pp-cli\config.toml` on Windows; `--home`, `GOOGLE_VOICE_HOME`, and per-kind env vars can relocate it.

The API origin is fixed to the verified Google client host.
Required Google headers come from the imported private session; this reader does not expose arbitrary per-command header overrides.

`--ca-cert`, `GOOGLE_VOICE_CA_CERT`, or `ca_cert` adds a PEM CA bundle to the system trust roots. `--tls-server-name`, `GOOGLE_VOICE_TLS_SERVER_NAME`, or `tls_server_name` is the name checked against the certificate when the dialed address does not match it. Precedence is flag, then environment, then the config file. A config file other users can change does not supply these settings. Certificate verification failures are not retried.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `google-voice-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run `google-voice-pp-cli threads list --json` to obtain current source thread IDs

### API-specific
- **Google returns 401 or the session is no longer usable.** — Capture a fresh signed-in Voice read HAR privately, import it with auth import-har, then run auth status --live.
- **A search is empty or older messages are missing.** — Run mirror coverage, then mirror sync with bounded --max-pages to continue pending inventory or thread history; an empty result is not completeness proof.
- **Google returns 429.** — Honor the reported retry time and rerun bounded mirror sync later; do not install a rapid polling loop.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**mautrix/gvoice**](https://github.com/mautrix/gvoice) — Go
- [**googlevoice-mcp**](https://github.com/zhuqf/googlevoice-mcp) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

## Cookbook

Recover evidence by running a bounded mirror sync, searching literal text, and passing an observed source message ID to history context.
Check mirror coverage before relying on a no-result lookup.
Use history arrivals to review archive growth, and history activity for source-time counts.
The Recipes section contains the corresponding invocations.

## Private profiles and collection limits

`GOOGLE_VOICE_PROFILE` selects a private Voice account profile; allowed names contain letters, digits, underscores and hyphens.
It defaults to `default` and is separate from the framework `--profile` flag for saved run flags.
Sessions live under the resolved config directory at `voice-profiles/<profile>/session.json`.
Messages live under the resolved data directory at `voice-profiles/<profile>/messages.sqlite3`.
The SQLite file has mode 0600 and its containing profile directory has mode 0700.
A profile pins the verified account ID and refuses a different account or database identity.
The framework learning store is separate from this message mirror.

Capture a successful signed-in Voice read as a HAR with sensitive headers, save it outside Dropbox, and restrict it to mode 0600.
Run `google-voice-pp-cli auth import-har --file /private/tmp/voice.har --json`, then `google-voice-pp-cli auth status --live --json`.
Restore sanitized HAR export in the browser after capturing.
The CLI imports only the successful fixed-host read session; it never prints cookies.
Google short-lived cookie rotation can invalidate a captured session; re-import a fresh capture when account verification returns an auth error.
The CLI retains compatible Set-Cookie updates but does not promise unattended login refresh.

The sync page budget includes inventory and message-history pages.
Run mirror sync again to resume saved work.
An inventory cursor and each thread's older-history cursor have separate checkpoints.
A bounded run may leave both unfinished; mirror coverage reports that state and explicitly marks folders never collected.
An exhausted cursor is an observation at its recorded time, not a guarantee of all historical data.
Text and archived folder mappings are live verified.
Call/voicemail metadata is preserved if returned, but dedicated call/voicemail folders and calling are outside this reader.
MMS attachment metadata is preserved; media bytes are neither downloaded nor uploaded.

No background service, relationship-memory adapter or identity merge is installed by this build.
Keep real messages, raw captures and credentials out of source, manuscripts and Dropbox.

The doctor command reports local setup; use auth status --live for remote verification.
The local-data purge command cleans framework cache/learning data and explicitly retains Voice profile mirrors and credentials.
GOOGLE_VOICE_NO_LOCAL_DATA=1 blocks mirror sync before any session or database access.
