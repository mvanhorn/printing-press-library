---
name: pp-american-express-offers
description: "Printing Press CLI for American Express Offers. List card-linked offers and savings on your American Express card"
author: "user"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - american-express-offers-pp-cli
---

# American Express Offers — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `american-express-offers-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install american-express-offers --cli-only
   ```
2. Verify: `american-express-offers-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

List card-linked offers and savings on your American Express card

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`analytics --type offers --group-by category`** — Group your synced card-linked offers by category to see at a glance where the bulk of your enrollable rewards live (Dining, Travel, Shopping, etc.).

  _Lets an agent answer 'which category has the most offers I haven't enrolled in yet' in one call instead of pulling and tallying the live list itself._

  ```bash
  american-express-offers-pp-cli analytics --type offers --group-by category --agent
  ```
- **`workflow archive`** — Snapshot every available and enrolled offer into the local SQLite store in one command, so subsequent reads and analytics work offline and at zero rate-limit cost.

  _Lets an agent run a tight loop of `--data-source local` queries against the offer set without repeatedly hitting the live AMEX hub._

  ```bash
  american-express-offers-pp-cli workflow archive
  ```

### Realtime monitoring
- **`tail --interval 60s`** — Poll the offers hub at a chosen interval and emit NDJSON change events when an offer is added, expires, or flips between available and enrolled.

  _Gives an agent a streaming hook to react to new statement-credit offers without having to schedule its own diffing job._

  ```bash
  american-express-offers-pp-cli tail --interval 60s | jq 'select(.event == "change")'
  ```

## HTTP Transport

This CLI uses standard HTTP transport with HTTP/2 disabled for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 543 API entries from 2241 total network entries
- Protocols: rest_json (75% confidence)
- Generation hints: browser_http_transport, requires_protected_client, weak_schema_confidence
- Candidate command ideas: create_FAM2kB — Derived from observed POST /CDH_JN/e/b/HUzGnmcaKw2T/h7YE6GDrirh0kmYG/Xmlu/SQlrIE/FAM2kB traffic.; create_Targeting.php — Derived from observed POST /{id}/Targeting.php traffic.; create_beacon — Derived from observed POST /beacon traffic.; create_errors — Derived from observed POST /_/report/errors traffic.; create_events — Derived from observed POST /personalization/v2/customers/treatments/events traffic.; create_inquiry_results — Derived from observed POST /servicing/v1/contact_management/chats/inquiry_results traffic.; get_Asset.php — Derived from observed GET /{id}/Asset.php traffic.; list_axp_personalized_marketing.json — Derived from observed GET /cdaas/one-app/modules/axp-personalized-marketing/1.28.0/en-us/axp-personalized-marketing.json traffic.
- Caveats: empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.

## Command Reference

**offers** — Card-linked offers on your American Express card

- `american-express-offers-pp-cli offers get-savings` — Show total amount saved via enrolled card offers
- `american-express-offers-pp-cli offers list-offers` — List available and enrolled card offers


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
american-express-offers-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

This CLI uses a browser session. Log in to [americanexpress.com](https://www.americanexpress.com) in Chrome, then:

```bash
american-express-offers-pp-cli auth login --chrome
```

Requires a cookie extraction tool (`pycookiecheat` via pip, or `cookies` via Homebrew).

Run `american-express-offers-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  american-express-offers-pp-cli offers get-savings --account-number-proxy example-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

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
american-express-offers-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
american-express-offers-pp-cli feedback --stdin < notes.txt
american-express-offers-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.american-express-offers-pp-cli/feedback.jsonl`. They are never POSTed unless `AMERICAN_EXPRESS_OFFERS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `AMERICAN_EXPRESS_OFFERS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
american-express-offers-pp-cli profile save briefing --json
american-express-offers-pp-cli --profile briefing offers get-savings --account-number-proxy example-value
american-express-offers-pp-cli profile list --json
american-express-offers-pp-cli profile show briefing
american-express-offers-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `american-express-offers-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add american-express-offers-pp-mcp -- american-express-offers-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which american-express-offers-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   american-express-offers-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `american-express-offers-pp-cli <command> --help`.
