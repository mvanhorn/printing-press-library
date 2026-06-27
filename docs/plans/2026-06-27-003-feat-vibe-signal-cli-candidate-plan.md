---
title: "feat: Add vibe-signal CLI candidate for repeatable editorial trend research"
type: feat
status: proposed
date: 2026-06-27
target_repo: mvanhorn/printing-press-library
candidate_cli: vibe-signal
related_clis:
  - library/social-and-messaging/x-twitter
  - library/media-and-entertainment/youtube
  - library/media-and-entertainment/hackernews
  - library/productivity/techmeme
  - library/marketing/producthunt
  - library/media-and-entertainment/podscan
  - library/media-and-entertainment/podcastindex
  - library/developer-tools/scrape-creators
---

# feat: Add vibe-signal CLI candidate for repeatable editorial trend research

## Summary

The catalog has strong source-specific CLIs for social, media, podcasts, and tech-news surfaces. It is missing the editorial workflow layer that asks one question across those sources and returns a repeatable, cited, recency-aware signal report.

This plan proposes `vibe-signal-pp-cli`: a CLI for finding what people are saying now about a topic, product, company, person, or market, with source-specific evidence preserved and trend deltas tracked over time.

## Problem Frame

The existing catalog can pull from many useful surfaces:

- `x-twitter`
- `youtube`
- `hackernews`
- `techmeme`
- `producthunt`
- `podscan`
- `podcastindex`
- `scrape-creators`
- `substack`
- `medium-reader`

But a publication or research agent does not want ten unrelated source dumps. It wants:

- What is the current conversation?
- What changed in the last 7/30 days?
- Which claims are backed by actual posts, comments, videos, or transcripts?
- Which sources disagree?
- Which examples are worth quoting or linking?
- Which topic is heating up enough to become an article?

`vibe-signal` should be a composed workflow CLI, not a replacement for the source CLIs.

## Proposed CLI Shape

```bash
vibe-signal-pp-cli report "AI browser agents" --window 30d
vibe-signal-pp-cli compare "Perplexity Comet" "Dia browser" --window 14d
vibe-signal-pp-cli watch add "AI coding agents" --sources x,youtube,hackernews,producthunt
vibe-signal-pp-cli watch run --since last
vibe-signal-pp-cli evidence "AI coding agents" --source youtube --limit 20
vibe-signal-pp-cli briefs daily --watchlist editorial-topics.yaml
```

## Requirements Trace

- R1. Query multiple source CLIs or APIs for a topic over a bounded recency window.
- R2. Preserve raw evidence pointers: post URL, video URL, comment URL, transcript segment, source timestamp, author/channel when available.
- R3. Separate observed evidence from synthesis. The CLI should never collapse sources into unsupported narrative.
- R4. Track historical snapshots in SQLite so a later run can answer what changed.
- R5. Support `report`, `compare`, `watch`, `evidence`, and `briefs` workflows.
- R6. Emit compact Markdown and JSON, with stable sections for agents and publications.
- R7. Include source coverage and failure notes when APIs are unavailable, rate-limited, or auth-gated.
- R8. Support source adapters so the CLI can use existing catalog CLIs when installed, direct APIs when configured, or saved fixtures for tests.

## Output Contract

`report` should produce:

- Query, time window, generated_at.
- Source coverage table.
- Top claims or themes, each backed by source evidence.
- Rising/falling signals compared with prior snapshot when available.
- Representative links.
- Open questions and weak evidence areas.
- JSON companion output for downstream ranking.

Example shape:

```text
Topic: AI browser agents
Window: 30d

Themes:
1. Users want browser agents that can complete authenticated workflows.
   Evidence: X posts, HN comments, YouTube transcript segments.
2. Reliability complaints cluster around login/session handling.
   Evidence: HN comments, GitHub issues, YouTube comments.

Coverage:
- X: ok, 122 posts
- YouTube: ok, 18 videos, 241 comments
- HN: ok, 9 threads
- ProductHunt: partial, 2 launches
```

## Implementation Units

- [ ] Unit 1: Define source adapter interface and fixture format.
- [ ] Unit 2: Add adapters for `hackernews`, `youtube`, `producthunt`, and `techmeme` first because they are lower-friction than authenticated X.
- [ ] Unit 3: Add optional `x-twitter` and `scrape-creators` adapters when credentials are present.
- [ ] Unit 4: Add local SQLite snapshot store with query, source, item, evidence, and run tables.
- [ ] Unit 5: Add `report` and `evidence` commands.
- [ ] Unit 6: Add `compare` and `watch` commands.
- [ ] Unit 7: Add Markdown and JSON output modes with source coverage notes.
- [ ] Unit 8: Dogfood against three topics: one AI tooling topic, one consumer product topic, one company/person topic.

## Source Strategy

Prefer installed source CLIs when available:

- `hackernews-pp-cli`
- `youtube-pp-cli`
- `producthunt-pp-cli`
- `techmeme-pp-cli`
- `x-twitter-pp-cli`
- `podscan-pp-cli`

Fallback to direct APIs only when the source CLI is missing or cannot provide the needed data. This keeps `vibe-signal` small and makes it a workflow layer over the catalog rather than a giant scraper.

## Validation

- Fixture-backed tests for each adapter.
- `go test ./...`
- `go vet ./...`
- `go build ./...`
- Live dogfood with source coverage notes.
- Manuscripts containing at least one raw evidence bundle and one final report.

## Why This Belongs In The Library

Printing Press already has the source CLIs. `vibe-signal` would prove the next layer: catalog CLIs composed into an agent-native editorial workflow. It creates a repeatable research loop rather than another one-off scraper.
