---
name: pp-sentry
description: "A broad Sentry API CLI with local search, SQL, export, and MCP surfaces for incident work. Trigger phrases: `check Sentry issues`, `list Sentry projects`, `debug a Sentry event`, `audit Sentry releases`, `search Sentry incidents`, `use Sentry`, `run Sentry`."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - sentry-pp-cli
    install:
      - kind: go
        bins: [sentry-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/monitoring/sentry/cmd/sentry-pp-cli
---

# Sentry — Printing Press CLI

Use Sentry from scripts and agents without memorizing endpoint shapes. The CLI wraps the catalog OpenAPI, supports structured output, and adds local sync/search/SQL paths for incident triage and release audits.

## When to Use This CLI

Use this CLI when an agent or script needs structured access to Sentry organizations, projects, issues, events, releases, monitors, teams, and related observability data. Prefer it over a hosted MCP when you need local binaries, local sync/search/SQL, CSV/JSON output, or direct control over auth and region routing.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Read-only Sentry inventory
- **`organizations list`** — List Sentry organizations available to the authenticated token with structured output.

  _Use this first when an agent needs to discover the organization slug for Sentry API work._

  ```bash
  sentry-pp-cli organizations list --json --select slug,name
  ```
- **`seer`** — List the active LLM model names available through Sentry Seer.

  _Use this when an agent needs to know which Seer-backed model identifiers Sentry exposes._

  ```bash
  sentry-pp-cli seer --json
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**organizations** — Endpoints for organizations

- `sentry-pp-cli organizations list-your` — Return a list of organizations available to the authenticated session in a region. This is particularly useful for...
- `sentry-pp-cli organizations retrieve-an` — Return details on an individual organization, including various details such as membership access and teams.
- `sentry-pp-cli organizations update-an` — Update various attributes and configurable settings for the given organization.

**projects** — Endpoints for projects

- `sentry-pp-cli projects delete-a` — Schedules a project for deletion. Deletion happens asynchronously and therefore is not immediate. However once...
- `sentry-pp-cli projects retrieve-a` — Return details on an individual project.
- `sentry-pp-cli projects update-a` — Update various attributes and configurable settings for the given project. Note that solely having the...

**seer** — Endpoints for Seer features

- `sentry-pp-cli seer` — Get list of actively used LLM model names from Seer. Returns the list of AI models that are currently used in...

**sentry-app-installations** — Manage sentry app installations


**sentry-apps** — Manage sentry apps

- `sentry-pp-cli sentry-apps delete-a-custom-integration` — Delete a custom integration.
- `sentry-pp-cli sentry-apps retrieve-a-custom-integration-by-id-or-slug` — Retrieve a custom integration.
- `sentry-pp-cli sentry-apps update-an-existing-custom-integration` — Update an existing custom integration.

**teams** — Endpoints for teams

- `sentry-pp-cli teams delete-a` — Schedules a team for deletion. **Note:** Deletion happens asynchronously and therefore is not immediate. Teams will...
- `sentry-pp-cli teams retrieve-a` — Return details on an individual team.
- `sentry-pp-cli teams update-a` — Update various attributes and configurable settings for the given team.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
sentry-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Discover orgs

```bash
sentry-pp-cli organizations list --json --select slug,name
```

Find organization slugs for scoped Sentry commands.

### List projects for an org

```bash
sentry-pp-cli organizations projects list-an-organization-s my-org --json --select slug,name
```

Find project slugs before project-scoped incident queries.

### List issues narrowly

```bash
sentry-pp-cli organizations issues list-an-organization-s my-org --agent --select shortId,title,count,userCount
```

Keep agent context focused while triaging current issues.

### Export organization inventory

```bash
sentry-pp-cli export organizations --format json --output sentry-organizations.json
```

Create a portable incident handoff file after syncing or listing data.

### Inspect CLI capabilities

```bash
sentry-pp-cli agent-context --pretty
```

Give an agent the live command and auth surface before choosing a Sentry operation.

## Auth Setup

Set SENTRY_AUTH_TOKEN to a Sentry user or organization token with read scopes such as org:read, project:read, and event:read. Set SENTRY_REGION=de for EU-region SaaS organizations, or configure the base URL for self-hosted Sentry if supported by the generated config.

Run `sentry-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  sentry-pp-cli organizations list-your --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag

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
sentry-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
sentry-pp-cli feedback --stdin < notes.txt
sentry-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.sentry-pp-cli/feedback.jsonl`. They are never POSTed unless `SENTRY_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SENTRY_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
sentry-pp-cli profile save briefing --json
sentry-pp-cli --profile briefing organizations list-your
sentry-pp-cli profile list --json
sentry-pp-cli profile show briefing
sentry-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `sentry-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → CLI installation
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.25+)
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/monitoring/sentry/cmd/sentry-pp-cli@latest
   ```
3. Verify: `sentry-pp-cli --version`
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/monitoring/sentry/cmd/sentry-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add sentry-pp-mcp -- sentry-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which sentry-pp-cli`
   If not found, offer to install (see CLI Installation above).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   sentry-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `sentry-pp-cli <command> --help`.
