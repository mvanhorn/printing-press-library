---
name: pp-devonthink
description: "Local-first DEVONthink automation with safer shell workflows than raw AppleScript or MCP alone. Trigger phrases: `search DEVONthink`, `pack DEVONthink context`, `export DEVONthink maintenance inventory`, `audit DEVONthink links`, `batch tag DEVONthink records`, `use devonthink`, `run devonthink`."
author: "rowdy"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - devonthink-pp-cli
    install:
      - kind: go
        bins: [devonthink-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/devonthink/cmd/devonthink-pp-cli
---

# DEVONthink — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `devonthink-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install devonthink --cli-only
   ```
2. Verify: `devonthink-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/devonthink/cmd/devonthink-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Use DEVONthink as the local source of truth while giving agents and scripts stable, compact CLI output. The CLI wraps the official local MCP surface, adds search scopes, inventory export, context packing, local mirrors, and safety-oriented workflow primitives.

## When to Use This CLI

Use this CLI for local DEVONthink search, context preparation, inventory export, safe batch edits, and shell automation. Prefer it when an agent needs deterministic JSON and compact outputs instead of a GUI or raw MCP chat interaction. Let dedicated maintenance plugins own filing policy and recurring inbox triage decisions.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to expose DEVONthink over the public internet.
- Do not use this CLI as the canonical store for documents; DEVONthink remains canonical.
- Do not put filing-model policy or recurring inbox triage decisions in the core CLI; use a maintenance plugin on top of inventory export.
- Do not use write commands for bulk changes without a dry-run batch plan.
- Do not use semantic search unless a local mirror/index has been built and the embedding provider is intentionally configured.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Agent-native plumbing
- **`records search`** — Scope a normal DEVONthink query to a Smart Group by UUID, exact name, or DEVONthink path while preserving normal search output.

  _Use this when a downstream tool needs a stable dynamic search scope without treating Smart Groups as workflow policy._

  ```bash
  devonthink-pp-cli records search "tags:waiting/rueckerstattung" --smart-group "Offene Rückerstattungen" --agent --select uuid,name,item_link,tags,databaseName
  ```
- **`inventory export`** — Export DEVONthink databases, groups, tags, and document metadata for maintenance plugins.

  _Use this when structure-audit or inbox-triage tooling needs a stable local inventory contract._

  ```bash
  devonthink-pp-cli inventory export --format maintenance --query "kind:document" --limit 500 --agent --select databases,documents
  ```
- **`mcp call`** — Call DEVONthink's official local MCP tools from scripts when the local MCP server is enabled.

  _Use this when the official MCP exposes a new read tool before the CLI adds a first-class command._

  ```bash
  devonthink-pp-cli mcp call search_records --args '{"query":"kind:pdf","limit":5}' --agent
  ```

### Local state that compounds
- **`context pack`** — Build a compact evidence packet from records, selections, highlights, links, and related items.

  _Use this when an agent needs enough DEVONthink context to reason without dumping whole documents._

  ```bash
  devonthink-pp-cli context pack --query "project alpha" --token-budget 6000 --agent
  ```
- **`graph audit`** — Detect orphans, broken links, unresolved wiki links, weak hubs, and tag-only clusters.

  _Use this when DEVONthink should behave like a maintained knowledge graph instead of a folder pile._

  ```bash
  devonthink-pp-cli graph audit --limit 50 --agent
  ```

### Local safety
- **`privacy audit`** — Preview database scope, content-size budget, and cloud/MCP exposure before a handoff.

  _Use this before exporting or sharing DEVONthink-derived context with another tool._

  ```bash
  devonthink-pp-cli privacy audit --query "kind:pdf" --agent
  ```
- **`batch plan`** — Stage multi-record edits as validated dry-run plans before applying them.

  _Use this when a script needs reviewable intent before any DEVONthink mutation._

  ```bash
  devonthink-pp-cli batch plan --dry-run --agent
  ```

## Command Reference

**ai** — DEVONthink AI and summary helpers

- `devonthink-pp-cli ai ask` — Ask DEVONthink AI about selected local records with explicit cloud-use warnings
- `devonthink-pp-cli ai summarize` — Summarize records or highlights

**batch** — Dry-run-first multi-record mutation plans

- `devonthink-pp-cli batch apply` — Apply a previously reviewed local JSON plan
- `devonthink-pp-cli batch plan` — Stage multi-record changes as a local JSON plan

**context** — Agent context bundles

- `devonthink-pp-cli context` — Build a compact local context pack from records, selection, or search

**databases** — Open DEVONthink databases

- `devonthink-pp-cli databases` — List open databases

**graph** — Links, mentions, and knowledge graph health

- `devonthink-pp-cli graph audit` — Detect orphans, unresolved wiki links, weak hubs, and tag-only clusters
- `devonthink-pp-cli graph links` — List item links, wiki links, mentions, and unresolved wiki names

**groups** — DEVONthink groups and folders

- `devonthink-pp-cli groups` — Render a bounded group tree

**ingest** — File and URL ingestion

- `devonthink-pp-cli ingest file` — Import or index a file or folder
- `devonthink-pp-cli ingest url` — Capture a URL as Markdown, HTML, PDF, bookmark, or webarchive

**inventory** — Stable inventory export contracts

- `devonthink-pp-cli inventory` — Export databases, groups, tags, and selected document metadata for downstream tools

**ledger** — Local operation ledger

- `devonthink-pp-cli ledger list` — List recent CLI operation ledger entries
- `devonthink-pp-cli ledger show` — Show one ledger entry with target proofs and rollback hints

**mcp** — Optional local official MCP passthrough

- `devonthink-pp-cli mcp call` — Call a local official DEVONthink MCP tool by name
- `devonthink-pp-cli mcp schema` — Emit cached MCP tool schemas
- `devonthink-pp-cli mcp tools` — List official DEVONthink MCP tools when local MCP HTTP is enabled

**media** — OCR and transcription

- `devonthink-pp-cli media ocr` — OCR an image or scanned PDF
- `devonthink-pp-cli media transcribe` — Transcribe audio, video, image, or PDF content

**mirror** — Local SQLite mirror

- `devonthink-pp-cli mirror search` — Search the local mirror with FTS
- `devonthink-pp-cli mirror sync` — Refresh the local SQLite mirror from open DEVONthink databases

**privacy** — Local privacy and exposure reports

- `devonthink-pp-cli privacy` — Preview database scope, content-size budget, and cloud/MCP exposure before handoff

**records** — DEVONthink records

- `devonthink-pp-cli records content` — Extract text content with length and redaction controls
- `devonthink-pp-cli records create` — Create a record or group after validating destination
- `devonthink-pp-cli records get` — Get record metadata
- `devonthink-pp-cli records highlights` — Extract highlights and annotations
- `devonthink-pp-cli records lookup` — Look up records by exact name, URL, path, filename, location, or comment
- `devonthink-pp-cli records move` — Move, duplicate, replicate, or trash a record with dry-run proof
- `devonthink-pp-cli records related` — Find related records using DEVONthink similarity
- `devonthink-pp-cli records search` — Search records using DEVONthink query syntax or local mirror fallback
- `devonthink-pp-cli records update` — Update record text, properties, tags, comment, URL, aliases, or rating
- `devonthink-pp-cli records versions` — List saved record versions

**runtime** — Local DEVONthink runtime health

- `devonthink-pp-cli runtime` — Check DEVONthink app, AppleScript, optional MCP, and local mirror readiness

**selection** — Current DEVONthink GUI selection

- `devonthink-pp-cli selection get` — Return currently selected records
- `devonthink-pp-cli selection snapshot` — Capture the current selection as a reusable workflow seed

**sheets** — DEVONthink sheets

- `devonthink-pp-cli sheets <uuid>` — Read a sheet as structured rows

**tags** — Tag taxonomy and hygiene

- `devonthink-pp-cli tags` — Analyze tags for duplicates, case drift, action tags, and maintenance tags


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
devonthink-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Search within a Smart Group

```bash
devonthink-pp-cli records search "tags:waiting/rueckerstattung" --smart-group "Offene Rückerstattungen" --agent --select uuid,name,item_link,tags,databaseName
```

Scopes a normal query to a Smart Group and returns normal search rows plus meta.scope.

### Feed the maintenance plugin

```bash
devonthink-pp-cli inventory export --format maintenance --query "kind:document" --limit 500 --agent --select databases.name,documents.name,documents.tags
```

Produces stable inventory JSON for structure-audit and inbox-triage workflows.

## Auth Setup

Default operation uses local macOS automation and requires no API key. Optional official MCP passthrough uses DEVONthink's local MCP server when you enable it in DEVONthink; keep it bound to localhost or your own LAN and set any bearer token through DEVONthink's MCP settings.

Run `devonthink-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  devonthink-pp-cli databases --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
devonthink-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
devonthink-pp-cli feedback --stdin < notes.txt
devonthink-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/devonthink-pp-cli/feedback.jsonl`. They are never POSTed unless `DEVONTHINK_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `DEVONTHINK_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
devonthink-pp-cli profile save briefing --json
devonthink-pp-cli --profile briefing databases
devonthink-pp-cli profile list --json
devonthink-pp-cli profile show briefing
devonthink-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `devonthink-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/devonthink/cmd/devonthink-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add devonthink-pp-mcp -- devonthink-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which devonthink-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   devonthink-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `devonthink-pp-cli <command> --help`.
