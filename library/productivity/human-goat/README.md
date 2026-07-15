# Human-Goat CLI

**Hire real humans from the terminal — autonomous TaskRabbit checkout with a verified undo, plus Magic remote errands, in one agent-native binary.**

goat unifies two human networks behind a common task model: TaskRabbit for in-person local labor and Magic for remote errands. Its headline is hands-off checkout on TaskRabbit against the card on file — searched, ranked by honest all-in price and review quality, and booked with no prompt — made safe by a spend cap and a cancel command that verifies the cancellation actually landed.

## Install

The recommended path installs both the `human-goat-pp-cli` binary and the `pp-human-goat` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install human-goat
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install human-goat --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install human-goat --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install human-goat --agent claude-code
npx -y @mvanhorn/printing-press-library install human-goat --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/human-goat/cmd/human-goat-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/human-goat-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install human-goat --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-human-goat --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-human-goat --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install human-goat --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
human-goat-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/human-goat-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/human-goat/cmd/human-goat-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "human-goat": {
      "command": "human-goat-pp-mcp"
    }
  }
}
```

</details>

## Authentication

TaskRabbit auth is cookie replay from a logged-in Chrome session: run `human-goat-pp-cli auth login --chrome` to lift the session + XSRF-TOKEN cookies. Never a programmatic password login (the login form is reCAPTCHA-gated). Magic uses an x-api-key read from $MAGIC_API_KEY or ~/.magic/api_key.

## Quick Start

```bash
# check both backends and resolve the account metro before doing anything
human-goat-pp-cli doctor --dry-run

# see the task templates available in your metro
human-goat-pp-cli categories list --agent

# find the best-value available Tasker with all-in pricing
human-goat-pp-cli best "help moving" --on saturday --min-rating 4.9 --agent

# render the confirm summary without booking; drop --dry-run to check out for real
human-goat-pp-cli hire "help moving" --on saturday --min-rating 4.9 --max-total 200 --dry-run --lat 47.6062 --lng -122.3321

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Hands-off hiring
- **`hire`** — Say the job and the date; goat searches, ranks by review quality and honest all-in price, and checks out against the card on file with no prompt.

  _Reach for this when the user wants a task done, not a Tasker shortlist to review._

  ```bash
  human-goat-pp-cli hire "help moving" --on 2026-07-11 --min-rating 4.8 --max-total 200 --agent --lat 47.6062 --lng -122.3321
  ```
- **`cancel`** — Cancels a booking and confirms it landed by re-reading status, reporting whether it was inside the free window and any fee.

  _This is the undo that makes autonomous checkout tolerable; always available after a hire._

  ```bash
  human-goat-pp-cli cancel task_abc123 --agent
  ```
- **`hire`** — Refuses to check out when the computed all-in total exceeds a configurable ceiling, printing the total and the cap.

  _Use --max-total (or the config default) to cap autonomous spend before any booking is placed._

  ```bash
  human-goat-pp-cli hire movers --on saturday --min-rating 4.9 --max-total 150 --lat 47.6062 --lng -122.3321
  ```

### Honest pricing
- **`best`** — Folds TaskRabbit's hidden service (~15%) and trust-and-support (5-15%) fees into the displayed hourly rate, honoring the CA/MA service-fee-only rule.

  _Every price surface (search, compare, best, hire, spend) shows the real all-in rate, not the teaser._

  ```bash
  human-goat-pp-cli best "help moving" --on saturday --min-rating 4.9 --agent
  ```
- **`watch`** — Polls recommendations for a category and date (optionally a favorite or a rate ceiling) and alerts when a match opens.

  _Use when nothing good is available now and you want the first qualifying slot._

  ```bash
  human-goat-pp-cli watch movers --on saturday --max-rate 60
  ```

### One surface, two human networks
- **`dispatch`** — Routes a plain-language task to Magic (remote-doable) or TaskRabbit (in-person) by task shape, with a --via override.

  _Say what you want done and let goat pick the human network; force it with --via taskrabbit|magic._

  ```bash
  human-goat-pp-cli dispatch "call the dentist and reschedule my cleaning"
  ```
- **`spend`** — SQL over local booking, invoice, and Magic-task history by category, tasker, source, or month, using true effective all-in $/hr for TaskRabbit.

  _Answers where the money went across both human networks with fees folded in._

  ```bash
  human-goat-pp-cli spend --by source --agent
  ```
- **`status`** — One list of every in-flight task across TaskRabbit bookings and Magic requests, joined on the common task model and sorted by state.

  _Use for a cross-source view of everything in flight; use 'tasks list' for TR-only and 'track <id>' for one Magic request._

  ```bash
  human-goat-pp-cli status --open --agent
  ```

## Recipes

### Autonomous hire with a spend cap

```bash
human-goat-pp-cli hire "help moving" --on 2026-07-11 --min-rating 4.8 --max-total 200 --agent --lat 47.6062 --lng -122.3321
```

Searches, ranks by all-in price x reviews, checks out under the cap, and prints the booking id, Tasker, time, and total with no prompt.

### Undo a booking and verify

```bash
human-goat-pp-cli cancel task_abc123 --agent
```

Cancels, re-reads status, and reports cancelled plus whether it was inside the free window and any fee.

### Dispatch a remote errand

```bash
human-goat-pp-cli call 5209076052 "when does the jewelry store open"
```

Sends a phone-call task to Magic and returns a request id to track; the answer comes back in the conversation.

### Narrow a verbose payload for an agent

```bash
human-goat-pp-cli taskers favorites --agent --select items.name,items.all_in_rate,items.rating
```

Returns only the fields an agent needs from a deeply nested Tasker list instead of the full payload.

### Where did the money go

```bash
human-goat-pp-cli spend --by source --agent
```

Splits TaskRabbit vs Magic totals from the local store, with TaskRabbit rows using true all-in effective rate.

## Usage

Run `human-goat-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `HUMAN_GOAT_CONFIG_DIR`, `HUMAN_GOAT_DATA_DIR`, `HUMAN_GOAT_STATE_DIR`, or `HUMAN_GOAT_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `HUMAN_GOAT_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export HUMAN_GOAT_HOME=/srv/human-goat
human-goat-pp-cli doctor
```

Under `HUMAN_GOAT_HOME=/srv/human-goat`, the four dirs resolve to `/srv/human-goat/config`, `/srv/human-goat/data`, `/srv/human-goat/state`, and `/srv/human-goat/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "human-goat": {
      "command": "human-goat-pp-mcp",
      "env": {
        "HUMAN_GOAT_HOME": "/srv/human-goat"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `HUMAN_GOAT_DATA_DIR` overrides an explicit `--home` for that kind. Use `HUMAN_GOAT_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `HUMAN_GOAT_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `human-goat-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### account

The logged-in TaskRabbit client account profile

- **`human-goat-pp-cli account`** - Get the authenticated account profile

### categories

TaskRabbit task categories and templates for the account metro

- **`human-goat-pp-cli categories`** - List task templates for the account metro (title, category_name, category_id, default_template_id, top_category)

### invoices

Invoice and payment-history flags

- **`human-goat-pp-cli invoices`** - Whether the account has submitted invoices (payment-history presence flag)

### system

TaskRabbit account bootstrap, metro, and dashboard reachability

- **`human-goat-pp-cli system bootstrap`** - Account bootstrap — metro (id, name, country), payment_method_types, stream_api_key. Used by doctor to resolve the account metro.
- **`human-goat-pp-cli system dashboard-counts`** - Dashboard tab counts (open tasks, messages, etc.)

### taskers

Favorite, past, and suggested TaskRabbit Taskers

- **`human-goat-pp-cli taskers favorites`** - List your favorited Taskers (poster=client, rabbit=tasker)
- **`human-goat-pp-cli taskers past`** - List Taskers you have hired before
- **`human-goat-pp-cli taskers suggestions`** - Tasker suggestions for the account metro


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`human-goat-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`human-goat-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`human-goat-pp-cli learnings list`** - Inspect taught rows
- **`human-goat-pp-cli learnings forget <query>`** - Undo a teach
- **`human-goat-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`human-goat-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`human-goat-pp-cli teach-pattern`** - Install a query/resource template up front
- **`human-goat-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `HUMAN_GOAT_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `human-goat-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
human-goat-pp-cli account

# JSON for scripting and agents
human-goat-pp-cli account --json

# Filter to specific fields
human-goat-pp-cli account --json --select id,name,status

# Dry run — show the request without sending
human-goat-pp-cli account --dry-run

# Agent mode — JSON + compact + no prompts in one flag
human-goat-pp-cli account --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
human-goat-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `human-goat-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/human-goat/config.toml`; `--home`, `HUMAN_GOAT_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `human-goat-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **doctor reports TaskRabbit unauthenticated** — run `human-goat-pp-cli auth login --chrome` while logged into taskrabbit.com in Chrome
- **hire refuses with a spend-cap message** — raise --max-total or pick a cheaper Tasker; the printed all-in total exceeded the cap
- **Magic task shows ONGOING forever** — the human's answer arrives in the conversation, not result; read it with `track <id>` which surfaces the conversation tail
