# Novel-features brainstorm — youtube reprint 20260819-035139 (subagent audit trail)

(Verbatim subagent output; three passes. Saved per novel-features-subagent.md output handling step 3.)

## Customer model

**Marek — the channel operator / competitor analyst**
- **Today (without this CLI):** He keeps vidIQ/ViewStats/OutlierKit tabs open because outlier metrics are paywalled, plus a youtube-data-ss MCP session for raw lookups, plus ad-hoc scripts that re-fetch the same channel uploads every time. He cannot answer "which of competitor X's videos beat X's own median, and by how much" without a subscription, and he cannot answer anything offline.
- **Weekly ritual:** Before picking next video topics, he sweeps a shortlist of competitor channels: pull recent uploads, eyeball view counts against the channel's normal, read the top comments and transcript of anything that spiked.
- **Frustration:** The one number he actually wants — per-video performance vs the channel's own baseline — is exactly the number every free tool omits and every paid tool sells.

**Lena — the niche scout**
- **Today (without this CLI):** She runs manual YouTube searches per candidate term, opens each channel page, and eyeballs upload dates to guess whether a niche's incumbents are accelerating or dying. Results live in a spreadsheet that goes stale immediately. She cannot compare ten channels' upload rhythm or median views side by side from data she owns.
- **Weekly ritual:** Niche evaluation sweep — search terms → collect the channels that keep appearing → judge vitality and saturation → go/no-go on the niche.
- **Frustration:** Vitality is a gut call from scrolling upload dates; there is no honest, repeatable cadence measurement, and re-checking a niche means redoing all the clicking.

**The MCP research agent**
- **Today (without this CLI):** It composes many raw endpoint calls per question, re-derives every aggregate in-context, burns search quota (100 calls/day bucket) on questions the local store could answer, and has no memory of medians it computed yesterday.
- **Weekly ritual:** On-demand multi-step analysis: backfill → outlier scan → forensics on the winners, driven by an operator prompt.
- **Frustration:** No computed, store-backed answers — every statistical claim must be reassembled from raw JSON each session.

## Candidates (pre-cut)

| # | Name | Command | Description | Persona | Source | Kill/keep verdict |
|---|------|---------|-------------|---------|--------|-------------------|
| N1 | Channel outlier report | outliers @handle | Each synced video vs its own channel's median views: ratio, rank, min-sample guard, sample-size disclosure | Marek, agent | (a)+(e) | keep |
| N2 | Upload cadence report | cadence @handle | Median inter-upload gap (de-burst <6h), Mann-Kendall >=6, windowed >=9, Pettitt >=12; describe-never-predict | Lena | (a)+(e) | keep |
| N3 | Title evidence report | titles @handle | Descriptive title stats joined with outlier ratio; descriptive-only below ~100 videos | Marek | (e) | keep |
| N4 | Channel side-by-side | compare-channels @a @b | Median views, cadence gap, outlier count, Shorts/long mix, subs, upload count across 2+ synced channels | Lena | (c) | keep |
| N5 | Growth between snapshots | growth @handle | View/sub/video deltas between dated statistics snapshots; >=2 snapshots guard | Marek | (c) | keep |
| N6 | Chapter extractor | youtube videos-chapters <id|url> | Parse 00:00-style chapters from cached description; optional per-chapter transcript join | Marek, agent | (b) | keep |
| N7 | Shorts/long-form split | format-mix @handle | Per-format medians | Lena | (b) | fold into N4 |
| N8 | Niche sweep orchestrator | sweep <terms...> | search-bulk → channels → backfill → compare chain | Lena | (a) | kill — scope creep |
| N9 | Publish heatmap | heatmap @handle | Day-of-week × hour distribution | Marek | (c) | kill — monthly curiosity |
| N10 | Topic-category map | topic-map @handle | topicDetails distribution | Marek | (b) | kill — no action attached |
| N11 | Premiere/livestream classifier | youtube videos-live-type | liveBroadcastContent labels | — | (b) | kill — no weekly use |
| N12 | Quota ledger | quota-report | Local call/unit ledger | agent | (a) | kill — doctor owns this |
| N13 | Channels-from-search | channels-from-results | Distinct channels across cached search results | Lena | (c) | kill — `sql` covers it |
| N14 | Title/transcript FTS | title-search <term> | FTS over synced titles+transcripts | agent | (c) | kill — framework `search` covers it |

Reprint reconciliation (d): P1 search-bulk prior-keep · P2 videos-transcript prior-keep (restore --format salvage) · P3 videos-embed prior-keep (scores exactly 5/10, no-regress mandate) · P4 videos-related prior-keep · P5 videos-comments prior-keep · P6 channel-uploads prior-keep · P7 playlist-enrich prior-keep · P8 videos-enrich prior-keep · P9 videos-links prior-keep.

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | Channel outlier report | outliers @handle | 10/10 | hand-code | Each video's view ratio vs its channel's median from statistics snapshots in local SQLite (filled by channel backfill), min-sample guard, sample-size disclosure | Brief Workflow 2: the metric vidIQ/ViewStats/OutlierKit sell | Use this command to find which of a channel's videos over- or under-perform that channel's own median. Do NOT use this command for upload-rhythm questions; use 'cadence' instead. Do NOT use it to compare different channels; use 'compare-channels' instead. |
| 2 | Upload cadence report | cadence @handle | 10/10 | hand-code | Median inter-upload gap (bursts <6h collapsed), Mann-Kendall (>=6), windowed (>=9), Pettitt (>=12) from publishedAt series in local SQLite | Brief Workflow 3 vitality check; binding methods | Use this command for upload-frequency, rhythm, and vitality questions about one channel. Do NOT use this command for video-performance questions; use 'outliers' instead. |
| 3 | Title evidence report | titles @handle | 8/10 | hand-code | Joins synced titles with outlier ratios in local SQLite; descriptive-only below ~100 videos | Build Priority 3: outlier ratio feeds titles | Use this command for title-pattern evidence on one synced channel. Do NOT use it to fetch a raw upload list; use 'youtube channel-uploads' instead. |
| 4 | Channel side-by-side | compare-channels @a @b | 8/10 | hand-code | Joins channels+videos across 2+ synced channels: median views, cadence gap, outlier count, Shorts/long mix, totals | Brief Workflow 4: backfill → compare offline | Use this command to put two or more synced channels side by side. Do NOT use it for one channel's internal over/under-performers; use 'outliers' instead. |
| 5 | Growth between snapshots | growth @handle | 8/10 | hand-code | View/sub/video-count deltas between dated snapshot rows (>=2 snapshots guard) | Brief Data Layer snapshot design; API has no history | Use this command for change over time between sync snapshots. Do NOT use it for a same-moment multi-channel comparison; use 'compare-channels' instead. |
| 6 | Chapter extractor | youtube videos-chapters <id|url> | 7/10 | hand-code | Parses 00:00-style chapter markers from cached description; optional per-chapter transcript join | Brief Workflow 5 forensics; chapters = YouTube identity pattern | Use this command to extract chapter markers and per-chapter text from one video. Do NOT use it for the full continuous transcript; use 'youtube videos-transcript' instead. |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| format-mix | Shipped as a column of comparison output, not a command | compare-channels |
| sweep orchestrator | Scope creep — chain of existing commands; agent composes it | compare-channels |
| heatmap | Monthly curiosity; cadence answers the vitality question | cadence |
| topic-map | Descriptive fluff, no packaging action | titles |
| videos-live-type | No weekly use | youtube videos-chapters |
| quota-report | doctor owns credential/cache reporting | — (framework) |
| channels-from-results | One SELECT; `sql` covers it | growth |
| title-search FTS | framework `search --type videos` covers it | — (framework) |

## Reprint verdicts

All nine prior features: **keep** (videos-embed at exactly 5/10 via no-regress mandate; others fit the analyst persona directly). No reframes, no drops.

---
ORCHESTRATOR ADDITION (disclosed at Phase Gate 1.5, not subagent output): `backfill <@handle|channelId>` — the channel-history sync command (uploads-playlist walk + batched videos.list into the store with dated snapshots). Added because 4 of 6 survivors depend on it and the Phase 3 gate needs an explicit approved command row. Score 10/10, hand-code.
