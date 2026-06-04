# Meta Ads CLI

**The first agent-native Meta Ads CLI with local SQLite history and compound queries the live API cannot answer.**

Read-only insights CLI for Meta Marketing API focused on creative-fatigue detection, audience overlap analysis, and attribution forensics. Daily insights land in a local SQLite store so commands like `fatigue`, `decay`, `learning`, and `reconcile` can join across history. Single META_ACCESS_TOKEN env var with `ads_read` scope.

Learn more at [Meta Ads](https://developers.facebook.com/docs/marketing-api).

Created by [@sdhilip200](https://github.com/sdhilip200) (Dhilip Subramanian).

## Install

The recommended path installs both the `meta-ads-pp-cli` binary and the `pp-meta-ads` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install meta-ads
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install meta-ads --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install meta-ads --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install meta-ads --agent claude-code
npx -y @mvanhorn/printing-press-library install meta-ads --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/meta-ads/cmd/meta-ads-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/meta-ads-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-meta-ads --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-meta-ads --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install meta-ads --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/meta-ads-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `META_ACCESS_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/meta-ads/cmd/meta-ads-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "meta-ads": {
      "command": "meta-ads-pp-mcp",
      "env": {
        "META_ACCESS_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Authenticate with a Meta access token in the META_ACCESS_TOKEN env var. Generate from Graph API Explorer (User Access Token, 1-2h lifespan) for ad-hoc use, or Business Manager System Users (no expiry) for repeat use. Grant ads_read scope only — never ads_management or any write permission. Token discovery via /me/adaccounts lists every account accessible to the token; no account ID configuration required.

## Quick Start

```bash
# Verify the token and config are wired correctly before any API calls.
meta-ads-pp-cli doctor --dry-run

# List every accessible ad account (returns paged data array with id, name, currency, etc.).
meta-ads-pp-cli me --agent

# List campaigns for the test account. Run live; --data-source auto caches results in the local SQLite store for fatigue/decay/learning/reconcile.
meta-ads-pp-cli campaigns act_4327210487520472 --agent

# Surface ads with creative-fatigue signals across the last 14 days.
meta-ads-pp-cli fatigue --account act_4327210487520472 --window 14d --agent

# Group every ad by effective_status, flagging configured-vs-effective mismatches.
meta-ads-pp-cli inventory --account act_4327210487520472 --by effective_status --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`fatigue`** — Detect ads whose CPM is drifting up, frequency is climbing, and CTR is falling across a configurable window.

  _Use this to decide which ads to retire. Spending on a fatigued creative is the highest-leverage waste in any account._

  ```bash
  meta-ads-pp-cli fatigue --campaign 23847265 --window 14d --agent
  ```
- **`decay`** — Compare a creative's first-impression CTR against its current CTR, with projected dead-date.

  _Use for retire/refresh decisions on a single creative. The slope and projected dead-date are the deciding factor._

  ```bash
  meta-ads-pp-cli decay --creative-id 120208734567 --agent
  ```
- **`overlap`** — Pairwise overlap percentages across custom audiences, with cannibalization recommendations.

  _Use when ad strategy is mature and ROAS feels diluted across similar audiences. Surfaces consolidate-or-exclude decisions._

  ```bash
  meta-ads-pp-cli overlap --audience 23847001 --audience 23847002 --audience 23847003 --agent
  ```
- **`learning`** — Surface adsets stuck in algorithmic learning >N days with root-cause hint (budget too low, audience too narrow, events too sparse).

  _Use when ROAS collapses across multiple adsets simultaneously. Stuck-in-learning adsets eat budget without optimizing._

  ```bash
  meta-ads-pp-cli learning --account act_4327210487520472 --min-days 7 --agent
  ```

### Forensic queries
- **`reconcile`** — Per-day diff between Meta-reported account spend and sum-of-insights spend; flags days where attribution drift exceeds threshold.

  _Use monthly for attribution audits. Surfaces specific days where Meta and insights disagree, usually delayed conversion attribution._

  ```bash
  meta-ads-pp-cli reconcile --account act_4327210487520472 --since 30d --threshold 5 --agent
  ```
- **`bottleneck`** — Highest-spend adsets with worst ROAS, ranked with 'why' column joining learning state and effective status.

  _Use weekly to decide what to pause. Highest-leverage targets surface first._

  ```bash
  meta-ads-pp-cli bottleneck --account act_4327210487520472 --limit 10 --agent
  ```

### Account hygiene
- **`stale`** — Active ads with zero impressions in N days.

  _Use quarterly for account hygiene. Active ads with zero impressions are usually misconfigured or post-deletion zombies._

  ```bash
  meta-ads-pp-cli stale --days 90 --agent
  ```
- **`inventory`** — Group every ad in an account by effective_status, surface ads where configured status='ACTIVE' but effective_status='WITH_ISSUES' or 'DISAPPROVED'.

  _Use first thing every morning. DISAPPROVED-but-still-ACTIVE ads cost real spend without delivering._

  ```bash
  meta-ads-pp-cli inventory --account act_4327210487520472 --by effective_status --agent
  ```

## Recipes

### Triage creative fatigue across a campaign

```bash
meta-ads-pp-cli fatigue --campaign 23847265 --window 14d --agent --select ad_id,ad_name,cpm_slope,frequency_now,ctr_slope,verdict
```

Returns one row per ad with CPM slope, current frequency, CTR slope, and a retire/keep verdict. Narrow output by selecting only the fatigue-relevant fields to keep agent context bounded.

### Find audience cannibalization risk

```bash
meta-ads-pp-cli overlap --audience 23847001 --audience 23847002 --audience 23847003 --agent
```

Returns pairwise overlap percentages and a recommendation per pair. Anything above 30% is a candidate for consolidation or exclusion.

### Spot stuck-in-learning adsets

```bash
meta-ads-pp-cli learning --account act_4327210487520472 --min-days 7 --agent
```

Surfaces adsets where learning_stage_info has been LEARNING for >7 days. The 'why' column hints at budget/audience/event sparsity.

### Daily inventory roll-up first thing

```bash
meta-ads-pp-cli inventory --account act_4327210487520472 --by effective_status --agent
```

Groups every ad by effective_status. The DISAPPROVED-but-configured-ACTIVE class is the highest-leverage daily finding.

### Monthly spend reconciliation

```bash
meta-ads-pp-cli reconcile --account act_4327210487520472 --since 30d --threshold 5 --agent
```

Per-day diff between Meta-reported account spend and sum-of-insights spend over the last 30 days. Flags days where drift exceeds 5%.

## Usage

Run `meta-ads-pp-cli --help` for the full command reference and flag list.

## Commands

### adcreatives

Manage adcreatives

- **`meta-ads-pp-cli adcreatives <adId>`** - Get creatives attached to an ad

### ads

Ad-level read operations

- **`meta-ads-pp-cli ads <adAccountId>`** - List ads in an ad account

### adsets

Ad set-level read operations

- **`meta-ads-pp-cli adsets <adAccountId>`** - Returns ad sets with targeting summary, budget, optimization goal, and
bid strategy. Includes learning_phase metadata when adsets are still
exiting the algorithm's learning window.

### campaigns

Campaign-level read operations

- **`meta-ads-pp-cli campaigns <adAccountId>`** - Returns campaigns with status, objective, and budget metadata. Use
effective_status (not status) for the live delivery state - status is
what you set; effective_status is what Meta is actually doing.

### customaudiences

Manage customaudiences

- **`meta-ads-pp-cli customaudiences <adAccountId>`** - List custom audiences in an ad account

### delivery-estimate

Manage delivery estimate

- **`meta-ads-pp-cli delivery-estimate <adAccountId>`** - Get reach/impressions delivery estimate

### insights

Performance and delivery insights (the creative-fatigue observatory)

- **`meta-ads-pp-cli insights get-account`** - Returns aggregated insights across the entire ad account. Requires
either time_range or date_preset - there is no default time window.
Breakdowns are mutually exclusive in groups (age+gender works,
age+placement does not).
- **`meta-ads-pp-cli insights get-ad`** - Ad-level insights with frequency, CPM, CTR over time. With
time_increment=1 and a date_preset like last_30d, this returns the
daily series that powers creative-fatigue detection.
- **`meta-ads-pp-cli insights get-ad-set`** - Get ad set-level insights
- **`meta-ads-pp-cli insights get-campaign`** - Get campaign-level insights

### me

Manage me

- **`meta-ads-pp-cli me`** - Returns every ad account the access token has been granted access to.
Use this on first run to discover which accounts you can query.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
meta-ads-pp-cli ads mock-value

# JSON for scripting and agents
meta-ads-pp-cli ads mock-value --json

# Filter to specific fields
meta-ads-pp-cli ads mock-value --json --select id,name,status

# Dry run — show the request without sending
meta-ads-pp-cli ads mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
meta-ads-pp-cli ads mock-value --agent
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
meta-ads-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/meta-marketing-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `META_ACCESS_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `meta-ads-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `meta-ads-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $META_ACCESS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Empty ad_accounts list with a valid token** — Business Manager assignment missing. Open Business Settings → Ad Accounts → Add People and assign your user (or System User) with View permission.
- **OAuth error code 190 'session has expired'** — Token expired. Regenerate a User Access Token in Graph API Explorer, or generate a System User token in Business Manager for long-lived use.
- **OAuth error code 200 'API access deactivated'** — The Meta App backing this token is deactivated. Reactivate at developers.facebook.com/apps/, or generate a token from a different active app.
- **Empty insights despite valid token and active ads** — Insights require time_range or date_preset; there is no default window. Pass --date-preset last_7d or --time-range {"since":"2026-01-01","until":"2026-01-31"}.
- **Breakdown returns an error** — Some breakdown combinations are mutually exclusive. age+gender works; age+placement does not. Use one breakdown group at a time, or split into multiple calls.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**facebook-business-sdk**](https://github.com/facebook/facebook-python-business-sdk) — Python (1500 stars)
- [**facebook-nodejs-business-sdk**](https://github.com/facebook/facebook-nodejs-business-sdk) — JavaScript (600 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
