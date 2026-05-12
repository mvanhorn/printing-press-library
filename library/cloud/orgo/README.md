# Orgo CLI

**The audit ledger Orgo doesn't otherwise have, plus every existing Orgo SDK feature in one Go binary.**

Every screenshot, bash command, and click your agent runs through this CLI lands in a local SQLite store, so commands like `replay`, `audit`, `grep`, and `cost` see what no Orgo API call can. On top of that, the full computer-control surface — clone, resize, screenshot, click, type, bash — works offline-first with auto-JSON when piped, typed exit codes, and a `doctor` that flags stuck or suspended computers across every workspace in one call.

Learn more at [Orgo](https://orgo.ai).

## Install

The recommended path installs both the `orgo-pp-cli` binary and the `pp-orgo` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install orgo
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install orgo --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/orgo-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-orgo --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-orgo --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-orgo skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-orgo. The skill defines how its required CLI can be installed.
```

## Authentication

Bearer auth via ORGO_API_KEY (sk_live_...). Get a key at https://www.orgo.ai/workspaces. The CLI reads ORGO_API_KEY on every invocation, so rotating keys requires no restart. orgo doctor probes the key against the live API and reports the source (env var vs config file) without printing the value.

## Quick Start

```bash
# Confirms auth works and shows what you've got
orgo workspaces list


# Spin up a new desktop
orgo computers create --workspace-id <workspace-uuid> --name agent-1 --cpu 2 --ram 8


# Pull the framebuffer; lands in the local actions store too
orgo computers screenshot agent-1 --out /tmp/shot.png


# Run a real shell command on the desktop
orgo computers bash agent-1 --command 'ls /home/orgo'


# Cross-workspace health rollup — suspended, errored, stuck
orgo doctor


# See exactly what the CLI just did, agent-shaped output
orgo audit --since 1h --agent --select timestamp,kind,summary

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Audit ledger only the CLI has
- **`replay`** — Generate a self-contained static HTML timeline of every screenshot, click, bash, and exec your agent ran on a computer.

  _Reach for this when you need to audit, share, or debug what an agent actually did on a desktop — no live API roundtrips, just a single HTML file you can email or attach to an issue._

  ```bash
  orgo replay desktop_abc --since 1h --out replay.html
  ```
- **`audit`** — Chronological table of every CLI-driven action against your computers in a time window, scoped by workspace, FTS-searchable.

  _Reach for this when a customer asks 'what did the agent do this week' or you need a regression bundle for an incident._

  ```bash
  orgo audit --workspace prod --since 7d --agent --select timestamp,computer,kind,summary
  ```
- **`grep`** — FTS5 search over historical bash commands, Python exec code, and click coordinates from the local actions store.

  _Reach for this when you need to find a specific command an agent ran but don't remember which computer or when._

  ```bash
  orgo grep 'pip install' --type bash --since 30d
  ```

### Fleet stewardship
- **`fleet`** — Cross-workspace health rollup: surfaces suspended (over-quota), errored, stuck-creating, and stuck-stopping computers, plus an API-key validity probe.

  _Reach for this for incident response, before pushing a fleet change, or as a daily cron — one call replaces walking every workspace by hand._

  ```bash
  orgo-pp-cli fleet --agent
  ```
- **`idle`** — Sorts running computers by hours-since-last-CLI-action, surfacing burns that could be stopped.

  _Reach for this on your weekly cost pass — every idle computer with auto-stop disabled is a known leak._

  ```bash
  orgo idle --threshold-hours 24
  ```
- **`oversized`** — Flags computers with CPU >= 4 cores or RAM >= 16 GB whose last CLI-recorded action is older than the threshold and whose auto-stop is disabled.

  _Reach for this when you suspect a workspace is overspending — it pinpoints the exact downsize candidates._

  ```bash
  orgo oversized --min-cores 4 --idle-days 7
  ```
- **`prune`** — Cross-workspace status-filtered batch delete with dry-run by default.

  _Reach for this for Friday cleanup or after a downgrade leaves a fleet of suspended computers behind._

  ```bash
  orgo prune --status suspended,error --older-than 7d --dry-run
  ```
- **`cost`** — Reconstructs per-computer running-hours from local action timestamps + observed status transitions, multiplies by per-tier rate, sums by workspace. --forecast projects month-end burn.

  _Reach for this for monthly invoicing, customer billing questions, or to forecast whether a workload will hit the plan ceiling._

  ```bash
  orgo cost --workspace prod --since 30d --forecast
  ```

## Usage

Run `orgo-pp-cli --help` for the full command reference and flag list.

## Commands

### computers

Provision and manage virtual computers

- **`orgo-pp-cli computers create`** - Creates a new virtual computer in a workspace. The computer starts automatically after creation.
- **`orgo-pp-cli computers delete`** - Permanently deletes a computer and all its data.
- **`orgo-pp-cli computers get`** - Returns computer details including current status.

### files

Upload and download files

- **`orgo-pp-cli files delete`** - Permanently deletes a file from storage.
- **`orgo-pp-cli files download`** - Returns a signed download URL for a file. URLs expire in 1 hour.
- **`orgo-pp-cli files export`** - Exports a file from the computer's filesystem and returns a download URL.
- **`orgo-pp-cli files list`** - Lists all files in a workspace, optionally filtered by computer.
- **`orgo-pp-cli files upload`** - Uploads a file to a workspace. Maximum file size is 10MB.

### workspaces

Organize computers into named workspaces

- **`orgo-pp-cli workspaces create`** - Creates a new workspace. Workspace names must be unique per user.
- **`orgo-pp-cli workspaces delete`** - Deletes a workspace and all its computers. This action cannot be undone.
- **`orgo-pp-cli workspaces get`** - Returns a workspace by ID, including its computers.
- **`orgo-pp-cli workspaces list`** - Returns all workspaces for the authenticated user.


## DOM-Aware Browser Automation (`chrome`)

Drive Chrome inside an Orgo VM at the **DOM level** — accessibility tree with element refs, click by ref, type, evaluate JavaScript, inspect console and network. Much faster and more reliable than pixel-based screenshot+click for any web workflow.

This surface is **hand-built on top of the printed CLI** (not generated by the Printing Press — the upstream OpenAPI spec doesn't describe a per-VM bridge protocol). The bridge is a single zero-dependency Node script shipped embedded in the binary; it auto-deploys to `/tmp/orgo-chrome-bridge.js` on first use and listens on `127.0.0.1:7331` inside the VM.

```bash
# Cold-start: bridge auto-deploys on first call (~6s including bridge launch).
orgo-pp-cli chrome navigate <id> --url https://example.com

# Read the page as an accessibility tree — cheap, fast, includes element refs.
orgo-pp-cli chrome read-page <id> --filter interactive

# Find by intent, then click by ref.
orgo-pp-cli chrome find <id> --query "search bar"
orgo-pp-cli chrome click <id> --ref ref_3

# Set form values directly — no focus-then-type dance.
orgo-pp-cli chrome form-input <id> --ref ref_7 --value "user@example.com"

# Evaluate JavaScript in the page (no 'return' — just the expression).
orgo-pp-cli chrome evaluate <id> --expression "document.title"

# Save a real PNG to disk.
orgo-pp-cli chrome screenshot <id> --out /tmp/page.png

# Inspect console and network buffers since the bridge started.
orgo-pp-cli chrome console <id> --only-errors
orgo-pp-cli chrome network <id> --url-pattern "/api/"

# Hot DOM loops benefit from VM-direct routing exactly like bash/click do.
orgo-pp-cli chrome read-page <id> --vm-from <id> --filter interactive
```

**Full command set (16):** `navigate`, `tabs`, `new-tab`, `switch-tab`, `read-page`, `find`, `page-text`, `screenshot`, `click`, `type`, `form-input`, `scroll`, `evaluate`, `console`, `network`, `resize`.

**MCP exposure:** the cobra-tree walker registers every `chrome` subcommand as an MCP tool (`chrome_navigate`, `chrome_read_page`, …), so a single `claude mcp add orgo` gets you workspaces + computers + computer-use + DOM-aware browser-use from one server.

**Requirements (inside the VM):** Chrome (any of `google-chrome`, `chromium`, `chromium-browser`), Node 18+, an X display. All present by default on Orgo VMs. The first chrome call kills any stale bridge process, uploads the embedded script via a `base64 -d` round-trip (avoids shell-quoting hazards), starts it under `DISPLAY=:99 nohup node`, and polls `/health` for up to 30 seconds. Subsequent calls reuse the running bridge.

**Hash check on every call:** the embedded bridge's SHA256 is written to `/tmp/orgo-chrome-bridge.hash` after a successful deploy and compared on each invocation; a CLI upgrade with a new bridge triggers a redeploy on long-lived VMs.

## VM-Direct Routing (latency optimization)

Computer-use commands (`bash`, `click`, `type`, `key`, `scroll`, `drag`, `exec`, `screenshot`) can bypass the central API and talk directly to the per-VM agent. Measured win on a sample machine: **~0.55s → ~0.16s per call (~70%)**, because the central API's proxy hop and (for screenshots) Supabase upload are skipped.

```bash
# One-call resolver: a single central GET /computers/<id> fetches the VM's
# instance URL and vnc_password, then every subsequent call in the same
# invocation goes direct to the VM.
orgo-pp-cli computers bash execute <id> --vm-from <id> --command 'hostname'

# Explicit: useful for agents born inside the VM with the values injected.
# Token is the computer's vnc_password (from `computers get`).
orgo-pp-cli computers screenshot get <id> \
    --vm-url http://1.2.3.4:36100 \
    --vm-token <vnc_password>

# Env-injected variant (no flags needed):
export ORGO_VM_URL=http://1.2.3.4:36100
export ORGO_VM_TOKEN=<vnc_password>
orgo-pp-cli computers click mouse <id> --x 640 --y 360
```

**Which commands bypass:** `bash`, `click`, `type`, `key`, `scroll`, `drag`, `exec`, `screenshot`. Everything else (workspace/computer management, files, fleet ops, audit, etc.) continues to use the central API — those endpoints don't exist on the per-VM agent. Non-bypassable commands run normally when `--vm-*` is set, so a mixed workload works without juggling flags.

**Response shape note for `screenshot get`:** central API returns `{image: <signed-Supabase-URL>, metadata: {...}}`; VM-direct returns `{image: <base64-PNG>, format, encoding, width, height}`. Both deliver a complete screenshot — they encode it differently. Inspect the `encoding` field (`"base64"` for VM-direct) or check whether `image` starts with `https://` vs `iVBOR...`.

**Caveats:**
- The response cache is bypassed for VM-direct calls (no point caching local-network responses with sub-200ms latency).
- The VM agent's auth keyspace is per-VM (`vnc_password`) — your `ORGO_API_KEY` is **not** valid against the per-VM agent.
- `--vm-from <id>` still costs one central API call (the resolver). Skip it by passing `--vm-url`/`--vm-token` explicitly when you already have them.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
orgo-pp-cli computers get mock-value

# JSON for scripting and agents
orgo-pp-cli computers get mock-value --json

# Filter to specific fields
orgo-pp-cli computers get mock-value --json --select id,name,status

# Dry run — show the request without sending
orgo-pp-cli computers get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
orgo-pp-cli computers get mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-orgo -g
```

Then invoke `/pp-orgo <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add orgo orgo-pp-mcp -e ORGO_API_KEY=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/orgo-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `ORGO_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "orgo": {
      "command": "orgo-pp-mcp",
      "env": {
        "ORGO_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
orgo-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/orgo-pp-cli/config.toml`

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `ORGO_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `orgo-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $ORGO_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 'Invalid API key' on every command** — Run `orgo doctor` to see which env var the CLI read; rotate the key at https://www.orgo.ai/workspaces and `export ORGO_API_KEY=sk_live_...`
- **Computer stuck in 'creating' or 'stopping' for >5 minutes** — `orgo doctor` flags stuck computers; `orgo computers restart <id>` is the typical recovery
- **Computer status is 'suspended' after a plan downgrade** — `orgo prune --status suspended --dry-run` lists every over-quota computer; remove some or upgrade the plan to resume
- **audit/replay/cost show no data** — Those commands read from the local actions store; you must have run actions through this CLI first. Ad-hoc curl calls don't populate it.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**@orgo-ai/mcp**](https://github.com/nickvasilescu/orgo-mcp) — TypeScript
- [**orgo (Python SDK)**](https://pypi.org/project/orgo/) — Python
- [**orgo (npm SDK)**](https://www.npmjs.com/package/orgo) — TypeScript
- [**n8n-nodes-orgo**](https://www.npmjs.com/package/n8n-nodes-orgo) — TypeScript
- [**@pipedream/orgo**](https://www.npmjs.com/package/@pipedream/orgo) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
