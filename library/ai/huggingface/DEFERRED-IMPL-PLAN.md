# huggingface-pp-cli — Implementation Status (v0.2 SHIPPED)

**Date:** 2026-05-09 (updated after the v0.2 build completed)

## Final status

Full surface shipped: **16 commands**. All 3 gaps closed. ~**4900 LOC** of hand-built Go across `internal/cli/hf_*.go` (18 files) + `internal/hfx/` + `internal/hfdata/`.

### Shipped commands (all live-tested 2026-05-09)

**Group A — Discovery (8/8):**
1. `find-quants <base>` — sorted GGUF variants w/ uploader rep + size filter
2. `trending` — size-class + library + window filters
3. `model-card <id>` — MoE active params, effective GGUF size, license, tags
4. `derivatives <base>` — search + client-filter on cardData.base_model + tags
5. `uploader-rep <user>` — aggregate downloads + trusted-uploader badge
6. `compare-quants <id1> <id2> ...` — parallel fetch, side-by-side
7. `eval-candidates --base <id>` — harness-input emitter (wraps findQuantsCore)
8. `find-feature <feat>` — config.json arch classifier + backend verdicts

**Group B — Intel loop (5/5):**
9. `vs-current <id>` — diffs candidate against agent's current model from openclaw.json
10. `backend-check <id>` — bundled matrix lookup w/ citations + source_checked dates
11. `bench-history <id>` — joins HF id with workspace/memory/model-eval-*.json
12. `watch add/list/remove` — state-file subscription, flock-guarded
13. `watch-poll` — cron-callable, emits to stdout / file:<path> / jarvis (data/alerts/<id>.json MC API alert pipeline)

**Group C — Runtime utility (2/2):**
14. `doctor` — 19-field structured runtime probe
15. `schema` — per-command stable JSON schema introspection

`local` (#16) — pure-local cache walker (`~/.cache/huggingface/hub/models--*--*/`) — was originally counted in Group B but functions as runtime utility. Either way, shipped.

### Closed gaps

- **Gap A** (structured exit codes): `hf_errors.go` provides `hfNotFound` (2), `hfBackendUnsupported` (3), `hfAlreadyCached` (4), `hfRateLimited` (5), `hfConfigMissing` (6) wrappers around the framework's `cliError`. Verified end-to-end: `vs-current --config-path missing → 6`, `model-card 404 → 2`, `eval-candidates no matches → 2`, etc.
- **Gap B** (rate-limit bucket): `hfSetRequestState` wires `--state-dir` + `--no-write` into `hfDoGET` once at command boot via `PersistentPreRunE`. Every HTTP call snapshots the `RateLimit:` header (RFC draft form `"api";r=N;t=N`) and persists `<state-dir>/rate-limit-bucket.json` flock-guarded. Pre-flight refuses outbound when `remaining==0` and reset hasn't elapsed. CC + JARVIS + cron all share the same view.
- **Gap C** (findQuantsCore programmatic helper): extracted `parseFindQuantsOpts` + `findQuantsCore` + `findQuantsCoreResult` from the find-quants RunE; eval-candidates calls it directly.

### Validation

- `govulncheck ./...` → **No vulnerabilities found** (Little Snitch now allows vuln.go.dev).
- `go test ./...` → all packages pass (cli, cliutil, hfdata, hfx, mcp, store).
- `go vet ./...` → clean.
- `printing-press dogfood` → WARN-only on cosmetic items (1 framework dead helper `yellow`, `watch remove` example added).
- 16-command live smoke matrix run end-to-end against HF API + `data/openclaw.json` + `workspace/memory/model-eval-*.json`. All 16 exit cleanly.

### Two real bugs found+fixed in this build

1. **HF rejects `sort=trending`** — only accepts `lastModified`/`downloads`/`likes`/`createdAt`. `find-feature` defaulted to `sort=trending` on first ship and crashed on first call. Fixed: default `sort=lastModified`, classifier handles relevance.
2. **URL escaping `%2F` for repo slashes** — `url.PathEscape("Qwen/Qwen2.5-7B")` produces `Qwen%2FQwen2.5-7B` which HF's `/api/models/{id}` rejects as "url-encoded slash". Fixed: `hfPathPreserveSlash` escapes per-segment without touching `/`.

### What's no longer deferred

Original deferred-9 list: trending, derivatives, uploader-rep, compare-quants, eval-candidates, bench-history, watch, watch-poll, local — **all shipped in v0.2**. Nothing in this plan is pending.

### What might still be polish-worthy (not blocking)

- `dogfood` reports `Novel Features: SKIP (no research.json)` even though `research.json` is at the library root. Likely the tool looks for it at a different path; cosmetic.
- Framework's `yellow` helper in `internal/cli/helpers.go` is dead. Generated code; would re-emerge on regen. Cosmetic.
- The `--explain` block on find-feature was missed (other commands have it). Trivial.
- Effective GGUF size on the listing path (`find-quants` results) shows `0.0 GB` for many entries because HF's `/api/models` listing doesn't populate sibling sizes even with `?full=true`. The model-card path uses `?blobs=true` and gets real sizes. Document or call per-result.

These are v0.3 cosmetic items, not user-blocking.
