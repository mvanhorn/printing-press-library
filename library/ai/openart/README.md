# Openart CLI

Spend your OpenArt credits programmatically. Generate videos with Seedance, Kling, Veo and more from the terminal.

Learn more at [Openart](https://openart.ai).

Printed by [@mvanhorn](https://github.com/mvanhorn) (Matt Van Horn).

## Install

The recommended path installs both the `openart-pp-cli` binary and the `pp-openart` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install openart
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install openart --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/openart-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-openart --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-openart --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-openart skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-openart. The skill defines how its required CLI can be installed.
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Authenticate

This CLI uses your browser session for authentication. Log in to  in Chrome, then:

```bash
openart-pp-cli auth login --chrome
```

Requires a cookie extraction tool. Install one:

```bash
pip install pycookiecheat          # Python (recommended)
brew install barnardb/cookies/cookies  # Homebrew
```

When your session expires, run `auth login --chrome` again.

### 3. Verify Setup

```bash
openart-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
openart-pp-cli history mock-value
```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Generation

- **`video gen`** — Submit + poll + download videos in one command (Seedance, Kling, Veo, Wan, Grok Imagine).

  ```bash
  openart-pp-cli video gen --prompt "a phoenix" --model byte-plus-seedance-2 --duration 10 --count 2 --wait --download ./out/
  ```

- **`image gen`** — Submit + poll + download images in one command. The Nano Banana family has two variants: `nano-banana` (50 credits, verified end-to-end) and `nano-banana-pro` (120 credits, higher quality, gated as experimental). There is no `nano-banana-2` — Pro is the upgrade path. The other image models (`gpt-image-2`, `gpt-image-1-5`, `flux-2-pro`, `byte-plus-seedream-4`, `byte-plus-seedream-4-5`, `google-imagen-4`, `qwen-image-max`) are also gated as `--accept-experimental` (their submit shapes were inferred from the JS bundle but not individually exercised live). Run `openart-pp-cli models list --family image` to see slugs, costs, and the `experimental` flag before picking.

  ```bash
  # verified, cheapest path
  openart-pp-cli image gen --prompt "donkey on Mercer Island" --model nano-banana --count 2 --wait --download ./out/

  # higher-quality Pro variant — opt in with --accept-experimental
  openart-pp-cli image gen --prompt "donkey on Mercer Island" --model nano-banana-pro --count 2 --accept-experimental --wait --download ./out/
  ```

- **`video gen --no-audio`** — Disables Seedance's auto-generated audio track. Workaround for `OutputAudioSensitiveContentDetected` moderation failures, and also halves the cost on Seedance 2.0 normal mode.

  ```bash
  openart-pp-cli video gen --prompt "..." --model seedance2 --no-audio --wait
  ```

### Credit-aware spending
- **`cost estimate`** — Project the credit cost of a generation before you spend.

  _Use before any batch submit to avoid burning credits on a misconfigured run._

  ```bash
  openart-pp-cli cost estimate --model byte-plus-seedance-2 --duration 10 --count 4 --agent
  ```
- **`credits burn`** — Aggregate credit spend by model, tool, day, or project.

  _Answers 'where did my credits go?' so you can shift work to cheaper models._

  ```bash
  openart-pp-cli credits burn --since 7d --by model --agent
  ```
- **`credits forecast`** — Project how many weeks of runway your current balance gives at recent burn.

  _Surfaces 'this paid plan covers another 6 weeks' so you can decide when to top up or slow down._

  ```bash
  openart-pp-cli credits forecast --agent
  ```

### Cross-model leverage
- **`models cheapest`** — Find the cheapest model that satisfies a target shape (type/duration/resolution/features).

  _Lets agents try a prompt on Grok Imagine for 150 credits before committing to Seedance 2's 800._

  ```bash
  openart-pp-cli models cheapest --type video --duration 10 --resolution 720p --agent
  ```
- **`compare`** — Run one prompt across multiple models in parallel; return a side-by-side report with cost and URLs.

  _Decide which model deserves the long Seedance shoot before committing._

  ```bash
  openart-pp-cli compare --prompt "a phoenix in molten gold" --models byte-plus-seedance-2,kling2-6,grok-imagine --duration 5 --agent
  ```

### Prompt history as memory
- **`prompts replay`** — Re-run a past generation, optionally on a different model, with parameter bumps.

  _Iterate on past wins without retyping prompts; A/B test models cheaply._

  ```bash
  openart-pp-cli prompts replay 3dVHEhDjyq82gLwBudaG --model byte-plus-seedance-2 --bump duration=10 --agent
  ```
- **`prompts find`** — Full-text search prior generations by prompt text with OpenArt-specific filters (model, audio, duration, starred, since).

  _Recover the prompt you wrote three weeks ago instead of paying to recreate it._

  ```bash
  openart-pp-cli prompts find "molten dragon" --model byte-plus-seedance-2 --has-audio --since 30d --agent
  ```
- **`prompts top`** — Rank past prompts by total credit spend over a window.

  _Surface 'I have spent 8k credits iterating this dragon prompt' before iterating again._

  ```bash
  openart-pp-cli prompts top --since 30d --by spend --agent
  ```
- **`stats`** — One-shot stats over the local media library: counts per type/model/period and rolling spend.

  _A 'where am I' snapshot for both human users and agents starting a session._

  ```bash
  openart-pp-cli stats --since 30d --agent
  ```

## Usage

Run `openart-pp-cli --help` for the full command reference and flag list.

## Commands

### credits

Credit balance and ledger

- **`openart-pp-cli credits logs`** - Get the credit ledger (CONSUME, RECHARGE, REFUND entries)

### forms

Generation forms (submit a video, image, lipsync, motion-sync, etc.)

- **`openart-pp-cli forms submit`** - Submit a generation. capability_id is '<model-slug>:<form-type>' URL-encoded (e.g. 'byte-plus-seedance-2:text2video'). Returns historyId + resourceIds; poll resources/<id> until status='completed'.

### history

Generation history (submissions)

- **`openart-pp-cli history get`** - Get a generation history entry by ID. Includes capability_id and the original input.

### media

Generation history and uploaded media

- **`openart-pp-cli media get`** - Get a single resource by ID. Use this to poll a generation in progress until status='completed' and url is populated.
- **`openart-pp-cli media list`** - List generated and uploaded resources (videos, images, audio) in the active project

### project

Projects within a workspace

- **`openart-pp-cli project default`** - Get the default project ID for the active workspace
- **`openart-pp-cli project folders`** - List folders inside a project
- **`openart-pp-cli project list`** - List projects in the active workspace

### prompt

Prompt utilities (auto-polish, reverse from image)

- **`openart-pp-cli prompt enhance`** - Auto-polish a prompt (LLM rewrite for better generation)
- **`openart-pp-cli prompt from_image`** - Reverse: extract a suggested prompt from an image URL

### templates

Saved generation templates

- **`openart-pp-cli templates list`** - List saved templates

### upload

Upload reference images for image-to-video and other ref-based forms

- **`openart-pp-cli upload list`** - List uploaded reference images
- **`openart-pp-cli upload persist`** - Persist an uploaded image as a referencable asset
- **`openart-pp-cli upload sign`** - Get a signed upload URL for a new reference image

### user

User identity, workspace, and credit balance

- **`openart-pp-cli user current_workspace`** - Get the active workspace for this session
- **`openart-pp-cli user info`** - Get current user identity, subscription, and credit balance
- **`openart-pp-cli user last_active`** - Heartbeat the active session
- **`openart-pp-cli user settings`** - Get user settings

### workspace

Workspaces and team membership

- **`openart-pp-cli workspace list`** - List workspaces this user can access
- **`openart-pp-cli workspace members`** - List members of the active workspace


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
openart-pp-cli history mock-value

# JSON for scripting and agents
openart-pp-cli history mock-value --json

# Filter to specific fields
openart-pp-cli history mock-value --json --select id,name,status

# Dry run — show the request without sending
openart-pp-cli history mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
openart-pp-cli history mock-value --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-openart -g
```

Then invoke `/pp-openart <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
# Some tools work without auth. For full access, set up auth first:
openart-pp-cli auth login --chrome

claude mcp add openart openart-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
openart-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/openart-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "openart": {
      "command": "openart-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
openart-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/openart-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `openart-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Capture coverage: 0 API entries from 0 total network entries
- Reachability: browser_clearance_http (0% confidence)

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
