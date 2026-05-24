# Apify CLI

**Every Apify platform feature, plus a local SQLite store, cross-Actor search, novelty diffing, cost-aware runs, and newsletter-ready digests no other Apify tool offers.**

apify-pp-cli is the operator CLI for the Apify platform — agent-native JSON across every endpoint, plus a layer of features built on top of a local store that the official CLI, SDKs, and MCP can't reach: cross-Actor full-text search (`search`), novel-only runs (`run --only-new`), per-run cost ledger (`cost report`), and templated newsletter digests (`digest`).

Printed by [@kjmagnan1s](https://github.com/kjmagnan1s) (Kevin Magnan).

## Install

The recommended path installs both the `apify-pp-cli` binary and the `pp-apify` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install apify
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install apify --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install apify --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install apify --agent claude-code
npx -y @mvanhorn/printing-press-library install apify --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/apify/cmd/apify-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/apify-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-apify --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-apify --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-apify skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-apify. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/apify-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `APIFY_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "apify": {
      "command": "apify-pp-mcp",
      "env": {
        "APIFY_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Set `APIFY_TOKEN` (from Apify Console → Settings → Integrations) or run `apify-pp auth set-token`. The CLI validates the token against `/v2/users/me` before any call. The query-param `?token=` form is supported but discouraged.

## Quick Start

```bash
# Verify token, API reachability, and quota status.
apify-pp doctor

# Find an Actor in the public store.
apify-pp store search 'twitter scraper' --json --select name,username,stats.totalUsers7Days

# Run an Actor and block until terminal state.
apify-pp run apidojo/twitter-scraper-lite --input @input.json --wait --json

# Pull items from the run's default dataset.
apify-pp datasets items <dataset-id> --format json --limit 100

# Hydrate local store, then full-text search across every cached dataset.
apify-pp sync --since 7d && apify-pp search 'AI' --since 7d --agent

# Render a newsletter-ready markdown digest from the local store.
apify-pp digest --topic AI --since 24h --template default --agent

```

## Unique Features

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

## Usage

Run `apify-pp-cli --help` for the full command reference and flag list.

## Commands

### actor-builds

The API endpoints described in this section enable you to manage, and delete Apify Actor builds.

Note that if any returned build object contains usage in dollars, your effective
unit pricing at the time of query has been used for computation of this dollar equivalent, and hence it should be
used only for informative purposes.

You can learn more about platform usage in the [documentation](https://docs.apify.com/platform/actors/running/usage-and-resources#usage).

- **`apify-pp-cli actor-builds delete`** - Delete the build. The build that is the current default build for the Actor
cannot be deleted.

Only users with build permissions for the Actor can delete builds.
- **`apify-pp-cli actor-builds get`** - Gets a list of all builds for a user. The response is a JSON array of
objects, where each object contains basic information about a single build.

The endpoint supports pagination using the `limit` and `offset` parameters
and it will not return more than 1000 records.

By default, the records are sorted by the `startedAt` field in ascending
order. Therefore, you can use pagination to incrementally fetch all builds while
new ones are still being started. To sort the records in descending order, use
the `desc=1` parameter.
- **`apify-pp-cli actor-builds get-actorbuilds`** - Gets an object that contains all the details about a specific build of an
Actor.

By passing the optional `waitForFinish` parameter the API endpoint will
synchronously wait for the build to finish. This is useful to avoid periodic
polling when waiting for an Actor build to finish.

This endpoint does not require the authentication token. Instead, calls are authenticated using a hard-to-guess ID of the build. However,
if you access the endpoint without the token, certain attributes, such as `usageUsd` and `usageTotalUsd`, will be hidden.

### actor-runs

The API endpoints described in this section enable you to manage, and delete Apify Actor runs.

If any returned run object contains usage in dollars, your effective unit pricing at the time of query
has been used for computation of this dollar equivalent, and hence it should be used only for informative purposes.

For completed runs, aggregated fields such as `stats` or dollar usage totals are eventually consistent and update within a few seconds. For values that must match finalized totals, wait about 10 seconds after the run completed, then fetch the run again.

You can learn more about platform usage in the [documentation](https://docs.apify.com/platform/actors/running/usage-and-resources#usage).

- **`apify-pp-cli actor-runs delete`** - Delete the run. Only finished runs can be deleted. Only the person or
organization that initiated the run can delete it.
- **`apify-pp-cli actor-runs get`** - Gets a list of all runs for a user. The response is a list of objects, where
each object contains basic information about a single Actor run.

The endpoint supports pagination using the `limit` and `offset` parameters
and it will not return more than 1000 array elements.

By default, the records are sorted by the `startedAt` field in ascending
order. Therefore, you can use pagination to incrementally fetch all records while
new ones are still being created. To sort the records in descending order, use
`desc=1` parameter. You can also filter runs by `startedAt`` and `status`` fields ([available
statuses](https://docs.apify.com/platform/actors/running/runs-and-builds#lifecycle)).
- **`apify-pp-cli actor-runs get-actorruns`** - This is not a single endpoint, but an entire group of endpoints that lets
you retrieve the run or any of its default storages.

##### Convenience endpoints for Actor run default storages

* [Dataset](/api/v2/default-dataset)

* [Key-value store](/api/v2/default-key-value-store)

* [Request queue](/api/v2/default-request-queue)

Gets an object that contains all the details about a
specific run of an Actor.

By passing the optional `waitForFinish` parameter the API endpoint will synchronously wait
for the run to finish. This is useful to avoid periodic polling when waiting for Actor run to complete.
Note that the first response after completion can still show preliminary `stats`, costs, and event counts.
For stable figures, wait about 10 seconds and call the endpoint again.

This endpoint does not require the authentication token. Instead, calls are authenticated using a hard-to-guess ID of the run. However,
if you access the endpoint without the token, certain attributes, such as `usageUsd` and `usageTotalUsd`, will be hidden.
- **`apify-pp-cli actor-runs put`** - This endpoint can be used to update both the run's status message and to configure its general resource access level.

**Status message:**

You can set a single status message on your run that will be displayed in
the Apify Console UI. During an Actor run, you will typically do this in order
to inform users of your Actor about the Actor's progress.

The request body must contain `runId` and `statusMessage` properties. The
`isStatusMessageTerminal` property is optional and it indicates if the
status message is the very last one. In the absence of a status message, the
platform will try to substitute sensible defaults.

**General resource access:**

You can also update the run's general resource access setting, which determines who can view the run and its related data.

Allowed values:

* `FOLLOW_USER_SETTING` - The run inherits the general access setting from the account level.
* `ANYONE_WITH_ID_CAN_READ` - The run can be viewed anonymously by anyone who has its ID.
* `RESTRICTED` - Only users with explicit access to the resource can access the run.

When a run is accessible anonymously, all of the run's default storages and logs also become accessible anonymously.

### actor-tasks

The API endpoints described in this section enable you to create, manage, delete, and run Apify Actor tasks.
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

- **`apify-pp-cli actor-tasks delete`** - Delete the task specified through the `actorTaskId` parameter.
- **`apify-pp-cli actor-tasks get`** - Gets the complete list of tasks that a user has created or used.

The response is a list of objects in which each object contains essential
information about a single task.

The endpoint supports pagination using the `limit` and `offset` parameters,
and it does not return more than a 1000 records.

By default, the records are sorted by the `createdAt` field in ascending
order; therefore you can use pagination to incrementally fetch all tasks while new
ones are still being created. To sort the records in descending order, use
the `desc=1` parameter.
- **`apify-pp-cli actor-tasks get-actortasks`** - Get an object that contains all the details about a task.
- **`apify-pp-cli actor-tasks post`** - Create a new task with settings specified by the object passed as JSON in
the POST payload.

The response is the full task object as returned by the
[Get task](#/reference/tasks/task-object/get-task) endpoint.

The request needs to specify the `Content-Type: application/json` HTTP header!

When providing your API authentication token, we recommend using the
request's `Authorization` header, rather than the URL. ([More
info](#/introduction/authentication)).
- **`apify-pp-cli actor-tasks put`** - Update settings of a task using values specified by an object passed as JSON
in the POST payload.

If the object does not define a specific property, its value is not updated.

The response is the full task object as returned by the
[Get task](#/reference/tasks/task-object/get-task) endpoint.

The request needs to specify the `Content-Type: application/json` HTTP
header!

When providing your API authentication token, we recommend using the
request's `Authorization` header, rather than the URL. ([More
info](#/introduction/authentication)).

### acts

Manage acts

- **`apify-pp-cli acts delete`** - Deletes an Actor.
- **`apify-pp-cli acts get`** - Gets the list of all Actors that the user created or used. The response is a
list of objects, where each object contains a basic information about a single Actor.

To only get Actors created by the user, add the `my=1` query parameter.

The endpoint supports pagination using the `limit` and `offset` parameters
and it will not return more than 1000 records.

By default, the records are sorted by the `createdAt` field in ascending
order, therefore you can use pagination to incrementally fetch all Actors while new
ones are still being created. To sort the records in descending order, use the `desc=1` parameter.

You can also sort by your last run by using the `sortBy=stats.lastRunStartedAt` query parameter.
In this case, descending order means the most recently run Actor appears first.
- **`apify-pp-cli acts get-actorid`** - Gets an object that contains all the details about a specific Actor.
- **`apify-pp-cli acts post`** - Creates a new Actor with settings specified in an Actor object passed as
JSON in the POST payload.
The response is the full Actor object as returned by the
[Get Actor](#/reference/actors/actor-object/get-actor) endpoint.

The HTTP request must have the `Content-Type: application/json` HTTP header!

The Actor needs to define at least one version of the source code.
For more information, see [Version object](#/reference/actors/version-object).

If you want to make your Actor
[public](https://docs.apify.com/platform/actors/publishing) using `isPublic:
true`, you will need to provide the Actor's `title` and the `categories`
under which that Actor will be classified in Apify Store. For this, it's
best to use the [constants from our `apify-shared-js`
package](https://github.com/apify/apify-shared-js/blob/2d43ebc41ece9ad31cd6525bd523fb86939bf860/packages/consts/src/consts.ts#L452-L471).
- **`apify-pp-cli acts put`** - Updates settings of an Actor using values specified by an Actor object
passed as JSON in the POST payload.
If the object does not define a specific property, its value will not be
updated.

The response is the full Actor object as returned by the
[Get Actor](#/reference/actors/actor-object/get-actor) endpoint.

The request needs to specify the `Content-Type: application/json` HTTP header!

When providing your API authentication token, we recommend using the
request's `Authorization` header, rather than the URL. ([More
info](#/introduction/authentication)).

If you want to make your Actor
[public](https://docs.apify.com/platform/actors/publishing) using `isPublic:
true`, you will need to provide the Actor's `title` and the `categories`
under which that Actor will be classified in Apify Store. For this, it's
best to use the [constants from our `apify-shared-js`
package](https://github.com/apify/apify-shared-js/blob/2d43ebc41ece9ad31cd6525bd523fb86939bf860/packages/consts/src/consts.ts#L452-L471).

### browser-info

Manage browser info

- **`apify-pp-cli browser-info tools-delete`** - Returns information about the HTTP request, including the client IP address,
country code, request headers, and body length.

This endpoint is designed for proxy testing. It accepts any HTTP method so you
can verify that your proxy correctly forwards requests of any type and that
client IP addresses are anonymized.
- **`apify-pp-cli browser-info tools-get`** - Returns information about the HTTP request, including the client IP address,
country code, request headers, and body length.

This endpoint is designed for proxy testing. It accepts any HTTP method so you
can verify that your proxy correctly forwards requests of any type and that
client IP addresses are anonymized.
- **`apify-pp-cli browser-info tools-post`** - Returns information about the HTTP request, including the client IP address,
country code, request headers, and body length.

This endpoint is designed for proxy testing. It accepts any HTTP method so you
can verify that your proxy correctly forwards requests of any type and that
client IP addresses are anonymized.
- **`apify-pp-cli browser-info tools-put`** - Returns information about the HTTP request, including the client IP address,
country code, request headers, and body length.

This endpoint is designed for proxy testing. It accepts any HTTP method so you
can verify that your proxy correctly forwards requests of any type and that
client IP addresses are anonymized.

### datasets

Manage datasets

- **`apify-pp-cli datasets delete`** - Deletes a specific dataset.
- **`apify-pp-cli datasets get`** - Lists all of a user's datasets.

The response is a JSON array of objects,
where each object contains basic information about one dataset.

By default, the objects are sorted by the `createdAt` field in ascending
order, therefore you can use pagination to incrementally fetch all datasets while new
ones are still being created. To sort them in descending order, use `desc=1`
parameter. The endpoint supports pagination using `limit` and `offset`
parameters and it will not return more than 1000 array elements.
- **`apify-pp-cli datasets get-datasetid`** - Returns dataset object for given dataset ID.

This does not return dataset items, only information about the storage itself.
To retrieve dataset items, use the [List dataset items](/api/v2/dataset-items-get) endpoint.

:::note

Keep in mind that attributes `itemCount` and `cleanItemCount` are not propagated right away after data are pushed into a dataset.

:::

There is a short period (up to 5 seconds) during which these counters may not match with exact counts in dataset items.
- **`apify-pp-cli datasets post`** - Creates a dataset and returns its object.
Keep in mind that data stored under unnamed dataset follows [data retention period](https://docs.apify.com/platform/storage#data-retention).
It creates a dataset with the given name if the parameter name is used.
If a dataset with the given name already exists then returns its object.
- **`apify-pp-cli datasets put`** - Updates a dataset's name and general resource access level using a value specified by a JSON object passed in the PUT payload.
The response is the updated dataset object, as returned by the [Get dataset](/api/v2/dataset-get) API endpoint.

### key-value-stores

Manage key value stores

- **`apify-pp-cli key-value-stores delete`** - Deletes a key-value store.
- **`apify-pp-cli key-value-stores get`** - Gets the list of key-value stores owned by the user.

The response is a list of objects, where each objects contains a basic
information about a single key-value store.

The endpoint supports pagination using the `limit` and `offset` parameters
and it will not return more than 1000 array elements.

By default, the records are sorted by the `createdAt` field in ascending
order, therefore you can use pagination to incrementally fetch all key-value stores
while new ones are still being created. To sort the records in descending order, use
the `desc=1` parameter.
- **`apify-pp-cli key-value-stores get-keyvaluestores`** - Gets an object that contains all the details about a specific key-value
store.
- **`apify-pp-cli key-value-stores post`** - Creates a key-value store and returns its object. The response is the same
object as returned by the [Get store](#/reference/key-value-stores/store-object/get-store)
endpoint.

Keep in mind that data stored under unnamed store follows [data retention
period](https://docs.apify.com/platform/storage#data-retention).

It creates a store with the given name if the parameter name is used.
If there is another store with the same name, the endpoint does not create a
new one and returns the existing object instead.
- **`apify-pp-cli key-value-stores put`** - Updates a key-value store's name and general resource access level using a value specified by a JSON object
passed in the PUT payload.

The response is the updated key-value store object, as returned by the [Get
store](#/reference/key-value-stores/store-object/get-store) API endpoint.

### logs

The API endpoints described in this section are used the download the logs
generated by Actor builds and runs. Note that only the trailing 5M characters
of the log are stored, the rest is discarded.

:::note

Note that the endpoints do not require the authentication token, the calls
are authenticated using a hard-to-guess ID of the Actor build or run.

:::

- **`apify-pp-cli logs <buildOrRunId>`** - Retrieves logs for a specific Actor build or run.

### request-queues

Manage request queues

- **`apify-pp-cli request-queues delete`** - Deletes given queue.
- **`apify-pp-cli request-queues get`** - Lists all of a user's request queues. The response is a JSON array of
objects, where each object
contains basic information about one queue.

By default, the objects are sorted by the `createdAt` field in ascending order,
therefore you can use pagination to incrementally fetch all queues while new
ones are still being created. To sort them in descending order, use `desc=1`
parameter. The endpoint supports pagination using `limit` and `offset`
parameters and it will not return more than 1000
array elements.
- **`apify-pp-cli request-queues get-requestqueues`** - Returns queue object for given queue ID.
- **`apify-pp-cli request-queues post`** - Creates a request queue and returns its object.
Keep in mind that requests stored under unnamed queue follows [data
retention period](https://docs.apify.com/platform/storage#data-retention).

It creates a queue of given name if the parameter name is used. If a queue
with the given name already exists then the endpoint returns
its object.
- **`apify-pp-cli request-queues put`** - Updates a request queue's name and general resource access level using a value specified by a JSON object
passed in the PUT payload.

The response is the updated request queue object, as returned by the
[Get request queue](#/reference/request-queues/queue-collection/get-request-queue) API endpoint.

### schedules

This section describes API endpoints for managing schedules.

Schedules are used to automatically start your Actors at certain times. Each schedule
can be associated with a number of Actors and Actor tasks. It is also possible
to override the settings of each Actor (task) similarly to when invoking the Actor
(task) using the API.
For more information, see [Schedules documentation](https://docs.apify.com/platform/schedules).

Each schedule is assigned actions for it to perform. Actions can be of two types
- `RUN_ACTOR` and `RUN_ACTOR_TASK`.

For details, see the documentation of the [Get schedule](#/reference/schedules/schedule-object/get-schedule) endpoint.

- **`apify-pp-cli schedules delete`** - Deletes a schedule.
- **`apify-pp-cli schedules get`** - Gets the list of schedules that the user created.

The endpoint supports pagination using the `limit` and `offset` parameters.
It will not return more than 1000 records.

By default, the records are sorted by the `createdAt` field in ascending
order. To sort the records in descending order, use the `desc=1` parameter.
- **`apify-pp-cli schedules get-scheduleid`** - Gets the schedule object with all details.
- **`apify-pp-cli schedules post`** - Creates a new schedule with settings provided by the schedule object passed
as JSON in the payload. The response is the created schedule object.

The request needs to specify the `Content-Type: application/json` HTTP header!

When providing your API authentication token, we recommend using the
request's `Authorization` header, rather than the URL. ([More
info](#/introduction/authentication)).
- **`apify-pp-cli schedules put`** - Updates a schedule using values specified by a schedule object passed as
JSON in the POST payload. If the object does not define a specific property,
its value will not be updated.

The response is the full schedule object as returned by the
[Get schedule](#/reference/schedules/schedule-object/get-schedule) endpoint.

**The request needs to specify the `Content-Type: application/json` HTTP
header!**

When providing your API authentication token, we recommend using the
request's `Authorization` header, rather than the URL. ([More
info](#/introduction/authentication)).

### store

[Apify Store](https://apify.com/store) is home to thousands of public Actors available
to the Apify community.
The API endpoints described in this section are used to retrieve these Actors.

:::note

These endpoints do not require the authentication token.

:::

- **`apify-pp-cli store`** - Gets the list of public Actors in Apify Store. You can use `search`
parameter to search Actors by string in title, name, description, username
and readme.
If you need detailed info about a specific Actor, use the [Get
Actor](#/reference/actors/actor-object/get-actor) endpoint.

The endpoint supports pagination using the `limit` and `offset` parameters.
It will not return more than 1,000 records.

### tools

The API endpoints described in this section provide utility tools for encoding,
signing, and verifying data, as well as inspecting HTTP request details.

- **`apify-pp-cli tools decode-and-verify-post`** - Decodes and verifies an encoded value previously created by the
encode-and-sign endpoint. Returns the original decoded object along with
information about the user who encoded it and whether that user is verified.

**Important**: The request must specify the `Content-Type: application/json`
HTTP header.
- **`apify-pp-cli tools encode-and-sign-post`** - Encodes and signs any JSON object. The encoded value includes a signature
tied to the authenticated user's ID, which can later be verified using the
decode-and-verify endpoint.

**Important**: The request must specify the `Content-Type: application/json`
HTTP header.

### users

The API endpoints described in this section return information about user accounts.

- **`apify-pp-cli users get`** - Returns public information about a specific user account, similar to what
can be seen on public profile pages (e.g. https://apify.com/apify).

This operation requires no authentication token.
- **`apify-pp-cli users me-get`** - Returns information about the current user account, including both public
and private information.

The user account is identified by the provided authentication token.

The fields `plan`, `email` and `profile` are omitted when this endpoint is accessed from Actor run.
- **`apify-pp-cli users me-limits-get`** - Returns a complete summary of your account's limits. It is the same
information you will see on your account's [Limits page](https://console.apify.com/billing#/limits). The returned data
includes the current usage cycle, a summary of your limits, and your current usage.
- **`apify-pp-cli users me-limits-put`** - Updates the account's limits manageable on your account's [Limits page](https://console.apify.com/billing#/limits).
Specifically the: `maxMonthlyUsageUsd` and `dataRetentionDays` limits (see request body schema for more details).
- **`apify-pp-cli users me-usage-monthly-get`** - Returns a complete summary of your usage for the current monthly usage cycle,
an overall sum, as well as a daily breakdown of usage. It is the same
information you will see on your account's [Billing > Historical usage page](https://console.apify.com/billing/historical-usage). The information
includes your use of Actors, compute, data transfer, and storage.

Using the `date` parameter will show your usage in the monthly usage cycle that
includes that date.

### webhook-dispatches

Manage webhook dispatches

- **`apify-pp-cli webhook-dispatches get`** - Gets the list of webhook dispatches that the user have.

The endpoint supports pagination using the `limit` and `offset` parameters
and it will not return more than 1000 records.
By default, the records are sorted by the `createdAt` field in ascending
order. To sort the records in descending order, use the `desc=1`
parameter.
- **`apify-pp-cli webhook-dispatches webhook-dispatch-get`** - Gets webhook dispatch object with all details.

### webhooks

Webhook-related API endpoints for configuring automated notifications.

- **`apify-pp-cli webhooks delete`** - Deletes a webhook.
- **`apify-pp-cli webhooks get`** - Gets the list of webhooks that the user created.

The endpoint supports pagination using the `limit` and `offset` parameters
and it will not return more than 1000 records.
By default, the records are sorted by the `createdAt` field in ascending
order. To sort the records in descending order, use the `desc=1`
parameter.
- **`apify-pp-cli webhooks get-webhookid`** - Gets webhook object with all details.
- **`apify-pp-cli webhooks post`** - Creates a new webhook with settings provided by the webhook object passed as
JSON in the payload.
The response is the created webhook object.

To avoid duplicating a webhook, use the `idempotencyKey` parameter in the
request body.
Multiple calls to create a webhook with the same `idempotencyKey` will only
create the webhook with the first call and return the existing webhook on
subsequent calls.
Idempotency keys must be unique, so use a UUID or another random string with
enough entropy.

To assign the new webhook to an Actor or task, the request body must contain
`requestUrl`, `eventTypes`, and `condition` properties.

* `requestUrl` is the webhook's target URL, to which data is sent as a POST
request with a JSON payload.
* `eventTypes` is a list of events that will trigger the webhook, e.g. when
the Actor run succeeds.
* `condition` should be an object containing the ID of the Actor or task to
which the webhook will be assigned.
* `payloadTemplate` is a JSON-like string, whose syntax is extended with the
use of variables.
* `headersTemplate` is a JSON-like string, whose syntax is extended with the
use of variables. Following values will be re-written to defaults: "host",
"Content-Type", "X-Apify-Webhook", "X-Apify-Webhook-Dispatch-Id",
"X-Apify-Request-Origin"
* `description` is an optional string.
* `shouldInterpolateStrings` is a boolean indicating whether to interpolate
variables contained inside strings in the `payloadTemplate`

```
    "isAdHoc" : false,
    "requestUrl" : "https://example.com",
    "eventTypes" : [
        "ACTOR.RUN.SUCCEEDED",
        "ACTOR.RUN.ABORTED"
    ],
    "condition" : {
        "actorId": "5sTMwDQywwsLzKRRh",
        "actorTaskId" : "W9bs9JE9v7wprjAnJ"
    },
    "payloadTemplate": "",
    "headersTemplate": "",
    "description": "my awesome webhook",
    "shouldInterpolateStrings": false,
```

**Important**: The request must specify the `Content-Type: application/json`
HTTP header.
- **`apify-pp-cli webhooks put`** - Updates a webhook using values specified by a webhook object passed as JSON
in the POST payload.
If the object does not define a specific property, its value will not be
updated.

The response is the full webhook object as returned by the
[Get webhook](#/reference/webhooks/webhook-object/get-webhook) endpoint.

The request needs to specify the `Content-Type: application/json` HTTP
header!

When providing your API authentication token, we recommend using the
request's `Authorization` header, rather than the URL. ([More
info](#/introduction/authentication)).

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
apify-pp-cli actor-builds get

# JSON for scripting and agents
apify-pp-cli actor-builds get --json

# Filter to specific fields
apify-pp-cli actor-builds get --json --select id,name,status

# Dry run — show the request without sending
apify-pp-cli actor-builds get --dry-run

# Agent mode — JSON + compact + no prompts in one flag
apify-pp-cli actor-builds get --agent
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

## Health Check

```bash
apify-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/apify-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `APIFY_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `apify-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $APIFY_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 token-invalid** — Re-run `apify-pp auth set-token` with a fresh token from Apify Console → Settings → Integrations.
- **Run stuck in READY/RUNNING** — Check `apify-pp runs log <id> --follow`; abort with `apify-pp runs abort <id>` if the Actor is hung.
- **Cost report shows zero rows** — Run `apify-pp sync --runs --since 30d` to backfill local run metadata first.
- **`--only-new` returns every item** — No prior runs for that Actor are in the local store yet; the second run onward will dedupe.
- **Schedule diff shows unexpected drift** — Someone edited the schedule in the dashboard; reconcile with `apify-pp schedule pull > schedules.yaml`.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**apify-cli**](https://github.com/apify/apify-cli) — JavaScript
- [**apify-mcp-server**](https://github.com/apify/apify-mcp-server) — TypeScript
- [**apify-client-js**](https://github.com/apify/apify-client-js) — JavaScript
- [**apify-client-python**](https://github.com/apify/apify-client-python) — Python
- [**apify-client-rs**](https://github.com/metalwarrior665/apify-client-rs) — Rust

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
