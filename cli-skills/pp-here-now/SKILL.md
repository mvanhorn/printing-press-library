---
name: pp-here-now
description: "Publish a folder to a live URL in one command, mirror your Drives offline Trigger phrases: `publish this folder to here.now`, `deploy my site to here.now`, `sync this folder to my here.now drive`, `what here.now sites are about to expire`, `how close am I to the here.now free plan limit`, `use here-now`, `run here-now-pp-cli`."
author: "Trevin Chow"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - here-now-pp-cli
    install:
      - kind: go
        bins: [here-now-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/cloud/here-now/cmd/here-now-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/cloud/here-now/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# here.now — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `here-now-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install here-now --cli-only
   ```
2. Verify: `here-now-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/here-now/cmd/here-now-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Reach for this CLI whenever an agent needs to turn generated content into a live URL, manage private Drive files, or read back Site Data form submissions. It is the right tool for free-tier-heavy workflows: anonymous-publish-and-claim, drives sync, and staying under plan limits without paid analytics. Prefer it over raw API calls when publishing a directory, syncing files, or reading a site's Site Data records.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Anonymous publish lifecycle
- **`claims`** — Every anonymous publish records its slug, claim token, and 24h expiry into a local vault, so you can make an expiring site permanent later without hunting for the token in terminal scrollback.

  _Reach for this when an agent publishes anonymously and needs to keep the site — the token to do so exists nowhere else._

  ```bash
  here-now-pp-cli claims --json
  ```
- **`claims expiring`** — Lists anonymous sites expiring within a time window so you can claim them before they vanish.

  _Run this before anonymous sites silently expire at 24h to catch the ones worth keeping._

  ```bash
  here-now-pp-cli claims expiring --within 6h --json
  ```
- **`publish resume`** — Detects a locally-recorded publish that uploaded its files but never finalized, and completes it instead of re-publishing from scratch.

  _Use this when a publish half-failed (uploads done, finalize missed) to finish it without burning publish-rate budget on a redo._

  ```bash
  here-now-pp-cli publish resume my-site --dry-run
  ```

### Drive as a filesystem
- **`drives sync`** — Rsync-style sync of a local directory to a Drive: compares local sha256 against the synced file checksums and uploads only what changed, new, or deleted.

  _Reach for this instead of re-uploading a whole folder — it only sends drift, saving rate budget and time._

  ```bash
  here-now-pp-cli drives sync ./assets --drive drv_example --dry-run
  ```
- **`drives diff`** — Shows which local files differ from a Drive (added, changed, deleted) without uploading anything.

  _Use this to preview exactly what a sync would change before committing to the upload._

  ```bash
  here-now-pp-cli drives diff ./assets --drive drv_example --json
  ```

### Free-plan health
- **`usage`** — Local rollup of site count, drive bytes, and recent publish cadence against the free-plan ceilings (500 sites, 10GB, 60 publishes/hour).

  _Use this to stay under the free-plan caps without paying for analytics; it's the free user's only proactive limit signal._

  ```bash
  here-now-pp-cli usage --json
  ```
- **`sites stale`** — Lists sites not updated in N days from the local mirror, so you can reclaim free-plan slots by deleting dead ones.

  _Run this near the 500-site free cap to find which sites to prune._

  ```bash
  here-now-pp-cli sites stale --days 30 --json
  ```

## Command Reference

**domains** — Custom domains, subdomain handles, and links.

- `here-now-pp-cli domains create` — Add a custom domain
- `here-now-pp-cli domains delete` — Remove a custom domain
- `here-now-pp-cli domains get` — Get custom domain status
- `here-now-pp-cli domains list` — List custom domains

**drives** — Private cloud storage for agent files.

- `here-now-pp-cli drives create` — Create a Drive
- `here-now-pp-cli drives delete` — Soft-delete a Drive
- `here-now-pp-cli drives get` — Get Drive details
- `here-now-pp-cli drives get-default` — Get or create the default Drive
- `here-now-pp-cli drives list` — List account Drives
- `here-now-pp-cli drives patch` — Patch Drive metadata

**handle** — Manage handle

- `here-now-pp-cli handle create` — Create account subdomain handle
- `here-now-pp-cli handle delete` — Delete account subdomain handle
- `here-now-pp-cli handle get` — Get account subdomain handle
- `here-now-pp-cli handle update` — Update account subdomain handle

**here-now-analytics** — Manage here.now analytics

- `here-now-pp-cli here-now-analytics` — Returns aggregate analytics across all Sites owned by the authenticated paid account.

**here-now-auth** — Manage here.now auth

- `here-now-pp-cli here-now-auth request-agent-code` — Starts the agent-assisted API key flow by emailing a one-time code to the user.
- `here-now-pp-cli here-now-auth verify-agent-code` — Completes agent-assisted sign-in. If the email is new, the account is created.

**links** — Manage links

- `here-now-pp-cli links create` — Create a link from a subdomain handle/domain path to a Site
- `here-now-pp-cli links delete` — Delete a link
- `here-now-pp-cli links get` — Get a link
- `here-now-pp-cli links list` — List subdomain handle or domain links
- `here-now-pp-cli links update` — Update a link

**me** — Manage me

- `here-now-pp-cli me delete-variable` — Delete a service variable
- `here-now-pp-cli me list-variables` — List service variables
- `here-now-pp-cli me set-variable` — Create or update a service variable

**profile_resource** — Manage profile resource

- `here-now-pp-cli profile-resource add-profile-site` — Shows an active owned Site on the authenticated user's public profile.
- `here-now-pp-cli profile-resource get-profile` — Returns the authenticated user's public profile settings and Sites shown on the profile.
- `here-now-pp-cli profile-resource list-profile-sites` — Lists the authenticated user's Sites currently shown on their public profile.
- `here-now-pp-cli profile-resource patch-profile` — Turns the public profile on or off and controls whether future Sites are added to the profile automatically.
- `here-now-pp-cli profile-resource patch-profile-username` — Changes the authenticated user's profile username and profile URL.
- `here-now-pp-cli profile-resource remove-profile-site` — Removes a Site from the authenticated user's public profile without deleting the Site.

**publish** — Manage publish

- `here-now-pp-cli publish create-site` — Creates a pending Site version and returns presigned upload URLs.
- `here-now-pp-cli publish delete-site` — Delete a Site
- `here-now-pp-cli publish from-drive` — Publish a Drive version as a Site
- `here-now-pp-cli publish get-site` — Get Site details
- `here-now-pp-cli publish update-site` — Creates a pending replacement version for an existing Site. Authenticated Sites require API key ownership.

**publishes** — Manage publishes

- `here-now-pp-cli publishes list-sites` — List account Sites
- `here-now-pp-cli publishes search-sites` — Searches the authenticated user's active owned Sites by slug, URL/domain, viewer metadata, file path

**support** — Authenticated support requests.

- `here-now-pp-cli support` — Send an authenticated support request


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
here-now-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Publish a folder and keep it permanently

```bash
here-now-pp-cli publish dir ./site --anon && here-now-pp-cli claims expiring --within 24h
```

Publish anonymously for a live URL, then see the claim window so you can `sites claim` before it expires.

### Sync a working folder to your Drive

```bash
here-now-pp-cli drives sync ./project --drive drv_example
```

Uploads only files that changed since the last sync, computed from local sha256 vs the mirror.

### Read form submissions from a site's collection

```bash
here-now-pp-cli publishes data list-site-records my-site signups --json --select id,data,createdAt
```

List a site's Site Data records with field selection so the agent's context stays small on deeply nested records.

### Find sites to prune before hitting the free cap

```bash
here-now-pp-cli sites stale --days 30 --json --select slug,url,updatedAt
```

Lists the least-recently-updated sites so you can reclaim free-plan slots.

### Check free-plan headroom

```bash
here-now-pp-cli usage --json
```

Local rollup of site count, drive bytes, and publish cadence against free-tier limits — no paid analytics required.

## Auth Setup

Most onboarding is key-less: anonymous publish, public site reads, and Site Data writes work with no credential at all. When you want permanent sites and your own Drives, run `here-now-pp-cli auth login` to do the email-code flow (a one-time code is emailed, you paste it back) and the API key is stored locally, or set HERENOW_API_KEY in your environment. Drive share tokens also authenticate Drive-scoped reads.

Run `here-now-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  here-now-pp-cli domains list --agent --select id,name,status
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
here-now-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
here-now-pp-cli feedback --stdin < notes.txt
here-now-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/here-now-pp-cli/feedback.jsonl`. They are never POSTed unless `HERE_NOW_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `HERE_NOW_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
here-now-pp-cli profile save briefing --json
here-now-pp-cli --profile briefing domains list
here-now-pp-cli profile list --json
here-now-pp-cli profile show briefing
here-now-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `here-now-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/cloud/here-now/cmd/here-now-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add here-now-pp-mcp -- here-now-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which here-now-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   here-now-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `here-now-pp-cli <command> --help`.
