# YesWeHack CLI

**Every YesWeHack researcher feature, plus an offline SQLite-backed cockpit for scope cartography, drift detection, draft reports, and hacktivity learning that no Burp/Caido extension can match.**

yeswehack-pp-cli is the researcher-side cockpit for the YesWeHack bug bounty platform. It syncs every program you can see, every scope, every hacktivity disclosure into a local SQLite store so an agent can answer 'what should I work on', 'has this been reported', and 'what is in scope here' in milliseconds, offline. Submit and draft commands are guard-railed by design - the goal is better reports, not more reports.

Learn more at [YesWeHack](https://api.yeswehack.com).

## Install

The recommended path installs both the `yeswehack-pp-cli` binary and the `pp-yeswehack` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install yeswehack
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install yeswehack --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/yeswehack-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-yeswehack --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-yeswehack --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-yeswehack skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-yeswehack. The skill defines how its required CLI can be installed.
```

## Authentication

Authentication is JWT-based and tied to your logged-in browser session. Run `yeswehack-pp-cli auth login --chrome` and the CLI reads the access_token from your Chrome profile's localStorage - no copy-paste from DevTools. The JWT refreshes automatically against the OAuth2 token endpoint when it expires. YesWeHack's Personal Access Tokens are gated to manager-tier accounts; the CLI does not support them for the researcher surface.

## Quick Start

```bash
# Pulls your JWT from Chrome's localStorage. No DevTools needed.
yeswehack-pp-cli auth login --chrome


# Builds the local store: programs, scopes, hacktivity, user reports, business units.
yeswehack-pp-cli sync


# The single command that says 'here is your weekend slate' - scope drift, reports needing reply, trending CWEs in your specialty.
yeswehack-pp-cli triage weekend --hours 6 --json


# What changed in your invited programs' scope this week.
yeswehack-pp-cli programs scope-drift --since-days 7


# Before you draft anything, see if it's already disclosed. Exit code 2 if a high-confidence collision exists.
yeswehack-pp-cli report dedupe --title 'SQLi in /api/users/{id}' --asset api.example.com --cwe CWE-89

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`programs scope-drift`** — See what changed in any program's scope this week — assets added, removed, or modified, with first-seen dates.

  _When an agent triages where to spend the hunter's week, drift is the highest-signal source of fresh attack surface. Pick this over a generic program list when the user has already chosen programs and wants to know what changed._

  ```bash
  yeswehack-pp-cli programs scope-drift --since-days 7 --json
  ```
- **`scopes overlap`** — Surface assets (host or wildcard) that appear in two or more of your invited programs, ranked by best payout.

  _When the agent finds a candidate finding on an asset, this answers 'which program pays the most for this asset' before drafting the report._

  ```bash
  yeswehack-pp-cli scopes overlap --min-programs 2 --json
  ```
- **`triage weekend`** — Ranked plan for a short hunting session - newly added scope, reports needing your response, and trending CWEs in your specialty.

  _Picks the right starting move when the hunter (or their agent) has limited time and needs a confidence-weighted plan, not a feed._

  ```bash
  yeswehack-pp-cli triage weekend --hours 6 --json
  ```
- **`programs fit`** — Rank invited and public programs by how well your historical CWE specialties match each program's hacktivity payout pattern.

  _Answers 'which program am I most likely to land on this week' before time is spent on scope reading or report drafting._

  ```bash
  yeswehack-pp-cli programs fit --specialty xss,ssrf,idor --json
  ```
- **`events calendar`** — Chronological view of platform events, payout deadlines, and CTFs gating private invites - filtered to programs you are invited to.

  _Surfaces time-bound opportunities (renewal bumps, CTF gates) the hunter would otherwise miss until after the fact._

  ```bash
  yeswehack-pp-cli events calendar --mine --json
  ```

### Anti-spam guard-rails
- **`report dedupe`** — FTS5 search over the public hacktivity feed plus your own reports for title, asset, or CWE overlap — exits 2 if a high-confidence collision exists.

  _Aligns with the YesWeHack Platform Code of Conduct's anti-spam rule. Before an agent drafts a report, this answers 'has someone already filed this' deterministically._

  ```bash
  yeswehack-pp-cli report dedupe --title 'SQLi in /api/users/{id}' --asset api.example.com --cwe CWE-89 --json
  ```
- **`report cvss-check`** — Parse a CVSS 3.1 vector, recompute its base score, and flag impossible combinations against report steps text - rule-based, no LLM.

  _Catches CVSS misrepresentations before the report is filed - the kind of mistake that loses credibility with triagers._

  ```bash
  yeswehack-pp-cli report cvss-check 'CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H' --steps draft.md --json
  ```
- **`report draft`** — Create a markdown draft pre-filled with the program's reward grid, accepted severity levels, and an allowed asset picker from local scopes - no network call.

  _Gives an agent a deterministic shape for a report instead of letting it fabricate the structure. Quality multiplier per the Platform CoC._

  ```bash
  yeswehack-pp-cli report draft yes-we-hack --output ./my-draft.md
  ```
- **`report submit`** — Submit a drafted report after dry-run preview, in-scope validation, and automatic pre-submit dedupe. Requires --confirm.

  _Lets an agent close the loop on submission without violating the platform's anti-AI-slop policy. No batch flag, no template-flood._

  ```bash
  yeswehack-pp-cli report submit ./my-draft.md --confirm
  ```

### Agent-native plumbing
- **`hacktivity trends`** — Histogram of disclosed report categories and average bounty for one program over a time window.

  _Calibrates severity expectations and report-style for a target program before the agent starts hunting it._

  ```bash
  yeswehack-pp-cli hacktivity trends gojek --since-days 90 --json
  ```
- **`hacktivity learn`** — Filtered slice of disclosed reports for a program and CWE - top N by bounty, with severity and writeup links, in pipe-friendly JSON.

  _Lets the agent calibrate from prior art before the hunter writes a single line - turning hacktivity into a learning surface, not just a feed._

  ```bash
  yeswehack-pp-cli hacktivity learn --program gojek --cwe CWE-89 --since-days 90 --json | claude 'summarize what worked'
  ```

## Usage

Run `yeswehack-pp-cli --help` for the full command reference and flag list.

## Commands

### business_units

Customer organizations that run programs

- **`yeswehack-pp-cli business_units list`** - List business units visible to the user

### events

Platform events (CTFs, dojos, live sessions)

- **`yeswehack-pp-cli events list`** - List YesWeHack events

### hacktivity

Public disclosed reports feed (the platform's learning surface)

- **`yeswehack-pp-cli hacktivity by_hunter`** - List a hunter's disclosed reports
- **`yeswehack-pp-cli hacktivity list`** - List recently disclosed reports across all public programs

### hunters

Researcher profiles (other hunters on the platform)

- **`yeswehack-pp-cli hunters get`** - Get a hunter's public profile (points, rank, impact, achievements)
- **`yeswehack-pp-cli hunters list_achievements`** - List a hunter's earned achievement badges

### programs

Bug bounty programs (public and private the user is invited to)

- **`yeswehack-pp-cli programs get`** - Get a program's full detail (rules, reward grid, scope counts, BU, etc.)
- **`yeswehack-pp-cli programs list`** - List bug bounty programs the user can see
- **`yeswehack-pp-cli programs list_scopes`** - List the in-scope and out-of-scope assets for a program

### ranking

Global researcher leaderboard

- **`yeswehack-pp-cli ranking list`** - Top hunters by points

### taxonomies

Reference data used by the platform (vulnerability parts, countries, profile URL types)

- **`yeswehack-pp-cli taxonomies list_countries`** - Country reference list (codes, names)
- **`yeswehack-pp-cli taxonomies list_profile_url_types`** - Allowed profile URL types (twitter, github, linkedin, etc.)
- **`yeswehack-pp-cli taxonomies list_vulnerable_parts`** - List vulnerability parts (CWE-like taxonomy used when filing reports)

### user

Authenticated user account, reports, invitations, email aliases

- **`yeswehack-pp-cli user get_self`** - Get the authenticated user
- **`yeswehack-pp-cli user list_email_aliases`** - List the authenticated user's email aliases (per-program forwarding addresses)
- **`yeswehack-pp-cli user list_invitations`** - List the authenticated user's program invitations
- **`yeswehack-pp-cli user list_reports`** - List reports the authenticated user has submitted


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
yeswehack-pp-cli business_units

# JSON for scripting and agents
yeswehack-pp-cli business_units --json

# Filter to specific fields
yeswehack-pp-cli business_units --json --select id,name,status

# Dry run — show the request without sending
yeswehack-pp-cli business_units --dry-run

# Agent mode — JSON + compact + no prompts in one flag
yeswehack-pp-cli business_units --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-yeswehack -g
```

Then invoke `/pp-yeswehack <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add yeswehack yeswehack-pp-mcp -e YESWEHACK_JWT=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/yeswehack-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `YESWEHACK_JWT` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "yeswehack": {
      "command": "yeswehack-pp-mcp",
      "env": {
        "YESWEHACK_JWT": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
yeswehack-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/yeswehack-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `YESWEHACK_JWT` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `yeswehack-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $YESWEHACK_JWT`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **`auth login --chrome` says 'access_token not found in localStorage'** — Open yeswehack.com in Chrome and sign in (or refresh the tab). The token lives in localStorage under the key `access_token`; the CLI reads it from the Chrome profile after login.
- **`401 unauthorized` on a private-program endpoint** — JWT expired - run `yeswehack-pp-cli auth refresh` to re-pull from Chrome, or sign in to yeswehack.com again.
- **`programs scopes <slug>` returns 401 for a public program** — Scope listing requires JWT even for public programs. Run `auth login --chrome` first.
- **`report submit` refuses with 'asset not in scope'** — Run `programs scopes <slug>` to verify the target asset, or `scopes find <pattern>` to find the program where the asset is in scope.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**YesWeBurp**](https://github.com/yeswehack/YesWeBurp) — Kotlin (108 stars)
- [**yeswecaido**](https://github.com/yeswehack/yeswecaido) — TypeScript (26 stars)
- [**ywh2bugtracker**](https://github.com/yeswehack/ywh2bugtracker) — Python (21 stars)
- [**yeswehack-mcp**](https://github.com/sebastianolaru3008/yeswehack-mcp) — Python
- [**ywh_program_selector**](https://github.com/jdouliez/ywh_program_selector) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
