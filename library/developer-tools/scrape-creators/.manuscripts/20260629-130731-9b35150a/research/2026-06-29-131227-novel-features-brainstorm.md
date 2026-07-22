# Scrape Creators — Novel Features Brainstorm (audit trail)

Subagent output (Phase 1.5c.5), 2026-06-29. Reprint reconciliation of prior 13 features.

## Customer model

**Priya — agency competitive-intelligence analyst.** Tracks 8-10 client competitors' paid social by hand, screenshotting Facebook Ad Library + Google Ads Transparency into a Monday deck; no view into TikTok/LinkedIn ad spend. Weekly ritual: "what's new in their ads" sweep — which creatives launched/pulled/changed. Frustration: no diff anywhere; re-reads the whole current ad set each week, misses pulled creatives.

**Marcus — influencer-marketing manager.** Vets creator shortlists, pulling follower counts + recent posts one platform at a time into a spreadsheet. Weekly ritual: shortlist triage — who over-performs vs. who just has a big follower number. Frustration: raw follower count lies; needs engagement-rate + viral outliers, and every-platform footprint before outreach (11 lookups/creator today).

**Dana — content/RAG researcher.** Builds keyword/topic corpora from transcripts, fetching one at a time into text files. Weekly ritual: "did anyone say X" sweeps across the transcript pile. Frustration: each transcript costs a credit and returns one per call with no search; wants to re-query paid transcripts forever, offline, cross-platform.

**Sam — growth marketer / trend scout.** Watches whether a hashtag/topic is cresting before recommending a bet, checking TikTok/YouTube manually. Weekly ritual: momentum check on 3-4 tracked topics + follower-trajectory check on partner creators. Frustration: a single call returns a count not a direction; can't tell rising from dying or which platform leads.

## Survivors (8, all hand-code, all >= 8/10)

| # | Feature | Command | Score | Build | Data | Persona |
|---|---------|---------|-------|-------|------|---------|
| 1 | Cross-platform presence matrix | `creator find <handle>` | 10/10 | hand-code | live fan-out (+cache) | Marcus |
| 2 | Multi-creator comparison | `creator compare <a> <b>...` | 9/10 | hand-code | local computed | Marcus |
| 3 | Engagement spike detector | `content spikes <handle>` | 8/10 | hand-code | local computed | Marcus |
| 4 | Transcript full-text search | `transcripts search <q>` | 10/10 | hand-code | local FTS5 | Dana |
| 5 | Trend triangulation | `trends triangulate <q>` | 9/10 | hand-code | computed | Sam |
| 6 | Follower growth tracker | `creator track <handle>` | 8/10 | hand-code | local (+live append) | Sam |
| 7 | Brand ad campaign monitor | `ads monitor <brand>` | 10/10 | hand-code | computed (live 4-lib + local diff) | Priya |
| 8 | Credit burn projection | `account budget` | 9/10 | hand-code | computed (local + live usage) | Priya |

## Killed candidates

| Feature | Kill reason | Closest sibling |
|---------|-------------|-----------------|
| `ads search` | One-shot search is just the monitor's first run; splitting one ad surface across two commands. | `ads monitor` |
| `content cadence` | Day/hour posting pattern is a niche low-weekly-use slice. | `creator compare` |
| `content analyze` | Same baseline math as spike detection, duller output. | `content spikes` |
| `trends delta` | Single-platform count delta is a subset of triangulation with `--days`. | `trends triangulate` |
| `bio resolve` | Lead-gen link unfurling is tangential to the four core personas. | `creator find` |
| `content sponsors` | Mechanical version is a saved transcript FTS + already-typed paid-promo flag; no new leverage. | `transcripts search` |
| `mentions watch` | A persisted query over synced transcripts/comments; no new computation. | `transcripts search` |
| `trends music` | New Spotify/SoundCloud platforms are just more endpoints in the existing fan-out. | `trends triangulate` |

## Reprint verdicts (prior 13)

- **Keep (7):** creator find, trends triangulate, transcripts search, content spikes, creator compare, creator track, account budget.
- **Reframe (1):** ads monitor — drop Reddit ad library, add TikTok ad library, absorb one-shot `ads search` as first-run.
- **Drop (1 reframe-merge + 4):** ads search (folded into ads monitor); content cadence; content analyze (subsumed by content spikes); trends delta (folded into trends triangulate --days); bio resolve. All drops gate-overridable.
