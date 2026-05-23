---
name: pp-obsidian
description: "Every Obsidian CLI feature, plus protocol-aware frontmatter enforcement, instant offline FTS5 search, and... Trigger phrases: `lint my obsidian vault`, `check my notes for protocol violations`, `find every fact about [[entity]]`, `what's in my obsidian vault about <topic>`, `create a meeting note in my vault`, `search my notes`, `use obsidian`, `run obsidian`."
author: "Damien Stevens"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - obsidian-pp-cli
---

# Obsidian — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `obsidian-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install obsidian --cli-only
   ```
2. Verify: `obsidian-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

A filesystem-direct CLI for Obsidian vaults that enforces the UCE three-layer-memory protocol (Knowledge Graph / Events / Patterns) on every write, indexes the whole vault into a local SQLite store for sub-100ms full-text search and cross-note SQL queries, and emits agent-friendly progressive-disclosure output so an LLM can pick what to read without ingesting full notes. An optional rest subcommand passes through to the coddingtonbear/obsidian-local-rest-api community plugin when Obsidian is running.

## When to Use This CLI

Pick obsidian-pp-cli when you need to write to an Obsidian vault from an agent and the frontmatter has to pass a strict validator on the first try, when you need sub-100ms search over the vault, or when you want to pull entity dossiers, decision traces, or fact provenance into an agent context without reading full notes. Skip it for in-app workflows (graph view, canvas, plugin UI) — that's the desktop app's job.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Three-layer protocol enforcement
- **`lint`** — Walks the vault and reports frontmatter violations with severity tiers, encoding the three-layer-memory protocol rules used by the UCE pipeline.

  _Reach for this before handing the vault to any downstream extractor (cm, search, sync). A protocol error costs hours of silent extraction drift; lint catches them at write time._

  ```bash
  obsidian-pp-cli lint --severity error --json
  ```
- **`migrate`** — Fixes the mechanical subset of lint violations: ISO date coercion, type enum normalization, fill missing description from body.

  _Use this after onboarding an old vault or after a manual editing spree. Always start with --dry-run._

  ```bash
  obsidian-pp-cli migrate --rule date-iso --dry-run
  ```
- **`layers stats`** — Counts of notes per memory layer (Knowledge Graph / Events / Patterns) with type breakdown, average age, and recent-write velocity.

  _Run before a triage session to see where the vault is heavy or light: too many Events, no new Patterns, abandoned Knowledge-Graph entities._

  ```bash
  obsidian-pp-cli layers stats --json
  ```
- **`readiness`** — Filters lint findings to the rule subset that the downstream cm extraction pipeline depends on (missing description, missing type, bad date format).

  _Run before a Tuck sync. Fixing readiness errors here is cheap; debugging them in cm output is expensive._

  ```bash
  obsidian-pp-cli readiness --since 2026-04-01 --json
  ```

### Fact and decision tracking
- **`facts graduation-candidates`** — Lists entities whose inline fact count is approaching or past the 20-fact threshold for graduation to a TOML sidecar file.

  _Pick this when a person/project file is starting to feel heavy. Graduating to TOML before 20 keeps frontmatter loadable._

  ```bash
  obsidian-pp-cli facts graduation-candidates --threshold 20 --json
  ```
- **`facts decision-trace`** — Given a decision_trace_id, returns every fact across the vault that cites it, ordered by timestamp.

  _Reach for this when auditing how a decision propagated through the vault. Required when reconstructing a decision for a stakeholder or post-mortem._

  ```bash
  obsidian-pp-cli facts decision-trace DT-2026-0142 --json
  ```
- **`provenance`** — Reads source frontmatter on a note plus the source field on every fact in that note; prints a chain showing where each datum came from.

  _Reach for this when a fact looks wrong or disputed. The chain shows whether it came from a transcript, manual edit, or agent._

  ```bash
  obsidian-pp-cli provenance 'People/Jeff Smith.md' --json
  ```

### Agent-native reads
- **`entity dossier`** — Joins notes + frontmatter + facts + backlinks + tags for one entity into a single agent-readable block.

  _Use this as the default first read when an agent needs context about a person, company, or project — replaces grep + cat + parse._

  ```bash
  obsidian-pp-cli entity dossier '[[Jeff Smith]]' --layer description --json
  ```
- **`stale`** — Lists notes whose mtime predates a threshold, optionally filtered by type.

  _Run weekly to find meetings/journals that never got promoted to a Pattern, or active entities that have gone cold._

  ```bash
  obsidian-pp-cli stale --type meeting --older-than 90d --json
  ```
- **`daily append`** — Resolves today's daily-note path, creates it from the periodic-note template (with protocol-compliant frontmatter) if missing, and appends under a named section.

  _Default capture path for transcript ingest, journal entries, and any 'remember this' agent task._

  ```bash
  obsidian-pp-cli daily append 'Talked to Mark about Servosity pricing' --section '## Notes'
  ```

## Command Reference

**rest** — Optional Local REST API passthrough (requires the Obsidian app running with the Local REST API community plugin enabled).

- `obsidian-pp-cli rest active` — Read the file currently open in the Obsidian editor (requires --rest mode).
- `obsidian-pp-cli rest append-active` — Append text to the file currently open in the Obsidian editor.
- `obsidian-pp-cli rest commands` — List available Obsidian commands (REST plugin only).
- `obsidian-pp-cli rest delete-active` — Delete the file currently open in the Obsidian editor.
- `obsidian-pp-cli rest exec-command` — Execute an Obsidian command by its ID.
- `obsidian-pp-cli rest ping` — Verify the Local REST API plugin is reachable.
- `obsidian-pp-cli rest search-simple` — Simple text search through the REST plugin.
- `obsidian-pp-cli rest tags` — List all tags in the vault via the REST plugin.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
obsidian-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Capture a meeting note an agent wrote, protocol-enforced

```bash
obsidian-pp-cli note new --type meeting --title '2026-05-15 Servosity Pricing' --body 'Discussed pricing tiers with Jeff'
```

Creates a new meeting note with protocol-compliant frontmatter. Fails closed if any required field would be missing rather than writing a broken note.

### Find every fact tied to a decision trace

```bash
obsidian-pp-cli facts decision-trace DT-2026-0142 --json --select fact,source,timestamp
```

Returns just fact text, source, and timestamp across inline and TOML facts. --select narrows the payload for agent reads.

### Layer-2 progressive disclosure for an agent

```bash
obsidian-pp-cli disclose People/ --layer description --json
```

Returns path-to-description pairs for every note in People/. Agent picks what to dossier without reading bodies.

### Audit cm extraction-readiness before a Tuck sync

```bash
obsidian-pp-cli readiness --since 2026-05-01 --exit-nonzero-on error
```

Filters lint findings to cm-blocking rules over notes touched since the date. Exit code 2 if any error remains, so you can wire this into a pre-sync gate.

### SQL query the vault for stale active entities

```bash
obsidian-pp-cli sql 'SELECT path, type, description FROM notes WHERE type="person" AND status="active" LIMIT 10'
```

Read-only SQLite over the synced store — composable, scriptable, and faster than walking the vault.

## Auth Setup

No authentication required for filesystem mode — set OBSIDIAN_VAULT_PATH to your vault directory and you're done. Optional --rest mode requires the Obsidian Local REST API plugin and a bearer token (OBSIDIAN_REST_TOKEN).

Run `obsidian-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  obsidian-pp-cli rest active --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
obsidian-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
obsidian-pp-cli feedback --stdin < notes.txt
obsidian-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.obsidian-pp-cli/feedback.jsonl`. They are never POSTed unless `OBSIDIAN_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `OBSIDIAN_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
obsidian-pp-cli profile save briefing --json
obsidian-pp-cli --profile briefing rest active
obsidian-pp-cli profile list --json
obsidian-pp-cli profile show briefing
obsidian-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `obsidian-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add obsidian-pp-mcp -- obsidian-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which obsidian-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   obsidian-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `obsidian-pp-cli <command> --help`.
