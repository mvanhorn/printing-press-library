---
name: pp-pinterest
description: "Every Pinterest v5 endpoint plus period-over-period analytics, Pin hygiene audits, and offline Pin search. Trigger phrases: `how did my Pinterest do this month`, `audit my Pins for missing alt text`, `what's trending on Pinterest that we haven't pinned`, `pull a Pinterest ads report`, `use pinterest-pp-cli`, `run pinterest`."
author: "Elizabeth Stone"
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

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install pinterest --cli-only
   ```
2. Verify: `pinterest-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.10 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/pinterest/cmd/pinterest-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Covers boards, Pins, analytics, trends, ads, catalogs, and audiences from one binary with --dry-run on writes, no interactive prompts, and explicit --yes confirmation. A bounded local sync powers offline search, the 'pins audit' hygiene report, and 'trends gaps'; 'perf compare' and 'pins funnel' turn raw analytics into answers.

## When to Use This CLI

Use this CLI to inspect or operate a Pinterest business account: boards, Pins, organic analytics, trends, and ads. It is the right tool for weekly performance reviews, Pin SEO cleanup, safe pre-publish validation, and ads reporting.

## Anti-triggers

Do not use this CLI for:
- Scraping other people's public Pinterest profiles or boards without OAuth
- Designing Pin images or generating Pin copy (use a design or writing tool)
- Bulk mutation of live Pins without explicit human approval

## Unique Capabilities

### Analytics that answer the question

- **`perf compare`** — See whether impressions, saves, and outbound clicks grew versus the previous equal-length period, with absolute and percent deltas.

  _Reach for this for a weekly or monthly 'did we grow?' answer instead of calling account analytics twice and doing date math._

  ```bash
  pinterest-pp-cli perf compare --days 28 --agent
  ```
- **`pins funnel`** — Rank your top Pins by impressions (or --sort-by metric), compute save and outbound-click rates, and flag Pins that get saved but don't send traffic.

  _Use to decide which Pins to refresh or repurpose to drive site traffic._

  ```bash
  pinterest-pp-cli pins funnel --days 30 --limit 25 --agent
  ```
- **`trends gaps`** — List trending keywords for a region with how many of your synced Pins cover each, zero-coverage first. Coverage counts need a synced local store.

  _Use when planning the next content batch to find trends you have nothing on._

  ```bash
  pinterest-pp-cli trends gaps --region US --agent
  ```

### Local state that compounds

- **`pins audit`** — Find published Pins missing alt text, a destination link, a title, or UTM tracking, grouped by board. Reads the local store; run sync first.

  _Use before an SEO cleanup pass to get the exact Pin IDs that need fixing._

  ```bash
  pinterest-pp-cli pins audit --group-by board --agent
  ```
- **`pins preflight`** — Validate a proposed Pin (field limits, board and section by name, UTM link) and get a ready-to-send create body without writing anything. Board names resolve from the local store; run sync first.

  _Use before pins create so an agent never publishes an incomplete or misrouted Pin._

  ```bash
  pinterest-pp-cli pins preflight --board "Recipes" --link https://example.com/blog/meal-prep?utm_source=pinterest --image-url https://example.com/img/meal-prep.jpg --title "Weeknight dinner ideas" --alt-text "Bowl of pasta on a table" --agent
  ```

### Agent-native plumbing

- **`ads report`** — Create, wait for, and download an ads performance report in one command. Requires the ads:read scope and --ad-account-id (or PINTEREST_AD_ACCOUNT_ID).

  _Use for campaign or ad performance instead of manually polling the async report endpoint._

  ```bash
  pinterest-pp-cli ads report --level campaign --days 7 --agent
  ```

`--ca-cert`, `PINTEREST_CA_CERT`, or `ca_cert` adds a PEM CA bundle to the system trust roots. `--tls-server-name`, `PINTEREST_TLS_SERVER_NAME`, or `tls_server_name` is the name checked against the certificate. Precedence is flag, then environment, then the config file. A config file other users can change does not supply these settings. `doctor` reports the trust in use and names the setting that fixes a chain or hostname failure.

## Command Reference

**ad-accounts** — View analytical information about advertising.

Note: If the current operation_user_account (defined by the access token)
has access to another user's Ad Accounts via
<a href='/docs/getting-started/using-business-access/'>Pinterest Business Access</a>,
you can modify your request to use the current operation_user_account's
permissions to those Ad Accounts by including the ad_account_id in the path
parameters for the request (e.g. .../?ad_account_id=12345&...).

- `pinterest-pp-cli ad-accounts create` — Create a new ad account. Different ad accounts can support different currencies, payment methods, etc.
- `pinterest-pp-cli ad-accounts get` — Get an ad account
- `pinterest-pp-cli ad-accounts list` — Get a list of the ad_accounts that the 'operation user_account' has access to.

**advanced-auction** — View, create, or update advanced auction item bid options.

- `pinterest-pp-cli advanced-auction items-get-post` — Get the bid options for a batch of retail catalog items. The catalog must be owned by the 'operation user_account'. _(gated: not available to all users)_
- `pinterest-pp-cli advanced-auction items-submit-post` — This endpoint supports multiple operations on a set of one or more bid options (bid price and bid adjustments for _(gated: not available to all users)_

**boards** — View, create, update, or delete information about boards.

- `pinterest-pp-cli boards create` — Create a board owned by the 'operation user_account'.
- `pinterest-pp-cli boards delete` — Delete a board owned by the 'operation user_account'.
- `pinterest-pp-cli boards get` — Get a board owned by the operation user_account - or a group board that has been shared with this account.
- `pinterest-pp-cli boards list` — Get a list of the boards owned by the 'operation user_account' + group boards where this account is a collaborator
- `pinterest-pp-cli boards update` — Update a board owned by the 'operating user_account'.

**business-access** — Manage business access

- `pinterest-pp-cli business-access brand-accounts-create` — Create a Brand Account that will be a child business of a business hierarchy.
- `pinterest-pp-cli business-access brand-accounts-update` — Update an existing Brand Account

**businesses** — Manage businesses

- `pinterest-pp-cli businesses get-business-employers` — Get all of the viewing user's business employers.
- `pinterest-pp-cli businesses respond-business-access-invites` — Accept or decline invites or requests.

**catalogs** — Manage information about shopping product catalogs and items.

- `pinterest-pp-cli catalogs available-filter-values` — Get the available filter attributes and values associated with a given feed or catalog owned by the 'operation
- `pinterest-pp-cli catalogs create` — Create a new catalog owned by the 'operation user_account'.
- `pinterest-pp-cli catalogs feed-processing-results-list` — Fetch a feed processing results owned by the 'operation user_account'.
- `pinterest-pp-cli catalogs feeds-create` — Create a new feed owned by the 'operation user_account'.
- `pinterest-pp-cli catalogs feeds-delete` — Delete a feed owned by the 'operation user_account'.
- `pinterest-pp-cli catalogs feeds-get` — Get a single feed owned by the 'operation user_account'.
- `pinterest-pp-cli catalogs feeds-ingest` — Ingest items for a given feed owned by the 'operation user_account'. _(restricted to a specific group of users)_
- `pinterest-pp-cli catalogs feeds-list` — Fetch feeds owned by the 'operation user_account'. - By default, the 'operation user_account' is the token user_account.
- `pinterest-pp-cli catalogs feeds-update` — Update a feed owned by the 'operation user_account'.
- `pinterest-pp-cli catalogs items-batch-get` — Get a single catalogs items batch owned by the 'operating user_account'. [See detailed documentation here.
- `pinterest-pp-cli catalogs items-batch-post` — This endpoint supports multiple operations on a set of one or more catalog items owned by the 'operation user_account'.
- `pinterest-pp-cli catalogs items-issues-list` — List item validation issues for a given feed processing result owned by the 'operation user_account'.
- `pinterest-pp-cli catalogs items-post` — Get the items of the catalog owned by the 'operation user_account'. [See detailed documentation here.
- `pinterest-pp-cli catalogs list` — Fetch catalogs owned by the 'operation user_account'.
- `pinterest-pp-cli catalogs product-group-pins-list` — Get a list of product pins for a given Catalogs Product Group Id owned by the 'operation user_account'.
- `pinterest-pp-cli catalogs product-groups-create` — Create product group to use in Catalogs owned by the 'operation user_account'.
- `pinterest-pp-cli catalogs product-groups-create-many` — Create product group to use in Catalogs owned by the 'operation user_account'.
- `pinterest-pp-cli catalogs product-groups-delete` — Delete a product group owned by the 'operation user_account' from being in use in Catalogs.
- `pinterest-pp-cli catalogs product-groups-delete-many` — Delete product groups owned by the 'operation user_account'.
- `pinterest-pp-cli catalogs product-groups-get` — Get a single product group for a given Catalogs Product Group Id owned by the 'operation user_account'.
- `pinterest-pp-cli catalogs product-groups-list` — Get a list of product groups for a given Catalogs Feed Id owned by the 'operation user_account'.
- `pinterest-pp-cli catalogs product-groups-product-counts-get` — Get a product counts for a given Catalogs Product Group owned by the 'operation user_account'.
- `pinterest-pp-cli catalogs product-groups-update` — Update product group owned by the 'operation user_account' to use in Catalogs.
- `pinterest-pp-cli catalogs products-by-product-group-filter-list` — List products Pins owned by the 'operation user_account' that meet the criteria specified in the Catalogs Product Group
- `pinterest-pp-cli catalogs reports-create` — Async request to create a report of the catalog owned by the 'operation user_account'.
- `pinterest-pp-cli catalogs reports-get` — This returns a URL to a report given a token returned from [Build catalogs
- `pinterest-pp-cli catalogs reports-stats` — List aggregated numbers of issues for a catalog owned by the 'operation user_account'.

**integrations** — View, create, or update commerce integrations.

- `pinterest-pp-cli integrations commerce-del` — Delete commerce integration metadata for the given external business ID. _(beta: contact your Pinterest account manager)_
- `pinterest-pp-cli integrations commerce-get` — Get commerce integration metadata associated with the given external business ID. _(beta: contact your Pinterest account manager)_
- `pinterest-pp-cli integrations commerce-patch` — Update commerce integration metadata for the given external business ID. _(beta: contact your Pinterest account manager)_
- `pinterest-pp-cli integrations commerce-post` — Create commerce integration metadata to link an external business ID with a Pinterest merchant & ad account. _(beta: contact your Pinterest account manager)_
- `pinterest-pp-cli integrations get-by-id` — Get integration metadata by ID. _(beta: contact your Pinterest account manager)_
- `pinterest-pp-cli integrations get-list` — Get integration metadata list. _(beta: contact your Pinterest account manager)_
- `pinterest-pp-cli integrations logs-post` — This endpoint receives batched logs from integration applications on partner platforms. _(beta: contact your Pinterest account manager)_

**media** — Register and manage media uploads.

- `pinterest-pp-cli media create` — Register your intent to upload media.
- `pinterest-pp-cli media get` — Get details for a registered media upload, including its current status.
- `pinterest-pp-cli media list` — List media uploads filtered by given parameters.

**notifications** — Send notifications.

- `pinterest-pp-cli notifications` — Used by third-party partners to send notifications to Pinterest. _(gated: Pinterest must grant access)_

**oauth** — Generate and refresh OAuth access tokens.

- `pinterest-pp-cli oauth conversion-token` — Generate a new and long-lived OAuth access token dedicated for sending conversions using a valid access token.
- `pinterest-pp-cli oauth token` — Generate a new OAuth access token using an authorization code
- `pinterest-pp-cli oauth token-revoke` — Revokes an access or refresh token. Only tokens issued for system users are currently supported.

**pins** — View, create, update, or delete information about Pins.

- `pinterest-pp-cli pins create` — Create a Pin on a board or board section owned by the 'operation user_account'.
- `pinterest-pp-cli pins delete` — Delete a Pins owned by the 'operation user_account' - or on a group board that has been shared with this account.
- `pinterest-pp-cli pins get` — Get a Pin owned by the 'operation user_account' - or on a group board that has been shared with this account.
- `pinterest-pp-cli pins list` — Get a list of the Pins owned by the 'operation user_account'.
- `pinterest-pp-cli pins multi-analytics` — **This endpoint is currently in beta and not available to all apps.**
- `pinterest-pp-cli pins update` — Update a pin owned by the 'operating user_account'. _(beta)_

**pinterest-search** — Manage pinterest search

- `pinterest-pp-cli pinterest-search partner-pins` — **This endpoint is currently in beta and not available to all apps.**
- `pinterest-pp-cli pinterest-search user-boards-get` — Search for boards for the 'operation user_account'. This includes boards of all board types.
- `pinterest-pp-cli pinterest-search user-pins-list` — Search for pins for the 'operation user_account'. - By default, the 'operation user_account' is the token user_account.

**resources** — View metadata about available metrics and targeting options in the Pinterest API.

- `pinterest-pp-cli resources ad-account-countries-get` — Get Ad Accounts countries
- `pinterest-pp-cli resources delivery-metrics-get` — Get the definitions for ads and organic metrics available across both synchronous and asynchronous report endpoints.
- `pinterest-pp-cli resources interest-targeting-options-get` — Get details of a specific interest given interest ID. Click [here](https://docs.google.
- `pinterest-pp-cli resources lead-form-questions-get` — Get a list of all lead form question type names. Some questions might not be used.
- `pinterest-pp-cli resources metrics-ready-state-get` — Learn whether conversion or non-conversion metrics are finalized and ready to query.
- `pinterest-pp-cli resources targeting-options-get` — You can use targeting values in ads placement to define your intended audience.

**terms** — View related and suggested terms for ads targeting.

- `pinterest-pp-cli terms related-list` — Get a list of terms logically related to each input term.
- `pinterest-pp-cli terms suggested-list` — Get popular search terms that begin with your input term.

**trends** — View trending terms, topics, and product categories.

- `pinterest-pp-cli trends editorial-articles-list` — Get a list of published editorial articles.
- `pinterest-pp-cli trends featured-topics-list` — Enables advertisers to pull top five trending topics by interest and market
- `pinterest-pp-cli trends product-categories-details-list` — Enables advertisers to retrieve demographic information, related pins, and trend lines for specified product categories
- `pinterest-pp-cli trends product-categories-trending-list` — Get a list of growing Shopping Product Categories in ranked order allowing filtering by engagement type, vertical, age
- `pinterest-pp-cli trends trending-keywords-list` — Get the top trending search keywords among the Pinterest user audience.

**user-account** — View user accounts associated with a given access token.

- `pinterest-pp-cli user-account analytics` — Get analytics for the 'operation user_account' - By default, the 'operation user_account' is the token user_account.
- `pinterest-pp-cli user-account analytics-top-pins` — Gets analytics data about a user's top pins (limited to the top 50).
- `pinterest-pp-cli user-account analytics-top-video-pins` — Gets analytics data about a user's top video pins (limited to the top 50).
- `pinterest-pp-cli user-account boards-user-follows-list` — Get a list of the boards a user follows. The request returns a board summary object array.
- `pinterest-pp-cli user-account follow-user-update` — **This endpoint is currently in beta and not available to all apps.**
- `pinterest-pp-cli user-account followers-list` — Get a list of your followers.
- `pinterest-pp-cli user-account get` — Get account information for the 'operation user_account' - By default
- `pinterest-pp-cli user-account linked-business-accounts-get` — Get a list of your linked business accounts.
- `pinterest-pp-cli user-account unverify-website-delete` — Unverify a website verified by the signed-in user.
- `pinterest-pp-cli user-account user-following-get` — Get a list of who a certain user follows.
- `pinterest-pp-cli user-account user-websites-get` — Get user websites, claimed or not
- `pinterest-pp-cli user-account verify-website-update` — Verify a website as a signed-in user.
- `pinterest-pp-cli user-account website-verification-get` — Get verification code for user to install on the website to claim it.

**users** — Manage users

- `pinterest-pp-cli users interests user-account-followed` — List the interests a user follows.


**ads** — Novel ads workflow (async report in one command)


- `pinterest-pp-cli ads report` — Create, wait for, and download an ads performance report in one command. Requires the ads:read scope and --ad-account-id (or PINTEREST_AD_ACCOUNT_ID).


**perf** — Account performance comparisons


- `pinterest-pp-cli perf compare` — See whether impressions, saves, and outbound clicks grew versus the previous equal-length period, with absolute and percent deltas.


**pins (workflow commands)**


- `pinterest-pp-cli pins audit` — Find published Pins missing alt text, a destination link, a title, or UTM tracking, grouped by board. Reads the local store; run sync first.

- `pinterest-pp-cli pins preflight` — Validate a proposed Pin (field limits, board and section by name, UTM link) and get a ready-to-send create body without writing anything. Board names resolve from the local store; run sync first.

- `pinterest-pp-cli pins funnel` — Rank your top Pins by impressions (or --sort-by metric), compute save and outbound-click rates, and flag Pins that get saved but don't send traffic.


**trends (workflow commands)**


- `pinterest-pp-cli trends gaps` — List trending keywords for a region with how many of your synced Pins cover each, zero-coverage first. Coverage counts need a synced local store.




## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `PINTEREST_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

Covered paths:

- `pinterest-pp-cli ad-accounts`
- `pinterest-pp-cli ad-accounts get`
- `pinterest-pp-cli ad-accounts list`
- `pinterest-pp-cli boards`
- `pinterest-pp-cli boards get`
- `pinterest-pp-cli boards list`
- `pinterest-pp-cli businesses`
- `pinterest-pp-cli catalogs`
- `pinterest-pp-cli catalogs list`
- `pinterest-pp-cli integrations`
- `pinterest-pp-cli media`
- `pinterest-pp-cli media get`
- `pinterest-pp-cli media list`
- `pinterest-pp-cli pins`
- `pinterest-pp-cli pins get`
- `pinterest-pp-cli pins list`
- `pinterest-pp-cli resources`
- `pinterest-pp-cli terms`
- `pinterest-pp-cli trends`
- `pinterest-pp-cli user-account`
- `pinterest-pp-cli user-account get`

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
pinterest-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

## Recipes


### Monthly growth check

```bash
pinterest-pp-cli perf compare --days 28 --agent
```

Current vs prior 28 days for impressions, saves, outbound clicks, and engagement.

### Narrow a large Pin listing

```bash
pinterest-pp-cli pins list --agent --select items.id,items.title,items.link,items.board_id
```

Pin payloads are large; select only the fields an agent needs.

### SEO cleanup list

```bash
pinterest-pp-cli pins audit --group-by board --agent
```

Pins missing alt text, links, titles, or UTM tags, grouped by board.

### Content gaps from trends

```bash
pinterest-pp-cli trends gaps --region US --agent
```

Trending keywords your synced Pins don't cover yet.

## Auth Setup

Pinterest uses OAuth 2.0 authorization code. Register an app at developers.pinterest.com with the redirect URI `http://localhost:8875/callback`, then log in:

```bash
PINTEREST_CLIENT_ID=<app-id> PINTEREST_CLIENT_SECRET=<app-secret> pinterest-pp-cli auth login --port 8875
```

The redirect URI must exactly match the one registered on the app. Token exchange and refresh authenticate the app with HTTP Basic, and continuous refresh keeps access alive.

Alternatives when you already have an access token: export it as `PINTEREST_ACCESS_TOKEN` (`PINTEREST_OAUTH2` is also accepted) for one-off runs, or store it with `echo "$TOKEN" | pinterest-pp-cli auth set-token`.

Run `pinterest-pp-cli doctor` to verify setup.

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
  pinterest-pp-cli ad-accounts list --agent --select bookmark,items
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit confirmation** — `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
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

- Use `--home <dir>` for one invocation, or set `PINTEREST_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `PINTEREST_CONFIG_DIR`, `PINTEREST_DATA_DIR`, `PINTEREST_STATE_DIR`, `PINTEREST_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `PINTEREST_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `pinterest-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "pinterest": {
        "command": "pinterest-pp-mcp",
        "env": {
          "PINTEREST_HOME": "/srv/pinterest"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `PINTEREST_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `PINTEREST_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, pass the question as an argv or MCP tool argument to `recall --agent`. Do not interpolate user-controlled text into a shell command line.

Quoted `recall "<question>"` breaks on an apostrophe, which is ordinary English. A quoted heredoc breaks when a body line equals the delimiter, and that delimiter is published in these docs. Write the question with a non-shell file-writing tool, then read it back as data:

```bash
# Write the question verbatim with your file-writing tool (no shell involved).
# Command substitution on a file only ever yields data — the shell never
# parses the file's bytes as syntax.
QUERY=$(cat /path/to/question.txt)
pinterest-pp-cli recall "$QUERY" --agent
```

Prefer MCP: pass the question as the tool's query argument. `"$QUERY"` after a file read is argv-safe; putting the question itself in the command text is not.

The response envelope:

```json
{
  "query": "...",
  "normalized": "<normalized form>",
  "query_entities": ["..."],
  "found": true | false,
  "match_score": 0.0,
  "results": [
    { "resource_id": "...", "resource_type": "...", "venue": "...",
      "confidence": 2, "entity_match": "exact|partial|unknown",
      "source": "taught|preseed|pattern", "warnings": ["..."] }
  ],
  "mismatches": [ /* only when --debug-mismatches */ ],
  "warnings": [ /* top-level */ ],
  "candidates": [
    { "id": 12, "class": "flag_alias | playbook_candidate",
      "summary": "...", "sightings": 3, "last_seen": "...",
      "rationale": "...",
      "next_action": ["<trial command>", "pinterest-pp-cli learnings confirm 12"] }
  ],
  "playbook": {
    "query_family": "...",
    "playbook": {
      "steps": [ { "cmd": "<command with {slot} substitution>", "purpose": "..." } ],
      "entity_slots": ["$ENTITY"],
      "expected_tool_calls": 3
    },
    "slots_resolved": { "$ENTITY": { "token": "<live token>", "canonical": "<canonical>" } },
    "notes": "<workarounds + gotchas for this query family>"
  },
  "notes": "<duplicate surface for non-playbook callers>"
}
```

Empty-store short-circuit: if the store has no learnings, playbooks, or candidates yet (recall finds nothing and `learnings list` and `learnings candidates` are both empty), skip recall for the rest of this session instead of taxing every query; resume recall-first once something has been taught.

### Step 2: decision tree

Read `candidates`, `playbook`, `notes`, `results[0]`, and warnings in that order:

```
if Candidates present (warnings include "candidates_present"):
    -> candidates are try-then-confirm, never facts. Follow each candidate's
       two-step next_action verbatim: run the trial command first, then run
       `learnings confirm <id>` only after the trial verified the behavior.
       Reject a wrong candidate with `learnings reject <id>`.
    -> NEVER re-teach something recall surfaced as a candidate; confirm or
       reject that candidate instead of teaching a duplicate.
    -> candidates ride alongside playbooks and resource hits, not instead of
       them; continue with the branches below after acting on them.

if Playbook present:
    -> READ Playbook.notes verbatim FIRST (workarounds + gotchas the CLI surface doesn't expose)
    -> replay Playbook.steps in order, substituting Playbook.slots_resolved entries
       for the entity slot tokens. If a step's slot is unresolved, fall back to
       discovery for that step only.
    -> the Playbook's expected_tool_calls is a budget; if you find yourself running
       materially more, record the divergence via `pinterest-pp-cli playbook amend`
       at end-of-session.

elif Notes present (no Playbook):
    -> read Notes verbatim before any discovery step; they carry known gotchas
       for this query family even when no structured choreography exists yet.

elif Found AND Results[0].EntityMatch == "exact" AND Results[0].Confidence >= 2:
    -> skip discovery; fetch live data for Results[*].ResourceID in parallel

elif Found AND Results[0].EntityMatch == "partial":
    -> candidate hint, NOT a hit; read the resource title to validate before trusting

elif (any row in Mismatches[] when --debug-mismatches was passed):
    -> treat as cold start; the stored learning is for a different entity
       (different canonical resolved from query_entities)

else:  // Found == false, no playbook, no notes
    -> cold start; run discovery normally; teach the answer afterward (Step 4).
       If the family has no playbook yet, that teach auto-synthesizes a
       playbook candidate from this session's journal - you do not need to
       record one by hand.
```

Playbook and Notes are orthogonal to the per-resource path. A recall response can carry both a Playbook AND a `Results[]` hit - use both: the Playbook tells you which choreography to run; the resource hits short-circuit specific steps. Default to skipping `mismatches`; pass `--debug-mismatches` only when investigating cold-start surprises.

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `pinterest-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `pinterest-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately. Pass the query the same way as recall — argv/MCP, or file-then-`$QUERY`. Do not splice the question into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
pinterest-pp-cli teach --query "$QUERY" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
QUERY=$(cat /path/to/question.txt)
pinterest-pp-cli teach \
  --query "$QUERY" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
QUERY=$(cat /path/to/question.txt)
pinterest-pp-cli teach-playbook \
  --query "$QUERY" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`. Pass the query and note as argv/MCP arguments, or write each with a non-shell file tool and read them back (`QUERY=$(cat ...)`, `NOTE=$(cat ...)`). Do not interpolate either string into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
NOTE=$(cat /path/to/note.txt)
pinterest-pp-cli playbook amend \
  --query "$QUERY" \
  --add-note "$NOTE"
# (append shell `&` to background it)
```

What counts as worth amending: a behavior you OBSERVED this session that future-you would benefit from knowing. Examples worth amending:

- A workaround for a CLI surface that silently drops or misorders a flag.
- An undocumented endpoint shape (response wrapped in `{meta, results}`, payload nested two levels deeper than the docs claim).
- Observed schema drift (a field renamed, an index that shifted between seasons, a category label that the API now returns lower-cased).

What does NOT belong in notes:

- The year-specific or entity-specific answer to the user's question. That's the response, not a learning.
- Per-team / per-athlete / per-row data the playbook already retrieves at runtime.
- Statements that paraphrase what the existing notes already say.

The amend command appends to the family's existing notes with a timestamped marker (`[amend YYYY-MM-DDTHH:MMZ]: <text>`). Multiple amends accumulate; the audit trail is visible. If no playbook exists yet for the family, amend creates a notes-only one (so cold-start corrections still land).

#### PII discipline for amend notes

`playbook amend` notes are designed to potentially flow upstream as shared knowledge in future versions of the Printing Press. Keep them clean of user-identifying content so the upstream-contribution path stays open without retroactive scrubbing:

- **Do NOT embed** paths to user filesystems, personal API keys or tokens, user email addresses, user GitHub handles, or specific query histories tied to a single user.
- **Acceptable**: endpoint shapes, undocumented field names, API gotchas, observed schema drift, workarounds for CLI surfaces, generalizable pagination or retry tactics.

If a correction is only meaningful with user-specific context, it belongs in a personal note, not in the playbook amend.

### Measuring the loop

`pinterest-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `PINTEREST_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
pinterest-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
pinterest-pp-cli feedback --stdin < notes.txt
pinterest-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `PINTEREST_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `PINTEREST_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
| 6 | Partial failure |
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
2. Match the user query to the best command from the Unique Capabilities and Command Reference above. The reference lists top-level commands only; for subgroups (boards sections, boards pins, pins analytics, pins save, ad-accounts campaigns / ad-groups / ads / audiences / reports, catalogs local-stores, businesses ...) run `pinterest-pp-cli which "your task" --json` or `pinterest-pp-cli <group> --help`.
3. Execute with the `--agent` flag:
   ```bash
   pinterest-pp-cli pins audit --group-by board --agent
   ```
4. If ambiguous, drill into subcommand help: `pinterest-pp-cli <command> --help`.
