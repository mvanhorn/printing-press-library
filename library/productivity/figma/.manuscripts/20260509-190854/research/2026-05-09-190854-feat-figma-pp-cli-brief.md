# Figma CLI Brief

## API Identity
- **Domain**: Figma — collaborative design tool. REST API exposes files, comments, teams/projects, components/styles/variables, webhooks v2, library analytics, dev resources (Dev Mode), activity logs (Enterprise), and oEmbed.
- **Users**: design-system maintainers, frontend engineers integrating tokens/icons into codebases, AI codegen agents extracting frame context for prompts, design ops engineers auditing comments/components, plugin developers debugging webhooks.
- **Data profile**: 41 endpoints across 12 resource families. Spec marks itself "beta given large surface area". File responses are routinely **megabytes to tens of megabytes** of nested JSON.
- **Spec**: `https://raw.githubusercontent.com/figma/rest-api-spec/refs/heads/main/openapi/openapi.yaml` — OpenAPI 3.1.0 YAML, ~10,400 lines, 372 KB.
- **Base URL**: `https://api.figma.com/v1` (and `/v2` for webhooks).

## Reachability Risk
**None / Low.** Public REST API; CloudFront-fronted; no JS interstitial. `GET /v1/files/abc123` returns 403 without auth — clean failure. Spec is canonical and machine-fetchable. The only real operational caveat: image URLs expire (14 days for image-fills, 30 days for renders) and the `/v1/images/{key}` rendering endpoint is async-shaped with concrete rate-limit lockouts on Tier-1 endpoints.

## Top Workflows
1. **Frame → codegen prompt**: extract a single frame's compact representation (variables in scope + node tree + dev resources) for an LLM to turn into React/SwiftUI/Compose. Killer use case for AI agents — the entire reason GLips/Figma-Context-MCP has 14.7k stars.
2. **Design-token sync**: pull variables out of a Figma file, emit `tokens.json` (W3C DTCG) or `tokens.css`, commit to a design-tokens repo.
3. **Comment audit**: find unresolved comments older than N days across every team file. Today this requires opening each file individually.
4. **Stale-component cleanup**: cross-reference team-library publish list with library-analytics usage data → flag components published but never used. Enterprise-only.
5. **Bulk icon export**: walk team → projects → files → components matching `icon/*`, render as SVG into a target directory, with rate-limit-aware batching.

## Table Stakes
- All 41 endpoints CRUD-mirrored: files, file-nodes, render-images, image-fills, file-versions, file-meta; comments + reactions; teams/projects; components + component-sets + styles (per-team and per-file); variables (local/published/write); dev resources; webhooks v2 + request log; activity logs; library analytics (3 entity types × 2 metrics); developer logs; payments; oEmbed; `/me`.
- Three auth modes (`X-Figma-Token` for PAT, `X-Figma-Token` for Plan Access Token, `Authorization: Bearer` for OAuth). Doctor probe-matrix that surfaces `X-Figma-Plan-Tier` + `X-Figma-Rate-Limit-Type` headers.
- `--json` / `--yaml` output, `--dry-run`, `--confirm` for destructive ops (delete comment, delete dev resource, delete webhook).
- Rate-limit-aware client that reads `X-Figma-Rate-Limit-Type` (`low`/`high`), `Retry-After`, and `X-Figma-Plan-Tier`; backs off per Figma's 3-tier endpoint × 5-tier plan × 2-tier seat matrix.
- Cursor pagination on team-scoped listings (`?after=<id>`); honor `branch_data=true` on file fetches.

## Data Layer
- **Cache**: `teams` (manual seed), `projects`, `files`, `file_versions`, `components`, `component_sets`, `styles`, `variables_local`, `variables_published`, `comments`, `comment_reactions`, `dev_resources`, `webhooks`, `webhook_requests`, `analytics_component_actions/usages` (× styles, variables), `activity_logs`. Plus `image_url_cache` with mandatory `fetched_at` so we can invalidate at 14d/30d.
- **Sync cursor**: `last_modified` on file metadata is the cheap driver — `GET /v1/files/{key}/meta` is Tier-1-light; refetch full body only when changed. Comments append-update with `created_at`; activity logs use `start_time`/`end_time` window.
- **FTS5 candidates**: `comments.message + author`, `components.name + description`, `styles.name + description`, `dev_resources.name + url`, `variables_local.name`, `files.name`, `activity_logs.actor + action`. Killer cross-resource search: "find every component named 'icon/check' across all team libraries."

## Codebase Intelligence
- Source: direct GitHub inspection of `GLips/Figma-Context-MCP` (`src/services/figma.ts`, `src/extractors/`, `src/transformers/`, `src/services/get-figma-data.ts`).
- **Compaction pipeline** is the single most valuable thing to port: `simplifyRawFigmaObject` does pre-order tree walk via `node-walker`, runs 5 transformers (component, effects, layout, style, text), collapses single-leaf SVG containers via `collapseSvgContainers`, and deduplicates style records into a global registry referenced by short id. Reports `simplifiedNodeCount` so the agent sees compression ratio. **This is the only credible answer to "Figma file responses are 10MB."**
- **Node-id input** must accept BOTH `1234:5678` and `1234-5678` formats AND deeply-nested instance-override chains like `I5666:180910;1:10515`. Regex documented in source. Replace `-` with `:` server-side before calling the API.
- **`imageRef` vs `gifRef`** — Figma's image-fills response distinguishes static vs animated; the wrong choice silently returns a snapshot of a GIF. Mirror this in `figma images download`.
- **Auth pattern**: GLips uses `FIGMA_API_KEY` (PAT) → `X-Figma-Token` header, with `FIGMA_OAUTH_TOKEN` falling through to `Authorization: Bearer`. Per-request `X-Figma-Token` accepted in HTTP transport.

## Auth
- **Token formats**: PAT prefix `figd_` (opaque base64, per-user); Plan Access Token (Org/Enterprise beta, same header); OAuth Bearer (standard OAuth 2.0, scope-controlled).
- **Headers**: `X-Figma-Token: <token>` for PAT and Plan tokens; `Authorization: Bearer <token>` for OAuth.
- **Env-var convention**: Figma's official Code Connect CLI uses `FIGMA_ACCESS_TOKEN`. GLips MCP uses `FIGMA_API_KEY` + `FIGMA_OAUTH_TOKEN`. Recommend our CLI accept all three in priority order: explicit flag → `FIGMA_ACCESS_TOKEN` (canonical) → `FIGMA_API_TOKEN` → `FIGMA_API_KEY` → `FIGMA_OAUTH_TOKEN`.
- **Endpoint differences**: PAT works for files/comments/components/styles/variables/webhooks/dev-resources. **OAuth-required**: `/v1/activity_logs`, `/v1/developer_logs`, the Discovery API, the Embed API. `/v1/me` is OAuth-preferred (PAT may 403 depending on scope).
- **Rate-limit observed**: response headers `X-Figma-Rate-Limit-Type` (`low`/`high`), `X-Figma-Plan-Tier`, `Retry-After` (seconds), `X-Figma-Upgrade-Link`. Figma rate-limits are 3-axis: endpoint-tier × plan-tier × seat-class. Tier-1 (files/images/nodes) is strict — single-digit RPS observed in forums for full-seat Pro/Org/Enterprise users; multi-day `Retry-After` lockouts reported.

## Spec Quirks
- **OpenAPI 3.1.0 YAML** — generator must handle 3.1 (3.0/3.1 differences mostly affect schemas).
- **`v2` namespace** for webhooks (`/v2/webhooks`, `/v2/teams/{id}/webhooks`, `/v2/webhooks/{id}/requests`). The generator must NOT normalize this to `v1`.
- **No top-level "list teams" endpoint** — teams enter the local store manually via `figma teams add <id> <name>`.
- **Three security schemes** with two header shapes (`X-Figma-Token` and `Authorization: Bearer`) — clean `securityRequirements` per operation gates Enterprise-only endpoints.
- **Pagination is per-endpoint**: cursor-style on team-scoped listings (`?after=<id>`); no uniform `next_cursor`/`Link` header.

## Tools Landscape
| Tool | Role | Stars |
|---|---|---|
| **Figma official Dev Mode MCP** | 16 tools, Desktop-aware, closed source, scriptable only via MCP | n/a |
| **GLips/Figma-Context-MCP** (TS) | 2 tools (`get_figma_data`, `download_figma_images`) — compaction pipeline is the killer feature | **14,689** |
| **figma/rest-api-spec** | Canonical spec + types | 209 |
| **vkhanhqui/figma-mcp-go** (Go) | "Free-tier" MCP claiming read+write — likely uses plugin runtime, not REST | 816 |
| **mikaelvesavuori/figmagic** | Tokens + graphics export + React scaffolding | 858 |
| **RedMadRobot/figma-export** (Swift) | iOS/Android-targeted bulk export, codegen | 811 |
| **marcomontalbano/figma-export** | Components → SVG/PNG/PDF/JPG | 341 |
| **Tokens Studio (Git Sync)** | De-facto tokens-as-code (in-Figma plugin) | very large |
| **didoo/figma-api** | TS REST client aligned with `@figma/rest-api-spec` | medium |

Multiple tools cover slices (tokens, icons, file extraction, MCP serving) — **none ship the union**. No surveyed tool covers comment audit, stale-component finder, fingerprint, tokens-diff-by-version, or webhook delivery replay.

## Product Thesis
- **Name**: `figma-pp-cli` (binary), CLI alias `figma`.
- **Why it should exist**: GLips/Figma-Context-MCP proved the appetite (14.7k★) but only as a Node MCP — no portable Go binary, no offline cache, no audit/diff/orphans. The official Dev Mode MCP is closed-source and Desktop-bound. Existing token tools (figmagic, figmage, figtree, Tokens Studio) each pick a different format. We absorb every endpoint AND ship the compaction-aware `frame extract` / `dev-mode dump` agents actually need, plus FTS5 search across comments/components/styles, plus the audit/diff/orphans commands no other tool ships.

## Build Priorities
1. **P0 foundation**: data layer for all primary entities. Cursor walker. Image-URL cache with `fetched_at` invalidation. Rate-limit-aware client (reads `X-Figma-Rate-Limit-Type`, `Retry-After`, `X-Figma-Plan-Tier`; token-bucket per endpoint tier). FTS5 across `comments.message`, `components.name+description`, `styles.name+description`, `dev_resources.name+url`, `variables_local.name`, `files.name`.
2. **P1 absorb**: every of the 41 endpoints — full file/comment/component/style/variable/dev-resource/webhook coverage, plus library analytics, activity logs, payments, oEmbed. Bulk image download with imageRef/gifRef distinction. Workspace seed (`teams add`).
3. **P2 transcend**: `frame extract` (port GLips compaction), `dev-mode dump` (fuse frame+variables+dev-resources+code-connect), `comments audit`, `orphans`, `tokens diff`, `webhooks test` (replay), `fingerprint`, `export-batch`. Doctor probe matrix that distinguishes PAT vs OAuth and warns on Enterprise-only endpoints.
4. **P3 polish**: SKILL.md with the 5 power workflows; flag enrichment (especially `--ids` semantics, `--depth`, `--branch_data`, `--scale` for renders); README cookbook for codegen, tokens, comment audit, orphans.
