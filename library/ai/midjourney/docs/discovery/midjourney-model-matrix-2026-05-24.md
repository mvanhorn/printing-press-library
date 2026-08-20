# Midjourney Model Matrix Capture — 2026-05-24

Captured through the logged-in Midjourney Create UI, then processed with Printing Press `browser-sniff`.

Artifacts live outside the repo under:

`~/printing-press/.runstate/midjourney-model-matrix-20260524/`

Key files:

- `midjourney-model-matrix.har`
- `midjourney-model-matrix-extra.har`
- `midjourney-model-matrix-summary.json`
- `midjourney-model-matrix-extra-summary.json`
- `midjourney-model-matrix-sniff-spec.yaml`
- `midjourney-model-matrix-extra-sniff-spec.yaml`
- `samples/` and `extra-samples/`

Observed endpoint behavior:

- The UI posts `POST /api/prompt-session-log` before `POST /api/submit-jobs`.
- `POST /api/submit-jobs` remains the mutating endpoint for all tested model/config variants.
- Model selection is encoded in the prompt string: `--v 7`, `--v 6.1`, `--v 6`, or `--niji 6`.
- Aspect ratio, quality, raw/style, chaos, stylize, seed, tile, weird, draft, style refs, omni refs, and profile are also prompt-suffix parameters.
- The UI canonicalizes `--style raw` to `--raw` in submit payloads for raw mode.
- Draft mode appends `--draft`; response `event_type` becomes `draft` and `job_type` becomes `v7_draft_diffusion`.
- Draft and tile are incompatible; Midjourney returns `invalid_parameter` with message `Draft jobs are not compatible with \`--tile\``.
- Prompt suffixes `--turbo` and `--relax` were accepted. The request still carried `f.mode: fast`, but response flags resolved to `turbo` and `relaxed`.
- First-class `f.mode` support remains useful for direct API calls; prompt suffix speed support may be worth preserving if we want exact UI parity.

Job type examples from capture:

- `--v 7`: `v7_diffusion`
- `--v 7 --style raw`: `v7_raw_diffusion`
- `--v 6.1 --style raw`: `v6-1_raw_diffusion`
- `--v 6`: `v6_diffusion`
- `--niji 6`: `v6_diffusion_anime`
- `--v 7 --draft`: `v7_draft_diffusion`

CLI changes from this capture:

- Added `imagine --style <value>` for non-raw styles such as `cute`.
- Added `imagine --niji <version>` and suppress `--v` when `--niji` is set.
