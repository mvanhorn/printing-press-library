---
name: pp-beehiiv
description: "Printing Press CLI for Beehiiv."
author: "Kevin Magnan"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - beehiiv-pp-cli
---

# Beehiiv — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `beehiiv-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install beehiiv --cli-only
   ```
2. Verify: `beehiiv-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/beehiiv/cmd/beehiiv-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Publication intelligence
- **`insights growth-summary`** — Summarize publication, subscriber, post, referral, and custom-field health in one read-only response.

  _Use this first when an agent needs a compact account-level picture before choosing a narrower Beehiiv endpoint._

  ```bash
  beehiiv-pp-cli insights growth-summary pub_00000000-0000-0000-0000-000000000000 --agent
  ```
- **`insights post-performance`** — List recent posts with status, audience, publish timing, and any available expanded stats.

  _Use this when an agent needs to inspect content output before drilling into one post._

  ```bash
  beehiiv-pp-cli insights post-performance pub_00000000-0000-0000-0000-000000000000 --limit 25 --agent
  ```

### Audience intelligence
- **`insights subscriber-sources`** — Group subscribers by UTM, channel, and referring-site fields to see where audience growth is coming from.

  _Use this when the question is about acquisition channels rather than individual subscribers._

  ```bash
  beehiiv-pp-cli insights subscriber-sources pub_00000000-0000-0000-0000-000000000000 --limit 100 --agent
  ```
- **`insights field-coverage`** — Inspect custom-field definitions alongside subscriber sample size for enrichment planning.

  _Use this before importing, enriching, or auditing subscriber metadata._

  ```bash
  beehiiv-pp-cli insights field-coverage pub_00000000-0000-0000-0000-000000000000 --agent
  ```
- **`insights subscriber-lookup`** — Find one subscriber by email or subscription ID and return a compact subscriber record.

  _Use this when the task is about one subscriber and broad list calls would waste context._

  ```bash
  beehiiv-pp-cli insights subscriber-lookup pub_00000000-0000-0000-0000-000000000000 --email reader@example.com --agent
  ```

### Growth loops
- **`insights referral-health`** — Summarize referral-program configuration and subscriber referral-code coverage.

  _Use this when an agent needs to check whether referral growth is configured and visible in subscriber data._

  ```bash
  beehiiv-pp-cli insights referral-health pub_00000000-0000-0000-0000-000000000000 --agent
  ```

## Command Reference

**advertisement-opportunities** — Manage advertisement opportunities

- `beehiiv-pp-cli advertisement-opportunities <publicationId>` — Get advertisement opportunities <Badge intent='info' minimal outlined>OAuth Scope: posts:read</Badge>

**authors** — Manage authors

- `beehiiv-pp-cli authors index` — Retrieve a list of authors available for the publication.
- `beehiiv-pp-cli authors show` — Retrieve a single author from a publication.

**automations** — Manage automations

- `beehiiv-pp-cli automations index` — List automations <Badge intent='info' minimal outlined>OAuth Scope: automations:read</Badge>
- `beehiiv-pp-cli automations show` — Get automation <Badge intent='info' minimal outlined>OAuth Scope: automations:read</Badge>

**bulk-subscription-updates** — Manage bulk subscription updates

- `beehiiv-pp-cli bulk-subscription-updates index` — List subscription updates <Badge intent='info' minimal outlined>OAuth Scope: subscriptions:read</Badge>
- `beehiiv-pp-cli bulk-subscription-updates show` — Get subscription update <Badge intent='info' minimal outlined>OAuth Scope: subscriptions:read</Badge>

**bulk-subscriptions** — Manage bulk subscriptions

- `beehiiv-pp-cli bulk-subscriptions <publicationId>` — Bulk create subscription <Badge intent='info' minimal outlined>OAuth Scope: subscriptions:write</Badge>

**condition-sets** — Manage condition sets

- `beehiiv-pp-cli condition-sets index` — Retrieve all active condition sets for a publication. Condition sets define reusable audience segments for targeting...
- `beehiiv-pp-cli condition-sets show` — Retrieve a single active dynamic content condition set for a publication. Use `expand[]=stats` to calculate and...

**custom-fields** — Manage custom fields

- `beehiiv-pp-cli custom-fields create` — Create custom field <Badge intent='info' minimal outlined>OAuth Scope: custom_fields:write</Badge>
- `beehiiv-pp-cli custom-fields delete` — Delete custom field <Badge intent='info' minimal outlined>OAuth Scope: custom_fields:write</Badge>
- `beehiiv-pp-cli custom-fields index` — List custom fields <Badge intent='info' minimal outlined>OAuth Scope: custom_fields:read</Badge>
- `beehiiv-pp-cli custom-fields patch` — Update custom field <Badge intent='info' minimal outlined>OAuth Scope: custom_fields:write</Badge>
- `beehiiv-pp-cli custom-fields put` — Update custom field <Badge intent='info' minimal outlined>OAuth Scope: custom_fields:write</Badge>
- `beehiiv-pp-cli custom-fields show` — Get custom field <Badge intent='info' minimal outlined>OAuth Scope: custom_fields:read</Badge>

**data-privacy** — Manage data privacy

- `beehiiv-pp-cli data-privacy data-deletion-create` — <Warning>This is a gated feature that requires enablement. Contact support to enable Data Deletion API access for...
- `beehiiv-pp-cli data-privacy data-deletion-index` — <Warning>This is a gated feature that requires enablement. Contact support to enable Data Deletion API access for...
- `beehiiv-pp-cli data-privacy data-deletion-show` — <Warning>This is a gated feature that requires enablement. Contact support to enable Data Deletion API access for...

**email-blasts** — Manage email blasts

- `beehiiv-pp-cli email-blasts index` — List email blasts <Badge intent='info' minimal outlined>OAuth Scope: posts:read</Badge>
- `beehiiv-pp-cli email-blasts show` — Get email blast <Badge intent='info' minimal outlined>OAuth Scope: posts:read</Badge>

**engagements** — Manage engagements

- `beehiiv-pp-cli engagements <publicationId>` — Retrieve email engagement metrics for a specific publication over a defined date range and granularity.<br><br> By...

**newsletter-lists** — Manage newsletter lists

- `beehiiv-pp-cli newsletter-lists index` — <Note title='Currently in beta' icon='b'> Newsletter Lists is currently in beta, the API is subject to change....
- `beehiiv-pp-cli newsletter-lists show` — <Note title='Currently in beta' icon='b'> Newsletter Lists is currently in beta, the API is subject to change....

**polls** — Manage polls

- `beehiiv-pp-cli polls index` — Retrieve all polls belonging to a specific publication. Poll choices are always included. Use `expand[]=stats` to...
- `beehiiv-pp-cli polls show` — Retrieve detailed information about a specific poll belonging to a publication. Use `expand[]=stats` for aggregate...

**post-templates** — Manage post templates

- `beehiiv-pp-cli post-templates <publicationId>` — Retrieve a list of post templates available for the publication.

**posts** — Manage posts

- `beehiiv-pp-cli posts aggregate-stats` — Get aggregate stats <Badge intent='info' minimal outlined>OAuth Scope: posts:read</Badge>
- `beehiiv-pp-cli posts create` — <Note title='Currently in beta' icon='b'> This feature is currently in beta, the API is subject to change, and...
- `beehiiv-pp-cli posts delete` — Delete or Archive a post. Any post that has been confirmed will have it's status changed to `archived`. Posts in the...
- `beehiiv-pp-cli posts index` — List posts <Badge intent='info' minimal outlined>OAuth Scope: posts:read</Badge>
- `beehiiv-pp-cli posts show` — Get post <Badge intent='info' minimal outlined>OAuth Scope: posts:read</Badge>
- `beehiiv-pp-cli posts update` — <Note title='Currently in beta' icon='b'> This feature is currently in beta, the API is subject to change, and...

**publications** — Manage publications

- `beehiiv-pp-cli publications index` — List publications <Badge intent='info' minimal outlined>OAuth Scope: publications:read</Badge>
- `beehiiv-pp-cli publications show` — Get publication <Badge intent='info' minimal outlined>OAuth Scope: publications:read</Badge>

**referral-program** — Manage referral program

- `beehiiv-pp-cli referral-program <publicationId>` — Get referral program <Badge intent='info' minimal outlined>OAuth Scope: referral_program:read</Badge>

**segments** — Manage segments

- `beehiiv-pp-cli segments create` — Create a new segment.<br><br> **Manual segments** — Use `subscriptions` or `emails` input to create a segment from...
- `beehiiv-pp-cli segments delete` — Delete a segment. Deleting the segment does not effect the subscriptions in the segment.
- `beehiiv-pp-cli segments index` — List segments <Badge intent='info' minimal outlined>OAuth Scope: segments:read</Badge>
- `beehiiv-pp-cli segments show` — Get segment <Badge intent='info' minimal outlined>OAuth Scope: segments:read</Badge>

**subscriptions** — Manage subscriptions

- `beehiiv-pp-cli subscriptions bulk-updates-patch` — Update subscriptions <Badge intent='info' minimal outlined>OAuth Scope: subscriptions:write</Badge>
- `beehiiv-pp-cli subscriptions bulk-updates-patch-status` — Update subscriptions' status <Badge intent='info' minimal outlined>OAuth Scope: subscriptions:write</Badge>
- `beehiiv-pp-cli subscriptions bulk-updates-put` — Update subscriptions <Badge intent='info' minimal outlined>OAuth Scope: subscriptions:write</Badge>
- `beehiiv-pp-cli subscriptions bulk-updates-put-status` — Update subscriptions' status <Badge intent='info' minimal outlined>OAuth Scope: subscriptions:write</Badge>
- `beehiiv-pp-cli subscriptions create` — Create subscription <Badge intent='info' minimal outlined>OAuth Scope: subscriptions:write</Badge>
- `beehiiv-pp-cli subscriptions delete` — <Warning>This cannot be undone. All data associated with the subscription will also be deleted. We recommend...
- `beehiiv-pp-cli subscriptions get-by-email` — <Info>Please note that this endpoint requires the email to be URL encoded. Please reference your language's...
- `beehiiv-pp-cli subscriptions get-by-id` — <Info>In previous versions of the API, another endpoint existed to retrieve a subscription by the subscriber ID....
- `beehiiv-pp-cli subscriptions get-by-subscriber-id` — Get subscription by subscriber ID <Badge intent='info' minimal outlined>OAuth Scope: subscriptions:read</Badge>
- `beehiiv-pp-cli subscriptions index` — Retrieve all subscriptions belonging to a specific publication. <Info> **New**: This endpoint now supports...
- `beehiiv-pp-cli subscriptions patch` — Update subscription by ID <Badge intent='info' minimal outlined>OAuth Scope: subscriptions:write</Badge>
- `beehiiv-pp-cli subscriptions put` — Update subscription by ID <Badge intent='info' minimal outlined>OAuth Scope: subscriptions:write</Badge>
- `beehiiv-pp-cli subscriptions update-by-email` — Update subscription by email <Badge intent='info' minimal outlined>OAuth Scope: subscriptions:write</Badge>

**tiers** — Manage tiers

- `beehiiv-pp-cli tiers create` — Create a tier <Badge intent='info' minimal outlined>OAuth Scope: tiers:write</Badge>
- `beehiiv-pp-cli tiers index` — List tiers <Badge intent='info' minimal outlined>OAuth Scope: tiers:read</Badge>
- `beehiiv-pp-cli tiers patch` — Update a tier <Badge intent='info' minimal outlined>OAuth Scope: tiers:write</Badge>
- `beehiiv-pp-cli tiers put` — Update a tier <Badge intent='info' minimal outlined>OAuth Scope: tiers:write</Badge>
- `beehiiv-pp-cli tiers show` — Get tier <Badge intent='info' minimal outlined>OAuth Scope: tiers:read</Badge>

**users** — Manage users

- `beehiiv-pp-cli users` — Identify user <Badge intent='info' minimal outlined>OAuth Scope: identify:read</Badge>

**webhooks** — Manage webhooks

- `beehiiv-pp-cli webhooks create` — Create a webhook <Badge intent='info' minimal outlined>OAuth Scope: webhooks:write</Badge>
- `beehiiv-pp-cli webhooks delete` — Delete a webhook <Badge intent='info' minimal outlined>OAuth Scope: webhooks:write</Badge>
- `beehiiv-pp-cli webhooks index` — List webhooks <Badge intent='info' minimal outlined>OAuth Scope: webhooks:read</Badge>
- `beehiiv-pp-cli webhooks show` — Get webhook <Badge intent='info' minimal outlined>OAuth Scope: webhooks:read</Badge>
- `beehiiv-pp-cli webhooks update` — Update webhook <Badge intent='info' minimal outlined>OAuth Scope: webhooks:write</Badge>

**workspaces** — Manage workspaces

- `beehiiv-pp-cli workspaces identify` — Identify workspace <Badge intent='info' minimal outlined>OAuth Scope: identify:read</Badge>
- `beehiiv-pp-cli workspaces publications-by-subscription-email` — Retrieve all publications in the workspace that have a subscription for the specified email address. The workspace...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
beehiiv-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Store your access token:

```bash
beehiiv-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `BEEHIIV_BEARER_AUTH` as an environment variable.

Run `beehiiv-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  beehiiv-pp-cli advertisement-opportunities mock-value --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
beehiiv-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
beehiiv-pp-cli feedback --stdin < notes.txt
beehiiv-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.beehiiv-pp-cli/feedback.jsonl`. They are never POSTed unless `BEEHIIV_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `BEEHIIV_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
beehiiv-pp-cli profile save briefing --json
beehiiv-pp-cli --profile briefing advertisement-opportunities mock-value
beehiiv-pp-cli profile list --json
beehiiv-pp-cli profile show briefing
beehiiv-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `beehiiv-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add beehiiv-pp-mcp -- beehiiv-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which beehiiv-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   beehiiv-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `beehiiv-pp-cli <command> --help`.
