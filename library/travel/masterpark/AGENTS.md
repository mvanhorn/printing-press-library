# MasterPark Printed CLI Agent Guide

This directory is a hand-authored `masterpark-pp-cli` printed CLI for MasterPark's netParkV2 reservation API. Treat systemic Printing Press issues as upstream generator work, but keep MasterPark-specific protocol and docs fixes local to this CLI.

## Local Operating Contract

Start by asking the CLI for current runtime truth:

```bash
masterpark-pp-cli --help
masterpark-pp-cli locations --json
```

Before running any command that may mutate remote state, inspect help and prefer a dry run:

```bash
masterpark-pp-cli reserve --help
masterpark-pp-cli reserve --lot B --dropoff "2030-06-11 07:00" --pickup "2030-06-13 18:30" --quote 0 --json
```

Use `--submit --yes` only after the booking target, dates, quote index, customer, vehicle, and payment-at-lot behavior are clear. Under `PRINTING_PRESS_VERIFY=1`, `reserve --submit` must no-op before calling the live mutation endpoint.

For install, credentials, reservation-list caveats, and longer product guidance, read `README.md` and `SKILL.md`.

## Secret Safety

Do not print or persist MasterPark passwords. Prefer `--username-command` / `--password-command` references, such as an `op read ...` command, so secret values stay outside transcripts and config files. The config file may contain non-secret profile and vehicle metadata only.

## Reservation Checking Caveat

`reservations list` is a profile/history endpoint, not the source of truth for a booking just created through `saveReservation`. Immediately after booking, use the successful `saveReservation` response and the MasterPark confirmation email as authoritative; `listReservations` may lag or omit new active reservations.

## Release Ledger

`CHANGELOG.md` and `.printing-press-release.json` are the public library's per-CLI release ledger. Fresh prints may carry blank skeletons, but the final `YYYY.M.N` CLI release version is assigned only after a publish PR merges in `mvanhorn/printing-press-library`. Do not hand-bump those files or edit `var version = ...` for release bookkeeping; preserve existing ledger files on reprint and let the library workflow stamp the next release.

## Local Customizations

This directory is generated/printed output. If you modify generated CLI files, record each code-level customization under `.printing-press-patches/` so a future regen knows which hand-authored behavior to preserve.
