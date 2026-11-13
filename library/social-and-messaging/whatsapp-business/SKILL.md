---
name: pp-whatsapp-business
description: "Operate the official WhatsApp Business MCP with reviewable writes and captured evidence for integration checks. Trigger phrases: `Inspect WhatsApp Business onboarding`, `List WhatsApp Business accounts`, `Compare captured WhatsApp templates`, `Check WhatsApp MCP catalog changes`, `use whatsapp-business`, `run whatsapp-business`."
author: "bobe"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - whatsapp-business-pp-cli
    install:
      - kind: go
        bins: [whatsapp-business-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/social-and-messaging/whatsapp-business/cmd/whatsapp-business-pp-cli
---

Verify an installed binary, or build the current module from source with Go 1.26.11 or newer; it selects the corrected Go 1.27.3 toolchain. Public-install commands require a merged catalog release.

# WhatsApp Business Tools — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `whatsapp-business-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install whatsapp-business --cli-only
   ```
2. Verify: `whatsapp-business-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.11 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/whatsapp-business/cmd/whatsapp-business-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Discover the correct business assets, inspect actual setup state, and prepare schema-aware operations. Compare saved configurations, templates and MCP contracts with explicit scope and capture times.

## When to Use This CLI

Use for the official Meta WhatsApp Business Tools MCP and its authorized business assets. Start with discovery, choose the business explicitly and read captured scope metadata before local comparisons.

## Anti-triggers

Do not use this CLI for:
- Do not use for personal WhatsApp chat history or inbox triage.
- Do not use for Cloud API capabilities absent from this official MCP.
- Do not infer send eligibility, pricing or delivery from local snapshots.

## Unique Capabilities

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

## Command Reference

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

Use the leaf command's `--help` for typed parameters. `--account-id` is the user-facing account flag. Read commands use live MCP state; local `ops` commands require explicitly selected captures.

- `catalog capture --codex-auth --agent`: capture the authenticated tool contract.
- `snapshots list --agent`: discover saved captures and their scope/coverage.
- `snapshots show --snapshot capture-001 --agent`: read one chosen capture.
- `sync --resources whatsapp_biz_accounts --param business_id=1234567890 --codex-auth --agent`: capture one explicit read. Replace the example ID with a discovery result.
- `search "approved" --type whatsapp_biz_list_templates --agent`: bounded, literal offline capture search.
- `sql 'SELECT tool, COUNT(*) AS captures FROM mcp_captures GROUP BY tool' --json`: bounded read-only SQL.

`apps publish` and `businesses verify` return status and handoff links, not completed publication or verification. `auth system-user-link` returns a secure setup link, never a generated token. The raw RPC command only previews.

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

## Auth Setup

Meta OAuth is required. Use --codex-auth (or WHATSAPP_BUSINESS_USE_CODEX_AUTH=1) to explicitly read the existing native Codex Meta OAuth credential on this Mac. Run auth codex --launch when that native session needs sign-in. An independently registered client was rejected by Meta; independent OAuth is not claimed. A resource-specific bearer can instead be supplied through the configured credential seam.

Run `whatsapp-business-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color`.

Global format flags share one contract on promoted, novel, sync, and `--deliver` paths:

- `--json` — one JSON document on stdout (sync progress events go to stderr)
- `--compact` — keep identity/status/timestamp fields; does not change the document vs stream shape
- `--csv` / `--plain` — tabular rows (collection envelopes unwrap to the row array). `--csv` prefixes `'` on a cell or header key that starts with `=`, `+`, `-`, `@`, tab, CR, or LF (fullwidth `＝＋－＠` too) so a spreadsheet does not run it as a formula; a plain signed number such as `-5` or `+2.5e3` is unchanged.
- `--quiet` — one identity value per row, no envelope

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  whatsapp-business-pp-cli connection ping --codex-auth --agent --select tool,data
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Explicit confirmation** — domain mutations preview until the separate `--confirm` flag is passed after approval of the exact action. `--yes` and `--agent` do not execute them.
- **Retries** — transient read failures may be retried within a bounded policy; writes are never retried automatically.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `WHATSAPP_BUSINESS_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `WHATSAPP_BUSINESS_CONFIG_DIR`, `WHATSAPP_BUSINESS_DATA_DIR`, `WHATSAPP_BUSINESS_STATE_DIR`, `WHATSAPP_BUSINESS_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `WHATSAPP_BUSINESS_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- `doctor --codex-auth --json` checks local credential availability, not connectivity. `agent-context` exposes a schema v4 `paths` block for resolved directories; `connection ping --codex-auth --agent` checks the live connection.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

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

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `WHATSAPP_BUSINESS_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `WHATSAPP_BUSINESS_HOME`, or `doctor` will not find credentials left under the former root.

## Local learning

Schema-aware WhatsApp tool commands disable invocation journaling so verification codes and other arguments do not enter learned queries. Captures are the explicit evidence store; they are not chat history.

The generic `recall`, `teach`, `teach-playbook`, `playbook` and `learnings` commands remain available for non-sensitive structural hints. Use only sanitized facts about schema shapes and command usage. Never teach phone numbers, names, customer content, account identifiers, verification values or credentials. Do not expect automatic playbook synthesis from domain calls whose journals are disabled.

`--no-learn` disables the generic learning path for an invocation; `WHATSAPP_BUSINESS_NO_LEARN=true` disables it globally. Inspect `learnings stats --help` or the relevant learning command's help before use. Learning is optional for this integration.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
whatsapp-business-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
whatsapp-business-pp-cli feedback --stdin < notes.txt
whatsapp-business-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `WHATSAPP_BUSINESS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `WHATSAPP_BUSINESS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` as a private 0600 file (exclusive randomly named temp in the destination directory, then rename; a planted symlink is not followed). Binary-response commands write decoded payload bytes (not the base64 JSON envelope) and print a small JSON receipt on stdout; `--json`/`--csv` do not refuse when this sink is set. |
| `webhook:<url>` | POST the output body to the URL (`application/json`). Redirects are not followed; a 3xx response fails the delivery and the body is not sent to the `Location`. |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures, including redirects, return non-zero and log the URL and HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
whatsapp-business-pp-cli profile save briefing --json
whatsapp-business-pp-cli --profile briefing connection ping --codex-auth
whatsapp-business-pp-cli profile list --json
whatsapp-business-pp-cli profile show briefing
whatsapp-business-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `whatsapp-business-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/whatsapp-business/cmd/whatsapp-business-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add whatsapp-business-pp-mcp -- whatsapp-business-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which whatsapp-business-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   whatsapp-business-pp-cli connection ping --agent
   ```
4. If ambiguous, drill into subcommand help: `whatsapp-business-pp-cli connection ping --help`.

## WhatsApp execution contract

Use `--codex-auth` explicitly for the supported native Mac OAuth context, or supply a resource-specific bearer through config. `auth codex` previews login; `auth codex --launch` invokes the native client and opens its sign-in flow. Do not promise independent OAuth registration, which Meta rejected for this custom client.

Every state-changing tool prints a redacted preview unless the separate `--confirm` flag is passed. `--yes`, agent mode and `confirm` inside JSON cannot execute it. Harnesses refuse confirmed writes. A changed live safety hint or input schema blocks execution until reviewed.

A no-selection `ops` result contains an empty array plus required-input guidance; it is not evidence that no differences or requirements exist. Select real saved captures to compute a result. A partial capture cannot prove deletion. Template checklists do not establish current approval, service windows, delivery or cost.
