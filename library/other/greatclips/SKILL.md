---
name: pp-greatclips
description: "First CLI for Great Clips Online Check-In: every endpoint shape and host route, ready for real calls once cookies... Trigger phrases: `great clips wait time`, `check me into great clips`, `how long is the wait at great clips`, `add me and my kids to the great clips wait list`, `great clips request shape`, `use greatclips`, `run greatclips`."
author: "Great Clips"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - greatclips-pp-cli
---

# Great Clips — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `greatclips-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install greatclips --cli-only
   ```
2. Verify: `greatclips-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/greatclips/cmd/greatclips-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Drives the Great Clips Online Check-In API (webservices.greatclips.com) and the ICS Net Check-In wait/check-in service (www.stylewaretouch.net) from one Go binary. v0.1 ships every endpoint with --dry-run and --json so an agent can build flows on top; live calls require pasting cookies from a logged-in Chrome session (documented v0.2 gap).

## When to Use This CLI

Use this CLI to send the exact HTTP requests the Great Clips Online Check-In SPA sends, from the command line, with dry-run and --json output. v0.1 ships every endpoint's request shape so an agent can build flows on top; real calls require manual cookie capture from a logged-in Chrome session.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 9 API entries from 22 total network entries
- Protocols: (0% confidence), (0% confidence), (0% confidence)
- Generation hints: Two-host single-auth: emit one bearer-token client that handles both webservices.greatclips.com and www.stylewaretouch.net., POST /api/store/waitTime takes a JSON array body of {storeNumber} objects, not a single object. Single-salon call sends an array of length 1., Auth captured live via Claude-in-Chrome MCP from a logged-in Chrome session; the printed CLI should ship a `auth login --chrome` helper command for the equivalent flow.

## Command Reference

**cancel** — Cancel your active check-in

- `greatclips-pp-cli cancel` — Cancel the active check-in for this account

**checkin** — Submit a check-in for yourself plus a party (1-5 people)

- `greatclips-pp-cli checkin` — Add yourself and optionally other party members to a salon waitlist

**customer** — Read your Great Clips customer profile (name, phone, favorites, recent visits)

- `greatclips-pp-cli customer` — Get the authenticated customer's profile

**geo** — Resolve a zip code or city term to latitude/longitude

- `greatclips-pp-cli geo` — Resolve a zip/postal code to lat/lng/city/state

**hours** — Read salon hours (today plus 14-day forecast with special hours)

- `greatclips-pp-cli hours` — Get 14-day hours forecast for one salon

**salons** — Search and look up Great Clips salons

- `greatclips-pp-cli salons get` — Get a single salon by its salon number
- `greatclips-pp-cli salons search` — Search salons by zip code, city, or coordinates within a radius

**status** — Check your current position in line for an active check-in

- `greatclips-pp-cli status` — Get your active check-in status (position in line, estimated wait)

**wait** — Read estimated wait times from the ICS Net Check-In service

- `greatclips-pp-cli wait` — Get wait time for one salon (body is a single-element array of {storeNumber})


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
greatclips-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Party-of-four check-in request body

```bash
greatclips-pp-cli checkin --first-name Matt --last-name VanHorn --phone-number '(206) 555-0100' --salon-number 8991 --guests 4 --dry-run --json
```

Emits the exact JSON body POSTed to www.stylewaretouch.net/api/customer/checkIn for a party of four at Island Square.

### Wait-time request for a salon

```bash
greatclips-pp-cli wait --store-number 8991 --dry-run --json
```

Request shape for the ICS Net Check-In waitTime endpoint.

### Salons near a zip

```bash
greatclips-pp-cli salons search --term 98040 --radius 5 --dry-run --json
```

Request shape for the salon-search/term endpoint with a 5-mile radius.

### 14-day hours forecast

```bash
greatclips-pp-cli hours --salon 8991 --dry-run
```

Calls salon-hours/upcoming for one salon; the response includes special-hours and holiday closures.

### Geocode a postal code

```bash
greatclips-pp-cli geo --query 98040 --dry-run --json --select results.city,results.state,results.lat,results.lng
```

Resolves a zip code to lat/lng/city/state via the geo-names endpoint.

## Auth Setup

Great Clips uses HttpOnly session cookies from an Auth0 tenant at cid.greatclips.com. Browser-sniff confirmed all calls succeed via cookies attached by the browser, not via an Authorization header. v0.1 of this CLI ships the request shape and host routing; to make real calls, paste the cookies from a logged-in Chrome session into ~/.config/greatclips-pp-cli/config.toml. Set GREATCLIPS_TOKEN to any placeholder so doctor passes (the env var is kept for v0.2 when bearer support might land).

Run `greatclips-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  greatclips-pp-cli salons get --num example-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
greatclips-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
greatclips-pp-cli feedback --stdin < notes.txt
greatclips-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.greatclips-pp-cli/feedback.jsonl`. They are never POSTed unless `GREATCLIPS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `GREATCLIPS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
greatclips-pp-cli profile save briefing --json
greatclips-pp-cli --profile briefing salons get --num example-value
greatclips-pp-cli profile list --json
greatclips-pp-cli profile show briefing
greatclips-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `greatclips-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add greatclips-pp-mcp -- greatclips-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which greatclips-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   greatclips-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `greatclips-pp-cli <command> --help`.
