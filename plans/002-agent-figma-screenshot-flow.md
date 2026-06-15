# Plan 002: One-shot agent screenshot flow (`agent shot`)

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md` — unless a reviewer dispatched you and told you they
> maintain the index.
>
> **Drift check (run first)**:
> `git diff --stat 85a7e5613..HEAD -- library/productivity/figma/internal/cli/agent.go library/productivity/figma/internal/cli/agent_test.go library/productivity/figma/internal/cli/root.go library/productivity/figma/internal/cli/promoted_images.go library/productivity/figma/internal/client/client.go library/productivity/figma/SKILL.md library/productivity/figma/README.md library/productivity/figma/.printing-press-patches`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: M
- **Risk**: MEDIUM (adds a new outbound HTTP download to the Figma render CDN; see egress caveat)
- **Depends on**: Plan 001 (DONE) — reuses its `parseFigmaRef`, `buildAgentNodeIndex`, `scoreAgentNode`, `agentMatch`, and `agentNextSteps`.
- **Category**: dx
- **Planned at**: commit `85a7e5613`, 2026-06-15

## Why this matters

Getting from a human prompt ("show me the Pain prototype") to a screenshot in
Slack currently takes 4–5 separate CLI calls across separate agent turns:
parse the URL, run `agent find-node`/`agent outline`, choose a node id, run
`images` to get an expiring URL, then fetch+upload that URL. Observed
wall-clock cost is dominated by model reasoning *between* turns, not the CLI
(the `images` render itself is ~8s). Every hop removed is a model round-trip
removed.

Two concrete gaps block a single-call flow:

1. **No render→file path.** `images` only emits expiring Figma S3 URLs; the
   CLI never downloads the bytes. `--deliver file:` writes the JSON envelope,
   not the PNG. So a screenshot can never land as a local file an agent can
   hand straight to Slack.
2. **Discovery returns structural noise.** `agent find-node` defaults include
   `INSTANCE`/`GROUP` and has no notion of a "screen", so label queries match
   numbered frames (`1`..`5`), `chapter`, `Group 13`, `Status Bar`,
   `Home Indicator`, and instance internals. Agents then guess badly or burn
   turns disambiguating.

This plan adds one read-only command, `agent shot`, that resolves a label (or
a URL node-id), filters to screen-like nodes, renders, and downloads PNGs to
local files in a single call — returning both the local `path` and the `url`
so the flow still degrades gracefully when the render CDN is not reachable.

## Egress caveat (read before implementing)

Figma's `/v1/images` endpoint returns **expiring URLs on a Figma AWS S3 host**
(`figma-alpha-api.s3.<region>.amazonaws.com`), which is a *different* host from
`api.figma.com`. In brokered/proxied deployments (e.g. an egress gateway that
only allows `api.figma.com`), the download step can fail even though the render
call succeeded. Therefore:

- Downloading is **best-effort**. On any download failure, the command must
  still return the node, its `url`, and a `download_error` string, and must
  exit 0. Never fail the whole command because one byte-fetch was blocked.
- A `--no-download` flag returns URLs only (no byte fetch), for environments
  where local download is intentionally disabled.

Do not add any allowlist/proxy configuration in this repo; that lives in the
deployment, not the CLI. Just document the dependency (Step 6).

## Current state

Relevant files and roles:

- `library/productivity/figma/internal/cli/agent.go` — the `agent` command
  group (`outline`, `find-node`) plus reusable helpers this plan builds on.
- `library/productivity/figma/internal/cli/promoted_images.go` — generated
  `images <file_key>` command; shows the exact render request shape.
- `library/productivity/figma/internal/client/client.go` — HTTP client; has
  **no** binary-download capability (only `Get` returning `json.RawMessage`).
- `library/productivity/figma/internal/cli/deliver.go` — `deliverFile` shows
  the atomic tmp+rename file-write pattern to mirror.
- `library/productivity/figma/internal/cli/agent_test.go` — command-path test
  harness (`runAgentRoot`, `setFigmaTestEnv`, `newFakeFigmaServer`).
- `library/productivity/figma/internal/cli/root.go` — registers `newAgentCmd`.

Reusable helpers already in `agent.go` (do NOT duplicate them):

```go
func parseFigmaRef(raw string) (figmaRef, error)            // raw key or figma.com URL (+ node-id)
func buildAgentNodeIndex(document map[string]any, allowedTypes map[string]bool, maxDepth int) []agentNodeSummary
func parseTypeSet(s string) map[string]bool
func scoreAgentNode(query string, node agentNodeSummary) int
type agentMatch struct { ID, Name, Type, Label string; Score int }
func agentNextSteps(fileKey string) []string
```

The `agent` group registers its subcommands here (`agent.go`, `newAgentCmd`):

```go
cmd.AddCommand(newAgentOutlineCmd(flags))
cmd.AddCommand(newAgentFindNodeCmd(flags))
return cmd
```

The render request shape (from `promoted_images.go`): method GET, path
`/v1/images/{file_key}`, query params `ids` (comma-separated), `format`
(default `png`), `scale`. The Figma response envelope is:

```json
{ "err": null, "images": { "1:2": "https://figma-alpha-api.s3...", "1:3": null } }
```

A `null` value means that node could not be rendered.

The client exposes only JSON GETs (`client.go`):

```go
func (c *Client) Get(path string, params map[string]string) (json.RawMessage, error)
```

There is no helper that downloads arbitrary bytes; the only file writer is
`deliverFile` in `deliver.go` (atomic tmp + rename), which is the pattern to
copy for writing image bytes.

Command-path test harness (`agent_test.go`):

```go
func setFigmaTestEnv(t *testing.T, baseURL string) {
    t.Setenv("FIGMA_BASE_URL", baseURL)
    t.Setenv("FIGMA_CONFIG", filepath.Join(t.TempDir(), "no-config.toml"))
}
func runAgentRoot(t *testing.T, args []string) (string, error) {
    flags := rootFlags{}
    root := newRootCmd(&flags)
    root.SetArgs(args)
    var out bytes.Buffer
    root.SetOut(&out); root.SetErr(&out)
    return out.String(), root.Execute()
}
```

`newFakeFigmaServer` currently handles `/v1/files/` (tree) and
`/v1/files/<key>/nodes`. It does **not** serve `/v1/images/`; this plan adds a
new fake server that does, plus a render-bytes route.

Repo conventions to honor:

- Published library repo, not the generator. Hand-edits to published CLI
  behavior are allowed but must be recorded in `.printing-press-patches/`
  (`AGENTS.md:13-17`, `AGENTS.md:77-85`).
- Do not edit `registry.json`, generated `cli-skills/pp-*`, release files, or
  MCP manifests for a normal behavior tweak.
- Quality gates (`CONTRIBUTING.md:31-49`): `go build ./...`, `go vet ./...`,
  `--help`, `--version`.
- Standalone module at `library/productivity/figma/go.mod`, Go `1.26.3`. Use
  the standard library only (`net/http`, `os`, `path/filepath`, `regexp` or
  manual sanitization, `encoding/json`, `strings`, `fmt`). Do not add deps.

## Commands you will need

Run from the repo root unless noted.

| Purpose | Command | Expected on success |
|---|---|---|
| Drift check | see "Drift check" above | no output, or only changes you review against this plan first |
| Focused tests | `cd library/productivity/figma && go test ./internal/cli` | exit 0; `ok .../internal/cli` |
| Full Figma tests | `cd library/productivity/figma && go test ./...` | exit 0 |
| Vet | `cd library/productivity/figma && go vet ./...` | exit 0, no diagnostics |
| Build | `cd library/productivity/figma && go build ./cmd/figma-pp-cli` | exit 0 |
| Help smoke | `cd library/productivity/figma && go run ./cmd/figma-pp-cli agent shot --help` | exit 0; shows flags `--max`, `--out-dir`, `--no-download`, `--scale` |
| Group smoke | `cd library/productivity/figma && go run ./cmd/figma-pp-cli agent --help` | exit 0; lists `outline`, `find-node`, `shot` |
| Version smoke | `cd library/productivity/figma && go run ./cmd/figma-pp-cli --version` | exit 0; prints `figma-pp-cli ...` |

## Scope

**In scope** (the only source files you may modify):

- `library/productivity/figma/internal/cli/agent.go` — add `newAgentShotCmd`,
  register it under `newAgentCmd`, add helpers (`sanitizeFilename`,
  `isLikelyScreenNode`, `downloadToFile`, render/parse helpers).
- `library/productivity/figma/internal/cli/agent_test.go` — add helper tests,
  a new fake server that serves `/v1/images/` + render bytes, and command-path
  tests. Reuse existing helpers; do not modify `newFakeFigmaServer`.
- `library/productivity/figma/internal/cli/root.go` — only if registration is
  not already inherited via `newAgentCmd` (it is: `agent shot` is a subcommand
  of `agent`, so `root.go` needs **no** change). Confirm; do not edit
  unnecessarily.
- `library/productivity/figma/.printing-press-patches/agent-screenshot-flow.json`
  — create patch metadata.
- `library/productivity/figma/SKILL.md` — document `agent shot`.
- `library/productivity/figma/README.md` — document `agent shot`.
- `plans/README.md` — update status row when finished, if you maintain it.

**Out of scope** (do NOT touch):

- `registry.json`, `cli-skills/pp-figma/SKILL.md` — regenerated after merge.
- `.printing-press-release.json`, `CHANGELOG.md`, version values — automation.
- `tools-manifest.json` / `manifest.json` / `internal/mcp/*` — keep CLI-first;
  MCP mirroring may pick up the new Cobra command automatically, which is fine,
  but do not hand-edit MCP manifests.
- The generated `images` command (`promoted_images.go`) and the shared
  `client.go` — do not modify them. `agent shot` uses `c.Get(...)` for the
  render call and its own standard-library `http.Client` for the byte download.
- Figma write/auth/credential behavior.
- Any non-Figma CLI under `library/**`; any generator repo.

## Git workflow

- Branch: continue on the current Figma agent branch unless told otherwise.
- Commit style: conventional commits, e.g. `feat(figma): add agent shot screenshot flow`.
- Do not push or open a PR unless the operator instructs it.

## Steps

### Step 1: Add helpers (filename sanitize, screen filter, byte download)

In `agent.go`, add unexported helpers:

1. `func sanitizeFilename(s string) string`
   - Lowercase optional; replace every rune not in `[A-Za-z0-9._-]` with `-`.
   - Collapse runs of `-` into one; trim leading/trailing `-` and `.`.
   - Truncate to a sane max (e.g. 60 chars).
   - If the result is empty, return `"node"`.
   - This strips emoji and slashes from path labels like
     `🕹️ Prototype / Prototype / Signup` → `prototype-prototype-signup`.

2. `func isLikelyScreenNode(n agentNodeSummary) bool`
   - Returns false for obvious chrome/structure so `shot` does not render junk.
   - Reject when `n.Name` (case-insensitive, trimmed) is:
     - purely numeric (`^\d+$`), or
     - in a stoplist: `status bar`, `home indicator`, `scrims`, `wordmark`,
       `content container`, `bottom nav (pain)`, `button groups`, `top`, or
     - matches `^group \d+$` or `^frame \d+`.
   - Reject `n.Type` of `GROUP` and `VECTOR` and `BOOLEAN_OPERATION`.
   - Keep it deterministic and table-simple; do not over-engineer. A few
     missed junk names are acceptable — the goal is to avoid the worst noise,
     not perfect classification.

3. `func downloadToFile(httpClient *http.Client, srcURL, destPath string) (int64, error)`
   - GET `srcURL` with the provided client (NOT the Figma `client.Client`; the
     render URL is a pre-signed S3 link and must not receive `X-Figma-Token`).
   - On non-2xx, return an error including the status.
   - Cap the body to a sane maximum (e.g. 40 MiB via `io.LimitReader`) so a
     bad URL cannot fill the disk.
   - Write atomically: `destPath + ".tmp"` then `os.Rename`, mirroring
     `deliverFile` in `deliver.go`. Create the parent dir with `0o755`.
   - Return bytes written.

**Verify**: `cd library/productivity/figma && go build ./internal/...` → exit 0.

### Step 2: Add unit tests for the helpers

In `agent_test.go`, add table-driven tests (style: `frame_test.go`):

- `TestSanitizeFilename` — emoji/slash/space inputs → safe slugs; empty →
  `"node"`; long input truncated.
- `TestIsLikelyScreenNode` — accepts a `FRAME` named `Cash transfer Intro`;
  rejects names `1`, `Group 13`, `Status Bar`, `Home Indicator`, and types
  `GROUP`/`VECTOR`.

**Verify**: `cd library/productivity/figma && go test ./internal/cli` → exit 0.

### Step 3: Add `figma-pp-cli agent shot`

In `agent.go`, add `func newAgentShotCmd(flags *rootFlags) *cobra.Command` and
register it in `newAgentCmd` after `newAgentFindNodeCmd(flags)`:

```go
cmd.AddCommand(newAgentOutlineCmd(flags))
cmd.AddCommand(newAgentFindNodeCmd(flags))
cmd.AddCommand(newAgentShotCmd(flags))
```

Command shape:

```bash
figma-pp-cli agent shot <figma-url-or-key> [query] \
  --max 3 --scale 2 --format png --depth 3 \
  --types SECTION,FRAME,COMPONENT,INSTANCE \
  --out-dir <dir> --no-download --agent
```

Flags (with defaults): `--max int` (3), `--scale float` (2),
`--format string` (`png`), `--depth int` (3),
`--types string` (`SECTION,FRAME,COMPONENT,INSTANCE`),
`--out-dir string` (default: `filepath.Join(os.TempDir(), "figma-pp-cli")`),
`--no-download bool` (false).

Annotate read-only: `Annotations: map[string]string{"mcp:read-only": "true"}`.

Behavior:

1. `if len(args) == 0 { return cmd.Help() }`.
2. `ref, err := parseFigmaRef(args[0])`; on error `return usageErr(err)`.
3. `query := ""; if len(args) >= 2 { query = strings.Join(args[1:], " ") }`.
4. If `dryRunOK(flags)`, print a dry-run envelope (mirror the `outline`/`find-node`
   dry-run shape: `command: "agent shot"`, `file_key`, `node_id`, `query`,
   `method: "GET"`, `path: "/v1/images/<key>"`, `params` with `format`/`scale`)
   and return.
5. `c, err := flags.newClient()`.
6. **Resolve target node ids** into an ordered, de-duplicated `[]agentMatch`:
   - **Direct hit**: `if query == "" && ref.NodeID != ""` → one match with
     `ID: ref.NodeID` (you may fetch `/v1/files/<key>/nodes?ids=<id>&depth=1`
     to fill `Name`/`Type`/`Label` exactly like `agentFindNodeDirect`; reuse
     that logic — do not fabricate a name if the node is missing, return its
     error).
   - **Query path**: require a non-empty query else
     `return usageErr(fmt.Errorf("a query is required (or supply a Figma URL with a node-id)"))`.
     Fetch `/v1/files/<key>?depth=<depth>`, decode `{name, document}`,
     `buildAgentNodeIndex(doc, parseTypeSet(types), 0)`, score every node with
     `scoreAgentNode`, drop score 0, drop `!isLikelyScreenNode(n)`, sort by
     score desc then shorter label then lexical (same comparator as
     `find-node`).
   - **No matches** → exit 0 with `{file_key, query, count: 0, images: [],
     ambiguous: false, next_steps: [...]}` where `next_steps` tells the agent
     to run `agent outline`. Do not error.
   - **Ambiguity guard (preserve the 001 no-guess contract)**: if `max == 1`
     and the top two matches tie on score, exit 0 with `ambiguous: true`,
     `images: []`, and the tied candidates under `matches`, and a `next_steps`
     hint to re-run with a more specific label or `--max`. Do NOT render.
   - Otherwise take the top `max` matches (de-duplicate by `ID`).
7. **Render**: GET `/v1/images/<key>` with params
   `{ids: <comma-joined ids>, format: <format>, scale: fmt.Sprintf("%g", scale)}`.
   Decode `{ err string|null, images map[string]*string }` (a `null` URL means
   that node did not render — keep it, with `render_error: "no image returned"`).
   On `c.Get` error → `return classifyAPIError(err, flags)`.
8. **Download** (unless `--no-download`): build one
   `&http.Client{Timeout: flags.timeout}` (fall back to 60s if zero). For each
   match with a non-nil URL, compute
   `dest := filepath.Join(outDir, sanitizeFilename(label)+"-"+sanitizeFilename(id)+"."+ext)`
   where `ext` is `format` (`jpg`→`jpg`, etc.). Call `downloadToFile`. On
   success record `path` + `bytes`; on failure record `download_error` and keep
   the `url`. Never abort the loop on a single download failure.
9. **Output** compact JSON via `printJSONFiltered`:

```json
{
  "file_key": "abc123",
  "query": "Cash transfer Intro",
  "count": 1,
  "ambiguous": false,
  "out_dir": "/tmp/figma-pp-cli",
  "images": [
    {
      "id": "26076:77921",
      "name": "Cash transfer Intro",
      "type": "INSTANCE",
      "label": "🕹️ Prototype / Prototype / Cash transfer Intro",
      "score": 100,
      "url": "https://figma-alpha-api.s3...",
      "path": "/tmp/figma-pp-cli/prototype-prototype-cash-transfer-intro-26076_77921.png",
      "bytes": 51234
    }
  ],
  "next_steps": [
    "Send a screenshot in Slack by attaching the local file at images[].path.",
    "If path is missing, the render CDN was unreachable; use images[].url instead (it expires)."
  ]
}
```

Exit codes: usage/no-query/invalid-arg → 2 (via `usageErr`). API failure →
`classifyAPIError`. Everything else (no matches, ambiguous, partial download
failures) → exit 0; these are normal agent branches.

**Verify**: `cd library/productivity/figma && go build ./cmd/figma-pp-cli` → exit 0.

### Step 4: Add command-path tests

In `agent_test.go`, add a fake server that serves the file tree, the render
JSON, and the render bytes, e.g.:

```go
func newFakeFigmaShotServer(t *testing.T) *httptest.Server {
    t.Helper()
    mux := http.NewServeMux()
    var base string // set after server starts; use a closure or set via field
    mux.HandleFunc("/v1/files/", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        _, _ = w.Write([]byte(figmaFileFixture()))
    })
    mux.HandleFunc("/v1/images/", func(w http.ResponseWriter, r *http.Request) {
        id := r.URL.Query().Get("ids")
        w.Header().Set("Content-Type", "application/json")
        // point the render URL back at this same server's /render route
        fmt.Fprintf(w, `{"err":null,"images":{%q:%q}}`, id, base+"/render/"+id+".png")
    })
    mux.HandleFunc("/render/", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "image/png")
        _, _ = w.Write([]byte("\x89PNG\r\n\x1a\nFAKEPNGBYTES"))
    })
    srv := httptest.NewServer(mux)
    base = srv.URL
    return srv
}
```

(Resolve the `base` chicken-and-egg however is cleanest — e.g. build the mux,
start the server, then set a package-local or closure variable before the
first request; tests are single-threaded enough that setting `base = srv.URL`
immediately after `httptest.NewServer` is fine.)

Use `figmaFileFixture()` — confirm it contains a frame whose label/name passes
`isLikelyScreenNode` and matches a query you test (if not, query by a name the
fixture already has, e.g. `Signup`). Tests to add:

- `TestAgentShotDownloadsFile` — `agent shot testKey Signup --max 1 --out-dir <tmp> --agent`
  → exit 0; JSON `count == 1`; `images[0].path` exists on disk and is non-empty;
  `images[0].url` is set.
- `TestAgentShotNoDownloadReturnsURLOnly` — same with `--no-download` → JSON has
  `url`, no `path`; assert no file was written under the temp dir.
- `TestAgentShotAmbiguousMaxOneDoesNotRender` — craft a query that ties two
  same-name nodes in the fixture (or add a tiny dedicated fixture) with
  `--max 1` → `ambiguous: true`, `images: []`, exit 0.
- `TestAgentShotNoMatch` — nonsense query → `count: 0`, `images: []`, exit 0,
  `next_steps` mentions `agent outline`.

Use `t.TempDir()` for `--out-dir`. No live Figma, no real credentials.

**Verify**: `cd library/productivity/figma && go test ./internal/cli` → exit 0.

### Step 5: Patch metadata

Create `library/productivity/figma/.printing-press-patches/agent-screenshot-flow.json`:

```json
{
  "schema_version": 2,
  "id": "agent-screenshot-flow",
  "applied_at": "2026-06-15",
  "base_run_id": "20260509-190854",
  "base_printing_press_version": "4.2.0",
  "summary": "Added read-only 'agent shot' command: resolve a label or URL node-id, filter to screen-like nodes, render via /v1/images, and download PNGs to local files in one call.",
  "reason": "Turning a human prompt into a Slack-ready screenshot previously required 4-5 separate CLI calls across agent turns, and no command could produce a local image file (images emits expiring S3 URLs only). agent shot collapses the flow into one read-only call and returns both a local file path and the URL fallback.",
  "files": [
    "internal/cli/agent.go",
    "internal/cli/agent_test.go",
    "README.md",
    "SKILL.md"
  ],
  "validated_outcome": "go test ./internal/cli, go test ./..., go vet ./..., go build ./cmd/figma-pp-cli, and help/version smoke checks pass. Command-path tests use httptest.Server (file tree + /v1/images + render bytes) with FIGMA_BASE_URL; no live Figma credential is required."
}
```

Update `validated_outcome` truthfully if results differ.

**Verify**: `python3 -m json.tool library/productivity/figma/.printing-press-patches/agent-screenshot-flow.json >/dev/null` → exit 0.

### Step 6: Document the command

1. In `SKILL.md`, under "Recipes" near the existing
   "Resolve Figma labels to node ids first" section, add:

````markdown
### One-shot screenshot from a prompt

```bash
figma-pp-cli agent shot <figma-url-or-key> "Cash transfer Intro" --max 3 --agent
```

Resolves the label (or a `?node-id=` in the URL), filters to screen-like
nodes, renders PNGs, and downloads them to local files. Returns
`{images: [{id, label, type, url, path}]}`. Attach `images[].path` directly in
Slack. If `path` is missing, the render CDN was unreachable — use the
(expiring) `url`. Render bytes come from a Figma S3 host distinct from
`api.figma.com`; in brokered/egress-gateway setups that host must be allowed
for local download to succeed, otherwise the command degrades to URL-only.
````

2. In `README.md`, add `agent shot` to the highlights/examples list, concise.

**Verify**: docs render (no broken fences); `go run ./cmd/figma-pp-cli agent shot --help` matches the documented flags.

### Step 7: Final verification gates

```bash
cd library/productivity/figma
go test ./...
go vet ./...
go build ./cmd/figma-pp-cli
go run ./cmd/figma-pp-cli agent --help
go run ./cmd/figma-pp-cli agent shot --help
go run ./cmd/figma-pp-cli --version
git status --short
```

Expected: all tests pass; vet clean; build ok; `agent --help` lists `shot`;
`agent shot --help` shows `--max`, `--scale`, `--out-dir`, `--no-download`;
version prints; `git status --short` shows only in-scope files.

## Test plan

New tests in `agent_test.go`:

- Helpers: `TestSanitizeFilename`, `TestIsLikelyScreenNode`.
- Command-path: `TestAgentShotDownloadsFile`,
  `TestAgentShotNoDownloadReturnsURLOnly`,
  `TestAgentShotAmbiguousMaxOneDoesNotRender`, `TestAgentShotNoMatch`.
- No live Figma calls; render bytes served by the in-process fake server.

Verification:

```bash
cd library/productivity/figma && go test ./internal/cli ./internal/client ./cmd/figma-pp-cli
```

Expected: exit 0; all new tests pass.

## Done criteria

- [ ] `figma-pp-cli agent --help` lists `shot` alongside `outline`/`find-node`.
- [ ] `agent shot` accepts a full Figma URL and a raw file key, with an optional label query.
- [ ] With a resolvable label/URL, `agent shot` writes one or more local image files and returns their `path` plus `url`.
- [ ] `--no-download` returns `url` only and writes no files.
- [ ] A single download failure does not fail the command; it surfaces `download_error` and still exits 0.
- [ ] `--max 1` with a top-score tie returns `ambiguous: true` and renders nothing (no-guess contract preserved).
- [ ] No-match queries exit 0 with `count: 0` and an `agent outline` hint.
- [ ] No Figma write/auth/credential behavior changed; `client.go` and `promoted_images.go` unmodified.
- [ ] `agent-screenshot-flow.json` exists and validates as JSON.
- [ ] `go test ./...`, `go vet ./...`, `go build ./cmd/figma-pp-cli` all exit 0.
- [ ] `git status --short` shows only in-scope files.
- [ ] `plans/README.md` status row updated if you maintain the index.

## STOP conditions

Stop and report (do not improvise) if:

- The `agent.go` helper signatures in "Current state" no longer match after the drift check.
- Implementing `agent shot` requires changing `client.go`, `promoted_images.go`, auth, or any out-of-scope file.
- `figmaFileFixture()` has no frame that both passes `isLikelyScreenNode` and matches a test query, AND adding a small dedicated fixture would require modifying existing shared fixtures/tests rather than adding new ones.
- The download step cannot be implemented with the standard library without a new dependency.
- Tests would require a live Figma token or real network egress to pass.
- Adding `agent shot` conflicts with existing command/flag names in a way not fixable locally.

## Maintenance notes

- `agent shot` is a thin read-only convenience over existing primitives
  (`parseFigmaRef` + index/scorer + `/v1/images`). Keep the renderable-node
  filter (`isLikelyScreenNode`) conservative; if it starts hiding real screens,
  prefer widening the stoplist over inverting to an allowlist.
- The download path is the one place this command touches a non-`api.figma.com`
  host. If a future deployment blocks the S3 render CDN, the command already
  degrades to URL-only — verify that behavior still holds in review.
- A natural follow-up (separate plan): `agent screens <url>` that lists only
  screen-like top-level frames as a clean discovery menu, and demoting chrome/
  numeric labels inside `find-node`'s scorer so plain discovery is less noisy.
- Reviewers: scrutinize (1) that `X-Figma-Token` is never sent to the render
  URL, (2) the body size cap on download, (3) atomic file writes, and (4) that
  the no-guess ambiguity contract from Plan 001 is preserved for `--max 1`.
