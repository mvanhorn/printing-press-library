---
type: ecosystem-absorb-manifest
api: keyword-planner
created: 2026-09-08
public_projection: true
---

# Reuse decision

The public library was source-inspected at commit 60351630b3d94c016faa7bfb65102e109da086c9. A focused new press was selected because the direct Google Ads press covers broad v22 advertising management, while this product requires a v25 immutable keyword-evidence portfolio.

| Candidate | Useful reference | Scope boundary |
|---|---|---|
| Google Ads press | OAuth/header conventions, customer requests, JSON output | Inspected keyword commands submit one POST with a caller-supplied page token; immutable raw/monthly portfolio behavior is absent from those commands. |
| DataForSEO press | Batch/cost UX, saved tasks and deltas | Different provider and authentication; full raw-envelope preservation was not established. |
| Semrush press | Tagged snapshots and comparisons | Different data source; no claim of Google monthly-evidence equivalence. |
| Ahrefs press | Volume-history presentation | Different data source and coverage. |
| eRank / EverBee presses | Saved keyword signal and freshness UX | Etsy-oriented tools, used only as design references. |

The direct transport is a narrow implementation of the official v25 REST contract. The design adds automatic paging, ordered historical batching, raw-before-decode persistence, immutable snapshots, typed nullable monthly rows and offline evidence views.

The inspected bundles use Apache-2.0 licensing. Google's official client examples are behavioral references. A community Go protobuf client and a v24 Keyword Planner MCP were considered; neither was added as a runtime dependency because the scoped REST transport was sufficient.

This comparison is source/feature research. Competitor binaries were not installed or benchmarked. It does not establish superior latency, overall reliability or broader advertising coverage.

[Canonical library](https://github.com/mvanhorn/printing-press-library/tree/60351630b3d94c016faa7bfb65102e109da086c9/library/marketing)
