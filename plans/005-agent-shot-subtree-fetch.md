# Plan 005: scope `agent shot` fetches to a node subtree (`--root`) + subtree-scoped `--children`

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving on. If a
> "STOP condition" occurs, stop and report — do not improvise. When done,
> update this plan's status row in `plans/README.md` unless a reviewer told you
> they maintain the index.
>
> **Drift check (run first)**:
> `git diff --stat 2ea2dc688..HEAD -- library/productivity/figma/internal/cli/agent.go library/productivity/figma/internal/cli/agent_test.go library/productivity/figma/internal/cli/files_nodes_get-file.go library/productivity/figma/SKILL.md library/productivity/figma/README.md library/productivity/figma/.printing-press-patches`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: M
- **Risk**: MEDIUM (changes which Figma endpoint `agent shot` reads from)
- **Depends on**: Plan 002 (DONE) + Plan 003 (DONE) — extends `agent shot` / `resolveAgentShotMatches` / the `--children` block.
- **Category**: performance
- **Planned at**: commit `2ea2dc688`, 2026-06-16

## Why this matters (measured)

`agent shot` always fetches the **entire file** at the requested depth via
`GET /v1/files/<key>?depth=<depth>` before it can resolve a label or expand
children. On the Pain file this whole-file read at `depth 3` is the dominant
latency: a live timed run showed the first `agent shot` taking **~32s**, almost
all of it this fetch (a second call was ~8s only because the 5-minute response
cache had the tree). `--children` makes it worse by pushing `--depth 4`, which
fetches an even bigger whole-file tree.

Figma also exposes `GET /v1/files/<key>/nodes?ids=<id>&depth=<n>`, which returns
**only that node's subtree** — far smaller and faster than the whole file at the
same depth. This plan lets `agent shot`:

1. **Skip the whole-file read** when a root node is known (`--root`, or a
   node-id already in the Figma URL) by fetching just that subtree.
2. **Scope `--children` expansion** to a subtree fetch of the matched
   page/section at `--depth`, instead of relying on the whole-file depth being
   high enough — which also fixes the Plan 003 "children only reachable at
   `--depth 4`" limitation cheaply.

Net: the common "a few screens of <page>" flow goes from one ~30s whole-file
read to one small targeted subtree read.

## Current state

`resolveAgentShotMatches` (`agent.go:714`) fetches the whole file, builds the
index, scores, then optionally expands children (`agent.go:760`):

```go
raw, err := c.Get("/v1/files/"+ref.FileKey, map[string]string{"depth": fmt.Sprintf("%d", depth)})
// ...decode {document}...
nodes := buildAgentNodeIndex(env.Document, parseTypeSet(types), 0)
// ...score/sort...
if children && len(matches) > 0 {
    top := matches[0]
    topType := strings.ToUpper(strings.TrimSpace(top.Type))
    if topType == "CANVAS" || topType == "SECTION" {
        childMatches := []agentMatch{}
        for _, n := range nodes {                       // ← children pulled from the whole-file index
            // FRAME/INSTANCE/COMPONENT + HasPrefix(n.Label, top.Label+" / ") + isLikelyScreenNode
        }
        // sort shallowest-first, then label
    }
}
```

The direct-hit helper already reads a subtree via the nodes endpoint
(`agent.go:944`), proving the shape and that it returns a per-id
`{nodes:{<id>:{document:{…}}}}` envelope:

```go
raw, err := c.Get("/v1/files/"+ref.FileKey+"/nodes", map[string]string{"ids": ref.NodeID, "depth": "1"})
// envelope: { "nodes": { "<id>": { "document": { … } } } }
```

`buildAgentNodeIndex(document, allowedTypes, maxDepth)` (`agent.go:132`) walks a
**document** node's children — it works on any subtree document, not just the
file root. `agentResolveDirectMatch` (`agent.go:~930`) already extracts the
`document` from the nodes envelope; reuse that extraction.

`figmaRef` carries `FileKey` and `NodeID` (normalized, from a URL `?node-id=`).
`newAgentShotCmd` (`agent.go:474`) defines the flags and calls
`resolveAgentShotMatches(flags, c, ref, query, types, depth, max, children)`.

Important subtlety for path labels: `buildAgentNodeIndex` builds labels from the
**subtree root down** (the subtree root's children are the first indexed level).
So a subtree index's labels start at the root node's children, not at the file
page. That is fine for matching/children, but the returned `label` will be
**relative to the subtree root**, not the full page path. Decide and document:
keep labels subtree-relative (simpler) — agents already treat labels as
human hints, and ids are what matter downstream.

Repo conventions: published library repo — record the hand-edit in
`.printing-press-patches/`; gates `go build ./...`, `go vet ./...`, `--help`,
`--version`; standard library only.

## Commands you will need

| Purpose | Command | Expected |
|---|---|---|
| Drift check | see top | no output / reviewed |
| Focused tests | `cd library/productivity/figma && go test ./internal/cli` | exit 0 |
| Full tests | `cd library/productivity/figma && go test ./...` | exit 0 |
| Vet | `cd library/productivity/figma && go vet ./...` | exit 0 |
| Build | `cd library/productivity/figma && go build ./cmd/figma-pp-cli` | exit 0 |
| Help smoke | `cd library/productivity/figma && go run ./cmd/figma-pp-cli agent shot --help` | shows `--root` |
| Version smoke | `cd library/productivity/figma && go run ./cmd/figma-pp-cli --version` | prints `figma-pp-cli ...` |

## Scope

**In scope:**
- `library/productivity/figma/internal/cli/agent.go` — add `--root`, a `fetchSubtreeDocument` helper, route `resolveAgentShotMatches` to the subtree fetch when a root is known, and make `--children` expand from a subtree fetch of the matched node.
- `library/productivity/figma/internal/cli/agent_test.go` — fake `/v1/files/<key>/nodes` subtree responses + tests.
- `library/productivity/figma/.printing-press-patches/agent-shot-subtree-fetch.json` — patch metadata.
- `library/productivity/figma/SKILL.md`, `library/productivity/figma/README.md` — document `--root` and the perf behavior.
- `plans/README.md` — status row.

**Out of scope:** `client.go`, the generated `files nodes`/`files` commands, `registry.json`, `cli-skills/**`, release files, MCP manifests, auth/write behavior, `outline`/`find-node` (a `--root` for those is a follow-up; keep 005 to `shot`), any non-Figma CLI, the generator repo.

## Steps

### Step 1: Subtree fetch helper

In `agent.go`, add:

```go
// fetchSubtreeDocument fetches a single node's subtree via the nodes endpoint
// and returns its document map. Returns (nil, nil) when the id is absent or the
// node was deleted (Figma returns {} or {"<id>":null}); callers decide whether
// that is an error.
func fetchSubtreeDocument(c interface{ Get(string, map[string]string) (json.RawMessage, error) },
    fileKey, nodeID string, depth int) (map[string]any, error)
```

- GET `/v1/files/<key>/nodes` with `{ids: nodeID, depth: fmt.Sprintf("%d", depth)}`.
- Decode `{ "nodes": { "<id>": { "document": {…} } } }`; return the first entry's `document`.
- Reuse the extraction logic already in `agentResolveDirectMatch` (factor a tiny shared helper if cleaner; do not change its behavior).

**Verify**: `cd library/productivity/figma && go build ./internal/...` → exit 0.

### Step 2: `--root` flag + scoped initial fetch

In `newAgentShotCmd` add `--root string` ("Node id to scope the search to a page/section subtree (skips the whole-file fetch)"), and thread it into `resolveAgentShotMatches` (new `root string` param).

In `resolveAgentShotMatches`, choose the index source:

1. Determine the effective root: `root` flag if set, else `ref.NodeID` if a query is **also** present (URL deep-linked to a page and the user named a screen within it). Normalize with `normalizeNodeID`.
2. **If an effective root is set**: `doc, err := fetchSubtreeDocument(c, ref.FileKey, root, depth)`. If `doc == nil`, return a clear error (`root node <id> not found in <key>`). Build `nodes := buildAgentNodeIndex(doc, parseTypeSet(types), 0)`.
3. **Else**: keep the current whole-file fetch.
4. Scoring/sorting/ambiguity logic is unchanged and runs on `nodes` either way.

Leave the existing `query == "" && ref.NodeID != ""` direct-hit path as-is (it already avoids the whole-file read).

**Verify**: `cd library/productivity/figma && go build ./cmd/figma-pp-cli` → exit 0.

### Step 3: Subtree-scoped `--children`

Replace the `--children` expansion so it does **not** depend on the whole-file
depth. After the top match is found and it is a `CANVAS`/`SECTION`:

1. `childDoc, err := fetchSubtreeDocument(c, ref.FileKey, top.ID, depth)` — fetch the matched node's subtree at `--depth`.
2. If `childDoc != nil`: `childNodes := buildAgentNodeIndex(childDoc, nil, 0)`; select `FRAME`/`INSTANCE`/`COMPONENT` that pass `isLikelyScreenNode`. Because this index is rooted at the section, **all of these are descendants by construction** — you no longer need the `HasPrefix(n.Label, top.Label+" / ")` guard (labels are now subtree-relative). Keep the shallowest-first then lexical sort.
3. If the subtree fetch fails or yields no screen-like children, fall back to the current whole-file-prefix selection (so behavior never regresses).
4. Apply the existing `max` cap / de-dup.

This makes `--children` correct at any nesting depth with a small fetch, removing the Plan 003 "needs `--depth 4` whole file" caveat.

**Verify**: `cd library/productivity/figma && go build ./cmd/figma-pp-cli` → exit 0.

### Step 4: Tests

Extend the fake shot server so `/v1/files/<key>/nodes?ids=<id>` returns that
node's subtree document (use the existing `figmaFileFixture` shapes — e.g. the
`Onboarding` section `1:10` with its `Welcome/Permissions/Complete` frames).
Add:

- `TestFetchSubtreeDocument` — returns the node's document; missing id → `(nil, nil)`.
- `TestAgentShotRootScopesFetch` — `agent shot testKey "Welcome" --root 1:10 --agent --no-cache` resolves from the **subtree** and the fake server asserts the request hit `/v1/files/testKey/nodes` (NOT `/v1/files/testKey`). (Track which paths the server received.)
- `TestAgentShotChildrenUsesSubtree` — `agent shot testKey "Onboarding" --children --max 5 --agent --no-cache` returns the 3 child frames, and the server recorded a `/nodes?ids=<onboarding-id>` request for the children expansion.
- Keep `TestAgentShotChildrenRendersSectionScreens` (Plan 003) passing — adjust only if the label-prefix assertion must become subtree-relative; if so, assert on `id`/`type` and that they are the expected frames, not the section.

No live Figma; `t.TempDir()` for `--out-dir`.

**Verify**: `cd library/productivity/figma && go test ./internal/cli` → exit 0.

### Step 5: Patch metadata + docs

Create `.printing-press-patches/agent-shot-subtree-fetch.json` (schema_version 2, id `agent-shot-subtree-fetch`, files: `internal/cli/agent.go`, `internal/cli/agent_test.go`, `README.md`, `SKILL.md`; summary/reason: scope fetches to a node subtree to cut whole-file read latency; truthful `validated_outcome`).

Docs:
- `SKILL.md`: document `--root <node-id>` ("scope to a page/section subtree for a much faster render; get the node id from `agent outline` or a Figma URL `?node-id=`"). Note `--children` now fetches the matched section's subtree directly, so deep screens no longer need `--depth 4`.
- `README.md`: one concise line for `--root`.

**Verify**: `python3 -m json.tool library/productivity/figma/.printing-press-patches/agent-shot-subtree-fetch.json >/dev/null` → exit 0.

### Step 6: Final gates

```bash
cd library/productivity/figma
go test ./... && go vet ./... && go build ./cmd/figma-pp-cli
go run ./cmd/figma-pp-cli agent shot --help
go run ./cmd/figma-pp-cli --version
git status --short
```

Expected: tests pass; vet clean; build ok; help shows `--root`; version prints; only in-scope files changed.

## Test plan

New tests: `TestFetchSubtreeDocument`, `TestAgentShotRootScopesFetch`,
`TestAgentShotChildrenUsesSubtree`, plus the Plan 003 children test still green.
The fake server tracks which Figma paths it served so tests can assert the
subtree endpoint was used (and the whole-file endpoint was not, on the `--root`
path). No live Figma.

## Done criteria

- [ ] `agent shot --root <node-id>` fetches `/v1/files/<key>/nodes?ids=<id>&depth=<depth>` and does **not** call `/v1/files/<key>` for the index.
- [ ] When a query is given alongside a URL `?node-id=`, the node-id is used as the root automatically.
- [ ] `--children` expands from a subtree fetch of the matched page/section at `--depth`, returning correct screens without requiring a high whole-file depth; falls back to the previous whole-file behavior if the subtree fetch fails/empties.
- [ ] The `--max 1` ambiguity guard and render/batch behavior (Plans 002/003) are unchanged.
- [ ] `go test ./...`, `go vet ./...`, `go build ./cmd/figma-pp-cli` exit 0.
- [ ] `git status --short` shows only in-scope files; patch metadata validates.

## STOP conditions

Stop and report if:
- The `resolveAgentShotMatches` / `--children` / `agentResolveDirectMatch` excerpts don't match after the drift check.
- The nodes endpoint envelope differs from `{nodes:{<id>:{document}}}` in practice.
- Making labels subtree-relative would break an existing Plan 002/003 test contract that other code depends on (if so, prefer keeping labels and only changing the fetch source; report the tension).
- Tests would require a live Figma token or network egress.

## Maintenance notes

- Labels become subtree-relative on the `--root`/`--children` paths. That is acceptable (ids drive downstream calls), but keep it consistent and documented so agents don't expect full page paths there.
- A natural follow-up: add `--root` to `agent outline`/`find-node` too, and let the consumer's known-files store per-page node ids so the skill can pass `--root` directly for the common "screens of <page>" ask. Out of scope here.
- Reviewers: confirm (1) the `--root` path issues zero `/v1/files/<key>` (whole-file) requests, (2) `--children` still returns real screens (not sections) and never fewer than before, (3) a missing/ío deleted root id yields a clean error, not a fabricated match.
