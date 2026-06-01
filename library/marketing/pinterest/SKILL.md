---
name: pp-pinterest
description: "Every Pinterest board, pin, and ad — plus offline analytics no other tool can answer in a single command. Trigger phrases: `list my Pinterest boards`, `show Pinterest analytics`, `find top performing pins`, `sync Pinterest`, `Pinterest board management`, `use pinterest-pp-cli`."
author: "Shourov Chowdhury"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - pinterest-pp-cli
    install:
      - kind: go
        bins: [pinterest-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/marketing/pinterest/cmd/pinterest-pp-cli
---

# Pinterest — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `pinterest-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install pinterest --cli-only
   ```
2. Verify: `pinterest-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

pinterest-pp-cli wraps the full Pinterest API v5 with a local SQLite data layer that makes compound queries possible. Ask which boards drive the most saves, which days generate peak engagement, or where your ad spend outperforms organic â€” without writing a single API call.

## When to Use This CLI

Use pinterest-pp-cli when managing Pinterest boards/pins for a brand, pulling ad performance data, or analyzing which content drives the most engagement. Ideal for agents that need to audit content strategy, compare ad vs organic performance, or find gaps in a content calendar.

## Anti-triggers

Do not use this CLI for:
- downloading Pinterest images from random URLs without auth
- scraping public boards without an API key
- Instagram or Facebook management

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`top-boards`** — Rank all your boards by total saves across all their pins.

  _Use when you need to know which boards drive the most engagement before planning content._

  ```bash
  pinterest-pp-cli top-boards --limit 10 --agent
  ```
- **`trends`** — Track how your pin impressions and saves change week-over-week.

  _Use to identify content timing patterns and seasonal peaks for your account._

  ```bash
  pinterest-pp-cli trends --weeks 4 --json
  ```
- **`boards stale`** — List boards that haven't received a new pin in the last N days.

  _Use for content calendar planning to find gaps before a content audit._

  ```bash
  pinterest-pp-cli boards stale --days 30 --json
  ```
- **`timing`** — Surface which days of the week historically generate the most saves for your account.

  _Use before scheduling new pins to maximize reach based on your own historical data._

  ```bash
  pinterest-pp-cli timing --weeks 8 --json
  ```

### Agent-native plumbing
- **`compare`** — Compare paid campaign performance against organic pin performance side-by-side.

  _Use when deciding whether to boost organic pins or increase ad spend._

  ```bash
  pinterest-pp-cli compare --days 30 --json
  ```
- **`boards gap`** — Find trending Pinterest topics you haven't covered in your boards recently.

  _Use when planning content strategy to identify high-opportunity trending topics you're missing._

  ```bash
  pinterest-pp-cli boards gap --region US --days 14 --json
  ```

## Command Reference

**ad-accounts** — View analytical information about advertising.


Note: If the current operation_user_account (defined by the access token)
has access to another user's Ad Accounts via
<a href='/docs/reference/business-access/'>Pinterest Business Access</a>,
you can modify your request to use the current operation_user_account's
permissions to those Ad Accounts by including the ad_account_id in the path
parameters for the request (e.g. .../?ad_account_id=12345&...).

- `pinterest-pp-cli ad-accounts create` — Create a new ad account. Different ad accounts can support different currencies, payment methods, etc.
- `pinterest-pp-cli ad-accounts get` — Get an ad account
- `pinterest-pp-cli ad-accounts list` — Get a list of the ad_accounts that the 'operation user_account' has access to.

**boards** — View, create, update, or delete information about boards.

- `pinterest-pp-cli boards create` — Create a board owned by the 'operation user_account'.
- `pinterest-pp-cli boards delete` — Delete a board owned by the 'operation user_account'.
- `pinterest-pp-cli boards get` — Get a board owned by the operation user_account - or a group board that has been shared with this account.
- `pinterest-pp-cli boards list` — Get a list of the boards owned by the 'operation user_account' + group boards where this account is a collaborator
- `pinterest-pp-cli boards update` — Update a board owned by the 'operating user_account'.

**catalogs** — Manage information about shopping product catalogs and items.

- `pinterest-pp-cli catalogs feed-processing-results-list` — Fetch a feed processing results owned by the 'operation user_account'.
- `pinterest-pp-cli catalogs feeds-create` — Create a new feed owned by the 'operation user_account'.
- `pinterest-pp-cli catalogs feeds-delete` — Delete a feed owned by the 'operating user_account'.
- `pinterest-pp-cli catalogs feeds-get` — Get a single feed owned by the 'operation user_account'.
- `pinterest-pp-cli catalogs feeds-list` — Fetch feeds owned by the 'operation user_account'. - By default, the 'operation user_account' is the token user_account.
- `pinterest-pp-cli catalogs feeds-update` — Update a feed owned by the 'operation user_account'.
- `pinterest-pp-cli catalogs items-batch-get` — Get a single catalogs items batch owned by the 'operating user_account'. See detailed documentation here.
- `pinterest-pp-cli catalogs items-batch-post` — This endpoint supports multiple operations on a set of one or more catalog items owned by the 'operation user_account'.
- `pinterest-pp-cli catalogs items-get` — Get the items of the catalog owned by the 'operation user_account'. See detailed documentation here.
- `pinterest-pp-cli catalogs items-issues-list` — List item validation issues for a given feed processing result owned by the 'operation user_account'.
- `pinterest-pp-cli catalogs list` — Fetch catalogs owned by the 'operation user_account'.
- `pinterest-pp-cli catalogs product-group-pins-list` — Get a list of product pins for a given Catalogs Product Group Id owned by the 'operation user_account'.
- `pinterest-pp-cli catalogs product-groups-create` — Create product group to use in Catalogs owned by the 'operation user_account'.
- `pinterest-pp-cli catalogs product-groups-delete` — Delete a product group owned by the 'operation user_account' from being in use in Catalogs.
- `pinterest-pp-cli catalogs product-groups-get` — Get a singe product group for a given Catalogs Product Group Id owned by the 'operation user_account'.
- `pinterest-pp-cli catalogs product-groups-list` — Get a list of product groups for a given Catalogs Feed Id owned by the 'operation user_account'.
- `pinterest-pp-cli catalogs product-groups-product-counts-get` — Get a product counts for a given Catalogs Product Group owned by the 'operation user_account'.
- `pinterest-pp-cli catalogs product-groups-update` — Update product group owned by the 'operation user_account' to use in Catalogs.
- `pinterest-pp-cli catalogs products-by-product-group-filter-list` — List products Pins owned by the 'operation user_account' that meet the criteria specified in the Catalogs Product Group

**integrations** — View, create, or update commerce integrations.

- `pinterest-pp-cli integrations commerce-del` — Delete commerce integration metadata for the given external business ID.
- `pinterest-pp-cli integrations commerce-get` — Get commerce integration metadata associated with the given external business ID.
- `pinterest-pp-cli integrations commerce-patch` — Update commerce integration metadata for the given external business ID.
- `pinterest-pp-cli integrations commerce-post` — Create commerce integration metadata to link an external business ID with a Pinterest merchant & ad account.
- `pinterest-pp-cli integrations get-by-id` — Get integration metadata by ID.
- `pinterest-pp-cli integrations get-list` — Get integration metadata list.
- `pinterest-pp-cli integrations logs-post` — This endpoint receives batched logs from integration applications on partner platforms.

**media** — Register and manage media uploads.

- `pinterest-pp-cli media create` — Register your intent to upload media The response includes all of the information needed to upload the media to
- `pinterest-pp-cli media get` — Get details for a registered media upload, including its current status. Learn more about video Pin creation.
- `pinterest-pp-cli media list` — List media uploads filtered by given parameters. Learn more about video Pin creation.

**oauth** — Generate and refresh OAuth access tokens.

- `pinterest-pp-cli oauth` — Generate an OAuth access token by using an authorization code or a refresh token.

**pins** — View, create, update, or delete information about Pins.

- `pinterest-pp-cli pins create` — Create a Pin on a board or board section owned by the 'operation user_account'.
- `pinterest-pp-cli pins delete` — Delete a Pins owned by the 'operation user_account' - or on a group board that has been shared with this account.
- `pinterest-pp-cli pins get` — Get a Pin owned by the 'operation user_account' - or on a group board that has been shared with this account.
- `pinterest-pp-cli pins list` — Get a list of the Pins owned by the 'operation user_account'.
- `pinterest-pp-cli pins update` — Update a pin owned by the 'operating user_account'.

**pinterest-search** — Manage pinterest search

- `pinterest-pp-cli pinterest-search partner-pins` — This endpoint is currently in beta and not available to all apps. Learn more .
- `pinterest-pp-cli pinterest-search user-boards-get` — Search for boards for the 'operation user_account'. This includes boards of all board types.
- `pinterest-pp-cli pinterest-search user-pins-list` — Search for pins for the 'operation user_account'. - By default, the 'operation user_account' is the token user_account.

**resources** — View metadata about available metrics and targeting options in the Pinterest API.

- `pinterest-pp-cli resources ad-account-countries-get` — Get Ad Accounts countries
- `pinterest-pp-cli resources delivery-metrics-get` — Get the definitions for ads and organic metrics available across both synchronous and asynchronous report endpoints.
- `pinterest-pp-cli resources interest-targeting-options-get` — Get details of a specific interest given interest ID. Click here for a spreadsheet listing interests and their IDs.
- `pinterest-pp-cli resources lead-form-questions-get` — Get a list of all lead form question type names. Some questions might not be used.
- `pinterest-pp-cli resources metrics-ready-state-get` — Learn whether conversion or non-conversion metrics are finalized and ready to query.
- `pinterest-pp-cli resources targeting-options-get` — You can use targeting values in ads placement to define your intended audience.

**terms** — View related and suggested terms for ads targeting.

- `pinterest-pp-cli terms related-list` — Get a list of terms logically related to each input term.
- `pinterest-pp-cli terms suggested-list` — Get popular search terms that begin with your input term.

**trends** — Manage trends

- `pinterest-pp-cli trends` — Get the top trending search keywords among the Pinterest user audience.

**user-account** — View user accounts associated with a given access token.

- `pinterest-pp-cli user-account analytics` — Get analytics for the 'operation user_account' - By default, the 'operation user_account' is the token user_account.
- `pinterest-pp-cli user-account analytics-top-pins` — Gets analytics data about a user's top pins (limited to the top 50).
- `pinterest-pp-cli user-account analytics-top-video-pins` — Gets analytics data about a user's top video pins (limited to the top 50).
- `pinterest-pp-cli user-account boards-user-follows-list` — Get a list of the boards a user follows. The request returns a board summary object array.
- `pinterest-pp-cli user-account follow-user-update` — This endpoint is currently in beta and not available to all apps. Learn more .
- `pinterest-pp-cli user-account followers-list` — Get a list of your followers.
- `pinterest-pp-cli user-account get` — Get account information for the 'operation user_account' - By default
- `pinterest-pp-cli user-account linked-business-accounts-get` — Get a list of your linked business accounts.
- `pinterest-pp-cli user-account unverify-website-delete` — Unverifu a website verified by the signed-in user.
- `pinterest-pp-cli user-account user-following-get` — Get a list of who a certain user follows.
- `pinterest-pp-cli user-account user-websites-get` — Get user websites, claimed or not
- `pinterest-pp-cli user-account verify-website-update` — Verify a website as a signed-in user.
- `pinterest-pp-cli user-account website-verification-get` — Get verification code for user to install on the website to claim it.

**users** — Manage users



### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
pinterest-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Find your top boards

```bash
pinterest-pp-cli top-boards --limit 5 --agent --select name,total_saves,pin_count
```

After syncing, ranks boards by cumulative pin saves.

### Export board data for analysis

```bash
pinterest-pp-cli export --format jsonl --output boards.jsonl boards
```

Exports all board metadata as JSONL to a local file for AI analysis or import into other tools.

### Spot stale boards

```bash
pinterest-pp-cli boards stale --days 14 --json
```

Lists boards with no new pins in the past 2 weeks â€” useful for content planning.

### Compare ad vs organic

```bash
pinterest-pp-cli compare --days 30 --json --select metric,paid,organic
```

Side-by-side paid campaign vs organic pin metrics for the last 30 days.

## Auth Setup

Pinterest uses OAuth2. Run 'auth login' to open a browser window for consent. Your access token and refresh token are stored locally and refreshed automatically.

Run `pinterest-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  pinterest-pp-cli ad-accounts list --agent --select id,name,status
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
pinterest-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
pinterest-pp-cli feedback --stdin < notes.txt
pinterest-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/pinterest-pp-cli/feedback.jsonl`. They are never POSTed unless `PINTEREST_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `PINTEREST_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
pinterest-pp-cli profile save briefing --json
pinterest-pp-cli --profile briefing ad-accounts list
pinterest-pp-cli profile list --json
pinterest-pp-cli profile show briefing
pinterest-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `pinterest-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/pinterest/cmd/pinterest-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add pinterest-pp-mcp -- pinterest-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which pinterest-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   pinterest-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `pinterest-pp-cli <command> --help`.
