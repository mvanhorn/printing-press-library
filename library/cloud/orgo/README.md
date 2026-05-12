# Orgo CLI

**One Go binary, two control surfaces, and the audit ledger Orgo doesn't otherwise have.**

- **Computer use** — drive a virtual desktop at the pixel level: `bash`, `screenshot`, `click`, `type`, `key`, `scroll`, `drag`, `exec`. Works for any app, native or web.
- **Browser use (Chrome)** — drive the page at the **DOM level** with element refs, find-by-intent, JavaScript evaluation, and console/network inspection. 16 subcommands; the bridge auto-deploys on first use.
- **Audit + fleet + cost** — every action lands in a local SQLite store, so `replay`, `audit`, `grep`, `fleet`, `idle`, `oversized`, `prune`, and `cost` see what no live Orgo API call can.

Everything also ships as an **MCP server** (65 tools), so one install gets your agent the full surface from Claude Desktop, Claude Code, or any MCP host.

Learn more about Orgo at [orgo.ai](https://orgo.ai).

---

## Install

```bash
# Recommended — installs both the CLI binary and the pp-orgo agent skill
npx -y @mvanhorn/printing-press install orgo

# CLI only (no agent skill)
npx -y @mvanhorn/printing-press install orgo --cli-only
```

### Pre-built binary

Download for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/orgo-current).

```bash
# macOS — clear Gatekeeper quarantine
xattr -d com.apple.quarantine ./orgo-pp-cli

# Unix — mark executable
chmod +x ./orgo-pp-cli
```

<!-- pp-hermes-install-anchor -->
### Install for Hermes

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-orgo --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-orgo --force
```

### Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-orgo skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-orgo. The skill defines how its required CLI can be installed.
```

---

## Authentication

Bearer auth via `ORGO_API_KEY` (`sk_live_...`). Get a key at https://www.orgo.ai/workspaces.

```bash
export ORGO_API_KEY=sk_live_...
```

The CLI reads `ORGO_API_KEY` on every invocation, so rotating keys requires no restart. `orgo-pp-cli doctor` probes the key against an authenticated endpoint and tells you definitively whether it's valid — without printing the value.

---

## Quick Start

```bash
# 1. Verify install + auth + connectivity
orgo-pp-cli doctor

# 2. Discover what you have
#    (workspaces list returns workspaces with nested .desktops arrays —
#     there's no top-level `computers list`; use jq to pick out computers)
orgo-pp-cli workspaces list
orgo-pp-cli workspaces list --agent | jq '.results.projects[].desktops[] | {id, name, status}'

# 3. Provision a fresh desktop
orgo-pp-cli computers create \
    --workspace-id <workspace-uuid> \
    --name agent-1 --cpu 2 --ram 8

# 4. Drive it (pixel-based computer use)
orgo-pp-cli computers bash execute <id> --command 'ls /home/orgo'
orgo-pp-cli computers screenshot get <id>      # signed URL (central) or base64 inline (VM-direct)

# Need a real PNG on disk right away? Use chrome (decodes base64 for you):
orgo-pp-cli chrome screenshot <id> --out /tmp/page.png

# 5. Drive Chrome inside it (DOM-aware browser use)
orgo-pp-cli chrome navigate <id> --url https://example.com
orgo-pp-cli chrome read-page <id> --filter interactive
orgo-pp-cli chrome find <id> --query "learn more"

# 6. Fleet hygiene + audit
orgo-pp-cli fleet
orgo-pp-cli audit --since 1h --agent
```

---

## Three ways to use this

| Mode | Setup | Best for |
|---|---|---|
| **Direct CLI** | `export ORGO_API_KEY=...` | Humans driving Orgo from a terminal, scripts, cron, CI |
| **MCP server** | `claude mcp add orgo -- orgo-pp-mcp` | AI agents in Claude Desktop, Claude Code, or any MCP host — 65 tools from one server |
| **In-VM agent** | Set `ORGO_VM_URL` + `ORGO_VM_TOKEN` | Agents born inside an Orgo VM that want ~3× lower per-call latency via VM-direct routing |

All three modes use the same binary, the same commands, the same flags. Pick whichever fits your stack.

---

## Two control surfaces

The CLI exposes two complementary ways to drive a virtual desktop. Both work in the same VM session — you can mix them freely.

| | **Computer use** | **Browser use** (`chrome`) |
|---|---|---|
| **Granularity** | Pixel-level, whole desktop | DOM-level (accessibility tree with refs) |
| **Works for** | Any app — terminals, native UIs, games, file managers | Web pages only |
| **Token cost (for agents)** | High (re-screenshot between actions) | 1-2 orders of magnitude lower (refs are stable) |
| **Primitives** | 8 commands | 16 commands |
| **Latency floor** | ~0.16s VM-direct, ~0.55s central | ~0.48s VM-direct, ~1.23s central |

**Rule of thumb**: prefer `chrome` for any web workflow. Reach for computer-use for native apps, terminals, or when the DOM isn't useful (canvases, maps, games, OS-level UIs).

### Computer use — 8 pixel primitives

```bash
orgo-pp-cli computers bash execute      <id> --command 'whatever'
orgo-pp-cli computers exec execute-python <id> --code 'print(1+1)'
orgo-pp-cli computers screenshot get    <id>                        # returns signed URL (central) or base64 inline (VM-direct)
orgo-pp-cli computers click mouse       <id> --x 640 --y 360 [--button right] [--double]
orgo-pp-cli computers type text         <id> --text "hello"
orgo-pp-cli computers key press         <id> --key Enter        # or ctrl+c, alt+F4, etc.
orgo-pp-cli computers scroll scroll     <id> --direction down --amount 3
orgo-pp-cli computers drag mouse        <id> --start-x 100 --start-y 100 --end-x 500 --end-y 400
```

All 8 inherit **VM-direct routing** — add `--vm-from <id>` (or `--vm-url`/`--vm-token`) for ~70% lower per-call latency.

Plus lifecycle:

```bash
orgo-pp-cli computers create   --workspace-id <ws> --name foo --cpu 2 --ram 8
orgo-pp-cli computers start    <id>                # / stop / restart
orgo-pp-cli computers clone    <id>                # snapshot + new VM
orgo-pp-cli computers resize   <id> --vcpus 4 --mem-gb 16
orgo-pp-cli computers move     <id> --project-id <other-ws>
orgo-pp-cli computers wait wait <id> --duration 5  # sleep in scripts
orgo-pp-cli computers get      <id>                # full metadata including url + vnc_password
orgo-pp-cli computers delete   <id>
```

There's no top-level `computers list` — use `workspaces list` and pull the nested `desktops` arrays:

```bash
orgo-pp-cli workspaces list --agent | jq '.results.projects[].desktops[] | {id, name, status, url}'
```

### Browser use — `chrome` subcommand (16 tools)

Drive Chrome inside the VM at the **DOM level**: accessibility tree with element refs, click by ref, type, evaluate JavaScript, inspect console and network. Much faster, cheaper, and more reliable than pixel-based screenshot+click loops for any web workflow.

A small zero-dependency Node bridge is shipped embedded in the binary. It auto-deploys to `/tmp/orgo-chrome-bridge.js` inside the VM on first use and listens on `127.0.0.1:7331`; subsequent calls reuse it. The embedded bridge's SHA256 is checked on every call — a CLI upgrade with a new bridge auto-redeploys on long-lived VMs.

```bash
# Cold-start: bridge auto-deploys on first call (~6s including bridge launch).
orgo-pp-cli chrome navigate <id> --url https://example.com

# Read the page as an accessibility tree — cheap, fast, gives you stable element refs.
orgo-pp-cli chrome read-page <id> --filter interactive

# Find elements by intent (returns up to 20 matches with refs).
orgo-pp-cli chrome find <id> --query "search bar"

# Click by ref (preferred — resilient to layout shifts).
orgo-pp-cli chrome click <id> --ref ref_3

# Set form values directly — no focus-then-type dance.
orgo-pp-cli chrome form-input <id> --ref ref_7 --value "user@example.com"

# Type text or press shortcuts.
orgo-pp-cli chrome type <id> --text "hello world"
orgo-pp-cli chrome type <id> --key Enter
orgo-pp-cli chrome type <id> --key ctrl+a

# Evaluate JavaScript in the page (no 'return' — just the expression).
orgo-pp-cli chrome evaluate <id> --expression "document.title"

# Save a real PNG (or print base64 inline if no --out).
orgo-pp-cli chrome screenshot <id> --out /tmp/page.png

# Inspect buffered console messages and network requests.
orgo-pp-cli chrome console <id> --only-errors
orgo-pp-cli chrome network <id> --url-pattern "/api/"

# Tab management.
orgo-pp-cli chrome tabs <id>
orgo-pp-cli chrome new-tab <id> --url https://news.ycombinator.com
orgo-pp-cli chrome switch-tab <id> --target-id <tid>

# Viewport.
orgo-pp-cli chrome resize <id> --width 1920 --height 1080
orgo-pp-cli chrome scroll <id> --direction down --amount 5
```

**Full command set (16):** `navigate`, `tabs`, `new-tab`, `switch-tab`, `read-page`, `find`, `page-text`, `screenshot`, `click`, `type`, `form-input`, `scroll`, `evaluate`, `console`, `network`, `resize`.

**VM-direct routing inherited transparently.** Chrome calls bottom out on `/computers/{id}/bash`, which is itself VM-bypassable — so `chrome <verb> --vm-from <id>` cuts latency the same way it does for `computers bash execute`.

**Requirements (inside the VM):** Chrome (any of `google-chrome`, `chromium`, `chromium-browser`), Node 18+, an X display. All present by default on Orgo VMs.

---

## Files and workspaces

```bash
orgo-pp-cli files list     --project-id <ws-id> [--desktop-id <vm-id>]
orgo-pp-cli files upload   --project-id <ws-id> --file ./data.csv
orgo-pp-cli files download --id <file-id>            # prints a signed URL; curl it to save
orgo-pp-cli files export   --desktop-id <vm-id> --path Desktop/results.txt
orgo-pp-cli files delete   --id <file-id>

orgo-pp-cli workspaces list
orgo-pp-cli workspaces get    <id>
orgo-pp-cli workspaces create --name "my project"
orgo-pp-cli workspaces delete <id>
```

---

## Unique features (no other Orgo tool has these)

### Audit ledger

Every screenshot, bash command, click, exec, and key press the CLI runs lands in a local SQLite store. These three commands read from it:

- **`replay`** — Generate a self-contained static HTML timeline of every action your agent ran on a computer. Reach for it to audit, share, or debug — no live API roundtrips, just a single HTML file you can email or attach to an incident.

  ```bash
  orgo-pp-cli replay <id> --since 1h --out replay.html
  ```

- **`audit`** — Chronological table of every CLI-driven action in a time window, scoped by workspace, FTS-searchable. Reach for it when a customer asks "what did the agent do this week" or you need a regression bundle.

  ```bash
  orgo-pp-cli audit --workspace prod --since 7d --agent --select timestamp,computer,kind,summary
  ```

- **`grep`** — FTS5 search over historical bash commands, Python exec code, and click coordinates. Reach for it when you need to find a specific command but don't remember which computer or when.

  ```bash
  orgo-pp-cli grep 'pip install pandas' --type bash --since 30d
  ```

### Fleet stewardship

- **`fleet`** — Cross-workspace health rollup: suspended (over-quota), errored, stuck-creating, stuck-stopping, plus an API-key validity probe. One call replaces walking every workspace by hand.

  ```bash
  orgo-pp-cli fleet --agent
  ```

- **`idle`** — Running computers sorted by hours-since-last-CLI-action. Every idle computer with auto-stop disabled is a known leak.

  ```bash
  orgo-pp-cli idle --threshold-hours 24
  ```

- **`oversized`** — Computers with CPU ≥ 4 cores or RAM ≥ 16 GB whose last CLI-recorded action is older than the threshold and whose auto-stop is disabled.

  ```bash
  orgo-pp-cli oversized --min-cores 4 --idle-days 7
  ```

- **`prune`** — Cross-workspace status-filtered batch delete. Dry-run by default; pass `--yes` to actually delete.

  ```bash
  orgo-pp-cli prune --status suspended,error --older-than 7d --dry-run
  orgo-pp-cli prune --status suspended,error --older-than 7d --yes
  ```

- **`cost`** — Reconstructs per-computer running-hours from local action timestamps + observed status transitions, multiplies by per-tier rate, sums by workspace. `--forecast` projects month-end burn.

  ```bash
  orgo-pp-cli cost --workspace prod --since 30d --forecast
  ```

---

## VM-Direct Routing (latency optimization)

Computer-use commands (`bash`, `click`, `type`, `key`, `scroll`, `drag`, `exec`, `screenshot`) — **plus every chrome subcommand** — can bypass the central API and talk directly to the per-VM agent. Measured win on a sample machine: **~0.55s → ~0.16s per call (~70%)** for raw bash, **~1.23s → ~0.48s** for chrome roundtrips. The central API's proxy hop and (for screenshots) Supabase upload step are skipped.

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

# Env-injected (no flags needed) — recommended for long-running in-VM loops.
export ORGO_VM_URL=http://1.2.3.4:36100
export ORGO_VM_TOKEN=<vnc_password>
orgo-pp-cli computers click mouse <id> --x 640 --y 360
orgo-pp-cli chrome read-page <id> --filter interactive
```

**Which commands bypass:** `bash`, `click`, `type`, `key`, `scroll`, `drag`, `exec`, `screenshot`. Every `chrome` subcommand inherits this transparently because it bottoms out on the bash channel. Everything else (workspace/computer management, files, fleet ops, audit, etc.) continues to use the central API — those endpoints don't exist on the per-VM agent. Non-bypassable commands run normally when `--vm-*` is set, so a mixed workload works without juggling flags.

**Response shape note for `computers screenshot get`:** central API returns `{image: <signed-Supabase-URL>, metadata: {...}}`; VM-direct returns `{image: <base64-PNG>, format, encoding, width, height}`. Both deliver a complete screenshot — they encode it differently. Inspect the `encoding` field or check whether `image` starts with `https://` vs `iVBOR...`.

**Caveats:**
- The response cache is bypassed for VM-direct calls (no point caching local-network sub-200ms responses).
- The VM agent's auth keyspace is per-VM (`vnc_password`) — your `ORGO_API_KEY` is **not** valid against the per-VM agent.
- `--vm-from <id>` still costs one central API call (the resolver). Skip it by passing `--vm-url`/`--vm-token` explicitly when you already have them.

---

## Recipes

### Recipe 1 — Provision and run a one-off script

```bash
# Pick a workspace
WS=$(orgo-pp-cli workspaces list --agent | jq -r '.results.projects[0].id')

# Spin up; --agent envelopes POST results under .data
ID=$(orgo-pp-cli computers create --workspace-id "$WS" --name task-$$ --cpu 2 --ram 4 --agent | jq -r '.data.id')

# Wait for it to be ready (in seconds), then work
orgo-pp-cli computers wait wait "$ID" --duration 10
orgo-pp-cli computers exec execute-python "$ID" --code '
import urllib.request, json
data = json.load(urllib.request.urlopen("https://httpbin.org/json"))
print(data["slideshow"]["title"])'

# Clean up
orgo-pp-cli computers stop "$ID"
```

### Recipe 2 — Web automation with login

```bash
ID=<computer-id>

# Open the target site, read its actionable elements
orgo-pp-cli chrome navigate $ID --url https://app.example.com/login
orgo-pp-cli chrome read-page $ID --filter interactive --agent

# Find inputs by intent
orgo-pp-cli chrome find $ID --query "email" --agent
orgo-pp-cli chrome find $ID --query "password" --agent

# Fill the form directly (no focus-then-type)
orgo-pp-cli chrome form-input $ID --ref ref_2 --value "agent@example.com"
orgo-pp-cli chrome form-input $ID --ref ref_3 --value "$EXAMPLE_PASSWORD"

# Find and click submit
orgo-pp-cli chrome find $ID --query "sign in" --agent
orgo-pp-cli chrome click $ID --ref ref_5

# Verify post-login state and capture evidence
orgo-pp-cli chrome evaluate $ID --expression "location.pathname"
orgo-pp-cli chrome screenshot $ID --out /tmp/logged-in.png
```

### Recipe 3 — Replay an incident

```bash
# A customer says their agent crashed at 2 PM yesterday on computer abc123.
# Build a single self-contained HTML timeline of everything the CLI saw.
orgo-pp-cli replay abc123 --since 18h --out /tmp/incident.html
open /tmp/incident.html

# Or grep for the specific command they reported running
orgo-pp-cli grep "rm -rf" --type bash --since 24h --agent
```

### Recipe 4 — Daily fleet cron

```bash
#!/usr/bin/env bash
# /etc/cron.daily/orgo-hygiene.sh

# Fail-fast if auth or connectivity is broken — exit 4 if creds bad, 5 on API
orgo-pp-cli doctor --agent || exit $?

# Surface anything obviously broken
orgo-pp-cli fleet --agent > /tmp/fleet.json

# Stop computers that have been idle > 24h with auto-stop disabled.
# `idle --agent` returns a bare array of computers (not enveloped).
orgo-pp-cli idle --threshold-hours 24 --agent | jq -r '.[].id' \
  | xargs -n1 -I{} orgo-pp-cli computers stop {}

# Email a cost forecast
orgo-pp-cli cost --since 30d --forecast --agent > /tmp/cost.json
```

### Recipe 5 — Hot-loop agent inside the VM

```bash
# Resolve once, then go direct for everything in this session.
# GET responses envelope payload under .results; `computers get` returns
# both url and vnc_password in one call, so a single GET suffices.
ORGO_VM_URL=$(orgo-pp-cli computers get $ID --agent | jq -r '.results.url')
ORGO_VM_TOKEN=$(orgo-pp-cli computers get $ID --agent | jq -r '.results.vnc_password')
export ORGO_VM_URL ORGO_VM_TOKEN

# Now every call skips the central API. Latency drops ~3×.
for url in $(cat urls.txt); do
    orgo-pp-cli chrome navigate $ID --url "$url"
    orgo-pp-cli chrome page-text $ID --agent > "scrapes/$(basename $url).txt"
done
```

---

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/orgo-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `ORGO_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`).

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle, install the MCP binary and configure it manually.

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

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

After install, the agent has 65 tools across:

- **Typed HTTP endpoint tools** (~33) — `workspaces_*`, `computers_*`, `files_*`, etc.
- **`chrome_*`** (16) — DOM-aware browser automation
- **Transcendence tools** (8) — `audit`, `grep`, `replay`, `fleet`, `idle`, `oversized`, `prune`, `cost`
- **`context`** — domain discovery; agents typically call this first
- **`search`, `sql`** — full-text and SQL over the local action ledger

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-orgo -g
```

Then invoke `/pp-orgo <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

```bash
claude mcp add orgo orgo-pp-mcp -e ORGO_API_KEY=<your-token>
```

</details>

---

## Output formats

```bash
# Human-readable table in a terminal, JSON when piped
orgo-pp-cli computers get <id>

# Force JSON
orgo-pp-cli computers get <id> --json

# Filter to specific fields (works on JSON + bare-array + envelope responses)
orgo-pp-cli computers get <id> --json --select id,name,status

# Dry run — show the request without sending
orgo-pp-cli computers get <id> --dry-run

# Agent mode — JSON + compact + no prompts + no color, all in one flag
orgo-pp-cli computers get <id> --agent
```

## Agent mode

Designed for AI agent and CI consumption:

- **Non-interactive** — never prompts, every input is a flag or env var
- **Pipeable** — `--json` to stdout, errors to stderr
- **Filterable** — `--select id,name` returns only fields you need
- **Previewable** — `--dry-run` shows the request without sending
- **Explicit retries** — `--idempotent` for create-retries, `--ignore-missing` for delete-retries (both make repeat calls a no-op success)
- **Confirmable** — `--yes` for destructive ops, `--no-input` to fail rather than prompt
- **Piped input** — write commands accept structured JSON on stdin when their help lists `--stdin`
- **Offline-friendly** — sync/search commands can use the local SQLite store

**Exit codes:**

| Code | Meaning |
|---|---|
| 0 | success |
| 2 | usage error |
| 3 | not found |
| 4 | auth error |
| 5 | API error |
| 7 | rate limited |
| 10 | config error |

---

## Health check

```bash
orgo-pp-cli doctor
```

Probes credentials against an authenticated endpoint, reports config source, checks network reachability, and surfaces fleet-level issues (suspended, errored, stuck) so you don't have to ask three questions to find one answer.

## Configuration

Config file: `~/.config/orgo-pp-cli/config.toml` (optional — env vars are usually enough).

| Env var | Required | Description |
|---|---|---|
| `ORGO_API_KEY` | Yes | Bearer credential for the central API |
| `ORGO_VM_URL` | No | When set, routes computer-use + chrome calls directly to this per-VM agent |
| `ORGO_VM_TOKEN` | No | Bearer for the per-VM agent (the computer's `vnc_password`) |

---

## Troubleshooting

**Authentication errors (exit 4)**
- Run `orgo-pp-cli doctor` — it tells you which env var the CLI read and whether the key is valid
- Verify the env var is set: `echo $ORGO_API_KEY`
- Rotate the key at https://www.orgo.ai/workspaces and `export ORGO_API_KEY=sk_live_...`

**Not found (exit 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

**Computer stuck in 'creating' or 'stopping' for >5 minutes**
- `orgo-pp-cli fleet` lists every stuck computer with rollup
- `orgo-pp-cli computers restart <id>` is the typical recovery

**Computer status is 'suspended' after a plan downgrade**
- `orgo-pp-cli prune --status suspended --dry-run` lists every over-quota computer
- Remove some or upgrade your plan to resume

**`audit` / `replay` / `cost` show no data**
- They read from the local actions ledger. You must have run actions through this CLI first.
- Ad-hoc `curl` calls against the API don't populate the ledger.

**`chrome` first-call timeout**
- The bridge auto-deploys on the first call (~6s). If it times out, run `orgo-pp-cli computers bash execute <id> --command 'cat /tmp/orgo-chrome-bridge.log | tail -20'` to see why the bridge failed to start.
- Confirm Chrome + Node 18+ + an X display exist in the VM (all present by default).

**VM-direct call fails with HTTP 401**
- The per-VM agent uses the computer's `vnc_password` as its bearer, not `ORGO_API_KEY`.
- Fetch it with `orgo-pp-cli computers vnc-password get <id>` (or use `--vm-from <id>` to resolve automatically).

---

## Sources & inspiration

This CLI was built by studying these projects:

- [**orgo-chrome-mcp**](https://github.com/nickvasilescu/orgo-chrome-mcp) — TypeScript MCP server, the original Chrome DOM bridge whose 16 tools we ported into `orgo-pp-cli`'s `chrome` subcommand set
- [**orgo (Python SDK)**](https://pypi.org/project/orgo/)
- [**orgo (npm SDK)**](https://www.npmjs.com/package/orgo)
- [**n8n-nodes-orgo**](https://www.npmjs.com/package/n8n-nodes-orgo)
- [**@pipedream/orgo**](https://www.npmjs.com/package/@pipedream/orgo)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press).
