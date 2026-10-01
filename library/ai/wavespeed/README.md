# WaveSpeed CLI

**Run any WaveSpeed model from the terminal, with price checks, safe uploads, and recovery-safe downloads.**

WaveSpeed hosts image, video, audio, and 3D models behind one API. This CLI adds a dynamic model runner for slash-delimited model IDs, free price estimates, local file uploads, a generation library with cost reports, and a D2C content layer (plan, pack, batch, variants, compose, aspects, restyle, brand, qa). Paid submissions are never replayed, and every post-submit failure keeps the prediction ID and a recovery command.

Learn more at [WaveSpeed](https://wavespeed.ai).

Created by [@cathrynlavery](https://github.com/cathrynlavery) (Cathryn Lavery).

## Install

The recommended path installs both the `wavespeed-pp-cli` binary and the `pp-wavespeed` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install wavespeed
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install wavespeed --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install wavespeed --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install wavespeed --agent claude-code
npx -y @mvanhorn/printing-press-library install wavespeed --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/wavespeed/cmd/wavespeed-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/wavespeed-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install wavespeed --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-wavespeed --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-wavespeed --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install wavespeed --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/wavespeed-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `WAVESPEED_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/ai/wavespeed/cmd/wavespeed-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "wavespeed": {
      "command": "wavespeed-pp-mcp",
      "env": {
        "WAVESPEED_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

WaveSpeed uses an API key sent as `Authorization: Bearer <key>`. Create one at https://wavespeed.ai/accesskey and export it as `WAVESPEED_API_KEY`. `doctor` verifies the key with a free `/balance` read.

## Quick Start

```bash
# Verify the API key with a free balance read
wavespeed-pp-cli doctor

# Find an image-edit model and its price
wavespeed-pp-cli models --capability image-edit --agent

# Quote the cost before spending
wavespeed-pp-cli price --model-id google/nano-banana-2/edit --set aspect_ratio=9:16 --set resolution=2k --agent

# Generate, wait, and download (local files work too: --images @ref.png)
wavespeed-pp-cli run --model-id google/nano-banana-2/edit --prompt "a red mug on oak" --images https://wavespeed.ai/ref.png --set aspect_ratio=9:16 --wait --download 'out/mug_{index}.{ext}' --record --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Core
- **`run`** — Submit WaveSpeed model runs with prompt shorthand, typed --set inputs, local @file media uploads, price estimates, waiting, recovery-safe downloads, and library recording.

  _Use it for any single generation; the prediction ID and recovery command survive every post-submit failure._

  ```bash
  wavespeed-pp-cli run --model-id google/nano-banana-2/edit --prompt "a red mug on oak" --images @ref.png --set aspect_ratio=9:16 --wait --download 'out/mug_{index}.{ext}' --agent
  ```
- **`schema`** — Fetch the live WaveSpeed model catalog and print the request schema for a model or project alias.

  _Check accepted inputs and enums before a paid run._

  ```bash
  wavespeed-pp-cli schema google/nano-banana-2/edit --agent
  ```
- **`price`** — Estimate WaveSpeed model pricing with the same input syntax used by run, without submitting a prediction.

  _Free; quote the cost before spending._

  ```bash
  wavespeed-pp-cli price --model-id google/nano-banana-2/edit --set aspect_ratio=9:16 --set resolution=2k --agent
  ```
- **`aliases`** — Read wavespeed.json aliases, default model, and output directory settings for repeatable local workflows.

  _See which short names map to which models._

  ```bash
  wavespeed-pp-cli aliases --agent
  ```
- **`init`** — Write a starter wavespeed.json with aliases, default model, and output directory.

  _Start a new image project._

  ```bash
  wavespeed-pp-cli init
  ```

### Media
- **`upload`** — Upload local image, video, or audio files to WaveSpeed media storage for use as model input URLs, retrying stalled uploads with a size-scaled deadline.

  _Turn a local file into a URL a model accepts._

  ```bash
  wavespeed-pp-cli upload ./ref.png --agent
  ```
- **`download`** — Download generated output URLs with directory, exact-path, or templated-path destinations, never sending API credentials to CDN hosts.

  _Re-fetch outputs from a finished prediction._

  ```bash
  wavespeed-pp-cli download "$OUTPUT_URL" --output 'out/{index}.{ext}'
  ```
- **`last`** — Print or open the most recent downloaded output.

  _Grab the path of the last generated file._

  ```bash
  wavespeed-pp-cli last
  ```

### Plan
- **`plan brief-to-shotlist`** — Turn a free-text brief into a structured shotlist across platforms and aspect ratios with a hybrid deterministic-parser/LLM planner.

  _Draft the shot list for a campaign._

  ```bash
  wavespeed-pp-cli plan brief-to-shotlist --prompt "Helm Black launch" --platforms instagram,tiktok --agent
  ```
- **`plan model-pick`** — Recommend a model for an intent from the live catalog with rationale.

  _Choose a model for a job._

  ```bash
  wavespeed-pp-cli plan model-pick "short product video" --agent
  ```
- **`plan cost-estimate`** — Price a shotlist against live /model/pricing and the account balance, with cached-pricing fallback and per-shot breakdown.

  _Check a shotlist fits the budget._

  ```bash
  wavespeed-pp-cli plan cost-estimate shotlist.json --agent
  ```
- **`qa preflight`** — Pass/warn/fail validation of a shotlist: balance vs cost, model availability, prompt safety, platform request-shape, and brand coverage.

  _Gate a pack before producing it._

  ```bash
  wavespeed-pp-cli qa preflight shotlist.json --agent
  ```

### Produce
- **`pack`** — Produce a multi-platform creative pack from one concept at stable packs/<slug>/<platform>/ paths with per-platform manifests, concurrency, cost ceiling, and image-dimension validation; a rerun archives the superseded manifest.

  _Produce platform-ready assets._

  ```bash
  wavespeed-pp-cli pack --concept "Helm Black hero" --platforms instagram,tiktok --max-cost 5.00 --agent
  ```
- **`batch`** — Submit many prompts from CSV or JSON with a spend ceiling and fail-fast/fail-tolerant semantics; records completed generations before any abort.

  _Run a prompt list under a budget._

  ```bash
  wavespeed-pp-cli batch --from prompts.csv --max-cost 5.00 --agent
  ```
- **`variants`** — Sweep seed, style, or model off a base shot to produce comparable outputs with side-by-side metadata.

  _Explore seeds or models for one shot._

  ```bash
  wavespeed-pp-cli variants --base shotlist.json --vary seed --count 4 --agent
  ```
- **`compose`** — Run an explicit multi-step pipeline (text->image->upscale->video), feeding each step's output to the next, with rollback of later steps on failure.

  _Turn a prompt into an image and then a clip._

  ```bash
  wavespeed-pp-cli compose --steps "text->image,image->video" --prompt "..." --models m1,m2 --agent
  ```

### Refine
- **`aspects`** — Re-frame one image into standard platform aspect ratios, using outpaint when supported and an anchored re-render otherwise.

  _Make 9:16 and 1:1 versions of a hero image._

  ```bash
  wavespeed-pp-cli aspects hero.png --platforms instagram,tiktok --agent
  ```
- **`restyle`** — Apply a brand profile or explicit style to an existing asset via img2img with a style prompt.

  _Bring an asset onto brand._

  ```bash
  wavespeed-pp-cli restyle hero.png --brand helm --agent
  ```

### Library
- **`library`** — List, search (FTS5), show, tag, export, and cost-report the local generation library by brand, model, platform, and tag.

  _Find a past generation or total spend._

  ```bash
  wavespeed-pp-cli library cost-report --group-by model --agent
  ```
- **`brand`** — Create, inspect, apply, and edit brand profiles that auto-merge into pack, compose, variants, restyle, and run.

  _Set up a brand for consistent output._

  ```bash
  wavespeed-pp-cli brand init helm --palette '#111,#eee' --voice calm
  ```

## Recipes

### Image edit with a reference, 9:16 at 2k

```bash
wavespeed-pp-cli run --model-id google/nano-banana-2/edit --prompt "$PROMPT" --images https://wavespeed.ai/anchor.png --set aspect_ratio=9:16 --set resolution=2k --wait --download 'raw/shot_{index}.{ext}' --record --agent
```

Pass a URL or a local `@file`; local files upload automatically. The download template names each output.

### First-and-last-frame video

```bash
wavespeed-pp-cli run --model-id bytedance/seedance-v1.5-pro/image-to-video --prompt "$MOTION" --image https://wavespeed.ai/a.jpg --last-image https://wavespeed.ai/b.jpg --set duration=5 --wait --wait-timeout 10m --download 'clip_{index}.{ext}' --record --agent
```

`--wait-timeout` bounds polling; on timeout the prediction ID and recovery command are printed.

### Spend for a session

```bash
wavespeed-pp-cli billings --page-size 20 --agent
```

Billing rows carry the charged price per prediction.

### Plan a campaign

```bash
wavespeed-pp-cli plan brief-to-shotlist --prompt "launch" --platforms instagram,tiktok --agent
```

Save the shotlist, then run `plan cost-estimate` and `qa preflight` on it before `pack`.

## Usage

Run `wavespeed-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `WAVESPEED_CONFIG_DIR`, `WAVESPEED_DATA_DIR`, `WAVESPEED_STATE_DIR`, or `WAVESPEED_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `WAVESPEED_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export WAVESPEED_HOME=/srv/wavespeed
wavespeed-pp-cli doctor
```

Under `WAVESPEED_HOME=/srv/wavespeed`, the four dirs resolve to `/srv/wavespeed/config`, `/srv/wavespeed/data`, `/srv/wavespeed/state`, and `/srv/wavespeed/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "wavespeed": {
      "command": "wavespeed-pp-mcp",
      "env": {
        "WAVESPEED_HOME": "/srv/wavespeed"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `WAVESPEED_DATA_DIR` overrides an explicit `--home` for that kind. Use `WAVESPEED_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `WAVESPEED_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `wavespeed-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### account_balance

Manage account balance

- **`wavespeed-pp-cli account-balance`** - Retrieve the authenticated account balance.

### billings

Billing and usage records

- **`wavespeed-pp-cli billings`** - Search billing records for the authenticated account.

### media_uploads

Manage media uploads

- **`wavespeed-pp-cli media-uploads`** - Upload one existing local media file (image, video or audio) to WaveSpeed storage and return its URL for model inputs.

### model_pricing

Manage model pricing

- **`wavespeed-pp-cli model-pricing`** - Estimate the unit price for a model run using the same inputs that will be submitted to the model endpoint.

### models

Model catalog and model metadata

- **`wavespeed-pp-cli models`** - List available WaveSpeed models and their API schemas.

### prediction_deletions

Manage prediction deletions

- **`wavespeed-pp-cli prediction-deletions`** - Delete one or more predictions from history.

### prediction_results

Manage prediction results

- **`wavespeed-pp-cli prediction-results <task_id>`** - Retrieve the latest status and result payload for a prediction task.

### predictions

Prediction submission history and result retrieval

- **`wavespeed-pp-cli predictions`** - Query recent prediction history. The API history window is limited; sync accumulates across runs.

### usage_stats

Manage usage stats

- **`wavespeed-pp-cli usage-stats`** - Retrieve usage statistics for the authenticated account.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`wavespeed-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`wavespeed-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`wavespeed-pp-cli learnings list`** - Inspect taught rows
- **`wavespeed-pp-cli learnings forget <query>`** - Undo a teach
- **`wavespeed-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`wavespeed-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`wavespeed-pp-cli teach-pattern`** - Install a query/resource template up front
- **`wavespeed-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `WAVESPEED_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `wavespeed-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
wavespeed-pp-cli billings

# JSON for scripting and agents
wavespeed-pp-cli billings --json
# Filter to specific fields
wavespeed-pp-cli billings --json --select code,data,message

# Dry run — show the request without sending
wavespeed-pp-cli billings --dry-run

# Agent mode — JSON + compact + no prompts in one flag
wavespeed-pp-cli billings --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
wavespeed-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `wavespeed-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/wavespeed-pp-cli/config.toml`; `--home`, `WAVESPEED_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `WAVESPEED_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `wavespeed-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `wavespeed-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $WAVESPEED_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **A run timed out or the download failed after submission** — The prediction may still be running and is billed once. Check it with `wavespeed-pp-cli prediction-results "$PREDICTION_ID"`; the ID is printed with every post-submit failure. A failed download after a completed run is a warning (exit 6, partial_failure), not a failed generation.
- **Upload of a large local file stalls** — Uploads retry network errors, 429, and 5xx up to three times with a per-attempt deadline of --timeout plus size / 128 KiB/s. Uploads are free, so retries cannot add cost.
- **price quotes more than billings charged** — `price` returns list price; `billings` shows the charged price after account discounts (`order.price` vs `order.origin_price`).
