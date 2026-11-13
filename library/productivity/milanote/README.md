# Milanote CLI

**Treat a Milanote board like an addressable canvas: native headings, lists and checkboxes, laid out on an even grid, and read back as Markdown.**

Every Milanote board, note, heading, to-do, image and comment from the command line, written through the same channel the web app uses. layout md turns a Markdown file into an evenly spaced board, outline turns a board back into Markdown, and a local mirror makes search and todos open work across every board offline.

Learn more at [Milanote](https://app.milanote.com).

Created by [@beausterling](https://github.com/beausterling) (Beau Sterling).

## Install

The recommended path installs both the `milanote-pp-cli` binary and the `pp-milanote` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install milanote
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install milanote --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install milanote --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install milanote --agent claude-code
npx -y @mvanhorn/printing-press-library install milanote --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.11 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/milanote/cmd/milanote-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/milanote-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install milanote --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-milanote --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-milanote --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install milanote --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
milanote-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/milanote-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/milanote/cmd/milanote-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS, `%APPDATA%\Claude\claude_desktop_config.json` on Windows):

```json
{
  "mcpServers": {
    "milanote": {
      "command": "milanote-pp-mcp"
    }
  }
}
```

</details>

### Remote MCP over HTTP

`milanote-pp-mcp --transport http` (or `PP_MCP_TRANSPORT=http`) serves streamable HTTP at `http://127.0.0.1:7777/mcp`. It refuses to start until `MILANOTE_MCP_HTTP_TOKEN` is set to at least 32 characters, and every request must send `Authorization: Bearer <token>`. The command below prints 64 hex characters and meets that minimum. Plain HTTP is allowed only on a loopback `--addr`; any other bind, including `:7777` (every interface), also needs `--tls-cert` and `--tls-key`. Requests with no `Origin` header are accepted, as are loopback origins (`localhost`, `127.0.0.1`, or `[::1]`, any port). Any other `Origin` is refused unless it is listed in `MILANOTE_MCP_HTTP_ALLOWED_ORIGINS`, a comma-separated list of `scheme://host[:port]` values with no path. That list is not a CORS policy; the server does not emit CORS headers.

```bash
export MILANOTE_MCP_HTTP_TOKEN="$(openssl rand -hex 32)"
milanote-pp-mcp --transport http
```

Set `MILANOTE_MCP_READ_ONLY=1` (exactly `1`) to drop mirrored commands that are not marked read-only and make the raw execute tool refuse state-changing calls. Commands marked local-write, which write only this CLI's own learn store, stay available.

## Authentication

Milanote has no API keys. The CLI uses the session cookie from a browser where you are logged in. Easiest: start Chrome with remote debugging and run chrome-login, which copies only the Milanote session cookie. Or paste it with auth login --cookie, or point MILANOTE_COOKIE_FILE at a JSON cookie export. Sessions last about a month; doctor tells you when it has expired.

## Quick Start

```bash
# Check config and auth wiring without touching the network
milanote-pp-cli doctor --dry-run

# Confirm the cookie works and find your root board id
milanote-pp-cli whoami

# Walk the board tree
milanote-pp-cli boards list --json

# Read a board as Markdown
milanote-pp-cli outline 1Ab2Cd3EfGh4iJ

# Preview a Markdown file laid out as a board before writing
milanote-pp-cli layout md notes.md --board 1Ab2Cd3EfGh4iJ --dry-run

```

## Unique Features

### Canvas layout
- **`layout md`** — Turn a Markdown file into native heading cards, rich-text notes, bullet lists and checkbox lists in evenly spaced columns.

  _Reach for this whenever a multi-section document has to land on a board looking organized, not one card at a time._

  ```bash
  milanote-pp-cli layout md outline.md --board 1Ab2Cd3EfGh4iJ --cols 3 --gutter 4
  ```
- **`layout grid`** — Re-space a board's cards (or empty its Unsorted panel) onto an even grid with equal gutters, and undo it if you don't like it.

  _Use it after edits to keep a board tidy without moving cards one by one._

  ```bash
  milanote-pp-cli layout grid 1Ab2Cd3EfGh4iJ --cols 4 --gutter 4 --dry-run
  ```
- **`images grid`** — Upload a folder of frames as an evenly spaced, numbered grid with optional captions.

  _Use it for storyboards and reference boards instead of dropping images one at a time._

  ```bash
  milanote-pp-cli images grid ./frames --board 1Ab2Cd3EfGh4iJ --cols 3 --number
  ```
- **`layout check`** — Report off-grid cards, uneven gutters, overlaps, cards stuck in Unsorted, and text that should be a heading or list.

  _Use it to verify a layout by numbers instead of a screenshot._

  ```bash
  milanote-pp-cli layout check 1Ab2Cd3EfGh4iJ --json
  ```
- **`mirror`** — Pair a Markdown file with a board so edits on either side flow to the other.

  _Use it when a document lives in both an editor and a board and must stay in step._

  ```bash
  milanote-pp-cli mirror script.md --board 1Ab2Cd3EfGh4iJ --once
  ```
- **`template apply`** — Save a board's structure as a small YAML kit and stamp out fresh copies.

  _Use it to start every project from the same organized layout._

  ```bash
  milanote-pp-cli template apply film-kit.yaml --parent 1Ab2Cd3EfGh4iJ --name 'New short' --dry-run
  ```
- **`layout csv`** — Turn spreadsheet rows such as a shot list into a numbered, evenly spaced grid of cards.

  _Use it for tabular data instead of building cards one by one._

  ```bash
  milanote-pp-cli layout csv shots.csv --board 1Ab2Cd3EfGh4iJ --title-col shot --cols 4
  ```

### Read it back
- **`outline`** — Get a board and its sub-boards back as Markdown in canvas reading order, with headings, lists and checkboxes intact.

  _Use it to read a whole board as text before editing or summarizing it._

  ```bash
  milanote-pp-cli outline 1Ab2Cd3EfGh4iJ --depth 2 --out board.md
  ```
- **`todos open`** — List every unchecked checkbox across your boards with where it lives, ready to tick off.

  _Use it to answer what is left to do without opening every board._

  ```bash
  milanote-pp-cli todos open --agent
  ```
- **`snapshot diff`** — Save a board's state and later see exactly which cards were added, moved, edited or ticked.

  _Use it to review what changed between revisions without eyeballing the canvas._

  ```bash
  milanote-pp-cli snapshot diff 1Ab2Cd3EfGh4iJ --json
  ```

### Agent-native plumbing
- **`chrome-login`** — Pull the Milanote login from a running Chrome in one step.

  _Use it to set up or refresh auth without exporting cookies by hand._

  ```bash
  milanote-pp-cli chrome-login --port 9222
  ```

## Recipes

### Push a scene outline

```bash
milanote-pp-cli layout md scenes.md --board 1Ab2Cd3EfGh4iJ --cols 3
```

Each ## becomes a heading column with its notes and checklists underneath, evenly spaced.

### Read a board back

```bash
milanote-pp-cli outline 1Ab2Cd3EfGh4iJ --out board.md
```

Markdown in reading order, checkboxes preserved.

### Just the card texts

```bash
milanote-pp-cli boards get 1Ab2Cd3EfGh4iJ --agent --select id,type,text
```

Narrow a large board payload to ids, types and plain text.

### What's left to do

```bash
milanote-pp-cli todos open --agent
```

Every unchecked task across boards with its board path and id.

## Usage

Run `milanote-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `MILANOTE_CONFIG_DIR`, `MILANOTE_DATA_DIR`, `MILANOTE_STATE_DIR`, or `MILANOTE_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `MILANOTE_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export MILANOTE_HOME=/srv/milanote
milanote-pp-cli doctor
```

Under `MILANOTE_HOME=/srv/milanote`, the four dirs resolve to `/srv/milanote/config`, `/srv/milanote/data`, `/srv/milanote/state`, and `/srv/milanote/cache`.

On Windows, set the same variable in PowerShell:

```powershell
$env:MILANOTE_HOME = "C:\srv\milanote"
milanote-pp-cli doctor
```

Under `$env:MILANOTE_HOME = "C:\srv\milanote"`, the four dirs resolve to `C:\srv\milanote\config`, `C:\srv\milanote\data`, `C:\srv\milanote\state`, and `C:\srv\milanote\cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "milanote": {
      "command": "milanote-pp-mcp",
      "env": {
        "MILANOTE_HOME": "/srv/milanote"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `MILANOTE_DATA_DIR` overrides an explicit `--home` for that kind. Use `MILANOTE_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `MILANOTE_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `milanote-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

### Local data

Reads are saved locally by default: in `--data-source auto` mode every successful read is also written to the SQLite store (`data.db`, or `data-<hash>.db` per credential) in the data directory. GET responses are cached for five minutes under `http/` in the cache directory. Credentials live in `credentials.toml` in the data directory. The learn journal lives under `learn/` in the state directory. `milanote-pp-cli doctor` prints the resolved paths, every store file, and the response-cache size.

Layout commands keep small JSON files under the state directory (`~/.local/state/milanote-pp-cli` unless `XDG_STATE_HOME` is set): `snapshots/` holds saved copies of board cards, `mirror/` remembers which card came from which part of a file, and `undo/` holds the positions `layout grid --undo` restores. `local-data purge` does not remove them; delete that directory to clear them. The layout, outline, to-do and snapshot commands read boards live rather than from the SQLite store.

None of it is encrypted. New files are created owner-only; beyond that the data relies on OS account permissions and full-disk encryption (FileVault, BitLocker, LUKS).

- **Erase it:** `milanote-pp-cli local-data purge` lists the store files, response cache, and learn journal it would delete; add `--yes` to delete them, or `--only store|cache|journal` to narrow it. Config and credentials are kept.
- **Run without a local copy:** set `MILANOTE_NO_LOCAL_DATA=1`. Reads go to the API (`--data-source live`; `--data-source local` still reads an existing store), the response cache is off (`--no-cache`), nothing new is written to the store, and `sync` and `workflow archive` refuse to run. Set it in an MCP host's `env` block and every command the server runs inherits it.
- **Choose the read source without flags:** `MILANOTE_DATA_SOURCE=auto|live|local` sets the `--data-source` default; an explicit flag wins.

## Commands

### account

Your Milanote account

- **`milanote-pp-cli account counts`** - Card and board counts used against plan limits
- **`milanote-pp-cli account history`** - Recently viewed boards (raw history ids and timestamps)
- **`milanote-pp-cli account me`** - Show the signed-in user, root board id and plan
- **`milanote-pp-cli account trash`** - Elements in your trash
- **`milanote-pp-cli account users`** - Look up collaborators by user id

### boards

Boards: the canvases that hold notes, headings, images and sub-boards

- **`milanote-pp-cli boards fetch`** - Fetch raw board payloads (elements, comments, labels) by id
- **`milanote-pp-cli boards modified`** - Last-modified times for boards
- **`milanote-pp-cli boards permissions`** - Who can see and edit a board
- **`milanote-pp-cli boards search`** - Search board titles on the server

### labels

Card labels

- **`milanote-pp-cli labels`** - List your labels


### auth


- **`milanote-pp-cli chrome-login`** - Pull the Milanote login from a running Chrome in one step.


### images


- **`milanote-pp-cli images grid`** - Upload a folder of frames as an evenly spaced, numbered grid with optional captions.


### layout


- **`milanote-pp-cli layout md`** - Turn a Markdown file into native heading cards, rich-text notes, bullet lists and checkbox lists in evenly spaced columns.

- **`milanote-pp-cli layout grid`** - Re-space a board's cards (or empty its Unsorted panel) onto an even grid with equal gutters, and undo it if you don't like it.

- **`milanote-pp-cli layout check`** - Report off-grid cards, uneven gutters, overlaps, cards stuck in Unsorted, and text that should be a heading or list.

- **`milanote-pp-cli layout csv`** - Turn spreadsheet rows such as a shot list into a numbered, evenly spaced grid of cards.


### mirror


- **`milanote-pp-cli mirror`** - Pair a Markdown file with a board so edits on either side flow to the other.


### outline


- **`milanote-pp-cli outline`** - Get a board and its sub-boards back as Markdown in canvas reading order, with headings, lists and checkboxes intact.


### snapshot


- **`milanote-pp-cli snapshot diff`** - Save a board's state and later see exactly which cards were added, moved, edited or ticked.


### template


- **`milanote-pp-cli template apply`** - Save a board's structure as a small YAML kit and stamp out fresh copies.


### todos


- **`milanote-pp-cli todos open`** - List every unchecked checkbox across your boards with where it lives, ready to tick off.




### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`milanote-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`milanote-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`milanote-pp-cli learnings list`** - Inspect taught rows
- **`milanote-pp-cli learnings forget <query>`** - Undo a teach
- **`milanote-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`milanote-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`milanote-pp-cli teach-pattern`** - Install a query/resource template up front
- **`milanote-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `MILANOTE_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `milanote-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
milanote-pp-cli boards search --query example-value

# JSON for scripting and agents
milanote-pp-cli boards search --query example-value --json
# Filter to specific fields by name
milanote-pp-cli boards search --query example-value --json --select <field>[,<field>...]

# Dry run — show the request without sending
milanote-pp-cli boards search --query example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
milanote-pp-cli boards search --query example-value --agent
```

`--csv` prefixes a leading `'` on cells and header keys that start with `=`, `+`, `-`, `@`, a tab, or a line break (including fullwidth `＝＋－＠`) so a spreadsheet does not run them as formulas. Plain signed numbers such as `-5` and `+2.5e3` are left unchanged.

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
milanote-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `milanote-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/milanote-pp-cli/config.toml` on macOS/Linux and `%USERPROFILE%\.config\milanote-pp-cli\config.toml` on Windows; `--home`, `MILANOTE_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

`--ca-cert`, `MILANOTE_CA_CERT`, or `ca_cert` adds a PEM CA bundle to the system trust roots. `--tls-server-name`, `MILANOTE_TLS_SERVER_NAME`, or `tls_server_name` is the name checked against the certificate when the dialed address does not match it. Precedence is flag, then environment, then the config file. A config file other users can change does not supply these settings. Certificate verification failures are not retried.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `MILANOTE_SESSION_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `milanote-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Card limits

Milanote's free plan allows 100 cards, and notes, to-do lists, images and links all count (boards, columns and comments do not). Commands that add cards can use up that allowance quickly: run them with `--dry-run` first, and check `milanote-pp-cli usage` for where you stand. Trashing cards gives the allowance back.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `milanote-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $MILANOTE_SESSION_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized on every command** — The session cookie expired; log in to Milanote in the browser and re-run auth login --cookie
- **socket.io handshake returns 400** — Load-balancer stickiness was lost; retry the command (the client re-handshakes with a fresh cookie jar)
- **Writes rejected on a free account** — Free plans stop at 100 cards; run usage to see the count

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://app.milanote.com/api/boards/modified
- Capture coverage: 20 API entries from 20 total network entries
- Reachability: standard_http (65% confidence)
- Protocols: rest_json (75% confidence)
- Candidate command ideas: create_presign — Derived from observed POST /api/elements/attachments/presign traffic.; list_boards — Derived from observed GET /api/boards traffic.; list_counts — Derived from observed GET /api/users/me/counts traffic.; list_custom_templates_init — Derived from observed GET /api/users/me/custom-templates-init traffic.; list_history — Derived from observed GET /api/users/me/history traffic.; list_icons — Derived from observed GET /api//icons traffic.; list_labels — Derived from observed GET /api/labels traffic.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**milanote-api-mcp-server**](https://github.com/cotletdev/milanote-api-mcp-server) — Python
- [**Milanote-Unofficial-API**](https://github.com/serhanceetin/Milanote-Unofficial-API) — Python
- [**milanote-api**](https://github.com/setobox/milanote-api) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
