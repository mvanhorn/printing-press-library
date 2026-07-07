# Function Health CLI

**Every Function Health feature, plus a local SQLite store with biomarker trends across every round you've ever drawn, plus a branded shareable PDF your doctor will actually read.**

Function Health gives you a great single-round dashboard but no way to see what's drifting across years of draws — and no way to share a clean, branded lab history with your physician. function-health-pp-cli pulls every round into a local SQLite store with FTS5, exposes cross-round trend analysis nobody else offers (`goat`, `biomarker trend`, `biomarkers trending`, `biomarkers oscillating`), and renders a Function-branded PDF with your name and date of birth that an MD can read in 30 seconds.

Learn more at [Function Health](https://member-app-mid.functionhealth.com).

Created by [@DamienStevens](https://github.com/DamienStevens) (Damien Stevens).

## Install

The recommended path installs both the `function-health-pp-cli` binary and the `pp-function-health` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install function-health
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install function-health --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install function-health --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install function-health --agent claude-code
npx -y @mvanhorn/printing-press-library install function-health --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/function-health-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-function-health --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-function-health --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-function-health skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-function-health. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/function-health-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `FUNCTION_HEALTH_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "function-health": {
      "command": "function-health-pp-mcp",
      "env": {
        "FUNCTION_HEALTH_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Function Health blocks Firebase REST email/password sign-in at the project level, so `function-health-pp-cli auth login` with `--email/--password` does not work and `auth login --chrome` is unreliable. The working path is `auth set-token`: log in to https://my.functionhealth.com in Chrome, open DevTools (Cmd+Option+I) -> Network, copy the `authorization: Bearer <token>` value from any `member-app-mid.functionhealth.com/api/v1/...` request, then run `function-health-pp-cli auth set-token <token>`. The token is stored in `~/.config/function-health-pp-cli/config.toml` (mode 0600) and lasts ~1 hour; re-run `auth set-token` on `HTTP 401`. Set `FUNCTION_HEALTH_TOKEN` to override for CI or one-shot use.

## Quick Start

```bash
# Paste a session token from DevTools (Network tab -> Authorization: Bearer ...). Function Health blocks password login.
function-health-pp-cli auth set-token <token>

# Pull every round, biomarker, recommendation, and clinician note into the local SQLite store with adaptive pacing.
function-health-pp-cli sync

# Show the single biomarker that's drifting fastest away from Function's optimal range right now.
function-health-pp-cli goat

# See every ApoB value you've ever drawn, with Function optimal range and delta — JSON for agents, sparkline in a terminal.
function-health-pp-cli biomarker trend ApoB --json

# Compose a Markdown context block (biomarker history + clinician notes + recommendations) and paste it straight into Claude or ChatGPT.
function-health-pp-cli bundle ApoB --window 3rounds | pbcopy

# Render a Function-branded multi-round PDF with your name and DOB — the file you actually email to your MD.
function-health-pp-cli export pdf-for-doctor --out ~/Downloads/function-history.pdf

```

## Known Gaps

- **Email/password `auth login` does not work.** Function Health blocks Firebase REST password sign-in at the project level. Use `auth set-token` (see [Authentication](#authentication)).
- **`auth login --chrome` is unreliable.** It drives [browser-use](https://github.com/browser-use/browser-use) against a sandboxed Chrome profile rather than your real one, so it frequently fails to capture the Firebase refresh token (leaving you without auto-refresh). Prefer `auth set-token`. A future pure-Go IndexedDB LevelDB reader would extract the token directly from Chrome's on-disk storage without a browser shell-out — tracked as a planned enhancement.
- **Tokens expire hourly.** Firebase id-tokens last ~1 hour and the CLI cannot silently refresh a token set via `auth set-token` (no refresh token is captured that way). Re-run `auth set-token` when you start seeing `HTTP 401`.
- **`bundle` clinician-notes are empty.** The `/notes` endpoint returns items without a stable ID, so `sync` cannot cache them locally; the notes section of `bundle` is therefore blank until the upstream shape gains an ID.

## Unique Features

These capabilities aren't available in any other tool for this API.

### Doctor-ready outputs
- **`export pdf-for-doctor`** — Render a Function-branded multi-round lab report PDF with your name and date of birth, suitable for emailing to your personal physician.

  _When an agent needs to produce something the user will hand to a third party, this is the durable artifact — not a JSON blob._

  ```bash
  function-health-pp-cli export pdf-for-doctor --out ~/Downloads/function-history.pdf
  ```

### Local-SQL trend analysis
- **`biomarker trend`** — Every value of a biomarker across every round you've ever drawn, with the delta from Function's optimal range, an ASCII sparkline in the terminal, and structured JSON for agents.

  _When the agent is asked 'how has X changed', this is the one query that answers it without re-fetching anything._

  ```bash
  function-health-pp-cli biomarker trend ApoB --json
  ```
- **`goat`** — Ranks every biomarker by distance-from-optimal multiplied by its slope-away-from-optimal across the last 3 rounds; returns the single most worrying biomarker right now with reasoning fields.

  _When the agent is asked 'what should I worry about', this is the one-call answer._

  ```bash
  function-health-pp-cli goat --agent
  ```
- **`biomarkers trending`** — Lists every biomarker whose slope across the last N rounds points away from Function's optimal range, sorted by magnitude.

  _Lets the agent prioritize a long biomarker list without burning the user's context window on every reading._

  ```bash
  function-health-pp-cli biomarkers trending --direction worse --last 3 --json
  ```
- **`category trend`** — For one of the ~13 categories, returns a per-round aggregate score (percent of biomarkers inside Function's optimal range) over time.

  _Rolls up 100+ biomarkers into a single trajectory per body system, so the agent can summarize organ-level changes._

  ```bash
  function-health-pp-cli category trend cardiovascular --json
  ```
- **`biomarkers oscillating`** — Biomarkers that crossed the optimal-range boundary at least twice in the last N rounds — flags instability separate from trend.

  _Distinguishes 'unstable measurement noise' from 'consistently bad' — important when the agent is reasoning about whether a single high reading matters._

  ```bash
  function-health-pp-cli biomarkers oscillating --rounds 4
  ```
- **`recommendations stale`** — Recommendations (supplements / foods) whose target biomarker is STILL outside Function's optimal range, joined by Quest code. The /recommendations endpoint has no issued-date, so staleness is by outcome not age; --min-rounds tightens to persistence and --group (supplements, foods_to_eat, foods_to_avoid) focuses a large set.

  _Closes the loop between guidance and outcome — the agent can ask 'did the last fix work?' instead of 'what is recommended now?'._

  ```bash
  function-health-pp-cli recommendations stale --json
  ```

### Agent-native plumbing
- **`bundle`** — Composes a single Markdown file with a biomarker's full history, every clinician note mentioning it (FTS5), Function's optimal range, and relevant recommendations — ready to paste into Claude or ChatGPT.

  _When the agent is being asked to draft a question for the user's clinician, this is the prefab context block._

  ```bash
  function-health-pp-cli bundle ApoB --window 3rounds
  ```

## Recipes


### Morning check-in

```bash
function-health-pp-cli sync && function-health-pp-cli goat
```

Cheap incremental sync, then the single most-worrying biomarker right now — both calls are local after the first sync.

### Doctor-ready PDF

```bash
# Full report
function-health-pp-cli export pdf-for-doctor --out ~/Downloads/function-$(date +%Y).pdf

# Only what's out of range, scoped to one section (combinable filters)
function-health-pp-cli export pdf-for-doctor --out ~/Downloads/heart.pdf --section Heart --out-of-range
```

Branded multi-round report with member name and DOB header, per-category sections, per-biomarker history. `--out-of-range` keeps only biomarkers outside Function's optimal range in their latest draw; `--section <name>` keeps only categories whose name contains the text (e.g. `Heart`, `Liver`, `Nutrients`); the two combine.

### Narrow JSON for agents

```bash
function-health-pp-cli biomarker trend ApoB --agent --select \\brounds.draw_date,rounds.value,rounds.status\\b
```

Strip the response down to the three fields an agent actually reasons about; `--select` accepts dotted paths into nested response objects.

### LLM context bundle

```bash
function-health-pp-cli bundle hs-CRP --window 3rounds | pbcopy
```

Compose a Markdown context block — history + clinician notes + recommendations — and paste into Claude or ChatGPT for a focused conversation.

### Stale-rec audit

```bash
function-health-pp-cli recommendations stale --json
```

Surface every recommendation issued at least one round ago whose target biomarker has not moved into Function's optimal range.

## Usage

Run `function-health-pp-cli --help` for the full command reference and flag list.

## Commands

### biological_age

Biological-age and BMI calculations Function derives from your panels.

- **`function-health-pp-cli biological-age bio-age`** - Your biological age vs. chronological age. 404 if not yet calculated.
- **`function-health-pp-cli biological-age bmi`** - BMI with the weight and height inputs Function recorded.

### biomarkers

Function Health's full biomarker catalog with names, units, reference ranges, and Function's "optimal" ranges.

- **`function-health-pp-cli biomarkers get`** - Get one biomarker's full result history across every round you've drawn, with values, units, status, and ranges.
- **`function-health-pp-cli biomarkers list`** - List every biomarker the platform knows about (across all categories).

### categories

The ~13 medical categories biomarkers are organized into (Heart, Hormones, Thyroid, Metabolic, Liver, Kidney, etc.).

- **`function-health-pp-cli categories`** - List every biomarker category with the biomarkers nested under it.

### notes

Clinician notes attached to your results.

- **`function-health-pp-cli notes`** - List all clinician notes (annotations on biomarkers / rounds).

### notifications

Change notifications (new results landing, biomarker direction changes).

- **`function-health-pp-cli notifications`** - Read all unread/read notifications about result changes.

### recommendations

Personalized health recommendations tied to your results.

- **`function-health-pp-cli recommendations`** - Per-category health guidance. May 404 if Function hasn't computed any yet.

### requisitions

Lab requisitions — pending (in-progress draws) and completed (rounds with results).

- **`function-health-pp-cli requisitions completed`** - Completed requisitions (your past test rounds, keyed by requisitionId).
- **`function-health-pp-cli requisitions pending`** - Currently in-progress requisitions (lab orders awaiting completion).

### results

Lab results — both the structured biomarker-level data and the raw requisition documents.

- **`function-health-pp-cli results list`** - Raw requisition / PDF result data (less useful for queries; see results-report for structured values).
- **`function-health-pp-cli results report`** - The structured lab-results report — every biomarker, every round, with value, unit, status, Quest reference range, and Function's optimal range. The headline endpoint for sync.

### schedules

Upcoming scheduled lab-draw appointments.

- **`function-health-pp-cli schedules`** - Upcoming scheduled lab visits you've booked.

### user

Your Function Health member profile (id, name, contact info, membership status).

- **`function-health-pp-cli user`** - Get the authenticated member profile. Used by doctor for reachability + auth check.

### visits

Individual lab-collection events within a test round.

- **`function-health-pp-cli visits`** - List every visit (draw event) — useful when a round has multiple draws.

### wearables

Wearable-device integrations (Apple Health, Garmin, Whoop, Oura, etc.).

- **`function-health-pp-cli wearables`** - List the wearable apps Function currently supports for integration.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
function-health-pp-cli biomarkers list

# JSON for scripting and agents
function-health-pp-cli biomarkers list --json

# Filter to specific fields
function-health-pp-cli biomarkers list --json --select id,name,status

# Dry run — show the request without sending
function-health-pp-cli biomarkers list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
function-health-pp-cli biomarkers list --agent
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
function-health-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/function-health-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `FUNCTION_HEALTH_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `function-health-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `function-health-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $FUNCTION_HEALTH_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Every call returns 401 `Not allowed to perform this operation.`** — Your session token expired (Firebase id-tokens last ~1 hour) or password login was attempted (blocked at the project level). Extract a fresh Bearer token from DevTools and run `function-health-pp-cli auth set-token <token>`.
- **`sync` finishes but `results list` returns nothing for a recent draw** — Run `function-health-pp-cli sync check` — Function may not have published the round yet; this command compares local round IDs against the live requisitions list and reports whether a new round landed.
- **`biomarker get <name>` reports `not found` for a biomarker visible in the Function web app** — Function reuses biomarker names across categories. Run `function-health-pp-cli biomarkers list --search <substring>` to find the exact name or UUID, or pass `--id <uuid>`.
- **`export pdf-for-doctor` renders an empty PDF** — PDF rendering pulls from the local SQLite — run `function-health-pp-cli sync` first if you haven't pulled any rounds yet.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**daveremy/function-health-mcp**](https://github.com/daveremy/function-health-mcp) — TypeScript
- [**bogini/function-health-exporter**](https://github.com/bogini/function-health-exporter) — TypeScript
- [**Greenband1/biomarker_chrome_extension**](https://github.com/Greenband1/biomarker_chrome_extension) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
