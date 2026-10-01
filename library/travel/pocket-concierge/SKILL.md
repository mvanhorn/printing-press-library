---
name: pp-pocket-concierge
description: Discover Pocket Concierge restaurants, read courses and explicit policies, inspect date/party availability, or return canonical restaurant booking links. Use for Pocket Concierge dining searches, courses, calendars, sessions, or booking handoff.
---

# Pocket Concierge

## When to use

Use this CLI for public Pocket Concierge restaurant/course discovery and date/party availability in Japan. Use only the named provider. Do not use for reservations, payments, account changes, authentication, hidden member inventory, or another booking provider.

## Install

Build from the project or local library directory:

```sh
go build -o bin/pocket-concierge-pp-cli ./cmd/pocket-concierge-pp-cli
```

No paid key, account, browser or SQLite runtime. Use the built binary directly; do not modify global agent configuration.

## Distribution status

The catalog installer becomes available after this CLI is merged and indexed in the public library. Until then, build from this source checkout.

## Prerequisites: Install the CLI

This skill drives the `pocket-concierge-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install pocket-concierge --cli-only
   ```
2. Verify: `pocket-concierge-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/pocket-concierge/cmd/pocket-concierge-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## Workflow

1. Run `pocket-concierge-pp-cli filters` to resolve first-party area/cuisine IDs.
2. Run `pocket-concierge-pp-cli restaurants search --query Murase --limit 5` for bounded summaries.
3. Run `pocket-concierge-pp-cli restaurants get --id 245672` and `pocket-concierge-pp-cli courses list --id 245672` to inspect policies, source conditions and prices.
4. Run `pocket-concierge-pp-cli availability dates --id 245672 --limit 10`, then use a returned current date with `pocket-concierge-pp-cli availability slots --id 245672 --date 2026-10-05 --party 2`.
5. Run `pocket-concierge-pp-cli booking handoff --id 245672 --course-id 182402` to return the canonical page. With a session, pass its source ID and date. The user completes booking on the provider website.

Example dates/IDs may become unavailable. Never invent session/course identifiers.

## Recipes

```sh
pocket-concierge-pp-cli restaurants search --query sushi --limit 5 --agent --select items.id,items.name,items.name_ja,items.url
pocket-concierge-pp-cli restaurants search --area-id 19 --cuisine-id 1 --service DINNER --max-price 30000 --limit 5
pocket-concierge-pp-cli courses list --id 245672 --select items.id,items.price,items.fee_statements,restaurant_fee_statements
pocket-concierge-pp-cli availability slots --id 245672 --date 2026-10-05 --party 2 --refresh
pocket-concierge-pp-cli booking handoff --id 245672 --course-id 182402
```

## Output and decision rules

Compact JSON is default; diagnostics/errors are stderr. Projection retains `meta`. Preserve numeric-string source IDs, Japanese names, canonical URLs, null unknowns, JPY units, timestamps, and partial coverage.

`instant_confirmation`, `reservation_request` and `waitlist` are distinct options, never confirmed reservations. Waitlist session ID is null. Calendars do not establish party suitability. A handoff is not a prepopulated or reserved booking.

Course guest/group prices are separate. `all_in_total` stays null. Retain both course and restaurant fee statements when they disagree. Dietary, child and language evidence is verbatim and never a guarantee; English page language does not imply English staff.

Source reads are serial and bounded. Date/party inventory is live by default; details/search cache 15m, filters 24h. `--refresh` explicitly refreshes queried entries; `--no-cache` bypasses all cache IO. Optional `--cache-availability` permits 30s, visible in `meta.observations`. Follow `pagination.next_page` or `next_offset` explicitly. `meta.partial` marks pagination or uncertain bounds.

## Errors and health

`pocket-concierge-pp-cli doctor --refresh` checks public JSON access; `--dry-run` performs no IO. `schema` describes fields and exit codes; `agent-context` explains task routing.

Codes: 2 usage; 3 public identity/session absent; 4 access denied; 5 rate limited; 6 network/timeout; 7 provider HTTP/GraphQL/schema failure. Empty successful arrays are not failures. On source errors, report the failure instead of claiming no availability. Retry rate limits later; refresh disappearing sessions. Do not substitute fixtures or another provider.
