---
title: "feat: vibe-signal v1 build-ready spec (no-auth sources) + generation blocker"
type: feat
status: ready-to-generate
date: 2026-06-27
target_repo: mvanhorn/printing-press-library
candidate_cli: vibe-signal
supersedes_scope_of: docs/plans/2026-06-27-003-feat-vibe-signal-cli-candidate-plan.md
related_clis:
  - library/media-and-entertainment/hackernews
  - library/productivity/techmeme
  - library/marketing/producthunt
  - library/media-and-entertainment/youtube
generation_recipe: aggregator-pattern (cli-printing-press)
dominant_source: hacker-news-algolia
---

# feat: vibe-signal v1 build-ready spec + generation blocker

## Summary

PR #1387 (`2026-06-27-003-...candidate-plan.md`) proposed `vibe-signal-pp-cli` as a
composed editorial trend-research CLI and explicitly deferred the build: *"Actual
implementation should happen through `/printing-press vibe-signal` after choosing
initial source adapters and fixtures."*

This document **makes those choices** so the run is executable, and records the
**environmental blocker** that prevented running the canonical Printing Press flow
in this session. It is intentionally narrow: it does not hand-build a generated CLI
tree (forbidden by `AGENTS.md`, and `verify-library-conventions.yml` would reject
it). It converts the open-ended proposal into a single, build-ready v1 the generator
can consume, grounded in **measured** source reachability rather than assumptions.

## Generation blocker (why this is a design PR, not a generated CLI)

The canonical path is `/printing-press vibe-signal` (research → generate → dogfood →
verify → archive → publish), which requires the `cli-printing-press` generator
binary. In this environment the generator **cannot be installed**:

```
go install github.com/mvanhorn/cli-printing-press/v4/cmd/cli-printing-press@latest
  → github.com/mvanhorn/cli-printing-press/v4@v4.27.0 requires go >= 1.26.4; switching to go1.26.4
  → compile: writing output: ... no space left on device
```

- The host data volume is at **100% capacity** (`df` shows ~0.8–1.5 GiB free of
  228 GiB). Freeing the regenerable Go build cache (`go clean -cache`, ~591 MB) was
  not enough — the compile still exhausts scratch space.
- The module requires **go ≥ 1.26.4** (installed: 1.26.1), so `go install` must also
  auto-download a new Go toolchain, compounding the space pressure.
- Deleting the user's ~180 GiB of data to make room is out of scope and not
  authorized.

Without the binary, the skill's own preflight hard-stops
(`[setup-error] cli-printing-press binary not found`), and every
generate/dogfood/verify/shipcheck/publish step is unavailable. **Unblock by freeing
disk (target ≥ ~4 GiB headroom) and/or installing Go ≥ 1.26.4**, then run the recipe
below.

## Reachability evidence (measured 2026-06-27)

| Source | Endpoint probed | Status | Auth | v1 verdict |
|---|---|---|---|---|
| Hacker News (Algolia) | `GET hn.algolia.com/api/v1/search` | **200** | none | **Primary** |
| Hacker News (Firebase) | `GET hacker-news.firebaseio.com/v0/topstories.json`, `/v0/item/{id}.json` | **200** | none | **Primary (enrich)** |
| Techmeme | `GET www.techmeme.com/feed.xml` (RSS) | **200** | none | **Secondary** |
| Product Hunt | `POST api.producthunt.com/v2/api/graphql` | 404 on GET | OAuth token required | **Defer to v2** |
| YouTube Data API v3 | `GET googleapis.com/youtube/v3/search` | 403 (no key) | API key required | **Defer to v2** |

Verified response shapes (drive the unified model + draft spec below):

- **HN Algolia hit:** `objectID, title, url, author, points, num_comments,
  created_at, created_at_i, story_id, story_text` (+ `_tags`, `_highlightResult`).
  Supports topic search over a recency window via
  `numericFilters=created_at_i>{epoch}`. This is the richest no-auth search surface.
- **HN Firebase item:** `id, type, by, time, title, url, score, descendants, kids`.
- **Techmeme RSS item:** `title, link, description, pubDate, guid` (15 items/feed).

**v1 scope decision:** ship the two **no-auth** sources (Hacker News + Techmeme).
The task's YouTube caveat ("if API path is feasible") resolves to **defer** — the
Data API requires a key, so it is not a low-friction v1 source. Product Hunt
likewise needs an OAuth token. Both are clean v2 additions once a credentialed path
is approved.

## Generation recipe (canonical aggregator pattern)

vibe-signal is a multi-source aggregator, so it follows
`skills/printing-press/references/aggregator-pattern.md`: **generate a baseline from
the dominant source, then hand-author secondary source clients.**

1. **Primary / baseline:** generate from a hand-authored internal-YAML spec for the
   HN Algolia + Firebase surface (Appendix A). `auth.type: none`.
   ```
   /printing-press vibe-signal
   # at the spec gate, supply the internal YAML in Appendix A
   # cli-printing-press generate --spec vibe-signal.yaml --name vibe-signal \
   #   --output $CLI_WORK_DIR --research-dir $API_RUN_DIR \
   #   --category media-and-entertainment --force --lenient --validate
   ```
2. **Secondary (Phase 3 hand-code):** add `internal/source/techmeme/` (RSS client)
   under the aggregator layer. Each source gets its own `cliutil.AdaptiveLimiter`
   and typed 429 behavior (`references/per-source-rate-limiting.md`).
3. **Editorial layer (Phase 3 novel commands):** `report`, `evidence`, and the
   `sources` tree, querying the unified store — not the source packages directly.

### Reconciliation with PR #1387 (decision)

PR #1387 imagined adapters that **shell out to installed catalog CLIs**
(`hackernews-pp-cli`, `youtube-pp-cli`, …) with API/fixture fallback. The canonical
aggregator pattern instead uses **in-process source clients** under
`internal/source/<slug>/`. v1 adopts the in-process model because it (a) is what the
generator + dogfood/verify gates expect, (b) keeps vibe-signal a single installable
binary with no runtime dependency on other CLIs being installed, and (c) avoids
brittle subprocess parsing. The "compose the catalog" framing remains the *thesis*;
the *implementation* is in-process clients that hit the same upstreams.

## v1 source contract

Per the aggregator reference, define the shared entity + `Source` interface outside
generator-owned packages (`internal/source/source.go`). The domain entity is a
`Signal` (an observed item of conversation), not a generic `Work`:

```go
type Signal struct {
    Source      string    // "hackernews" | "techmeme"
    ID          string    // source-native id (HN objectID / Techmeme guid)
    Title       string
    URL         string    // canonical link to the item
    Author      string    // optional (HN author); empty for Techmeme
    Points      int       // HN points; 0 when not applicable
    Comments    int       // HN num_comments/descendants; 0 when n/a
    PublishedAt time.Time // source timestamp (recency window filtering)
    Excerpt     string    // story_text / RSS description, cleaned via cliutil.CleanText
    RawJSON     string    // verbatim source payload — preserves raw evidence (R2/R3)
}

type SyncOptions struct {
    Query  string    // topic; Techmeme ignores (river feed) and is tagged accordingly
    Since  time.Time // recency window lower bound
    Limit  int
}

type Source interface {
    Name() string
    Description() string
    AuthRequired() bool            // false for both v1 sources
    Sync(context.Context, SyncOptions) ([]Signal, error)
}
```

`RawJSON` is load-bearing: requirement **R2/R3** (preserve raw evidence, never
collapse sources into unsupported narrative) is satisfied by persisting the verbatim
payload and surfacing it through `evidence`.

## Unified store / snapshot schema

One lossy cross-source table keyed by `(source, source_id)` (collision-safe), plus a
run table so a later run can answer "what changed" (**R4**). FTS5 only if added with
the generator's contentful-table + trigger shape.

```sql
CREATE TABLE signals (
    source      TEXT NOT NULL,
    source_id   TEXT NOT NULL,
    query       TEXT NOT NULL,           -- the topic this row was captured under
    title       TEXT NOT NULL,
    url         TEXT,
    author      TEXT,
    points      INTEGER DEFAULT 0,
    comments    INTEGER DEFAULT 0,
    published_at TEXT,                    -- RFC3339
    excerpt     TEXT,
    raw_json    TEXT,                     -- raw evidence (R2/R3)
    run_id      TEXT NOT NULL,            -- snapshot grouping (R4)
    PRIMARY KEY (source, source_id, query, run_id)
);

CREATE TABLE runs (
    run_id      TEXT PRIMARY KEY,
    query       TEXT NOT NULL,
    window_days INTEGER NOT NULL,
    created_at  TEXT NOT NULL,
    coverage_json TEXT                    -- per-source ok/partial/failed (R7)
);
```

## Fixture format (fixture-backed tests, R8)

Each source ships a captured-response fixture so adapter tests run offline and
deterministically. Store under `internal/source/<slug>/testdata/`:

```
internal/source/hackernews/testdata/algolia_search_ai-agents.json   # verbatim Algolia response
internal/source/hackernews/testdata/firebase_item_8863.json         # verbatim Firebase item
internal/source/techmeme/testdata/feed.xml                          # verbatim RSS
```

Table-driven test contract per source (one happy-path test per exported func, per
the Phase 3 Completion Gate's pure-logic-package rule):

1. Load fixture from `testdata/`.
2. Run the source's parse/normalize into `[]Signal`.
3. Assert field mapping (e.g. HN `objectID→ID`, `num_comments→Comments`,
   `created_at_i→PublishedAt`; Techmeme `guid→ID`, `pubDate→PublishedAt`).
4. Assert `RawJSON` is non-empty and round-trips.
5. Negative: a fixture with a missing/empty field maps to the zero value, not a
   dropped row (NULL-safe).

## v1 command surface (small first version)

Ship the read-only core; defer the rest to keep v1 small (the task asked for "a
small first version"):

| Command | v1 | Notes |
|---|---|---|
| `vibe-signal report "<topic>" --window 30d` | ✅ | sync both sources for the topic, snapshot to store, print themes + per-source coverage + representative links + JSON companion (`--json`) |
| `vibe-signal evidence "<topic>" --source hackernews --limit 20` | ✅ | raw evidence rows for a topic/source from the store (R2) |
| `vibe-signal sources list` | ✅ | registered sources + auth needs; `mcp:read-only` |
| `vibe-signal sources sync [--source <name>]` | ✅ | populate the store |
| `vibe-signal compare A B --window 14d` | ⏭ v2 | needs ≥2 snapshots |
| `vibe-signal watch ...` / `briefs ...` | ⏭ v2 | scheduling layer |

`report` must separate **observed evidence** from **synthesis** (R3) and emit a
source-coverage table noting any source that was rate-limited or unavailable (R7).
All read commands annotate `mcp:read-only`.

## Validation gates the build must clear (unchanged from #1387)

`go build ./...`, `go vet ./...`, `govulncheck ./...`, `go test ./...`,
fixture-backed adapter tests, `cli-printing-press shipcheck`, plus a live dogfood of
`report`/`evidence` against the no-auth sources (no credentials needed — both v1
sources are freely testable, so Phase 5 is mandatory, not skippable).

## Exact next action (when unblocked)

1. Free disk to ≥ ~4 GiB headroom; ensure Go ≥ 1.26.4 (or let the toolchain
   auto-download once there is room).
2. `go install github.com/mvanhorn/cli-printing-press/v4/cmd/cli-printing-press@latest`
   and verify `cli-printing-press --version`.
3. Run `/printing-press vibe-signal`; at the spec gate supply Appendix A; at the
   Phase 1.5 absorb gate confirm the v1 scope above; build the Techmeme source +
   editorial layer in Phase 3; ship via `/printing-press-publish vibe-signal`.

---

## Appendix A — draft internal-YAML spec for the HN baseline

> **Draft.** Validate with `cli-printing-press generate --validate` before relying
> on it; field/param shapes are grounded in the measured responses above but have
> not been run through the generator (blocked this session).

```yaml
name: vibe-signal
description: "Find what people are saying now about a topic across low-friction sources, with raw evidence preserved."
version: "0.1.0"
base_url: "https://hn.algolia.com/api/v1"
health_check_path: "/search?query=test&hitsPerPage=1"
category: media-and-entertainment
auth:
  type: none
required_headers:
  - name: User-Agent
    value: "vibe-signal-pp-cli (+https://github.com/mvanhorn/printing-press-library)"

resources:
  search:
    description: "Search Hacker News stories and comments (Algolia)"
    endpoints:
      stories:
        method: GET
        path: "/search_by_date"
        description: "Search HN stories by recency for a topic"
        example: "  vibe-signal-pp-cli search stories --query \"AI agents\" --tags story --hits 20"
        happy_args: "--query=AI agents --tags=story --hits=5"
        params:
          - name: query
            type: string
            required: true
            description: "Topic to search for"
          - name: tags
            type: string
            required: false
            default: "story"
            description: "Algolia tag filter (story, comment, show_hn, ...)"
          - name: numericFilters
            flag_name: since-epoch-filter
            type: string
            required: false
            description: "Recency filter, e.g. created_at_i>1719446400"
          - name: hitsPerPage
            flag_name: hits
            type: int
            required: false
            default: 20
            description: "Results per page (max 1000)"
        response:
          type: object
          item: AlgoliaSearchResult
        response_path: hits

  item:
    description: "Fetch a Hacker News item by id (Firebase)"
    base_url: "https://hacker-news.firebaseio.com/v0"
    endpoints:
      get:
        method: GET
        path: "/item/{id}.json"
        description: "Get a single HN item (story/comment) with score and comment count"
        example: "  vibe-signal-pp-cli item get 8863"
        params:
          - name: id
            type: int
            required: true
            positional: true
            description: "HN item id"
        response:
          type: object
          item: HNItem

types:
  AlgoliaSearchResult:
    fields:
      - name: objectID
        type: string
      - name: title
        type: string
      - name: url
        type: string
      - name: author
        type: string
      - name: points
        type: int
      - name: num_comments
        type: int
      - name: created_at_i
        type: int
      - name: story_text
        type: string
  HNItem:
    fields:
      - name: id
        type: int
      - name: type
        type: string
      - name: by
        type: string
      - name: title
        type: string
      - name: url
        type: string
      - name: score
        type: int
      - name: descendants
        type: int
      - name: time
        type: int
```
