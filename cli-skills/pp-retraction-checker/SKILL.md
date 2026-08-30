---
name: pp-retraction-checker
description: "Check whether a paper is retracted, why, and what the current research says now — keyless, over Crossref and OpenAlex. Trigger phrases: `is this paper retracted`, `check retraction`, `check this DOI`, `scan my bibliography for retractions`, `what replaced this retracted paper`, `use retraction-checker`, `run retraction-checker`."
author: "laci141"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - retraction-checker-pp-cli
    install:
      - kind: go
        bins: [retraction-checker-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/retraction-checker/cmd/retraction-checker-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/other/retraction-checker/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Retraction Checker — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `retraction-checker-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install retraction-checker --cli-only
   ```
2. Verify: `retraction-checker-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/retraction-checker/cmd/retraction-checker-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Retraction Checker turns Crossref's embedded Retraction Watch data into a one-shot verdict: is this DOI or PMID retracted, when, why, and where is the notice. It batch-scans reading lists and .bib files, finds citation-ranked superseding research via OpenAlex, and watches a topic or library for newly-announced retractions. Fully keyless.

## When to Use This CLI

Use this CLI when an agent or researcher needs to verify that a cited paper has not been retracted, audit a bibliography for retracted references, or find current research that supersedes a retracted study. It is the right choice for citation-integrity checks in literature reviews and RAG pipelines.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to fetch full-text papers or PDFs
- Do not use it for general scholarly search unrelated to retraction — use scientific-consensus for consensus and evidence analytics

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Retraction intelligence
- **`check`** — Tell whether a paper (by DOI or PMID) has been retracted, when, why, and where the notice is.

  _Agents citing a paper should verify it is not retracted before relying on it._

  ```bash
  retraction-checker-pp-cli check 10.1016/j.micpro.2020.103768 --json
  ```
- **`scan`** — Batch-check a reading list or .bib file and flag every retracted entry.

  _Catches retracted citations across a whole manuscript or literature review at once._

  ```bash
  retraction-checker-pp-cli scan refs.bib --json
  ```
- **`superseded`** — For a retracted or older paper, find related more-recent research on the same topic, ranked by citation count.

  _When a paper is retracted, the agent still needs the current best evidence on the topic._

  ```bash
  retraction-checker-pp-cli superseded 10.1016/j.micpro.2020.103768 --json
  ```

### Local state that compounds
- **`watch`** — Monitor a topic or reading list for newly-announced retractions since the last run.

  _Surfaces new retractions in a field or personal library without re-reading everything._

  ```bash
  retraction-checker-pp-cli watch "machine learning" --json
  ```

## Command Reference

**works** — Manage works

- `retraction-checker-pp-cli works get` — Get a single work by DOI
- `retraction-checker-pp-cli works search` — Search or filter scholarly works


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
retraction-checker-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Check a DOI

```bash
retraction-checker-pp-cli check 10.1016/j.micpro.2020.103768 --json
```

Returns retraction status, date, reason source, and notice reference for one paper.

### Audit a bibliography

```bash
retraction-checker-pp-cli scan reading-list.txt --agent --select doi,retracted,reason
```

Scans one DOI/PMID per line and returns only the key retraction fields for each entry.

### Find superseding work

```bash
retraction-checker-pp-cli superseded 10.1016/j.micpro.2020.103768 --json
```

Lists more-recent related papers ranked by citations, published after the retracted paper.

### Watch a field

```bash
retraction-checker-pp-cli watch "crispr" --json
```

Baselines retraction notices for a topic and reports new ones on later runs.

## Auth Setup

No authentication required.

Run `retraction-checker-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  retraction-checker-pp-cli works get mock-value --agent --select id,name,status
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

- Use `--home <dir>` for one invocation, or set `RETRACTION_CHECKER_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `RETRACTION_CHECKER_CONFIG_DIR`, `RETRACTION_CHECKER_DATA_DIR`, `RETRACTION_CHECKER_STATE_DIR`, `RETRACTION_CHECKER_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `RETRACTION_CHECKER_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `retraction-checker-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "retraction-checker": {
        "command": "retraction-checker-pp-mcp",
        "env": {
          "RETRACTION_CHECKER_HOME": "/srv/retraction-checker"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `RETRACTION_CHECKER_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `RETRACTION_CHECKER_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
retraction-checker-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
retraction-checker-pp-cli feedback --stdin < notes.txt
retraction-checker-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `RETRACTION_CHECKER_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `RETRACTION_CHECKER_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
retraction-checker-pp-cli profile save briefing --json
retraction-checker-pp-cli --profile briefing works get mock-value
retraction-checker-pp-cli profile list --json
retraction-checker-pp-cli profile show briefing
retraction-checker-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `retraction-checker-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/retraction-checker/cmd/retraction-checker-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add retraction-checker-pp-mcp -- retraction-checker-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which retraction-checker-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   retraction-checker-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `retraction-checker-pp-cli <command> --help`.
