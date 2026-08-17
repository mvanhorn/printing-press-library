# Judge.me CLI Brief

## API Identity

- Domain: ecommerce review operations, moderation, storefront widgets, and review-data export.
- Users: ecommerce operations leads who reconcile storefront-visible vs internal review populations; customer-insight analysts who export review corpora for VoC work; integration engineers who maintain review syncs, webhooks, replies, and moderation automations.
- Data profile: 34 documented REST operations. Reviews are the high-gravity entity and include raw unpublished content, reviewer data, moderation state, product association, media flags, and timestamps.

## Reachability Risk

- Decision: PASS.
- Evidence: authenticated `GET /api/v1/reviews/count` returned HTTP 200 with 12,871; authenticated `GET /api/v1/reviews?page=1&per_page=1` returned HTTP 200 with one review.
- Credentials are sent with `X-Api-Token`; the generated CLI must never place or log the private token in a URL.
- Known defect: `GET /reviews` repeats page 100 forever past 10,000 rows. A repeated all-seen page is truncation evidence, never successful completion.

## Top Workflows

1. Ecommerce operations lead runs a complete review sync, verifies the final unique-ID count against `/reviews/count`, and compares published, pending/hidden, and archived populations.
2. Customer-insight analyst exports a date/rating/product slice as JSON or CSV, optionally deduplicated by normalized body hash so syndicated bundle copies do not dominate analysis.
3. Review operations lead ranks products and moderation candidates from the local mirror, then uses explicit apply flags for any publish/hide/reply action.
4. Integration engineer manages webhooks, store settings, reviewers, widgets, and replies through the documented endpoint surface.
5. Agent workflows use `--agent --select` to receive the provenance envelope and only the fields needed for downstream commerce-intel ingestion.

## Table Stakes

- All 34 official OpenAPI operations.
- The public `judge-me` printed CLI's five reputation commands: summary, product hotspots, moderation queue, trust-settings audit, and product evidence.
- Standard `doctor`, feedback, profiles, MCP mirror, JSON/CSV/compact/select output, dry-run, and typed exit codes.
- Pipedream's Judge.me MCP endpoint coverage and the official Hydrogen wrappers' storefront-widget access are covered by the typed endpoints and MCP mirror.

## Data Layer

- Primary entity: `reviews`, keyed by Judge.me review ID.
- Clean schema fields: moderation status, published/hidden flags, rating, product identifiers/title/handle, body, `body_hash`, reviewer JSON, source, media flags, timestamps, and raw JSON.
- Sync strategy: compare total and per-rating counts with `/reviews/count`; pull rating partitions; detect repeated all-seen pages; dedupe by ID; use date windows only as a guarded fallback and reject ignored/ineffective partition filters; commit only after exact final-count equality.
- Search/FTS: title, body, product title/handle, source, and reviewer-facing display fields, with raw reviewer details retained only in the local database.

## User Vision

- Produce `judgeme-pp-cli` for the private RonanRx library.
- The safe full-corpus SQLite mirror is the real deliverable for later commerce-intel ingestion.
- Every count/list/report labels its population. Unique-body analysis must make review syndication visible.
- Mutations exist but default to read-only; real writes require an explicit apply flag and support `--dry-run`.

## Product Thesis

- Name: Judge.me Review Corpus CLI.
- Why it should exist: the documented API surface is easy to wrap, but reliable review intelligence requires defending against silent pagination loops, internal-vs-storefront population ambiguity, and syndicated review bodies.

## Build Priorities

1. Completeness-safe review client and transactional SQLite sync.
2. Population-explicit list/count/export with body-hash deduplication.
3. Preserve full endpoint and reputation parity, while gating every mutation.
4. Full live pull and exact count proof against the current account.
