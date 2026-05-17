# Higgsfield CLI

**Every Higgsfield workflow with a local memory: batch model fanouts, Soul library search, and credit guards no other Higgsfield tool has.**

This CLI mirrors the official Higgsfield CLI surface (auth, generate, soul-id, marketing-studio, product-photoshoot, marketplace-cards, upload, account, workspace) and adds the local SQLite store that powers fanout, soul-ids search, credit guards, and prompt history search. Single binary, agent-native JSON-first, MCP-exposed via the runtime Cobra-tree mirror.

Printed by [@kizillion](https://github.com/kizillion) (kizillion).

## Install

The recommended path installs both the `higgsfield-pp-cli` binary and the `pp-higgsfield` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press install higgsfield
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install higgsfield --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press install higgsfield --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press install higgsfield --agent claude-code
npx -y @mvanhorn/printing-press install higgsfield --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/higgsfield-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-higgsfield --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-higgsfield --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-higgsfield skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-higgsfield. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/higgsfield-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `HIGGSFIELD_JWT` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "higgsfield": {
      "command": "higgsfield-pp-mcp",
      "env": {
        "HIGGSFIELD_JWT": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Two auth paths. Web backend (default, recommended for most 2026 models): set HIGGSFIELD_JWT from your logged-in Higgsfield CLI's credentials file and HIGGSFIELD_ENABLE_WEB_BACKEND=1. Platform API (stable, ~8 models): set HIGGSFIELD_API_KEY and HIGGSFIELD_SECRET (or HF_CREDENTIALS=KEY:SECRET). The CLI picks the right backend per model; commands that need the unselected backend will print an actionable error.

## Quick Start

```bash
# browse the 17 video models locally with offline search
higgsfield-pp-cli models list --type video --json --select job_set_type,display_name


# find the Soul ID you trained for your recurring character
higgsfield-pp-cli soul-ids search "riggs" --json


# submit one prompt to two models with a credit guard
higgsfield-pp-cli fanout --prompt "cinematic dawn over Manhattan" --models veo3_1,seedance_2_0 --max-cost 80


# poll all jobs in the fanout and render results side by side
higgsfield-pp-cli fanout wait fan_<id> --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Batch orchestration

- **`fanout`** — Submit one prompt to N models in parallel and track them as a single fanout group.

  _Reach for this whenever you need to compare the same prompt across models; one command replaces N submits and N polls._

  ```bash
  higgsfield-pp-cli fanout --prompt "cinematic dawn over Manhattan, slow camera push" --models veo3_1,seedance_2_0,kling3_0 --max-cost 200 --json
  ```
- **`fanout wait`** — Poll every job in a fanout, then render a side-by-side table of model, cost, duration, and result URL.

  _Decide which model to ship by reading one table, not N tabs._

  ```bash
  higgsfield-pp-cli fanout wait fan_20260516_001 --json --select model,cost,result_url
  ```

### Local state that compounds

- **`soul-ids search`** — Full-text search across Soul ID names plus the prompts each Soul ID has been used in, ranked by last-used.

  _Stop scrolling Soul IDs to find the one you trained two months ago; ask by name or by the prompt that used it._

  ```bash
  higgsfield-pp-cli soul-ids search "riggs" --json
  ```
- **`fanout --max-cost`** — Pre-flight cost estimate across every fanout model; refuses to submit if total exceeds the cap and prints the per-model breakdown.

  _Run before you spend; the operator (or agent) sees the credit hit before the run lands._

  ```bash
  higgsfield-pp-cli fanout --prompt "..." --models veo3_1,veo3_1_lite --max-cost 80
  ```
- **`search`** — FTS5 search across every prompt you've ever submitted, joined with model, Soul ID name, and transaction memo.

  _Recall the exact prompt that produced last week's hero shot without re-typing or guessing._

  ```bash
  higgsfield-pp-cli search "cinematic riggs dawn" --json --select prompt,model,result_url,created_at
  ```
- **`soul-ids usage`** — Every generation that used a given Soul ID, ordered by date, with thumbnail URLs and total cost.

  _Audit a Soul ID's history before retiring it, or pull every cut-in for an episode summary._

  ```bash
  higgsfield-pp-cli soul-ids usage soul_riggs_42 --since 30d --json
  ```
- **`account spend`** — Local SQL aggregation over synced credit transactions, grouped by model, day, or Soul ID.

  _Weekly credit review without manual pagination; forecasts next month's spend by trend._

  ```bash
  higgsfield-pp-cli account spend --since 7d --group-by model --json
  ```

### Agent-native plumbing

- **`generate export`** — Export the local generations table joined with models and Soul IDs to CSV or JSONL for spreadsheet review or downstream pipelines.

  _Hand the week's output to any tool that reads CSV/JSONL; no manual scraping required._

  ```bash
  higgsfield-pp-cli export models --format jsonl --output models.jsonl
  ```

## Usage

Run `higgsfield-pp-cli --help` for the full command reference and flag list.

## Commands

### account

Account status, credit balance, and credit transactions

- **`higgsfield-pp-cli account status`** - Show signed-in account email, plan, and available credits
- **`higgsfield-pp-cli account transactions`** - List recent credit transactions (debits for generations, credits for top-ups/refunds)

### generations

AI generation jobs — image, video, text-to-image, image-to-video, Soul-driven, Marketing Studio

- **`higgsfield-pp-cli generations cancel`** - Cancel a pending generation
- **`higgsfield-pp-cli generations get`** - Get one generation by ID (web backend)
- **`higgsfield-pp-cli generations list`** - List recent generation jobs from the web backend (cloud.higgsfield.ai). Returns the most-recent 100 by default.

### models

Higgsfield model catalog — 35 image, video, and text models across both backends

- **`higgsfield-pp-cli models get`** - Get one model's full parameter schema
- **`higgsfield-pp-cli models list`** - List all available models with type, display name, and backend assignment

### soul_ids

Custom Soul references (trained characters and styles) for use in soul-aware models

- **`higgsfield-pp-cli soul_ids create`** - Train a new Soul ID from at least 5 reference images. Returns the new Soul ID's training job — poll with `soul-id wait`.
- **`higgsfield-pp-cli soul_ids get`** - Get one Soul ID and its training status
- **`higgsfield-pp-cli soul_ids list`** - List all your Soul IDs (custom-trained character references)

### uploads

Upload media (images, videos, audio) for use in subsequent generations as upload_id references

- **`higgsfield-pp-cli uploads`** - Initialize a media upload — returns a media_id for the second step

### workspaces

Billing workspaces — switch between personal and shared workspace contexts

- **`higgsfield-pp-cli workspaces list`** - List workspaces available to your account
- **`higgsfield-pp-cli workspaces status`** - Show the currently selected workspace


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
higgsfield-pp-cli generations list

# JSON for scripting and agents
higgsfield-pp-cli generations list --json

# Filter to specific fields
higgsfield-pp-cli generations list --json --select id,name,status

# Dry run — show the request without sending
higgsfield-pp-cli generations list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
higgsfield-pp-cli generations list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
higgsfield-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/higgsfield-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `HIGGSFIELD_JWT` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `higgsfield-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $HIGGSFIELD_JWT`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **auth: no token configured** — Run `higgsfield auth login` in your terminal (the official CLI), then `higgsfield-pp-cli auth set-token "$(higgsfield auth token)"`
- **web backend disabled** — Export HIGGSFIELD_ENABLE_WEB_BACKEND=1 to opt into cloud.higgsfield.ai endpoints (Veo 3.x, Sora 2, Kling 3, Soul V2, Nano Banana 2, Seedance 2)
- **model X requires platform API key** — Set HIGGSFIELD_API_KEY and HIGGSFIELD_SECRET (or HF_CREDENTIALS=KEY:SECRET); generate a key at platform.higgsfield.ai
- **fanout: cost cap exceeded ($240 > $200)** — Raise --max-cost, drop a model from --models, or run `generate cost <model> --prompt ...` individually to inspect
- **JWT expired** — The web backend __session cookie rotates every ~1 minute. Re-run `higgsfield auth login` (the official CLI handles the refresh) then re-set HIGGSFIELD_JWT

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**higgsfield-ai/cli**](https://github.com/higgsfield-ai/cli) — Go
- [**Hikhakk/higgsfield-mcp-unified**](https://github.com/Hikhakk/higgsfield-mcp-unified) — Python
- [**higgsfield-ai/higgsfield-client**](https://github.com/higgsfield-ai/higgsfield-client) — Python
- [**higgsfield-ai/higgsfield-js**](https://github.com/higgsfield-ai/higgsfield-js) — TypeScript
- [**geopopos/higgsfield_ai_mcp**](https://github.com/geopopos/higgsfield_ai_mcp) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
