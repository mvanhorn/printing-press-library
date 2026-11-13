# WhatsApp Business Tools CLI

**Operate the official WhatsApp Business MCP with reviewable writes and captured evidence for integration checks.**

Discover the correct business assets, inspect actual setup state, and prepare schema-aware operations. Compare saved configurations, templates and MCP contracts with explicit scope and capture times.

## Source build

Build from this module with Go 1.26.11 or newer. The module selects the corrected Go 1.27.3 toolchain automatically:

```bash
go build -o whatsapp-business-pp-cli ./cmd/whatsapp-business-pp-cli
go build -o whatsapp-business-pp-mcp ./cmd/whatsapp-business-pp-mcp
./whatsapp-business-pp-cli doctor --codex-auth --json
./whatsapp-business-pp-cli connection ping --codex-auth --agent
```

Public installers require a merged catalog release. Before that release, use the source build above.

<details>
<summary>Public-library installation (requires a merged catalog release)</summary>

## Install

The recommended path installs both the `whatsapp-business-pp-cli` binary and the `pp-whatsapp-business` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install whatsapp-business
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install whatsapp-business --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install whatsapp-business --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install whatsapp-business --agent claude-code
npx -y @mvanhorn/printing-press-library install whatsapp-business --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.11 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/whatsapp-business/cmd/whatsapp-business-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/whatsapp-business-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine whatsapp-business-pp-cli`. On Unix, mark it executable: `chmod +x whatsapp-business-pp-cli`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install whatsapp-business --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-whatsapp-business --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-whatsapp-business --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install whatsapp-business --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/whatsapp-business-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `WHATSAPP_BUSINESS_MCP_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/whatsapp-business/cmd/whatsapp-business-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS, `%APPDATA%\Claude\claude_desktop_config.json` on Windows):

```json
{
  "mcpServers": {
    "whatsapp-business": {
      "command": "whatsapp-business-pp-mcp",
      "env": {
        "WHATSAPP_BUSINESS_MCP_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>


</details>

## Authentication

Meta OAuth is required. Use --codex-auth (or WHATSAPP_BUSINESS_USE_CODEX_AUTH=1) to explicitly read the existing native Codex Meta OAuth credential on this Mac. Run auth codex --launch when that native session needs sign-in. An independently registered client was rejected by Meta; independent OAuth is not claimed. A resource-specific bearer can instead be supplied through the configured credential seam.

## Quick Start

```bash
# Inspect the local CLI health-check plan without credentials or network.
whatsapp-business-pp-cli doctor --dry-run

# Check the authenticated MCP connection.
whatsapp-business-pp-cli connection ping --codex-auth --agent

# Discover authorized business scopes before selecting one.
whatsapp-business-pp-cli businesses list --codex-auth --agent

```

## Recipes

### Inspect captured scope relationships

```bash
whatsapp-business-pp-cli ops scope-map --snapshot capture-001 --agent --select relationships,gaps
```

Review a chosen inventory capture and its unresolved relationships.

### Compare captured asset state

```bash
whatsapp-business-pp-cli ops changes --before capture-001 --after capture-002 --agent
```

Compare matching scopes without treating incomplete pages as deletions.

### Extract template requirements

```bash
whatsapp-business-pp-cli ops template-checklist --template template-capture-001 --agent
```

List component requirements and unsupported mappings without sending.

### Inspect catalog contract changes

```bash
whatsapp-business-pp-cli ops catalog-drift --before catalog-001 --after catalog-002 --agent
```

Review potentially breaking tool and required-input changes.

## Unique Features

### Captured integration evidence
- **`ops scope-map`** — See the captured business, account and phone relationships for an explicitly selected scope, including gaps and shared-account roles.

  _Use before preparing an operation to verify the captured business/account/phone relationships and evidence gaps._

  ```bash
  whatsapp-business-pp-cli ops scope-map --snapshot capture-001 --agent --select relationships,gaps
  ```
- **`ops catalog-drift`** — Compare saved MCP catalogs and identify removed tools, new required inputs and changed schema paths that could break saved commands.

  _Use before updating scripts against the beta MCP contract._

  ```bash
  whatsapp-business-pp-cli ops catalog-drift --before catalog-001 --after catalog-002 --agent
  ```
- **`ops changes`** — Compare two captures of the same selected scope and see changed fields, additions and evidence gaps without mistaking incomplete pages for deletions.

  _Use when resuming integration work to compare the exact same scope against its previous capture._

  ```bash
  whatsapp-business-pp-cli ops changes --before capture-001 --after capture-002 --agent
  ```

### Template review
- **`ops template-checklist`** — Extract the values a saved template needs and flag components the MCP send tool cannot express, without sending or inventing parameter values.

  _Use before composing template values to identify unsupported component mappings without sending._

  ```bash
  whatsapp-business-pp-cli ops template-checklist --template template-capture-001 --agent
  ```
- **`ops template-compare`** — Compare two explicitly chosen captured templates across accounts, with field and component differences tied to their original account and capture time.

  _Use when reviewing two explicitly selected account-specific template documents._

  ```bash
  whatsapp-business-pp-cli ops template-compare --left template-001 --right template-002 --agent
  ```

## Usage

Run `whatsapp-business-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `WHATSAPP_BUSINESS_CONFIG_DIR`, `WHATSAPP_BUSINESS_DATA_DIR`, `WHATSAPP_BUSINESS_STATE_DIR`, or `WHATSAPP_BUSINESS_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `WHATSAPP_BUSINESS_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export WHATSAPP_BUSINESS_HOME=/srv/whatsapp-business
whatsapp-business-pp-cli doctor
```

Under `WHATSAPP_BUSINESS_HOME=/srv/whatsapp-business`, the four dirs resolve to `/srv/whatsapp-business/config`, `/srv/whatsapp-business/data`, `/srv/whatsapp-business/state`, and `/srv/whatsapp-business/cache`.

On Windows, set the same variable in PowerShell:

```powershell
$env:WHATSAPP_BUSINESS_HOME = "C:\srv\whatsapp-business"
whatsapp-business-pp-cli doctor
```

Under `$env:WHATSAPP_BUSINESS_HOME = "C:\srv\whatsapp-business"`, the four dirs resolve to `C:\srv\whatsapp-business\config`, `C:\srv\whatsapp-business\data`, `C:\srv\whatsapp-business\state`, and `C:\srv\whatsapp-business\cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "whatsapp-business": {
      "command": "whatsapp-business-pp-mcp",
      "env": {
        "WHATSAPP_BUSINESS_HOME": "/srv/whatsapp-business"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `WHATSAPP_BUSINESS_DATA_DIR` overrides an explicit `--home` for that kind. Use `WHATSAPP_BUSINESS_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `WHATSAPP_BUSINESS_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Use `agent-context` to inspect the resolved paths before running automation.

## Commands

The live MCP was queried during this print. Every official tool below has a schema-aware command. Use the leaf's `--help` for exact parameters; `--account-id` maps to the upstream account identifier.

| Command | Upstream tool |
|---|---|
| `businesses list` | `whatsapp_biz_businesses` |
| `connection ping` | `ping` |
| `phones add` | `whatsapp_biz_add_phone_number` |
| `templates get` | `whatsapp_biz_get_template` |
| `templates archival` | `whatsapp_biz_manage_templates_archival` |
| `apps publish` | `whatsapp_biz_publish_app` |
| `templates list` | `whatsapp_biz_list_templates` |
| `phones send-code` | `whatsapp_biz_send_verification_code` |
| `onboarding status` | `whatsapp_biz_onboarding_status` |
| `auth system-user-link` | `whatsapp_biz_system_user_token` |
| `templates update` | `whatsapp_biz_update_template` |
| `webhooks configure` | `whatsapp_biz_configure_webhooks` |
| `accounts list` | `whatsapp_biz_accounts` |
| `templates create` | `whatsapp_biz_create_template` |
| `templates from-library` | `whatsapp_biz_create_template_from_library` |
| `phones list` | `whatsapp_biz_phone_numbers` |
| `webhooks subscribe` | `whatsapp_biz_subscribe_webhook` |
| `messages send` | `whatsapp_biz_send_message` |
| `templates delete` | `whatsapp_biz_delete_template` |
| `businesses verify` | `whatsapp_biz_verify_business` |
| `templates library` | `whatsapp_biz_template_library` |
| `phones register` | `whatsapp_biz_register_phone_number` |
| `phones verify` | `whatsapp_biz_verify_phone_number` |
| `payments configure` | `whatsapp_biz_configure_payments` |

`apps publish` reads publication status and returns a link; it does not publish an app. `businesses verify` reads verification status and returns a link; document submission remains in Meta's UI. `auth system-user-link` returns a secure setup link rather than generating or printing a token.

Additional evidence commands: `catalog capture`, `snapshots list`, `snapshots show`, `sync`, `search` and `sql`. `sync` captures exactly one named read-only tool and its explicit `--param` selection. It does not sweep businesses automatically. SQL uses a read-only connection and caps results at 1000 rows. Search bounds both returned matches and scanned captures.

All five `ops` commands read only local captures. Use the capture IDs returned by prior reads. Captures preserve timestamps, source tool, exact arguments and completeness. Missing entities on a partial page remain unobserved. Use `--max-age 0` to disable the default age warning on local inspections.

The hidden raw RPC command is preview-only. Executable operations use the schema-aware commands and check the live catalog before transmitting a tool call.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
whatsapp-business-pp-cli whatsapp-business-tools --jsonrpc 2.0

# JSON for scripting and agents
whatsapp-business-pp-cli whatsapp-business-tools --jsonrpc 2.0 --json
# Filter to specific fields
whatsapp-business-pp-cli whatsapp-business-tools --jsonrpc 2.0 --json --select error,id,jsonrpc

# Dry run — show the request without sending
whatsapp-business-pp-cli whatsapp-business-tools --jsonrpc 2.0 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
whatsapp-business-pp-cli whatsapp-business-tools --jsonrpc 2.0 --agent
```

`--csv` prefixes a leading `'` on cells and header keys that start with `=`, `+`, `-`, `@`, a tab, or a line break (including fullwidth `＝＋－＠`) so a spreadsheet does not run them as formulas. Plain signed numbers such as `-5` and `+2.5e3` are left unchanged.

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Retries** - transient reads have bounded retries; domain writes are never retried automatically.
- **Explicit confirmation** - domain mutations preview until the separate `--confirm` flag is passed after exact action approval; `--yes` cannot execute them.
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
whatsapp-business-pp-cli doctor
```

Checks local credential availability and reports the capture path. It does not contact Meta. Use `connection ping --codex-auth --agent` for connectivity.

Selected output from a verified native session:

```bash
whatsapp-business-pp-cli doctor --codex-auth --json --select status,auth_available,network_checked
```

```json
{"auth_available":true,"network_checked":false,"status":"ok"}
```


## Configuration

Use `whatsapp-business-pp-cli agent-context` to inspect its `paths` object. The default macOS config file is `~/.config/whatsapp-business-pp-cli/config.toml`; the runtime path resolver determines other platforms. `--home`, `WHATSAPP_BUSINESS_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `WHATSAPP_BUSINESS_MCP_TOKEN` | per_call | Alternative | Resource-specific bearer; omitted when native OAuth reuse is enabled. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `whatsapp-business-pp-cli doctor` to check credentials
- Use `doctor --codex-auth --json` to check native credential availability without printing secrets.
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Use `snapshots list --agent` for local capture IDs or `businesses list --codex-auth --agent` for live discovery.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**WBMCP**](https://github.com/saravanaspar/WBMCP) — TypeScript
- [**whatsapp-business-mcp**](https://github.com/Gonzalez8/whatsapp-business-mcp) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

## Cookbook

Select actual IDs from discovery before using the shell variables below. `SELECTION_JSON` is an object containing the chosen `business_id`; capture IDs (`BEFORE`, `AFTER`, `LEFT`, `RIGHT`, `TEMPLATE_CAPTURE`) must come from the saved evidence. These are explicit user choices, not defaults.

### Discover a business before selecting its assets

```bash
whatsapp-business-pp-cli businesses list --codex-auth --agent
whatsapp-business-pp-cli accounts list --arguments "$SELECTION_JSON" --codex-auth --agent
```

### Read onboarding and webhook state

```bash
whatsapp-business-pp-cli onboarding status --arguments "$SELECTION_JSON" --codex-auth --agent
```

Read `done`, `todo` and `unknown` as reported; a callback URL alone does not prove account subscriptions or delivery.

### Capture one read operation

```bash
whatsapp-business-pp-cli sync --resources ping --codex-auth --agent
```

This captures only the selected read, without sweeping businesses.

### Locate saved evidence

```bash
whatsapp-business-pp-cli snapshots list --agent
```

Choose capture IDs from this result; a bare inspection command reports required inputs without inventing a result.

### Compare the MCP contract

```bash
whatsapp-business-pp-cli catalog capture --codex-auth --agent
whatsapp-business-pp-cli ops catalog-drift --before "$BEFORE" --after "$AFTER" --agent
```

Both selected captures must describe the same actual endpoint and negotiated protocol.

### Compare the same asset scope over time

```bash
whatsapp-business-pp-cli ops changes --before "$BEFORE" --after "$AFTER" --agent
```

Scopes and filters must match. Unknown scope is rejected; incomplete pages cannot establish deletion.

### Compare two template documents

```bash
whatsapp-business-pp-cli ops template-compare --left "$LEFT" --right "$RIGHT" --agent
```

Select saved get-template captures, including captures from different accounts when that is the intended comparison.

### Extract a template's required values

```bash
whatsapp-business-pp-cli ops template-checklist --template "$TEMPLATE_CAPTURE" --agent
```

Unsupported mappings remain explicit; the checklist does not establish delivery eligibility or approval.

### Search saved template evidence

```bash
whatsapp-business-pp-cli search "approved" --type whatsapp_biz_list_templates --agent
```

The search is literal and bounded, and reports its scan cap.

### Query the capture store after a read

```bash
whatsapp-business-pp-cli sql "SELECT count(*) AS captures FROM mcp_captures" --agent
```

SQL is read-only and capped at 1000 returned rows. The capture table exists after a successful captured read.

### Preview a message without sending

```bash
whatsapp-business-pp-cli messages send --dry-run --json
```

This shows the schema-aware request plan without reading input or sending. Inspect `messages send --help`, then provide the exact chosen target and content through `--stdin` for a populated preview. Only a separate `--confirm` flag after approval of the exact action enables execution; JSON `confirm`, `--yes` and `--agent` cannot. Harnesses refuse confirmed writes.

## Runtime boundaries

Native OAuth reuse currently supports this Mac's Codex Keychain entry; other platforms require a resource-specific bearer through the normal config seam. This client does not register itself under Codex's OAuth identity. No generic Cloud API token is assumed to work with this MCP. Expired native sessions require `auth codex --launch` through the supported native client.

No inbox, chat history, service-window prediction, delivery guarantee, cost estimate, automatic app publication or independent OAuth registration is claimed. No remote writes were used to validate this print. Schema and safety-hint changes in the beta server fail closed before a domain call.

## Adapter boundaries

Raw JSONL import is disabled for this MCP resource. Use schema-aware domain commands for writes and `sync --arguments` for typed read arguments. The JSON object is validated against the selected read tool; `--param` remains available for scalar selections. Inspection output retains evidence fields under `--agent` and `--compact`; use `--select` to intentionally narrow the result.
