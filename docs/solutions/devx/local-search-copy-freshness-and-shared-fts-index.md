---
name: local-search-copy-freshness-and-shared-fts-index
title: A removed search API turned into a local FTS copy that served stale or partial data as current
date: 2026-10-22
problem_type: bug
category: devx
module: digg CLI search and sync
component: library/media-and-entertainment/digg/internal/diggstore/feed.go, library/media-and-entertainment/digg/internal/diggstore/store.go, library/media-and-entertainment/digg/internal/cli/digg_sync.go, library/media-and-entertainment/digg/internal/cli/digg_commands.go
root_cause: Digg removed its search and pipeline-status endpoints; the replacement local copy then marked capped walks fresh, rebuilt a shared FTS index from blank incoming fields, matched columns the help did not promise, and returned empty success when it had no usable data
resolution_type: fix
tags: [printing-press, digg, fts5, sqlite, sync, freshness, pagination, greptile, fail-closed]
---

# A removed search API turned into a local FTS copy that served stale or partial data as current

## Problem

`digg-pp-cli search` called `/api/search/stories`, which Digg removed in October 2026 along with `/api/trending/status`. Both returned 404; this was not a bot block. Search fell back to an empty or stale local FTS table and exited 0 with no results, so last30days saw zero Digg items without any error (issue mvanhorn/printing-press-library#2317). The repair, in PR mvanhorn/printing-press-library#2354 (open as of this writing), made search read a local copy of Digg's story feed. Building that local copy exposed four further traps, each of which silently served wrong data.

## Symptoms

- `search` returned `count: 0` and exit 0 while Digg had matching stories.
- `sync` failed on every run against the removed status endpoint.
- During the repair, review found:
  - A `sync --max-pages 1` made search report the section as fresh.
  - Stories went unsearchable after a second sync.
  - Words found only in a story's source title matched.
  - `--data-source local` on an empty store returned an empty success.

## What Didn't Work

- Assuming bot blocking. The removed endpoints return 404 (issue mvanhorn/printing-press-library#2317), and the live site's own bundles call `/api/feed/continuous` instead.
- Looking for a replacement search endpoint. `/api/feed/continuous` has no query parameter and no page-size parameter. It serves 20 stories per page with a cursor and rejects unknown parameters with HTTP 400 (`library/media-and-entertainment/digg/internal/client/feed_continuous.go:20-22`). Search therefore has to be local.
- Recording freshness whenever a sync walk returned without error. A walk stopped by `--max-pages` returns cleanly, so search treated a one-page sync as a complete 30-day copy.
- Skipping the FTS reindex when the stored text was unchanged. This was an optimization added in the same PR so deep syncs did not slow down: the per-row delete scans the whole index because `cluster_id` is an UNINDEXED FTS column. It was correct in isolation but exposed a second writer's bug (see below).

## Solution

1. Search reads a local FTS copy that `sync` builds by cursor-walking each section's feed. When a requested section has no completed sync within `searchFreshFor` (6 hours, `library/media-and-entertainment/digg/internal/cli/digg_commands.go:742`), search first fetches at most `searchTopUpPages` (8, `digg_commands.go:745`) live pages. The JSON result then carries `meta.stale` and `meta.source`, so callers can tell a current answer from a partial one.

2. A capped walk counts as fresh only if it reached about 90% of the way back through its range (`library/media-and-entertainment/digg/internal/cli/digg_sync.go:224`, `cappedCoverage` at `digg_sync.go:309`):

   ```go
   covered := res.Complete || cappedWalkCovers(feedRange, oldest, now)
   ```

3. Every writer to a shared FTS index rebuilds the row from the stored table row, never from its own incoming fields. The HTML-scrape upsert keeps saved text when a scrape leaves a field blank (`COALESCE(NULLIF(excluded.title,''), digg_clusters.title)`). Before the fix it inserted the FTS row from the blank incoming struct. The feed writer then saw unchanged stored text and skipped its reindex (`library/media-and-entertainment/digg/internal/diggstore/feed.go:159`), so the story stayed unsearchable for good. Fixed form (`library/media-and-entertainment/digg/internal/diggstore/store.go:390`):

   ```sql
   INSERT INTO digg_clusters_fts (cluster_id, cluster_url_id, label, title, tldr, source_title)
   SELECT cluster_id, cluster_url_id, label, title, tldr, source_title FROM digg_clusters WHERE cluster_id = ?
   ```

4. Restrict FTS5 matching to the documented columns with a column filter on each quoted term (`feed.go:325`). The shared index also holds `label` and `source_title`:

   ```go
   parts = append(parts, `{title tldr}:"`+w+`"`)
   ```

5. Fail closed on missing data, but not on empty windows. Search exits non-zero with a `sync` hint only when all three of these hold (`digg_commands.go:911`):
   - the section is stale or was never synced;
   - nothing is stored inside the `--since` window (`CountTopicStories` is window-scoped, `feed.go:297`);
   - no live pages arrived.

   A section synced within the freshness window that has nothing in `--since` returns an empty success, because that is a real "no matches".

## Why This Works

A local copy has two states a remote API never had: how complete the copy is, and how current it is. Every path that returns results has to account for both. The walk-coverage rule ties "fresh" to evidence that the copy spans the range. The window-scoped count plus the freshness flag separates "Digg has nothing" from "we have nothing". Rebuilding FTS rows from the stored row makes the index a pure function of the table, so another writer's skip-if-unchanged check stays sound.

## Prevention

- When a vendor removes a search API, look for the feed the live site's own JavaScript calls before concluding the data is gone.
- Any optimization that skips work "because the stored value is unchanged" assumes every other writer keeps derived state, such as an FTS index or a cache, in sync with that stored value. Check every writer before adding the skip.
- Tests that pin each trap:
  - `TestSyncMaxPagesCapsWalk` and `TestSyncCappedWalkReachingRangeStartCountsAsSynced` (capped-walk freshness)
  - `TestScrapeWithBlankTextKeepsFeedStoryIndexed` (scrape, then feed write, then search)
  - `TestSearchFeedStoriesIgnoresSourceTitleAndLabel`
  - `TestSearchDataSourceLocalEmptyStoreErrors` and `TestSearchStaleStoreOutsideWindowWithFailingFeedErrors`; `TestSearchAgeComesFromWallClock` keeps the fresh-but-empty case an empty success.
- Live smoke check: run `sync --max-pages 1`, then `search --agent`. The result must show `stale: true`.

## Related Issues

- mvanhorn/printing-press-library#2317: the original 404 report, with the empty fallback hiding it.
- mvanhorn/printing-press-library#1374: same CLI and same failure class (Digg-side drift yields 0 records), different command.
- [static-site-cli-fail-closed.md](static-site-cli-fail-closed.md): the same fail-closed rule for parser drift in a static-site CLI.
- [full-text-fanout-partial-failures.md](full-text-fanout-partial-failures.md): keep partial results when one source fails. This doc adds the counterpart: label partial data as partial (`meta.stale`).
