---
name: pp-dnsmadeeasy
description: "Every DNS Made Easy operation, plus a local record store that makes cross-zone search, drift detection, and 'where is this IP used' instant and offline. Trigger phrases: `where is this IP used in DNS Made Easy`, `export my DNS Made Easy zone`, `what DNS records changed`, `bulk update DNS records across zones`, `audit my DNS zones`, `use dnsmadeeasy`, `run dnsmadeeasy`."
author: "Derick Ng"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - dnsmadeeasy-pp-cli
    install:
      - kind: go
        bins: [dnsmadeeasy-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/developer-tools/dnsmadeeasy/cmd/dnsmadeeasy-pp-cli
---

# DNS Made Easy — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `dnsmadeeasy-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install dnsmadeeasy --cli-only
   ```
2. Verify: `dnsmadeeasy-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/dnsmadeeasy/cmd/dnsmadeeasy-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

A single Go binary for the DNS Made Easy V2.0 API: managed and secondary DNS, records, templates, folders, and failover — all with agent-native --json/--select output. Once synced, a local SQLite mirror of your whole account powers commands no incumbent has: where-used across every zone, drift between snapshots, BIND export, and dry-run bulk changes.

## When to Use This CLI

Use this CLI to manage DNS Made Easy zones and records from scripts or agents, and especially to answer account-wide questions the web UI and thin API wrappers can't: where an IP or hostname is used across every zone, what changed since the last sync, and which zones fail DNS hygiene. It is ideal for IP migrations, ACME cleanup, backups, and post-deploy verification.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to register or buy domain names — DNS Made Easy hosts DNS, it is not a registrar.
- Do not use it to manage DNS at other providers (Cloudflare, Route 53, Namecheap) — it only speaks the DNS Made Easy API.
- Do not use it as a live resolver or dig replacement — it manages authoritative records, it does not perform recursive lookups.

## Unique Capabilities

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

## Command Reference

**acl** — Manage zone transfer ACLs

- `dnsmadeeasy-pp-cli acl delete` — Delete a transfer ACL
- `dnsmadeeasy-pp-cli acl get` — Get a transfer ACL by id
- `dnsmadeeasy-pp-cli acl list` — List transfer ACLs

**contact-lists** — Manage failover contact lists

- `dnsmadeeasy-pp-cli contact-lists get` — Get a contact list by id
- `dnsmadeeasy-pp-cli contact-lists list` — List failover contact lists

**domains** — Manage managed (primary) DNS domains

- `dnsmadeeasy-pp-cli domains create` — Create a managed domain
- `dnsmadeeasy-pp-cli domains delete` — Delete a managed domain and all its records
- `dnsmadeeasy-pp-cli domains find` — Look up a managed domain by its name
- `dnsmadeeasy-pp-cli domains get` — Get a managed domain by numeric id
- `dnsmadeeasy-pp-cli domains list` — List all managed domains
- `dnsmadeeasy-pp-cli domains update` — Update a managed domain's configuration

**folders** — Manage security folders

- `dnsmadeeasy-pp-cli folders get` — Get a security folder by id
- `dnsmadeeasy-pp-cli folders list` — List security folders

**ipsets** — Manage secondary DNS IP sets

- `dnsmadeeasy-pp-cli ipsets create` — Create a secondary DNS IP set
- `dnsmadeeasy-pp-cli ipsets delete` — Delete a secondary DNS IP set
- `dnsmadeeasy-pp-cli ipsets get` — Get a secondary DNS IP set by id
- `dnsmadeeasy-pp-cli ipsets list` — List secondary DNS IP sets

**monitor** — Manage per-record failover and system monitoring

- `dnsmadeeasy-pp-cli monitor get` — Get failover/monitor configuration for a record
- `dnsmadeeasy-pp-cli monitor update` — Update failover/monitor configuration for a record

**records** — Manage DNS records within a managed domain

- `dnsmadeeasy-pp-cli records create` — Create a DNS record in a domain
- `dnsmadeeasy-pp-cli records delete` — Delete a single DNS record
- `dnsmadeeasy-pp-cli records get` — Get a single record by id
- `dnsmadeeasy-pp-cli records list` — List records in a domain, optionally filtered by type or name
- `dnsmadeeasy-pp-cli records update` — Update a DNS record

**secondary** — Manage secondary (slave) DNS domains

- `dnsmadeeasy-pp-cli secondary delete` — Delete a secondary DNS domain
- `dnsmadeeasy-pp-cli secondary get` — Get a secondary DNS domain by id
- `dnsmadeeasy-pp-cli secondary list` — List all secondary DNS domains

**soa** — Manage custom SOA records

- `dnsmadeeasy-pp-cli soa create` — Create a custom SOA record
- `dnsmadeeasy-pp-cli soa delete` — Delete a custom SOA record
- `dnsmadeeasy-pp-cli soa get` — Get a custom SOA record by id
- `dnsmadeeasy-pp-cli soa list` — List custom SOA records

**template-records** — Manage records within a DNS template

- `dnsmadeeasy-pp-cli template-records create` — Create a record in a template
- `dnsmadeeasy-pp-cli template-records delete` — Delete a record from a template
- `dnsmadeeasy-pp-cli template-records list` — List records in a template

**templates** — Manage DNS record templates

- `dnsmadeeasy-pp-cli templates create` — Create a DNS template
- `dnsmadeeasy-pp-cli templates delete` — Delete a DNS template
- `dnsmadeeasy-pp-cli templates get` — Get a DNS template by id
- `dnsmadeeasy-pp-cli templates list` — List DNS templates
- `dnsmadeeasy-pp-cli templates update` — Update a DNS template

**vanity** — Manage Vanity DNS configurations

- `dnsmadeeasy-pp-cli vanity delete` — Delete a Vanity DNS configuration
- `dnsmadeeasy-pp-cli vanity get` — Get a Vanity DNS configuration by id
- `dnsmadeeasy-pp-cli vanity list` — List Vanity DNS configurations


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
dnsmadeeasy-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

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

## Auth Setup

DNS Made Easy signs every request with HMAC-SHA1. You need two values from the control panel (Config → API Keys): an API Key and a Secret Key. Set DNSMADEEASY_API_KEY and DNSMADEEASY_API_SECRET (or run 'dnsmadeeasy-pp-cli auth set-token'). The CLI computes the x-dnsme-hmac / x-dnsme-requestDate signature for you on every call. A sandbox account (api.sandbox.dnsmadeeasy.com) is available for safe testing.

Run `dnsmadeeasy-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  dnsmadeeasy-pp-cli acl list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and use `--ignore-missing` only when a missing delete target should count as success

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

- Use `--home <dir>` for one invocation, or set `DNSMADEEASY_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `DNSMADEEASY_CONFIG_DIR`, `DNSMADEEASY_DATA_DIR`, `DNSMADEEASY_STATE_DIR`, `DNSMADEEASY_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `DNSMADEEASY_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `dnsmadeeasy-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

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

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `DNSMADEEASY_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `DNSMADEEASY_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
dnsmadeeasy-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
dnsmadeeasy-pp-cli feedback --stdin < notes.txt
dnsmadeeasy-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `DNSMADEEASY_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `DNSMADEEASY_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
dnsmadeeasy-pp-cli profile save briefing --json
dnsmadeeasy-pp-cli --profile briefing acl list
dnsmadeeasy-pp-cli profile list --json
dnsmadeeasy-pp-cli profile show briefing
dnsmadeeasy-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `dnsmadeeasy-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/dnsmadeeasy/cmd/dnsmadeeasy-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add dnsmadeeasy-pp-mcp -- dnsmadeeasy-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which dnsmadeeasy-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   dnsmadeeasy-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `dnsmadeeasy-pp-cli <command> --help`.
