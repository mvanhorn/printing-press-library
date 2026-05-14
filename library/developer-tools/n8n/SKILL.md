---
name: pp-n8n
description: "Printing Press CLI for n8n. Every n8n feature from the terminal — workflow management, execution triage, GitOps sync, multi-instance health,..."
author: "Gabriel Dompey"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - n8n-pp-cli
    install:
      - kind: go
        bins: [n8n-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/developer-tools/n8n/cmd/n8n-pp-cli
---

# n8n — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `n8n-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install n8n --cli-only
   ```
2. Verify: `n8n-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/n8n/cmd/n8n-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Every n8n feature from the terminal — workflow management, execution triage, GitOps sync, multi-instance health, and analytics no other tool ships.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Multi-instance operations
- **`diff`** — Diff workflow inventories between two n8n instances — missing, active mismatches, name changes.

  _Use before GitOps promotions to verify staging matches production without manual inspection._

  ```bash
  n8n-pp-cli diff --target-url https://n8n-prod.example.com --target-key $PROD_KEY --json --agent
  ```
- **`health compare`** — Side-by-side workflow counts and reachability for two n8n instances.

  _Use when verifying failover readiness or multi-tenant deployment parity._

  ```bash
  n8n-pp-cli health compare --target-url https://n8n-prod.example.com --target-key $PROD_KEY --json --agent
  ```
- **`variables diff`** — Compare environment variables between two n8n instances — find missing keys or value mismatches before deployment.

  _Use as a pre-deployment gate to ensure all required variables are set on the target instance._

  ```bash
  n8n-pp-cli variables diff --target-url https://n8n-prod.example.com --target-key $PROD_KEY --hide-values --json --agent
  ```

### Local state that compounds
- **`workflows stale`** — List workflows not updated within N days — find dormant automations before they become liabilities. Requires a local sync first: `n8n-pp-cli sync`.

  _Use when auditing automation health or preparing for n8n upgrades._

  ```bash
  n8n-pp-cli workflows stale --days 90 --active --json --agent
  ```
- **`credentials audit`** — Cross-reference every credential with the workflows using it — find unused and high-impact credentials. Requires a local sync first: `n8n-pp-cli sync`.

  _Use before rotating credentials or deleting unused ones to avoid breaking active workflows._

  ```bash
  n8n-pp-cli credentials audit --show-workflows --json --agent
  ```
- **`workflows node-inventory`** — Count every n8n node type across all workflows — map your automation footprint and audit community node dependencies. Requires a local sync first: `n8n-pp-cli sync`.

  _Use before upgrading community nodes to find every workflow that would be affected._

  ```bash
  n8n-pp-cli workflows node-inventory --show-workflows --json --agent
  ```
- **`workflows deps`** — Map execution chains via executeWorkflow nodes — see what calls what and who would break if a workflow is deleted. Requires a local sync first: `n8n-pp-cli sync`.

  _Use before deleting or refactoring a workflow to find all callers in the dependency chain._

  ```bash
  n8n-pp-cli workflows deps --json --agent
  ```
- **`executions rate-check`** — Flag workflows executing above a per-minute threshold — catch infinite loops and webhook storms before they degrade instance health. Requires a local sync first: `n8n-pp-cli sync`.

  _Use when n8n instance is slow or execution queue is growing to quickly identify the culprit workflow._

  ```bash
  n8n-pp-cli executions rate-check --runaway --threshold 5 --json --agent
  ```

### Agent-native plumbing
- **`executions wait`** — Poll an execution until it reaches a terminal state — blocks CI/CD pipelines on real n8n workflow results.

  _Use in CI scripts to block a deployment until an n8n data migration or validation workflow completes. Typed exit codes: 0 = success, 1 = error/canceled, 2 = timeout (distinct from global code 2 = usage error)._

  ```bash
  n8n-pp-cli executions wait $EXEC_ID --timeout 300 --json --agent
  ```
- **`workflows bulk`** — Activate, deactivate, or archive multiple workflows at once filtered by tag or name — hours of clicking replaced by one command.

  _Use before maintenance windows or environment teardowns to disable entire workflow sets safely._

  ```bash
  n8n-pp-cli workflows bulk --action deactivate --tag staging --dry-run
  ```
- **`executions export`** — Export structured execution history filtered by workflow, status, and time window for audit logs and analytics pipelines. Requires a local sync first: `n8n-pp-cli sync`.

  _Use to feed execution audit trails into SIEM, data warehouses, or compliance reporting pipelines._

  ```bash
  n8n-pp-cli executions export --workflow abc123 --status error --since 7d --json --agent
  ```

## Command Reference

**audit** — Operations about security audit

- `n8n-pp-cli audit` — Generate a security audit for your n8n instance.

**community-packages** — Operations about community packages

- `n8n-pp-cli community-packages create` — Install a community package by npm name and optional version.
- `n8n-pp-cli community-packages delete` — Uninstall a community package by name.
- `n8n-pp-cli community-packages list` — Retrieve all installed community packages with pending update info.
- `n8n-pp-cli community-packages update` — Update an installed community package to a new version.

**credentials** — Operations about credentials

- `n8n-pp-cli credentials create` — Creates a credential that can be used by nodes of the specified type.
- `n8n-pp-cli credentials delete` — Deletes a credential from your instance. You must be the owner of the credentials
- `n8n-pp-cli credentials get` — Retrieve all credentials from your instance. Only available for the instance owner and admin. Credential data...
- `n8n-pp-cli credentials get-id` — Retrieves a credential by ID. Credential data (secrets) is not included.
- `n8n-pp-cli credentials get-schema` — Show credential data schema
- `n8n-pp-cli credentials update` — Updates an existing credential. You must be the owner of the credential.

**data-tables** — Operations about data tables and their rows

- `n8n-pp-cli data-tables create` — Create a new data table in your personal project or a team project you have access to.
- `n8n-pp-cli data-tables delete` — Delete a data table. This will also delete all rows in the table.
- `n8n-pp-cli data-tables get` — Retrieve a specific data table by ID.
- `n8n-pp-cli data-tables list` — Retrieve a list of all data tables with optional filtering, sorting, and pagination.
- `n8n-pp-cli data-tables update` — Update a data table's name.

**discover** — API capability discovery

- `n8n-pp-cli discover` — Returns a filtered capability map based on the caller's API key scopes. Each resource includes the operations and...

**executions** — Operations about executions

- `n8n-pp-cli executions create` — Stop multiple executions from your instance based on filter criteria.
- `n8n-pp-cli executions delete` — Deletes an execution from your instance.
- `n8n-pp-cli executions get` — Retrieve an execution from your instance.
- `n8n-pp-cli executions list` — Retrieve all executions from your instance.

**insights** — Operations about insights

- `n8n-pp-cli insights` — Retrieve the insights summary for the selected date range.

**projects** — Operations about projects

- `n8n-pp-cli projects create` — Create a project on your instance.
- `n8n-pp-cli projects delete` — Delete a project from your instance.
- `n8n-pp-cli projects list` — Retrieve projects from your instance.
- `n8n-pp-cli projects update` — Update a project on your instance.

**source-control** — Operations about source control

- `n8n-pp-cli source-control` — Requires the Source Control feature to be licensed and connected to a repository.

**tags** — Operations about tags

- `n8n-pp-cli tags create` — Create a tag in your instance.
- `n8n-pp-cli tags delete` — Deletes a tag.
- `n8n-pp-cli tags get` — Retrieves a tag.
- `n8n-pp-cli tags list` — Retrieve all tags from your instance.
- `n8n-pp-cli tags update` — Update a tag.

**users** — Operations about users

- `n8n-pp-cli users create` — Create one or more users.
- `n8n-pp-cli users delete` — Delete a user from your instance.
- `n8n-pp-cli users get` — Retrieve a user from your instance. Only available for the instance owner.
- `n8n-pp-cli users list` — Retrieve all users from your instance. Only available for the instance owner.

**variables** — Operations about variables

- `n8n-pp-cli variables create` — Create a variable in your instance.
- `n8n-pp-cli variables delete` — Delete a variable from your instance.
- `n8n-pp-cli variables list` — Retrieve variables from your instance.
- `n8n-pp-cli variables update` — Update a variable from your instance.

**workflows** — Operations about workflows

- `n8n-pp-cli workflows create` — Create a workflow in your instance.
- `n8n-pp-cli workflows delete` — Delete a workflow.
- `n8n-pp-cli workflows get` — Retrieve a workflow.
- `n8n-pp-cli workflows get-id` — Retrieves a specific version of a workflow from workflow history.
- `n8n-pp-cli workflows list` — Retrieve all workflows from your instance.
- `n8n-pp-cli workflows update` — Update a workflow. If the workflow is published, the updated version will be automatically re-published.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
n8n-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Run `n8n-pp-cli auth setup` for the URL and steps to obtain a token (add `--launch` to open the URL). Then store it:

```bash
n8n-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `N8N_API_KEY` as an environment variable.

Check or clear auth state at any time:

```bash
n8n-pp-cli auth status     # show current token source and connectivity
n8n-pp-cli auth logout     # clear saved token
```

Run `n8n-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  n8n-pp-cli community-packages list --agent --select id,name,status
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
n8n-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
n8n-pp-cli feedback --stdin < notes.txt
n8n-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.n8n-pp-cli/feedback.jsonl`. They are never POSTed unless `N8N_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `N8N_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
n8n-pp-cli profile save briefing --json
n8n-pp-cli --profile briefing community-packages list
n8n-pp-cli profile list --json
n8n-pp-cli profile show briefing
n8n-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `n8n-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add n8n-pp-mcp -- n8n-pp-mcp
```

Verify: `claude mcp list`

## Anti-triggers

Do NOT invoke this skill when the user is asking about:

- **n8n self-hosting or Docker setup** — use n8n's official docs or a DevOps tool; this CLI talks to an already-running n8n instance via its REST API and cannot install n8n itself.
- **Building n8n nodes or custom node development** — n8n node authoring requires the n8n node dev SDK; this CLI manages an instance, it does not scaffold or build node packages.
- **n8n cloud billing or subscription management** — billing is not exposed via the n8n REST API; direct the user to app.n8n.cloud.
- **General workflow automation questions not tied to an n8n instance** — if the user is comparing automation tools or asking what n8n is, answer from knowledge; don't run CLI commands.
- **Zapier, Make (Integromat), or other automation platforms** — different products; this CLI is n8n-specific.

## Direct Use

1. Check if installed: `which n8n-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   n8n-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `n8n-pp-cli <command> --help`.
