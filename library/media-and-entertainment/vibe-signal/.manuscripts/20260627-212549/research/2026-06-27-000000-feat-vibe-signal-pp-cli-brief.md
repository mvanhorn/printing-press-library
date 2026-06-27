# Vibe Signal CLI Brief

## API Identity
- Domain: editorial trend research — "what are people saying now about X" across low-friction sources.
- Users: writers, researchers, and agents who need the current cross-source conversation with citable evidence.
- Data profile: short-lived conversation items (HN stories/comments, Techmeme headlines), persisted as snapshots for recency deltas.

## Reachability Risk
- None. v1 sources are no-auth and were probed live 2026-06-27:
  - Hacker News Algolia `GET hn.algolia.com/api/v1/search` → 200
  - Hacker News Firebase `GET hacker-news.firebaseio.com/v0/item/{id}.json` → 200
  - Techmeme RSS `GET www.techmeme.com/feed.xml` → 200
- Product Hunt (OAuth) and YouTube (API key) need credentials → deferred to a credentialed v2.

## Top Workflows
1. Cross-source signal report for a topic over a recency window.
2. Pull the raw, cited evidence behind a topic from a chosen source.
3. Inspect which sources are wired and their auth needs.

## Table Stakes
- Topic search (HN Algolia), recency windowing, JSON + human output, per-source coverage notes.

## Data Layer
- Primary entities: `signals` (unified cross-source items), `runs` (snapshots for deltas).
- Search/recency: query + published_at; snapshot grouping by run_id.

## Product Thesis
- Name: Vibe Signal.
- Why it should exist: the catalog has strong single-source CLIs (hackernews, techmeme). vibe-signal is the missing
  editorial workflow layer that asks one question across them and returns a repeatable, cited, recency-aware report —
  the aggregator pattern composing the catalog rather than another single-source scraper.

## Build Priorities
1. Aggregator source layer (Source contract + registry) with in-process HN + Techmeme clients.
2. Unified SQLite signals/runs store with raw-evidence preservation.
3. report / evidence / sources commands with fixture-backed adapter tests.
