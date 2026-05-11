# Novel Features Brainstorm — yc-companies-pp-cli

Full subagent output preserved for retro/dogfood audit.

## Customer model

**Persona A: Maya, Seed-Stage VC Scout**

- Today: Manually filters ycombinator.com/companies, copies into Notion, runs ad-hoc Python scrapers, ~90s re-fetch per refresh.
- Weekly ritual: Monday-morning shortlist of 20-30 latest-batch AI/dev-tools/fintech companies for Wednesday partner meeting.
- Frustration: No "what's new since last Monday?" — eyeballs hundreds of rows every week.

**Persona B: Devin, Founder Doing Competitive Research**

- Today: Googles + Ctrl-Fs the YC site; keeps ~30 peer slugs in a Google Doc, visits each weekly.
- Weekly ritual: Friday — refresh watch list, check for status flips, team-size jumps, new hiring signals.
- Frustration: Cross-time drift on a 30-slug list is impossible without screen-scraping.

**Persona C: Priya, Technical Recruiter Sourcing from YC Alumni**

- Today: LinkedIn Recruiter + YC hiring page; manually filters by region then clicks each one.
- Weekly ritual: Tue/Thu — pull hiring YC companies, filter by region + team-size band (11-50), CSV → ATS.
- Frustration: YC hiring page is single-axis; no team_size filter; manual CSV every time.

**Persona D: Ren, Data Journalist / Analyst**

- Today: Ad-hoc pandas against downloaded JSON dump for each story.
- Weekly ritual: Per-story — cross-batch aggregates by industry/tag/status.
- Frustration: Rewriting the same GROUP BY in pandas every time.

## Candidates (pre-cut)

(See subagent output — 16 candidates C1-C16.)

## Survivors and kills

### Survivors (7 features, all >= 5/10)

| # | Feature | Command | Score | Persona |
|---|---------|---------|-------|---------|
| 1 | Watch list management | `watch add/remove/list` | 8/10 | Devin, Maya |
| 2 | Watch diff | `watch diff [--since <date>]` | 10/10 | Devin, Maya |
| 3 | New since date / last sync | `companies new --since <date>` | 10/10 | Maya, Ren |
| 4 | Cross-index change feed | `companies changes --field <f> [--to v] --since <date>` | 10/10 | Maya, Ren, Priya |
| 5 | Peer discovery by tags | `companies similar <slug>` | 9/10 | Devin, Maya |
| 6 | Cross-batch aggregates | `stats by-batch` / `stats by-industry` | 10/10 | Ren, Maya |
| 7 | Batch summary card | `batches show <slug>` | 7/10 | Maya, Ren |

### Killed candidates

| Feature | Kill reason | Sibling |
|---|---|---|
| C7 tag cooccurrence | Borderline; too close to C6 | C6 with `--group-by tag` |
| C8 founder search | Verifiability — depends on optional `question_answers` field | revisit second print |
| C9 portfolio diff (ad-hoc list) | Subsumed by C4 `--slugs` filter | C4 |
| C10 hiring-flipped-true | Special case of C4 | C4 `--field isHiring --to true` |
| C12 CRM export | Already in absorbed `--csv`/`--select` | absorb items #12/#13 |
| C13 MCP serve | Infra; not novel | covered by pp primitives |
| C14 stale-data check | Already in absorbed `doctor` | absorb item #20 |
| C15 launch histogram | Sibling-killed by C6 | C6 |
| C16 named snapshots | Ceremony; `--since <date>` covers 95% | C3, C4 |
