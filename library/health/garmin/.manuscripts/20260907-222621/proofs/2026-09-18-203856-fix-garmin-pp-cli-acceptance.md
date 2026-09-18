# Garmin CLI — Phase 5 Live Acceptance

## Verdict: PASS (full level), one novel feature not exercisable by a harness

- Run: `cli-printing-press dogfood --live --level full --timeout 120s --auth-env GARMIN_ACCESS_TOKEN`, press 4.32.0, 2026-09-19 02:01 UTC, on the source tree this PR ships.
- Target: the real Garmin Connect API, one personal account, bearer token supplied through `GARMIN_ACCESS_TOKEN`. The CLI uses an environment token as supplied and never refreshes it, so the run could not rotate the account's stored refresh token.
- Matrix: 150 counted tests, 150 passed, 0 failed, 110 skipped. Marker: `phase5-acceptance.json` (`status: pass`, `level: full`). Every row without its response sample: `phase5-matrix.json` (`source_live` marks the 39 rows that reached the API).

## What ran against the live API

Nineteen endpoint commands returned live data (`meta.source: live`):

`account personal-information`, `account settings`, `account social-profile`, `activities breakdown`, `activities list`, `fitness age`, `fitness max-metrics`, `heart-rate daily-alt`, `heart-rate zones`, `sleep night-alt`, `sleep score-stats`, `sleep stats`, `steps daily`, `steps weekly`, `training readiness`, `training status`, `wellness hydration`, `wellness hydration-alt`, `wellness intensity-minutes-weekly`.

Four error-path probes ran with an invalid id and each exited non-zero — `activities get`, `activities splits`, `activities hr-time-in-zones` (exit 5) and `activities download-original` (exit 3). The matrix marks none of the four `meta.source: live`, so these exit codes are the whole of the evidence for them.

The archive commands (`history --status`, `insights sleep`, `insights training`, `sql`, `analytics`, `workflow archive`, `workflow status`) ran against the matrix's empty sandbox archive and passed.

## What the matrix skipped, and why

| Count | Reason |
|---|---|
| 38 | error-path probe on a command with no positional argument |
| 33 | command needs a positional the matrix cannot invent (`displayName`, a search query, a profile name) — 14 happy-path, 14 JSON-fidelity, 5 error-path rows |
| 16 | mutating framework commands, dry-run only |
| 14 | the matrix could not read an activity id or a learnings id out of the list command's output |
| 9 | other fixture gaps: `export` ×3, `sync` ×3 (mutating, no runnable example), `teach-playbook` ×2, `tail` ×1 |

Four endpoint commands take a `displayName` positional and fall in row two: `heart-rate daily`, `sleep night`, `wellness daily-summary` and `wellness metrics-daily`. `heart-rate daily-alt` and `sleep night-alt` return the same payloads from a date alone, and both ran live. The four per-activity reads (`activities get`, `splits`, `hr-time-in-zones`, `download-original`) fall in row four, so only their error paths were exercised at all.

## Hand pass over the reads the matrix could not reach

Run on 2026-09-18 with `--json` on two personal accounts, each with its own token through `GARMIN_ACCESS_TOKEN`, fixtures read from `account social-profile` and `activities list`. All seven commands exited 0 with `meta.source: live` on both accounts:

`heart-rate daily <displayName> --date`, `sleep night <displayName> --date`, `wellness daily-summary <displayName> --calendar-date`, `wellness metrics-daily <displayName> --from-date --until-date --metric-id 60`, `activities get <id>`, `activities splits <id>`, `activities hr-time-in-zones <id>`.

| Read | Account A | Account B |
|------|-----------|-----------|
| `heart-rate daily` | 59 B, empty result | 28 658 B |
| `sleep night` | 914 B | 184 596 B |
| `wellness daily-summary` | 2 913 B | 3 280 B |
| `wellness metrics-daily` | 247 B | 397 B |
| `activities get` | 5 787 B | 3 802 B |
| `activities splits` | 66 655 B | 1 423 B |
| `activities hr-time-in-zones` | 57 B, empty list | 508 B, five zones |

Account A has no heart-rate data on record, so its two heart-rate reads return an empty result. Account B has it, and all seven reads return data there. Sizes are whole-response byte counts; no response content is kept.

`activities download-original` was left out: it returns a binary ZIP of one recorded activity.

## Hollow coverage: `auth login`

The marker records `coverage_hollow: true` for `auth login`. The press's live matrix carries no `auth` command at all by design, because a login opens a browser and changes session state, and `auth login` is one of this CLI's five novel features. No harness run can clear that flag. The command's behaviour is covered by its unit tests (state and ticket parsing, account mismatch, refresh body) and by hand sign-ins on two accounts.

`history` runs in the matrix as `history --status`. A plain `history` on the matrix's empty sandbox home is a full-account fill from 2007 and outlasts any per-test timeout.
