# Wahoo Cloud API — Novel Features Brainstorm (audit trail)

Subagent: general-purpose. First print (no prior research.json). Output below verbatim.

## Customer model

**Marcus — the data-owner roadie ("only an app" guy)**
Owns an ELEMNT BOLT, ~4 years of rides in Wahoo's cloud. The brief's archetypal user: perceives Wahoo as "only an app," chose the Cloud-API path over BLE because he wants his data scriptable and backup-able.
- **Today:** Rides upload from the BOLT over WiFi, fan out to Strava. FIT files live only in Wahoo's cloud and Strava's. No local copy, no bulk-export. Old ride = open app and scroll.
- **Weekly ritual:** Sunday long ride, glances at the app's single-ride screen, manually checks Strava for the week's total.
- **Frustration:** "I own this device and these rides, but I can't get my raw data out or back it up. If Wahoo's sync breaks or I churn off Strava, my history is hostage."

**Diane — the self-coached time-trialist / FTP chaser**
Structured-training cyclist on a KICKR, tracks FTP, rides to power zones. Resents TrainingPeaks paywall on basic load metrics.
- **Today:** Pays TrainingPeaks for CTL/ATL/TSB and FTP charts. The Wahoo app shows per-ride power but computes nothing across rides.
- **Weekly ritual:** Checks training load mid-week to decide whether to add intensity; bumps FTP after a breakthrough and wants the progression.
- **Frustration:** "The API has every watt I've ever pushed but does zero math. I'm paying a subscription to compute Form from data I already own."

**Theo — the route-planning randonneur / gravel adventurer**
Plans big routes with elevation, pushes routes to the ELEMNT for nav. Dozens of saved routes.
- **Today:** Builds routes in third-party tools, uploads them, then can't find anything in the flat app list.
- **Weekly ritual:** Saturday adventure ride; Friday night picks/uploads a route to the BOLT.
- **Frustration:** "I have 60 routes and the app gives me a scroll list with no filtering. I can't query by distance, elevation, or location."

**Sasha — the coach with a roster (agent-native power user)**
Coaches a handful of athletes, lives in the terminal, wants scriptable ride data.
- **Today:** Logs into each athlete's connected platform manually; copies numbers into spreadsheets.
- **Weekly ritual:** Monday review of each athlete's prior-week volume and load; flags who overreached.
- **Frustration:** "I want `wahoo ... | claude` or a SQL query over rides, not a phone app."

## Candidates (pre-cut)

- **C1 backup** (KEEP) — mirror every workout record + FIT file locally, resumable. FIT downloads rate-limit-free.
- **C2 load** (KEEP, correctness flag) — CTL/ATL/TSB from per-ride stress; parse string metrics NULL-safe; power+FTP with HR/duration fallback.
- **C3 ftp-history** (KEEP, sibling watch) — FTP progression from power_zones records/snapshots.
- **C4 bests** (KEEP, descoped) — records over summary fields (avg power, distance, ascent, duration, work). MMP power-curve version cut (needs FIT streams).
- **C5 digest** (KEEP) — last-N-days rollup, pipeable.
- **C6 routes find** (KEEP) — filter routes by distance/elevation band + proximity (haversine).
- **C7 push** (KILL) — wrapper over absorbed routes/plans create.
- **C8 recap** (KILL) — merge into `digest --days 365`.
- **C9 webhook serve** (KILL) — background process + needs public endpoint; non-core.
- **C10 gear** (KILL) — no gear entity in spec.
- **C11 calendar** (KILL) — presentation gloss over load/digest daily buckets.
- **C12 streaks** (KILL) — niche, weak evidence.
- **C13 workouts dupes** (KEEP→cut in Pass 3) — data-quality query, lowest evidence of the keeps.
- **C14 zones** (KILL) — time-in-zone needs per-second streams; API exposes only avg power.

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | Ride + FIT archive | `backup --out ./dir [--since] [--full]` | 9/10 | hand-code | Iterates the local workouts mirror, downloads each workout's `file.url` FIT + writes record JSON to a resumable directory tree; FIT downloads are rate-limit-exempt | Brief Top Workflow #1 + Product Thesis; `james-millner/go-wahoo-cloud-api` stores FIT files (2 sources) | none |
| 2 | Training load (CTL/ATL/TSB) | `load [--since] [--days 90] [--json]` | 9/10 | hand-code | Parses string-typed summary metrics (NULL-safe) into per-ride training stress, then computes 42d/7d exponentially-weighted Fitness/Fatigue and Form=CTL−ATL over local SQLite | Brief Top Workflow #2 + TrainingPeaks paywall gap (2 sources) | Use for the multi-day Fitness/Fatigue/Form trend. For all-time single-metric records use `bests`; for a fixed recent window total use `digest`. |
| 3 | Offline route finder | `routes find [--distance A-B] [--ascent] [--near LAT,LNG --radius] [--json]` | 8/10 | hand-code | Range-filters the routes mirror on stored `distance`/`ascent`/`descent` and applies haversine from `starting_lat/lng` — local SQLite | Brief Top Workflows #3/#4 + routes geo+elevation fields (2 sources) | Use to pick a saved route by distance/elevation/location. For free-text name/description matching use `search --type route`. |
| 4 | FTP progression | `ftp-history [--json]` | 7/10 | hand-code | Reads FTP/critical-power snapshots from the power_zones mirror over time and emits a dated progression, adding watts/kg when profile weight is set | Brief Top Workflow #2 + power_zones FTP fields (2 sources) | Use for the FTP-over-time view derived from power zones. For training-load trend use `load`; this command does not compute Form. |
| 5 | Recent-window digest | `digest [--days 7] [--json]` | 8/10 | hand-code | Aggregates count/distance/time/ascent/work and load delta for the last N days from the local workouts mirror; plain output pipeable to `\| claude` | Brief Top Workflow #4 + agent-native thesis; coach persona (2 sources) | Use for a one-shot rollup of any recent window (`--days 365` covers year-in-review). For the continuous load curve use `load`; for all-time records use `bests`. |
| 6 | Personal bests | `bests [--metric power\|distance\|ascent\|duration] [--json]` | 7/10 | hand-code | Sorts/maxes the NULL-safe-parsed summary fields across the workouts mirror for all-time and per-period records | Brief Top Workflow #2 (PRs) + workout_summary metric set (2 sources) | Use for record summary-metric values per ride. Uses stored ride summaries, not per-second streams, so reports avg/total records, not mean-maximal power. For trends use `load`. |

### Killed candidates

| Feature | Kill reason | Closest-surviving-sibling |
|---------|-------------|---------------------------|
| Plan/route push (`push`) | Friendlier verb over absorbed multipart `routes create`/`plans create` — wrapper, not leverage. | absorbed `routes create` |
| Year in review (`recap`) | Same machinery as `digest` with a fixed window; `digest --days 365` covers it. | `digest` |
| Webhook receiver (`webhook serve`) | Persistent background process needing a public endpoint; scope-creep + non-core. | none |
| Gear mileage (`gear`) | No gear/equipment entity in the 28-op spec; would be invention. | none |
| Training heatmap (`calendar`) | Duplicates load/digest daily buckets; only delta is ASCII-grid gloss. | `load` / `digest` |
| Streaks (`streaks`) | Niche motivational stat, thin domain value, weak evidence. | `digest` |
| Duplicate/corrupt finder (`workouts dupes`) | Legit local-data query but lowest persona-pain evidence; cut to hold at six. | `digest` |
| Zone distribution (`zones`) | Time-in-zone needs per-second power streams; API exposes only avg-power summary. | none |
