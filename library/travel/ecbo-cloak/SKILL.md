---
name: pp-ecbo-cloak
description: Find ecbo cloak luggage storage in Japan, inspect facility hours/restrictions, or check a read-only quote and validation for exact dates, times and bag counts. Use for ecbo cloak nearby search, overnight storage, luggage-storage offers, or run ecbo-cloak-pp-cli.
---

# ecbo cloak luggage storage

Use `ecbo-cloak-pp-cli`. This provider-only CLI reads public first-party ecbo data without an account or paid key. Booking, payment and account operations are outside its scope. Use another tool for other providers, nationwide exhaustive inventory, or free-text geocoding.

## Prerequisites: Install the CLI

This skill drives the `ecbo-cloak-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install ecbo-cloak --cli-only
   ```
2. Verify: `ecbo-cloak-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/ecbo-cloak/cmd/ecbo-cloak-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## Workflow

1. Obtain Japan coordinates from the itinerary/user. Discover a bounded nearest window:
   `ecbo-cloak-pp-cli facilities near --lat 35.6812 --lon 139.7671 --limit 5 --agent --select id,name,name_ja,booking_url`
2. Fetch the shortlisted facility lazily:
   `ecbo-cloak-pp-cli facilities get GBy4uBrI --agent`
   Read facility restrictions, overnight flags and hours; independent acceptance/pickup cutoffs may be null.
3. Inspect an actual future interval in Asia/Tokyo and explicit small/large piece counts:
   `ecbo-cloak-pp-cli offer inspect 0c3fb1e9-ad5a-42de-bfda-3027ebe4921e --from 2026-10-03T19:00 --to 2026-10-03T21:00 --small 1 --large 1 --agent --select quote,validation,availability,booking_url`
   Completion means a source quote and validation observation, or an explicit source/access failure. Return the canonical booking link for human handoff.

## Interpretation

Distinguish listed facility, filtered search match and exact source validation. Listed maximum counts and raw availability ratios never establish remaining capacity. Validation holds no slots; remaining count remains null. Report observation timestamp and cache status. Quotes always fetch live; cached detail lasts one hour and nearby discovery five minutes. Use `--refresh` for new detail/discovery observation. At exactly 45cm the provider's language variants disagree; preserve the uncertainty and check the facility page. Each piece usually counts separately; special station rules can require multiple slots for long items.

## Local inventory

`ecbo-cloak-pp-cli inventory refresh --lat 35.6812 --lon 139.7671` explicitly saves one nearest window, up to 50 records.
`ecbo-cloak-pp-cli inventory list --query 東京 --limit 5 --agent` reads that snapshot offline.
Name queries and pagination cover the source window only. `--offset 5` retrieves the next local slice; honor `next_offset`. Metadata reports partial coverage. Use `--cache-dir` to isolate observations for a task. A fresh inventory list may be empty until refresh; no implicit network request.

## Errors

JSON stdout, diagnostics stderr. Focused commands reject `--csv`, `--plain`, `--quiet`, and explicit `--compact` with usage exit 2; use `--select` to narrow JSON. The defaults supplied by `--agent` are supported. Input failures exit 2; missing facility 3; public access denied 4; transport/source-shape failure 5; exhausted throttling 7; cache/config failure 10. Source-rejected offers are successful reads; inspect `validation`, never infer availability from price alone.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.
- **`facilities near`** — Nearby discovery

  ```bash
  ecbo-cloak-pp-cli facilities near --lat 35.6812 --lon 139.7671 --limit 3 --agent --select id,name,name_ja,booking_url
  ```
- **`facilities get`** — Facility identity/hours/restrictions

  ```bash
  ecbo-cloak-pp-cli facilities get GBy4uBrI --agent
  ```
- **`offer inspect`** — Source date/time/count price and validity

  ```bash
  ecbo-cloak-pp-cli offer inspect 0c3fb1e9-ad5a-42de-bfda-3027ebe4921e --from 2026-10-03T19:00 --to 2026-10-03T21:00 --small 1 --large 1 --agent --select quote,validation,availability,booking_url
  ```
- **`inventory refresh`** — Explicit bounded inventory refresh

  ```bash
  ecbo-cloak-pp-cli inventory refresh --lat 35.6812 --lon 139.7671 --limit 3 --agent
  ```
- **`inventory list`** — Offline inventory/query

  ```bash
  ecbo-cloak-pp-cli inventory list --limit 3 --agent
  ```

## Recipes

### Find nearby storage with Japanese names

```bash
ecbo-cloak-pp-cli facilities near --lat 35.6812 --lon 139.7671 --limit 3 --agent --select id,name,name_ja,booking_url
```

Find nearby storage with Japanese names

### Inspect source quote and validation separately

```bash
ecbo-cloak-pp-cli offer inspect 0c3fb1e9-ad5a-42de-bfda-3027ebe4921e --from 2026-10-03T19:00 --to 2026-10-03T21:00 --small 1 --large 1 --agent --select quote,validation,availability,booking_url
```

Inspect source quote and validation separately

### Explicitly refresh a bounded window

```bash
ecbo-cloak-pp-cli inventory refresh --lat 35.6812 --lon 139.7671 --limit 3 --agent
```

Explicitly refresh a bounded window

### Read the last window offline

```bash
ecbo-cloak-pp-cli inventory list --limit 3 --agent
```

Read the last window offline
