# Artistly CLI — Absorb Manifest

**Landscape:** Research found NO existing Artistly CLI, MCP server, Claude plugin/skill, npm package, or PyPI wrapper. This is the **first tool ever** for app.artistly.ai. The "absorbed" set below is therefore the Artistly web app's own capabilities the CLI mirrors (and beats with offline + agent-native + scriptable). Generation is a paid/quota'd write operation (~400/day cap, failures count), so the transcendence layer leans on leverage the manual web flow can't offer: batch, automation, offline history search, presets, bulk export, quota guardrails.

## Absorbed (match or beat the web app)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Text-to-image generation | Artistly web app (`POST /ai/{feature}/store`) | `(behavior in artistly-pp-cli generate)` POST /ai/{feature}/store with Inertia+CSRF headers, 302=queued | Scriptable, `--wait` blocks until done, `--download` to disk, `--json` design records |
| 2 | List my designs | Artistly web app (`/fetch-personal-designs`) | `artistly-pp-cli designs list` | Offline mirror, `--json`/`--select`/`--csv`, filter by folder/status, FTS |
| 3 | Designs by folder | Artistly web app (`/designs-by-folder`) | `(behavior in artistly-pp-cli designs list --folder)` | Same, folder-scoped |
| 4 | Download a design | Artistly web app (`/{uuid}/download`, CDN urls) | `artistly-pp-cli designs download` | Direct-to-disk, multi-image, filename templating |
| 5 | Delete designs | Artistly web app (`POST /delete-designs`) | `artistly-pp-cli designs delete` | Batch ids, `--dry-run`, typed exit codes |
| 6 | Move designs to folder | Artistly web app (`POST /move-designs`, `/change-folder`) | `artistly-pp-cli designs move` | Batch, `--dry-run` |
| 7 | Folder management | Artistly web app (`POST/PUT/DELETE /designs/folder`) | `artistly-pp-cli folders` (list/create/rename/remove) | Scriptable folder CRUD |
| 8 | Enhance a prompt | Artistly web app (`POST /prompt/enhance`) | `artistly-pp-cli prompt enhance` | Pipe-friendly, `--json` |
| 9 | Image-to-prompt (extract) | Artistly web app (`POST /prompt/extract`) | `artistly-pp-cli prompt extract` | Pipe-friendly |
| 10 | Browse style catalog | Artistly web app (Inertia shared props) | `artistly-pp-cli styles list` | Offline, `--json`, fuzzy `--match` to resolve a human term to the exact style value |
| 11 | Quota / usage status | Artistly web app (shared props) | `(behavior in artistly-pp-cli doctor)` | Surfaces remaining daily generations + concurrent limit |

Notes:
- `generate` is the headline; all generators (text-to-image, inpaint, outpaint, product images, logo, story books, flipbooks, puzzles, t-shirts) share `POST /ai/{feature}/store`, so `generate --tool <feature>` covers the whole catalog with one command. Default tool: `image-designer-v6`.
- Edit pipeline (upscale/bg-remove/inpaint/expand) is **deferred** — its image-upload payload shape is unverified (would need an additional sniff). Not a stub in scope; explicitly out of v1.

## Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Batch generate from a prompt file (quota-aware) | batch | hand-code | Web UI is one-prompt-at-a-time; loops the generate endpoint over a prompt/CSV/JSONL file, concurrency-capped, refuses to exceed ~400/day cap | Use this to submit MANY prompts from a file. For a SINGLE prompt use 'generate'. To re-run ONE past design's settings use 'redo'. To download finished images use 'export'. |
| 2 | Generate, wait, and download in one shot | generate --wait --download | hand-code | Completion is pushed over a Pusher socket scripts can't see; polls /fetch-personal-designs until processing→private then pulls CDN images | Use --wait/--download on 'generate'/'batch' to block until rendered and save. To download EXISTING designs (no quota) use 'export'. |
| 3 | Local archive sync + offline prompt search | search | hand-code | Artistly has no history search; mirrors /fetch-personal-designs into SQLite with FTS over prompts/tags/checkpoint/dimensions | Use 'search' to FIND past designs in your local mirror; run 'sync' first. Never generates. To regenerate a result use 'redo'. |
| 4 | Reproduce / remix a past design | redo | hand-code | Reads a prior design's exact params from the mirror and resubmits; web UI has no "make more like this" reusing seed/settings | Use this to GENERATE from ONE existing design's settings. For many prompts use 'batch'. To re-download originals use 'export'. Requires 'sync'. |
| 5 | Generation settings presets | preset | hand-code | No preset feature in Artistly; stores a named bundle of checkpoint+style+dims+aspect+negative+quality in local config; zero quota | Use 'preset' to save/reuse generation SETTINGS (not prompts). Apply with --preset on 'generate'/'batch'. Config only; does not generate. |
| 6 | Bulk export by query | export | hand-code | Selects designs from mirror by folder/date/prompt-match and downloads all CDN images into a tree with templated filenames; no quota | Use 'export' to bulk-download images from designs you ALREADY made, by query. Never generates. To generate-then-download use 'generate --wait --download'. |
| 7 | Quota preflight / batch budget check | quota | hand-code | Arithmetic the web UI won't do: reads limit/today counts and reports whether a planned batch fits under the cap before burning it on failures | Use 'quota' to check remaining gens and whether a batch fits. 'batch' enforces the same cap automatically. |

## Build summary
- Absorbed commands: 11 (generate, designs list/download/delete/move, folders, prompt enhance/extract, styles, doctor/quota).
- Transcendence (hand-code): 7 (batch, generate --wait/--download, search+sync, redo, preset, export, quota preflight).
- Auth: cookie (`artistly_session`) + CSRF (`XSRF-TOKEN`→`X-XSRF-TOKEN`), Inertia headers on writes. `auth login --chrome` cookie import.
- Deferred (out of v1, not stubs): image-to-image edit pipeline (unverified upload payload); the dozens of niche puzzle/story/flipbook generators (reachable via `generate --tool <feature>` escape hatch, no dedicated commands).
