# Lever Postings API — research notes

Run: 20260807-044310 (cli-printing-press 4.28.0)
API: Lever public postings API, api.lever.co/v0

## Endpoint recon

- `GET /postings/{company}?mode=json` — every open posting for a company.
  Live probe: leverdemo returns 388 postings. No auth. Response is a bare
  JSON array of posting objects.
- `GET /postings/{company}/{posting_id}?mode=json` — single posting detail.
  posting_id is a UUID from the list payload.
- Company slug is a path segment identifying the employer's Lever board
  (leverdemo, palantir, etc.). Slugs 404 when the company is not on Lever
  (probed: eventbrite, faire, rippling, netflix all 404 or empty; palantir
  returns 301 postings).
- Pagination: `limit` / `offset` query params (offset pagination), default
  limit 100. With mode=json the API returns the full posting set.

## Auth

None. Public job board data, matching the generic-token pattern of the
greenhouse package. No bot/bearer/basic scheme; agentcookie correctly
skips.

## Spec shape

- Base URL: https://api.lever.co/v0
- Two read-only GET endpoints, both `mcp:read-only` and OpenWorld.
- Response schema `Posting`: id, text (HTML), categories (team, location,
  commitment, level, workplaceType), hostedUrl, applyUrl, createdAt,
  department, contactEmail.

## Generic-CLI design

Single CLI serves ANY company's public board: company is the first
positional argument on every command (`lever-pp-cli postings list stripe`).
Same pattern as greenhouse-pp-cli (jobs list <company>), the seed entry of
the job-boards/ category.

## Local sync design

`sync <company>` fetches all open postings (mode=json) and persists them
under a company-scoped store key (postings:<company>) with durability
verification. Local reads (`--data-source local`) resolve the company from
the path (second segment, since lever paths lead with the resource name)
and never mix companies. Verified with leverdemo (388) and palantir (301)
in an isolated state dir.

## Live dogfood notes

- `doctor --json` reachable: PASS
- `postings list leverdemo` live: 388 postings, mode=json
- `postings get leverdemo <uuid>` live: full posting detail
- `sync leverdemo` then `postings list leverdemo --data-source local`:
  388 rows, source=local, synced_at stamped
- MCP surface: 2 public tools (postings_get, postings_list), runtime
  Cobra-tree parity confirmed by mcp-sync
