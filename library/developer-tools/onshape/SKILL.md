---
name: pp-onshape
description: "Printing Press CLI for Onshape. Onshape cloud CAD API CLI — manage documents, parts, assemblies, and exports"
author: "Markimus"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - onshape-pp-cli
---

# Onshape — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `onshape-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install onshape --cli-only
   ```
2. Verify: `onshape-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Onshape cloud CAD API CLI — manage documents, parts, assemblies, and exports

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Agent-native CAD navigation
- **`documents search`** — Find recent CAD documents with compact structured fields that agents can carry into follow-up workspace, element, and export calls.

  _Use this first when an agent needs to locate the right assembly or part studio without burning context on full document metadata._

  ```bash
  onshape-pp-cli documents search --query Trailer --limit 5 --agent --select id,name,modifiedAt
  ```
- **`elements`** — Turn an Onshape document/workspace pair into a typed map of Part Studios, assemblies, BOM tabs, blobs, and application elements.

  _Use this after choosing a document to identify which element should feed part inspection, assembly inspection, export, or rendering._

  ```bash
  onshape-pp-cli elements --did 3cb6ad4256bb099a0e4813ab --wvm w --wvmid b3fe484986a689a317b7259b --agent --select id,name,elementType
  ```

### Assembly intelligence
- **`assemblies get`** — Fetch an assembly definition and select just the instance graph fields needed for CAD review, BOM reasoning, or downstream Blender planning.

  _Use this when an agent needs to understand assembly composition before exporting geometry or planning an animation/rendering scene._

  ```bash
  onshape-pp-cli assemblies get --did 3cb6ad4256bb099a0e4813ab --wvm w --wvmid b3fe484986a689a317b7259b --eid f22782a9f60e037e2f4d7c39 --agent --select rootAssembly.instances.id,rootAssembly.instances.name
  ```

### Export readiness
- **`parts list`** — List part IDs and names from a Part Studio so an agent can choose export targets without opening Onshape manually.

  _Use this before exporting a single component to Blender, CAM, simulation, or a rendering pipeline._

  ```bash
  onshape-pp-cli parts list --did 3cb6ad4256bb099a0e4813ab --wvm w --wvmid b3fe484986a689a317b7259b --eid 1753f0a84436bf2bc39d6da6 --agent --select name,partId
  ```

### Local state that compounds
- **`sync`** — Hydrate a local SQLite mirror so agents can search and analyze previously seen Onshape data without repeated live API calls.

  _Use this when an agent will work across many CAD documents or needs resilient offline search during a longer design/review session._

  ```bash
  onshape-pp-cli sync --resources documents --latest-only --agent
  ```

## Command Reference

**assemblies** — Inspect assemblies and their structure

- `onshape-pp-cli assemblies get` — Get assembly definition and instances
- `onshape-pp-cli assemblies list` — List assemblies in a document

**documents** — Search and manage Onshape documents

- `onshape-pp-cli documents create` — Create a new document
- `onshape-pp-cli documents delete` — Delete a document
- `onshape-pp-cli documents get` — Get document metadata by ID
- `onshape-pp-cli documents search` — Search documents by name or query

**elements** — List elements (tabs) in a document

- `onshape-pp-cli elements` — List elements in a document version/workspace

**exports** — Export geometry and drawings

- `onshape-pp-cli exports element` — Export all parts in a part studio
- `onshape-pp-cli exports part` — Export a single part

**parts** — Inspect parts and their metadata

- `onshape-pp-cli parts get` — Get part metadata
- `onshape-pp-cli parts list` — List parts in an element

**translations** — Async translation/export jobs

- `onshape-pp-cli translations create` — Create a translation (export) job
- `onshape-pp-cli translations get` — Check translation job status

**versions** — Manage versions within a document

- `onshape-pp-cli versions create` — Create a new version
- `onshape-pp-cli versions list` — List versions in a document

**workspaces** — Manage workspaces within a document

- `onshape-pp-cli workspaces create` — Create a new workspace
- `onshape-pp-cli workspaces delete` — Delete a workspace
- `onshape-pp-cli workspaces list` — List workspaces in a document


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
onshape-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

No authentication required.

Run `onshape-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  onshape-pp-cli assemblies list --did 550e8400-e29b-41d4-a716-446655440000 --wvm example-value --wvmid 550e8400-e29b-41d4-a716-446655440000 --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

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
onshape-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
onshape-pp-cli feedback --stdin < notes.txt
onshape-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.onshape-pp-cli/feedback.jsonl`. They are never POSTed unless `ONSHAPE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ONSHAPE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
onshape-pp-cli profile save briefing --json
onshape-pp-cli --profile briefing assemblies list --did 550e8400-e29b-41d4-a716-446655440000 --wvm example-value --wvmid 550e8400-e29b-41d4-a716-446655440000
onshape-pp-cli profile list --json
onshape-pp-cli profile show briefing
onshape-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `onshape-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add onshape-pp-mcp -- onshape-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which onshape-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   onshape-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `onshape-pp-cli <command> --help`.
