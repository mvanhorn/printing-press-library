# Thunderbird CLI

**Search, read and audit the mail Thunderbird already downloaded — offline, read-only, no credentials.**

Indexes your Thunderbird profile (mbox stores, address book, filters) into a local full-text store you can query while Thunderbird is running. Adds what no other Thunderbird tool has: awaiting-reply, correspondent ranking, filters audit and bulk-mail triage.

## Install

The recommended path installs both the `thunderbird-pp-cli` binary and the `pp-thunderbird` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install thunderbird
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install thunderbird --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install thunderbird --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install thunderbird --agent claude-code
npx -y @mvanhorn/printing-press-library install thunderbird --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/cmd/thunderbird-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/thunderbird-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install thunderbird --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-thunderbird --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-thunderbird --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install thunderbird --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/thunderbird-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/cmd/thunderbird-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "thunderbird": {
      "command": "thunderbird-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No authentication. The CLI reads your local Thunderbird profile read-only and never touches passwords, key4.db or logins.json. It auto-detects the default profile; override with --profile or THUNDERBIRD_PROFILE.

## Quick Start

```bash
# Check that the profile is found and readable
thunderbird-pp-cli doctor

# Index mail, contacts and filters into the local store
thunderbird-pp-cli sync

# Full-text search across every account
thunderbird-pp-cli search "invoice" --limit 10

# Threads still waiting on your answer
thunderbird-pp-cli awaiting-reply --since 14d

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Inbox intelligence
- **`awaiting-reply`** — See every thread where someone is waiting on your answer, across all accounts, oldest first.

  _Reach for this when asked what mail still needs a reply instead of scanning recent messages._

  ```bash
  thunderbird-pp-cli awaiting-reply --since 14d --agent
  ```
- **`contacts top`** — Rank the people you exchange mail with by volume and recency, and spot frequent ones missing from your address book.

  _Use it to answer who someone is to the user and when they last talked._

  ```bash
  thunderbird-pp-cli contacts top --since 180d --agent
  ```

### Mailbox hygiene
- **`filters audit`** — Check every message filter: how often it matches, whether it is disabled, and whether its target folder still exists.

  _Use it when mail lands in the wrong folder or to clean up dead filter rules._

  ```bash
  thunderbird-pp-cli filters audit --since 90d --agent
  ```
- **`largest`** — Find the biggest messages or attachments across all accounts and folders.

  _Use it to free disk or mailbox quota._

  ```bash
  thunderbird-pp-cli largest --attachments --limit 20 --agent
  ```
- **`newsletters`** — List mailing lists and bulk senders by volume and how much of their mail you never read.

  _Use it to pick unsubscribe candidates._

  ```bash
  thunderbird-pp-cli newsletters --since 90d --min 3 --agent
  ```

## Recipes

### Find and read a mail

```bash
thunderbird-pp-cli search "contratto" --agent --select id,subject,from_addr,date
```

Narrow search results to the fields an agent needs, then open one with messages show.

### Read a long conversation cheaply

```bash
thunderbird-pp-cli threads show 3f9a1c2b7d4e --last 5 --agent --select id,date,direction,from_addr,total_messages
thunderbird-pp-cli messages show 3f9a1c2b7d4e --no-quotes --agent
```

Take only the latest messages of the thread, then read each without the quoted history. Inline signature images are already left out of attachment lists; `sync --full` recomputes the inline flag on stores synced by older versions.

### Who owes whom

```bash
thunderbird-pp-cli awaiting-reply --since 7d --agent
```

Threads where the last message is inbound and you have not replied.

### Unsubscribe candidates

```bash
thunderbird-pp-cli newsletters --since 90d --agent
```

Bulk senders ranked by volume and unread share.

### Draft a reply without sending

```bash
thunderbird-pp-cli drafts reply "3f9a1c2e7b40" --body "Thanks, confirmed." --agent
```

Prints the prefilled reply fields and the thunderbird -compose command line; add --open to open the compose window. Nothing is ever sent.

## Usage

Run `thunderbird-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: one mail index per Thunderbird profile at `profiles/<profile hash>/data.db` (`doctor` shows the path; an index left at `data.db` by earlier versions is no longer read, so run `sync` once to rebuild it) |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `THUNDERBIRD_CONFIG_DIR`, `THUNDERBIRD_DATA_DIR`, `THUNDERBIRD_STATE_DIR`, or `THUNDERBIRD_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `THUNDERBIRD_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

These variables only move the CLI's own files. The Thunderbird profile it reads is chosen separately: `--profile` or `THUNDERBIRD_PROFILE` picks the profile, and `THUNDERBIRD_ROOT` points at a non-standard Thunderbird directory (the one holding profiles.ini).

For containers and agent sandboxes, prefer a single relocated root:

```bash
export THUNDERBIRD_HOME=/srv/thunderbird
thunderbird-pp-cli doctor
```

Under `THUNDERBIRD_HOME=/srv/thunderbird`, the four dirs resolve to `/srv/thunderbird/config`, `/srv/thunderbird/data`, `/srv/thunderbird/state`, and `/srv/thunderbird/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "thunderbird": {
      "command": "thunderbird-pp-mcp",
      "env": {
        "THUNDERBIRD_HOME": "/srv/thunderbird"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `THUNDERBIRD_DATA_DIR` overrides an explicit `--home` for that kind. Use `THUNDERBIRD_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `THUNDERBIRD_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `thunderbird-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### accounts

Accounts and identities configured in Thunderbird

- **`thunderbird-pp-cli accounts`** - List accounts and identities from prefs.js

### attachments

Attachments of a message

- **`thunderbird-pp-cli attachments list`** - List the attachments of a message (index, filename, type, decoded size); inline signature logos and `cid:` images are hidden unless `--include-inline`
- **`thunderbird-pp-cli attachments save`** - Decode and save a message's attachments into a directory (`--output <dir>`); without `--index` inline parts are skipped unless `--include-inline`, `--index N` saves any part by its original index

### calendar

Local Thunderbird calendar

- **`thunderbird-pp-cli calendar events`** - List calendar events in a time window, earliest first

### contacts

Address book contacts

- **`thunderbird-pp-cli contacts search`** - Find contacts whose name or email contains the query (case-insensitive)
- **`thunderbird-pp-cli contacts show`** - Show one contact by card id or email address
- **`thunderbird-pp-cli contacts top`** - Rank correspondents by volume and recency

### drafts

Prepare messages in a Thunderbird compose window (never sends)

- **`thunderbird-pp-cli drafts new`** - Print the compose fields for a new message; `--open` opens the compose window
- **`thunderbird-pp-cli drafts reply`** - Prepare a reply (or `--all` reply-all) with the original quoted; `--open` opens the compose window

### filters

Message filter rules

- **`thunderbird-pp-cli filters list`** - List filter rules with action, target folder and condition summary
- **`thunderbird-pp-cli filters audit`** - Hits, disabled rules and missing target folders per filter

### folders

Mail folders across all accounts

- **`thunderbird-pp-cli folders`** - List folders with total, unread, size and offline status

### messages

Messages indexed from the Thunderbird profile's mbox stores

- **`thunderbird-pp-cli messages show`** - Show one message with headers, flags, text body and attachments (`get` is an alias); `--no-quotes` keeps only the new text of the body, `--include-inline` also lists inline parts
- **`thunderbird-pp-cli messages list`** - List indexed messages, newest first
- **`thunderbird-pp-cli messages auth`** - Show SPF, DKIM and DMARC verdicts of a message
- **`thunderbird-pp-cli messages export`** - Export messages as the original .eml or as JSON

### threads

Conversations across folders

- **`thunderbird-pp-cli threads show`** - Show every message of a thread in chronological order with direction in/out; `--last N` keeps the N most recent, every row carries `total_messages`

### Store and profile

- **`thunderbird-pp-cli sync`** - Index the local Thunderbird profile into the offline SQLite store
- **`thunderbird-pp-cli search`** - Full-text search of the synced store
- **`thunderbird-pp-cli stats`** - Show per-account totals: messages, unread, flagged, folders, last message date
- **`thunderbird-pp-cli profiles`** - List Thunderbird profiles from profiles.ini and which one is selected
- **`thunderbird-pp-cli doctor`** - Check the Thunderbird profile, its accounts and folders, and the local store


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`thunderbird-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`thunderbird-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`thunderbird-pp-cli learnings list`** - Inspect taught rows
- **`thunderbird-pp-cli learnings forget <query>`** - Undo a teach
- **`thunderbird-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`thunderbird-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`thunderbird-pp-cli teach-pattern`** - Install a query/resource template up front
- **`thunderbird-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `THUNDERBIRD_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `thunderbird-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
thunderbird-pp-cli accounts

# JSON for scripting and agents
thunderbird-pp-cli accounts --json
# Filter to specific fields by name
thunderbird-pp-cli accounts --json --select id,name,identity_emails

# Dry run — report what the command would do without running it
thunderbird-pp-cli sync --dry-run

# Agent mode — JSON + compact + no prompts in one flag
thunderbird-pp-cli accounts --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` reports what a command would do without running it
- **Read-only** - never writes to the Thunderbird profile and never sends, moves, flags or deletes mail; `drafts --open` only opens a prefilled compose window you send yourself
- **Offline** - reads only the local Thunderbird profile and the CLI's SQLite store; no network or mail server access
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `1` other error, `2` usage error, `3` not found (message, contact, identity or attachment), `10` config error.

## Health Check

```bash
thunderbird-pp-cli doctor
```

Verifies the Thunderbird profile, its accounts and folders, and the local store.

## Configuration

Run `thunderbird-pp-cli doctor` to see the selected Thunderbird profile and the resolved config, data, state, and cache directories; `--home`, `THUNDERBIRD_HOME`, and per-kind env vars can relocate the CLI's own directories.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the message or contact id; ids come from `messages list`, `search` or `contacts search`
- Run `sync` again if the message arrived after the last sync

### API-specific
- **Profile not found** — Pass --profile <path to profile dir> or set THUNDERBIRD_PROFILE
- **A folder shows 0 messages** — Enable offline sync for that folder in Thunderbird (Folder Properties > Synchronization), then run sync again

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**avikalpa/thunderbird-cli**](https://github.com/avikalpa/thunderbird-cli) — Go
- [**mudler/thunderbird-mcp**](https://github.com/mudler/thunderbird-mcp) — Go
- [**vitalio-sh/thunderbird-cli**](https://github.com/vitalio-sh/thunderbird-cli) — JavaScript
- [**lanterieur/thunderbird-sqlite-eml**](https://github.com/lanterieur/thunderbird-sqlite-eml) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
