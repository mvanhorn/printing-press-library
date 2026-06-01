---
name: pp-google-cloud-run
description: "Every Cloud Run service, job, and revision -- plus a deployment gate, exposure audit Trigger phrases: `list Cloud Run services`, `deploy to Cloud Run`, `check my Cloud Run jobs`, `which services are publicly accessible`, `wait for my revision to get traffic`, `use google-cloud-run`, `run google-cloud-run-pp-cli`."
author: "never-mind-3"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - google-cloud-run-pp-cli
    install:
      - kind: go
        bins: [google-cloud-run-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/cloud/google-cloud-run/cmd/google-cloud-run-pp-cli
---

# Google Cloud Run — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `google-cloud-run-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install google-cloud-run --cli-only
   ```
2. Verify: `google-cloud-run-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/google-cloud-run/cmd/google-cloud-run-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Replaces multi-tab Console sessions and fragile gcloud scripts with a single agent-native CLI. Syncs your entire Cloud Run fleet to a local SQLite store for offline queries, cross-project inventory, and zero-lag CI/CD primitives. Ships the wait-for-traffic gate, public exposure audit, and revision diff workflows that power users have been scripting by hand for years.

## When to Use This CLI

Use this CLI when managing, deploying, or auditing Google Cloud Run services or jobs. Best for post-deploy verification, CI/CD gates, security posture checks, and cross-project workflows.

## Anti-triggers

Do not use this CLI for:
- Viewing Cloud Run logs (use gcloud logging or Cloud Logging API directly)
- Managing Cloud Run on Anthos or GKE-based environments
- Deploying from source code (use gcloud run deploy --source)

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Fleet-wide visibility
- **`services list-all`** — List Cloud Run services across multiple GCP projects in a single unified table.

  _Use when an agent needs a fleet-wide view of services without multiple separate list calls per project._

  ```bash
  google-cloud-run-pp-cli services list-all --projects my-proj-1,my-proj-2 --agent
  ```

### Lifecycle management
- **`revisions prune`** — Delete old revisions that are not serving any traffic, keeping only the N most recent.

  _Use to free up Cloud Run revision quota without manually identifying and deleting inactive revisions._

  ```bash
  google-cloud-run-pp-cli revisions prune --service my-service --keep-last 3 --dry-run --agent
  ```

### Deploy visibility
- **`services traffic`** — Show the current traffic split across revisions with image tag and deploy timestamp in one table.

  _Use when an agent needs to verify which image version is serving what percentage of traffic after a canary deploy._

  ```bash
  google-cloud-run-pp-cli services traffic --service my-service --agent
  ```
- **`revisions diff`** — Show a field-by-field diff between two revisions: image, CPU/memory, scaling config, and env var key names.

  _Use during incident triage to see what configuration changed between the last good and first bad revision._

  ```bash
  google-cloud-run-pp-cli revisions diff --service my-service --from my-service-00041-abc --to my-service-00042-def --agent
  ```

### Job observability
- **`executions summary`** — Show a per-execution summary of a Cloud Run Job with succeeded/failed task counts and duration.

  _Use as the git log for Cloud Run Jobs to quickly assess whether recent job executions succeeded._

  ```bash
  google-cloud-run-pp-cli executions summary --job nightly-etl --last 7 --agent
  ```

### Security posture
- **`iam audit`** — List all services in a project that are publicly accessible via allUsers or allAuthenticatedUsers IAM bindings.

  _Use for security posture checks to surface any Cloud Run service unintentionally exposed to the public internet._

  ```bash
  google-cloud-run-pp-cli iam audit --project my-project --show-public --agent
  ```
- **`iam diff`** — Compare the current IAM policy of a service against a saved snapshot to detect added or removed bindings.

  _Use for compliance audits to confirm no IAM bindings changed since the last approved baseline._

  ```bash
  google-cloud-run-pp-cli iam diff --service my-service --snapshot ./iam-baseline.json --agent
  ```

### CI/CD primitives
- **`revisions wait-traffic`** — Block until a specific revision reaches a target traffic percentage -- a CI/CD gate primitive.

  _Use in CI/CD pipelines after deploying -- prevents running smoke tests before the revision is actually serving traffic._

  ```bash
  google-cloud-run-pp-cli revisions wait-traffic --service my-service --revision my-service-00042-abc --target-pct 100 --timeout 300s
  ```

## Command Reference

**cloud-run-admin-jobs** — Manage cloud run admin jobs

- `google-cloud-run-pp-cli cloud-run-admin-jobs create` — Creates a Job.
- `google-cloud-run-pp-cli cloud-run-admin-jobs list` — Lists Jobs.
- `google-cloud-run-pp-cli cloud-run-admin-jobs run` — Triggers creation of a new Execution of this Job.

**executions** — Manage executions
- `google-cloud-run-pp-cli executions summary` — Per-execution summary (succeeded/failed task counts, duration) for a Cloud Run Job.


**operations** — Manage operations

- `google-cloud-run-pp-cli operations list` — Lists operations that match the specified filter in the request.
- `google-cloud-run-pp-cli operations wait` — Waits until the specified long-running operation is done or reaches at most a specified timeout

**services** — Manage services

- `google-cloud-run-pp-cli services create` — Creates a new Service in a given project and location.
- `google-cloud-run-pp-cli services get-iam-policy` — Gets the IAM Access Control policy currently in effect for the given Cloud Run Service.
- `google-cloud-run-pp-cli services list` — Lists Services.
- `google-cloud-run-pp-cli services patch` — Updates a Service.
- `google-cloud-run-pp-cli services set-iam-policy` — Sets the IAM Access control policy for the specified Service. Overwrites any existing policy.
- `google-cloud-run-pp-cli services test-iam-permissions` — Returns permissions that a caller has on the specified Project.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
google-cloud-run-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### CI/CD post-deploy gate

```bash
google-cloud-run-pp-cli revisions wait-traffic --service my-service --revision my-service-00042-abc --target-pct 100 --timeout 300s
```

Block the pipeline until the new revision is fully serving traffic before running smoke tests.

### Weekly revision cleanup

```bash
google-cloud-run-pp-cli revisions prune --service my-service --keep-last 5 --dry-run
```

Preview which old revisions will be deleted, then re-run without --dry-run to free up revision quota.

### Security posture check

```bash
google-cloud-run-pp-cli iam audit --project my-project --show-public --agent --select services.name,services.uri,services.offending_binding
```

Compact agent-readable list of publicly exposed services.

### Incident diff

```bash
google-cloud-run-pp-cli revisions diff --service my-service --from my-service-00041-abc --to my-service-00042-def --agent
```

Immediately see what config changed between the last good and first bad revision.

### Multi-project fleet view

```bash
google-cloud-run-pp-cli services list-all --projects prod-us,prod-eu,staging --agent --select services.name,services.region,services.uri,services.serving_revision
```

Cross-project service inventory with fields an agent needs for deploy decisions.

## Auth Setup

Cloud Run uses OAuth2 with the cloud-platform scope. Set GOOGLE_CLOUD_RUN_TOKEN to a bearer token from gcloud auth print-access-token. Falls back to Application Default Credentials (ADC) -- run gcloud auth application-default login for automatic refresh.

Run `google-cloud-run-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  google-cloud-run-pp-cli cloud-run-admin-jobs list mock-value --agent --select id,name,status
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
google-cloud-run-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
google-cloud-run-pp-cli feedback --stdin < notes.txt
google-cloud-run-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/google-cloud-run-pp-cli/feedback.jsonl`. They are never POSTed unless `GOOGLE_CLOUD_RUN_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `GOOGLE_CLOUD_RUN_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
google-cloud-run-pp-cli profile save briefing --json
google-cloud-run-pp-cli --profile briefing cloud-run-admin-jobs list mock-value
google-cloud-run-pp-cli profile list --json
google-cloud-run-pp-cli profile show briefing
google-cloud-run-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `google-cloud-run-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/cloud/google-cloud-run/cmd/google-cloud-run-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add google-cloud-run-pp-mcp -- google-cloud-run-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which google-cloud-run-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   google-cloud-run-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `google-cloud-run-pp-cli <command> --help`.
