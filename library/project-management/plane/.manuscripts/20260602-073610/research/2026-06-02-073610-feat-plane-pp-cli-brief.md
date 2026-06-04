# Plane CLI Brief (degraded reprint, run 20260602-073610)

> Provenance note: this CLI was hand-built and dogfooded against a self-hosted
> Plane workspace before `research.json` provenance was captured as files. This
> brief reconstructs the real design basis at reprint time — vendored official
> spec plus two dogfooded novel features — rather than a fresh web-research pass.

## API Identity

Plane — open-source project management (issues, cycles, modules, sub-issues,
workspaces); a self-hostable Jira / Linear / ClickUp alternative. REST API is
workspace-scoped (`/api/v1/workspaces/{slug}/...`), API-key auth via the
`X-API-Key` header. Spec is vendored from Plane v1.3.0 CE (drf-spectacular at
`/api/schema/`), re-expressed so the workspace scope is a `servers:` `{slug}`
variable and workspace paths are relative — the form the profiler understands.

## Users

- **Engineering lead / PM** running a self-hosted Plane: wants offline,
  scriptable access to issues/cycles/modules without clicking through the web UI.
- **Agent operator**: drives Plane through the `plane-pp-mcp` MCP server (125
  tools), needing the typed surface plus the workflow gaps closed.
- **Power user / ops**: bulk reads, full-text search and analytics over a local
  SQLite mirror, and the relation/module operations the REST surface omits.

## Top Workflows

1. **Sync → search/analyze**: `sync` fans out across projects to populate a typed
   local store (projects → issues, cycles, modules, labels, states, …), then
   `search`, `analytics`, `stale`, `orphans`, `load` run offline.
2. **Relations**: `relations list/set/unset` to read and manage issue
   blocking/blocked_by/duplicate/relates_to and temporal links.
3. **Modules**: `module sync` backfills module membership the issue serializer
   omits; `module of <issue>` reads it; `module create-issue` creates a work item
   and adds it to a module in one step.

## Table Stakes

Typed list/retrieve/create/update for projects, issues/work-items, cycles,
modules, labels, states, members, intake; `--json`/`--agent` output; offline
store with incremental (`updated_at__gt`) sync; MCP server mirroring the tree.

## Data Layer

SQLite local store with typed tables and a generic mirror; project-scoped
dependents synced via parent-ID fan-out. Issues land in `projects_issues`
(FK `projects_id`); a runtime `module_issues` junction table backs the module
enrichment without a generated-schema patch.

## Spec Strategy

Vendored official spec, restructured (workspace scope → `servers:{slug}`,
relative workspace paths, `updated_at__gt` since-param declared). In this form a
stock `generate` auto-detects the `workspace → projects → children` sync model.
The only post-generation work is pinning the canonical issues list (around a
profiler list-endpoint non-determinism) and porting the two novel commands.

## Novel Features

- **Issue Relations (`relations`)** — Plane CE has no DELETE on
  `/work-items/{id}/relations/`; `unset` shells into a user-configured Plane API
  container Django shell (self-hosted only), while `list`/`set` wrap the existing
  endpoints with flat ergonomics.
- **Module Membership & Sync Enrichment (`module`)** — the issue API never
  returns `module_ids`; the enrichment walks `/modules/{id}/module-issues/` into
  a junction table and patches each issue's cached `module_ids`, running
  automatically at the tail of `sync`.

## Build Priorities

Working `sync` parity with the dogfooded baseline (≈529 records across 11
resources) and the two novel commands, validated by a live dogfood matrix and an
isolated sync against a real workspace.
