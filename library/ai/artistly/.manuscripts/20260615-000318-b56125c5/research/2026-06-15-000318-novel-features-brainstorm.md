# Artistly novel-features brainstorm (subagent audit trail)

## Customer model

**Maya — KDP / merch seller ("the production-line creator").** Generates coloring-book interiors, t-shirt designs, book covers; Sunday batch of 40-80 prompt variations pasted one at a time into the web generator, waiting + downloading each by hand; slams into the undocumented ~400/day cap mid-batch with no warning (failed gens count).

**Devon — marketing freelancer / agency operator ("the client-deliverable runner").** Reuses a "house style" (checkpoint + style + aspect ratio + negative prompt) across many prompts for campaign coherence; delivers ZIPs to clients; no preset feature, re-types settings every time, downloads/sorts images by hand.

**Sam — automation-minded power user / indie hacker ("the pipeline builder").** Wants Artistly as a step in a Makefile/cron/agent flow; needs search over own prompt history and programmatic job-done signal; completion is pushed over a Pusher socket invisible to scripts.

## Survivors (transcendence rows)

| # | Feature | Command | Score | Buildability | Persona | Why only we can do this | Long Description |
|---|---------|---------|-------|--------------|---------|-------------------------|------------------|
| 1 | Batch generate from a prompt file (quota-aware) | `batch <file>` | 8 | hand-code | Maya, Sam | Web UI is one-prompt-at-a-time; loops POST /ai/{feature}/store over a prompt/CSV/JSONL file, concurrency capped to limit, refuses to exceed ~400/day cap | Use this to submit MANY prompts from a file. For a SINGLE prompt use `generate`. To re-run ONE past design's settings use `redo`. To download finished images use `export`. |
| 2 | Generate, wait, and download in one shot | `generate --wait --download <dir>` | 8 | hand-code | Maya, Sam | Completion is pushed over a Pusher socket scripts can't see; polls /fetch-personal-designs until processing→private then pulls CDN images | Use `--wait`/`--download` on `generate`/`batch` to block until rendered and save. To download EXISTING designs (no quota) use `export`. |
| 3 | Local archive sync + offline prompt search | `sync`; `search <text>` | 7 | hand-code | Sam, Maya | Artistly has no history search; mirrors /fetch-personal-designs into SQLite, FTS over prompts/tags/checkpoint/dims | Use `search` to FIND past designs in your local mirror; run `sync` first. Never generates. To regenerate a result use `redo`. |
| 4 | Reproduce / remix a past design | `redo <design-id>` | 7 | hand-code | Sam | Reads a prior design's exact params from the mirror and resubmits; web UI has no "make more like this" reusing seed/settings | Use this to GENERATE from ONE existing design's settings. For many prompts use `batch`. To re-download originals use `export`. Requires `sync`. |
| 5 | Generation settings presets | `preset save/use` | 6 | hand-code | Devon | No preset feature in Artistly; stores named bundle of checkpoint+style+dims+aspect+negative+quality in local config, replays across generate/batch; zero quota | Use `preset` to save/reuse generation SETTINGS (not prompts). Apply with `--preset`. Config only; does not generate. |
| 6 | Bulk export by query | `export` | 6 | hand-code | Devon, Maya | Selects designs from mirror by folder/date/prompt-match, downloads all CDN images into a tree with templated filenames; web UI downloads one at a time; no quota | Use `export` to bulk-download images from designs you ALREADY made, by query. Never generates. To generate-then-download use `generate --wait --download`. |
| 7 | Quota preflight / batch budget check | `quota --for <file>` | 6 | hand-code | Maya | Arithmetic the web UI won't do: reads limit/today counts and tells you if a planned batch fits under the cap before burning it on failures | Use `quota` to check remaining gens and whether a batch fits. `batch` enforces the same cap automatically. |

## Killed candidates
- Standalone `make` — folded into `generate --wait --download`.
- `styles search` — folded as a fuzzy-match flag on `styles list`.
- `edit` image-to-image pipeline (upscale/bg-remove/inpaint/expand) — CUT: edit payload field names unverified, build feasibility unproven; revisit after sniffing upload contract.
- `designs dups` / `archive stats` — covered by `quota` preflight + `search --status processing`.
- `watch` / `status --watch` — subsumed by `--wait`; scope creep toward a monitor.
- `character` (consistent-character) — it's `batch`/`redo` with locked seed; payload unverified.
- `generate --enhance` chain — reduced to a thin flag over absorbed `prompt enhance`.
