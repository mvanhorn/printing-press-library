# Texas Roadhouse Absorb Manifest

## Absorbed

| Capability | Source | Required CLI behavior |
|---|---|---|
| Nearby store lookup | Sniffed `GET /api/stores/near` | Return stores with `extref`; waitlist paths use extref, not internal id. |
| Quote by party size | Sniffed `GET /waitlist/{extref}/quote` | Expose `MinQuote` / `MaxQuote` buckets 1–6 so submit can send `WaitMinutes`. |
| Join waitlist | Sniffed `POST /waitlist/{extref}/submit` | Require email, party size ≤ 6, `WaitMinutes` from quote MinQuote. Live POST only with `--yes`. |
| Check-in (HERE) | Sniffed `POST /waitlist/{extref}/checkin` | Mark the party HERE at the host stand. Live POST only with `--yes`. |
| Cancel request | Sniffed `POST /waitlist/cancel` | Body `waitlistRequestId` is a JSON number; `siteId` is extref. Live POST only with `--yes`. |

## Transcendence

- Store presentation stops for a human pick; the CLI never auto-selects a store.
- `--dry-run` previews join/check-in/cancel bodies without POSTing.
- Default (neither `--yes` nor `--dry-run`) refuses live waitlist mutation.
- MCP submit/checkin/cancel are destructive and require the same confirmation.

## Reprint watch-list

- Keep cancel `waitlistRequestId` as a JSON number (string flags 400).
- Keep waitlist path ids as store extref, not internal store id.
- Keep email required and party size max 6 on submit.
- Keep `WaitMinutes` sourced from quote MinQuote.
- Keep check-in as HERE / POST checkin.
- Keep the live-mutation `--yes` / `--dry-run` gate on submit, checkin, and cancel.
