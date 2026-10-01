---
name: pp-tailscale
description: "Tailscale admin from the terminal, with safe route and policy edits an agent can plan before it writes. Trigger phrases: `approve the exit node`, `check tailscale key expiry`, `add a tailscale policy entry`, `who has this machine shared`, `who can reach the nas on port 445`, `use tailscale`, `run tailscale-pp-cli`."
author: "Cathryn Lavery"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - tailscale-pp-cli
    install:
      - kind: go
        bins: [tailscale-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/cloud/tailscale/cmd/tailscale-pp-cli
---

# Tailscale — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `tailscale-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install tailscale --cli-only
   ```
2. Verify: `tailscale-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/tailscale/cmd/tailscale-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Nearly every Tailscale admin API endpoint (tailnet deletion is deliberately left out), plus commands that make the two dangerous whole-object writes safe: routes approve keeps the routes a device already has, and policy add-entry keeps comments, backs up, validates, diffs, and writes with an ETag check. Those safe-write commands (routes approve and unapprove, shares revoke, policy add-entry and restore) write only with --yes and are plan-only as MCP tools; in --agent mode every other mutation is a dry run unless --yes. devices expiry, routes overview, and shares audit answer fleet questions the admin console shows one machine at a time, and access check and devices inspect give an agent one target's full picture before it acts.

## When to Use This CLI

Use this CLI when an agent or operator needs to administer a tailnet: approve routes and exit nodes, edit the policy file, audit or revoke machine shares, check key expiry, or ask which rules allow a connection. It is the right choice when a change must not clobber existing routes or policy content. The device selector self means this machine and needs the local tailscale command; otherwise pass a hostname, MagicDNS name, Tailscale IP, or nodeId.

## Anti-triggers

Do not use this CLI for:
- Connecting this machine to a tailnet or changing its local settings; use the local tailscale command (tailscale up, tailscale set)
- Sending files with Taildrop or serving a local port; use tailscale file and tailscale serve
- Managing Headscale or other self-hosted control servers

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Safe writes to whole-object endpoints
- **`routes approve`** — Approve a subnet route or exit node on one device without dropping the routes it already has.

  _Reach for this instead of the raw routes endpoint whenever an agent is asked to enable an exit node or subnet route._

  ```bash
  tailscale-pp-cli routes approve self --all-advertised
  ```
- **`routes unapprove`** — Remove approval for one route or the exit-node pair while leaving every other approved route in place.

  _Use it to turn off an exit node without breaking the subnet routes on the same machine._

  ```bash
  tailscale-pp-cli routes unapprove self --exit-node
  ```
- **`policy add-entry`** — Append one nodeAttrs, grants, acls, or ssh entry to the policy file with comments preserved, a local backup, validation, a diff, and a concurrency check.

  _Use it whenever an agent needs to add Taildrive, Funnel, or access grants without rewriting the whole policy file._

  ```bash
  tailscale-pp-cli policy add-entry nodeAttrs --entry '{"target":["autogroup:member"],"attr":["drive:access"]}' --dry-run
  ```
- **`policy restore`** — Roll the policy file back to a local backup after validating it and showing the diff.

  _This is the undo for every policy write the CLI makes._

  ```bash
  tailscale-pp-cli policy restore --list
  ```

### Tailnet state an agent can reason about
- **`routes overview`** — See approved, pending, and stale routes and exit-node state for every device in one table.

  _Run it before approving anything to see what is actually waiting._

  ```bash
  tailscale-pp-cli routes overview --pending
  ```
- **`devices expiry`** — List every device by days until its key expires and flag the ones inside a window.

  _Use it as a weekly check or a CI gate so nobody gets logged out by surprise._

  ```bash
  tailscale-pp-cli devices expiry --within 14
  ```
- **`shares audit`** — List every machine-share invite across the tailnet with who accepted it and which invites are still redeemable.

  _Answer who has access to a shared machine in one call._

  ```bash
  tailscale-pp-cli shares audit --pending
  ```
- **`shares revoke`** — Revoke machine-share invites by ID or by device and acceptor, with a plan before any delete.

  _Use it to revoke a contractor's share invites without hunting IDs device by device, then confirm in the admin console that an accepted share is gone._

  ```bash
  tailscale-pp-cli shares revoke --device self --pending
  ```

### Agent-native networking
- **`access check`** — Show which policy rules let a user reach a device and port, against the live policy or a candidate file.

  _Check access before and after a policy change without guessing at rule evaluation._

  ```bash
  tailscale-pp-cli access check --to self:22
  ```
- **`devices inspect`** — Get one device's identity, routes, key expiry, shares, and recent changes in a single record.

  _Make this the first call in any task about a specific machine._

  ```bash
  tailscale-pp-cli devices inspect self --agent
  ```

## Command Reference

Every Tailscale admin API endpoint except tailnet deletion, grouped by resource. Writes (POST/PUT/PATCH/DELETE) run as dry runs under `--agent` unless `--yes` is passed. Add `--help` to any command for its flags.

**device**

- `tailscale-pp-cli device attributes delete <deviceId> <attributeKey>` (DELETE): Delete a posture attribute from the specified device.
- `tailscale-pp-cli device attributes list <deviceId>` (GET): Retrieve all posture attributes for the specified device.
- `tailscale-pp-cli device attributes set <deviceId> <attributeKey>` (POST): Create or update a custom posture attribute on the specified device.
- `tailscale-pp-cli device authorized set <deviceId>` (POST): This call marks a device as authorized or revokes its authorization for tailnets where device authorization is required.
- `tailscale-pp-cli device delete <deviceId>` (DELETE): Deletes the device from its tailnet.
- `tailscale-pp-cli device device-invites create <deviceId>` (POST): Create new share invites for a device.
- `tailscale-pp-cli device device-invites list <deviceId>` (GET): List all share invites for a device. OAuth Scope: `device_invites:read`.
- `tailscale-pp-cli device expire key <deviceId>` (POST): Mark a device's node key as expired.
- `tailscale-pp-cli device get <deviceId>` (GET): Retrieve the details for the specified device. OAuth Scope: `devices:core:read`.
- `tailscale-pp-cli device ip set <deviceId>` (POST): When a device is added to a tailnet, its Tailscale IPv4 address is set at random either from the CGNAT range.
- `tailscale-pp-cli device key set <deviceId>` (POST): When a device is added to a tailnet, its key expiry is set according to the tailnet's key expiry setting.
- `tailscale-pp-cli device name set <deviceId>` (POST): When a device is added to a tailnet, its Tailscale device name.
- `tailscale-pp-cli device routes list <deviceId>` (GET): Retrieve the list of subnet routes that a device is advertising, as well as those that are enabled for it.
- `tailscale-pp-cli device routes set <deviceId>` (POST): Set a device's enabled subnet routes by replacing the existing list of subnet routes with the supplied parameters.
- `tailscale-pp-cli device tags set <deviceId>` (POST): Tags let you assign an identity to a device that is separate from human users.

**device-invites**

- `tailscale-pp-cli device-invites accept` (POST): Accepts the invitation to share a device into the requesting user's tailnet.
- `tailscale-pp-cli device-invites delete <deviceInviteId>` (DELETE): Delete a specific device invite. OAuth Scope: `device_invites`.
- `tailscale-pp-cli device-invites get <deviceInviteId>` (GET): Retrieve a specific device invite. OAuth Scope: `device_invites:read`.
- `tailscale-pp-cli device-invites resend device-invite <deviceInviteId>` (POST): Resend a device invite by email.

**organizations**

- `tailscale-pp-cli organizations tailnets create <organization>` (POST): Create an API-only tailnet in the organization.
- `tailscale-pp-cli organizations tailnets list <organization>` (GET): List all tailnets in the organization, including the original tailnet and any API-only tailnets.

**posture**

- `tailscale-pp-cli posture delete <id>` (DELETE): Delete a specific posture integration. OAuth Scope: `feature_settings`.
- `tailscale-pp-cli posture get <id>` (GET): Gets the posture integration identified by `{id}`. OAuth Scope: `feature_settings:read`.
- `tailscale-pp-cli posture update <id>` (PATCH): Updates the posture integration identified by `{id}`.

**tailnet**

- `tailscale-pp-cli tailnet acl get` (GET): Retrieves the current policy file for the given tailnet.
- `tailscale-pp-cli tailnet acl preview` (POST): When given a user or IP port to match against, returns the tailnet policy rules that apply to that resource.
- `tailscale-pp-cli tailnet acl set` (POST): Sets the ACL for the given tailnet.
- `tailscale-pp-cli tailnet acl validate` (POST): This endpoint works in one of two modes, neither of which modifies your current tailnet policy file: - Run ACL tests.
- `tailscale-pp-cli tailnet aws-external-id create-or-get` (POST): Get an AWS external id to use for streaming tailnet logs to S3 using role-based authentication.
- `tailscale-pp-cli tailnet aws-external-id validate <id>` (POST): Validate that Tailscale can assume your IAM role with (and only with) this external ID. OAuth Scope: `log_streaming`.
- `tailscale-pp-cli tailnet contacts get` (GET): Retrieve the tailnet's current contacts. OAuth Scope: `account_settings:read`.
- `tailscale-pp-cli tailnet contacts resend-verification-email` (POST): Resends the verification email for this contact, if and only if verification is still pending.
- `tailscale-pp-cli tailnet contacts update` (PATCH): Update the preferences for this type of contact.
- `tailscale-pp-cli tailnet device-attributes batch-update` (PATCH): Batch updates posture attributes across devices in a tailnet.
- `tailscale-pp-cli tailnet devices list` (GET): Lists the devices in a tailnet. OAuth Scope: `devices:core:read`.
- `tailscale-pp-cli tailnet dns get-configuration` (GET): Retrieves the full DNS configuration for a tailnet, including global nameservers, split DNS routes, search paths.
- `tailscale-pp-cli tailnet dns get-preferences` (GET): Retrieves the DNS preferences that are currently set for the given tailnet.
- `tailscale-pp-cli tailnet dns get-split` (GET): Retrieves the split DNS settings, which is a map from domains to lists of nameservers.
- `tailscale-pp-cli tailnet dns list-nameservers` (GET): Lists the global DNS nameservers for a tailnet.
- `tailscale-pp-cli tailnet dns list-search-paths` (GET): Retrieves the list of search paths, also referred to as *search domains*, that is currently set for the given tailnet.
- `tailscale-pp-cli tailnet dns set-configuration` (POST): Replaces the DNS configuration for the given tailnet.
- `tailscale-pp-cli tailnet dns set-nameservers` (POST): Replaces the list of global DNS nameservers for the given tailnet with the list supplied in the request.
- `tailscale-pp-cli tailnet dns set-preferences` (POST): Set the DNS preferences for a tailnet; specifically, the MagicDNS setting.
- `tailscale-pp-cli tailnet dns set-search-paths` (POST): Replaces the list of search paths for the given tailnet.
- `tailscale-pp-cli tailnet dns set-split` (PUT): Replaces the split DNS settings for a given tailnet.
- `tailscale-pp-cli tailnet dns update-split` (PATCH): Performs partial updates of the split DNS settings for a given tailnet.
- `tailscale-pp-cli tailnet keys create` (POST): Creates a new auth key.
- `tailscale-pp-cli tailnet keys delete <keyId>` (DELETE): Deletes a specific api access token or auth key.
- `tailscale-pp-cli tailnet keys get <keyId>` (GET): Returns a JSON object with information about a specific api access token, OAuth client, federated identity, or auth key.
- `tailscale-pp-cli tailnet keys list` (GET): Returns a list of active auth keys, API access tokens and trust credentials.
- `tailscale-pp-cli tailnet keys set <keyId>` (PUT): Set the configuration for an existing OAuth client or federated identity.
- `tailscale-pp-cli tailnet logging disable-log-streaming` (DELETE): Delete the log streaming configuration for the provided log type. OAuth Scope: `log_streaming`.
- `tailscale-pp-cli tailnet logging get-log-streaming-configuration` (GET): Retrieve the log streaming configuration for the provided log type. OAuth Scope: `log_streaming:read`.
- `tailscale-pp-cli tailnet logging get-log-streaming-status` (GET): Retrieve the log streaming status for the provided log type. OAuth Scope: `log_streaming:read`.
- `tailscale-pp-cli tailnet logging list-configuration-audit-logs` (GET): List all configuration audit logs for a tailnet. OAuth Scope: `logs:configuration:read`.
- `tailscale-pp-cli tailnet logging list-network-flow-logs` (GET): List all network flow logs for a tailnet. OAuth Scope: `logs:network:read`.
- `tailscale-pp-cli tailnet logging set-log-streaming-configuration` (PUT): Set the log streaming configuration for the provided log type. OAuth Scope: `log_streaming`.
- `tailscale-pp-cli tailnet oauth-apps create` (POST): Create an OAuth app within a tailnet.
- `tailscale-pp-cli tailnet oauth-apps delete <appId>` (DELETE): Delete a specific OAuth app. OAuth Scope: `oauth_apps`.
- `tailscale-pp-cli tailnet oauth-apps get <appId>` (GET): Retrieve a specific OAuth app. OAuth Scope: `oauth_apps:read`.
- `tailscale-pp-cli tailnet oauth-apps list` (GET): List all OAuth apps for a tailnet. OAuth Scope: `oauth_apps:read`.
- `tailscale-pp-cli tailnet oauth-apps update <appId>` (PUT): Update a specific OAuth app.
- `tailscale-pp-cli tailnet posture create` (POST): Create a posture integration, returning the resulting PostureIntegration.
- `tailscale-pp-cli tailnet posture list` (GET): List all of the posture integrations for a tailnet. OAuth Scope: `feature_settings:read`.
- `tailscale-pp-cli tailnet services delete <serviceName>` (DELETE): Delete the specified Service from the tailnet. OAuth Scope: `services`.
- `tailscale-pp-cli tailnet services get <serviceName>` (GET): Retrieve the details for the specified Service. OAuth Scope: `services:read`.
- `tailscale-pp-cli tailnet services get-device-approval <serviceName> <deviceId>` (GET): Retrieve the approval status of the specified Service on a specific device. OAuth Scope: `services`.
- `tailscale-pp-cli tailnet services list` (GET): List all Services configured for the tailnet.
- `tailscale-pp-cli tailnet services list-hosts <serviceName>` (GET): List all devices that are hosting the specified Service. OAuth Scope: `services`.
- `tailscale-pp-cli tailnet services update <serviceName>` (PUT): Update or create the specified Service.
- `tailscale-pp-cli tailnet services update-device-approval <serviceName> <deviceId>` (POST): Update the approval status of the specified Service on a specific device. OAuth Scope: `services`.
- `tailscale-pp-cli tailnet settings get` (GET): Retrieve the settings for a specific tailnet.
- `tailscale-pp-cli tailnet settings update` (PATCH): Update the settings for a specific tailnet.
- `tailscale-pp-cli tailnet user-invites create` (POST): Create, and optionally email out, new user invites to join the tailnet.
- `tailscale-pp-cli tailnet user-invites list` (GET): List all open (not yet accepted) user invites to the tailnet.
- `tailscale-pp-cli tailnet users list` (GET): List all users of a tailnet. OAuth Scope: `users:read`.
- `tailscale-pp-cli tailnet webhooks create` (POST): Create a webhook within a tailnet. OAuth Scope: `webhooks`.
- `tailscale-pp-cli tailnet webhooks list` (GET): List all webhooks for a tailnet. OAuth Scope: `webhooks:read`.

**user-invites**

- `tailscale-pp-cli user-invites delete <userInviteId>` (DELETE): Deletes a specific user invite.
- `tailscale-pp-cli user-invites get <userInviteId>` (GET): Retrieve a specific user invite.
- `tailscale-pp-cli user-invites resend user-invite <userInviteId>` (POST): Resend a user invite by email.

**users**

- `tailscale-pp-cli users <userId>` (GET): Retrieve details about the specified user. OAuth Scope: `users:read`.
- `tailscale-pp-cli users approve user <userId>` (POST): Approve a pending user's access to the tailnet.
- `tailscale-pp-cli users delete user <userId>` (POST): Delete a user from their tailnet.
- `tailscale-pp-cli users restore user <userId>` (POST): Restores a suspended user's access to their tailnet.
- `tailscale-pp-cli users role set <userId>` (POST): Update the role for the specified user. OAuth Scope: `users`.
- `tailscale-pp-cli users suspend user <userId>` (POST): Suspends a user from their tailnet.

**webhooks**

- `tailscale-pp-cli webhooks delete <endpointId>` (DELETE): Delete a specific webhook. OAuth Scope: `webhooks`.
- `tailscale-pp-cli webhooks get <endpointId>` (GET): Retrieve a specific webhook. OAuth Scope: `webhooks:read`.
- `tailscale-pp-cli webhooks rotate webhook-secret <endpointId>` (POST): Rotate and generate a new secret for a specific webhook.
- `tailscale-pp-cli webhooks test webhook <endpointId>` (POST): Test a specific webhook by sending out a test event to the endpoint URL.
- `tailscale-pp-cli webhooks update <endpointId>` (PATCH): Update a specific webhook. OAuth Scope: `webhooks`.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
tailscale-pp-cli which "approve an exit node"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

## Recipes

### Turn a Mac into an exit node safely

```bash
tailscale-pp-cli routes approve home-mac --exit-node --dry-run
```

Prints the before and after enabled routes; rerun with --yes to apply.

### Give members Taildrive access

```bash
tailscale-pp-cli policy add-entry nodeAttrs --entry '{"target":["autogroup:member"],"attr":["drive:access"]}' --dry-run
```

Shows the diff and validation result; rerun with --yes to write with the ETag check.

### Expiry report for agents

```bash
tailscale-pp-cli devices expiry --within 30 --agent --select items.machine,items.days_left,items.flagged
```

Narrows the report to the three fields an agent needs.

### Who can reach the NAS over SMB

```bash
tailscale-pp-cli access check --to nas:445
```

Lists the policy rules that grant access to that device and port.

### Roll back the last policy change

```bash
tailscale-pp-cli policy restore latest
```

Shows the diff and validation for restoring the newest local backup; add --yes to restore it, or use --list to pick an older one.

## Auth Setup

Create an API access token on the admin console Keys page (https://login.tailscale.com/admin/settings/keys) and export TAILSCALE_API_KEY, or set TAILSCALE_OAUTH_CLIENT_ID and TAILSCALE_OAUTH_CLIENT_SECRET to use a scoped OAuth client; the CLI exchanges the client for a short-lived token. The tailnet defaults to '-', meaning the token's own tailnet. Set TAILSCALE_TAILNET only to target a different one.

Run `tailscale-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color`.

Global format flags share one contract on promoted, novel, sync, and `--deliver` paths:

- `--json` — one JSON document on stdout (sync progress events go to stderr)
- `--compact` — keep identity/status/timestamp fields; does not change the document vs stream shape
- `--csv` / `--plain` — tabular rows (collection envelopes unwrap to the row array)
- `--quiet` — one identity value per row, no envelope

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  tailscale-pp-cli device get n1234567890CNTRL --agent --select addresses,advertisedRoutes,authorized
  ```
- **Previewable** — `--dry-run` shows the request without sending; the safe-write commands still read live state and print a plan
- **Offline search** — `search` reads only the local SQLite store filled by `sync` (Tailscale has no search endpoint); run `sync` first
- **Non-interactive** — never prompts, every input is a flag
- **Explicit confirmation** — `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Safe writes** — routes approve/unapprove, shares revoke, and policy add-entry/restore print a plan from live reads and write only with `--yes`, in every mode; as MCP tools they are plan-only. The MCP `tailscale_execute` tool returns the planned request for any write endpoint (policy validate and preview excepted) unless it is called with `confirm: true`. Under `--agent`, every other mutating command runs as a dry run unless `--yes` is passed
- **`self`** — the device selector `self` means this machine and needs the local `tailscale` command; otherwise pass a hostname, MagicDNS name, Tailscale IP, or nodeId
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and use `--ignore-missing` only when a missing delete target should count as success

### Response envelope

The local store is kept per credential: each API key or OAuth client gets its own database file under the data directory, so two tailnets never share synced data, search results, or learnings.

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

With `--agent`, reads are wrapped this way. With `--json` alone, the hand-written commands (routes overview, devices expiry, shares audit, and the rest) print their object directly, usually an `items` array plus counts. Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `TAILSCALE_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `TAILSCALE_CONFIG_DIR`, `TAILSCALE_DATA_DIR`, `TAILSCALE_STATE_DIR`, `TAILSCALE_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `TAILSCALE_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `tailscale-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "tailscale": {
        "command": "tailscale-pp-mcp",
        "env": {
          "TAILSCALE_HOME": "/srv/tailscale"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `TAILSCALE_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `TAILSCALE_HOME`, or `doctor` will not find credentials left under the former root.

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
tailscale-pp-cli recall "$QUERY" --agent
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
      "next_action": ["<trial command>", "tailscale-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `tailscale-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `tailscale-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "machines" teach satisfying a "devices" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "shares" resolving to both device invites and user invites). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `tailscale-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately. Pass the query the same way as recall — argv/MCP, or file-then-`$QUERY`. Do not splice the question into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
tailscale-pp-cli teach --query "$QUERY" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "acls") satisfies future queries under another alias (e.g., "policy file", "access controls") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
QUERY=$(cat /path/to/question.txt)
tailscale-pp-cli teach \
  --query "$QUERY" \
  --resource-type devices \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
QUERY=$(cat /path/to/question.txt)
tailscale-pp-cli teach-playbook \
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
tailscale-pp-cli playbook amend \
  --query "$QUERY" \
  --add-note "$NOTE"
# (append shell `&` to background it)
```

What counts as worth amending: a behavior you OBSERVED this session that future-you would benefit from knowing. Examples worth amending:

- A workaround for a CLI surface that silently drops or misorders a flag.
- An undocumented endpoint shape (response wrapped in `{meta, results}`, payload nested two levels deeper than the docs claim).
- Observed schema drift (a field renamed, an index that shifted between API versions, a category label that the API now returns lower-cased).

What does NOT belong in notes:

- The year-specific or entity-specific answer to the user's question. That's the response, not a learning.
- Per-device / per-user / per-row data the playbook already retrieves at runtime.
- Statements that paraphrase what the existing notes already say.

The amend command appends to the family's existing notes with a timestamped marker (`[amend YYYY-MM-DDTHH:MMZ]: <text>`). Multiple amends accumulate; the audit trail is visible. If no playbook exists yet for the family, amend creates a notes-only one (so cold-start corrections still land).

#### PII discipline for amend notes

`playbook amend` notes are designed to potentially flow upstream as shared knowledge in future versions of the Printing Press. Keep them clean of user-identifying content so the upstream-contribution path stays open without retroactive scrubbing:

- **Do NOT embed** paths to user filesystems, personal API keys or tokens, user email addresses, user GitHub handles, or specific query histories tied to a single user.
- **Acceptable**: endpoint shapes, undocumented field names, API gotchas, observed schema drift, workarounds for CLI surfaces, generalizable pagination or retry tactics.

If a correction is only meaningful with user-specific context, it belongs in a personal note, not in the playbook amend.

### Measuring the loop

`tailscale-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `TAILSCALE_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
tailscale-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
tailscale-pp-cli feedback --stdin < notes.txt
tailscale-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `TAILSCALE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TAILSCALE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename). Binary-response commands write decoded payload bytes (not the base64 JSON envelope) and print a small JSON receipt on stdout; `--json`/`--csv` do not refuse when this sink is set. |
| `webhook:<url>` | POST the output body to the URL (`application/json`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
tailscale-pp-cli profile save briefing --json
tailscale-pp-cli --profile briefing device get n1234567890CNTRL
tailscale-pp-cli profile list --json
tailscale-pp-cli profile show briefing
tailscale-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error; also `devices expiry --fail-on-flagged` when an item is inside the window |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 6 | Partial failure |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `tailscale-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/cloud/tailscale/cmd/tailscale-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add tailscale-pp-mcp -- tailscale-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which tailscale-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   tailscale-pp-cli <command> [subcommand] [args] --agent
   # e.g.
   tailscale-pp-cli routes overview --pending --agent
   ```
4. If ambiguous, drill into subcommand help: `tailscale-pp-cli <command> --help`.
