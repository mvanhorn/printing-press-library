---
name: pp-pangolin
description: "The first agent-native CLI for Pangolin — every endpoint, plus offline SQLite, cross-org audits Trigger phrases: `expose service through pangolin`, `back up pangolin config`, `audit pangolin access`, `which pangolin certs expire`, `list pangolin resources`, `use pangolin`, `run pangolin`."
author: "CFinney"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - pangolin-pp-cli
    install:
      - kind: go
        bins: [pangolin-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/developer-tools/pangolin/cmd/pangolin-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/developer-tools/pangolin/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Pangolin — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `pangolin-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install pangolin --cli-only
   ```
2. Verify: `pangolin-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/pangolin/cmd/pangolin-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Pangolin is the open-source, self-hosted alternative to Cloudflare Tunnels. This CLI exposes all 157 integration-API endpoints with --json, --select, --csv, and --dry-run, mirrors the full configuration into a local SQLite store, and adds commands that span orgs — audit, cert-watch, access-graph, backup, restore, expose — that no Pangolin tool offers today.

## When to Use This CLI

Reach for this CLI whenever the user mentions Pangolin, exposing a homelab service through a tunnel, managing reverse-proxy resources, or auditing who can reach what on a self-hosted edge proxy. The CLI is the right tool for any Pangolin workflow that involves more than a single resource or a single org, or anything an agent should drive programmatically.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`audit`** — Find stale targets, orphaned resources, and missing role bindings across every org you administer, in one command.

  _When the user asks 'is anything broken in my Pangolin stack?', this is the single command that answers it — no dashboard tab-clicking._

  ```bash
  pangolin-pp-cli audit --json --select issues
  ```
- **`cert-watch`** — List certificates sorted by days-until-expiry across every org, with a configurable warning window.

  _When the user asks 'what certs expire soon?', skip the dashboard and get an actionable list with one call._

  ```bash
  pangolin-pp-cli cert-watch --days 30 --json
  ```
- **`access-graph`** — Answer 'who can reach what' by joining users, roles, resources, and orgs into one queryable view.

  _When the user asks 'what does $person have access to?' or 'who can hit this resource?', this is the answer._

  ```bash
  pangolin-pp-cli access-graph --user $USER_ID --json
  ```

### Disaster recovery
- **`backup`** — Export the complete Pangolin configuration — orgs, sites, resources, targets, roles, IdPs — as version-controllable JSON.

  _When the user wants a disaster-recovery snapshot or a config diff between dates, this is the artifact._

  ```bash
  pangolin-pp-cli backup --out pangolin-backup.json
  ```
- **`restore`** — Re-apply a backup against a fresh Pangolin install, in the correct dependency order, with dry-run preview.

  _When the user rebuilds a Pangolin host or moves between machines, this is the way back up._

  ```bash
  pangolin-pp-cli restore pangolin-backup.json --dry-run
  ```

### Agent-native plumbing
- **`expose`** — Create site (if needed), create resource, attach target, bind a role — in one command, with dry-run preview.

  _When the user (or an agent) wants to expose a homelab service, this is the entire workflow in one line._

  ```bash
  pangolin-pp-cli expose grafana --target 192.168.1.50:3000 --site site_42 --role admins --dry-run
  ```
- **`doctor`** — Validate auth, probe both /v1 and /api/v1 mount paths, and report the working integration-API base URL with environment-variable guidance.

  _When the user sets up the CLI for the first time, this catches the single most common misconfiguration before any other command fails._

  ```bash
  pangolin-pp-cli doctor
  ```

## Command Reference

**access-token** — Manage access token

- `pangolin-pp-cli access-token <accessTokenId>` — Delete a access token.

**certificate** — Manage certificate

- `pangolin-pp-cli certificate <certId> <orgId>` — Restart a certificate by ID.

**client** — Manage client

- `pangolin-pp-cli client create` — Update a client by its client ID.
- `pangolin-pp-cli client delete` — Delete a client by its client ID.
- `pangolin-pp-cli client get` — Get a client by its client ID.

**domain** — Manage domain

- `pangolin-pp-cli domain` — Check if a domain namespace is available based on subdomain

**domains** — Manage domains

- `pangolin-pp-cli domains` — List all domain namespaces in the system

**idp** — Manage idp

- `pangolin-pp-cli idp delete` — Delete IDP.
- `pangolin-pp-cli idp get` — Get an IDP by its IDP ID.
- `pangolin-pp-cli idp list` — List all IDP in the system.
- `pangolin-pp-cli idp update` — Create an OIDC IdP.

**maintenance** — Manage maintenance

- `pangolin-pp-cli maintenance` — Get maintenance information for a resource by domain.

**openapi-json** — Manage openapi json

- `pangolin-pp-cli openapi-json` — Get OpenAPI specification as JSON

**openapi-yaml** — Manage openapi yaml

- `pangolin-pp-cli openapi-yaml` — Get OpenAPI specification as YAML

**org** — Manage org

- `pangolin-pp-cli org create` — Update an organization
- `pangolin-pp-cli org delete` — Delete an organization
- `pangolin-pp-cli org get` — Get an organization
- `pangolin-pp-cli org update` — Create a new organization

**orgs** — Manage orgs

- `pangolin-pp-cli orgs` — List all organizations in the system.

**resource** — Manage resource

- `pangolin-pp-cli resource create` — Update a resource.
- `pangolin-pp-cli resource delete` — Delete a resource.
- `pangolin-pp-cli resource get` — Get a resource by resourceId.

**role** — Manage role

- `pangolin-pp-cli role create` — Update a role.
- `pangolin-pp-cli role delete` — Delete a role.
- `pangolin-pp-cli role get` — Get a role.

**site** — Manage site

- `pangolin-pp-cli site create` — Update a site.
- `pangolin-pp-cli site delete` — Delete a site and all its associated data.
- `pangolin-pp-cli site get` — Get a site by siteId.

**site-resource** — Manage site resource

- `pangolin-pp-cli site-resource create` — Update a site resource.
- `pangolin-pp-cli site-resource delete` — Delete a site resource.
- `pangolin-pp-cli site-resource get` — Get a specific site resource by siteResourceId.

**target** — Manage target

- `pangolin-pp-cli target create` — Update a target.
- `pangolin-pp-cli target delete` — Delete a target.
- `pangolin-pp-cli target get` — Get a target.

**user** — Manage user

- `pangolin-pp-cli user <userId>` — Get a user by ID.

**users** — Manage users

- `pangolin-pp-cli users` — List non–server-admin users (server admin).


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
pangolin-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Expose a homelab service in one command

```bash
pangolin-pp-cli expose grafana --target 192.168.1.50:3000 --role admins --dry-run
```

Previews the full create-site-resource-target-role chain; drop --dry-run to apply.

### Find expiring certs across every org

```bash
pangolin-pp-cli cert-watch --days 30 --json --select domain,daysUntilExpiry
```

Returns a compact JSON array of certs expiring within 30 days. Pair with --select to shrink the agent's context cost.

### Audit access for a specific user

```bash
pangolin-pp-cli access-graph --user $USER_ID --json --select resource,org,role
```

Joins users x roles x resources x orgs into a single answer for 'what can this person reach?'.

### Disaster-recovery backup

```bash
pangolin-pp-cli backup --out backups/pangolin-$(date +%F).json
```

Snapshots the entire Pangolin configuration as JSON; commit to git for diffable history.

### Search resources by partial name

```bash
pangolin-pp-cli search 'grafana' --json --select id,name,fullDomain
```

FTS over the local store finds resources without paginating the live API.

## Auth Setup

Authentication is a single Bearer integration token. Set PANGOLIN_TOKEN in your environment; the base URL is typically https://<your-dashboard>/api/v1 (note: the OpenAPI spec advertises /v1 but real EE deployments mount the API under /api/v1). Set PANGOLIN_BASE_URL accordingly and run `pangolin-pp-cli doctor` to confirm.

Run `pangolin-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  pangolin-pp-cli client get mock-value --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
pangolin-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
pangolin-pp-cli feedback --stdin < notes.txt
pangolin-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.pangolin-pp-cli/feedback.jsonl`. They are never POSTed unless `PANGOLIN_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `PANGOLIN_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
pangolin-pp-cli profile save briefing --json
pangolin-pp-cli --profile briefing client get mock-value
pangolin-pp-cli profile list --json
pangolin-pp-cli profile show briefing
pangolin-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `pangolin-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/pangolin/cmd/pangolin-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add pangolin-pp-mcp -- pangolin-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which pangolin-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   pangolin-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `pangolin-pp-cli <command> --help`.
