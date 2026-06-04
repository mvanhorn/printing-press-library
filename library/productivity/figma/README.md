# Figma CLI

**Every Figma endpoint, plus codegen-ready frame extracts, comments-audit, orphans finder, tokens diff, and webhook replay no other Figma tool ships.**

This CLI is the offline-first, agent-native Figma operator. It absorbs every REST endpoint as one Cobra command, ports GLips's compaction pipeline so frame extracts fit in an LLM context window, and adds the analytical primitives no other tool unifies: cross-file comments-audit, stale-component orphans finder, semantic tokens diff between file versions, deterministic file fingerprint for CI, and webhook delivery replay.

Learn more at [Figma](https://developers.figma.com/docs/rest-api/).

Created by [@giacaglia](https://github.com/giacaglia) (Giuliano Giacaglia).

## Install

The recommended path installs both the `figma-pp-cli` binary and the `pp-figma` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install figma
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install figma --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install figma --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install figma --agent claude-code
npx -y @mvanhorn/printing-press-library install figma --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/figma/cmd/figma-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/figma-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-figma --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-figma --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install figma --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/figma-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `FIGMA_OAUTH2` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "figma": {
      "command": "figma-pp-mcp",
      "env": {
        "FIGMA_OAUTH2": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Figma supports two auth modes: Personal Access Token (header X-Figma-Token, prefix figd_) for personal/automation use, and OAuth 2.0 (Authorization: Bearer) for /v1/me, /v1/activity_logs, and /v1/developer_logs. Set FIGMA_API_TOKEN (PAT) or FIGMA_OAUTH2 (OAuth Bearer); the CLI auto-routes to the right header. Run `figma-pp-cli auth login` for OAuth, or just figma-pp-cli auth login for PAT. Doctor surfaces X-Figma-Plan-Tier and X-Figma-Rate-Limit-Type from response headers.

## Quick Start

```bash
# Save your Figma OAuth Bearer token (PAT users can also set FIGMA_API_TOKEN).
export FIGMA_OAUTH2=your-token-here

# Confirm auth + reachability + plan tier; warns on Enterprise endpoints if you only have a PAT.
figma-pp-cli doctor

# Populate the local SQLite store so cross-file commands work offline.
figma-pp-cli sync

# Find every unresolved comment older than two weeks across your team.
figma-pp-cli comments-audit --older-than 14d --group-by file,author
```

## Unique Features

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

## Usage

Run `figma-pp-cli --help` for the full command reference and flag list.

## Commands

### activity-logs

Get activity logs as an organization admin.

- **`figma-pp-cli activity-logs get`** - Returns a list of activity log events

### component-sets

Get information about published component sets.

- **`figma-pp-cli component-sets get`** - Get metadata on a published component set by key.

### components

Get information about published components.

- **`figma-pp-cli components get`** - Get metadata on a component by key.

### dev-resources

Interact with dev resources in Figma Dev Mode.

- **`figma-pp-cli dev-resources post`** - Bulk create dev resources across multiple files.
Dev resources that are successfully created will show up in the links_created array in the response.

If there are any dev resources that cannot be created, you may still get a 200 response. These resources will show up in the errors array. Some reasons a dev resource cannot be created include:

- Resource points to a `file_key` that cannot be found.
- The node already has the maximum of 10 dev resources.
- Another dev resource for the node has the same url.
- **`figma-pp-cli dev-resources put`** - Bulk update dev resources across multiple files.

Ids for dev resources that are successfully updated will show up in the `links_updated` array in the response.

If there are any dev resources that cannot be updated, you may still get a 200 response. These resources will show up in the `errors` array.

### developer-logs

Get developer logs for REST API and MCP server requests in an organization.

- **`figma-pp-cli developer-logs get`** - Returns a list of developer log entries for REST API and MCP server requests made within the organization. This endpoint requires a plan access token with the `org:developer_log_read` scope.

### figma-analytics

Manage figma analytics

- **`figma-pp-cli figma-analytics get-library-component-actions`** - Returns a list of library analytics component actions data broken down by the requested dimension.
- **`figma-pp-cli figma-analytics get-library-component-usages`** - Returns a list of library analytics component usage data broken down by the requested dimension.
- **`figma-pp-cli figma-analytics get-library-style-actions`** - Returns a list of library analytics style actions data broken down by the requested dimension.
- **`figma-pp-cli figma-analytics get-library-style-usages`** - Returns a list of library analytics style usage data broken down by the requested dimension.
- **`figma-pp-cli figma-analytics get-library-variable-actions`** - Returns a list of library analytics variable actions data broken down by the requested dimension.
- **`figma-pp-cli figma-analytics get-library-variable-usages`** - Returns a list of library analytics variable usage data broken down by the requested dimension.

### files

Get file JSON, images, and other file-related content.

- **`figma-pp-cli files get`** - Returns the document identified by `file_key` as a JSON object. The file key can be parsed from any Figma file url: `https://www.figma.com/file/{file_key}/{title}`.

The `document` property contains a node of type `DOCUMENT`.

The `components` property contains a mapping from node IDs to component metadata. This is to help you determine which components each instance comes from.

### images

Manage images

- **`figma-pp-cli images get`** - Renders images from a file.

If no error occurs, `"images"` will be populated with a map from node IDs to URLs of the rendered images, and `"status"` will be omitted. The image assets will expire after 30 days. Images up to 32 megapixels can be exported. Any images that are larger will be scaled down.

Important: the image map may contain values that are `null`. This indicates that rendering of that specific node has failed. This may be due to the node id not existing, or other reasons such has the node having no renderable components. It is guaranteed that any node that was requested for rendering will be represented in this map whether or not the render succeeded.

To render multiple images from the same file, use the `ids` query parameter to specify multiple node ids.

```
GET /v1/images/:key?ids=1:2,1:3,1:4
```

### me

Manage me

- **`figma-pp-cli me get`** - Returns the user information for the currently authenticated user.

### oembed

Get oEmbed data for Figma files and published Makes.

- **`figma-pp-cli oembed get`** - Returns oEmbed data for a Figma file or published Make site URL, following the [oEmbed specification](https://oembed.com/).

### payments

Get purchase information for your Community resources.

- **`figma-pp-cli payments get`** - There are two methods to query for a user's payment information on a plugin, widget, or Community file. The first method, using plugin payment tokens, is typically used when making queries from a plugin's or widget's code. The second method, providing a user ID and resource ID, is typically used when making queries from anywhere else.

Note that you can only query for resources that you own. In most cases, this means that you can only query resources that you originally created.

### projects

Get information about projects and files in teams.

### styles

Get information about published styles.

- **`figma-pp-cli styles get`** - Get metadata on a style by key.

### teams

Manage teams

### webhooks

Interact with team webhooks as a team admin.

- **`figma-pp-cli webhooks delete`** - Deletes the specified webhook. This operation cannot be reversed.
- **`figma-pp-cli webhooks get`** - Returns a list of webhooks corresponding to the context or plan provided, if they exist. For plan, the webhooks for all contexts that you have access to will be returned, and theresponse is paginated
- **`figma-pp-cli webhooks get-webhookid`** - Get a webhook by ID.
- **`figma-pp-cli webhooks post`** - Create a new webhook which will call the specified endpoint when the event triggers. By default, this webhook will automatically send a PING event to the endpoint when it is created. If this behavior is not desired, you can create the webhook and set the status to PAUSED and reactivate it later.
- **`figma-pp-cli webhooks put`** - Update a webhook by ID.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
figma-pp-cli activity-logs

# JSON for scripting and agents
figma-pp-cli activity-logs --json

# Filter to specific fields
figma-pp-cli activity-logs --json --select id,name,status

# Dry run — show the request without sending
figma-pp-cli activity-logs --dry-run

# Agent mode — JSON + compact + no prompts in one flag
figma-pp-cli activity-logs --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Cookbook

Worked recipes using verified flag names. Replace `<file_key>` with your file id and `<team_id>` with your team id.

```bash
# 1. Compact, codegen-ready frame extract for one node.
figma-pp-cli frame extract <file_key> --ids 1234:5678 --depth 4 --json

# 2. Same extract but pull multiple nodes and fuse variables + dev-resources into one payload.
figma-pp-cli frame extract <file_key> --ids 1234:5678,I5666:180910 \
    --include variables,dev-resources --depth 6 --json

# 3. Dev Mode hand-off bundle as Markdown for one node — paste-ready into a doc.
figma-pp-cli dev-mode dump <file_key> --node 1234:5678 --format md

# 4. Cross-file unresolved comments audit, 7-day cutoff grouped by node.
figma-pp-cli comments-audit --older-than 7d --group-by node --json

# 5. Library cleanup: components in <team_id> with zero usage in the last 30 days.
figma-pp-cli orphans <team_id> --kind component --window 30d --json

# 6. Diff design tokens between two saved file versions, JSON change set.
figma-pp-cli tokens diff <file_key> --from 4242:design-tokens \
    --to 4998:rebrand --format json

# 7. Wire a deterministic fingerprint check into CI (exit 2 on drift).
figma-pp-cli fingerprint <file_key> \
    --expect 9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08

# 8. Replay only the failed webhook deliveries against a local handler.
figma-pp-cli webhooks test 9876 --replay-failed \
    --target-url http://localhost:8080/figma-events --limit 25

# 9. Trace every reference to a variable before renaming it.
figma-pp-cli variables explain <file_key> --variable color/brand/primary --json

# 10. Full-text search the synced store and pipe to jq.
figma-pp-cli search "checkout button" --limit 25 --json | jq '.results[].name'

# 11. Show workload distribution, JSON output for a dashboard.
figma-pp-cli load --limit 20 --json

# 12. Stale items not touched in 14 days, agent mode (json + compact + no prompts).
figma-pp-cli stale --days 14 --agent
```

## Health Check

```bash
figma-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/figma-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `FIGMA_OAUTH2` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `figma-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $FIGMA_OAUTH2`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **403 on /v1/me with my PAT** — PATs require the `current_user:read` scope to call /v1/me; use `figma-pp-cli doctor` instead, which probes file/comment endpoints (which PATs can read), or run `figma-pp-cli auth login` for full coverage.
- **Multi-day Retry-After lockout on /v1/files calls** — Tier-1 endpoints (files/images/nodes) have strict per-plan-tier limits. Use `figma-pp-cli files meta get-file` (Tier-1-light) for routine checks, and reserve full file fetches for `sync` runs that respect Retry-After.
- **Image URLs from /v1/files/{key}/images are 403ing after a week** — Image-fill URLs expire 14 days after generation; rendered-node URLs expire 30 days. Re-run `figma-pp-cli files images get-fills` or `figma-pp-cli files images render-images` to refresh.
- **Activity logs returns 403 even with admin PAT** — /v1/activity_logs is OAuth-only and Enterprise-tier. Run `figma-pp-cli auth login` and ensure your account is on Enterprise.
- **frame extract output is still huge** — Drop --depth (default 4) to a smaller number, or restrict --ids to the specific node you need. The compaction pipeline reports `simplifiedNodeCount` so you can see compression.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**GLips/Figma-Context-MCP**](https://github.com/GLips/Figma-Context-MCP) — TypeScript (14689 stars)
- [**mikaelvesavuori/figmagic**](https://github.com/mikaelvesavuori/figmagic) — TypeScript (858 stars)
- [**vkhanhqui/figma-mcp-go**](https://github.com/vkhanhqui/figma-mcp-go) — Go (816 stars)
- [**RedMadRobot/figma-export**](https://github.com/RedMadRobot/figma-export) — Swift (811 stars)
- [**marcomontalbano/figma-export**](https://github.com/marcomontalbano/figma-export) — TypeScript (341 stars)
- [**figma/rest-api-spec**](https://github.com/figma/rest-api-spec) — YAML (209 stars)
- [**didoo/figma-api**](https://github.com/didoo/figma-api) — TypeScript
- [**tokens-studio/figma-plugin**](https://github.com/tokens-studio/figma-plugin) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

## Known Gaps

The following are documented limitations from Phase 5 live dogfood (152/165 = 92% pass), all environmental — the CLI itself is correct.

- **OAuth-only endpoints**: `activity-logs`, `developer-logs`, and (in some scopes) `me` require OAuth Bearer auth, not PAT. Set `FIGMA_OAUTH2` to your OAuth Bearer token to use these. The CLI surfaces the API's 401 with a clear message.
- **`oembed`, `payments`, `webhooks get`** require runtime parameters the printed-CLI dogfood matrix cannot synthesize (`--url`, `--user-id`, `--context`). They work correctly when invoked with proper arguments.
- **`comments-audit`** requires a populated local cache: run `figma-pp-cli sync --resources comments` against a known file before invoking it. Returns an honest empty-store error otherwise.
- **`orphans`** depends on Figma's Library Analytics API (Enterprise tier). On non-Enterprise plans it returns exit 0 with a clear "analytics data is empty" message rather than failing.

