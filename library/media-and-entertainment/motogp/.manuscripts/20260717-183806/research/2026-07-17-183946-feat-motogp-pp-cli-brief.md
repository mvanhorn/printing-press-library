# MotoGP CLI Brief

## API Identity
- Domain: MotoGP / Moto2 / Moto3 racing data (official Dorna data via Pulselive)
- Base URL: https://api.motogp.pulselive.com/motogp/v1
- Auth: None (read-only, GET only, JSON)
- Users: MotoGP fans, fantasy players, data analysts, journalists, AI agents answering race questions
- Data profile: seasons → events → categories → sessions → classifications; riders + career stats; teams; standings; live timing. Everything keyed by UUID (seasonUuid, eventUuid, categoryUuid, sessionId).

## Reachability Risk
- None. Live probe returned HTTP 200 with clean JSON; 2026 season present and current. No auth, no bot protection observed.

## Top Workflows
1. "Who won [race]?" — resolve year+event+category → session classification
2. "Show championship standings for [year] [class]" — standings by season+category
3. "Season calendar + which rounds are done" — events list with finished flag + winners
4. "Rider profile / career stats / head-to-head" — riders + stats endpoints
5. "Live timing during a session weekend" — livetiming-lite feed

## Table Stakes (from competitors)
- Read results/classifications by category, year, session type (xNegis, ParsaD23)
- Standings/points (all importers)
- Full entity import: seasons, events, categories, sessions, results, standings, teams (racingmike)
- Live timing lite (racingmike, motogp-zero)
- ICS calendar export (racingmike)
- Rider career + season-by-season stats (broadcast API)

## The UUID Problem (core value-add)
Every Results-API call needs chained UUIDs. No competitor CLI exists; all are libraries.
Our CLI resolves human inputs (year, class name, event name/country, rider name) → UUIDs
via a local SQLite resolution cache, so `motogp results 2024 dutch motogp race` just works.

## Data Layer
- Primary entities: seasons, events, categories, sessions, classifications, standings, riders, rider_stats, teams
- Sync cursor: seasonUuid (per-season sync); events isFinished flag
- FTS/search: riders (name/country/team), events (name/circuit), teams
- Enables: cross-season rider aggregation, championship progression, circuit history — none of which is a single API call

## Product Thesis
- Name: motogp-pp-cli
- Why it should exist: first CLI/MCP for MotoGP. Turns a UUID-chained JSON API into human-friendly
  commands with offline SQLite history, agent-native output, and analyses (title race, h2h, circuit
  history) that no single endpoint provides.

## Build Priorities
1. Data layer + sync (seasons/events/categories/sessions/standings/riders/teams) + UUID resolver
2. Absorbed read commands: seasons, calendar, results, standings, riders, rider-stats, teams, grid, entry, sessions, live
3. Transcendence: resolve-by-name, title-race progression, h2h, circuit-history, rider-career, calendar --ics, "since"
