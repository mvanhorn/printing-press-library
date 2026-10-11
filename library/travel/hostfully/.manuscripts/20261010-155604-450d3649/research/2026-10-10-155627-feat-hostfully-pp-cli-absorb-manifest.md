# Hostfully CLI Absorb Manifest

## Sources Analyzed
- Hostfully API v3 / v3.3 documentation (dev.hostfully.com)
- Prior printed hostfully-pp-cli (run 20260525-111012) and its post-print patches
- Zapier Hostfully integration (feature inventory only)

Note: the novel-features subagent was not spawned in this run (executed inside a worker that cannot spawn subagents). The transcendence list below is the operator-approved set carried from the prior print plus its live-operation patches; the user pre-approved it at the absorb gate.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | List reservations with filters | API docs | (generated endpoint) leads list | --json, --select, archive-backed |
| 2 | Property listing | API docs | (generated endpoint) properties list | Property discovery feeds every per-unit command |
| 3 | Today's summary | Prior CLI | hostfully-pp-cli today | Live, per-property, check-ins/outs/in-house/vacant |
| 4 | Availability grid | Prior CLI | hostfully-pp-cli availability | Live only, no cache fallback |
| 5 | Current guests | Prior CLI | hostfully-pp-cli guests | Phone/email/dates/channel |
| 6 | Reservation sync | API docs | hostfully-pp-cli fetch | Per-property pagination with GraphQL 500 recovery |
| 7 | Search guests by name | New | hostfully-pp-cli search | Offline FTS |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Caller-ID guest lookup | whois | hand-code | Joins every property's history by phone/name with live payment status and access PIN | Use for "who is calling/texting". Do NOT use for availability; use 'availability'. |
| 2 | Guest messaging inbox | inbox | hand-code | v3.3 threads + guarded replies with --dry-run | none |
| 3 | Live complete quote | quote | hand-code | v3.3 quote calc with itemized tax/fees per property | none |
| 4 | Service jobs | jobs | hand-code | Jobs with service and provider identity, cancelled retained, for invoice review | none |
| 5 | Multi-property vacancy finder | vacancy | hand-code | Cross-property date intersection | none |
| 6 | Occupancy analysis | occupancy | hand-code | Historical archive per property/period | none |
| 7 | Turnover gaps | turnover | hand-code | Cleaning windows between stays | none |
| 8 | Channel mix | channels | hand-code | Aggregated booking sources over time | none |
| 9 | Revenue estimate | revenue | hand-code | Nightly revenue from history | none |
| 10 | Booking patterns | patterns | hand-code | Weekday/weekend and seasonal trends | none |
| 11 | Maintenance windows | maintenance | hand-code | Slots across properties around bookings | none |
| 12 | Native mock mode | (behavior in hostfully-pp-cli whois) --mock | hand-code | Placeholder data for agent tests without credentials | none |

## Feature Count
- Absorbed: 7; Transcendence: 12
