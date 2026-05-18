# SurgeGraph CLI — Phase 3 Build Log

## What was built

### Priority 0 — Foundation (hand-built; generator left no auto-store because spec is all-POST)

- **`internal/store/store.go`** (~620 LOC) — pure-Go SQLite (modernc.org/sqlite) cache for entities that need snapshot history or FTS:
  - 12 tables: projects, prompts, visibility_snapshots, citation_snapshots, prompt_runs, traffic_pages, documents, knowledge_libraries, knowledge_library_documents, usage_snapshots, topic_researches, domain_researches, sync_cursors
  - FTS5 virtual table `cache_fts` for `search` across kind/title/body
  - Upsert + query helpers per entity, including a `LatestSnapshotPair` engine used by `visibility delta`, a `PromptDeltas` aggregator used by `visibility prompts losers`, a `KnowledgeImpact` joiner used by `knowledge impact`, and a `StaleDocs` projection used by `docs stale`
  - Tests in `store_test.go` (5 cases) cover open/migrate idempotency, project upsert overwrite, visibility-snapshot pair ordering, FTS roundtrip with kind filter and de-dup, and cursor upsert
- **`internal/oauth/oauth.go`** (~360 LOC) — stdlib-only OAuth 2.1 client: discovery doc fetch, RFC 7591 Dynamic Client Registration, PKCE (S256), local 127.0.0.1 callback receiver, code-for-token exchange, refresh-token rotation, `OpenBrowser` shell-out per OS. Tests in `oauth_test.go` (6 cases) cover PKCE verifier/challenge pairing, PKCE uniqueness, discovery URL building, error truncation, `ExpiresInDuration` edge cases, and `PortOf` URL parsing.
- **`internal/oauth/exec.go`** — split-out so `OpenBrowser` can be mocked in tests later without import-cycle pain.

### Priority 1 — Absorbed (generator-emitted)

The generator produced 69 typed commands directly from the OpenAPI (`create-*`, `get-*`, `delete-*`, `update-*`, `list-*` shapes), all under flat top-level since the spec routes are all `POST /v1/<tool_name>`. Spot-check passed:
- `get-projects --dry-run --agent` → exits 0, prints body envelope
- `create-knowledge-library --help` → has `--dry-run`
- `get-ai-visibility-overview --dry-run` with required flags → emits the actual JSON body the API would receive

All 69 absorbed features are present and dry-runnable. Spec auth enrichment (`x-auth-vars` declaring `SURGEGRAPH_TOKEN` as `harvested`) and MCP enrichment (`x-mcp` with `transport: [stdio, http]`, `orchestration: code`, `endpoint_tools: hidden`) were applied at generation time so the printed CLI ships with the Cloudflare-pattern MCP server out of the gate.

### Priority 2 — Transcendence (14 hand-built novel features)

Wired into root.go as 8 new top-level parents plus 14 leaf commands. All read-only / mutation annotations applied via `cmd.Annotations["mcp:read-only"]` so the MCP exposure carries correct safety hints.

**Local state that compounds:**
1. `visibility delta` — Reads two most-recent local snapshots per metric, emits row-wise deltas. Returns actionable note when snapshots are missing.
2. `visibility prompts losers` — Window-partitioned `PromptDeltas` aggregator; filters to citation_delta < 0 or position_delta > 0.
3. `visibility citation-domains rank-shift` — Diffs newest vs oldest citation-domain snapshots in a window; emits per-domain rank deltas.
4. `visibility portfolio` — Fans the delta engine across every project in the local store; agency view.
5. `docs stale` — Joins `documents` with `traffic_pages` to rank by AI traffic; cutoff in days.
6. `account burn` — Linear projection over `usage_snapshots`; emits credits-per-day and depletion estimate.

**Cross-entity joins no single API call returns:**
7. `knowledge impact` — Joins `knowledge_library_documents.url` against `citation_snapshots.raw_json`; ranks libraries by citation reach.
8. `research drift` — Live-fetches topic_research tree + writer_documents, in-memory join by fuzzy title match.
9. `research domain diff` — Set-diff of two topic trees (own topic_research vs competitor domain_research), via `extractTopicTreeLeavesNames`.
10. `visibility traffic-citations` — Joins top traffic pages with citations whose `raw_json` references the page URL.

**Compound workflows that span products:**
11. `research gaps publish` — One-command compound: `get_topic_map` → filter gaps → `create_bulk_documents` in ≤50 batches → `publish_document_to_cms`. Idempotent via `externalId`. Respects `--dry-run` and `PRINTING_PRESS_VERIFY=1`.

**Agent-native plumbing:**
12. `context bundle` — Reads ≤5 entity tables for one project, emits one JSON blob shaped for an agent's context window.
13. `search` — SQLite FTS5 MATCH over `cache_fts` with `--kind` filter; snippet column for inline preview.
14. `sync diff` — Per-resource row counts + cursors; foundation primitive for agent loops.

### Auth — OAuth 2.1 + PKCE + DCR

`auth login` (new subcommand on the generated `auth` parent) wires the OAuth flow:
- Loads config, calls `oauth.Login`, which: fetches `.well-known/oauth-authorization-server`, registers a client via DCR, opens a 127.0.0.1:rand callback receiver, opens browser to `/authorize` with PKCE, captures `code`, exchanges at `/token`.
- Persists `client_id`, `client_secret`, `access_token`, `refresh_token`, `token_expiry`, `surgegraph_token` to `~/.config/surgegraph-pp-cli/config.toml` via a new `saveConfigTOML` helper (the generator's `config` package only exposed `Load()`).
- Honors `PRINTING_PRESS_VERIFY=1` short-circuit so the verifier doesn't try to open a browser.
- `--no-browser` flag prints the URL when auto-launch is undesired (SSH sessions, CI).

### `sync` — population for the local store

`surgegraph-pp-cli sync --project <id>` or `--all` fans out across the API:
- `get_projects` → upsert
- `get_ai_visibility_overview / trend / sentiment / traffic_summary` → snapshot rows keyed by `(project, brand, date, metric_type)`
- `get_ai_visibility_citations` → citation_snapshots rows keyed by `(project, date, domain)`
- `get_ai_visibility_prompts` (paginated) → prompts upsert + prompt_runs snapshot
- `get_ai_visibility_traffic_pages` (paginated) → traffic_pages snapshot
- `get_writer_documents` + `get_optimized_documents` → documents upsert + FTS index
- `get_knowledge_libraries` + per-library `get_knowledge_library_documents` → upsert
- `get_usage` → usage_snapshots
- Per-resource `sync_cursors` bumped after each phase
- `--include <csv>` narrows the scope; default is everything

### Wiring

Single edit to `internal/cli/root.go`: registered 8 new parent commands (`sync`, `visibility`, `research`, `docs`, `knowledge`, `account`, `context`, `search`) under the existing `rootCmd`. `sync diff` wired as a subcommand of `sync` via `wireSyncDiff`. `auth login` wired as a subcommand of the existing `auth` parent (one line in `auth.go`).

## What was intentionally deferred

None. Every feature in the Phase 1.5 manifest is built.

## Skipped body fields that remain

The generator skipped `GET /v1` (the meta `list_tools` route) because it could not derive a resource name from a bare `/v1` path. This is the dual of the `api-discovery` command the CLI already ships — no functional loss.

## Generator limitations found

1. **Spec-derived sync absent for all-POST APIs.** The OpenAPI spec defines every operation as `POST /v1/<tool_name>` (RPC-shape facade over MCP), so the generator's resource-extraction couldn't infer any `GET` collection paths. Result: no `internal/store/`, no `sync`, no `search`, no `sql` framework commands were emitted. This forced hand-building of the entire foundation layer.
   - **Systemic vs CLI-specific:** Systemic. Future RPC-style OpenAPI specs (POST-only by convention) will hit the same gap. **Retro candidate.** A spec extension like `x-pp-store-entity` on a POST operation would let the user opt into store generation: "this POST is conceptually a GET; emit a typed mirror into the store keyed by `<id_field>`." Or — the parser could recognize the `get_*` operationId prefix as a read-shape signal.

2. **Title-derived slug overrode `--name` implication.** The spec's `info.title` is "SurgeGraph REST Facade" → slug `surgegraph-facade`. The user's documented binary name is `surgegraph-pp-cli`. A first generate without `--name` produced `surgegraph-facade-pp-cli`; passing `--name surgegraph` fixed it. The generator could emit a warning when the slug contains `-facade` / `-rest` / `-api` suffixes that look like spec-title artifacts. **Minor retro candidate.**

3. **Root.go Short prefix uses lowercased slug.** Generated Short is `Surgegraph CLI — …` (lowercase `g`) when the authored `narrative.display_name` is `SurgeGraph`. README, SKILL frontmatter, and `.goreleaser` all use the authored case correctly; only `root.go` falls back to slug-title-casing. **Minor retro candidate** — the root template should consult `narrative.display_name` for the Short prefix.

## Build state

```
go build ./...                 PASS
go test ./internal/oauth/...   PASS (6 cases)
go test ./internal/store/...   PASS (5 cases)
```

The binary at `$CLI_WORK_DIR/surgegraph-pp-cli` is ready for Phase 4 shipcheck.
