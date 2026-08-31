# Google Flow CLI

**Everything Flow's community CLIs and MCP servers do, plus the two things none of them do: real Google Drive ingestion and an honest audio-drama pipeline that closes the loop Flow itself cannot.**

Flow has no in-app Google Drive picker and no way to import or sync to your own audio track -- this CLI pulls seed images straight from Drive, turns a Scribe recap script into ready-to-approve per-shot prompts, and muxes your real audio back onto the rendered clips locally, while staying honest that the final credit-spend click stays a transparent, user-driven action because of Google's reCAPTCHA gate on that step. The full pipeline is five steps, two of them manual by necessity: (1) `episode import` drafts the prompt queue from your two Drive folders, (2) you submit each shot's prompt in the real Flow UI (this CLI cannot automate that click -- see Authentication and Troubleshooting below), (3) you copy each returned job/workflow name into the queue file's job_name field, (4) `video watch --batch` polls them all at once, (5) `mux` lays your real audio back over the finished clips.

Learn more at [Google Flow](https://labs.google/fx/tools/flow).

Created by [@wryenmeek](https://github.com/wryenmeek) (github-actionsbot).

## Install

The recommended path installs both the `flow-pp-cli` binary and the `pp-flow` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install flow
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install flow --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install flow --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install flow --agent claude-code
npx -y @mvanhorn/printing-press-library install flow --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/flow/cmd/flow-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/flow-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install flow --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-flow --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-flow --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install flow --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/flow-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `FLOW_SESSION_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/flow/cmd/flow-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "flow": {
      "command": "flow-pp-mcp",
      "env": {
        "FLOW_SESSION_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Flow has two independent auth surfaces, both rooted in the same Google sign-in but neither auto-refreshing. (1) aisandbox-pa.googleapis.com (credits, video status, generation) needs a harvested `ya29.*` Bearer token: labs.google/fx/tools/flow is a client-rendered page whose JavaScript mints it via Google Identity Services, with no server-side endpoint this CLI can call instead -- open the page in a logged-in browser, open its network tab, copy the `Authorization: Bearer ya29....` header value off any aisandbox-pa.googleapis.com request, and `export FLOW_SESSION_TOKEN=<that value>`. (2) labs.google's own Next.js BFF (project, projects, scenes gaps, drive import --tag-scene) needs a NextAuth session cookie instead -- capture `__Secure-next-auth.session-token` from the same logged-in browser (e.g. via a Playwright storage_state() export or any cookie-export tool) and run `flow-pp-cli auth login --cookies-file storage-state.json` once. Re-capture and re-import/re-export either credential whenever a command reports the session has expired (~1 hour for the Bearer token).

## Quick Start

```bash
# Health check first -- works without auth, confirms the CLI itself is wired correctly.
flow-pp-cli doctor --dry-run

# Import a labs.google NextAuth session cookie (see Authentication below) -- covers project/projects/scenes gaps/drive import --tag-scene. credits/video watch need a separate FLOW_SESSION_TOKEN Bearer token, set as documented in Authentication below.
flow-pp-cli auth login --cookies-file storage-state.json

# List your Flow projects (sync --resources projects and export projects don't work yet for this resource -- see Troubleshooting below -- use this command directly instead).
flow-pp-cli projects

# Check your remaining Veo credit balance before planning any batch (the api-key is the public-looking 'key=' value Flow's own network requests send -- copy it once from your browser's dev tools).
flow-pp-cli credits --api-key AIzaSyDUMMY00000000000000000000000000

# Pull a whole episode's Scribe script + seed images from their two Drive folders and draft the prompt queue in one command.
flow-pp-cli episode import --scribe-folder ~/gdrive/session12-scribe --images-folder ~/gdrive/episode12-images

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Audio-drama pipeline
- **`script draft-prompts`** — Turn a Scribe recap-script JSON into a ready-to-approve queue of per-segment Flow prompts, each matched to the right seed image automatically.

  _Reach for this whenever the user has a Scribe-produced recap script and wants shot-by-shot Flow prompts drafted without hand-typing each one._

  ```bash
  flow-pp-cli script draft-prompts recap_script.json --images-dir ./seed-images --out episode3-queue.json
  ```
- **`episode import`** — Pull a whole episode's assets from two separate Google Drive folders -- Scribe's session output and the images folder -- into a Flow project and draft the prompt queue, in one command.

  _Reach for this to start a new episode end-to-end from the two Drive folders instead of running drive import and script draft-prompts separately._

  ```bash
  flow-pp-cli episode import --scribe-folder ~/gdrive/session12-scribe --images-folder ~/gdrive/episode12-images
  ```
- **`mux`** — Lay your real audio-drama mp3 back over the rendered Flow clips, in the right order and at the right offsets, with one local command.

  _Reach for this as the last step of any audio-drama animation pass -- it replaces the manual video-editor re-assembly step entirely._

  ```bash
  flow-pp-cli mux shot1.mp4 shot2.mp4 shot3.mp4 --audio episode3.mp3 --beats episode3-beats.json --out episode3-final.mp4
  ```

### Batch economics
- **`queue estimate`** — See whether a prepared batch of generations fits your remaining Flow credits before you spend a single one.

  _Reach for this before submitting any batch, especially when running multiple client projects against a shared credit pool._

  ```bash
  flow-pp-cli queue estimate episode3-queue.json
  ```
- **`video watch`** — Check on an entire submitted batch of generations with one command instead of clicking through each one.

  _Reach for this after queuing several generations and walking away -- one glance at aggregate progress instead of N manual look-ups._

  ```bash
  flow-pp-cli video watch --batch episode3-queue.json
  ```

### Drive ingestion
- **`drive import`** — Pull seed images straight out of a local Google Drive folder into a Flow project, with character tags inferred automatically from filenames (requires --project for character names).

  _Reach for this whenever new reference images land in a Drive folder and need staging into a Flow project -- eliminates the download-then-reupload dance entirely._

  ```bash
  flow-pp-cli drive import --folder-id ~/gdrive/episode3-images --tag-scene --project a1b2c3d4-e5f6-47a8-9b0c-1d2e3f4a5b6c
  ```

### Local sanity checks
- **`scenes gaps`** — Find characters missing a reference image and see a media-status breakdown for a project, before you submit a batch.

  _Reach for this as a pre-flight check before a big batch submission to catch missing assets early._

  ```bash
  flow-pp-cli scenes gaps --project a1b2c3d4-e5f6-47a8-9b0c-1d2e3f4a5b6c
  ```

## Recipes

### Stage a whole episode from Drive

```bash
flow-pp-cli drive import --folder-id ~/gdrive/episode3-images --tag-scene --project a1b2c3d4-e5f6-47a8-9b0c-1d2e3f4a5b6c
```

Pulls every new reference image out of a local Drive folder into the active project in one call, tagged with matching character names.

### Draft a shot-by-shot queue from a recap script

```bash
flow-pp-cli script draft-prompts recap_script.json --images-dir ./seed-images --out episode3-queue.json
```

Mechanically merges each recap-script element with its matching seed image into a Flow-ready prompt, no hand-typing required.

### Check a batch fits your credits before spending anything

```bash
flow-pp-cli queue estimate episode3-queue.json --api-key AIzaSyDUMMY00000000000000000000000000
```

Sums the queue's expected Veo-tier cost against your live balance and flags what to trim (omit --api-key for a local-only total with no live comparison).

### Track a whole batch instead of babysitting one clip at a time

```bash
flow-pp-cli video watch --batch episode3-queue.json
```

Aggregates repeated status polls across every job id in the queue into one progress table.

### List projects with just the fields you need

```bash
flow-pp-cli projects --json --select id,title,modifiedTime
```

Pairs --json with --select to avoid burning context on Flow's full nested project payload. Use the `projects` command directly, not `sync --resources projects` -- the generic sync/export engine can't build the tRPC input envelope this endpoint requires yet (see Troubleshooting below).

### Finish the loop: mux your real audio back onto the rendered clips

```bash
flow-pp-cli mux shot1.mp4 shot2.mp4 shot3.mp4 --audio episode3.mp3 --beats episode3-beats.json --out episode3-final.mp4
```

Overlays the user's real audio-drama track onto the ordered, rendered clips locally -- the step Flow itself cannot do.

## Usage

Run `flow-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `FLOW_CONFIG_DIR`, `FLOW_DATA_DIR`, `FLOW_STATE_DIR`, or `FLOW_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `FLOW_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export FLOW_HOME=/srv/flow
flow-pp-cli doctor
```

Under `FLOW_HOME=/srv/flow`, the four dirs resolve to `/srv/flow/config`, `/srv/flow/data`, `/srv/flow/state`, and `/srv/flow/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "flow": {
      "command": "flow-pp-mcp",
      "env": {
        "FLOW_HOME": "/srv/flow"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `FLOW_DATA_DIR` overrides an explicit `--home` for that kind. Use `FLOW_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `FLOW_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `flow-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### credits

Flow/Veo credit balance

- **`flow-pp-cli credits`** - Check remaining Flow/Veo credit balance

### flowWorkflows

Generation workflow lifecycle state

- **`flow-pp-cli flow-workflows <workflowId>`** - Fetch a generation workflow's current state

### project

Full contents of one Flow project (media, characters/scenes, workflows)

- **`flow-pp-cli project`** - Fetch a project's full contents: media assets, character/scene entities, and generation workflows -- everything the web UI's Images/Characters/Scenes tabs filter client-side from this one call

### projects

Flow projects (offline-mirrored)

- **`flow-pp-cli projects`** - List the authenticated user's Flow projects

### video

Async video generation jobs

- **`flow-pp-cli video`** - Poll status for one or more in-flight/queued video generations


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`flow-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`flow-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`flow-pp-cli learnings list`** - Inspect taught rows
- **`flow-pp-cli learnings forget <query>`** - Undo a teach
- **`flow-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`flow-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`flow-pp-cli teach-pattern`** - Install a query/resource template up front
- **`flow-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `FLOW_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `flow-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
flow-pp-cli credits --api-key your-token-here

# JSON for scripting and agents
flow-pp-cli credits --api-key your-token-here --json
# Filter to specific fields
flow-pp-cli credits --api-key your-token-here --json --select remainingCredits,planTier

# Dry run — show the request without sending
flow-pp-cli credits --api-key your-token-here --dry-run

# Agent mode — JSON + compact + no prompts in one flag
flow-pp-cli credits --api-key your-token-here --agent
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
flow-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `flow-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/flow-pp-cli/config.toml`; `--home`, `FLOW_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `FLOW_SESSION_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `flow-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `flow-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $FLOW_SESSION_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Commands fail with an auth/session error after working fine earlier** — There is no automatic token refresh -- Flow session page mints its Bearer token client-side in the browser, with no server endpoint this CLI can call. Re-open labs.google/fx/tools/flow in a logged-in browser, copy the fresh Authorization: Bearer ya29.... header value from a network request, and export FLOW_SESSION_TOKEN=<that value> again; the token is short-lived (~1 hour).
- **There is no command to submit a generation from this CLI** — This is intentional, not a missing feature -- Google's reCAPTCHA Enterprise gate on Flow's generation-submit step (flowCreationAgent:streamChat) cannot be solved headlessly, so this CLI drafts and tracks batches but always hands the actual submit click back to you in the real Flow UI.
- **`drive import --tag-scene` succeeds but tags come back empty** — Tagging is a best-effort filename match against character names synced from --project, not a fixed naming convention -- pass --project, confirm the project actually has CHARACTER entities via `scenes gaps --project <id>`, and check the image filenames actually contain a character's display name.
- **`scenes gaps`, `drive import --tag-scene`, `project`, or `projects` fail with an auth error even with a fresh FLOW_SESSION_TOKEN set** — These commands call labs.google's own Next.js BFF (project.* tRPC endpoints), which authenticates via a NextAuth session cookie, not the ya29.* Bearer token -- run `flow-pp-cli auth login --cookies-file storage-state.json` with a storage-state export containing your logged-in labs.google cookies (Playwright's storage_state(), or any browser-cookie export tool). FLOW_SESSION_TOKEN remains separately required for credits/video/generation; the two credentials are independent and neither auto-refreshes.
- **`sync --resources projects` or `export projects` fail even with both a valid FLOW_SESSION_TOKEN and an imported labs.google session cookie** — This is a known, unimplemented gap, not a misconfiguration on your end: project.searchUserProjects requires its query params wrapped in a single JSON-encoded `input=` envelope, which the generic sync/export engine has no mechanism to build. Use `flow-pp-cli projects` directly instead -- the hand-written command constructs the envelope correctly.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

TLS certificates are verified by default. For a trusted development or self-signed endpoint only, pass `--insecure` for one invocation, set `FLOW_SKIP_TLS_VERIFY=true` for the current environment, or set `skip_tls_verify = true` in the config file for a persistent override.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Capture coverage: 0 API entries from 0 total network entries
- Reachability: browser_clearance_http (85% confidence)

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**gflow-cli**](https://github.com/ffroliva/gflow-cli) — Python
- [**Google-Flow_MCP**](https://github.com/Mitanshp5/Google-Flow_MCP) — TypeScript
- [**google-flow-browser-mcp**](https://github.com/TMSSS05/google-flow-browser-mcp) — TypeScript
- [**veo-automation-user-guide**](https://github.com/trgkyle/veo-automation-user-guide) — JavaScript
- [**gemini-video-producer-skill**](https://github.com/zysilm-ai/gemini-video-producer-skill) — Markdown

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
