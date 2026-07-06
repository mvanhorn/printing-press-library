# Agilix Dawn CLI

**The only CLI for the modern Agilix Dawn API — browse and AUTHOR courses (sections, lessons, quizzes, publish), manage enrollments, export curriculum and rosters, and mirror your catalog offline.**

Every existing Agilix library wraps the legacy Buzz/DLAP API; this CLI talks to the modern same-origin Dawn REST API that the web app itself uses. It turns Dawn's search DSL into ergonomic commands, renders a course's entire section/instruction/interaction tree that the UI only paginates, and keeps a local SQLite mirror so 'course stats', 'catalog diff', and 'purchase reconcile' work offline.

## Install

The recommended path installs both the `agilix-dawn-pp-cli` binary and the `pp-agilix-dawn` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install agilix-dawn
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install agilix-dawn --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install agilix-dawn --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install agilix-dawn --agent claude-code
npx -y @mvanhorn/printing-press-library install agilix-dawn --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/agilix-dawn/cmd/agilix-dawn-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/agilix-dawn-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install agilix-dawn --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-agilix-dawn --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-agilix-dawn --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install agilix-dawn --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/agilix-dawn-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `AGILIX_DAWN_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/agilix-dawn/cmd/agilix-dawn-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "agilix-dawn": {
      "command": "agilix-dawn-pp-mcp",
      "env": {
        "AGILIX_DAWN_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Dawn authenticates with a raw token in the Authorization header (no 'Bearer ' prefix). Set AGILIX_DAWN_TOKEN to a Dawn API-user token (recommended, durable) or a session token from your logged-in browser. The token is passed as the Authorization header on every request.

## Quick Start

```bash
# verify the CLI is wired before hitting the API
agilix-dawn-pp-cli doctor --dry-run

# confirm the tenant and reachability (public endpoint)
agilix-dawn-pp-cli config

# browse the catalog
agilix-dawn-pp-cli concept list --search '{"limit":25}'

# see a whole course's structure at once
agilix-dawn-pp-cli course tree c_f4bff87c0cab456984f2860af3e427d0

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Course structure insight
- **`course tree`** — Render a course's full section → instruction → interaction hierarchy as a tree.

  _Reach for this when an agent needs the whole shape of a course at once instead of paging the UI._

  ```bash
  agilix-dawn-pp-cli course tree c_f4bff87c0cab456984f2860af3e427d0 --json
  ```
- **`course stats`** — Aggregate total instruction time, points, and section/instruction/interaction counts for a course.

  _Use to answer 'how big / how many hours is this course' without opening it._

  ```bash
  agilix-dawn-pp-cli course stats c_f4bff87c0cab456984f2860af3e427d0 --json
  ```
- **`course outline`** — Flatten a course's section/instruction tree into a Markdown or CSV curriculum.

  _Use to export a course's curriculum for documentation or compliance._

  ```bash
  agilix-dawn-pp-cli course outline c_f4bff87c0cab456984f2860af3e427d0 --format md
  ```

### Local state that compounds
- **`catalog diff`** — Show what changed in the catalog since the last local sync (new/removed courses, price/status/title changes).

  _Use to detect catalog changes over time; requires a prior sync._

  ```bash
  agilix-dawn-pp-cli catalog diff --json
  ```
- **`roster export`** — Export users (id, name, email, status, verified) to CSV for reporting.

  _Use to pull a roster for spreadsheets or state reporting._

  ```bash
  agilix-dawn-pp-cli roster export --format csv
  ```
- **`purchase reconcile`** — Join purchases against user records to show who paid for what.

  _Use to reconcile Stripe-backed purchases against enrolled users._

  ```bash
  agilix-dawn-pp-cli purchase reconcile --json
  ```

### Course authoring
- **`edit concept`** — Update a course's title, description, price, or status (preview by default; --apply to write).

  _Use to change course details scriptably; it previews the change and only writes with --apply._

  ```bash
  agilix-dawn-pp-cli edit concept c_216daf6f76024e43b03b229895686555 --title "New Title"
  ```
- **`edit section add`** — Add, rename, or remove sections in a course (preview by default; --apply to write).

  _Use to build a course's structure from the CLI without touching the whole document by hand._

  ```bash
  agilix-dawn-pp-cli edit section add c_216daf6f76024e43b03b229895686555 --title "Module 1"
  ```
- **`edit instruction add`** — Add or remove instructions (lessons) within a section (preview by default; --apply to write).

  _Use to add lessons to a section scriptably; previews first, writes with --apply._

  ```bash
  agilix-dawn-pp-cli edit instruction add c_216daf6f76024e43b03b229895686555 s_dd819fbaf2a642ffa9474c7f38e7f318 --title "Lesson 1"
  ```
- **`edit interaction add-choice`** — Add a typed multiple-choice question to a lesson (options + correct answers), preview by default; --apply to write.

  _Use to script multiple-choice quiz authoring; previews first, writes with --apply._

  ```bash
  agilix-dawn-pp-cli edit interaction add-choice c_216daf6f76024e43b03b229895686555 s_1 i_1 --body "2+2=?" --option 3 --option 4 --answer 2
  ```
- **`edit interaction add-response`** — Add a typed short-answer question to a lesson (accepted answers or coach review), preview by default; --apply to write.

  _Use to script short-answer quiz authoring; previews first, writes with --apply._

  ```bash
  agilix-dawn-pp-cli edit interaction add-response c_216daf6f76024e43b03b229895686555 s_1 i_1 --body "Capital of Idaho?" --answer Boise
  ```
- **`resource list`** — List the file resources (video, images, PDFs, SCORM) attached to a course, with sizes and content types.

  _Use to audit a course's media footprint. Note: uploading files is a browser-only signed-URL flow this CLI does not replicate._

  ```bash
  agilix-dawn-pp-cli resource list c_f4bff87c0cab456984f2860af3e427d0 --json
  ```

### Enrollment management
- **`enrollment group create`** — List, create, or delete enrollment groups for a course (preview by default; --apply to write).

  _Use to organize learners into cohorts before enrolling them._

  ```bash
  agilix-dawn-pp-cli enrollment group list c_216daf6f76024e43b03b229895686555 --json
  ```
- **`enrollment add`** — Enroll or unenroll a user in a course by adding/removing them from an enrollment group (preview by default; --apply to write).

  _Use to script student enrollment; previews first, writes only with --apply._

  ```bash
  agilix-dawn-pp-cli enrollment members g_05d557b932364b66a310316a0e050ba7 --json
  ```

## Recipes

### Export the big course's curriculum

```bash
agilix-dawn-pp-cli course outline c_f4bff87c0cab456984f2860af3e427d0 --format md
```

Flattens a 34-section course into a Markdown syllabus.

### Total seat-time of a course

```bash
agilix-dawn-pp-cli course stats c_f4bff87c0cab456984f2860af3e427d0 --json --select total_duration_hours,instructions,interactions
```

Sums instruction durations and counts locally, then narrows the output.

### Roster to CSV

```bash
agilix-dawn-pp-cli roster export --format csv
```

Pulls all users into a clean CSV for reporting.

### Search the catalog

```bash
agilix-dawn-pp-cli concept list --search '{"query":"title:driver","limit":10}' --agent --select id,title,status
```

Runs a Lucene title search and narrows to the fields an agent needs.

## Usage

Run `agilix-dawn-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `AGILIX_DAWN_CONFIG_DIR`, `AGILIX_DAWN_DATA_DIR`, `AGILIX_DAWN_STATE_DIR`, or `AGILIX_DAWN_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `AGILIX_DAWN_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export AGILIX_DAWN_HOME=/srv/agilix-dawn
agilix-dawn-pp-cli doctor
```

Under `AGILIX_DAWN_HOME=/srv/agilix-dawn`, the four dirs resolve to `/srv/agilix-dawn/config`, `/srv/agilix-dawn/data`, `/srv/agilix-dawn/state`, and `/srv/agilix-dawn/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "agilix-dawn": {
      "command": "agilix-dawn-pp-mcp",
      "env": {
        "AGILIX_DAWN_HOME": "/srv/agilix-dawn"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `AGILIX_DAWN_DATA_DIR` overrides an explicit `--home` for that kind. Use `AGILIX_DAWN_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `AGILIX_DAWN_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `agilix-dawn-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### concept

Browse the course/content catalog (Dawn concepts)

- **`agilix-dawn-pp-cli concept get`** - Get one concept (course) by id, including full section/instruction structure
- **`agilix-dawn-pp-cli concept list`** - List catalog concepts (courses/content). Uses the Dawn search DSL.

### config

Show tenant configuration

- **`agilix-dawn-pp-cli config`** - Show the tenant's public configuration (name, root org, version, auth providers, payment)

### conversation

List conversations/messages

- **`agilix-dawn-pp-cli conversation`** - List conversations. Uses the Dawn search DSL.

### organization

List organizations/domains in the tenant

- **`agilix-dawn-pp-cli organization`** - List organizations. Uses the Dawn search DSL.

### progress

List learner progress records

- **`agilix-dawn-pp-cli progress`** - List learner progress. Uses the Dawn search DSL (e.g. query:"user:u_... AND NOT state:preview").

### purchase

List purchases (Stripe-backed commerce records)

- **`agilix-dawn-pp-cli purchase`** - List purchases. Uses the Dawn search DSL.

### user

Look up users in the tenant

- **`agilix-dawn-pp-cli user list`** - List/search users. Uses the Dawn search DSL.
- **`agilix-dawn-pp-cli user me`** - Show the authenticated user (whoami)


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
agilix-dawn-pp-cli concept list

# JSON for scripting and agents
agilix-dawn-pp-cli concept list --json

# Filter to specific fields
agilix-dawn-pp-cli concept list --json --select id,name,status

# Dry run — show the request without sending
agilix-dawn-pp-cli concept list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
agilix-dawn-pp-cli concept list --agent
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
agilix-dawn-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `agilix-dawn-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/agilix-dawn-pp-cli/config.toml`; `--home`, `AGILIX_DAWN_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `AGILIX_DAWN_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `agilix-dawn-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `agilix-dawn-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $AGILIX_DAWN_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **403 Forbidden with {description, message, requestId, status}** — Set AGILIX_DAWN_TOKEN to a valid Dawn token (API-user token recommended).
- **totalMatches is non-zero but no rows returned** — Dawn requires the search wrapper; pass --search '{"limit":25}' rather than relying on top-level params.
- **Wrong tenant** — Override the base URL in ~/.config/agilix-dawn-pp-cli/config.toml (default is drivered.agilixdawn.com).

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**beneggett/agilix**](https://github.com/beneggett/agilix) — Ruby
- [**StrongMind/agilix-buzz-client**](https://github.com/StrongMind/agilix-buzz-client) — Ruby

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
