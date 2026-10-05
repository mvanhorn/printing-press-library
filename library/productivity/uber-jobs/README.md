# Uber Careers CLI

**Every public Uber job posting as one tracker-ready JSON feed with offline history, new-since diffs and a fallback when the careers site refuses**

Pull open Uber postings by country, team or keyword into the single envelope a job tracker already parses, keep a local history of what opened and closed, and screen descriptions for sponsorship or language disqualifiers with the matching sentence shown. When the careers site refuses a request, reads fall back to Uber's own Oracle candidate-experience API instead of returning nothing. That fallback has limits: description and job_category are null; --team, --sub-team, --contract-type, --work-pattern and the description filters have no fallback (the command exits 7 instead); neither do facets or the raw careers commands; and new does not diff saved searches that use a keyword.

Learn more at [Uber Careers](https://jobs.uber.com).

Created by [@qazmataz](https://github.com/qazmataz) (qazmataz).

## Install

The recommended path installs both the `uber-jobs-pp-cli` binary and the `pp-uber-jobs` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install uber-jobs
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install uber-jobs --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install uber-jobs --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install uber-jobs --agent claude-code
npx -y @mvanhorn/printing-press-library install uber-jobs --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/cmd/uber-jobs-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/uber-jobs-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install uber-jobs --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-uber-jobs --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-uber-jobs --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install uber-jobs --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/uber-jobs-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/cmd/uber-jobs-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "uber-jobs": {
      "command": "uber-jobs-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No account and no key are needed because the CLI reads public postings only and never signs in, applies or subscribes to alerts

## Quick Start

```bash
# Confirm the binary runs, without touching the network
uber-jobs-pp-cli doctor --dry-run

# The newest postings in one market, in the tracker's envelope
uber-jobs-pp-cli postings --country GBR --limit 20 --sort recent --json

# Mirror every open posting locally so history and offline commands work
uber-jobs-pp-cli sync --json

# Name a search once so new can diff it on every later run
uber-jobs-pp-cli save uk-strategy --country GBR --base-query strategy

# See what appeared or closed in that search since its last complete scan (the first run takes the baseline)
uber-jobs-pp-cli new uk-strategy

# Where Uber is hiring right now, with churn once history builds up
uber-jobs-pp-cli stats --by country

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local history that compounds
- **`new`** — See which Uber postings appeared in or closed from a saved search since its last complete scan

  _Reach for this on a recurring check of a market so an agent reports only what changed instead of re-listing everything. Create the name with save first (an unknown name exits 3); the first run only takes the baseline; when the careers site refuses, saved searches with a keyword are not diffed, and ones with a work pattern exit 7 (or, in auto mode, use the last local sync)_

  ```bash
  uber-jobs-pp-cli new uk-strategy --agent
  ```
- **`check`** — Report whether each known posting id is still open, closed, never seen or unknown, with the date it closed

  _Use this to keep tracker rows honest in one call instead of one lookup per id_

  ```bash
  uber-jobs-pp-cli check 302906 160425 --json
  ```
- **`stats`** — Count open, newly posted, opened and closed Uber postings per country, team, subteam or category

  _Use this for a market read-out of where Uber is hiring and how fast roles turn over; opened_30d and closed_30d stay null until the local store's first complete sync is 30 days old, and are always null on a live read. In auto mode it counts from the last local sync whatever its age (meta.note gives the date)_

  ```bash
  uber-jobs-pp-cli stats --by country --json
  ```
- **`save`** — Store a named set of filters so new can diff it on every later run

  _Use this once per recurring market check before running new_

  ```bash
  uber-jobs-pp-cli save uk-strategy --country GBR --base-query strategy
  ```
- **`searches`** — Show every saved search with its filters, baseline size and last advance, or delete one by name

  _Use this to audit what new will diff, or to prune stale searches_

  ```bash
  uber-jobs-pp-cli searches --json
  ```

### Screening without reading every posting
- **`screen`** — Keep or drop postings by phrases in their description and show the sentence each phrase matched

  _Use this to rule out postings that need sponsorship, relocation or a language, with the evidence sentence an agent can quote. The match is literal, so read the sentence (--verdict all) to judge negations. Check meta.source: under the Oracle fallback descriptions are null, every posting is unscreened and the default --verdict keep is empty. In auto mode it reads the last local sync whatever its age (meta.note gives the date)_

  ```bash
  uber-jobs-pp-cli screen --country NLD --posted-within 7d --exclude "fluent Dutch" --verdict all --agent
  ```

### Tracker-ready reads
- **`postings`** — List Uber postings for an ISO3 market, newest first, in the single envelope a job tracker parses

  _Use this as the default read for any market pull; it keeps the tracker's flags and field names stable_

  ```bash
  uber-jobs-pp-cli postings --country GBR --limit 100 --offset 0 --sort recent --json --data-source live
  ```
- **`get`** — Fetch one Uber posting by its id and exit not-found when the site no longer lists it

  _Use this when an agent has one id and needs the full posting or a reliable gone signal_

  ```bash
  uber-jobs-pp-cli get 302906 --json
  ```
- **`facets`** — List the countries, teams, subteams, contract types and work patterns the careers site currently offers

  _Use this before filtering so team and country values match the site exactly_

  ```bash
  uber-jobs-pp-cli facets --json
  ```

## Recipes

### Newest postings in a market

```bash
uber-jobs-pp-cli postings --country USA --limit 50 --sort recent --agent --select results.id,results.title,results.posted_on,results.location
```

Narrows a large envelope to the four fields an agent needs for a shortlist

### Postings from the last 7 days

```bash
uber-jobs-pp-cli postings --country GBR --posted-within 7d --sort recent --json
```

Answers "what is new this week" without a saved search; recency comes from the true posting date

### Weekly new-since check

```bash
uber-jobs-pp-cli new --all --json
```

Runs every saved search and returns what appeared or closed since each baseline

### Screen out language requirements

```bash
uber-jobs-pp-cli screen --country NLD --exclude "fluent Dutch" --verdict all --json
```

Gives every Netherlands posting a keep or drop verdict and quotes the sentence each phrase matched; the match is literal, so a negation such as "no fluent Dutch needed" also drops

### Are my tracked postings still open

```bash
uber-jobs-pp-cli check 302906 160425 --json
```

One call returns open, closed, never_seen or unknown for each id

### Build the local history

```bash
uber-jobs-pp-cli sync --json
```

Run it daily: closures get dates, and check, stats and screen answer from the local store

## Usage

Run `uber-jobs-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state: the machine-wide request gate, `refusals.tsv`, `refused-<host>.json` refusal latches, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `UBER_JOBS_CONFIG_DIR`, `UBER_JOBS_DATA_DIR`, `UBER_JOBS_STATE_DIR`, or `UBER_JOBS_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `UBER_JOBS_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export UBER_JOBS_HOME=/srv/uber-jobs
uber-jobs-pp-cli doctor
```

Under `UBER_JOBS_HOME=/srv/uber-jobs`, the four dirs resolve to `/srv/uber-jobs/config`, `/srv/uber-jobs/data`, `/srv/uber-jobs/state`, and `/srv/uber-jobs/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "uber-jobs": {
      "command": "uber-jobs-pp-mcp",
      "env": {
        "UBER_JOBS_HOME": "/srv/uber-jobs"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `UBER_JOBS_DATA_DIR` overrides an explicit `--home` for that kind. Use `UBER_JOBS_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `UBER_JOBS_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `uber-jobs-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### careers

Raw Uber careers search and id lookup, faithful to the site API wire keys

- **`uber-jobs-pp-cli careers lookup`** - Look up Uber careers postings by id through the site's batch lookup; unknown ids are silently dropped
- **`uber-jobs-pp-cli careers search`** - Search Uber careers postings with the site's own query keys and page numbering

### sync

- **`uber-jobs-pp-cli sync --json`** - Mirror every open posting into the local store with first_seen, last_seen and closures, so `check`, `stats` and `screen` answer offline. A fallback sync (meta.source oracle-ce) keeps stored descriptions but marks no closures and does not count as a complete sync


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`uber-jobs-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`uber-jobs-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`uber-jobs-pp-cli learnings list`** - Inspect taught rows
- **`uber-jobs-pp-cli learnings forget <query> --all`** - Undo every teach for a query (or pass `--resource`/`--action` to forget one)
- **`uber-jobs-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`uber-jobs-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`uber-jobs-pp-cli teach-pattern`** - Install a query/resource template up front
- **`uber-jobs-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `UBER_JOBS_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `uber-jobs-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
uber-jobs-pp-cli careers search --countries Germany

# JSON for scripting and agents
uber-jobs-pp-cli careers search --countries Germany --json
# Filter to specific fields
uber-jobs-pp-cli careers search --json --select Id,Reference,Title

# Dry run — show the request without sending
uber-jobs-pp-cli careers search --dry-run

# Agent mode — JSON + compact + no prompts in one flag
uber-jobs-pp-cli careers search --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Never writes to Uber** - read-only toward the site; `sync`, `save`, `searches --delete` and `new` write only the CLI's local store
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `1` other local failure (local store, `doctor --fail-on`), `2` usage error, `3` not found, `5` API or content error, `6` DNS or transport failure, `7` refused (403, 429, or a bot challenge; never retried, and nothing more is sent to that host until 00:00 UTC), `10` config error.

## Health Check

```bash
uber-jobs-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `uber-jobs-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/uber-jobs-pp-cli/config.toml`; `--home`, `UBER_JOBS_HOME`, and per-kind env vars can relocate it.

Static request headers configured under `headers` apply only to the raw `careers` commands and `doctor`; the posting commands always send their fixed identity.

## Troubleshooting
**Not found errors (exit code 3)**
- `get`: the posting id is no longer listed (check it with `check <id>`)
- `new` or `searches --delete`: no saved search by that name (list them with `searches`)

### API-specific
- **Exit code 7 with a careers-site refusal message** — Neither the careers site nor the Oracle fallback answered; the error text says which refused. Nothing more is sent to that host until 00:00 UTC, so rerun after then, or use --data-source local to read the last sync
- **The DNS or transport exit code from any command** — Check your resolver or VPN split DNS, then run uber-jobs-pp-cli doctor
- **new reports baseline_advanced false** — Read that row's note for the cause. Run uber-jobs-pp-cli sync --json only when it names a missing or stale local sync; after a refusal, wait until 00:00 UTC, since a fallback sync never counts as a complete one
- **A posting looks missing right after it went live** — Search replies are edge-cached for hours; new and check tolerate the lag, so rerun after the cache refreshes

## Known Limitations

- **The careers site's own search indexes disagree.** The unfiltered list, keyword search, and the `--team`, `--sub-team` and `--contract-type` filters can each include or miss a few postings the others do not. In a check on 5 October 2026, a keyword search returned 6 postings the unfiltered list lacked, the Full time filter missed 5 Full-time postings, and a team plus sub-team filter missed 2 postings that carry exactly those labels. So a live filtered read can differ slightly from `--data-source local` after a `sync`, and a whole-corpus `sync` can close a posting that a later keyword `sync` reopens. The CLI reports what the site returns; it does not reconcile the indexes.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**freehire**](https://github.com/strelov1/freehire) — Go (816 stars)
- [**ats-scrapers**](https://github.com/kalil0321/ats-scrapers) — Python (168 stars)
- [**headstart**](https://github.com/sarthakjain004/headstart) — Python
- [**fetchaller-mcp**](https://github.com/Averyy/fetchaller-mcp) — Python
- [**openings-mcp**](https://github.com/amikai/openings-mcp) — Go
- [**JobSpy**](https://github.com/speedyapply/JobSpy) — Python
- [**jobradar**](https://github.com/adityasingh2400/jobradar) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
