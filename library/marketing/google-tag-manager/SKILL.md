---
name: pp-google-tag-manager
description: "Mirror any GTM container into a local database, then audit, diff Trigger phrases: `audit my GTM container`, `diff my GTM workspace against live`, `what GA4 ids are in my tag manager`, `check consent mode coverage`, `what fires on this GTM trigger`, `use google-tag-manager`, `run google-tag-manager`."
author: "Greg Stellato"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - google-tag-manager-pp-cli
    install:
      - kind: go
        bins: [google-tag-manager-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/marketing/google-tag-manager/cmd/google-tag-manager-pp-cli
---

# Google Tag Manager — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `google-tag-manager-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install google-tag-manager --cli-only
   ```
2. Verify: `google-tag-manager-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/google-tag-manager/cmd/google-tag-manager-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

The Google Tag Manager console can't diff anything, can't bulk-query, can't cross-reference the tag/trigger/variable graph, and can't export to version control. This CLI pulls a container into local SQLite and turns all of that into one command: audit hygiene, diff workspace-vs-live or container-vs-container, inventory GA4 ids and Consent Mode coverage across your whole fleet, and full-text search every parameter. It is read-only by construction — the mutating endpoints are stripped, so it is safe to point at a production container.

## When to Use This CLI

Use this CLI to audit, diff, export, and query Google Tag Manager container configuration from the terminal, CI, or an agent. It is the right tool for hygiene checks before a release, comparing two containers or a workspace against live, inventorying GA4 ids and Consent Mode coverage across many containers, and full-text search over tag/trigger/variable config. It is read-only — it never creates, edits, deletes, or publishes.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to create, edit, delete, or publish tags, triggers, variables, or versions — it is read-only by design; use the GTM console or the write API for changes.
- Do not use it to render or test tag firing in a browser; it reads configuration, not runtime behavior — use Google Tag Assistant for live firing tests.
- Do not use it for Google Analytics report data; this is GTM container configuration, not GA4 analytics.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local mirror that compounds
- **`pull`** — Walk an entire GTM container — every tag, trigger, variable, folder, template, client, zone, and gtag config — into a local SQLite store in one command.

  _Reach for this first — every other command reads the mirror, and re-pulls become time-stamped snapshots for history/drift._

  ```bash
  google-tag-manager-pp-cli pull --account 6012345 --container 9876543 --live
  ```
- **`export`** — Emit a deterministic flattened representation of the container for clean version-control diffs, or container-export-compatible JSON.

  _Use to check GTM config into git and review changes in pull requests._

  ```bash
  google-tag-manager-pp-cli export --flat
  ```

### Hygiene, compliance & governance
- **`audit`** — Run a battery of hygiene checks against the mirror: dead tags, orphan triggers, unused variables, paused tags, Custom HTML tags, and tags firing on All Pages.

  _Use to gate a release in CI or sanity-check a container before publishing; typed exit code fails the build on high-severity findings._

  ```bash
  google-tag-manager-pp-cli audit --json --fail-on high
  ```
- **`consent-report`** — Per-tag consentSettings coverage and a list of tags that fire without consent gating, classified by vendor (ads vs analytics).

  _Use to answer 'are we Consent Mode v2 compliant for the EEA' without clicking through every tag._

  ```bash
  google-tag-manager-pp-cli consent-report --json
  ```

### Diff & fleet visibility
- **`diff`** — Field-level diff between two snapshots: workspace vs live, version vs version, or container vs container, with stable name+type keys so renames are not add+remove.

  _Use before publishing to see exactly what changed, or to compare two properties' containers._

  ```bash
  google-tag-manager-pp-cli diff live workspace:7
  ```
- **`fleet`** — One table across all pulled containers: GA4/measurement-id inventory, Consent Mode coverage, Custom HTML count, and entity counts per container.

  _Use when you manage containers across multiple properties and need a portfolio view the console can never give._

  ```bash
  google-tag-manager-pp-cli fleet --metric ga4 --json
  ```
- **`search`** — Substring search across every entity's name, type, parameters, and notes — find hardcoded GA4 ids, stray pixels, or leftover URLs across all pulled containers.

  _Use to track down where a measurement id, domain, or vendor pixel is referenced before changing or removing it._

  ```bash
  google-tag-manager-pp-cli search "G-ABCDE12345"
  ```

### Dependency graph
- **`uses`** — Show everything that references a variable or trigger — the safe-to-delete / what-breaks-if-I-touch-this check.

  _Use before deleting or renaming a variable or trigger to see its blast radius._

  ```bash
  google-tag-manager-pp-cli uses "DLV - userId"
  ```
- **`fires`** — Bidirectional graph walk: which tags fire on a trigger, or which triggers and variables a tag depends on.

  _Use for incident debugging — 'why did this fire' or 'what fires here'._

  ```bash
  google-tag-manager-pp-cli fires --trigger 42 --json
  ```

## Command Reference

**tagmanager** — Manage tagmanager

- `google-tag-manager-pp-cli tagmanager accounts-containers-destinations-list` — Lists all Destinations linked to a GTM Container.
- `google-tag-manager-pp-cli tagmanager accounts-containers-environments-list` — Lists all GTM Environments of a GTM Container.
- `google-tag-manager-pp-cli tagmanager accounts-containers-list` — Lists all Containers that belongs to a GTM Account.
- `google-tag-manager-pp-cli tagmanager accounts-containers-lookup` — Looks up a Container by destination ID.
- `google-tag-manager-pp-cli tagmanager accounts-containers-snippet` — Gets the tagging snippet for a Container.
- `google-tag-manager-pp-cli tagmanager accounts-containers-version-headers-latest` — Gets the latest container version header
- `google-tag-manager-pp-cli tagmanager accounts-containers-version-headers-list` — Lists all Container Versions of a GTM Container.
- `google-tag-manager-pp-cli tagmanager accounts-containers-versions-live` — Gets the live (i.e. published) container version
- `google-tag-manager-pp-cli tagmanager accounts-containers-workspaces-built-in-variables-list` — Lists all the enabled Built-In Variables of a GTM Container.
- `google-tag-manager-pp-cli tagmanager accounts-containers-workspaces-clients-list` — Lists all GTM Clients of a GTM container workspace.
- `google-tag-manager-pp-cli tagmanager accounts-containers-workspaces-folders-list` — Lists all GTM Folders of a Container.
- `google-tag-manager-pp-cli tagmanager accounts-containers-workspaces-get-status` — Finds conflicting and modified entities in the workspace.
- `google-tag-manager-pp-cli tagmanager accounts-containers-workspaces-gtag-config-list` — Lists all Google tag configs in a Container.
- `google-tag-manager-pp-cli tagmanager accounts-containers-workspaces-list` — Lists all Workspaces that belong to a GTM Container.
- `google-tag-manager-pp-cli tagmanager accounts-containers-workspaces-tags-list` — Lists all GTM Tags of a Container.
- `google-tag-manager-pp-cli tagmanager accounts-containers-workspaces-templates-list` — Lists all GTM Templates of a GTM container workspace.
- `google-tag-manager-pp-cli tagmanager accounts-containers-workspaces-triggers-list` — Lists all GTM Triggers of a Container.
- `google-tag-manager-pp-cli tagmanager accounts-containers-workspaces-variables-list` — Lists all GTM Variables of a Container.
- `google-tag-manager-pp-cli tagmanager accounts-containers-workspaces-zones-list` — Lists all GTM Zones of a GTM container workspace.
- `google-tag-manager-pp-cli tagmanager accounts-list` — Lists all GTM Accounts that a user has access to.
- `google-tag-manager-pp-cli tagmanager accounts-user-permissions-get` — Gets a user's Account & Container access.
- `google-tag-manager-pp-cli tagmanager accounts-user-permissions-list` — List all users that have access to the account along with Account and Container user access granted to each of them.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
google-tag-manager-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Pre-publish review

```bash
google-tag-manager-pp-cli diff live workspace:7 --json
```

See exactly what a workspace changes relative to the published live container before publishing.

### Consent Mode v2 audit

```bash
google-tag-manager-pp-cli consent-report --json --select tags.name,tags.consentStatus
```

List every tag with its consent gating status, narrowed to just name and status for a compact agent-friendly payload.

### Fleet GA4 inventory

```bash
google-tag-manager-pp-cli fleet --metric ga4 --csv
```

Export which GA4 measurement ids appear in which containers across your whole estate as CSV.

### Blast-radius before deleting a variable

```bash
google-tag-manager-pp-cli uses "DLV - userId" --json
```

List every tag and trigger that references a variable so you know what breaks before touching it.

### Find stray pixels

```bash
google-tag-manager-pp-cli search "facebook" --json --select results.container,results.name,results.type
```

Full-text search every entity across all pulled containers for leftover vendor pixels, narrowing the nested response to the fields that matter.

## Auth Setup

Google Tag Manager has no API key — it uses OAuth 2.0. Provide a read-scoped access token as GTM_ACCESS_TOKEN. The quickest path with gcloud: run `gcloud auth application-default login --scopes=https://www.googleapis.com/auth/tagmanager.readonly`, then `export GTM_ACCESS_TOKEN=$(gcloud auth application-default print-access-token)`. Alternatively use a service account whose email is granted Viewer access in GTM admin and mint a token for it. The CLI sends the token as `Authorization: Bearer <token>` and only ever needs the read-only scope.

Run `google-tag-manager-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  google-tag-manager-pp-cli tagmanager accounts-containers-destinations-list mock-value --agent --select id,name,status
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

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
google-tag-manager-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
google-tag-manager-pp-cli feedback --stdin < notes.txt
google-tag-manager-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/google-tag-manager-pp-cli/feedback.jsonl`. They are never POSTed unless `GOOGLE_TAG_MANAGER_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `GOOGLE_TAG_MANAGER_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
google-tag-manager-pp-cli profile save briefing --json
google-tag-manager-pp-cli --profile briefing tagmanager accounts-containers-destinations-list mock-value
google-tag-manager-pp-cli profile list --json
google-tag-manager-pp-cli profile show briefing
google-tag-manager-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `google-tag-manager-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/google-tag-manager/cmd/google-tag-manager-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add google-tag-manager-pp-mcp -- google-tag-manager-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which google-tag-manager-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   google-tag-manager-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `google-tag-manager-pp-cli <command> --help`.
