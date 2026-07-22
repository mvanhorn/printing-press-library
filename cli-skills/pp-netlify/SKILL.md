---
name: pp-netlify
description: "Every Netlify API endpoint, plus a local SQLite mirror of your whole account so you can search, diff, and audit across all sites at once — offline and agent-native. Trigger phrases: `list my netlify sites`, `check netlify deploys`, `search netlify form submissions`, `audit netlify dns`, `compare netlify env vars`, `use netlify`, `run netlify`."
author: "Charles Denzel Segovia"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - netlify-pp-cli
    install:
      - kind: go
        bins: [netlify-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/cloud/netlify/cmd/netlify-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/cloud/netlify/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Netlify — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `netlify-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install netlify --cli-only
   ```
2. Verify: `netlify-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/netlify/cmd/netlify-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

The official netlify-cli is stateless and per-site. netlify-pp-cli syncs your sites, deploys, DNS, env vars, and form submissions into a local database, then adds cross-site commands no other tool has: env-drift compares variables across sites, submissions search runs offline full-text search over every form, dns-audit flags dangling records, and overview shows the health of every site in one view.

## When to Use This CLI

Use this CLI when you manage more than one Netlify site and need account-wide answers: comparing env vars across sites, searching form submissions, auditing DNS, or reviewing recent deploys. It is also the right tool for agents that need structured JSON output and typed exit codes over the Netlify API.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to run local dev servers or `netlify dev` — use the official netlify-cli for local development.
- Do not use it to build or bundle site assets; it manages the account via the API, it does not compile sites.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Cross-site state that compounds
- **`overview`** — One command shows build status, last deploy time, and form counts for every site in your account.

  _Reach for this to survey an entire account at once instead of paging through the dashboard per site._

  ```bash
  netlify-pp-cli overview --json --select name,last_deploy,state
  ```
- **`env-drift`** — Finds env vars that exist on some sites but are missing or differ on others, across contexts.

  _Use before a release to catch a secret set on staging but missing on production._

  ```bash
  netlify-pp-cli env-drift --json
  ```
- **`dns-audit`** — Aggregates DNS records across all zones and flags records pointing at deleted sites and soon-expiring SNI certs.

  _Use to find dangling DNS after tearing down sites, in one pass over the whole account._

  ```bash
  netlify-pp-cli dns-audit --json
  ```

### Offline search
- **`submissions search`** — Full-text search across every form submission from every site, offline.

  _Reach for this to find a specific contact/lead across all forms without clicking through the UI._

  ```bash
  netlify-pp-cli submissions search "refund" --json --limit 20
  ```
- **`since`** — Shows deploys and builds across all sites within a time window (e.g. last 2h).

  _Reach for this to answer 'what shipped recently' across an account after an incident._

  ```bash
  netlify-pp-cli since 2h --json
  ```
- **`deploy-diff`** — Compares two deploys of a site (state, commit, context, error) side by side from the local mirror.

  _Use to see exactly what changed between a working deploy and a broken one when hunting a regression._

  ```bash
  netlify-pp-cli deploy-diff <deploy-a> <deploy-b> --json
  ```

## Command Reference

**accounts** — Manage accounts

- `netlify-pp-cli accounts cancel` — Cancel
- `netlify-pp-cli accounts create` — Create
- `netlify-pp-cli accounts get` — Get
- `netlify-pp-cli accounts list-for-user` — List for user
- `netlify-pp-cli accounts list-types-for-user` — List types for user
- `netlify-pp-cli accounts update` — Update

**agent-runners** — Manage agent runners

- `netlify-pp-cli agent-runners create` — Create
- `netlify-pp-cli agent-runners create-upload-url` — Create upload url
- `netlify-pp-cli agent-runners delete` — Delete
- `netlify-pp-cli agent-runners get` — Get
- `netlify-pp-cli agent-runners list` — List
- `netlify-pp-cli agent-runners update` — Update

**ai-gateway** — Manage ai gateway

- `netlify-pp-cli ai-gateway` — Get providers

**billing** — Manage billing

- `netlify-pp-cli billing` — List payment methods for user

**builds** — Manage builds

- `netlify-pp-cli builds <build_id>` — Get site

**deploy-keys** — Manage deploy keys

- `netlify-pp-cli deploy-keys create` — Create
- `netlify-pp-cli deploy-keys delete` — Delete
- `netlify-pp-cli deploy-keys get` — Get
- `netlify-pp-cli deploy-keys list` — List

**deploys** — Manage deploys

- `netlify-pp-cli deploys delete` — Delete
- `netlify-pp-cli deploys get` — Get

**dns-zones** — Manage dns zones

- `netlify-pp-cli dns-zones create` — Create
- `netlify-pp-cli dns-zones delete` — Delete
- `netlify-pp-cli dns-zones get` — Get
- `netlify-pp-cli dns-zones get-dnszones` — Get dnszones

**forms** — Manage forms


**hooks** — Manage hooks

- `netlify-pp-cli hooks create-by-site-id` — Create by site id
- `netlify-pp-cli hooks delete` — Delete
- `netlify-pp-cli hooks get` — Get
- `netlify-pp-cli hooks list-by-site-id` — List by site id
- `netlify-pp-cli hooks list-types` — List types
- `netlify-pp-cli hooks update` — Update

**oauth** — Manage oauth

- `netlify-pp-cli oauth create-ticket` — Create ticket
- `netlify-pp-cli oauth exchange-ticket` — Exchange ticket
- `netlify-pp-cli oauth show-ticket` — Show ticket

**purge** — Manage purge

- `netlify-pp-cli purge` — Purges cached content from Netlify's CDN. Supports purging by Cache-Tag.

**services** — Manage services

- `netlify-pp-cli services get` — Get
- `netlify-pp-cli services show` — Show

**sites** — Manage sites

- `netlify-pp-cli sites create` — **Note:** Environment variable keys and values have moved from `build_settings.env` and `repo.env` to a new endpoint.
- `netlify-pp-cli sites delete` — Delete
- `netlify-pp-cli sites get` — **Note:** Environment variable keys and values have moved from `build_settings.env` and `repo.env` to a new endpoint.
- `netlify-pp-cli sites list` — **Note:** Environment variable keys and values have moved from `build_settings.env` and `repo.env` to a new endpoint.
- `netlify-pp-cli sites update` — **Note:** Environment variable keys and values have moved from `build_settings.env` and `repo.env` to a new endpoint.

**submissions** — Manage submissions

- `netlify-pp-cli submissions delete` — Delete
- `netlify-pp-cli submissions list-form` — List form

**user** — Manage user

- `netlify-pp-cli user` — Get current


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
netlify-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Audit env vars before a release

```bash
netlify-pp-cli env-drift --json --select key,sites_missing
```

Lists variables set on some sites but missing on others so nothing ships half-configured.

### Find a lead across all forms

```bash
netlify-pp-cli submissions search "acme corp" --json --select site_name,form_name,email,created_at
```

Offline full-text search over every submission with only the fields an agent needs.

### Post-incident deploy review

```bash
netlify-pp-cli since 4h --json --select site_name,state,commit_ref
```

Shows every deploy across the account in the last four hours.

## Auth Setup

Run `netlify-pp-cli auth setup` for the URL and steps to obtain a token (add `--launch` to open the URL). Then store it:

```bash
netlify-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `NETLIFY_AUTH_TOKEN` as an environment variable.

Run `netlify-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  netlify-pp-cli accounts get mock-value --agent --select id,name,status
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

- Use `--home <dir>` for one invocation, or set `NETLIFY_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `NETLIFY_CONFIG_DIR`, `NETLIFY_DATA_DIR`, `NETLIFY_STATE_DIR`, `NETLIFY_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `NETLIFY_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `netlify-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "netlify": {
        "command": "netlify-pp-mcp",
        "env": {
          "NETLIFY_HOME": "/srv/netlify"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `NETLIFY_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `NETLIFY_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
netlify-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
netlify-pp-cli feedback --stdin < notes.txt
netlify-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `NETLIFY_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `NETLIFY_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
netlify-pp-cli profile save briefing --json
netlify-pp-cli --profile briefing accounts get mock-value
netlify-pp-cli profile list --json
netlify-pp-cli profile show briefing
netlify-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `netlify-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/cloud/netlify/cmd/netlify-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add netlify-pp-mcp -- netlify-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which netlify-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   netlify-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `netlify-pp-cli <command> --help`.
