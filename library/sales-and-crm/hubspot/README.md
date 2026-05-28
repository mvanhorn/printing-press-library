# HubSpot CLI

**Read-only contact analytics for HubSpot CRM — score drift, engagement decay, source ROI, and a daily digest, with a local store no other HubSpot tool ships.**

Every existing HubSpot CLI is CMS dev tooling or generic CRUD; every HubSpot MCP is conversational and write-heavy. This CLI mirrors the CRM Contacts API as typed commands, then layers nine cross-time analytics commands on top of a local SQLite store: engagement-decay, lifecycle-stuck, stale-but-valuable, source-roi, score-drift, owner-overload, silent-after-first-touch, duplicate-suspects, and daily-digest.

Learn more at [HubSpot](https://developers.hubspot.com/docs/api).

Printed by [@sdhilip200](https://github.com/sdhilip200) (sdhilip200).

## Install

The recommended path installs both the `hubspot-pp-cli` binary and the `pp-hubspot` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install hubspot
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install hubspot --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install hubspot --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install hubspot --agent claude-code
npx -y @mvanhorn/printing-press-library install hubspot --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/cmd/hubspot-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/hubspot-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-hubspot --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-hubspot --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-hubspot skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-hubspot. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/hubspot-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `HUBSPOT_PRIVATE_APP_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/cmd/hubspot-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "hubspot": {
      "command": "hubspot-pp-mcp",
      "env": {
        "HUBSPOT_PRIVATE_APP_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

HubSpot read-only access uses a Private App access token. Create one at https://app.hubspot.com/private-apps, grant `crm.objects.contacts.read` and `crm.schemas.contacts.read`, then export it as `HUBSPOT_PRIVATE_APP_TOKEN`. The CLI sends it as `Authorization: Bearer <token>`. No write scopes — by design.

## Quick Start

```bash
# Verify env var, token shape, and reachability without hitting paid endpoints
hubspot-pp-cli doctor --dry-run

# Incremental sync into local SQLite — pulls contacts via cursor pagination and snapshots score for trend math
hubspot-pp-cli sync

# Morning briefing — score deltas, lifecycle promotions, freshly stale VIPs
hubspot-pp-cli daily-digest --since 1d --json

# Re-engagement list with field projection — keeps agent context small
hubspot-pp-cli engagement-decay --window 30d --min-prior-opens 3 --json --select email,firstname,lastname,hs_last_activity_date

# Channel ranking by revenue, filtered to sources with enough volume to matter
hubspot-pp-cli source-roi --since 2026-01-01 --min-contacts 20 --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Cross-time analytics
- **`engagement-decay`** — Find previously-active contacts who have gone dark, ranked by prior engagement intensity.

  _Reach for this on Monday mornings to find warm leads that cooled — the classic re-engagement list a rep would otherwise build by hand._

  ```bash
  hubspot-pp-cli engagement-decay --window 30d --min-prior-opens 3 --json
  ```
- **`lifecycle-stuck`** — Surface contacts stuck in a lifecycle stage past team-median dwell time.

  _Reach for this when a sales manager asks 'who's breaching SLA?' — outliers relative to the team's own baseline, not a hardcoded threshold._

  ```bash
  hubspot-pp-cli lifecycle-stuck --stage marketingqualifiedlead --multiplier 2.0 --json
  ```
- **`score-drift`** — Find contacts whose HubSpot score dropped meaningfully over a window — leading indicator before lifecycle catches up.

  _Reach for this as an early disengagement signal that beats lifecycle stage by days or weeks._

  ```bash
  hubspot-pp-cli score-drift --window 14d --drop-pct 25 --json
  ```
- **`daily-digest`** — One-command morning briefing — new high-score contacts, biggest score drops, lifecycle promotions, freshly stale VIPs.

  _Reach for this on Monday morning as the single command a sales manager runs with coffee._

  ```bash
  hubspot-pp-cli daily-digest --since 1d --json
  ```

### Composite ranking
- **`stale-but-valuable`** — Find high-score, high-revenue contacts whose engagement has cooled — the VIPs going cold.

  _Reach for this when prioritizing save-the-whale outreach. Composite ranking is what makes it different from a plain filter._

  ```bash
  hubspot-pp-cli stale-but-valuable --score-min 70 --silent-days 21 --json
  ```
- **`source-roi`** — Rank contact acquisition sources by realized revenue and conversion, not raw volume.

  _Reach for this when marketing asks where to defund. Filters small-N sources so the ranking isn't dominated by noise._

  ```bash
  hubspot-pp-cli source-roi --since 2026-01-01 --min-contacts 20 --json
  ```

### Org-level pivots
- **`owner-overload`** — Flag sales reps carrying more active contacts than team median, weighted by lifecycle stage.

  _Reach for this when balancing book-of-business across AEs — the standard manual answer is 'export to Sheets and pivot.'_

  ```bash
  hubspot-pp-cli owner-overload --weight-by-stage --top 10 --json
  ```

### Pattern detection
- **`silent-after-first-touch`** — Find contacts who engaged exactly once and then went silent — distinguish tire-kickers from genuine interest.

  _Reach for this when triaging which one-touch contacts are worth a follow-up vs. archive._

  ```bash
  hubspot-pp-cli silent-after-first-touch --since 90d --json
  ```
- **`duplicate-suspects`** — Surface likely duplicate contacts via fuzzy match on email-local-part, normalized name, and company.

  _Reach for this during quarterly CRM hygiene. Outputs pair candidates with scores; merge stays manual (write scope intentionally out)._

  ```bash
  hubspot-pp-cli duplicate-suspects --threshold 0.85 --json
  ```

## Recipes


### Morning sales-manager digest

```bash
hubspot-pp-cli daily-digest --since 1d --json --select score_movers,stage_promotions,fresh_stale_vips
```

One command, three sections of cross-time deltas. Use `--select` so the response stays small enough to thread through an agent.

### Find dark VIPs

```bash
hubspot-pp-cli stale-but-valuable --score-min 70 --silent-days 21 --limit 25 --json
```

Composite rank by value × silence. Pair with `engagement-decay` to triangulate which save-the-whale candidates deserve outreach this week.

### Channel ROI without small-N noise

```bash
hubspot-pp-cli source-roi --since 2026-01-01 --min-contacts 20 --json
```

Joins source attribution to deal revenue, filters sources below the volume threshold so noisy 2-contact channels can't dominate.

### Owner load sanity check

```bash
hubspot-pp-cli owner-overload --weight-by-stage --top 10 --json
```

Weighted by lifecycle stage so SQLs count more than Leads. Shows reps above the team median — surface for the next 1:1.

### Dedupe candidates for hygiene sprints

```bash
hubspot-pp-cli duplicate-suspects --threshold 0.85 --json --select left_email,right_email,similarity,evidence
```

Pairwise fuzzy match goes beyond HubSpot's exact-email dedupe. `--select` keeps the envelope tight.

## Usage

Run `hubspot-pp-cli --help` for the full command reference and flag list.

## Commands

### crm

Manage HubSpot CRM contact objects (read, create, update, archive, batch, merge, search).

> **Write endpoints note.** The CLI also exposes write endpoints (create/update/archive/batch operations, gdpr-delete, merge) generated from the upstream OpenAPI spec. They are NOT used by the headline analytics commands (`engagement-decay`, `lifecycle-stuck`, `score-drift`, `daily-digest`, `stale-but-valuable`, `source-roi`, `owner-overload`, `silent-after-first-touch`, `duplicate-suspects`) and will return HTTP 403 if you set up the recommended read-only Private App scopes (`crm.objects.contacts.read`, `crm.schemas.contacts.read`).

- **`hubspot-pp-cli crm delete-v3-objects-contacts-contact-id-archive`** - Delete a contact by ID. Deleted contacts can be restored within 90 days of deletion. Learn more about the [data impacted by contact deletions](https://knowledge.hubspot.com/privacy-and-consent/understand-restorable-and-permanent-contact-deletions) and how to [restore archived records](https://knowledge.hubspot.com/records/restore-deleted-records).
- **`hubspot-pp-cli crm get-v3-objects-contacts-contact-id-get-by-id`** - Retrieve a contact by its ID (`contactId`) or by a unique property (`idProperty`). You can specify what is returned using the `properties` query parameter.
- **`hubspot-pp-cli crm get-v3-objects-contacts-get-page`** - Retrieve all contacts, using query parameters to specify the information that gets returned.
- **`hubspot-pp-cli crm patch-v3-objects-contacts-contact-id-update`** - Update an existing contact, identified by ID or email/unique property value. To identify a contact by ID, include the ID in the request URL path. To identify a contact by their email or other unique property, include the email/property value in the request URL path, and add the `idProperty` query parameter (`/crm/v3/objects/contacts/<contact-email>?idProperty=email`). Provided property values will be overwritten. Read-only and non-existent properties will result in an error. Properties values can be cleared by passing an empty string.
- **`hubspot-pp-cli crm post-v3-objects-contacts-batch-archive-archive`** - Archive a batch of contacts by ID. Archived contacts can be restored within 90 days of deletion. Learn more about the [data impacted by contact deletions](https://knowledge.hubspot.com/privacy-and-consent/understand-restorable-and-permanent-contact-deletions) and how to [restore archived records](https://knowledge.hubspot.com/records/restore-deleted-records).
- **`hubspot-pp-cli crm post-v3-objects-contacts-batch-create-create`** - Create a batch of contacts. The `inputs` array can contain a `properties` object to define property values for each record, along with an `associations` array to define [associations](https://developers.hubspot.com/docs/guides/api/crm/associations/associations-v4) with other CRM records.
- **`hubspot-pp-cli crm post-v3-objects-contacts-batch-read-read`** - Retrieve a batch of contacts by ID (`contactId`) or unique property value (`idProperty`).
- **`hubspot-pp-cli crm post-v3-objects-contacts-batch-update-update`** - Update a batch of contacts by ID (`contactId`) or unique property value (`idProperty`). Provided property values will be overwritten. Read-only and non-existent properties will result in an error. Properties values can be cleared by passing an empty string.
- **`hubspot-pp-cli crm post-v3-objects-contacts-batch-upsert-upsert`** - Upsert a batch of contacts. The `inputs` array can contain a `properties` object to define property values for each record.
- **`hubspot-pp-cli crm post-v3-objects-contacts-create`** - Create a single contact. Include a `properties` object to define [property values](https://developers.hubspot.com/docs/guides/api/crm/properties) for the contact, along with an `associations` array to define [associations](https://developers.hubspot.com/docs/guides/api/crm/associations/associations-v4) with other CRM records.
- **`hubspot-pp-cli crm post-v3-objects-contacts-gdpr-delete-purge`** - Permanently delete a contact and all associated content to follow GDPR. Use optional property `idProperty` set to `email` to identify contact by email address. If email address is not found, the email address will be added to a blocklist and prevent it from being used in the future. Learn more about [permanently deleting contacts](https://knowledge.hubspot.com/privacy-and-consent/how-do-i-perform-a-gdpr-delete-in-hubspot).
- **`hubspot-pp-cli crm post-v3-objects-contacts-merge-merge`** - Merge two contact records. Learn more about [merging records](https://knowledge.hubspot.com/records/merge-records).
- **`hubspot-pp-cli crm post-v3-objects-contacts-search-do-search`** - Search for contacts by filtering on properties, searching through associations, and sorting results. Learn more about [CRM search](https://developers.hubspot.com/docs/guides/api/crm/search#make-a-search-request).


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
hubspot-pp-cli crm get-v3-objects-contacts-get-page --limit 5

# JSON for scripting and agents
hubspot-pp-cli crm get-v3-objects-contacts-get-page --limit 5 --json

# Filter to specific fields
hubspot-pp-cli crm get-v3-objects-contacts-get-page --limit 5 --json --select id,properties.email,properties.firstname

# Dry run — show the request without sending
hubspot-pp-cli crm get-v3-objects-contacts-get-page --limit 5 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
hubspot-pp-cli crm get-v3-objects-contacts-get-page --limit 5 --agent
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
hubspot-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/hubspot-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `HUBSPOT_PRIVATE_APP_TOKEN` | per_call | No | Set to your API credential. |
| `HUBSPOT_OAUTH2` | per_call | No | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `hubspot-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `hubspot-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $HUBSPOT_PRIVATE_APP_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized on every call** — Re-create the Private App at https://app.hubspot.com/private-apps with `crm.objects.contacts.read` and `crm.schemas.contacts.read` scopes, then `export HUBSPOT_PRIVATE_APP_TOKEN=<new-token>`.
- **Empty results from analytics commands** — Run `hubspot-pp-cli sync` first — analytics commands query the local store, which is empty until the first sync completes.
- **429 rate-limit during sync** — HubSpot allows ~100 req/10s. The CLI backs off automatically; re-run `sync` to resume from the cursor.
- **score-drift returns no rows** — score-drift needs at least two sync snapshots to compute a delta. Run `sync` twice with the desired window between runs.
- **engagement-decay returns empty** — Pass `--param properties=hs_email_open,hs_email_click,hs_last_activity_date` to `sync` so the local store carries the engagement fields; the default property set omits them.
- **lifecycle-stuck dwell looks too short** — Default sync doesn't return per-stage entry timestamps; lifecycle-stuck approximates dwell from `createdate`. For long-lived contacts the approximation can compress true dwell relative to a stage-entered timestamp.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**HubSpot/hubspot-cli**](https://github.com/HubSpot/hubspot-cli) — TypeScript (195 stars)
- [**peakmojo/mcp-hubspot**](https://github.com/baryhuang/mcp-hubspot) — Python (124 stars)
- [**clarkmcc/go-hubspot**](https://github.com/clarkmcc/go-hubspot) — Go (34 stars)
- [**lkm1developer/hubspot-mcp-server**](https://github.com/lkm1developer/hubspot-mcp-server) — TypeScript (13 stars)
- [**leonelquinteros/hubspot**](https://github.com/leonelquinteros/hubspot) — Go (12 stars)
- [**open-cli-collective/hubspot-cli**](https://github.com/open-cli-collective/hubspot-cli) — Go (1 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
