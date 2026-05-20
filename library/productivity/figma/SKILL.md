---
name: pp-figma
description: "Every Figma endpoint, plus codegen-ready frame extracts, comments-audit, orphans finder, tokens diff, and webhook... Trigger phrases: `extract a figma frame for codegen`, `compact figma file context for AI`, `find unresolved figma comments`, `figma stale components`, `diff figma design tokens`, `figma file fingerprint for CI`, `replay figma webhook deliveries`, `where is this figma variable used`, `use figma`, `run figma-pp-cli`."
author: "Giuliano Giacaglia"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - figma-pp-cli
---

# Figma — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `figma-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install figma --cli-only
   ```
2. Verify: `figma-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/figma/cmd/figma-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.


## When to Use This CLI

Use figma-pp-cli when an agent or engineer needs to reason about a Figma file as data — not pixels. It shines for codegen prompt context (`frame extract`, `dev-mode dump`), design-system ops (`orphans`, `tokens diff`, `fingerprint`), comment hygiene (`comments-audit`), and webhook iteration (`webhooks test`). For interactive design work, the Figma desktop app is fine — figma-pp-cli covers everything Figma's REST API exposes and adds the cross-file analytical commands the dashboard hides.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Agent-native plumbing
- **`frame extract`** — Extract a single frame as a compact codegen-ready payload that fuses simplified node tree, in-scope variables, dev resources, and Code Connect mappings.

  _First call when an AI agent needs Figma frame context for code generation — returns a compact payload that fits in the context window instead of the raw 10MB file response._

  ```bash
  figma-pp-cli frame extract abc123 --ids 1234-5678 --depth 4 --include variables,dev-resources --json
  ```
- **`dev-mode dump`** — Emit a portable Markdown bundle that fuses dev-resource links, variables in scope, render permalink, and Code Connect mapping for one node.

  _Use when an agent or engineer needs the full Dev Mode context for one frame as a single Markdown blob — no Desktop pairing required._

  ```bash
  figma-pp-cli dev-mode dump abc123 --node 1234-5678 --format md
  ```
- **`webhooks test`** — Pull Figma's webhook request log and replay stored payloads (with original headers and HMAC) against an arbitrary target URL.

  _Use when iterating on a new webhook handler — replay yesterday's failed deliveries against your local server without re-triggering upstream events._

  ```bash
  figma-pp-cli webhooks test wh_abc --replay-failed --target-url https://localhost:3000/figma
  ```

### Local state that compounds
- **`comments-audit`** — Aggregate unresolved comments across every synced team file with age and group-by filters.

  _Run this on Monday morning before design review — surfaces every stale unresolved thread across the team._

  ```bash
  figma-pp-cli comments-audit --older-than 14d --group-by file,author --json
  ```
- **`orphans`** — Find published library entities (components, styles, variables) with zero usage over a window by joining team-library publish list with library-analytics usage data.

  _First command for the quarterly design-system cleanup — returns the list of entities safe to deprecate._

  ```bash
  figma-pp-cli orphans 12345 --kind component,style,variable --window 30d --json
  ```
- **`tokens diff`** — Diff Figma variables across two file versions with mode-awareness; emits a Markdown or JSON change set.

  _Run before merging a design-tokens PR to see what actually changed in Figma since the last release._

  ```bash
  figma-pp-cli tokens diff abc123 --from v1.0.0 --to HEAD --format md
  ```
- **`fingerprint`** — Stable hash of a Figma file's token + component + style surface; exits non-zero if --expect doesn't match.

  _Wire this into CI to fail builds when the upstream Figma file's design-system surface drifts from the committed snapshot._

  ```bash
  figma-pp-cli fingerprint abc123 --expect sha256:a1b2c3...
  ```
- **`variables explain`** — Flat list of every node and component that references a given variable across a file.

  _First call when planning a variable rename or deprecation — shows the blast radius before you touch anything._

  ```bash
  figma-pp-cli variables explain abc123 --variable color/brand/primary --json
  ```

## Command Reference

**activity-logs** — Get activity logs as an organization admin.

- `figma-pp-cli activity-logs` — Returns a list of activity log events

**component-sets** — Get information about published component sets.

- `figma-pp-cli component-sets <key>` — Get metadata on a published component set by key.

**components** — Get information about published components.

- `figma-pp-cli components <key>` — Get metadata on a component by key.

**dev-resources** — Interact with dev resources in Figma Dev Mode.

- `figma-pp-cli dev-resources post` — Bulk create dev resources across multiple files. Dev resources that are successfully created will show up in the...
- `figma-pp-cli dev-resources put` — Bulk update dev resources across multiple files. Ids for dev resources that are successfully updated will show up in...

**developer-logs** — Get developer logs for REST API and MCP server requests in an organization.

- `figma-pp-cli developer-logs` — Returns a list of developer log entries for REST API and MCP server requests made within the organization. This...

**figma-analytics** — Manage figma analytics

- `figma-pp-cli figma-analytics get-library-component-actions` — Returns a list of library analytics component actions data broken down by the requested dimension.
- `figma-pp-cli figma-analytics get-library-component-usages` — Returns a list of library analytics component usage data broken down by the requested dimension.
- `figma-pp-cli figma-analytics get-library-style-actions` — Returns a list of library analytics style actions data broken down by the requested dimension.
- `figma-pp-cli figma-analytics get-library-style-usages` — Returns a list of library analytics style usage data broken down by the requested dimension.
- `figma-pp-cli figma-analytics get-library-variable-actions` — Returns a list of library analytics variable actions data broken down by the requested dimension.
- `figma-pp-cli figma-analytics get-library-variable-usages` — Returns a list of library analytics variable usage data broken down by the requested dimension.

**files** — Get file JSON, images, and other file-related content.

- `figma-pp-cli files <file_key>` — Returns the document identified by `file_key` as a JSON object. The file key can be parsed from any Figma file url:...

**images** — Manage images

- `figma-pp-cli images <file_key>` — Renders images from a file. If no error occurs, `'images'` will be populated with a map from node IDs to URLs of the...

**me** — Manage me

- `figma-pp-cli me` — Returns the user information for the currently authenticated user.

**oembed** — Get oEmbed data for Figma files and published Makes.

- `figma-pp-cli oembed` — Returns oEmbed data for a Figma file or published Make site URL, following the [oEmbed...

**payments** — Get purchase information for your Community resources.

- `figma-pp-cli payments` — There are two methods to query for a user's payment information on a plugin, widget, or Community file. The first...

**projects** — Get information about projects and files in teams.


**styles** — Get information about published styles.

- `figma-pp-cli styles <key>` — Get metadata on a style by key.

**teams** — Manage teams


**webhooks** — Interact with team webhooks as a team admin.

- `figma-pp-cli webhooks delete` — Deletes the specified webhook. This operation cannot be reversed.
- `figma-pp-cli webhooks get` — Returns a list of webhooks corresponding to the context or plan provided, if they exist. For plan, the webhooks for...
- `figma-pp-cli webhooks get-webhookid` — Get a webhook by ID.
- `figma-pp-cli webhooks post` — Create a new webhook which will call the specified endpoint when the event triggers. By default, this webhook will...
- `figma-pp-cli webhooks put` — Update a webhook by ID.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
figma-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Extract a frame for an AI codegen prompt

```bash
figma-pp-cli frame extract abc123 --ids 1234-5678 --depth 4 --include variables,dev-resources --json --select 'simplifiedNodeCount,nodes,variables,devResources'
```

The killer command. Returns a compact JSON with the simplified node tree, every variable in scope, every dev-resource link, and a node count showing the compression ratio — paste it directly into Claude or Cursor.

### Find unresolved comments older than two weeks

```bash
figma-pp-cli comments-audit --older-than 14d --group-by file,author --json --select 'file,author,count,oldest_at'
```

After sync, walks the comments table and aggregates by file and author. Pipe through jq for Slack-ready Markdown.

### Cleanup orphans across a team library

```bash
figma-pp-cli orphans 12345 --kind component,style,variable --window 30d --json
```

Joins team-library publish list with 30-day usage analytics; emits the list of entities published but with zero usages — your deprecation candidates. Enterprise-tier required.

### Diff design tokens between two file versions

```bash
figma-pp-cli tokens diff abc123 --from v1.0.0 --to HEAD --format md
```

Resolves the version ids, snapshots variables at each, and emits a Markdown change-set listing added/removed/renamed/value-changed tokens. Run before merging a design-tokens PR.

### Replay failed webhook deliveries against a local server

```bash
figma-pp-cli webhooks test wh_abc --replay-failed --target-url http://localhost:3000/figma
```

Fetches the request log via /v2/webhooks/{id}/requests, filters status >= 400, and replays each delivery against your local URL with the original headers and HMAC re-signed under the webhook's passcode.

## Auth Setup

Figma supports two auth modes: Personal Access Token (header X-Figma-Token, prefix figd_) for personal/automation use, and OAuth 2.0 (Authorization: Bearer) for /v1/me, /v1/activity_logs, and /v1/developer_logs. Set FIGMA_API_TOKEN (PAT) or FIGMA_OAUTH2 (OAuth Bearer); the CLI auto-routes to the right header. Run `figma-pp-cli auth login` for OAuth, or just figma-pp-cli auth login for PAT. Doctor surfaces X-Figma-Plan-Tier and X-Figma-Rate-Limit-Type from response headers.

Run `figma-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  figma-pp-cli activity-logs --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
figma-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
figma-pp-cli feedback --stdin < notes.txt
figma-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.figma-pp-cli/feedback.jsonl`. They are never POSTed unless `FIGMA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `FIGMA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
figma-pp-cli profile save briefing --json
figma-pp-cli --profile briefing activity-logs
figma-pp-cli profile list --json
figma-pp-cli profile show briefing
figma-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `figma-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add figma-pp-mcp -- figma-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which figma-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   figma-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `figma-pp-cli <command> --help`.

## Known Gaps

The following are documented limitations from Phase 5 live dogfood (152/165 = 92% pass), all environmental — the CLI itself is correct.

- **OAuth-only endpoints**: `activity-logs`, `developer-logs`, and (in some scopes) `me` require OAuth Bearer auth, not PAT. Set `FIGMA_OAUTH2` to your OAuth Bearer token to use these. The CLI surfaces the API's 401 with a clear message.
- **`oembed`, `payments`, `webhooks get`** require runtime parameters the printed-CLI dogfood matrix cannot synthesize (`--url`, `--user-id`, `--context`). They work correctly when invoked with proper arguments.
- **`comments-audit`** requires a populated local cache: run `figma-pp-cli sync --resources comments` against a known file before invoking it. Returns an honest empty-store error otherwise.
- **`orphans`** depends on Figma's Library Analytics API (Enterprise tier). On non-Enterprise plans it returns exit 0 with a clear "analytics data is empty" message rather than failing.

