# Ashby CLI Brief

## API identity

Ashby's official lightweight Job Postings API exposes currently published jobs
for one named public board through an unauthenticated GET request. The CLI is a
read-only job-board client, not an administrative client for Ashby's authenticated
recruiting API.

## Product thesis

Ashby's public feed is not only a careers-page payload. Repeated local snapshots
turn user-selected company boards into a keyless job-market change feed.

## Primary workflows

1. List and retrieve listed openings for a user-supplied company board.
2. Filter by title, query, department, team, location, workplace, employment type,
   publication date, and disclosed compensation.
3. Synchronize selected boards into SQLite for offline full-text search.
4. Compare snapshots to identify newly listed, changed, and removed openings.
5. Produce compact JSON, CSV, selected fields, and MCP responses for agents.

## Safety and scope

- No API key, cookies, browser session, or authenticated API endpoints.
- GET-only requests to `api.ashbyhq.com/posting-api/job-board/{jobBoardName}`.
- General listing, search, sync, and export exclude `isListed=false` postings.
- No automatic enumeration of Ashby customers and no application submission.
- Preserve original `jobUrl` and `applyUrl`; do not rehost employer media.
- Conservative bounded concurrency, caching, and retry/backoff.

## Data layer

The high-gravity entity is a job posting. Store stable UUID, board, title,
department, team, workplace and employment types, locations, descriptions,
publication timestamp, source/application URLs, disclosed compensation, first
seen, last seen, content hash, and active/removed state. Use FTS5 over title,
team, department, location, and plain-text description.

## Lever parity and differentiators

Match Lever's generic `postings list <company>` and `postings get <company> <id>`
shape plus Printing Press agent output, profiles, delivery, learning, SQLite, and
MCP conventions. Ashby adds first-class structured workplace, employment,
secondary-location, publication-time, and compensation filtering.

## Sources

- https://developers.ashbyhq.com/docs/public-job-posting-api
- https://docs.ashbyhq.com/using-the-lightweight-job-posting-api-to-list-openings-on-your-site
- https://github.com/mvanhorn/printing-press-library/tree/main/library/job-boards/lever
