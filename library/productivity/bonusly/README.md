# Bonusly CLI

**The recognition analytics Bonusly reserves for admins -- rebuilt from data any employee can already read.**

Bonusly's own Analytics API and admin reports compute participation, spend, and recognition-equity insights, but they require an admin scope most employees don't have. This CLI mirrors exactly the data a regular employee CAN read -- the company feed, org chart, your own balance and redemptions -- into a local SQLite database, then computes that same category of insight offline: budget pacing, burn-rate forecasting, neglected-teammate detection, and company-values trends. No admin access required, and it works when you're offline.

## Install

The recommended path installs both the `bonusly-pp-cli` binary and the `pp-bonusly` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install bonusly
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install bonusly --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install bonusly --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install bonusly --agent claude-code
npx -y @mvanhorn/printing-press-library install bonusly --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/bonusly/cmd/bonusly-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/bonusly-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install bonusly --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-bonusly --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-bonusly --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install bonusly --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/bonusly-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `BONUSLY_API_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/bonusly/cmd/bonusly-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "bonusly": {
      "command": "bonusly-pp-mcp",
      "env": {
        "BONUSLY_API_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Bonusly authenticates with a Personal Access Token (PAT), not OAuth. Any employee can mint one from their own Settings -> Services page (not the admin-only Company -> Integrations -> API & Tokens page) -- pick only the scopes this CLI needs (user:read, recognition:read, recognition:write, rewards:read; admin scopes won't even be offered to a non-admin account). Tokens expire after up to 365 days with email reminders 30 and 7 days out. If self-serve token generation isn't available on your account, `bonusly-pp-cli auth login --chrome` authenticates from a captured browser session instead.

## Quick Start

```bash
# Health check -- works before you've set an auth token
bonusly-pp-cli doctor --dry-run

# Mirror the company feed, org chart, and department headcounts locally
bonusly-pp-cli sync --resources recognition,users,departments

# Browse the synced company feed filtered by hashtag
bonusly-pp-cli recognition feed --hashtag teamwork --agent

# Check your current giving/redeemable balance
bonusly-pp-cli balance --agent

# Find direct reports you haven't recognized in the last 30 days
bonusly-pp-cli recognition gap --manager me --days 30 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`recognition audit`** — See whether your team's recognition spend is on pace with its monthly budget, broken down by department -- the report Bonusly normally reserves for admins.

  _Reach for this when a team lead wants pacing/budget visibility without admin access, instead of manually tallying the web feed._

  ```bash
  bonusly-pp-cli recognition audit --dept engineering --agent
  ```
- **`recognition search-mine`** — Search your own given and received recognition offline, without scrolling the public company feed. Requires one live lookup to confirm your identity (no local fallback); not yet verified against a real API response in this build.

  _Use this for self-review prep or recalling why you were recognized -- the company-wide feed search returns everyone's posts, not just yours._

  ```bash
  bonusly-pp-cli recognition search-mine "migration project" --agent
  ```
- **`balance history`** — Track your giving-allowance burn rate over time and see forfeiture coming before the monthly reset, not after.

  _Check this a few days before month-end to see if points are about to be forfeited._

  ```bash
  bonusly-pp-cli balance history --agent
  ```
- **`recognition gap`** — Find direct reports you haven't recognized recently, without an admin's Participation Report. Requires a live lookup to resolve your manager identity and direct reports (no local fallback); not yet verified against a real API response in this build.

  _Use before 1:1s or sprint retros to catch teammates who've gone unrecognized._

  ```bash
  bonusly-pp-cli recognition gap --manager me --days 30 --agent
  ```
- **`recognition values`** — See which company-value hashtags are actually trending in a department, instead of manually tallying the feed.

  _Use this for a culture pulse-check across a team or the whole company._

  ```bash
  bonusly-pp-cli recognition values --dept engineering --agent
  ```
- **`redemptions forecast`** — Project your reward-redemption spend from your own history -- a simple trend line, not a black box.

  _Use this to sanity-check whether your redeemable balance will cover a reward you're eyeing._

  ```bash
  bonusly-pp-cli redemptions forecast --agent
  ```

## Recipes

### Narrow a noisy feed response to just the fields you need

```bash
bonusly-pp-cli recognition feed --hashtag teamwork --agent --select recognitions.giver.display_name,recognitions.receivers.display_name,recognitions.amount
```

The feed returns giver/receiver/hashtag/reason/amount per row; --select keeps only the fields an agent actually needs instead of the full nested payload.

### Check budget pacing before month-end

```bash
bonusly-pp-cli recognition audit --dept engineering --agent
```

Joins the synced feed against department headcount to show spend-vs-budget offline.

### Catch a neglected direct report before a 1:1

```bash
bonusly-pp-cli recognition gap --manager me --days 30 --agent
```

Flags direct reports you haven't recognized in the last N days by joining the org tree against your own giving history.

### Forecast when your monthly allowance will forfeit

```bash
bonusly-pp-cli balance history --agent
```

Diffs locally-snapshotted balance history to project burn rate against the monthly reset.

## Usage

Run `bonusly-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml` (your Bearer Personal Access Token), `data.db`, and other local CLI state |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `BONUSLY_CONFIG_DIR`, `BONUSLY_DATA_DIR`, `BONUSLY_STATE_DIR`, or `BONUSLY_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `BONUSLY_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export BONUSLY_HOME=/srv/bonusly
bonusly-pp-cli doctor
```

Under `BONUSLY_HOME=/srv/bonusly`, the four dirs resolve to `/srv/bonusly/config`, `/srv/bonusly/data`, `/srv/bonusly/state`, and `/srv/bonusly/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "bonusly": {
      "command": "bonusly-pp-mcp",
      "env": {
        "BONUSLY_HOME": "/srv/bonusly"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `BONUSLY_DATA_DIR` overrides an explicit `--home` for that kind. Use `BONUSLY_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `BONUSLY_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `bonusly-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

<!-- pp:hand-edit bonusly-remove-broken-commands — awards, groups, incentives,
     and meetings command groups removed below: no working endpoint could be
     found for any of them despite ~20 live path probes each. See
     .printing-press-patches/bonusly-remove-broken-commands.json. -->

### give

Give recognition to one or more colleagues using structured flags instead of composing the raw reason-string DSL by hand.

- **`bonusly-pp-cli give`** - Give recognition with `--to`, `--amount`, `--message`, and `--hashtag`. Synthesizes the `+N @mention message #hashtag` string Bonusly's API expects and wraps the same endpoint as `recognition create`.

### balance

Your points balance and lifetime stats

- **`bonusly-pp-cli balance`** - Your current giving/redeemable balance, monthly budget, and lifetime stats

### company

Company metadata

- **`bonusly-pp-cli company`** - Company metadata: name, locale, plan, feature flags, subscription state

### departments

Departments configured for your company, with headcounts

- **`bonusly-pp-cli departments list`** - List departments with per-department user counts
- **`bonusly-pp-cli departments users`** - List users belonging to a department (exact match)

### locations

Locations configured for your company, with headcounts

- **`bonusly-pp-cli locations list`** - List locations with per-location user counts
- **`bonusly-pp-cli locations users`** - List users belonging to a location (exact match)

### org

Org-chart traversal: top-level users, direct reports, manager chains, reporting trees

- **`bonusly-pp-cli org chain`** - Walk the manager chain upward from a user, closest-first
- **`bonusly-pp-cli org reports`** - List users who report directly to a given manager
- **`bonusly-pp-cli org top`** - List users with no manager (org-chart entry points)
- **`bonusly-pp-cli org tree`** - Walk the reporting tree downward from a user

### recognition

Give, browse, and manage recognition (bonuses)

- **`bonusly-pp-cli recognition create`** - Give recognition. reason must be the full mini-DSL string: '+N @mention #hashtag your message' -- the promoted 'give' command synthesizes this from structured flags; this raw endpoint is the low-level passthrough
- **`bonusly-pp-cli recognition delete`** - Delete (undo) a recognition you gave, within 24 hours of creation
- **`bonusly-pp-cli recognition feed`** - List/browse the company recognition feed with filters
- **`bonusly-pp-cli recognition get`** - Get a single recognition by id
- **`bonusly-pp-cli recognition given`** - List recognition given by a user
- **`bonusly-pp-cli recognition group-count`** - Resolve a group (department/location/team) and count how many recipients a post would reach
- **`bonusly-pp-cli recognition last-given`** - Get when you last gave recognition to each of a batch of users (max 20 ids)
- **`bonusly-pp-cli recognition list-types`** - List the recognition-type values accepted by feed filters (celebrations, awards, incentives, peer, external_recognition)
- **`bonusly-pp-cli recognition received`** - List recognition received by a user
- **`bonusly-pp-cli recognition update`** - Edit a recognition you gave, within 24 hours of creation. reason REPLACES the entire message -- include the +amount, @mentions, and #hashtag you want to keep

### redemptions

Your own reward redemptions

- **`bonusly-pp-cli redemptions get`** - Get a single reward redemption by id (your own, or any if you have rewards-admin)
- **`bonusly-pp-cli redemptions list-mine`** - List your own reward redemptions, newest first

### users

Your own profile and other users in your company

- **`bonusly-pp-cli users get`** - Resolve a single user by id, email, or display name
- **`bonusly-pp-cli users get-bulk`** - Bulk-fetch users by a list of user IDs
- **`bonusly-pp-cli users me`** - Get the authenticated user's own profile
- **`bonusly-pp-cli users search`** - Search users by name or email, with optional department/location filters


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`bonusly-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`bonusly-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`bonusly-pp-cli learnings list`** - Inspect taught rows
- **`bonusly-pp-cli learnings forget <query>`** - Undo a teach
- **`bonusly-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`bonusly-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`bonusly-pp-cli teach-pattern`** - Install a query/resource template up front
- **`bonusly-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `BONUSLY_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `bonusly-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
bonusly-pp-cli org top

# JSON for scripting and agents
bonusly-pp-cli org top --json

# Filter to specific fields
bonusly-pp-cli org top --json --select id,name,email

# Dry run — show the request without sending
bonusly-pp-cli give --to jane@example.com --amount 50 --message "great work" --dry-run

# Agent mode — JSON + compact + no prompts in one flag
bonusly-pp-cli org top --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `1` unexpected/internal error, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
bonusly-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `bonusly-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/bonusly-pp-cli/config.toml`; `--home`, `BONUSLY_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `BONUSLY_API_TOKEN` | per_call | No | Set to your Bonusly Personal Access Token. Required only if you have not saved a token with `bonusly-pp-cli auth set-token`. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `bonusly-pp-cli doctor` reports `agentcookie: detected` and `bonusly-pp-cli auth status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Known Gaps

This CLI was originally generated without access to a live Bonusly API credential, so its first cut of endpoint paths was inferred from documentation and pattern-matching rather than confirmed. It has since been live-tested end-to-end against a real authenticated session (`auth login --chrome`), and several of those original inferences turned out wrong in ways the no-auth probing heuristic (401 vs. 404 with no token) could not have caught, because Bonusly returns 401 on some genuinely nonexistent paths too. This section reflects what live testing actually found.

1. **Removed: `awards`, `groups`, `incentives`, `meetings`.** None of these command groups could be made to work. Live probing (~20 path variants each, using a real authenticated session) found no working endpoint for the awards catalog, custom/system user groups, or 1:1 meetings resources — every variant tried returned 404. Rather than ship commands that always fail, they were removed. If you find the real endpoints (e.g. by capturing your own browser's network traffic while using these Bonusly features), please open an issue or PR.
2. **Fixed: `balance`, `balance history`, `recognition audit`.** The originally-inferred `/users/points_balance` does not exist; the real data (`giving_balance`, `earning_balance`, `lifetime_earnings`) lives directly on `GET /users/me`. Bonusly does not appear to expose `monthly_budget`, `currency`, `exchange_rate`, `lifetime_given`, or `lifetime_redeemed` to a non-admin account via any endpoint found so far — those fields report as zero rather than a guessed value.
3. **Fixed: `org top`.** The real endpoint is `GET /users?top_level=true`, not `/users/top_level`.
4. **Fixed: `redemptions list-mine`.** The literal string `"me"` is not accepted on this sub-resource; the command now resolves the caller's real id via `/users/me` first, then calls `GET /users/{id}/redemptions`.
5. **Fixed: `recognition gap`.** `GET /users/me`'s response is wrapped in `{"success":...,"result":{...}}` — an unwrapped parse left the resolved manager id empty. Also fixed: the live shape of `GET /users/{id}/direct_reports` is `{"data":{"users":[...]}}`, not a bare array.
6. **`sync`'s own copy of the `org` and `redemptions` paths is still the old, broken one.** The dedicated `org top` / `redemptions list-mine` commands above are fixed and confirmed live; `bonusly-pp-cli sync` has an independent, not-yet-updated path table and will still fail on these two resources until that's addressed too.
7. **Remaining inferred paths**: most other endpoints are still inferred from documentation rather than individually live-confirmed. If a path is wrong, the affected command fails with a clear HTTP error (404/405) rather than returning silently-wrong data. Please report any 404/405 you hit.

**If you hit a 404/405 on any command:** run `bonusly-pp-cli doctor` to confirm auth is configured, then try the command with `--json` to see the raw error. If it's a path issue, capturing real browser traffic while using the equivalent Bonusly web UI feature is the most reliable way to find the correct endpoint — no-auth probing (401 vs. 404) is not a reliable signal, per the findings above.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `bonusly-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $BONUSLY_API_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Bad or missing access token** — Mint a personal token at bonus.ly -> Settings -> Services, then set BONUSLY_API_TOKEN (or run `bonusly-pp-cli auth set-token`).
- **403 Forbidden on an admin-shaped command** — That endpoint needs an *:administer scope your personal token can't carry -- expected for a non-admin account, not a bug.
- **429 rate limited** — Bonusly publishes no rate-limit numbers; the CLI's client applies adaptive backoff automatically -- just retry.
- **recognition gap or audit commands return empty** — Run `sync --resources recognition,users` first -- these commands read the local mirror, not the live API.
