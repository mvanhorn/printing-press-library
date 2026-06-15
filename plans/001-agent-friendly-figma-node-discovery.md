# Plan 001: Add agent-friendly Figma node discovery commands

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md` — unless a reviewer dispatched you and told you they
> maintain the index.
>
> **Drift check (run first)**: `git diff --stat 2a98659ce..HEAD -- library/productivity/figma/internal/cli/root.go library/productivity/figma/internal/cli/frame.go library/productivity/figma/internal/cli/dev_mode.go library/productivity/figma/internal/cli/promoted_files.go library/productivity/figma/SKILL.md library/productivity/figma/README.md library/productivity/figma/.printing-press-patches`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: M
- **Risk**: LOW
- **Depends on**: none
- **Category**: dx
- **Planned at**: commit `2a98659ce`, 2026-06-15
- **Issue**: https://github.com/dhruvkelawala/printing-press-library/issues/1

## Why this matters

The Figma CLI already exposes strong read-only primitives (`files`, `nodes`, `images`, `frame extract`, `dev-mode dump`), but agents must know raw Figma node IDs before using most of them. That forces an LLM to fetch large file trees, manually search JSON, and copy IDs, which is brittle and token-expensive. Add a small read-only `agent` command group that accepts Figma URLs/file keys, produces compact path labels, and resolves human labels like "Prototype" into node IDs. This makes Mario/OpenClaw-style agents able to use Figma files by labels while preserving the existing generated REST command surface.

## Current state

Relevant files and roles:

- `library/productivity/figma/internal/cli/root.go` — Cobra root command registration.
- `library/productivity/figma/internal/cli/frame.go` — handwritten agent-oriented frame extraction and existing node ID normalization helpers.
- `library/productivity/figma/internal/cli/dev_mode.go` — handwritten read-only one-node Dev Mode bundle command.
- `library/productivity/figma/internal/cli/promoted_files.go` — generated `files <file_key>` command; this is the current way to fetch shallow file trees.
- `library/productivity/figma/internal/cli/frame_test.go` — existing style for simple helper tests in this package.
- `library/productivity/figma/.printing-press-patches/` — patch metadata for hand-edits to generated/curated CLI output.
- `library/productivity/figma/SKILL.md` and `library/productivity/figma/README.md` — user/agent docs for the published CLI.

Current root registration (`library/productivity/figma/internal/cli/root.go:182-219`) registers generated command families and the handwritten `frame` / `dev-mode` commands, but no label-oriented helper group:

```go
rootCmd.AddCommand(newAgentContextCmd(rootCmd))
rootCmd.AddCommand(newProfileCmd(flags))
rootCmd.AddCommand(newFeedbackCmd(flags))
rootCmd.AddCommand(newWhichCmd(flags))
// ...
rootCmd.AddCommand(newFilesPromotedCmd(flags))
rootCmd.AddCommand(newImagesPromotedCmd(flags))
// ...
rootCmd.AddCommand(newFrameCmd(flags))
rootCmd.AddCommand(newDevModeCmd(flags))
rootCmd.AddCommand(newCommentsAuditCmd(flags))
rootCmd.AddCommand(newTokensCmd(flags))
rootCmd.AddCommand(newFingerprintCmd(flags))
rootCmd.AddCommand(newVariablesCmd(flags))
```

`frame.go` already has good node ID normalization helpers (`library/productivity/figma/internal/cli/frame.go:15-50`) that should be reused rather than duplicated:

```go
func normalizeNodeID(s string) string {
    s = strings.TrimSpace(s)
    if s == "" {
        return s
    }
    parts := strings.Split(s, ";")
    for i, p := range parts {
        parts[i] = strings.ReplaceAll(p, "-", ":")
    }
    return strings.Join(parts, ";")
}

func normalizeNodeIDList(ids []string) string { /* ... */ }
```

`frame extract` still requires node IDs (`library/productivity/figma/internal/cli/frame.go:93-97`):

```go
key := args[0]
normIDs := normalizeNodeIDList(ids)
if normIDs == "" {
    return usageErr(fmt.Errorf("--ids is required (one or more node ids, comma-separated or repeated)"))
}
```

`dev-mode dump` has the same one-node ID requirement (`library/productivity/figma/internal/cli/dev_mode.go:49-61`):

```go
if strings.TrimSpace(node) == "" {
    return usageErr(fmt.Errorf("--node is required"))
}
key := args[0]
normID := normalizeNodeID(node)
// ...
nodesRaw, err := c.Get("/v1/files/"+key+"/nodes", map[string]string{"ids": normID, "depth": "2"})
```

`files <file_key>` can fetch the tree but emits the full REST shape; it is not a compact label index (`library/productivity/figma/internal/cli/promoted_files.go:23-28`, `49-68`):

```go
Use:         "files <file_key>",
Annotations: map[string]string{"pp:endpoint": "files.get", "pp:method": "GET", "pp:path": "/v1/files/{file_key}", "mcp:read-only": "true"},
// ...
path = replacePathParam(path, "file_key", args[0])
params := map[string]string{}
if flagDepth != 0.0 {
    params["depth"] = fmt.Sprintf("%v", flagDepth)
}
data, prov, err := resolveRead(cmd.Context(), c, flags, "files", false, path, params, nil)
```

Existing tests use simple table-driven helpers in package `cli` (`library/productivity/figma/internal/cli/frame_test.go:7-36`):

```go
func TestNormalizeNodeID(t *testing.T) {
    cases := []struct {
        name string
        in   string
        want string
    }{ /* ... */ }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) { /* ... */ })
    }
}
```

Repo conventions to honor:

- This is the published library repo, not the generator repo. `AGENTS.md:13-17` says broken or useful published CLI behavior can be patched in `library/<cat>/<slug>/`, but hand-edits must be recorded in `.printing-press-patches/`.
- `AGENTS.md:77-85` says behavior tweaks in existing published CLI code are acceptable; do not edit `registry.json`, generated `cli-skills/pp-*`, release files, or broad generated artifacts.
- `CONTRIBUTING.md:31-49` lists quality gates: `go build ./...`, `go vet ./...`, `--help`, `--version`.
- The Figma module is standalone at `library/productivity/figma/go.mod` with module path `github.com/mvanhorn/printing-press-library/library/productivity/figma` and Go `1.26.3`.
- Existing Figma patch metadata uses JSON files under `library/productivity/figma/.printing-press-patches/`, for example `pat-auth-wiring.json` explains the reason, files changed, and validation outcome.

## Commands you will need

Run these from the repo root unless the command says otherwise.

| Purpose | Command | Expected on success |
|---|---|---|
| Drift check | `git diff --stat 2a98659ce..HEAD -- library/productivity/figma/internal/cli/root.go library/productivity/figma/internal/cli/frame.go library/productivity/figma/internal/cli/dev_mode.go library/productivity/figma/internal/cli/promoted_files.go library/productivity/figma/SKILL.md library/productivity/figma/README.md library/productivity/figma/.printing-press-patches` | no output, or only changes you review against this plan before proceeding |
| Focused tests | `cd library/productivity/figma && go test ./internal/cli ./internal/client ./cmd/figma-pp-cli` | exit 0; `ok .../internal/cli`; no failures |
| Full Figma tests | `cd library/productivity/figma && go test ./...` | exit 0; all package tests pass |
| Vet | `cd library/productivity/figma && go vet ./...` | exit 0, no diagnostics |
| Build | `cd library/productivity/figma && go build ./cmd/figma-pp-cli` | exit 0; binary builds |
| Help smoke | `cd library/productivity/figma && go run ./cmd/figma-pp-cli agent --help` | exit 0; shows `outline` and `find-node` subcommands |
| Version smoke | `cd library/productivity/figma && go run ./cmd/figma-pp-cli --version` | exit 0; prints `figma-pp-cli 2026.6.1` or current release-ledger value if it changed upstream |

## Scope

**In scope** (the only source files you should modify):

- `library/productivity/figma/internal/cli/agent.go` — create; new read-only `agent` command group and helpers.
- `library/productivity/figma/internal/cli/agent_test.go` — create; helper and command tests.
- `library/productivity/figma/internal/cli/root.go` — add `rootCmd.AddCommand(newAgentCmd(flags))`.
- `library/productivity/figma/.printing-press-patches/agent-node-discovery.json` — create patch metadata for this hand-edit.
- `library/productivity/figma/SKILL.md` — document the new agent-friendly commands.
- `library/productivity/figma/README.md` — document the new agent-friendly commands.
- `plans/README.md` — update status row when finished, if you are the executor maintaining the index.

**Out of scope** (do NOT touch, even though they look related):

- `registry.json` and `cli-skills/pp-figma/SKILL.md` — generated after merge by repo automation.
- `.printing-press-release.json`, `CHANGELOG.md`, and runtime version values — release-ledger automation owns them after merge.
- `tools-manifest.json` / `manifest.json` / `internal/mcp/*` — keep this first slice CLI-first. If MCP mirroring picks up the Cobra command automatically, that is acceptable, but do not hand-edit MCP manifests.
- Figma write commands, auth commands, or credential behavior.
- Any non-Figma CLI under `library/**`.
- Any generator repo changes in `cli-printing-press`.

## Git workflow

- Branch: continue on `figma-agent-friendly` unless the operator tells you to rename it.
- Commit style: conventional commits. Use `feat(figma): add agent node discovery` for the implementation commit.
- Do not push or open a PR unless the operator explicitly instructs it.
- Before working on a public issue, follow `AGENTS.md:29-61`: check whether the issue is already claimed, then comment a claim. If no issue exists or GitHub auth is unavailable, proceed locally and report that claim/publish was skipped.

## Steps

### Step 1: Add URL/file-key parsing and compact node indexing helpers

Create `library/productivity/figma/internal/cli/agent.go` with package `cli`. Add pure helpers before wiring Cobra commands:

1. `type figmaRef struct { FileKey string; NodeID string; Raw string }`
2. `parseFigmaRef(raw string) (figmaRef, error)`:
   - Accept raw file keys like `JZyB6K6Z22YyObBdj1r4v1`.
   - Accept `https://www.figma.com/design/<file_key>/<title>?node-id=123-456`.
   - Accept `https://www.figma.com/file/<file_key>/<title>?node-id=123-456`.
   - Optionally accept `/proto/<file_key>/...` because Figma prototype links use the same key shape.
   - Extract `node-id` or `node_id` query values when present and normalize with existing `normalizeNodeID`.
   - Return a usage-style error for URLs that do not contain a file key.
3. `type agentNodeSummary struct` with JSON fields:
   - `id string`
   - `name string`
   - `type string`
   - `path []string`
   - `label string` (path joined with ` / `)
   - `parent_id string,omitempty`
   - `child_count int`
   - `absolute_bounding_box map[string]any,omitempty` or a typed small struct if you prefer.
4. `buildAgentNodeIndex(root map[string]any, allowedTypes map[string]bool, maxDepth int) []agentNodeSummary`:
   - Walk the Figma document tree depth-first.
   - Include `CANVAS`, `SECTION`, `FRAME`, `COMPONENT`, `INSTANCE`, and `GROUP` by default.
   - Always include enough path context to disambiguate duplicate node names.
   - Count children without dumping the full children arrays.
5. `scoreAgentNode(query string, node agentNodeSummary) int`:
   - Case-insensitive exact name match should score highest.
   - Case-insensitive exact label match next.
   - Substring name/label matches next.
   - Token matches across label next.
   - Return 0 for no match.
   - Keep this deterministic; do not add non-deterministic fuzzy libraries.

Keep the helpers unexported. Use only standard library packages already acceptable in the module (`net/url`, `strings`, `sort`, `encoding/json`, `fmt` are enough).

**Verify**: `cd library/productivity/figma && go test ./internal/cli` → exits 0. It is okay that no new tests exist yet at this step if the package still compiles.

### Step 2: Add unit tests for parsing, indexing, and scoring

Create `library/productivity/figma/internal/cli/agent_test.go`. Follow the table-driven style in `frame_test.go`.

Required tests:

- `TestParseFigmaRefRawKey` — raw key returns `FileKey` and empty `NodeID`.
- `TestParseFigmaRefDesignURL` — `/design/<key>/... ?node-id=123-456` returns key and `123:456`.
- `TestParseFigmaRefFileURL` — `/file/<key>/... ?node_id=123-456` returns key and `123:456`.
- `TestParseFigmaRefRejectsUnknownURL` — non-Figma/path-without-key returns an error.
- `TestBuildAgentNodeIndexIncludesPathLabels` using a small inline Figma-like fixture:
  - root `DOCUMENT` → page `CANVAS` named `🕹️ Prototype` → section `SECTION` named `Prototype` → frame `FRAME` named `Signup`.
  - Assert label is exactly `🕹️ Prototype / Prototype / Signup` for the frame.
  - Assert `child_count` is set.
- `TestScoreAgentNodePrefersExactNameOverSubstring`.
- `TestScoreAgentNodeFindsTokenMatchesInLabel`.

Do not use live Figma API calls in tests.

**Verify**: `cd library/productivity/figma && go test ./internal/cli` → exits 0 with the new tests passing.

### Step 3: Add `figma-pp-cli agent outline`

In `agent.go`, add:

```go
func newAgentCmd(flags *rootFlags) *cobra.Command
func newAgentOutlineCmd(flags *rootFlags) *cobra.Command
```

Command shape:

```bash
figma-pp-cli agent outline <figma-url-or-key> --depth 2 --types CANVAS,SECTION,FRAME,INSTANCE,COMPONENT,GROUP --limit 200 --agent
```

Behavior:

1. Parse the argument with `parseFigmaRef`.
2. If `--dry-run` is set, print a small JSON dry-run envelope and do not make network calls. Match existing `dryRunOK(flags)` patterns if possible.
3. Fetch `/v1/files/<file_key>` with the requested `depth` through `flags.newClient()` and `c.Get(...)` or `c.GetWithHeaders(...)`.
4. Decode the response. The Figma `files` endpoint returns a top-level object with `name` and `document`; tests should use this shape.
5. Build node summaries with `buildAgentNodeIndex`.
6. Respect `--limit` by truncating the result slice after traversal. If there are more results than the limit, include `truncated: true` and `total_before_limit`.
7. Output JSON via `printJSONFiltered(cmd.OutOrStdout(), out, flags)` with a compact shape:

```json
{
  "file_key": "abc123",
  "file_name": "Example Design",
  "depth": 2,
  "count": 3,
  "truncated": false,
  "nodes": [
    {
      "id": "1:2",
      "name": "Signup",
      "type": "FRAME",
      "path": ["Prototype", "Signup"],
      "label": "Prototype / Signup",
      "parent_id": "1:1",
      "child_count": 4
    }
  ]
}
```

Cobra metadata:

- Annotate the command read-only: `Annotations: map[string]string{"mcp:read-only": "true"}`.
- Keep examples focused on full Figma URLs and raw keys.

**Verify**: `cd library/productivity/figma && go test ./internal/cli` → exits 0.

### Step 4: Add `figma-pp-cli agent find-node`

In `agent.go`, add `newAgentFindNodeCmd(flags *rootFlags)` and register it under `newAgentCmd`.

Command shape:

```bash
figma-pp-cli agent find-node <figma-url-or-key> <query> --depth 3 --types SECTION,FRAME,INSTANCE,COMPONENT --limit 20 --agent
```

Behavior:

1. Parse the Figma ref.
2. If the URL contains `node-id` and the query is omitted, return that node as a resolved direct hit only if you can fetch it with `/v1/files/<key>/nodes?ids=<node_id>&depth=1`; otherwise keep requiring a query. Do not invent a name.
3. Fetch a shallow file tree with `/v1/files/<key>?depth=<depth>`.
4. Build the node index with the requested types.
5. Score every node with `scoreAgentNode`.
6. Sort by descending score, then shorter label, then stable lexical label.
7. Return a JSON envelope:

```json
{
  "file_key": "abc123",
  "query": "Prototype",
  "match_count": 2,
  "ambiguous": true,
  "best": {
    "id": "25728:160141",
    "name": "Prototype",
    "type": "SECTION",
    "label": "🕹️ Prototype / Prototype",
    "score": 100
  },
  "matches": [
    { "id": "25728:160141", "name": "Prototype", "type": "SECTION", "label": "🕹️ Prototype / Prototype", "score": 100 },
    { "id": "25012:56965", "name": "pain", "type": "SECTION", "label": "💎 Templates & Symbols / pain", "score": 50 }
  ],
  "next_steps": [
    "Use the chosen id with: figma-pp-cli files nodes get-file abc123 --ids <id> --depth 2 --agent",
    "Render it with: figma-pp-cli images abc123 --ids <id> --format png --scale 1 --agent"
  ]
}
```

Ambiguity rules:

- `ambiguous` should be `true` when more than one match has the same top score.
- Do not exit non-zero just because matches are ambiguous; ambiguity is a normal agent workflow. Exit 0 with candidates.
- Exit code 2 / usage error only for invalid arguments or no query.
- For no matches, exit 0 with `match_count: 0`, `matches: []`, and a helpful `next_steps` hint to run `agent outline`. This is easier for agents to handle than a hard failure.

**Verify**: `cd library/productivity/figma && go test ./internal/cli` → exits 0.

### Step 5: Register the command group and add command-level tests

Modify `library/productivity/figma/internal/cli/root.go` to add the new command group. Put it near `agent-context` so discovery-related commands are grouped together:

```go
rootCmd.AddCommand(newAuthCmd(flags))
rootCmd.AddCommand(newAgentCmd(flags))
rootCmd.AddCommand(newAgentContextCmd(rootCmd))
```

Add tests in `agent_test.go` that exercise the Cobra commands with a local `httptest.Server`:

- Set `FIGMA_BASE_URL` to the test server URL for the duration of the test.
- Use `t.Setenv("FIGMA_BASE_URL", server.URL)`.
- Set `FIGMA_API_TOKEN` or write a temp config if auth is required by the client path; use a dummy token only, never a real credential.
- Instantiate `flags := rootFlags{}` and `cmd := newRootCmd(&flags)` from package `cli`.
- Set args for `agent outline` and `agent find-node`.
- Set stdout/stderr buffers.
- Have the fake server assert the requested path is `/v1/files/testKey` and return a small fixture with `document.children`.
- Assert the JSON output includes expected labels and IDs.

If `newRootCmd` command tests are too brittle because root global state (`noColor`, `humanFriendly`) leaks between tests, keep the command tests minimal and rely on helper tests plus help smoke. Do not spend more than one reasonable attempt fighting global state; record the limitation in the patch metadata validation outcome.

**Verify**: `cd library/productivity/figma && go test ./internal/cli` → exits 0.

### Step 6: Document the commands and patch metadata

Update docs:

1. In `library/productivity/figma/SKILL.md`, add a short "Agent navigation" section near the top-level recipes:

```bash
figma-pp-cli agent outline <figma-url-or-key> --depth 2 --agent
figma-pp-cli agent find-node <figma-url-or-key> "Prototype" --depth 3 --agent
```

Explain that agents should resolve labels first, then pass the returned `id` to existing `files nodes`, `images`, `frame extract`, or `dev-mode dump` commands.

2. In `library/productivity/figma/README.md`, add the same commands under highlights or examples. Keep this concise; do not rewrite the full README.

3. Create `library/productivity/figma/.printing-press-patches/agent-node-discovery.json` with schema version 2. Use this shape:

```json
{
  "schema_version": 2,
  "id": "agent-node-discovery",
  "applied_at": "2026-06-15",
  "base_run_id": "20260509-190854",
  "base_printing_press_version": "4.2.0",
  "summary": "Added read-only agent outline and find-node commands for compact Figma node label discovery.",
  "reason": "Figma's REST node/image/frame commands require node IDs, but agents usually receive full Figma URLs or human labels. The new commands parse Figma URLs, build compact path labels from shallow file trees, and resolve label queries to node IDs without exposing write operations.",
  "files": [
    "internal/cli/agent.go",
    "internal/cli/agent_test.go",
    "internal/cli/root.go",
    "README.md",
    "SKILL.md"
  ],
  "validated_outcome": "go test ./internal/cli, go test ./..., go vet ./..., go build ./cmd/figma-pp-cli, and help/version smoke checks pass. No live Figma credential is required for tests."
}
```

If the real validation outcome differs, update that field truthfully.

**Verify**: `python3 -m json.tool library/productivity/figma/.printing-press-patches/agent-node-discovery.json >/dev/null` → exits 0.

### Step 7: Run final verification gates

Run the full Figma-module gates:

```bash
cd library/productivity/figma
go test ./...
go vet ./...
go build ./cmd/figma-pp-cli
go run ./cmd/figma-pp-cli agent --help
go run ./cmd/figma-pp-cli agent outline --help
go run ./cmd/figma-pp-cli agent find-node --help
go run ./cmd/figma-pp-cli --version
```

Expected results:

- All `go test` packages pass.
- `go vet` exits 0 with no diagnostics.
- `go build` exits 0.
- Help commands exit 0 and show the new command names.
- Version command prints `figma-pp-cli ...`.

Then check scope:

```bash
git status --short
```

Expected: only the in-scope files from this plan are modified or added.

## Test plan

New tests go in `library/productivity/figma/internal/cli/agent_test.go`.

Required coverage:

- URL parsing for raw keys, `/design/`, `/file/`, optional `/proto/`, and invalid URLs.
- Node ID normalization from URL query (`123-456` → `123:456`).
- Tree indexing preserves path labels and child counts.
- Type filtering excludes disallowed node types.
- Scoring prefers exact name over substring/label token matches.
- `agent outline` against a fake server emits compact JSON with labels and IDs.
- `agent find-node` against a fake server returns stable sorted matches and `ambiguous: true` when top scores tie.
- No tests call live Figma.

Use `frame_test.go` as the style model for table-driven helper tests. Use `httptest.Server` only for command-path tests.

Verification:

```bash
cd library/productivity/figma && go test ./internal/cli ./internal/client ./cmd/figma-pp-cli
```

Expected: exit 0; all new tests pass.

## Done criteria

All must hold:

- [ ] `figma-pp-cli agent --help` lists `outline` and `find-node`.
- [ ] `agent outline` accepts a full Figma URL and raw file key.
- [ ] `agent find-node` accepts a full Figma URL and raw file key.
- [ ] Output from both commands is compact JSON and contains `file_key`, labels, node IDs, node types, and counts.
- [ ] Ambiguous matches return candidates instead of guessing or failing.
- [ ] No Figma write endpoints, auth flows, or credential handling changed.
- [ ] `library/productivity/figma/.printing-press-patches/agent-node-discovery.json` exists and validates as JSON.
- [ ] `cd library/productivity/figma && go test ./...` exits 0.
- [ ] `cd library/productivity/figma && go vet ./...` exits 0.
- [ ] `cd library/productivity/figma && go build ./cmd/figma-pp-cli` exits 0.
- [ ] `git status --short` shows only in-scope files.
- [ ] `plans/README.md` status row updated if the executor is responsible for the plan index.

## STOP conditions

Stop and report back (do not improvise) if:

- The root registration or existing `frame.go` / `dev_mode.go` excerpts do not match the current code after the drift check.
- Implementing the command requires changes outside the in-scope files.
- Tests require a live Figma token or network access to pass.
- The fake-server command tests require changing shared client behavior or auth behavior.
- You discover an existing upstream command already provides equivalent label-based node discovery.
- `go test ./internal/cli` fails for unrelated pre-existing reasons.
- Adding the `agent` Cobra command conflicts with existing command names or global `--agent` flag parsing in a way that cannot be fixed locally without broad root command changes.

## Maintenance notes

- This is an intentionally thin read-only layer over existing REST primitives. Future convenience commands like `agent image <url> <label>` or `agent extract <url> <label>` should reuse the same parser/index/scorer instead of reimplementing discovery.
- Keep output compact; agents should not receive full Figma REST node trees unless they explicitly call `files nodes` or `frame extract`.
- If this command proves useful across other printed CLIs with hierarchical resources, consider moving the pattern into the generator later. For this plan, keep the patch local to the published Figma CLI per `AGENTS.md` guidance.
- Reviewers should scrutinize deterministic matching and ambiguity handling. The command must not silently choose one of several same-score labels.
