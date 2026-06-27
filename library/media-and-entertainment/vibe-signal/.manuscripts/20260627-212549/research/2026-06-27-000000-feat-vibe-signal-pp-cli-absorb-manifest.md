# Vibe Signal Absorb Manifest

## Absorbed (baseline from the dominant source)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | HN topic search (recency) | HN Algolia | (generated endpoint) hn stories | Recency window via numericFilters, --json/--select |
| 2 | HN topic search (relevance) | HN Algolia | (generated endpoint) hn relevance | Popularity-ranked variant |
| 3 | HN item lookup | HN Firebase | (generated endpoint) hn item | Score + comment count |

## Transcendence (only possible with the aggregator approach)
| # | Feature | Command | Buildability | Why Only We Can Do This |
|---|---------|---------|--------------|-------------------------|
| 1 | Cross-source trend report | report | hand-code | Unifies HN + Techmeme under one topic/window in the local store; no single source API returns this |
| 2 | Raw evidence preservation | evidence | hand-code | Persists verbatim source payloads so synthesis never replaces citable evidence; fetch-on-miss |
| 3 | Source registry | sources list | hand-code | Inspectable registry of wired sources + auth needs |
| 4 | Topic sync to store | sources sync | hand-code | Populates the snapshot store for a topic without rendering a report |

## Notes
- In-process source clients (internal/source/<slug>/), not shell-outs to other catalog CLIs — single installable binary,
  what the generator + dogfood/verify expect (reconciles the original #1387 framing).
- v1 sources: hackernews, techmeme (both no-auth). Product Hunt + YouTube deferred (credentialed v2).
