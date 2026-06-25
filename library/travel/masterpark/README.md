# masterpark-pp-cli

Unofficial command-line client for the **MasterPark** airport parking
reservation system (the `netParkV2` WordPress plugin behind
<https://www.masterparking.com>).

It talks to the same public AJAX endpoint the website's reservation page uses,
fetching the `_wpnonce` CSRF token automatically before each call. Nothing is
faked: `locations` and `quote` hit the real API; mutating actions are gated.

> This is an independent tool and is not affiliated with or endorsed by
> MasterPark / Master Parking.

## Install / build

```bash
go build -o ./masterpark-pp-cli ./cmd/masterpark-pp-cli
```

## Global flags

| Flag        | Description                                                        |
|-------------|--------------------------------------------------------------------|
| `--json`    | Emit JSON instead of a table                                       |
| `--config`  | Config file path (default `~/.config/masterpark-pp-cli/config.json`)|
| `--timeout` | HTTP timeout (default `30s`)                                        |

`MASTERPARK_BASE_URL` overrides the API origin (useful for testing).

## Commands

### `locations` — list parking lots

```bash
masterpark-pp-cli locations
masterpark-pp-cli locations --json
```

Known lots: **MasterPark Lot B** (`2515-1-889`) and **MasterPark Lot G**
(`2525-1-893`).

### `quote` — price a date range

```bash
# The canonical example:
masterpark-pp-cli quote --lot G \
  --dropoff "2030-06-11 07:00" \
  --pickup  "2030-06-13 18:30"

# Oversize vehicle, Lot B, JSON:
masterpark-pp-cli quote --lot B \
  --dropoff "2030-06-11 07:00" \
  --pickup  "2030-06-13 18:30" \
  --vehicle-type oversize --json

# Raw codeID and a promo code:
masterpark-pp-cli quote --lot 2515-1-889 \
  --dropoff "2030-06-11 07:00" --pickup "2030-06-13 18:30" \
  --promo-code SAVE10
```

Flags: `--lot B|G|<codeID>` (required), `--dropoff`, `--pickup`
(`"YYYY-MM-DD HH:MM"`), `--vehicle-type standard|oversize`, `--promo-code`.

### `auth check` — show available credentials

Reports which sources provide a username/password without ever printing the
password. Accepts the generic credential flags (`--username`, `--password`,
`--username-command`, `--password-command`) to preview a resolution.

```bash
masterpark-pp-cli auth check
masterpark-pp-cli auth check --json
```

### `auth from-1password` — load credentials from 1Password

Uses the `op` CLI. The password is loaded into memory only — never printed or
written to disk.

```bash
masterpark-pp-cli auth from-1password \
  --vault Agent --item Masterparking \
  --username-field username --password-field password

# Validate against the real verifyLogin endpoint and save the (non-secret)
# 1Password reference to config:
masterpark-pp-cli auth from-1password --login-check --save

# Also log in and persist the non-secret customer profile + vehicles:
masterpark-pp-cli auth from-1password --save --sync-profile
```

### `auth sync-profile` — save your customer profile + vehicles

Logs in (using any credential source below) and saves the **non-secret**
customer profile and vehicles returned by MasterPark to the config file, so
`reserve` can fill those defaults automatically. The password is **never**
stored. No-ops under `PRINTING_PRESS_VERIFY=1`.

```bash
# Using env vars or a saved 1Password/command reference:
masterpark-pp-cli auth sync-profile --lot B

# Using generic, 1Password-independent credential injection:
masterpark-pp-cli auth sync-profile \
  --username-command "op read op://Agent/Masterparking/username" \
  --password-command "op read op://Agent/Masterparking/password"
```

### `reservations list` — list reservations returned by the profile endpoint

Requires credentials (flags, commands, env, config, or 1Password). Establishes a
real session via `verifyLogin` then calls `listReservations`. If the endpoint
requires a captured browser session, it returns a clear error rather than faking
data.

Important: MasterPark's `listReservations` endpoint is **not authoritative for
freshly-created bookings**. In live testing, `saveReservation` returned an active
reservation and MasterPark sent a confirmation email, but `listReservations` did
not immediately include that new reservation. Treat the successful
`saveReservation` response (especially the returned `reservation` number)
and the confirmation email as the source of truth for a just-booked reservation.
Use `reservations list` as a profile/history view that may lag or omit new active
reservations.

```bash
export MASTERPARK_USERNAME="you@example.com"
export MASTERPARK_PASSWORD="..."   # or use a credential command / 1Password
masterpark-pp-cli reservations list --json
```

### `reserve` — compose / submit a reservation

**Dry-run by default.** It prints a summary and does not book anything. Booking
for real requires both `--submit` and `--yes`, plus all customer/vehicle
fields. Under `PRINTING_PRESS_VERIFY=1` it no-ops and never calls the live API.

Missing customer/vehicle fields are filled from the saved profile (see
`auth sync-profile`) unless `--use-saved-profile=false`. Explicit flags always
win over saved-profile values.

```bash
# Dry run (safe), relying entirely on a previously synced profile:
masterpark-pp-cli reserve --lot G \
  --dropoff "2030-06-11 07:00" --pickup "2030-06-13 18:30" --quote 1

# Dry run with explicit fields:
masterpark-pp-cli reserve --lot G \
  --dropoff "2030-06-11 07:00" --pickup "2030-06-13 18:30" \
  --quote 1 --first-name Alice --last-name Smith \
  --email alice@example.com --phone "<phone>" \
  --vehicle-make Honda --vehicle-model Civic --plate ABC123

# Real booking (calls saveReservation); replace flags with the reviewed
# booking window/profile before running:
masterpark-pp-cli reserve --lot G \
  --dropoff "2030-06-11 07:00" --pickup "2030-06-13 18:30" \
  --quote 1 --submit --yes
```

## Credentials & configuration

Credential injection is **generic** and not tied to 1Password. Resolution order:

1. **Explicit flags** — `--username` / `--password`
2. **Credential commands** — `--username-command` / `--password-command`
   (run directly, no shell; their stdout is the credential, e.g.
   `op read op://Agent/Masterparking/password`). The agent never sees the value.
3. **Environment** — `MASTERPARK_USERNAME` / `MASTERPARK_PASSWORD`
4. **Config command refs** — `username_command` / `password_command` in the
   config file (references, not secrets)
5. **1Password** — `op item get` via a saved non-secret reference

The credential flags are available on `auth check`, `auth sync-profile`,
`reservations list`, and `reserve`.

- The config file (`config.json`) stores **only non-secret** metadata
  (`username`, `base_url`, a 1Password reference, command references, and the
  saved customer/vehicle profile). It has no password field, so a password can
  never be persisted.
- Credential commands are executed **without a shell** (argv is parsed with
  simple quote handling), and their output is never logged or printed.

## Safety

- Mutating commands (`reserve --submit`) require explicit confirmation and
  no-op under `PRINTING_PRESS_VERIFY=1`.
- The password is never printed by any command, including `--json` output of
  `auth check`.

## Tests

```bash
go test ./...
```

Tests use a mock HTTP server (validating the exact request shapes and headers),
a mock `op` runner (1Password field extraction), a mock command runner (generic
`--username-command` / `--password-command` injection), saved-profile round-trip
and reservation-default coverage, and assert the verify-mode no-op for
mutations. No test prints or persists a password.
