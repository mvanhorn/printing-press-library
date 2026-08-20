# RideWithGPS Novel Features Brainstorm (audit trail)

Subagent: general-purpose · first print (no prior research) · 6 survivors / 8 killed

## Customer model

### Persona 1 — Marco, gravel/road power user with a head-unit habit
- **Today:** Plans on web, exports GPX one route at a time to side-load onto Wahoo. ~600 routes over 8 years, many near-dupes.
- **Weekly ritual:** Friday night picks 2-3 routes, exports each by hand, copies to head unit.
- **Frustration:** No bulk export (#1 unmet need); library is a junk drawer of dupes he can't see or clean.

### Persona 2 — Dana, high-mileage rider who babies her drivetrain
- **Today:** Tracks chain/cassette/tire mileage in a spreadsheet, manually adding each ride.
- **Weekly ritual:** After Saturday long ride, updates spreadsheet, eyeballs chain wear.
- **Frustration:** Gear mileage locked in the dashboard, not queryable or alertable. Finds out a chain is worn by feeling it skip.

### Persona 3 — Priya, data-driven trainer chasing numbers
- **Today:** Exports to third-party analytics or scrolls app month by month for volume and PRs.
- **Weekly ritual:** Sunday review — tallies week distance/elevation/time, checks for records.
- **Frustration:** No training aggregates or PRs in any RideWithGPS tool; manual scrolling, PRs from memory.

### Persona 4 — Sam, club organizer / randonneur running events at scale
- **Today:** Manages club events + large route catalog through web UI, one entity at a time.
- **Weekly ritual:** Before event weekends, audits which routes are stale, mis-tagged, missing cue sheets.
- **Frustration:** No offline mirror, no scriptable search/audit; shared-route blindspot; everything click-through.

## Survivors (transcendence features)

| # | Feature | Command | Score | Buildability | Persona |
|---|---------|---------|-------|--------------|---------|
| 1 | Bulk export | `export --type routes\|trips --format gpx\|tcx\|csv\|kml [--native] [--out DIR]` | 9/10 | hand-code | Marco |
| 2 | Gear mileage + maintenance | `gear mileage [--bike NAME]` / `gear due [--threshold KM]` | 9/10 | hand-code | Dana |
| 3 | Route dedup | `dedup [--threshold M] [--apply]` | 7/10 | hand-code | Marco |
| 4 | Training stats | `stats [--period week\|month\|year] [--by activity-type]` | 8/10 | hand-code | Priya |
| 5 | Personal records | `records [--metric distance\|elevation\|speed\|power] [--top N]` | 7/10 | hand-code | Priya |
| 6 | Library audit | `audit [--checks stale,no-cue-sheet,untagged,private]` | 6/10 | hand-code | Sam |

## Killed candidates

| Feature | Kill reason | Closest sibling |
|---------|-------------|-----------------|
| `elevation` (ASCII climb profile) | Single-entity render, no cross-join, not a weekly ritual | `export` |
| `cues` (cue sheet render) | Thin view of `routes get`; overlaps `export --format csv` | `export` |
| `places` (locality rollup) | Speculative, no research backing; overlaps `stats` | `stats` |
| `compare` (two-ride compare) | Duplicates `dedup`+`records`; speculative; scope creep | `records` |
| `load` (rolling weekly load) | Pure overlap with `stats --period week` | `stats` |
| `photos` (media export) | No demand; generic CDN fetch, not cycling-content transcendence | `export` |
| `gear log` (maintenance writeback) | No API maintenance resource; local-only can't round-trip; app scope creep | `gear` |
| `import-shared` (shared-route copy) | No confirmed copy endpoint in 29-op spec; feasibility unproven | `export` |
