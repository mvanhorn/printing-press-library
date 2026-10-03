# WheeLog CLI

**Find and compare recorded accessibility facts with gaps and conflicting reports visible.**

Search public WheeLog spots, inspect category-specific equipment questions, and compare a bounded shortlist. Save normalized public-place evidence for offline lookup, recheck triage, straight-line proximity and exact observation changes.

Learn more at [WheeLog](https://app.wheelog.com).

Created by [@zjsng](https://github.com/zjsng) (Jet Sng).

## Install

The recommended path installs both the `wheelog-pp-cli` binary and the `pp-wheelog` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install wheelog
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install wheelog --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install wheelog --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install wheelog --agent claude-code
npx -y @mvanhorn/printing-press-library install wheelog --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/wheelog/cmd/wheelog-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/wheelog-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install wheelog --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-wheelog --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-wheelog --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install wheelog --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/wheelog-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/wheelog/cmd/wheelog-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "wheelog": {
      "command": "wheelog-pp-mcp"
    }
  }
}
```

</details>

## Authentication

The supported public spot surface works anonymously over ordinary HTTPS. No account, API key, cookies or resident browser is needed.

## Quick Start

```bash
# Check the CLI setup without making source requests.
wheelog-pp-cli doctor --dry-run

# Choose an exact category and understand its question-ID range.
wheelog-pp-cli categories --agent

# Find original source spot IDs for a public facility keyword.
wheelog-pp-cli spots search 成田空港 --category toilet --limit 3 --agent

# Read aggregate equipment reports for a selected source spot.
wheelog-pp-cli spots inspect 166345 --agent --select id,name,questions

```

## Evidence contract

Results retain source IDs, Japanese names, public facility addresses, category question labels, aggregate positive/negative counts and canonical WheeLog URLs. An affirmative report is a contributor report, not a guarantee of accessibility. `reported_affirmative`, `reported_negative`, `conflicting`, `unreported`, `counts_missing`, `inapplicable`, `not_checked` and `unavailable` remain distinct. Use `categories` to choose exact question IDs; a restroom question does not apply to an elevator.

`record_created_at` and `record_updated_at` are source record metadata. `retrieved_at` dates this CLI's observation. Missing update times remain unknown. `--from`/`--to` select record dates in the inclusive `--timezone` calendar window (default `Asia/Tokyo`); output echoes the UTC request. They do not select individual contributor report dates or trip availability.

Search scans at most five source pages and returns at most 50 records. Detail expansion and comparison are capped at five spots; coverage states how many records were listed, checked or unavailable. `--require-question` triggers real detail reads and returns an assessment rather than filtering away gaps. Keywords use the source's matching rules; an empty bounded result is not proof that no accessible facility exists.

The public contract supplies aggregate equipment answers rather than typed measured widths or slopes, individual report dates, current opening conditions or accessible routes. Read supplementary notes on the canonical source page. Contributor profiles, raw narratives, photos, comments and personal TrackLogs are excluded before cache and output.

`auto` prefers a fresh source read and labels saved fallback; `live` requires source requests; `local` uses saved evidence. `shortlist list` is always local. `categories` computes the recorded source catalog and rejects live mode. Save up to 50 selected public spots, retaining only the latest two normalized observations per spot. Retrieval-only differences do not count as source changes. Straight-line distances cover only saved facilities and do not establish a wheelchair route; the supplied origin is not stored.

## Unique Features

These workflows combine verified public spot evidence with a bounded saved shortlist.

### Recorded evidence
- **`spots compare`** — Align requested source question IDs and report supporting, opposing, mixed, unknown and inapplicable evidence.

  _Choose this when comparing an explicit shortlist against source question evidence._

  ```bash
  wheelog-pp-cli spots compare 166345 166344 --require-question 102 --agent
  ```
- **`spots search`** — Expand a bounded keyword shortlist into actual detail evidence for requested questions.

  _Choose this when candidate search results need actual question counts before triage._

  ```bash
  wheelog-pp-cli spots search 成田空港 --category toilet --require-question 102 --limit 3 --agent
  ```

### Saved shortlist
- **`shortlist changes`** — Show exact changes between two allowlisted source observations for selected public places.

  _Choose this to inspect source evidence changes without asserting physical changes._

  ```bash
  wheelog-pp-cli shortlist changes --data-source local --agent
  ```
- **`shortlist list`** — Prioritize old, unknown or conflicting evidence in a saved shortlist with explicit reasons.

  _Choose this for saved-list maintenance and missing or conflicting question evidence._

  ```bash
  wheelog-pp-cli shortlist list --audit --require-question 102 --max-record-age 180d --agent
  ```
- **`shortlist list`** — Rank saved public facilities by straight-line distance from an explicit origin.

  _Choose this for offline proximity within the saved shortlist, with no route promise._

  ```bash
  wheelog-pp-cli shortlist list --origin 35.7742,140.3879 --radius-m 500 --category toilet --agent
  ```

## Recipes

### Dated restroom records

```bash
wheelog-pp-cli spots search 成田空港 --category toilet --from 2026-10-02 --to 2026-10-02 --timezone Asia/Tokyo --limit 3 --agent
```

Request the explicit Japanese record-date window; output echoes the UTC backend interval.

### Requested equipment comparison

```bash
wheelog-pp-cli spots compare 166345 166344 --require-question 102 --agent
```

Keep inapplicable, unreported, conflicting and unavailable evidence distinct.

### Small inspection output

```bash
wheelog-pp-cli spots inspect 166345 --agent --select id,name,questions
```

Return only selected public spot and question facts.

### Offline shortlist rechecks

```bash
wheelog-pp-cli shortlist list --audit --require-question 102 --data-source local --agent
```

Prioritize local evidence gaps without network requests.

## Usage

Run `wheelog-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `WHEELOG_CONFIG_DIR`, `WHEELOG_DATA_DIR`, `WHEELOG_STATE_DIR`, or `WHEELOG_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `WHEELOG_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export WHEELOG_HOME=/srv/wheelog
wheelog-pp-cli doctor
```

Under `WHEELOG_HOME=/srv/wheelog`, the four dirs resolve to `/srv/wheelog/config`, `/srv/wheelog/data`, `/srv/wheelog/state`, and `/srv/wheelog/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "wheelog": {
      "command": "wheelog-pp-mcp",
      "env": {
        "WHEELOG_HOME": "/srv/wheelog"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `WHEELOG_DATA_DIR` overrides an explicit `--home` for that kind. Use `WHEELOG_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `WHEELOG_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `wheelog-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### spots

Public crowdsourced facility accessibility evidence

- **`wheelog-pp-cli spots inspect`** - Inspect allowlisted public accessibility question reports for a source spot ID.
- **`wheelog-pp-cli spots search`** - Search public spot records by source keyword, category, and evidence record dates.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`wheelog-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`wheelog-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`wheelog-pp-cli learnings list`** - Inspect taught rows
- **`wheelog-pp-cli learnings forget <query>`** - Undo a teach
- **`wheelog-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`wheelog-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`wheelog-pp-cli teach-pattern`** - Install a query/resource template up front
- **`wheelog-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `WHEELOG_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once a writable operation opens the database with this version of `wheelog-pp-cli`, older binaries refuse it with a version error — upgrade the binary rather than downgrading. Saved-only WheeLog reads skip migrations and do not advance this schema stamp.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
wheelog-pp-cli spots search

# JSON for scripting and agents
wheelog-pp-cli spots search --json
# Filter to specific fields
wheelog-pp-cli spots search --json --select results.id,results.name,coverage

# Dry run — summarize the intended action without executing the command
wheelog-pp-cli spots search --dry-run

# Agent mode — JSON + compact + no prompts in one flag
wheelog-pp-cli spots search --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts; inputs use flags or positional arguments
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` summarizes the intended action without executing the command
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
wheelog-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `wheelog-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/wheelog-pp-cli/config.toml`; `--home`, `WHEELOG_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Use `spots search` to discover source spot IDs, or `shortlist list` to inspect saved membership

### API-specific
- **No keyword matches** — Use the original place name or a broader source keyword; an empty bounded result is not proof of no accessible facilities.
- **A question has zero or conflicting reports** — Inspect the retained positive/negative counts and use the canonical source page for further checking.
- **Source request is unavailable** — Use --data-source local for saved evidence or retry the bounded source request later.

## Discovery Signals

The public web application's anonymous search and detail requests were observed on 2026-10-03 and replayed over ordinary HTTPS. The supported source surface is POST-based read-only RPC with an explicit semantic success envelope. It requires no credentials, cookies or browser runtime. The CLI allowlists public-place fields before storage or output and stops with a contract error if the source envelope changes.

Saved-only commands read existing evidence without migrations, table creation or permission changes. A missing shortlist is empty. Saved path aliases are resolved to the guarded target; multiply linked database files are rejected. Windows checks file attributes for link counts; unavailable link-count metadata fails closed. A non-empty WAL/rollback journal or a database change during the read returns `cache_visibility_unavailable`; close other database writers and retry. Auto discovery and inspection try the source first and read saved fallback only after a source failure; a failed cache fallback remains an explicit error. Saved reads use a private snapshot copied from a verified open descriptor (at most 64 MiB); SQL never reopens the selected pathname, and temporary snapshots are removed on completion or error. Refreshing or saving observations still needs a writable database. Source saves, refreshes and removals bind to the same canonical target and verify selected-path identity and a single hard link before opening and around commits on one reserved connection. Writable source-cache paths containing `?` or `#` are rejected before opening; saved-only reads support those names through escaped private-snapshot URIs.

