# DNS Made Easy CLI

**Every DNS Made Easy operation, plus a local record store that makes cross-zone search, drift detection, and 'where is this IP used' instant and offline.**

A single Go binary for the DNS Made Easy V2.0 API: managed and secondary DNS, records, templates, folders, and failover — all with agent-native --json/--select output. Once synced, a local SQLite mirror of your whole account powers commands no incumbent has: where-used across every zone, drift between snapshots, BIND export, and dry-run bulk changes.

## Install

The recommended path installs both the `dnsmadeeasy-pp-cli` binary and the `pp-dnsmadeeasy` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install dnsmadeeasy
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install dnsmadeeasy --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install dnsmadeeasy --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install dnsmadeeasy --agent claude-code
npx -y @mvanhorn/printing-press-library install dnsmadeeasy --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/dnsmadeeasy/cmd/dnsmadeeasy-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/dnsmadeeasy-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install dnsmadeeasy --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-dnsmadeeasy --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-dnsmadeeasy --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install dnsmadeeasy --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/dnsmadeeasy-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `DNSMADEEASY_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/dnsmadeeasy/cmd/dnsmadeeasy-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "dnsmadeeasy": {
      "command": "dnsmadeeasy-pp-mcp",
      "env": {
        "DNSMADEEASY_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

DNS Made Easy signs every request with HMAC-SHA1. You need two values from the control panel (Config → API Keys): an API Key and a Secret Key. Set DNSMADEEASY_API_KEY and DNSMADEEASY_API_SECRET (or run 'dnsmadeeasy-pp-cli auth set-token'). The CLI computes the x-dnsme-hmac / x-dnsme-requestDate signature for you on every call. A sandbox account (api.sandbox.dnsmadeeasy.com) is available for safe testing.

## Quick Start

```bash
# Check credentials and API reachability before anything else.
dnsmadeeasy-pp-cli doctor --dry-run

# List every managed zone on the account.
dnsmadeeasy-pp-cli domains list

# Mirror every zone's records into the local store (the framework 'sync' cannot mirror hierarchical records).
dnsmadeeasy-pp-cli sync-records

# Find every record across all zones pointing at an IP.
dnsmadeeasy-pp-cli where-used 52.10.4.7 --agent

# Audit all zones for dangling CNAMEs, TTL issues, and missing mail-security records.
dnsmadeeasy-pp-cli health --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`where-used`** — Find every zone and record whose value or CNAME/MX/ANAME target matches an IP, hostname, or string across your entire account.

  _Pick this before an IP migration or decommission to see everywhere a value still appears, instead of paginating every zone by hand._

  ```bash
  dnsmadeeasy-pp-cli where-used 52.10.4.7 --agent
  ```
- **`drift`** — Diff the two most recent local sync snapshots and report which records were added, changed, or removed.

  _Use for post-deploy verification that only intended records changed, or to catch out-of-band edits the API would never surface._

  ```bash
  dnsmadeeasy-pp-cli drift --agent
  ```

### Migration and backup
- **`export`** — Serialize a zone's records from the local mirror to a standards-compliant BIND zone file or JSON, with no API call.

  _Reach for this to back up a zone, hand off to another provider, or review a full zone as one document._

  ```bash
  dnsmadeeasy-pp-cli export example.com --format bind
  ```
- **`acme-purge`** — Preview all _acme-challenge TXT records, or only those older than a requested age, then delete selected records with `--apply`.

  _Run after certificate automation to clean up the challenge records lego/ACME clients leave behind, account-wide._

  ```bash
  dnsmadeeasy-pp-cli acme-purge --older-than 24h --dry-run
  ```

### Cross-zone insight
- **`health`** — Audit every zone for dangling CNAMEs, out-of-range TTLs, zones missing SPF/DMARC/CAA, and duplicate or conflicting records.

  _Run before a cutover or as a periodic sweep to surface takeover risk and mail-security gaps across every zone at once._

  ```bash
  dnsmadeeasy-pp-cli health --agent
  ```
- **`bulk-apply`** — Select records by value and type across all zones locally, preview the plan, then apply the change via the real multi-record update endpoint per affected zone.

  _Use for IP migrations that touch many zones; --dry-run shows exactly what will change before any write._

  ```bash
  dnsmadeeasy-pp-cli bulk-apply --match 52.10.4.7 --set 52.10.4.9 --type A --dry-run
  ```

## Recipes

### Plan an IP migration safely

```bash
dnsmadeeasy-pp-cli where-used 52.10.4.7 --agent --select zone,name,type
```

See every affected record across all zones before changing anything.

### Apply the migration across zones

```bash
dnsmadeeasy-pp-cli bulk-apply --match 52.10.4.7 --set 52.10.4.9 --type A --dry-run
```

Preview the multi-zone change; drop --dry-run to apply via updateMulti.

### Back up a zone to BIND

```bash
dnsmadeeasy-pp-cli export example.com --format bind
```

Produce a standards zone file from the local mirror for backup or handoff.

### Review what changed after a deploy

```bash
dnsmadeeasy-pp-cli drift --agent
```

Diff the two latest sync snapshots to confirm only intended records changed.

### Search records offline

```bash
dnsmadeeasy-pp-cli search mail --type records --agent --select zone,name,value
```

Full-text search across every synced record without hitting the API.

## Usage

Run `dnsmadeeasy-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `DNSMADEEASY_CONFIG_DIR`, `DNSMADEEASY_DATA_DIR`, `DNSMADEEASY_STATE_DIR`, or `DNSMADEEASY_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `DNSMADEEASY_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export DNSMADEEASY_HOME=/srv/dnsmadeeasy
dnsmadeeasy-pp-cli doctor
```

Under `DNSMADEEASY_HOME=/srv/dnsmadeeasy`, the four dirs resolve to `/srv/dnsmadeeasy/config`, `/srv/dnsmadeeasy/data`, `/srv/dnsmadeeasy/state`, and `/srv/dnsmadeeasy/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "dnsmadeeasy": {
      "command": "dnsmadeeasy-pp-mcp",
      "env": {
        "DNSMADEEASY_HOME": "/srv/dnsmadeeasy"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `DNSMADEEASY_DATA_DIR` overrides an explicit `--home` for that kind. Use `DNSMADEEASY_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `DNSMADEEASY_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `dnsmadeeasy-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### acl

Manage zone transfer ACLs

- **`dnsmadeeasy-pp-cli acl delete`** - Delete a transfer ACL
- **`dnsmadeeasy-pp-cli acl get`** - Get a transfer ACL by id
- **`dnsmadeeasy-pp-cli acl list`** - List transfer ACLs

### contact-lists

Manage failover contact lists

- **`dnsmadeeasy-pp-cli contact-lists get`** - Get a contact list by id
- **`dnsmadeeasy-pp-cli contact-lists list`** - List failover contact lists

### domains

Manage managed (primary) DNS domains

- **`dnsmadeeasy-pp-cli domains create`** - Create a managed domain
- **`dnsmadeeasy-pp-cli domains delete`** - Delete a managed domain and all its records
- **`dnsmadeeasy-pp-cli domains find`** - Look up a managed domain by its name
- **`dnsmadeeasy-pp-cli domains get`** - Get a managed domain by numeric id
- **`dnsmadeeasy-pp-cli domains list`** - List all managed domains
- **`dnsmadeeasy-pp-cli domains update`** - Update a managed domain's configuration

### folders

Manage security folders

- **`dnsmadeeasy-pp-cli folders get`** - Get a security folder by id
- **`dnsmadeeasy-pp-cli folders list`** - List security folders

### ipsets

Manage secondary DNS IP sets

- **`dnsmadeeasy-pp-cli ipsets create`** - Create a secondary DNS IP set
- **`dnsmadeeasy-pp-cli ipsets delete`** - Delete a secondary DNS IP set
- **`dnsmadeeasy-pp-cli ipsets get`** - Get a secondary DNS IP set by id
- **`dnsmadeeasy-pp-cli ipsets list`** - List secondary DNS IP sets

### monitor

Manage per-record failover and system monitoring

- **`dnsmadeeasy-pp-cli monitor get`** - Get failover/monitor configuration for a record
- **`dnsmadeeasy-pp-cli monitor update`** - Update failover/monitor configuration for a record

### records

Manage DNS records within a managed domain

- **`dnsmadeeasy-pp-cli records create`** - Create a DNS record in a domain
- **`dnsmadeeasy-pp-cli records delete`** - Delete a single DNS record
- **`dnsmadeeasy-pp-cli records get`** - Get a single record by id
- **`dnsmadeeasy-pp-cli records list`** - List records in a domain, optionally filtered by type or name
- **`dnsmadeeasy-pp-cli records update`** - Update a DNS record

### secondary

Manage secondary (slave) DNS domains

- **`dnsmadeeasy-pp-cli secondary delete`** - Delete a secondary DNS domain
- **`dnsmadeeasy-pp-cli secondary get`** - Get a secondary DNS domain by id
- **`dnsmadeeasy-pp-cli secondary list`** - List all secondary DNS domains

### soa

Manage custom SOA records

- **`dnsmadeeasy-pp-cli soa create`** - Create a custom SOA record
- **`dnsmadeeasy-pp-cli soa delete`** - Delete a custom SOA record
- **`dnsmadeeasy-pp-cli soa get`** - Get a custom SOA record by id
- **`dnsmadeeasy-pp-cli soa list`** - List custom SOA records

### template-records

Manage records within a DNS template

- **`dnsmadeeasy-pp-cli template-records create`** - Create a record in a template
- **`dnsmadeeasy-pp-cli template-records delete`** - Delete a record from a template
- **`dnsmadeeasy-pp-cli template-records list`** - List records in a template

### templates

Manage DNS record templates

- **`dnsmadeeasy-pp-cli templates create`** - Create a DNS template
- **`dnsmadeeasy-pp-cli templates delete`** - Delete a DNS template
- **`dnsmadeeasy-pp-cli templates get`** - Get a DNS template by id
- **`dnsmadeeasy-pp-cli templates list`** - List DNS templates
- **`dnsmadeeasy-pp-cli templates update`** - Update a DNS template

### vanity

Manage Vanity DNS configurations

- **`dnsmadeeasy-pp-cli vanity delete`** - Delete a Vanity DNS configuration
- **`dnsmadeeasy-pp-cli vanity get`** - Get a Vanity DNS configuration by id
- **`dnsmadeeasy-pp-cli vanity list`** - List Vanity DNS configurations


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
dnsmadeeasy-pp-cli acl list

# JSON for scripting and agents
dnsmadeeasy-pp-cli acl list --json

# Filter to specific fields
dnsmadeeasy-pp-cli acl list --json --select id,name,status

# Dry run — show the request without sending
dnsmadeeasy-pp-cli acl list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
dnsmadeeasy-pp-cli acl list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
dnsmadeeasy-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `dnsmadeeasy-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/dnsmadeeasy-pp-cli/config.toml`; `--home`, `DNSMADEEASY_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `DNSMADEEASY_API_KEY` | per_call | Yes | Set to your API credential. |
| `DNSMADEEASY_API_SECRET` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `dnsmadeeasy-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `dnsmadeeasy-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $DNSMADEEASY_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 or signature errors on every call** — Re-check DNSMADEEASY_API_KEY and DNSMADEEASY_API_SECRET; run 'dnsmadeeasy-pp-cli doctor'. The secret signs the request date, so a wrong secret fails all calls.
- **HTTP 429 / rate limited** — DNS Made Easy allows ~150 requests per 5 minutes. Use 'sync' once then query the local store; the CLI honors x-dnsme-requestsRemaining and backs off.
- **where-used / drift return nothing** — Run 'dnsmadeeasy-pp-cli sync-records' first; these commands read the local mirror.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**kigster/dnsmadeeasy (dme)**](https://github.com/kigster/dnsmadeeasy) — Ruby
- [**DNSMadeEasy/dme-go-client**](https://github.com/DNSMadeEasy/dme-go-client) — Go
- [**jswank/dnsme**](https://github.com/jswank/dnsme) — Go
- [**mhenderson-so/godnsmadeeasy**](https://github.com/mhenderson-so/godnsmadeeasy) — Go
- [**john-k/dnsmadeeasy**](https://github.com/john-k/dnsmadeeasy) — Go
- [**soniah/dnsmadeeasy**](https://github.com/soniah/dnsmadeeasy) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
