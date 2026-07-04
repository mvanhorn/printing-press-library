---
name: pp-google-business-profile
description: "Printing Press CLI for Google Business Profile. Combined CLI for multiple API services"
author: "Raj Yaligar"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - google-business-profile-pp-cli
    install:
      - kind: go
        bins: [google-business-profile-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/marketing/google-business-profile/cmd/google-business-profile-pp-cli
---

# Google Business Profile — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `google-business-profile-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install google-business-profile --cli-only
   ```
2. Verify: `google-business-profile-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/google-business-profile/cmd/google-business-profile-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Use the Google Business Profile CLI to manage API resources, then archive them locally for fast search, grouped analytics, and rollout verification. It is designed for operator workflows where repeated read access and auditability matter as much as raw endpoint coverage.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local operator workflows
- **`workflow archive`** — Build a local SQLite archive of Google Business Profile resources so agents can inspect accounts and locations without re-hitting the API on every question.

  _Lets an agent answer follow-up questions from a synchronized snapshot instead of burning quota and latency on repeated list calls._

  ```bash
  google-business-profile-pp-cli workflow archive --agent
  ```
- **`workflow status`** — Show local archive freshness and sync state across resource tables so operators know whether offline answers are still trustworthy.

  _Prevents agents from making stale operational recommendations from an old archive._

  ```bash
  google-business-profile-pp-cli workflow status --agent
  ```
- **`search`** — Search locally synced Business Profile data with FTS5 when a native API search path is absent or live auth is unavailable.

  _Gives fast operator lookup across synced records without crafting custom filters for each API surface._

  ```bash
  google-business-profile-pp-cli search "Toronto" --data-source local --type locations --limit 20 --json
  ```
- **`analytics`** — Run grouped analytics over synced data to count resources and surface distribution patterns without exporting to spreadsheets first.

  _Turns the CLI into a lightweight reporting surface for location inventory and QA work._

  ```bash
  google-business-profile-pp-cli analytics --type locations --group-by storeCode --limit 25 --json
  ```

### Monitoring and verification
- **`tail`** — Poll the API and emit NDJSON change events so operators can watch location-related activity in near real time.

  _Makes the CLI useful for active monitoring, not just one-shot API calls._

  ```bash
  google-business-profile-pp-cli tail --resource locations --interval 30s --follow=false --agent
  ```

## Recipes

### Archive then search for a location clue

```bash
google-business-profile-pp-cli workflow archive --agent && google-business-profile-pp-cli search "Main Street" --data-source local --type locations --limit 10 --json
```

Refresh the local store, then search it immediately for a location name, city, or other text clue.

### Count locations by a local field

```bash
google-business-profile-pp-cli analytics --type locations --group-by storeCode --limit 25 --json
```

Get a quick grouped count from the synced archive without exporting records to another tool first.

## Command Reference

**accounts** — Manage accounts

- `google-business-profile-pp-cli accounts accept` — Accepts the specified invitation.
- `google-business-profile-pp-cli accounts create` — Creates an account with the specified name and type under the given parent.
- `google-business-profile-pp-cli accounts create-admins` — Invites the specified user to become an administrator for the specified account.
- `google-business-profile-pp-cli accounts decline` — Declines the specified invitation.
- `google-business-profile-pp-cli accounts delete` — Removes the specified admin from the specified account.
- `google-business-profile-pp-cli accounts get` — Gets the specified account.
- `google-business-profile-pp-cli accounts list` — Lists all of the accounts for the authenticated user.
- `google-business-profile-pp-cli accounts list-admins` — Lists the admins for the specified account.
- `google-business-profile-pp-cli accounts list-invitations` — Lists pending invitations for the specified account.
- `google-business-profile-pp-cli accounts patch` — Updates the Admin for the specified Account Admin.

**attributes** — Manage attributes

- `google-business-profile-pp-cli attributes` — Returns the list of attributes that would be available for a location with the given primary category and country.

**business-profile-performance-locations** — Manage business profile performance locations

- `google-business-profile-pp-cli business-profile-performance-locations fetch-multi-daily-metrics-time-series` — Returns the values for each date from a given time range that are associated with the specific daily metrics.
- `google-business-profile-pp-cli business-profile-performance-locations get-daily-metrics-time-series` — Returns the values for each date from a given time range that are associated with the specific daily metric.
- `google-business-profile-pp-cli business-profile-performance-locations list` — Returns the search keywords used to find a business in search or maps.

**categories** — Manage categories

- `google-business-profile-pp-cli categories batch-get` — Returns a list of business categories for the provided language and GConcept ids.
- `google-business-profile-pp-cli categories list` — Returns a list of business categories. Search will match the category name but not the category ID.

**chains** — Manage chains

- `google-business-profile-pp-cli chains get` — Gets the specified chain. Returns `NOT_FOUND` if the chain does not exist.
- `google-business-profile-pp-cli chains search` — Searches the chain based on chain name.

**google_locations** — Manage google locations

- `google-business-profile-pp-cli google-locations search` — Search all of the possible locations that are a match to the specified request.

**locations** — Manage locations

- `google-business-profile-pp-cli locations <name>` — Moves a location from an account that the user owns to another account that the same user administers.

**my-business-business-accounts** — Manage my business business accounts

- `google-business-profile-pp-cli my-business-business-accounts create` — Creates a new Location that will be owned by the logged in user.
- `google-business-profile-pp-cli my-business-business-accounts list` — Lists the locations for the specified account.

**my-business-business-locations** — Manage my business business locations

- `google-business-profile-pp-cli my-business-business-locations delete` — Deletes a location. If this location cannot be deleted using the API and it is marked so in the `google.mybusiness.
- `google-business-profile-pp-cli my-business-business-locations get-google-updated` — Retrieves attributes for a location as they appear live on Google Maps and Search.
- `google-business-profile-pp-cli my-business-business-locations patch` — Updates the specified location.

**my-business-lodging-locations** — Manage my business lodging locations

- `google-business-profile-pp-cli my-business-lodging-locations get-google-updated` — Returns the Google updated Lodging of a specific location.
- `google-business-profile-pp-cli my-business-lodging-locations get-lodging` — Returns the Lodging of a specific location.
- `google-business-profile-pp-cli my-business-lodging-locations update-lodging` — Updates the Lodging of a specific location.

**my-business-notifications-accounts** — Manage my business notifications accounts

- `google-business-profile-pp-cli my-business-notifications-accounts get-notification-setting` — Returns the pubsub notification settings for the account.
- `google-business-profile-pp-cli my-business-notifications-accounts update-notification-setting` — Sets the pubsub notification setting for the account informing Google which topic to send pubsub notifications for.

**my-business-place-locations** — Manage my business place locations

- `google-business-profile-pp-cli my-business-place-locations create` — Creates a place action link associated with the specified location, and returns it.
- `google-business-profile-pp-cli my-business-place-locations delete` — Deletes a place action link from the specified location.
- `google-business-profile-pp-cli my-business-place-locations get` — Gets the specified place action link.
- `google-business-profile-pp-cli my-business-place-locations list` — Lists the place action links for the specified location.
- `google-business-profile-pp-cli my-business-place-locations patch` — Updates the specified place action link and returns it.

**my-business-q-locations** — Manage my business q locations

- `google-business-profile-pp-cli my-business-q-locations create` — Adds a question for the specified location.
- `google-business-profile-pp-cli my-business-q-locations delete` — Deletes a specific question written by the current user.
- `google-business-profile-pp-cli my-business-q-locations delete-answersdelete` — Deletes the answer written by the current user to a question.
- `google-business-profile-pp-cli my-business-q-locations list` — Returns the paginated list of questions and some of its answers for a specified location.
- `google-business-profile-pp-cli my-business-q-locations list-answers` — Returns the paginated list of answers for a specified question.
- `google-business-profile-pp-cli my-business-q-locations patch` — Updates a specific question written by the current user.
- `google-business-profile-pp-cli my-business-q-locations upsert` — Creates an answer or updates the existing answer written by the user for the specified question.

**my-business-verifications-locations** — Manage my business verifications locations

- `google-business-profile-pp-cli my-business-verifications-locations complete` — Completes a `PENDING` verification. It is only necessary for non `AUTO` verification methods.
- `google-business-profile-pp-cli my-business-verifications-locations fetch-verification-options` — Reports all eligible verification options for a location in a specific language.
- `google-business-profile-pp-cli my-business-verifications-locations get-voice-of-merchant-state` — Gets the VoiceOfMerchant state.
- `google-business-profile-pp-cli my-business-verifications-locations list` — List verifications of a location, ordered by create time.
- `google-business-profile-pp-cli my-business-verifications-locations verify` — Starts the verification process for a location.

**place_action_type_metadata** — Manage place action type metadata

- `google-business-profile-pp-cli place-action-type-metadata` — Returns the list of available place action types for a location or country.

**verification_tokens** — Manage verification tokens

- `google-business-profile-pp-cli verification-tokens` — Generate a token for the provided location data to verify the location.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
google-business-profile-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Authenticate with OAuth2 using `google-business-profile-pp-cli login` so the CLI can mint and refresh access tokens for Google Business Profile APIs. The `GOOGLE_BUSINESS_PROFILE_OAUTH2` env var is a fallback only when you already have a short-lived access token and need a non-interactive run.

Run `google-business-profile-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  google-business-profile-pp-cli accounts list --agent --select id,name,status
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
google-business-profile-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
google-business-profile-pp-cli feedback --stdin < notes.txt
google-business-profile-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/google-business-profile-pp-cli/feedback.jsonl`. They are never POSTed unless `GOOGLE_BUSINESS_PROFILE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `GOOGLE_BUSINESS_PROFILE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
google-business-profile-pp-cli profile save briefing --json
google-business-profile-pp-cli --profile briefing accounts list
google-business-profile-pp-cli profile list --json
google-business-profile-pp-cli profile show briefing
google-business-profile-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `google-business-profile-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/google-business-profile/cmd/google-business-profile-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add google-business-profile-pp-mcp -- google-business-profile-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which google-business-profile-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   google-business-profile-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `google-business-profile-pp-cli <command> --help`.
