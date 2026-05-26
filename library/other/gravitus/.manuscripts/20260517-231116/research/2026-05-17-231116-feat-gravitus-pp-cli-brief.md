# Gravitus CLI Brief

## API Identity
- Domain: Strength training workout tracking (iOS app + Django web backend)
- Users: Lifters, gym-goers tracking sets/reps/weight; powerlifters tracking PRs; anyone using Gravitus as their training log
- Data profile: Workouts (sessions with exercises, sets, reps, weight, notes), exercise library (300k+ exercises), routines/programs, personal records, body measurements, community social features (video sharing, leaderboards, streaks)

## Reachability Risk
- **HIGH** — Gravitus has no public API and no developer documentation.
- Only one GitHub repo in their org: a forked Swift UI indicator (last updated 2022).
- No community reverse-engineered API found anywhere.
- Web login exists at gravitus.com/accounts/sign_in/ (Django backend) — this is the primary discovery target.
- The mobile app calls an undiscovered REST backend; browser-sniffing the web app is the only viable path to spec discovery.
- No data export feature found in any app store listing or community discussion.

## Top Workflows
1. **Log a workout**: Select a routine or ad-hoc, record sets/reps/weight for each exercise, finish and save
2. **Track progress on an exercise**: View PR history, volume trends, 1RM estimates for a specific lift (bench, squat, deadlift, etc.)
3. **Manage routines**: Create/edit workout programs (PPL, Upper/Lower, 5x5, etc.), organize into folders
4. **Review workout history**: Browse past sessions, filter by date/exercise, see streaks and frequency
5. **Analyze body measurements**: Track weight, body fat %, measurements over time

## Table Stakes (from Hevy competitor)
- List workouts with pagination
- Get single workout detail (exercises, sets, reps, weight, notes)
- Create/update workouts
- Workout count and event stream
- List/get/create/update routines
- Routine folder management
- Exercise template search and listing (300k+ exercises)
- Exercise history by template (PR tracking)
- Body measurements CRUD
- User profile info
- `--json` output for all commands
- Personal records viewing

## Data Layer
- Primary entities: workouts, exercises, routines, sets, body_measurements, personal_records
- Sync cursor: updated_at timestamp per entity (workout events endpoint pattern like Hevy)
- FTS/search: exercise name full-text search (300k+ exercises makes this critical)
- SQLite entities: workouts, exercises, routines, sets (denormalized for join queries), body_measurements, prs

## Reachability Strategy
- Auth type: cookie/session (Django web app) — discovered via browser-sniff
- Browser-sniff target: gravitus.com (authenticated session required)
- User must be logged in to Gravitus web app in Chrome for authenticated endpoint discovery
- Printed CLI auth: `auth login --chrome` to capture and replay session cookies

## Codebase Intelligence
- No GitHub source to analyze
- Django backend inferred from URL patterns (/accounts/sign_in/, /accounts/sign_up/, /accounts/password/reset/)
- Likely Django REST Framework for mobile API
- Response format: almost certainly JSON

## Competitor Analysis
| Tool | Platform | Features | Gap |
|------|----------|----------|-----|
| hevycli | Go CLI (Hevy) | Full CRUD + analytics + interactive TUI | Hevy-specific, requires PRO API key |
| hevy-mcp | Node MCP (Hevy) | Full CRUD via MCP | Hevy only, no offline/local |
| hevy-api (Python) | Python client | Hevy API wrapper | Library only, no CLI |
| No Gravitus CLI | — | Nothing exists | **We are first** |

## Product Thesis
- Name: gravitus-pp-cli
- Why it should exist: Gravitus has 10M+ workouts logged and 300k+ lifters with no programmatic access to their own data. This CLI is the first and only way to query, analyze, and export Gravitus training data from the terminal — enabling power users, coaches, and AI agents to work with their strength data like any other dataset.

## Build Priorities
1. Browser-sniff gravitus.com to discover the actual API surface (auth + workout + exercise + routine endpoints)
2. Sync engine pulling all workouts, exercises, routines into SQLite for offline analysis
3. Workout CRUD + list/get with filtering (matching and beating hevycli)
4. Exercise search with local FTS5 (critical for 300k+ exercise database)
5. Personal records and progress analytics (the killer feature for lifters)
6. Routine management
7. Body measurements
8. Novel transcendence features leveraging the local SQLite store
