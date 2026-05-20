# Novel Features Brainstorm — figma

## Customer model

**Persona A — Maya, AI codegen engineer using Claude Code on a Pro-tier Figma design-system file.**
- Today: pastes Figma URL into Cursor; agent calls `get_figma_data` (GLips MCP) but gets a 4MB JSON blob that blows the context window — or a render PNG with no semantics. Re-prompts manually.
- Weekly ritual: picks 5-15 frames per week from a shared file, extracts each into prompt-ready context, iterates on generated code.
- Frustration: no tool fuses node tree + variables in scope + dev-resource links + Code Connect mappings into one compact payload. Wants `--ids` to take URL fragments and instance-override chains like `I5666:180910;1:10515`.

**Persona B — Priya, design-system maintainer at an Enterprise-tier shop with 200+ files and 1200 components.**
- Today: opens library analytics dashboard in browser per file, copies usage numbers into a spreadsheet, manually flags components published but never instanced. Same drill for styles and variables.
- Weekly ritual: reviews unresolved comments across the design-system file plus 5 consumer files; bumps owners on stale threads.
- Frustration: library analytics is per-file in the UI; no cross-team `published-but-zero-usages` finder. Comments audit requires opening every file.

**Persona C — Diego, frontend engineer integrating design tokens into a CI pipeline at a mid-market company on Pro tier.**
- Today: runs Figmagic locally, commits `tokens.css`. PR review can't tell what changed in the token set when the Figma file moves.
- Weekly ritual: pulls latest tokens before a release, diffs against trunk, files cleanup tickets.
- Frustration: no way to fingerprint a file deterministically for "did these tokens change?" without running the full export. No way to diff variable values across two `file_versions`.

**Persona D — Sam, plugin/integration developer building a webhook-driven Figma plugin.**
- Today: registers a webhook, creates a test file event, waits, hopes. When delivery fails, looks at server logs but has no way to replay without re-triggering the upstream Figma event.
- Weekly ritual: iterates on payload parsing, verifies HMAC, debugs delivery failures.
- Frustration: webhook request log endpoint exists but no tool ships a one-command "fetch failed deliveries and replay them against my new endpoint."

## Candidates (pre-cut)

(see subagent transcript above for the full 16-row table; the cut leaves 8 survivors)

## Survivors and kills

### Survivors

| # | Feature | Command | Why Only We Can Do This | Persona Served | Score | Buildability Proof |
|---|---|---|---|---|---|---|
| 1 | Compaction-aware frame extract for codegen prompts | `figma frame extract <key> --ids=1234-5678 --depth=4 --include=variables,dev-resources,code-connect` | GLips ships compaction in a Node MCP only; no other tool fuses node tree + in-scope variables + dev resources + Code Connect into one compact payload, accepts both `1234:5678` and `1234-5678`, and resolves instance-override chains like `I5666:180910;1:10515`. | Maya | 10/10 | Uses `GET /v1/files/{key}/nodes` + `GET /v1/files/{key}/dev_resources` + locally-synced `variables_local` joined by node id, runs ported `simplifyRawFigmaObject` compaction in Go, emits compact JSON with `simplifiedNodeCount`. |
| 2 | Dev-mode resource bundle for a single node | `figma dev-mode dump <key> --node=<id> --format=md` | Official Dev Mode MCP is closed-source and Desktop-bound; no surveyed tool emits a portable Markdown bundle that fuses dev-resource links, variables in scope, render permalink, and Code Connect mapping for one node. | Maya | 9/10 | Uses `GET /v1/files/{key}/nodes?ids=<id>` + `GET /v1/files/{key}/dev_resources` filtered by node + locally-synced `variables_published` joined by `variableId`, emits Markdown bundle. |
| 3 | Cross-file unresolved comments audit | `figma comments audit --older-than=14d --group-by=file,author` | `figma comments list` ships per-file; no surveyed tool aggregates comments across every synced team file with age + group-by; brief Top Workflow #3. | Priya / design-ops | 10/10 | Uses locally-synced `comments` table, runs SQL aggregation `WHERE resolved_at IS NULL AND created_at < now()-INTERVAL`, writes to stdout. |
| 4 | Stale-component / style / variable orphans finder | `figma orphans <team_id> --kind=component,style,variable --window=30d` | Library analytics is per-entity-per-file in the UI; no surveyed tool joins `components` ⨝ `analytics.usages` to surface published-but-zero-usage entities across an entire team library. Brief Top Workflow #4. | Priya | 10/10 | Uses locally-synced `components`/`styles`/`variables_published` and `analytics_*_usages`, joins on `key`, aggregates `SUM(total) = 0` over window. Enterprise-gated via doctor. |
| 5 | Semantic tokens diff between file versions | `figma tokens diff <key> --from=<v1> --to=<v2> --format=md` | No surveyed tool diffs Figma variables across two `file_versions` with mode-awareness; figmagic/figmage emit current state only, Tokens Studio diffs Git not Figma. Brief P2 list. | Diego | 9/10 | Uses `GET /v1/files/{key}/versions` to resolve ids, then `GET /v1/files/{key}/variables/local` snapshotted by `file_version`, computes set-diff + value-compare in Go. |
| 6 | Deterministic file fingerprint for CI contract | `figma fingerprint <key> --expect=<hash>` | No surveyed tool offers a stable hash of a Figma file's token + component + style surface; figmagic emits artifacts but not a contract-check exit code. Brief P2 list. | Diego | 7/10 | Uses locally-synced `variables_local` + `components` + `styles`, sorts by `key`, content-hashes canonical JSON via `crypto/sha256`, exits 0/2 on match/mismatch. |
| 7 | Webhook delivery replay against new target | `figma webhooks test <id> --replay-failed --target-url=https://localhost:3000/figma` | No surveyed tool pulls Figma's webhook request log and replays stored payloads (with original headers + HMAC) against an arbitrary target. Brief P2 list. | Sam | 9/10 | Uses `GET /v2/webhooks/{id}/requests` to fetch deliveries, filters `status >= 400`, replays each via stdlib `net/http` with captured payload + original headers + HMAC re-signed under the webhook's `passcode`. |
| 8 | Variable usage tracer | `figma variables explain <key> --variable=<name>` | Figma's UI shows variable references modally per-node; no surveyed tool emits a flat list of every node + component that references a given variable across the file. Brief Top Workflow #2 + #4. | Priya | 8/10 | Uses locally-synced `variables_local` + a `node_variable_refs` join table populated during `figma sync files` (extracted from each node's `boundVariables`), runs SQL JOIN on `variable_id`. |

### Killed candidates

| Candidate | Kill reason | Closest survivor |
|---|---|---|
| `tokens export` (C5) | Already in absorb #38 — not novel. | (#5) `tokens diff` extends it. |
| `export-batch` (C9) | Already in absorb #39 — not novel. | — |
| `library inventory` (C10) | Already in absorb #40 — not novel. | (#4) `orphans` is the actionable cut. |
| `frame screenshot --diff-base` (C11, pixel diff) | Pixel-diff output requires manual inspection; Persona C is better served by semantic `tokens diff`. | (#5) `tokens diff`. |
| `node inspect` (C12) | Strict subset of `frame extract` / `dev-mode dump`; collapsing removes redundant surface. | (#1), (#2). |
| `instance-overrides resolve` (C13) | Reframe: belongs as `--ids` parsing inside extract/dump, not a top-level command. | (#1) `frame extract`. |
| `commands grep --has-comment-mentioning` (C15) | Covered by absorb #44 (FTS5 `figma search`) + (#3) `comments audit`. | absorb #44, (#3). |
| `me whoami-pretty` (C16) | Redundant with absorb #28 `figma me` and #34 `figma doctor`; no novel work. | absorb #28, #34. |
