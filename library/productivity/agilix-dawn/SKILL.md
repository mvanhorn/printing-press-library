---
name: pp-agilix-dawn
description: "The only CLI for the modern Agilix Dawn API — render a course's full structure, export curriculum and rosters, and mirror your catalog offline. Trigger phrases: `list dawn courses`, `show the course structure`, `export the curriculum`, `reconcile dawn purchases`, `export the roster`, `use agilix-dawn`, `run agilix-dawn`."
author: "Ryan Gravette"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - agilix-dawn-pp-cli
    install:
      - kind: go
        bins: [agilix-dawn-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/agilix-dawn/cmd/agilix-dawn-pp-cli
---

# Agilix Dawn — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `agilix-dawn-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install agilix-dawn --cli-only
   ```
2. Verify: `agilix-dawn-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/agilix-dawn/cmd/agilix-dawn-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Every existing Agilix library wraps the legacy Buzz/DLAP API; this CLI talks to the modern same-origin Dawn REST API that the web app itself uses. It turns Dawn's search DSL into ergonomic commands, renders a course's entire section/instruction/interaction tree that the UI only paginates, and keeps a local SQLite mirror so 'course stats', 'catalog diff', and 'purchase reconcile' work offline.

## When to Use This CLI

Use this CLI to browse and export the Agilix Dawn course catalog and its deep content structure, look up users and organizations, and reconcile Stripe-backed purchases — scriptably and offline. It is aimed at Dawn tenant admins/staff who need the catalog and roster as data, not clicks.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for the legacy Agilix Buzz/DLAP API (api.agilixbuzz.com, gls.agilix.com) — use the Ruby 'agilix' gem for that generation.
- Do not expect grade, enrollment, or certification collection endpoints — the modern Dawn /api does not expose them as top-level searchable resources.
- Do not use it to modify a live course structure; it is read-oriented catalog/roster tooling.

## Unique Capabilities

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

## Command Reference

**concept** — Browse the course/content catalog (Dawn concepts)

- `agilix-dawn-pp-cli concept get` — Get one concept (course) by id, including full section/instruction structure
- `agilix-dawn-pp-cli concept list` — List catalog concepts (courses/content). Uses the Dawn search DSL.

**config** — Show tenant configuration

- `agilix-dawn-pp-cli config` — Show the tenant's public configuration (name, root org, version, auth providers, payment)

**conversation** — List conversations/messages

- `agilix-dawn-pp-cli conversation` — List conversations. Uses the Dawn search DSL.

**organization** — List organizations/domains in the tenant

- `agilix-dawn-pp-cli organization` — List organizations. Uses the Dawn search DSL.

**progress** — List learner progress records

- `agilix-dawn-pp-cli progress` — List learner progress. Uses the Dawn search DSL (e.g. query:'user:u_... AND NOT state:preview').

**purchase** — List purchases (Stripe-backed commerce records)

- `agilix-dawn-pp-cli purchase` — List purchases. Uses the Dawn search DSL.

**user** — Look up users in the tenant

- `agilix-dawn-pp-cli user list` — List/search users. Uses the Dawn search DSL.
- `agilix-dawn-pp-cli user me` — Show the authenticated user (whoami)


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
agilix-dawn-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

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

## Auth Setup

Dawn authenticates with a raw token in the Authorization header (no 'Bearer ' prefix). Set AGILIX_DAWN_TOKEN to a Dawn API-user token (recommended, durable) or a session token from your logged-in browser. The token is passed as the Authorization header on every request.

Run `agilix-dawn-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  agilix-dawn-pp-cli concept list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `AGILIX_DAWN_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `AGILIX_DAWN_CONFIG_DIR`, `AGILIX_DAWN_DATA_DIR`, `AGILIX_DAWN_STATE_DIR`, `AGILIX_DAWN_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `AGILIX_DAWN_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `agilix-dawn-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

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

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `AGILIX_DAWN_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `AGILIX_DAWN_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
agilix-dawn-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
agilix-dawn-pp-cli feedback --stdin < notes.txt
agilix-dawn-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `AGILIX_DAWN_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `AGILIX_DAWN_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
agilix-dawn-pp-cli profile save briefing --json
agilix-dawn-pp-cli --profile briefing concept list
agilix-dawn-pp-cli profile list --json
agilix-dawn-pp-cli profile show briefing
agilix-dawn-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `agilix-dawn-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/agilix-dawn/cmd/agilix-dawn-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add agilix-dawn-pp-mcp -- agilix-dawn-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which agilix-dawn-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   agilix-dawn-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `agilix-dawn-pp-cli <command> --help`.
