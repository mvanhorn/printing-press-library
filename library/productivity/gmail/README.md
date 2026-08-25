# Gmail CLI

**Mailbox cleanup that can prove itself — preview, confirm, undo, verify — from a binary that structurally cannot send email.**

Every read and cleanup surface from the Gmail tool landscape, multi-account, with a local SQLite store underneath: sender intelligence, all-category digests, bulk trash/label with preview-confirm-undo, RFC 8058 one-click unsubscribes with a compliance ledger. Send, drafts, settings, and permanent deletion are absent from the binary by construction — Trash is the ceiling.

Learn more at the [Gmail API docs](https://developers.google.com/gmail/api).

Created by [@dmarketingllm](https://github.com/dmarketingllm) (Derik Parkinson).

## Install

The recommended path installs both the `gmail-pp-cli` binary and the `pp-gmail` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install gmail
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install gmail --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install gmail --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install gmail --agent claude-code
npx -y @mvanhorn/printing-press-library install gmail --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/gmail-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install gmail --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-gmail --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-gmail --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install gmail --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/gmail-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `GMAIL_OAUTH2C` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "gmail": {
      "command": "gmail-pp-mcp",
      "env": {
        "GMAIL_USER_ID": "me",
        "GMAIL_OAUTH2C": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Installed-app OAuth with named multi-account profiles (per-profile token store, consented-account verification). The only scope ever requested is gmail.modify; send/draft/settings endpoints do not exist in this binary, and permanent deletion is impossible under this scope — Google enforces the Trash ceiling, not us.

## Quick Start

```bash
# One-time OAuth consent per named account profile
gmail-pp-cli accounts auth --account personal

# Build the local store the intelligence commands run on
gmail-pp-cli sync --account personal --full

# The all-category summary: counts, unread aging, top senders per tab
gmail-pp-cli digest --account personal --agent

# Who actually fills this mailbox — volume, size, unread rate
gmail-pp-cli senders --account personal --top 20 --agent

# Preview first, always: counts and samples before anything moves
gmail-pp-cli cleanup plan --q "category:promotions older_than:1y" --action trash --account personal

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Cleanup that compounds
- **`unsub verify`** — See which senders kept mailing you after a one-click unsubscribe, with an escalation query per violator.

  _After running unsubscribes, this is the only way to learn which ones actually stuck before escalating to block/filter decisions._

  ```bash
  gmail-pp-cli unsub verify --account ads --agent
  ```
- **`rules run`** — Named local recipes (query plus trash or label action) replayed through the preview-confirm-undo engine as one merged plan.

  _Standing hygiene (old promos to trash, receipts to their folder) becomes one command a scheduled job can run daily, always previewed before applying._

  ```bash
  gmail-pp-cli rules run --plan-only --agent
  ```
- **`sort suggest`** — Senders whose mail you already label consistently, with a generated plan to label the rest the same way.

  _Folder-sorting at scale without guessing: the plan only proposes what the operator's own labeling history already proves._

  ```bash
  gmail-pp-cli sort suggest --account personal --min-confidence 0.8 --agent
  ```

### Local state that answers instantly
- **`delta`** — Everything new since your last check: new messages per category and sender, never-seen senders, volume spikes.

  _The first question of any recurring mailbox check is what changed — this answers it without re-reporting what the operator already saw._

  ```bash
  gmail-pp-cli delta --account personal --agent
  ```
- **`storage report`** — Which senders, labels, years, and attachments own your storage, with ready-to-run cleanup queries per row.

  _Turns a vague quota number into a ranked hit-list whose rows paste straight into cleanup plan._

  ```bash
  gmail-pp-cli storage report --account ads --top 15 --agent
  ```

### Safety made visible
- **`trash report`** — What you trashed, grouped by applied plan, with days remaining before Gmail's 30-day purge makes undo impossible.

  _The last regret-check: surfaces batches whose undo window is closing so recovery happens while it still can._

  ```bash
  gmail-pp-cli trash report --closing-soon --agent
  ```
- **`score`** — Per-account hygiene metrics — unread share, promo share, subscription count, storage headroom — snapshotted over time.

  _Shows whether the cleanup campaign is actually winning — Promotions down 60% since baseline beats a feeling._

  ```bash
  gmail-pp-cli score --account ads --agent
  ```

## Recipes

### Morning mailbox delta

```bash
gmail-pp-cli delta --account ads --agent
```

What arrived since the last check, grouped by category and sender, without re-reporting what was already seen.

### Top senders, narrowed for an agent

```bash
gmail-pp-cli senders --account personal --top 25 --agent --select senders.email,senders.count,senders.unread_rate
```

Dotted --select keeps the payload to the three fields a triage decision needs.

### Unsubscribe audit before acting

```bash
gmail-pp-cli unsub audit --account ads --min-count 5 --agent
```

Ranks subscription senders by volume and classifies each as one-click (actionable) or mailto-only (desk list).

### Preview a year-old promotions purge

```bash
gmail-pp-cli cleanup plan --q "category:promotions older_than:1y" --action trash --account personal --agent
```

Counts and samples first; the printed plan token is required by cleanup apply, and every apply is undoable.

### Verify last week's unsubscribes stuck

```bash
gmail-pp-cli unsub verify --account ads --since 7d --agent
```

Joins the unsubscribe ledger to fresh arrivals — violators come back with an escalation query.

## Usage

Run `gmail-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `GMAIL_CONFIG_DIR`, `GMAIL_DATA_DIR`, `GMAIL_STATE_DIR`, or `GMAIL_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `GMAIL_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export GMAIL_HOME=/srv/gmail
gmail-pp-cli doctor
```

Under `GMAIL_HOME=/srv/gmail`, the four dirs resolve to `/srv/gmail/config`, `/srv/gmail/data`, `/srv/gmail/state`, and `/srv/gmail/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "gmail": {
      "command": "gmail-pp-mcp",
      "env": {
        "GMAIL_HOME": "/srv/gmail"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `GMAIL_DATA_DIR` overrides an explicit `--home` for that kind. Use `GMAIL_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `GMAIL_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `gmail-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

The mutation surface is deliberately narrow: raw modify/trash/delete subcommands do not exist. Every mailbox mutation flows through `cleanup plan` → `cleanup apply` (reversible via `undo`), `labels create`/`labels rename`, or `unsub plan` → `unsub run` — all gated by one-time plan tokens.

### Mailbox engine

- **`gmail-pp-cli accounts`** / **`accounts auth`** - List gauth profiles and their token status; run the browser OAuth flow for one profile (or `--all`).
- **`gmail-pp-cli sync`** - Sync Gmail message metadata into the local store (full or historyId-incremental); local-only writes, never mutates the mailbox.
- **`gmail-pp-cli digest`** - Per-category mailbox summary: totals, unread, oldest-unread age, top senders, and size — plus an account rollup.
- **`gmail-pp-cli senders`** - Rank senders in the local store by volume, with size, unread rate, and unsubscribe capability.
- **`gmail-pp-cli delta`** / **`storage report`** / **`trash report`** / **`score`** / **`sort suggest`** - The local-intelligence reports described under Unique Features.
- **`gmail-pp-cli cleanup plan|apply|recover`** - Preview-confirm-undo cleanup: `plan` freezes what would change (mailbox untouched), `apply` executes exactly that, `recover` finishes a crashed apply.
- **`gmail-pp-cli undo --ledger <id>`** - Reverse a ledgered apply delta-by-delta; ids whose state changed since are skipped as conflicts, never forced.
- **`gmail-pp-cli rules add|list|rm|run`** - Named local cleanup recipes replayed through the preview-confirm-undo engine as one merged plan; `run` always stops at the plan.
- **`gmail-pp-cli unsub audit|plan|run|verify`** - One-click unsubscribe engine: `audit` classifies senders, `plan` freezes who to leave, `run` POSTs RFC 8058 one-click unsubscribes (hardened), `verify` catches senders that keep mailing.
- **`gmail-pp-cli search`** / **`analytics`** / **`tail`** / **`workflow archive|status`** / **`api`** - Local full-text search and analytics over synced data, polling change stream, archive workflows, and endpoint browsing.

### API reads (plus the two safe label writes)

`<userId>` is the Gmail user id — `me` works for the authenticated account.

- **`gmail-pp-cli history <userId>`** - Lists the history of all changes to the given mailbox. History results are returned in chronological order (increasing `historyId`).
- **`gmail-pp-cli labels list|get`** - Read labels.
- **`gmail-pp-cli labels create`** - Create a label, idempotently by name: a case-insensitive existing match returns that label instead of minting a duplicate.
- **`gmail-pp-cli labels rename`** - Rename a label by id, ledgering the inverse so `undo --ledger <id>` can verify-and-reverse it. There is no `labels delete` and no raw update/patch.
- **`gmail-pp-cli messages list|get`** / **`messages attachments get`** - Read messages and attachments (reads only — mutations go through the cleanup engine).
- **`gmail-pp-cli threads list|get`** - Read threads (thread-level mutations do not exist in this binary).
- **`gmail-pp-cli users-profile <userId>`** - Gets the current user's Gmail profile.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`gmail-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`gmail-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`gmail-pp-cli learnings list`** - Inspect taught rows
- **`gmail-pp-cli learnings forget <query>`** - Undo a teach
- **`gmail-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`gmail-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`gmail-pp-cli teach-pattern`** - Install a query/resource template up front
- **`gmail-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `GMAIL_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `gmail-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
gmail-pp-cli labels list me

# JSON for scripting and agents
gmail-pp-cli labels list me --json

# Filter to specific fields
gmail-pp-cli labels list me --json --select id,name

# Dry run — show the request without sending
gmail-pp-cli labels list me --dry-run

# Agent mode — JSON + compact + no prompts in one flag
gmail-pp-cli labels list me --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to treat an already-existing create as a successful no-op (`labels create` already matches by name)
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error. The mutation engine (`cleanup apply|recover`, `unsub run`, `undo`) reuses `3` for a partial run and `7` for a busy apply lock — each command's `--help` documents its typed exits.

## Runtime Endpoint

This CLI resolves endpoint placeholders at runtime, so one installed binary can target different tenants or API versions without regeneration.

Endpoint environment variables:
- `GMAIL_USER_ID` resolves `{userId}`

Base URL: `https://gmail.googleapis.com`

## Health Check

```bash
gmail-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `gmail-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/gmail-pp-cli/config.toml`; `--home`, `GMAIL_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `GMAIL_USER_ID` | endpoint | MCP host config; CLI commands take `<userId>` positionally | Default for `{userId}` when a call doesn't supply it — Gmail accepts `me`. |
| `GMAIL_OAUTH2C` | per_call | MCP host config; optional for the CLI | Pasted OAuth bearer token, used as a fallback when no `accounts auth` profile token is minted. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `gmail-pp-cli doctor` reports `agentcookie: detected` and `auth status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `gmail-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $GMAIL_OAUTH2C`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized on any command** — Re-run gmail-pp-cli accounts auth --account <name> — the refresh token was revoked (password changes revoke Gmail tokens)
- **sync reports an expired history cursor (HTTP 404)** — Run gmail-pp-cli sync --account <name> --full — Gmail expires historyId cursors after long gaps; a full resync rebuilds cleanly
- **cleanup apply refuses without a plan token** — Run cleanup plan first and pass its printed token to apply — applies never run unplanned
- **unsub run skips a sender** — Two common cases. (1) The sender only offers mailto: unsubscribe — it appears in unsub audit --mailto-only for manual handling; this tool never sends email. (2) The one-click URL host lives outside the sender's registrable domain (ESP-hosted): unsub plan lists these under third_party_hosts, and unsub run skips them unless invoked with --allow-third-party

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**googleworkspace/cli**](https://github.com/googleworkspace/cli) — Rust
- [**Gmail-MCP-Server**](https://github.com/GongRzhe/Gmail-MCP-Server) — TypeScript
- [**gmail-cleaner**](https://github.com/Gururagavendra/gmail-cleaner) — JavaScript
- [**gmail-declutter-extension**](https://github.com/InboxWhiz/gmail-declutter-extension) — TypeScript
- [**gmail-unsubscribe**](https://github.com/justjake/gmail-unsubscribe) — JavaScript
- [**gmail-multi-cli**](https://github.com/davidtkeane/gmail-multi-cli) — Python
- [**cmdg**](https://github.com/ThomasHabets/cmdg) — Go
- [**mcp-google-gmail**](https://github.com/cablate/mcp-google-gmail) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
