# Pexels CLI — Novel Features Brainstorm (audit trail)

## Customer model

**Mara — faceless-YouTube / Shorts pipeline operator.** Publishes 5-10 vertical videos/week, each needing 8-15 scene-matched b-roll clips. Today: ad-hoc `requests` + SDK script, eyeballs `video_files[]` for a 1080×1920 clip per scene, re-downloads the same establishing shots across episodes. Frustration: burns her 200/hr quota mid-batch with no warning (rate headers vanish on 429) and has no memory of what she already downloaded.

**Devraj — Go/Python app dev wiring Pexels into a CMS.** Needs deterministic, parseable output + license compliance. Today: wraps the SDK, writes his own attribution builder, post-processes the 8-size `src` blob to one URL. Frustration: every response is a wall of 8 `src` sizes; re-implements the same attribution-HTML; no machine-stable `{data,meta}` envelope.

**Priya — designer / brand researcher curating mood boards.** Pulls cohesive sets from curated feeds + her own/featured collections for client decks. Today: browses pexels.com, downloads one at a time, pastes credits by hand. Frustration: re-searching a theme returns different ordering; can't re-find a photo from two weeks ago; attribution by hand is tedious + error-prone.

**Sasha — ML dataset builder.** Harvests large, deduplicated, attributed image sets. Today: paginated loops with bespoke dedup-by-id JSON; loses progress when the script dies. Frustration: no durable cross-session dedup/provenance ledger; quota exhaustion kills long harvests with no forecast.

## Survivors (post adversarial cut)

| # | Feature | Command | Score | Buildability | Persona | Buildability proof | Long Description |
|---|---------|---------|-------|--------------|---------|--------------------|------------------|
| 1 | Quota forecaster | `quota forecast --resources photos,videos --max-pages N` | 8/10 | hand-code | Mara, Sasha | Reads cached X-Ratelimit-* from local rate ledger, divides planned pages vs remaining budget, returns affordable-pages + reset ETA | Use to check BEFORE a bulk pull whether it fits remaining quota. Do NOT use to show current usage of a single call. |
| 2 | Best-fit resolution picker | `resolve <id> --target-width W --target-height H` | 8/10 | hand-code | Mara, Devraj | Picks smallest photo `src` size / `video_file` meeting target without upscaling, honoring crop vs scale semantics | Use to pick one resolution for a known id against an exact pixel target. Do NOT use to assemble multi-clip b-roll. Do NOT use to merely list all sizes (field projection). |
| 3 | Cross-session dedup + rate-aware checkpointed download | `download "term" --type photo --limit N --max-pages M` | 7/10 | hand-code | Sasha, Mara | Joins results against local downloads ledger by id (skips dupes), watches surfaced X-Ratelimit-Remaining, checkpoints each page, halts gracefully before 429 | Use for repeated/large harvests that must not re-download and risk quota. Do NOT use for one-off single downloads; do NOT use to predict feasibility (use `quota forecast`). |
| 4 | Attribution ledger + SOURCES export | `attribution export --resources photos,videos --csv` | 8/10 | hand-code | Priya, Devraj | Reads local downloads/attribution ledger, emits SOURCES.md + per-file .meta.json sidecars (id, URLs, photographer, avg_color, alt, license, attribution+attribution_html) | none |
| 5 | Offline re-search of synced media | `search "term" --type photo --limit N` (local FTS) | 7/10 | hand-code | Priya, Sasha | Local FTS over synced photo alt/photographer/query + video user.name, live fallback via sync-hint helper | Use to re-find media you already synced with stable ordering. Do NOT use to discover NEW media not yet synced. |
| 6 | Photographer / attribution roll-up | `analytics --type photos --group-by photographer` | 6/10 | spec-emits | Priya, Devraj | Local GROUP BY over synced photos + ledger for credit-balance / licensing review | none |

## Killed candidates
| Feature | Kill reason | Closest survivor |
|---------|-------------|------------------|
| B-roll shot-list assembler | Multi-search orchestration over a phrase file is scope creep; core value covered by `resolve`+`download` looped per shot | resolve |
| Color-cohesion / palette filter | Curated has no color param; avg_color-distance post-filter speculative; color search already absorbed | resolve |
| Collection mirror + diff | "What changed in a collection" speculative (collections static); sync+offline-read covered by offline search | rate-aware download / search |
| Stable reproducible snapshot | Pure duplicate of sync-then-read-locally; subsumed by offline re-search | search |
| Cross-collection duplicate detector | Marginal value over download dedup ledger | download dedup |

Note: C3 (dedup) and C6 (rate-aware checkpoint) from the pre-cut list were merged into one `download` command (survivor #3) since they target the same command.
