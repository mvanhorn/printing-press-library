---
name: pp-apify
description: "Every Apify platform feature, plus a local SQLite store, cross-Actor search, novelty diffing, cost-aware runs, and... Trigger phrases: `scrape twitter for AI mentions`, `run apify actor`, `what's new in my reddit scraper`, `build a newsletter from this week's scrapes`, `how much did my apify scrapes cost this month`, `use apify`, `run apify`."
author: "Kevin Magnan"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - apify-pp-cli
---

# Apify — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `apify-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install apify --cli-only
   ```
2. Verify: `apify-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/apify/cmd/apify-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Reach for apify-pp-cli when you need to orchestrate multiple Apify Actors as part of a recurring research, scraping, or newsletter pipeline. It's the right tool when you want per-run cost visibility, novel-only output, cross-Actor search, or schedule-as-code. For Actor development (writing your own scrapers), use the official `apify-cli`; this CLI is for platform operators.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`run`** — Run an Actor and emit only items not seen in prior runs of that Actor.

  _Reach for this when you need 'what's new since last run' across recurring scrapes for a newsletter or alert workflow._

  ```bash
  apify-pp run trudax/reddit-scraper --input @sub-list.json --only-new --format markdown
  ```
- **`search items`** — Full-text search across every cached dataset from every Actor, normalized into a common schema.

  _Use this when you want 'what did anyone say about X this week' without juggling 8 dataset IDs._

  ```bash
  apify-pp search "model context protocol" --since 7d --actors twitter,reddit,hn --json --select url,title,source_actor
  ```
- **`digest --offline`** — Run queries and template renders against the local SQLite copy of past datasets without hitting the API.

  _Use this while tuning templates or dedupe heuristics so iteration is free._

  ```bash
  apify-pp digest --offline --topic AI --since 7d --template draft.tmpl
  ```

### Cost discipline
- **`cost report`** — Per-run USD cost report joining cached run metadata with the Apify monthly usage endpoint.

  _Reach for this whenever the user asks 'why is my Apify bill so high' or 'which Actor costs most per useful item.'_

  ```bash
  apify-pp cost report --since 30d --group-by actor,schedule --json
  ```
- **`run --max-cost`** — Pre-flight cost projection from local p50/p90 of past runs for the same Actor; aborts run if projection exceeds budget.

  _Use this on any long-tail or untrusted Actor to fail-closed on cost rather than learning about it on the invoice._

  ```bash
  apify-pp run apidojo/twitter-scraper-lite --input @q.json --max-cost 0.50 --max-cu 100
  ```
- **`ab run`** — Run the same input through two competing Actors, normalize via unified schema, report cost-per-novel-item and overlap percentage.

  _Reach for this whenever the user wonders 'is the cheaper Actor good enough' before committing to a recurring schedule._

  ```bash
  apify-pp ab run apidojo/tweet-scraper kaitoeasyapi/twitter-x-data-tweet-scraper-pay-per-result-cheapest --input shared.json --judge novelty --json
  ```

### Newsletter workflows
- **`digest`** — Render a markdown digest from local store: dedupe by URL + title-similarity, rank by engagement+recency+novelty, fill a Go template.

  _Use this as the last step of any newsletter / weekly report workflow; output is ready to paste into Beehiiv or pipe to a writer agent._

  ```bash
  apify-pp digest --topic "AI dev tools" --since 24h --template weekly.tmpl
  ```
- **`workflow run`** — Run a YAML-declared workflow that chains run → normalize → novelty → digest → publish across multiple Actors.

  _Use this when the user wants a recurring 'fire one command, get the newsletter' pipeline rather than orchestrating Actor calls by hand._

  ```bash
  apify-pp workflow run ./weekly-newsletter.yaml --json
  ```

### GitOps
- **`schedules apply`** — Declarative cron + Actor input bundle with terraform-style plan/apply/diff against the live Apify schedule API.

  _Reach for this when the user manages 3+ schedules and wants them version-controlled._

  ```bash
  apify-pp schedule apply ./schedules.yaml --dry-run && apify-pp schedule diff
  ```

### Repeatability
- **`preset save`** — Capture known-good Actor input JSON from a prior run and replay with overrides.

  _Use this to stop hand-rebuilding input JSON every time you re-run a recurring scrape._

  ```bash
  apify-pp preset save twitter weekly-ai --from-run abc123 && apify-pp call twitter --preset weekly-ai --override startUrls=@new-list.txt
  ```

## Command Reference

**actor-builds** — The API endpoints described in this section enable you to manage, and delete Apify Actor builds.

Note that if any returned build object contains usage in dollars, your effective
unit pricing at the time of query has been used for computation of this dollar equivalent, and hence it should be
used only for informative purposes.

You can learn more about platform usage in the [documentation](https://docs.apify.com/platform/actors/running/usage-and-resources#usage).

- `apify-pp-cli actor-builds delete` — Delete the build. The build that is the current default build for the Actor cannot be deleted. Only users with build...
- `apify-pp-cli actor-builds get` — Gets a list of all builds for a user. The response is a JSON array of objects, where each object contains basic...
- `apify-pp-cli actor-builds get-actorbuilds` — Gets an object that contains all the details about a specific build of an Actor. By passing the optional...

**actor-runs** — The API endpoints described in this section enable you to manage, and delete Apify Actor runs.

If any returned run object contains usage in dollars, your effective unit pricing at the time of query
has been used for computation of this dollar equivalent, and hence it should be used only for informative purposes.

For completed runs, aggregated fields such as `stats` or dollar usage totals are eventually consistent and update within a few seconds. For values that must match finalized totals, wait about 10 seconds after the run completed, then fetch the run again.

You can learn more about platform usage in the [documentation](https://docs.apify.com/platform/actors/running/usage-and-resources#usage).

- `apify-pp-cli actor-runs delete` — Delete the run. Only finished runs can be deleted. Only the person or organization that initiated the run can delete it.
- `apify-pp-cli actor-runs get` — Gets a list of all runs for a user. The response is a list of objects, where each object contains basic information...
- `apify-pp-cli actor-runs get-actorruns` — This is not a single endpoint, but an entire group of endpoints that lets you retrieve the run or any of its default...
- `apify-pp-cli actor-runs put` — This endpoint can be used to update both the run's status message and to configure its general resource access...

**actor-tasks** — The API endpoints described in this section enable you to create, manage, delete, and run Apify Actor tasks.
For more information, see the [Actor tasts documentation](https://docs.apify.com/platform/actors/running/tasks).

:::note

For all the API endpoints that accept the `actorTaskId` parameter to
specify a task, you can pass either the task ID (e.g. `HG7ML7M8z78YcAPEB`) or a tilde-separated
username of the task's owner and the task's name (e.g. `janedoe~my-task`).

:::

Some of the API endpoints return run objects. If any such run object
contains usage in dollars, your effective unit pricing at the time of query
has been used for computation of this dollar equivalent, and hence it should be
used only for informative purposes.

You can learn more about platform usage in the [documentation](https://docs.apify.com/platform/actors/running/usage-and-resources#usage).

- `apify-pp-cli actor-tasks delete` — Delete the task specified through the `actorTaskId` parameter.
- `apify-pp-cli actor-tasks get` — Gets the complete list of tasks that a user has created or used. The response is a list of objects in which each...
- `apify-pp-cli actor-tasks get-actortasks` — Get an object that contains all the details about a task.
- `apify-pp-cli actor-tasks post` — Create a new task with settings specified by the object passed as JSON in the POST payload. The response is the full...
- `apify-pp-cli actor-tasks put` — Update settings of a task using values specified by an object passed as JSON in the POST payload. If the object does...

**acts** — Manage acts

- `apify-pp-cli acts delete` — Deletes an Actor.
- `apify-pp-cli acts get` — Gets the list of all Actors that the user created or used. The response is a list of objects, where each object...
- `apify-pp-cli acts get-actorid` — Gets an object that contains all the details about a specific Actor.
- `apify-pp-cli acts post` — Creates a new Actor with settings specified in an Actor object passed as JSON in the POST payload. The response is...
- `apify-pp-cli acts put` — Updates settings of an Actor using values specified by an Actor object passed as JSON in the POST payload. If the...

**browser-info** — Manage browser info

- `apify-pp-cli browser-info tools-delete` — Returns information about the HTTP request, including the client IP address, country code, request headers, and body...
- `apify-pp-cli browser-info tools-get` — Returns information about the HTTP request, including the client IP address, country code, request headers, and body...
- `apify-pp-cli browser-info tools-post` — Returns information about the HTTP request, including the client IP address, country code, request headers, and body...
- `apify-pp-cli browser-info tools-put` — Returns information about the HTTP request, including the client IP address, country code, request headers, and body...

**datasets** — Manage datasets

- `apify-pp-cli datasets delete` — Deletes a specific dataset.
- `apify-pp-cli datasets get` — Lists all of a user's datasets. The response is a JSON array of objects, where each object contains basic...
- `apify-pp-cli datasets get-datasetid` — Returns dataset object for given dataset ID. This does not return dataset items, only information about the storage...
- `apify-pp-cli datasets post` — Creates a dataset and returns its object. Keep in mind that data stored under unnamed dataset follows [data...
- `apify-pp-cli datasets put` — Updates a dataset's name and general resource access level using a value specified by a JSON object passed in the...

**key-value-stores** — Manage key value stores

- `apify-pp-cli key-value-stores delete` — Deletes a key-value store.
- `apify-pp-cli key-value-stores get` — Gets the list of key-value stores owned by the user. The response is a list of objects, where each objects contains...
- `apify-pp-cli key-value-stores get-keyvaluestores` — Gets an object that contains all the details about a specific key-value store.
- `apify-pp-cli key-value-stores post` — Creates a key-value store and returns its object. The response is the same object as returned by the [Get...
- `apify-pp-cli key-value-stores put` — Updates a key-value store's name and general resource access level using a value specified by a JSON object passed...

**logs** — The API endpoints described in this section are used the download the logs
generated by Actor builds and runs. Note that only the trailing 5M characters
of the log are stored, the rest is discarded.

:::note

Note that the endpoints do not require the authentication token, the calls
are authenticated using a hard-to-guess ID of the Actor build or run.

:::

- `apify-pp-cli logs <buildOrRunId>` — Retrieves logs for a specific Actor build or run.

**request-queues** — Manage request queues

- `apify-pp-cli request-queues delete` — Deletes given queue.
- `apify-pp-cli request-queues get` — Lists all of a user's request queues. The response is a JSON array of objects, where each object contains basic...
- `apify-pp-cli request-queues get-requestqueues` — Returns queue object for given queue ID.
- `apify-pp-cli request-queues post` — Creates a request queue and returns its object. Keep in mind that requests stored under unnamed queue follows [data...
- `apify-pp-cli request-queues put` — Updates a request queue's name and general resource access level using a value specified by a JSON object passed in...

**schedules** — This section describes API endpoints for managing schedules.

Schedules are used to automatically start your Actors at certain times. Each schedule
can be associated with a number of Actors and Actor tasks. It is also possible
to override the settings of each Actor (task) similarly to when invoking the Actor
(task) using the API.
For more information, see [Schedules documentation](https://docs.apify.com/platform/schedules).

Each schedule is assigned actions for it to perform. Actions can be of two types
- `RUN_ACTOR` and `RUN_ACTOR_TASK`.

For details, see the documentation of the [Get schedule](#/reference/schedules/schedule-object/get-schedule) endpoint.

- `apify-pp-cli schedules delete` — Deletes a schedule.
- `apify-pp-cli schedules get` — Gets the list of schedules that the user created. The endpoint supports pagination using the `limit` and `offset`...
- `apify-pp-cli schedules get-scheduleid` — Gets the schedule object with all details.
- `apify-pp-cli schedules post` — Creates a new schedule with settings provided by the schedule object passed as JSON in the payload. The response is...
- `apify-pp-cli schedules put` — Updates a schedule using values specified by a schedule object passed as JSON in the POST payload. If the object...

**store** — [Apify Store](https://apify.com/store) is home to thousands of public Actors available
to the Apify community.
The API endpoints described in this section are used to retrieve these Actors.

:::note

These endpoints do not require the authentication token.

:::

- `apify-pp-cli store` — Gets the list of public Actors in Apify Store. You can use `search` parameter to search Actors by string in title,...

**tools** — The API endpoints described in this section provide utility tools for encoding,
signing, and verifying data, as well as inspecting HTTP request details.

- `apify-pp-cli tools decode-and-verify-post` — Decodes and verifies an encoded value previously created by the encode-and-sign endpoint. Returns the original...
- `apify-pp-cli tools encode-and-sign-post` — Encodes and signs any JSON object. The encoded value includes a signature tied to the authenticated user's ID, which...

**users** — The API endpoints described in this section return information about user accounts.

- `apify-pp-cli users get` — Returns public information about a specific user account, similar to what can be seen on public profile pages (e.g....
- `apify-pp-cli users me-get` — Returns information about the current user account, including both public and private information. The user account...
- `apify-pp-cli users me-limits-get` — Returns a complete summary of your account's limits. It is the same information you will see on your account's...
- `apify-pp-cli users me-limits-put` — Updates the account's limits manageable on your account's [Limits page](https://console.apify.com/billing#/limits)....
- `apify-pp-cli users me-usage-monthly-get` — Returns a complete summary of your usage for the current monthly usage cycle, an overall sum, as well as a daily...

**webhook-dispatches** — Manage webhook dispatches

- `apify-pp-cli webhook-dispatches get` — Gets the list of webhook dispatches that the user have. The endpoint supports pagination using the `limit` and...
- `apify-pp-cli webhook-dispatches webhook-dispatch-get` — Gets webhook dispatch object with all details.

**webhooks** — Webhook-related API endpoints for configuring automated notifications.

- `apify-pp-cli webhooks delete` — Deletes a webhook.
- `apify-pp-cli webhooks get` — Gets the list of webhooks that the user created. The endpoint supports pagination using the `limit` and `offset`...
- `apify-pp-cli webhooks get-webhookid` — Gets webhook object with all details.
- `apify-pp-cli webhooks post` — Creates a new webhook with settings provided by the webhook object passed as JSON in the payload. The response is...
- `apify-pp-cli webhooks put` — Updates a webhook using values specified by a webhook object passed as JSON in the POST payload. If the object does...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
apify-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Weekly newsletter scrape

```bash
apify-pp workflow run ./weekly-newsletter.yaml --agent
```

One YAML-declared chain across Twitter + Reddit + HN + News scrapers; emits a single newsletter-ready digest.

### Cost-capped exploratory run

```bash
apify-pp run apidojo/tweet-scraper --input @q.json --max-cost 0.25 --wait --agent
```

Pre-flight projection from local history; aborts before start if projected cost exceeds budget.

### Cross-Actor topic search (compact)

```bash
apify-pp search 'agent' --since 30d --agent --select url,title,source_actor,published_at
```

FTS5 over the local store; use `--select` with dotted paths to avoid pulling the full normalized item payload.

### Schedule drift check

```bash
apify-pp schedule diff --agent
```

Terraform-style diff between local YAML and the live Apify schedules; output is a JSON change set.

### A/B test two Twitter scrapers

```bash
apify-pp ab run apidojo/tweet-scraper kaitoeasyapi/twitter-x-data-tweet-scraper --input shared.json --judge cost-per-novel --agent
```

Runs both, normalizes via unified schema, reports cost-per-novel-item and overlap %.

## Auth Setup

Set `APIFY_TOKEN` (from Apify Console → Settings → Integrations) or run `apify-pp auth set-token`. The CLI validates the token against `/v2/users/me` before any call. The query-param `?token=` form is supported but discouraged.

Run `apify-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  apify-pp-cli actor-builds get --agent --select id,name,status
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
apify-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
apify-pp-cli feedback --stdin < notes.txt
apify-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.apify-pp-cli/feedback.jsonl`. They are never POSTed unless `APIFY_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `APIFY_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
apify-pp-cli profile save briefing --json
apify-pp-cli --profile briefing actor-builds get
apify-pp-cli profile list --json
apify-pp-cli profile show briefing
apify-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `apify-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add apify-pp-mcp -- apify-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which apify-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   apify-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `apify-pp-cli <command> --help`.
