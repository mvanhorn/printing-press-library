---
name: pp-uspto-tsdr
description: "Printing Press CLI for Uspto Tsdr. Beginning on October 2, 2020, you will need an API key to access the TSDR REST API See..."
author: "H179922"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - uspto-tsdr-pp-cli
---

# Uspto Tsdr — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `uspto-tsdr-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install uspto-tsdr --cli-only
   ```
2. Verify: `uspto-tsdr-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Beginning on October 2, 2020, you will need an API key to access the TSDR REST API See https://account.uspto.gov/api-manager and the uspto's TSDR Data API webpage for more information on retrieving bulk data.

 Click on the Authorize box and enter your api key.  It is required and will be sent on all requests.

 This uses the uspto's swagger object with a number of changes.  The uspto's api does not allow browser request (CORS issues) so requests from this page will not actually work.  The generated curl commands will work and the modified swagger object can be imported into postman.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Trademark intelligence
- **`trademark status`** — Full current state of a trademark in one command — mark text, status, owner, classes, filing/registration dates, attorney, and prosecution event count

  _Agents evaluating trademark status need the complete picture in one call instead of parsing XML manually_

  ```bash
  uspto-tsdr-pp-cli trademark status 97123456 --json
  ```
- **`trademark timeline`** — Every prosecution event in chronological order — office actions, examiner reviews, publication events, and registration milestones

  _Trademark attorneys need the full event history to evaluate prosecution strength and identify potential issues_

  ```bash
  uspto-tsdr-pp-cli trademark timeline 97123456 --json
  ```
- **`trademark docs`** — List all documents in the prosecution file — office actions, responses, specimens, registration certificates — with type and date filtering

  _Litigation prep and due diligence require reviewing every document in a trademark file without clicking through the TSDR web UI_

  ```bash
  uspto-tsdr-pp-cli trademark docs 97123456 --filter-type SPE --json
  ```

### Portfolio management
- **`trademark deadlines`** — Calculate Section 8, 9, and 15 maintenance deadlines with window-open dates and days-away countdown

  _Missing a maintenance deadline means losing the registration — this is the #1 pain point for trademark portfolio managers_

  ```bash
  uspto-tsdr-pp-cli trademark deadlines 97123456 --json
  ```
- **`trademark watch`** — Monitor multiple trademarks for status changes — caches previous statuses locally and flags any changes since last check

  _Agents monitoring trademark portfolios need change detection, not full status dumps they have to diff themselves_

  ```bash
  uspto-tsdr-pp-cli trademark watch 97123456 97654321 --json
  ```
- **`trademark batch`** — Batch status lookup for multiple trademarks using the multi-case endpoint or individual fallback with rate-limit throttling

  _IP paralegals managing hundreds of marks need batch status without manually checking each one_

  ```bash
  uspto-tsdr-pp-cli trademark batch 97123456 97654321 --json
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**case-multi-status** — Manage case multi status

- `uspto-tsdr-pp-cli case-multi-status` — Parameters can be one of the following: rnXXXXXXX for US registration number, snXXXXXXXX for US serial number,...

**casedoc** — Manage casedoc


**casedocs** — Manage casedocs

- `uspto-tsdr-pp-cli casedocs get-bundle-info-pdf` — Digits can be entered in one of the first four parameters. rnXXXXXXX for US registration number, snXXXXXXXX for US...
- `uspto-tsdr-pp-cli casedocs get-bundle-info-xml` — Digits can be entered in one of the first four parameters. rnXXXXXXX for US registration number, snXXXXXXXX for US...
- `uspto-tsdr-pp-cli casedocs get-bundle-info-zip` — Parameters can be one of the following: rnXXXXXXX for US registration number, snXXXXXXXX for US serial number,...

**casestatus** — Manage casestatus


**raw-image** — Manage raw image

- `uspto-tsdr-pp-cli raw-image <serial_number>` — Parameter is the digits only of the serial number, no leading sn. Example:...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
uspto-tsdr-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup
Set your API key via environment variable:

```bash
export TSDR_APIKEY_HEADER="<your-key>"
```

Or persist it in `~/.config/tsdr-pp-cli/config.toml`.

Run `uspto-tsdr-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  uspto-tsdr-pp-cli case-multi-status --ids example-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

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
uspto-tsdr-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
uspto-tsdr-pp-cli feedback --stdin < notes.txt
uspto-tsdr-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.uspto-tsdr-pp-cli/feedback.jsonl`. They are never POSTed unless `USPTO_TSDR_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `USPTO_TSDR_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
uspto-tsdr-pp-cli profile save briefing --json
uspto-tsdr-pp-cli --profile briefing case-multi-status --ids example-value
uspto-tsdr-pp-cli profile list --json
uspto-tsdr-pp-cli profile show briefing
uspto-tsdr-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `uspto-tsdr-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add uspto-tsdr-pp-mcp -- uspto-tsdr-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which uspto-tsdr-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   uspto-tsdr-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `uspto-tsdr-pp-cli <command> --help`.
