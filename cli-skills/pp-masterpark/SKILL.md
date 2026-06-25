---
name: pp-masterpark
description: "Unofficial CLI for MasterPark airport parking reservations, quotes, profile sync, and booking safety."
author: "bwishan"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - masterpark-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/travel/masterpark/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# MasterPark — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `masterpark-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install masterpark --cli-only
   ```
2. Verify: `masterpark-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/masterpark/cmd/masterpark-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Unofficial command-line client for MasterPark airport parking reservations using the netParkV2 WordPress/AJAX endpoints behind masterparking.com.

## Command Reference

**locations** — List MasterPark parking locations.

- `masterpark-pp-cli locations` — List parking lots.
- `masterpark-pp-cli locations --json` — List parking lots as JSON.

**quote** — Price a parking date range.

- `masterpark-pp-cli quote --lot B --dropoff "2030-06-11 07:00" --pickup "2030-06-13 18:30"` — Get standard quotes.
- `masterpark-pp-cli quote --lot G --dropoff "2030-06-11 07:00" --pickup "2030-06-13 18:30" --vehicle-type oversize --json` — Get oversize quotes as JSON.

**auth** — Manage credential sources without printing passwords.

- `masterpark-pp-cli auth check` — Report which credential sources are available.
- `masterpark-pp-cli auth from-1password --vault Agent --item Masterparking --login-check --save` — Load from 1Password, validate login, and save only non-secret references.
- `masterpark-pp-cli auth sync-profile --username-command "op read op://Agent/Masterparking/username" --password-command "op read op://Agent/Masterparking/password"` — Save non-secret customer and vehicle defaults from the account.

**reservations** — Read the reservations profile endpoint.

- `masterpark-pp-cli reservations list --json` — List reservations returned by MasterPark's profile/history endpoint.

Important: `reservations list` is not authoritative for freshly-created bookings. In live testing, `saveReservation` returned an active reservation and MasterPark sent a confirmation email, but `listReservations` did not immediately include that new reservation. Treat the successful `saveReservation` response and confirmation email as the source of truth immediately after booking.

**reserve** — Compose or submit a parking reservation.

- `masterpark-pp-cli reserve --lot B --dropoff "2030-06-11 07:00" --pickup "2030-06-13 18:30" --quote 0` — Dry-run a reservation; does not book.
- `masterpark-pp-cli reserve --lot B --dropoff "2030-06-11 07:00" --pickup "2030-06-13 18:30" --quote 0 --submit --yes` — Submit the reservation for real.

`reserve` is dry-run by default. Real booking requires both `--submit` and `--yes`. Under `PRINTING_PRESS_VERIFY=1`, it no-ops before calling live mutation endpoints.

## Credential Safety

Credential injection is generic and not tied to 1Password. Commands that need login accept direct flags and command-output flags:

```bash
masterpark-pp-cli auth check --username-command "op read op://Agent/Masterparking/username" --password-command "op read op://Agent/Masterparking/password"
```

Password values are never printed or persisted. Config may store non-secret metadata, command references, and saved customer/vehicle profile data.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `masterpark-pp-cli --help` output.
2. **Starts with `install`** → follow Prerequisites above.
3. **Anything else** → run the requested `masterpark-pp-cli` command. Prefer `--json` for machine-readable output.
