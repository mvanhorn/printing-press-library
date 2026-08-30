# CAROL Bike CLI Brief

## API Identity
- Domain: personal fitness and workout history.
- Users: CAROL Bike riders reading their own account data.
- Data profile: private ride records, aggregate statistics, calendars, and trends.

## Reachability Risk
- High: the API is undocumented, authenticated, and unsupported. It was observed in the authenticated rider dashboard and may change without notice.
- Publication caveat: technical access does not establish legal permission. Public distribution must retain the unofficial/read-only warning and receive human review before PR creation.

## Top Workflows
1. Read the latest ride and weekly ride frequency.
2. Page through personal REHIT ride history.
3. Read aggregate statistics, recent calendar activity, and trend series.
4. Mirror ride history into SQLite for offline search and analysis.

## Table Stakes
- Bearer authentication through environment variables; no committed credentials.
- JSON, CSV, compact, select, agent, and dry-run output modes.
- Typed read-only endpoint commands.
- Idempotent local sync and SQLite search.

## Data Layer
- Primary entities: rides, rider statistics, calendar rows, trends.
- Sync cursor: full pagination; the observed API exposes no durable updated-since cursor.
- FTS/search: locally synced ride payloads.

## Product Thesis
- Name: CAROL Bike CLI.
- Why it should exist: CAROL provides a private dashboard but no supported workout export or public API; this CLI gives riders agent-friendly read-only access to their own history without keeping a browser running.

## Build Priorities
1. Generate all seven observed GET endpoints.
2. Generate bearer auth and rider-scoped configuration from the spec.
3. Use the standard generated sync/search/SQLite framework as the sole novel capability.
4. Add no custom summary or other hand-written command.
