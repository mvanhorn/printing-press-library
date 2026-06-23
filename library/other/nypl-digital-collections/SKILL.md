---
name: pp-nypl-digital-collections
description: "Printing Press CLI for Nypl Digital Collections. New York Public Library Digital Collections API. Source docs: https://api.repo.nypl.org/."
author: "kierandotai"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - nypl-digital-collections-pp-cli
    install:
      - kind: go
        bins: [nypl-digital-collections-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/nypl-digital-collections/cmd/nypl-digital-collections-pp-cli
---

# Nypl Digital Collections — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `nypl-digital-collections-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install nypl-digital-collections --cli-only
   ```
2. Verify: `nypl-digital-collections-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/nypl-digital-collections/cmd/nypl-digital-collections-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

New York Public Library Digital Collections API. Source docs: https://api.repo.nypl.org/. Note: NYPL states the Repo API is being deprecated and will no longer be available starting August 1st, 2026.

## Command Reference

**collections** — Manage collections

- `nypl-digital-collections-pp-cli collections get` — Returns paginated child captures for a collection or subcollection/container UUID.
- `nypl-digital-collections-pp-cli collections list` — Returns paginated information about collections.

**items** — Manage items

- `nypl-digital-collections-pp-cli items get-counts` — Returns collection item counts.
- `nypl-digital-collections-pp-cli items get-details` — Returns MODS for a capture UUID plus related capture information. UUID must belong to a valid capture.
- `nypl-digital-collections-pp-cli items get-featured` — Returns featured items.
- `nypl-digital-collections-pp-cli items get-mets-alto` — Returns METS ALTO for a given capture UUID.
- `nypl-digital-collections-pp-cli items get-minified-alto` — Returns minified ALTO for a given capture UUID.
- `nypl-digital-collections-pp-cli items get-mods-captures` — Returns MODS and capture information for a capture, item, container, or collection UUID.
- `nypl-digital-collections-pp-cli items get-plain-text` — Returns parsed plain text ALTO for a given capture UUID.
- `nypl-digital-collections-pp-cli items get-rights` — Returns rights profile information for a UUID.
- `nypl-digital-collections-pp-cli items get-total` — Returns the total number of digitized items.
- `nypl-digital-collections-pp-cli items list-all-collection-captures` — Returns all capture UUIDs, image IDs, item links, and titles for a capture, item, container, or collection UUID.
- `nypl-digital-collections-pp-cli items list-captures` — Returns capture UUIDs, image IDs, item links, and titles for an item, container, or collection UUID.
- `nypl-digital-collections-pp-cli items list-collection-captures` — Returns capture UUIDs, image IDs, item links, and titles for a capture, item, container, or collection UUID.
- `nypl-digital-collections-pp-cli items list-recent` — Returns the most recently added captures.
- `nypl-digital-collections-pp-cli items list-root` — Returns all top-level UUIDs for collections and orphan items.
- `nypl-digital-collections-pp-cli items lookup-identifier` — Returns UUIDs for a given identifier type and identifier value such as local_image_id, local_bnumber, local_barcode
- `nypl-digital-collections-pp-cli items search-digital` — Returns results matching keywords anywhere in a MODS metadata record.

**mods** — Manage mods

- `nypl-digital-collections-pp-cli mods <uuid>` — Returns MODS bibliographic data for a capture, item, container, or collection UUID.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
nypl-digital-collections-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup
Run `nypl-digital-collections-pp-cli auth setup` to print the URL and steps for getting a key (add `--launch` to open the URL). Then set:

```bash
export NYPL_DIGITAL_COLLECTIONS_NYPL_TOKEN="<your-key>"
```

Or persist it in `~/.config/nypl-digital-collections-pp-cli/config.toml`.

Run `nypl-digital-collections-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  nypl-digital-collections-pp-cli collections list --agent --select id,name,status
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
nypl-digital-collections-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
nypl-digital-collections-pp-cli feedback --stdin < notes.txt
nypl-digital-collections-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/nypl-digital-collections-pp-cli/feedback.jsonl`. They are never POSTed unless `NYPL_DIGITAL_COLLECTIONS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `NYPL_DIGITAL_COLLECTIONS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
nypl-digital-collections-pp-cli profile save briefing --json
nypl-digital-collections-pp-cli --profile briefing collections list
nypl-digital-collections-pp-cli profile list --json
nypl-digital-collections-pp-cli profile show briefing
nypl-digital-collections-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `nypl-digital-collections-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/nypl-digital-collections/cmd/nypl-digital-collections-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add nypl-digital-collections-pp-mcp -- nypl-digital-collections-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which nypl-digital-collections-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   nypl-digital-collections-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `nypl-digital-collections-pp-cli <command> --help`.
