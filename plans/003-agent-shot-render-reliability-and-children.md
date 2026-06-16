# Plan 003: `agent shot` render reliability (split-on-timeout) + section/page children rendering

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving on. If a
> "STOP condition" occurs, stop and report — do not improvise. When done,
> update this plan's status row in `plans/README.md` unless a reviewer told you
> they maintain the index.
>
> **Drift check (run first)**:
> `git diff --stat 47e979045..HEAD -- library/productivity/figma/internal/cli/agent.go library/productivity/figma/internal/cli/agent_test.go library/productivity/figma/internal/client/client.go library/productivity/figma/SKILL.md library/productivity/figma/README.md library/productivity/figma/.printing-press-patches`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: M
- **Risk**: MEDIUM (changes the render request pattern and adds retry logic)
- **Depends on**: Plan 002 (DONE) — extends `agent shot` and reuses its helpers (`resolveAgentShotMatches`, `isLikelyScreenNode`, `downloadToFile`, `buildAgentNodeIndex`, `agentMatch`).
- **Category**: correctness / dx
- **Planned at**: commit `47e979045`, 2026-06-16

## Why this matters (evidence)

A live multi-screen request ("show me the Pain onboarding screens — a few of them") exposed two concrete problems in the shipped `agent shot`:

1. **Render-timeout 400.** `agent shot "Onboarding" --max 5` returned
   `HTTP 400: {"status":400,"err":"Render timeout, try requesting fewer or smaller images"}`
   and produced **nothing** (~55s wasted), forcing a manual retry at `--max 3`.
   Cause: the command puts *all* node ids into a **single** `/v1/images`
   request at the default `scale 2`. Figma times out rendering several large
   frames in one shot.
2. **"A few screens of X" doesn't map to a single call.** `agent shot
   "Onboarding" --max 3` matched only nodes whose *name contains* "Onboarding"
   (all under one MVP section), not "a few distinct onboarding screens". The
   agent then fell back to `agent outline --depth 3` (a 200-node JSON) plus a
   low-level `images --ids …` call to assemble variety — the exact multi-call
   sprawl `agent shot` was meant to remove.

This plan makes `agent shot` (a) render reliably by batching + splitting on
render-timeout (and degrading scale as a last resort), and (b) optionally
render the screen-like **children** of a matched page/section so a "a few X
screens" request is one call.

## Current state

`agent shot` lives in `library/productivity/figma/internal/cli/agent.go`.

The render step issues a **single** request for all ids (around `agent.go:540`):

```go
ids := make([]string, 0, len(matches))
for _, m := range matches {
    ids = append(ids, m.ID)
}
raw, err := c.Get("/v1/images/"+ref.FileKey, map[string]string{"ids": strings.Join(ids, ","), "format": format, "scale": fmt.Sprintf("%g", scale)})
if err != nil {
    return classifyAPIError(err, flags)   // a 400 render-timeout kills the whole command
}
var render struct {
    Err    *string            `json:"err"`
    Images map[string]*string `json:"images"`
}
```

Flags (around `agent.go:597`):

```go
cmd.Flags().IntVar(&max, "max", 3, "Max screenshots to render")
cmd.Flags().Float64Var(&scale, "scale", 2, "Image render scale")
cmd.Flags().StringVar(&format, "format", "png", "Image format")
cmd.Flags().IntVar(&depth, "depth", 3, "Max tree depth to request from the Figma file API")
cmd.Flags().StringVar(&types, "types", "SECTION,FRAME,COMPONENT,INSTANCE", "Comma-separated node types to include")
cmd.Flags().StringVar(&outDir, "out-dir", filepath.Join(os.TempDir(), "figma-pp-cli"), "Directory for downloaded screenshots")
cmd.Flags().BoolVar(&noDownload, "no-download", false, "Return render URLs without downloading image bytes")
```

Match resolution is `resolveAgentShotMatches(flags, c, ref, query, types, depth, max)` (`agent.go:608`). It builds the node index with `buildAgentNodeIndex(env.Document, parseTypeSet(types), 0)`, scores with `scoreAgentNode`, filters with `isLikelyScreenNode`, sorts, applies the `--max 1` ambiguity guard, and returns up to `max` `agentMatch` values. `agentMatch` carries `ID, Name, Type, Label, Score`. `agentNodeSummary` (returned by the index) additionally carries `Path []string` and `ParentID`.

The error type for render failures (`internal/client/client.go`):

```go
type APIError struct {
    Method     string
    Path       string
    StatusCode int
    Body       string
}
func (e *APIError) Error() string { /* "...returned HTTP <code>: <body>" */ }
```

So a render-timeout is an `*APIError` with `StatusCode == 400` and `Body`
containing `Render timeout`. Use `errors.As` to detect it.

Test harness (`agent_test.go`): `newFakeFigmaShotServer(t)` serves the file
tree, `/v1/images/`, and `/render/` bytes; `runAgentRoot(t, args)` runs a root
command and returns stdout; `setFigmaTestEnv(t, srv.URL)` points the client at
the fake server. The render route already special-cases a node id to return a
503 (download-failure test). You will extend this server to simulate a
render-timeout 400 for a multi-id request.

Repo conventions: published library repo — hand-edits recorded in
`.printing-press-patches/` (`AGENTS.md:13-17,77-85`); gates `go build ./...`,
`go vet ./...`, `--help`, `--version` (`CONTRIBUTING.md:31-49`); standard
library only (`errors`, `strings`, `fmt`, `net/http`, `encoding/json`).

## Commands you will need

Run from the repo root unless noted.

| Purpose | Command | Expected |
|---|---|---|
| Drift check | see top | no output, or reviewed changes |
| Focused tests | `cd library/productivity/figma && go test ./internal/cli` | exit 0 |
| Full tests | `cd library/productivity/figma && go test ./...` | exit 0 |
| Vet | `cd library/productivity/figma && go vet ./...` | exit 0 |
| Build | `cd library/productivity/figma && go build ./cmd/figma-pp-cli` | exit 0 |
| Help smoke | `cd library/productivity/figma && go run ./cmd/figma-pp-cli agent shot --help` | shows `--batch`, `--children`, `--max-scale-downgrade` (or chosen names) |
| Version smoke | `cd library/productivity/figma && go run ./cmd/figma-pp-cli --version` | prints `figma-pp-cli ...` |

## Scope

**In scope:**
- `library/productivity/figma/internal/cli/agent.go` — render batching/split + scale downgrade; `--children` expansion in/around `resolveAgentShotMatches`; new flags; a `renderImages` helper.
- `library/productivity/figma/internal/cli/agent_test.go` — extend the fake server to simulate render-timeout; add tests.
- `library/productivity/figma/.printing-press-patches/agent-shot-render-reliability.json` — patch metadata.
- `library/productivity/figma/SKILL.md`, `library/productivity/figma/README.md` — document new behavior/flags.
- `plans/README.md` — status row.

**Out of scope:** `internal/client/client.go` (use it as-is), `promoted_images.go`, generated commands, `registry.json`, `cli-skills/**`, release files, MCP manifests, auth/write behavior, any non-Figma CLI, the generator repo.

## Steps

### Step 1: Extract a batched, timeout-aware `renderImages` helper

In `agent.go`, add:

```go
// renderImages renders ids via /v1/images in batches, splitting a batch and
// (as a last resort) lowering scale when Figma returns a 400 "Render timeout".
// Returns id->url (nil url means "could not render"). Never returns a hard
// error for a render-timeout; only transport/other errors propagate.
func renderImages(c interface{ Get(string, map[string]string) (json.RawMessage, error) },
    fileKey, format string, scale float64, ids []string, batch int) (map[string]*string, error)
```

Behavior:
1. `if batch <= 0 { batch = 1 }`. Chunk `ids` into groups of `batch`.
2. For each chunk, call `/v1/images/<key>?ids=<csv>&format=<format>&scale=<scale>`.
3. Parse `{err, images}`. Merge `images` into the result map.
4. **On a render-timeout** (`errors.As(err, &apiErr)` with `apiErr.StatusCode == 400` and `strings.Contains(strings.ToLower(apiErr.Body), "render timeout")`), or when the response `err` field mentions render timeout:
   - If the chunk has >1 id, split it in half and retry each half (recurse / queue).
   - If the chunk is a single id, retry **once at a lower scale** (`scale > 1` → retry at `1`); if it still times out, record that id with a `nil` url (caller surfaces `render_error`).
5. **For any other transport/API error**, return it (caller maps via `classifyAPIError`).
6. Keep total work bounded: a single id is retried at most once at reduced scale; do not loop forever.

Keep it deterministic and standard-library only. Add `"errors"` to imports.

**Verify**: `cd library/productivity/figma && go build ./internal/...` → exit 0.

### Step 2: Use `renderImages` in `agent shot` and add `--batch` / `--max-scale` flags

Replace the single-request render block (current `agent.go:540`) with a call to
`renderImages(c, ref.FileKey, format, scale, ids, batch)`:

- Add flag `--batch int` default `2` ("node ids per render request; lower avoids Figma render timeouts").
- Keep `--scale` default `2`. Downgrade-on-timeout (Step 1.4) handles large frames without changing the default.
- On a non-timeout error from `renderImages`, `return classifyAPIError(err, flags)`.
- Build the `images` output array from the returned map exactly as today
  (per-match: `url`, then download to `path`/`bytes` unless `--no-download`,
  else `render_error` when the url is nil). Preserve the existing output shape,
  `next_steps`, and `--no-download` behavior.

**Verify**: `cd library/productivity/figma && go build ./cmd/figma-pp-cli` → exit 0.

### Step 3: Add `--children` to render a section/page's screens in one call

Goal: `agent shot <file> "Onboarding" --children --max 5` renders the first
N screen-like frames **under** the matched page/section, instead of the
section node itself.

In `resolveAgentShotMatches` (add a `children bool` parameter, thread it from
the command), after computing the sorted `matches` from the full index:

1. If `children` is false → behave exactly as today.
2. If `children` is true and the **top** match's `Type` is `CANVAS` or
   `SECTION`:
   - You already have the full node index (`nodes := buildAgentNodeIndex(...)`).
     Re-resolve it to `agentNodeSummary` (the index has `Path`/`Label`/`Type`).
     Select summaries that are **descendants of the matched node** — i.e. their
     `Label` has the matched node's `Label` as a strict prefix (`strings.HasPrefix(child.Label, top.Label+" / ")`) — AND `isLikelyScreenNode(child)` AND `child.Type` is `FRAME`/`INSTANCE`/`COMPONENT` (not nested SECTION/CANVAS).
   - Prefer the shallowest descendants first (fewest `" / "` separators beyond
     the parent), then stable label order. Take up to `max`.
   - If no screen-like descendants are found, fall back to rendering the matched
     section node itself (current behavior).
3. The `--max 1` ambiguity guard still applies to the *initial* match selection
   (a tie on which section/screen to expand is still ambiguous).

Add the `--children` bool flag (default `false`). Document that agents should
set it for "a few screens of <page/section>" requests.

> Note: `resolveAgentShotMatches` currently returns `[]agentMatch`. To select
> descendants you need the richer `agentNodeSummary` list. Either (a) keep a
> local `[]agentNodeSummary` alongside, or (b) have the helper build children
> from the index it already constructed. Do not change the index/scoring
> contract from Plan 002.

**Verify**: `cd library/productivity/figma && go build ./cmd/figma-pp-cli` → exit 0.

### Step 4: Tests

Extend `newFakeFigmaShotServer` (or add `newFakeFigmaShotTimeoutServer`) so the
`/v1/images/` handler returns
`{"status":400,"err":"Render timeout, try requesting fewer or smaller images"}`
with HTTP 400 **when the `ids` query contains more than one id**, and succeeds
for a single id. This simulates the real failure.

Add tests:
- `TestRenderImagesSplitsOnTimeout` — call `renderImages` with 3 ids and
  `batch=3` against the timeout server; assert all 3 ids resolve to non-nil
  urls (because it split down to single-id requests that succeed).
- `TestAgentShotSucceedsDespiteRenderTimeout` — `agent shot testKey <multi-match-query> --max 3 --out-dir <tmp> --agent --no-cache`
  against the timeout server; assert `count == 3` and each image has a `path`
  (or `url`), and the command exits 0 (no 400 bubbling up).
- `TestAgentShotChildrenRendersSectionScreens` — with a fixture page/section
  that has ≥2 screen-like FRAME children, `agent shot testKey "<section name>" --children --max 5 --agent`
  returns the child frames (assert their labels are descendants of the section
  label), not the section node itself.
- Keep an existing single-id happy-path test passing unchanged.

Use `t.TempDir()` for `--out-dir`; no live Figma.

**Verify**: `cd library/productivity/figma && go test ./internal/cli` → exit 0.

### Step 5: Patch metadata + docs

Create `library/productivity/figma/.printing-press-patches/agent-shot-render-reliability.json` (schema_version 2, id `agent-shot-render-reliability`, files: `internal/cli/agent.go`, `internal/cli/agent_test.go`, `README.md`, `SKILL.md`; summary/reason describing batch+split-on-timeout and `--children`; truthful `validated_outcome`).

Update `SKILL.md` `agent shot` section:
- Note that `agent shot` now renders in batches and auto-retries on Figma render
  timeouts, so large multi-screen requests no longer hard-fail.
- Document `--children`: "for 'a few screens of <page/section>' requests, add
  `--children` so one call renders the section's screens."
- Update `README.md` highlight similarly (concise).

**Verify**: `python3 -m json.tool library/productivity/figma/.printing-press-patches/agent-shot-render-reliability.json >/dev/null` → exit 0.

### Step 6: Final gates

```bash
cd library/productivity/figma
go test ./... && go vet ./... && go build ./cmd/figma-pp-cli
go run ./cmd/figma-pp-cli agent shot --help
go run ./cmd/figma-pp-cli --version
git status --short
```

Expected: tests pass; vet clean; build ok; help shows `--batch` and
`--children`; version prints; only in-scope files changed.

## Test plan

New tests in `agent_test.go`: `TestRenderImagesSplitsOnTimeout`,
`TestAgentShotSucceedsDespiteRenderTimeout`,
`TestAgentShotChildrenRendersSectionScreens`, plus the unchanged single-id
happy path. Fake server simulates the 400 render-timeout for multi-id requests.
No live Figma.

## Done criteria

- [ ] `agent shot` renders in batches (default `--batch 2`) instead of one request for all ids.
- [ ] A Figma 400 "Render timeout" no longer fails the command: the batch is split, a single id is retried once at lower scale, and unresolved ids surface `render_error` while the rest succeed (exit 0).
- [ ] `--children` renders the screen-like descendants of a matched CANVAS/SECTION (up to `--max`), falling back to the section node when none are found.
- [ ] The `--max 1` ambiguity guard from Plan 002 still holds.
- [ ] Output shape, `--no-download`, and download-to-`path` behavior are unchanged for the success path.
- [ ] `go test ./...`, `go vet ./...`, `go build ./cmd/figma-pp-cli` exit 0.
- [ ] `git status --short` shows only in-scope files.
- [ ] Patch metadata exists and validates.

## STOP conditions

Stop and report if:
- The `agent.go` render block or `resolveAgentShotMatches` excerpts don't match after the drift check.
- Detecting the render-timeout reliably requires changing `client.go` (it should be doable with `errors.As` on the existing `*APIError`).
- `--children` descendant selection can't be done from the existing index without changing the Plan 002 scoring/index contract.
- Tests would require a live Figma token or network egress.

## Maintenance notes

- The split-on-timeout is bounded (single ids retried at most once at lower scale). Keep it bounded; do not add unbounded retry loops.
- `--children` selection is prefix-on-label; if labels ever stop being unique-prefix-safe, switch to parent-id chaining. The index already carries `ParentID`.
- If many files need multi-screen rendering often, consider a follow-up that scopes the initial file fetch to the matched page subtree (`/v1/files/<key>/nodes?ids=<page>&depth=2`) instead of the whole file at `--depth 3`, to cut the dominant fetch latency. Out of scope here.
- Reviewers: verify (1) a single-id render-timeout degrades to scale 1 then `render_error`, not an infinite loop; (2) `--children` never renders nested sections as if they were screens; (3) the success-path JSON is byte-compatible with Plan 002 consumers.
