# Changelog

All notable changes to this project are documented here. Format loosely
follows [Keep a Changelog](https://keepachangelog.com/) with one section
per released tag. Each entry summarizes what a user would notice on
upgrade.

## 2026.6.2 - 2026-06-21

- fix(catalog): require Go 1.26.4 across published modules (#1308).

## 2026.6.1 - 2026-06-08

- Baseline release metadata added for this published CLI.

## v0.1.7 — 2026-06-02

### Fixed

- `sync --resources tickets --since ...` now keeps using documented
  `order_by=updated_datetime:desc` plus a local cutoff for tickets,
  and detects out-of-order API pages before trusting an early cutoff.
  The docs now explicitly warn agents not to invent unsupported filters
  such as `updated_datetime__gte`, which the live API rejects.
- Local FTS search now separates global and resource-scoped SQL paths,
  restoring expected results for `search <query> --data-source local`
  without requiring `--type`.
- `analytics --type tickets --group-by status` now aggregates over the
  generic `resources` table instead of assuming a typed `tickets` table
  or silently sampling only the first 200 local rows.
- Public release docs no longer reference a company-specific security
  contact.
- Public release hygiene now omits the live-credential acceptance proof
  artifact and uses secret-manager-neutral wrapper guidance.

### Added

- Restored an MCP package `manifest.json` for public-library/MCPB-style
  installs, including the required `GORGIAS_BASE_URL` tenant setting.

## v0.1.6 — 2026-05-15

### Fixed

- `gorgias-pp-cli api <iface> --json` for promoted single-endpoint
  resources (`messages`, `pickups`, `reporting`, `ticket-search`) was
  returning `methods: []`. Programmatic summation across all interfaces
  totaled 104 instead of the documented 108. Each promoted leaf now
  synthesizes a single self-method derived from its `pp:endpoint`
  annotation (e.g. `messages` → `list`), so the JSON count matches the
  headline claim.

## v0.1.5 — 2026-05-15

### Fixed

- MCP `context` tool reported `archetype: "project-management"` — a
  leftover from the generator template. Set to `"customer-support"`
  to match every other doc surface.
- README troubleshooting paragraph said the MCP server "only shows
  `gorgias_search`/`gorgias_execute`/`context`/`sql`/`search`" — five
  tools. Live `tools/list` returns fifteen. Rewrote to enumerate all
  fifteen and explain the gateway-vs-typed split.
- `gorgias-pp-cli api` browser listed 104 endpoints across 16 resource
  groups because the four promoted single-endpoint commands
  (`messages`, `pickups`, `reporting`, `ticket-search`) were excluded
  from the hidden-only filter. Added a `promotedLeafResources`
  allowlist so the surface count matches the documented 108.
- "~1K context tokens vs ~25K for one-tool-per-endpoint" claim was
  true for description text alone but ignored JSON schemas. Reworded
  every site (README:10, README:94, CURSOR.md:88, SKILL.md:28,
  MCP.md:27, .printing-press.json:43) with measured numbers (~1K
  descriptions + ~7K schemas = ~9K total; ~5× that for
  one-tool-per-endpoint).

## v0.1.4 — 2026-05-15

### Fixed

- `spec.yaml` at the repo root was byte-identical to
  `spec-sources/gorgias-crowd.yaml` (149KB of duplicate provenance).
  Removed; the three flag descriptions that named it now point at the
  canonical `spec-sources/` path.
- Scrubbed the 19 "Operations on `<resource>`" boilerplate descriptions
  from `SKILL.md` (resource group headers) and
  `spec-sources/gorgias-crowd.yaml` by mirroring the curated parent-
  command Shorts already in `internal/cli/*.go`.
- Refreshed `.printing-press.json`'s `spec_checksum` to track the
  spec edit.
- Clarified the relationship between `internal/client/client.go`'s
  `clientVersion` / `SetVersion` and `cli.Version()` /
  `resolveVersion()` so the User-Agent and the CLI's reported version
  always come from the same resolution chain.

### Added

- `CONTRIBUTING.md`, `SECURITY.md`, and `.github/ISSUE_TEMPLATE/`
  (bug_report.md + feature_request.md). Standard polish to match the
  Linear and allrecipes PP CLI gallery entries.

## v0.1.3 — 2026-05-15

### Fixed

- README claimed `gorgias-pp-mcp` shipped as a Claude Desktop MCPB
  bundle; no `.mcpb` files were attached to any release. Dropped the
  claim and routed Claude Desktop users to a documented manual config
  in [MCP.md](./MCP.md).
- "Also indexed in the Printing Press library" link was a 404 — the
  library PR hasn't been filed. Removed the broken reference.
- `tool_count` was inconsistent across docs: README implied 11–12,
  `.printing-press.json` said `mcp_tool_count: 108` (actually the
  endpoint count), and the live MCP server reports `15`. Reconciled
  every doc to reference the live count from the `context` tool, and
  renamed the `.printing-press.json` field to `mcp_endpoint_count` so
  the 108 number doesn't pretend to be tools.
- A tenant-specific view id was hardcoded in the README, profile.go, tickets_list.go,
  and client_test.go as an example. Scrubbed to `<view-id>` placeholders
  (or `123456789` in test fixtures) so the docs don't read like they
  expose tenant-specific state.
- `mcp-descriptions.json` carried a tenant-specific numeric id as a documentation
  example. Scrubbed to the standard `123456789` placeholder.

### Added

- Release notes are now auto-generated from conventional-commit
  subjects (feat / fix / docs) via goreleaser's git-changelog group.
  Empty release bodies are gone.
- Repo metadata: GitHub topics (`gorgias`, `customer-support`, `mcp`,
  `agent-tools`, `cli`, `printing-press`, `golang`), homepage set to
  the Gorgias product page, wiki disabled.
- This `CHANGELOG.md`.

### Removed

- `manifest.json` + `scripts/sync-manifest-version.sh`. The manifest
  was an MCPB bundle descriptor; with the MCPB build dropped it served
  no purpose, and the sync hook was rewriting it on every release.

## v0.1.2 — 2026-05-15

### Fixed

- `brew install chrisyoungcooks/tap/gorgias-pp-cli` only linked
  `gorgias-pp-cli` into `/opt/homebrew/bin` and left `gorgias-pp-mcp`
  stranded in the Caskroom. The cask now declares both binaries.
- The brew-installed binaries exited 137 (SIGKILL) with no error on
  first run because macOS Gatekeeper quarantined them. The cask now
  runs `xattr -dr com.apple.quarantine` on both binaries in a
  `postflight` block.

## v0.1.1 — 2026-05-15

### Fixed

- `go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/gorgias/cmd/...@v0.1.0`
  reported version `0.0.0-dev` because the source-built path didn't see
  goreleaser's ldflag. Version resolution now falls back to
  `runtime/debug.ReadBuildInfo()` and reports the module version
  recorded by `go install ...@vX.Y.Z`. Goreleaser-built release
  binaries are unaffected.

## v0.1.0 — 2026-05-15

### Added

- Initial release. Token-efficient Go CLI + sibling MCP server for the
  Gorgias REST API. 108 endpoints reachable from one binary or a
  code-orchestration MCP gateway. Local SQLite mirror with FTS5 search,
  `sql` escape hatch, `stale`/`orphans`/`load` queue analytics, `doctor`
  health check, single-emission JSON error envelopes, XDG-compliant
  config/state/data paths. Built across seven adversarial-review
  iterations on the patterns top-10% Printing Press CLIs share.
