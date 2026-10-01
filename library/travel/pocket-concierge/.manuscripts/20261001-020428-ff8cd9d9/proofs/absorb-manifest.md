# Approved focused manifest
The user's task preauthorizes these public read features. No stubs. Sole builder performs ideation inline per explicit instruction; no novel-features worker. No external community tool contributed runnable endpoint features.

| Feature | Best source | Our Implementation | Added value |
|---|---|---|---|
| Native area/cuisine filters | First-party guest GraphQL | pocket-concierge-pp-cli filters | Bilingual IDs, no guessed region aliases |
| Restaurant discovery | venuesSearch | pocket-concierge-pp-cli restaurants search | Bounded pagination/date/party/budget, compact summaries |
| Restaurant policies/detail | venue | pocket-concierge-pp-cli restaurants get | Japanese identity, explicit conditions, lazy details |
| Course identity and prices | venue.courses | pocket-concierge-pp-cli courses list | Guest/group units; original inclusion/additional fee text |
| Reservation/waitlist calendar | availabilityCalendar | pocket-concierge-pp-cli availability dates | Distinct statuses, explicit freshness |
| Date/party sessions | availabilitySearch | pocket-concierge-pp-cli availability slots | Source session IDs, request vs instant vs waitlist, null unknowns |
| Canonical handoff | first-party restaurant routes | pocket-concierge-pp-cli booking handoff | Validated venue/course/session ownership, safe page URL |
| Runtime/source health | public taxonomy query | pocket-concierge-pp-cli doctor | Typed diagnostics, no credentials |

## Focused added behaviors (hand-written within commands)
- Bilingual identity joins by source ID instead of translated names.
- Preserve session identity and filter party bounds without treating waitlists as confirmed seats.
- Expose fee statement conflicts instead of inventing a payable total.
- Explicit inventory refresh and short optional availability cache with age/TTL.
- Projection, dry-run and runtime request/latency metrics for agent context efficiency.

Local data: bounded response cache; no full inventory sync or SQL/learning framework. No mutation endpoint/raw query exposed. Source only Pocket Concierge en/ja. No payments, bookings, accounts, publication, PRs, or shared configuration changes.
