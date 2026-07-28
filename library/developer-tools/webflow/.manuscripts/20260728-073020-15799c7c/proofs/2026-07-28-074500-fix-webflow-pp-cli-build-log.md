Manifest transcendence rows: 7 planned, 7 built. Phase 3 will not pass until all 7 ship.

# Webflow CLI — Build Log

Run: 20260728-073020-15799c7c · Printing Press v4.29.0 · Go 1.26.5

## Spec normalization (before generation)

The official Webflow OpenAPI is 3.1.0 and uses JSON Schema constructs that
`kin-openapi` (the generator's parser) cannot unmarshal. Two passes were needed
against a derived overlay at `research/webflow-openapi-v2-enriched.yml`; the
pristine vendor file is preserved beside it as `webflow-openapi-v2.yml`.

1. **Tuple-form `items` (654 sites).** Draft-04-style `items: [schemaA, schemaB]`
   fails with `cannot unmarshal array into field Schema.items`. Converted to
   `items: {oneOf: [...]}`. The transform skips any subtree under `example`,
   `examples`, `default`, or `enum`, so the 23 occurrences of a *data* key named
   `items` inside response examples were left untouched.
2. **Boolean JSON Schemas (3 sites).** `properties: {additionalProperties: true}`
   fails with `cannot unmarshal bool into field Schema.properties`. Converted to
   an empty permissive schema `{}`.

Both are generator-shaped findings, not Webflow bugs — the spec is valid
OpenAPI 3.1. Filed as retro candidates.

## Spec enrichment

- **Auth:** global `security` reordered so the `ApiKey` scheme (`type: http`,
  `scheme: bearer`) is primary rather than `OAuth2`. Added
  `x-auth-env-vars: [WEBFLOW_API_TOKEN]` on that scheme, matching the official
  JS SDK and CLI convention.
- **Category:** `developer-tools`, passed on the generate invocation. Matches
  the closest public-library peer (`framer`).
- **Cache:** deliberately NOT enabled. Webflow allows 60 req/min on Starter and
  Basic plans, so a pre-read upstream refresh is a real cost. The CLI ships
  manual `sync` plus the generated `doctor` freshness report instead.
- **Learn:** no-entities escape, recorded at the spec root. Webflow's aliasable
  vocabulary (site names, collection slugs, page slugs, field names) is
  per-account and discovered at sync time; seeding canonical/alias pairs at
  generate time would bake one user's workspace into every install.
- **MCP:** not hand-set. 117 endpoints is past the 50 threshold, so the
  generator applied the Cloudflare pattern automatically
  (`orchestration: code`, `endpoint_tools: hidden`, `transport: [stdio,http]`).

## What was generated

164 leaf commands covering all 117 operations across all 16 tags. Every gate
passed on the generate run: `go mod tidy`, `govulncheck`, `go vet`, `go build`,
runnable binary, `--help`, `version`, `doctor`.

Four request bodies contain `oneOf`/`anyOf` and fell back to `--body-json`:
`POST /collections/{id}/fields`, `POST /collections/{id}/items`,
`POST /collections/{id}/items/live`, `POST /collections/{id}/items/publish`.

## What was hand-built (Priority 2)

All seven transcendence rows are hand-code; none came free from the spec.
Shared plumbing lives in `internal/cli/webflow_localq.go` (store open,
missing-mirror guard, drain-first row scanning, Webflow record shapes,
timestamp and path normalization).

| Command | File | Data source | What it does |
|---|---|---|---|
| `seo audit [site-id]` | `seo_audit.go` | local | Missing/over-length/thin SEO title and description, missing Open Graph, plus self-joined duplicate title, description, and slug detection |
| `drift [collection-id]` | `drift.go` | local | Items never published or edited since publish, with drafts and archived counted separately |
| `publish preview [site-id]` | `publish_preview.go` | local | Pages changed since `lastPublished`, draft pages, per-collection unpublished item counts, redirect count |
| `items bulk-set [collection-id]` | `items_bulk_set.go` | local | Local selection by repeatable `--match`, preview by default, `--apply` writes through the generated client's adaptive limiter |
| `collections completeness [collection-id]` | `collections_completeness.go` | local | Per-field fill rate, required-but-empty gaps, dead schema fields |
| `redirects audit [site-id]` | `redirects_audit.go` | local | Shadowed live pages, unknown targets, self-redirects, loops, duplicate sources |
| `overview` | `overview.go` | local | One row per synced site: pages, collections, unpublished items, SEO findings, days since publish |

Design decisions worth carrying forward:

- **`drift` computes from staged items alone.** The subagent's plan was a local
  join of a staged-items table against a live-items table, but `sync` stores
  only the staged `/items` endpoint. Webflow puts `lastPublished` and
  `lastUpdated` on every item record, so drift is computed from those instead:
  never-published, or updated after publish. Same answer, no second sync
  resource, no extra API cost.
- **`items bulk-set` is `pp:data-source local`, not `computed`.** The subagent
  proposed `computed`; that value is reserved for pure policy/math commands.
  The directive describes where the command's *data* comes from, and the
  selection comes from the mirror. The upstream PATCH is the action.
- **Audits degrade honestly rather than lying.** `redirects audit` sets
  `targetsChecked: false` and explains itself when pages were not synced,
  instead of reporting every target as unknown. `collections completeness`
  infers fields from item data when the collection schema is absent and says so
  in `note`. Both were written this way because the alternative is a
  confident-looking false positive.

## Verification

- Per-row Cobra resolution: all 7 approved command paths resolve with the
  correct `Usage:` spec line.
- Dry-run probes: all 7 exit 0 under `--dry-run`.
- Deterministic backstop: `dogfood --json` reports `novel_features_check`
  planned 7, found 7, missing none, not skipped.
- Tests: 1022 pass across 17 packages, including 54 behavioral assertions in
  `webflow_novel_behavior_test.go` covering each command's happy path, its
  absence-of-correctness case (empty input must produce empty output, never
  fabricated findings), and negative selection checks.

## Intentionally deferred

- **Ecommerce commands are generated but not behaviorally verified.** Products,
  orders, SKUs, and inventory ship from the spec. No ecommerce site was
  available to exercise them.
- **Page and component DOM commands ship as generated endpoint commands.** The
  `pages dom get-static-content` / `update-static-content` pair is the absorbed
  surface; no hand-authored roundtrip wrapper was built (candidate A10 was cut
  in Pass 3 as file plumbing rather than a command).

## Generator limitations found

1. `kin-openapi` cannot parse OpenAPI 3.1 tuple-form `items` or boolean JSON
   Schemas. Both are valid 3.1. The generator's `--lenient` cleanup pass does
   not handle either. Retro candidate.
2. The novel-feature scaffolder skipped `collections completeness` with
   `novel feature command "collections" maps to generated command path;
   skipping novel stub`, but then emitted the scaffold anyway. The warning is
   misleading; the command was present and only needed a body.
