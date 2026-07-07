---
name: pp-openart
description: "Generate videos and images on OpenArt from the terminal. Spend your existing OpenArt credits across Seedance, Kling,..."
author: "Matt Van Horn"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - openart-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/ai/openart/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Openart — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `openart-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install openart --cli-only
   ```
2. Verify: `openart-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Spend your OpenArt credits programmatically. Generate videos with Seedance, Kling, Veo and more from the terminal.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Generation
- **`video gen`** — Submit + poll + download videos in one command.

  ```bash
  openart-pp-cli video gen --prompt "..." --model byte-plus-seedance-2 --duration 10 --count 2 --wait --download ./out/
  ```

- **`image gen`** — Submit + poll + download images in one command. The Nano Banana family has two variants: `nano-banana` (50 credits, verified) and `nano-banana-pro` (120 credits, higher quality, experimental). There is no `nano-banana-2` — Pro is the upgrade path. The other image models (`gpt-image-2`, `gpt-image-1-5`, `flux-2-pro`, `byte-plus-seedream-4`, `byte-plus-seedream-4-5`, `google-imagen-4`, `qwen-image-max`) are also experimental: their submit shapes were inferred from the JS bundle but not individually verified live, so they require `--accept-experimental`. Run `openart-pp-cli models list --family image` to see slugs, costs, and the `experimental` flag before picking.

  ```bash
  # verified, cheapest path
  openart-pp-cli image gen --prompt "donkey on Mercer Island" --model nano-banana --count 2 --wait --download ./out/

  # higher-quality Pro variant — opt in with --accept-experimental
  openart-pp-cli image gen --prompt "donkey on Mercer Island" --model nano-banana-pro --count 2 --accept-experimental --wait --download ./out/
  ```

- **`video gen --no-audio`** — Disables Seedance's auto-generated audio. Workaround for `OutputAudioSensitiveContentDetected` failures; halves the cost on Seedance 2.0 normal mode.

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

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 0 API entries from 0 total network entries

## Command Reference

**credits** — Credit balance and ledger

- `openart-pp-cli credits` — Get the credit ledger (CONSUME, RECHARGE, REFUND entries)

**forms** — Generation forms (submit a video, image, lipsync, motion-sync, etc.)

- `openart-pp-cli forms <capability_id>` — Submit a generation. capability_id is '<model-slug>:<form-type>' URL-encoded (e.g....

**history** — Generation history (submissions)

- `openart-pp-cli history <historyId>` — Get a generation history entry by ID. Includes capability_id and the original input.

**media** — Generation history and uploaded media

- `openart-pp-cli media get` — Get a single resource by ID. Use this to poll a generation in progress until status='completed' and url is populated.
- `openart-pp-cli media list` — List generated and uploaded resources (videos, images, audio) in the active project

**project** — Projects within a workspace

- `openart-pp-cli project default` — Get the default project ID for the active workspace
- `openart-pp-cli project folders` — List folders inside a project
- `openart-pp-cli project list` — List projects in the active workspace

**prompt** — Prompt utilities (auto-polish, reverse from image)

- `openart-pp-cli prompt enhance` — Auto-polish a prompt (LLM rewrite for better generation)
- `openart-pp-cli prompt from_image` — Reverse: extract a suggested prompt from an image URL

**templates** — Saved generation templates

- `openart-pp-cli templates` — List saved templates

**upload** — Upload reference images for image-to-video and other ref-based forms

- `openart-pp-cli upload list` — List uploaded reference images
- `openart-pp-cli upload persist` — Persist an uploaded image as a referencable asset
- `openart-pp-cli upload sign` — Get a signed upload URL for a new reference image

**user** — User identity, workspace, and credit balance

- `openart-pp-cli user current_workspace` — Get the active workspace for this session
- `openart-pp-cli user info` — Get current user identity, subscription, and credit balance
- `openart-pp-cli user last_active` — Heartbeat the active session
- `openart-pp-cli user settings` — Get user settings

**workspace** — Workspaces and team membership

- `openart-pp-cli workspace list` — List workspaces this user can access
- `openart-pp-cli workspace members` — List members of the active workspace


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
openart-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

This CLI uses a browser session. Log in to  in Chrome, then:

```bash
openart-pp-cli auth login --chrome
```

Requires a cookie extraction tool (`pycookiecheat` via pip, or `cookies` via Homebrew).

Run `openart-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  openart-pp-cli history mock-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
openart-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
openart-pp-cli feedback --stdin < notes.txt
openart-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.openart-pp-cli/feedback.jsonl`. They are never POSTed unless `OPENART_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `OPENART_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
openart-pp-cli profile save briefing --json
openart-pp-cli --profile briefing history mock-value
openart-pp-cli profile list --json
openart-pp-cli profile show briefing
openart-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `openart-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add openart-pp-mcp -- openart-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which openart-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   openart-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `openart-pp-cli <command> --help`.
