# Plan 004: `agent index-files` — populate a known-files map from Figma projects/teams

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving on. If a
> "STOP condition" occurs, stop and report — do not improvise. When done,
> update this plan's status row in `plans/README.md` unless a reviewer told you
> they maintain the index.
>
> **Drift check (run first)**:
> `git diff --stat 47e979045..HEAD -- library/productivity/figma/internal/cli/agent.go library/productivity/figma/internal/cli/agent_test.go library/productivity/figma/internal/cli/projects_files_get-project.go library/productivity/figma/internal/cli/teams_projects_get-team.go library/productivity/figma/SKILL.md library/productivity/figma/README.md library/productivity/figma/.printing-press-patches`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P2
- **Effort**: M
- **Risk**: LOW (read-only Figma listing; the only write is an additive local-file merge behind an explicit flag)
- **Depends on**: Plan 001 (DONE) for `newAgentCmd`/`parseFigmaRef`/`printJSONFiltered`. Independent of 002/003.
- **Category**: dx
- **Planned at**: commit `47e979045`, 2026-06-16

## Why this matters

Agents resolve bare Figma names ("the Pain prototype") to a `file_key` via a
small **known-files map** (alias → file). Today that map is hand-maintained.
For a team with many files this is tedious and drifts. Figma's REST API can
enumerate files per project and projects per team, so a read-only
`agent index-files` command can generate (and optionally merge) known-files
entries automatically.

Scope boundary that keeps this clean: **file-level only.** This command maps
*files* (key, name, aliases), not screens/nodes inside a file. Node/screen
labels must stay live via `agent outline` / `find-node` / `agent shot` — baking
a node tree into a static file goes stale on every design edit and bloats. So
`index-files` produces a compact file registry; nothing more.

## Current state

Two generated commands already wrap the needed endpoints:

- `internal/cli/projects_files_get-project.go` — `GET /v1/projects/{project_id}/files`
  (`projects files get-project <project_id>`). Figma returns
  `{ "name": "...", "files": [ { "key": "...", "name": "...", "last_modified": "...", "thumbnail_url": "..." } ] }`.
- `internal/cli/teams_projects_get-team.go` — `GET /v1/teams/{team_id}/projects`
  (`teams projects get-team <team_id>`). Figma returns
  `{ "name": "...", "projects": [ { "id": "...", "name": "..." } ] }`.

`agent.go` exposes the `agent` group and reusable helpers:

```go
func newAgentCmd(flags *rootFlags) *cobra.Command   // registers outline, find-node, shot
func parseFigmaRef(raw string) (figmaRef, error)     // /design,/file,/proto,/board file URLs only
func sanitizeFilename(s string) string               // lowercased, hyphen-safe slug (reuse as alias base)
func printJSONFiltered(w io.Writer, v any, flags *rootFlags) error
func dryRunOK(flags *rootFlags) bool
func usageErr(err error) error
func classifyAPIError(err error, flags *rootFlags) error
```

`parseFigmaRef` does **not** handle team/project URLs (only file URLs). The
client is JSON-only: `c.Get(path, params) (json.RawMessage, error)`. The atomic
local-file write pattern to mirror for `--merge-into` is `deliverFile` in
`internal/cli/deliver.go` (tmp + rename).

The **consumer** known-files shape (for reference — this file lives in the
operator's workspace, NOT in this repo) is:

```json
{
  "_comment": "…",
  "files": {
    "pain": {
      "file_key": "JZyB6K6Z22YyObBdj1r4v1",
      "name": "PaiN (1.7) — V2",
      "url": "https://www.figma.com/design/JZyB6K6Z22YyObBdj1r4v1/PaiN--1.7--…",
      "aliases": ["pain", "pain prototype"],
      "notes": "…"
    }
  }
}
```

`index-files` must emit/merge **this exact shape** (`files` object keyed by
alias slug) so it can be dropped into any known-files.json.

Access note: listing projects/files requires the token to have project/file
metadata read scope (Figma `projects:read` / `file_metadata:read`). If the
token lacks it, the endpoints return 403; surface that via `classifyAPIError`
and stop (do not retry auth).

Repo conventions: published library repo — hand-edits recorded in
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
| Help smoke | `cd library/productivity/figma && go run ./cmd/figma-pp-cli agent index-files --help` | shows `--project`, `--team`, `--merge-into`, `--force` |
| Version smoke | `cd library/productivity/figma && go run ./cmd/figma-pp-cli --version` | prints `figma-pp-cli ...` |

## Scope

**In scope:**
- `library/productivity/figma/internal/cli/agent.go` — `newAgentIndexFilesCmd`, register under `newAgentCmd`, helpers (`parseProjectTeamRef`, `slugifyAlias`, `buildKnownFilesEntries`, `mergeKnownFiles`).
- `library/productivity/figma/internal/cli/agent_test.go` — fake project/team server + tests.
- `library/productivity/figma/.printing-press-patches/agent-index-files.json` — patch metadata.
- `library/productivity/figma/SKILL.md`, `library/productivity/figma/README.md` — document the command.
- `plans/README.md` — status row.

**Out of scope:** `client.go`, the generated `projects`/`teams` commands, `registry.json`, `cli-skills/**`, release files, MCP manifests, any Figma write/auth behavior, the operator's actual known-files.json (the CLI is generic — it emits/merges a path the caller provides), any non-Figma CLI, the generator repo.

## Steps

### Step 1: Parsers and helpers

In `agent.go` add:

1. `func parseProjectTeamRef(raw string) (kind string, id string, err error)`:
   - `kind` is `"project"` or `"team"`.
   - Accept raw ids only via the flags (Step 2); this parser handles **URLs**:
     `https://www.figma.com/files/project/<id>/...` → `("project", id)`;
     `https://www.figma.com/files/team/<id>/...` (and `.../files/<org>/team/<id>/...`) → `("team", id)`.
   - Return a usage error for unrecognized URLs.
2. `func slugifyAlias(name string) string` — alias base: reuse `sanitizeFilename` then ensure lowercase; it already lowercases, strips emoji/punct to hyphens, trims, caps length. Good enough; do not duplicate logic.
3. `type knownFile struct` with JSON tags matching the consumer shape:
   `FileKey, Name, URL string; LastModified string,omitempty; Project string,omitempty; Aliases []string`.
4. `func buildKnownFilesEntries(files []figmaFileMeta, project string) map[string]knownFile`:
   - For each file build `URL = "https://www.figma.com/design/" + key + "/" + url.PathEscape(name)`.
   - `Aliases = dedupe([slugifyAlias(name), strings.ToLower(strings.TrimSpace(name))])`.
   - Key the map by `slugifyAlias(name)`; on collision append `-2`, `-3`, … (deterministic).
   - Set `Project` when non-empty (team walk).

Standard library only (`net/url`, `strings`, `sort`, `encoding/json`, `fmt`).

**Verify**: `cd library/productivity/figma && go build ./internal/...` → exit 0.

### Step 2: The command

Add `func newAgentIndexFilesCmd(flags *rootFlags) *cobra.Command` and register
it in `newAgentCmd` after `newAgentShotCmd(flags)`.

Shape:

```bash
figma-pp-cli agent index-files (--project <id> | --team <id> | <project-or-team-url>) \
  [--merge-into <known-files.json>] [--force] [--agent]
```

Flags: `--project string`, `--team string`, `--merge-into string`, `--force bool`.
Annotate `mcp:read-only` for the listing (the optional merge writes a local file
only, never Figma).

Behavior:
1. Resolve source precedence: `--project` → `--team` → positional URL via
   `parseProjectTeamRef`. If none, `usageErr`.
2. `dryRunOK(flags)` → print a dry-run envelope (source kind/id, the endpoint(s)
   that would be called) and return.
3. `c, _ := flags.newClient()`.
4. **Project source**: `GET /v1/projects/<id>/files`, decode `{name, files:[{key,name,last_modified}]}`, `buildKnownFilesEntries(files, projectName)`.
   **Team source**: `GET /v1/teams/<id>/projects`, decode `{name, projects:[{id,name}]}`; for each project `GET /v1/projects/<pid>/files` and merge entries (carry the project name). On a per-project API error, record it under an `errors` array and continue (one bad project shouldn't sink the whole index).
5. Build the output object: `{ "_comment": "...", "source": {...}, "generated_at": <RFC3339>, "files": <map> }` (and `errors` if any).
6. If `--merge-into` is **unset**: print the object via `printJSONFiltered` (read-only; the caller can redirect with `--deliver file:<path>`).
7. If `--merge-into <path>` is set: call `mergeKnownFiles` (Step 3), then print a summary `{ "merged_into": path, "added": n, "skipped": n, "updated": n, "files": <final map> }`.

**Verify**: `cd library/productivity/figma && go build ./cmd/figma-pp-cli` → exit 0.

### Step 3: Additive merge

`func mergeKnownFiles(path string, generated map[string]knownFile, force bool) (added, skipped, updated int, err error)`:

1. Read `path` if it exists; decode into `{ "_comment"?, "files": map[string]json.RawMessage, ...passthrough }`. If the file does not exist, start from an empty map (and a default `_comment`).
2. For each generated alias:
   - If the alias is absent → add it (`added++`).
   - If present and `force` is false → **skip** (`skipped++`), preserving the existing entry verbatim (including any hand-written `notes`).
   - If present and `force` is true → overwrite (`updated++`) but **preserve the existing `notes` field** if the generated entry has none.
3. Preserve any top-level keys the file already had (`_comment`, etc.) and any existing entries not in the generated set.
4. Write atomically: marshal indented, write `path+".tmp"`, `os.Rename` (mirror `deliverFile`). Create parent dir `0o755`.

Never delete existing aliases. The merge is strictly additive unless `--force`.

**Verify**: `cd library/productivity/figma && go build ./cmd/figma-pp-cli` → exit 0.

### Step 4: Tests

Add to `agent_test.go` a fake server (new helper, do not modify existing ones):

```go
func newFakeFigmaProjectsServer(t *testing.T) *httptest.Server {
    mux := http.NewServeMux()
    mux.HandleFunc("/v1/teams/", func(w, r) {  // /v1/teams/<id>/projects
        w.Write([]byte(`{"name":"Design (READY)","projects":[{"id":"P1","name":"App"},{"id":"P2","name":"Marketing"}]}`))
    })
    mux.HandleFunc("/v1/projects/", func(w, r) {  // /v1/projects/<id>/files
        if strings.Contains(r.URL.Path, "P1") {
            w.Write([]byte(`{"name":"App","files":[{"key":"AAA","name":"PaiN (1.7) — V2","last_modified":"2026-06-16T10:00:00Z"},{"key":"BBB","name":"PaiN (1.7) — V2","last_modified":"2026-06-15T10:00:00Z"}]}`))
        } else {
            w.Write([]byte(`{"name":"Marketing","files":[{"key":"CCC","name":"Brand Site","last_modified":"2026-06-10T10:00:00Z"}]}`))
        }
    })
    return httptest.NewServer(mux)
}
```

Tests:
- `TestParseProjectTeamRef` — project URL → `("project", id)`; team URL → `("team", id)`; junk → error.
- `TestSlugifyAliasCollisions` — two files named `PaiN (1.7) — V2` produce `pain-1-7-v2` and `pain-1-7-v2-2` (deterministic).
- `TestIndexFilesProject` — `agent index-files --project P1 --agent --no-cache` → JSON `files` has 2 entries with `file_key` AAA/BBB and `url` containing the key; aliases lowercased.
- `TestIndexFilesTeamWalksProjects` — `agent index-files --team T1 --agent --no-cache` → aggregates files from P1 and P2; entries carry the project name.
- `TestIndexFilesMergeIntoAdditive` — pre-write a temp known-files.json with an existing `pain` entry that has a hand-written `notes`. Run with `--merge-into <tmp>`. Assert: existing `pain` preserved with its `notes` (skipped, not overwritten), new files added, summary counts correct. Run again with `--force` and assert an overwrite updates the entry but keeps `notes` when the generated entry lacks one.

Use `t.TempDir()`; no live Figma.

**Verify**: `cd library/productivity/figma && go test ./internal/cli` → exit 0.

### Step 5: Patch metadata + docs

Create `library/productivity/figma/.printing-press-patches/agent-index-files.json` (schema_version 2, id `agent-index-files`, files: `internal/cli/agent.go`, `internal/cli/agent_test.go`, `README.md`, `SKILL.md`; summary/reason explaining read-only file-index generation + additive merge; truthful `validated_outcome`).

Document in `SKILL.md` and `README.md`:
- `agent index-files --project <id>` / `--team <id>` lists files and emits known-files entries.
- `--merge-into <path>` additively updates a known-files.json (existing aliases/notes preserved unless `--force`).
- Note it is **file-level only**; screen labels stay live via `outline`/`find-node`/`shot`.
- Note the required `projects:read` token scope.

**Verify**: `python3 -m json.tool library/productivity/figma/.printing-press-patches/agent-index-files.json >/dev/null` → exit 0.

### Step 6: Final gates

```bash
cd library/productivity/figma
go test ./... && go vet ./... && go build ./cmd/figma-pp-cli
go run ./cmd/figma-pp-cli agent index-files --help
go run ./cmd/figma-pp-cli --version
git status --short
```

Expected: tests pass; vet clean; build ok; help shows the flags; version prints; only in-scope files changed.

## Test plan

New tests: `TestParseProjectTeamRef`, `TestSlugifyAliasCollisions`,
`TestIndexFilesProject`, `TestIndexFilesTeamWalksProjects`,
`TestIndexFilesMergeIntoAdditive`. Fake project/team server; no live Figma.

## Done criteria

- [ ] `agent index-files --project <id>` emits a known-files-shaped `files` map (alias → {file_key, name, url, aliases, last_modified}).
- [ ] `agent index-files --team <id>` walks projects and aggregates files, tagging each with its project; a single failing project is recorded under `errors` and does not abort the run.
- [ ] A project/team URL positional resolves to the right id/kind.
- [ ] Alias collisions are deterministic (`-2`, `-3`).
- [ ] `--merge-into <path>` is strictly additive: existing aliases and hand-written `notes` are preserved unless `--force`; the file is written atomically; absent file is created.
- [ ] Without `--merge-into`, the command only reads (prints to stdout).
- [ ] Output is file-level only; no node/screen trees are emitted.
- [ ] `go test ./...`, `go vet ./...`, `go build ./cmd/figma-pp-cli` exit 0.
- [ ] `git status --short` shows only in-scope files; patch metadata validates.

## STOP conditions

Stop and report if:
- The generated `projects`/`teams` command excerpts or `agent.go` helper signatures don't match after the drift check.
- Implementing requires changing `client.go` or the generated commands.
- The merge would need to delete or reorder existing entries (it must be additive only).
- Tests would require a live Figma token or network egress.

## Maintenance notes

- Keep this **file-level**. If someone asks to also index screens/nodes here, decline — that belongs to live `outline`/`find-node`, and a static node dump would be stale and huge.
- The merge preserves operator-authored `notes`; never clobber them. This is the field humans use to record node hints (e.g. a prototype page id) per file.
- The command is consumer-agnostic: it does not know about any specific `known-files.json` location. Operators point `--merge-into` at their own file (e.g. an OpenClaw skill's `references/known-files.json`); that wiring lives in the consumer, not this CLI.
- Reviewers: verify (1) `--merge-into` never drops existing entries, (2) the listing path stays read-only (no Figma writes), (3) a team walk tolerates a 403 on one project without failing the whole command, (4) URL escaping in the generated `url` is correct for names with spaces/emoji.
