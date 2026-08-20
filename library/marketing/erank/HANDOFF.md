# eRank CLI Handoff

## Status

The eRank CLI work has been imported into this standalone repository from the promoted Printing Press library copy:

- Source import: `/Users/smacdonald/printing-press/library/erank`
- Run artifacts import: `/Users/smacdonald/printing-press/manuscripts/erank/20260522-130049`
- Target repository: `/Users/smacdonald/homegit/erank-cli`
- Run ID: `20260522-130049`

The source copy in Printing Press was left in place intentionally. This repo is now the standalone handoff target; delete or archive the Printing Press copy only after confirming no active Printing Press workflow still references it.

## Repository Layout

- `cmd/erank-pp-cli`: CLI entrypoint
- `cmd/erank-pp-mcp`: MCP server entrypoint
- `internal/cli`: Cobra command implementations
- `internal/client`: eRank HTTP client and endpoint behavior
- `internal/config`: local config loading and auth material handling
- `internal/mcp`: MCP tool surface
- `internal/store`: workflow archive and local persistence helpers
- `docs/printing-press-run/20260522-130049`: sanitized discovery, research, and proof artifacts

The root-level compiled `erank-pp-cli` binary was not imported. Rebuild it from source.

## Build And Test

```sh
go test ./...
go build -o erank-pp-cli ./cmd/erank-pp-cli
go build -o erank-pp-mcp ./cmd/erank-pp-mcp
```

Make targets available:

```sh
make build
make test
make lint
make build-mcp
make build-all
```

## Auth

This CLI currently uses authenticated browser-session access to eRank member endpoints. Treat this as a private, session-backed integration, not a stable public API.

Local validated auth material exists on this machine at:

- `/Users/smacdonald/.config/erank-pp-cli/config.toml`
- `/Users/smacdonald/.config/erank-pp-cli/browser-session-proof.json`

Do not commit those files or derived secret values. Use `ERANK_CONFIG` when running live checks from tool sandboxes:

```sh
ERANK_CONFIG=/Users/smacdonald/.config/erank-pp-cli/config.toml ./erank-pp-cli doctor --json
```

## Verification Snapshot

Latest full Printing Press run:

- Full live dogfood: pass
- Dogfood matrix: 129 passed, 72 skipped
- Phase 5 acceptance: pass
- Shipcheck: pass, 6/6 legs
- Workflow verify: `workflow-pass`
- Command resolution: pass, 20/20 commands
- `govulncheck`: code affected by 0 vulnerabilities
- Exact-value secret scan over run/archive: 0 hits

Primary proof files:

- `docs/printing-press-run/20260522-130049/proofs/phase5-acceptance.json`
- `docs/printing-press-run/20260522-130049/proofs/2026-05-22-130049-dogfood-results.json`
- `docs/printing-press-run/20260522-130049/proofs/2026-05-22-130049-fix-erank-pp-cli-shipcheck.md`
- `docs/printing-press-run/20260522-130049/proofs/2026-05-22-130049-fix-erank-pp-cli-live-smoke.md`
- `workflow-verify-report.json`
- `dogfood-results.json`

## Implemented Scope

The imported work includes:

- Browser-session proof command
- OAuth no-argument JSON guidance
- Live dogfood invalid-sentinel rejection
- `workflow archive --json` stdout fix
- `quota list-daily` wrapper
- Seven seller-oriented workflow commands
- eRank endpoint discovery artifacts from browser traffic
- MCP entrypoint and generated tool surface
- `golang.org/x/net` upgraded to `v0.55.0`

## Current Score

Printing Press scorecard result after the local score-improvement pass:

- Grade: A
- Score: 82/100 with the browser-sniff spec path used by shipcheck
- Standalone structural score: 86/100 via `cli-printing-press scorecard --dir .`

Known score gaps:

- `insight`: 6/10
- `breadth`: 5/10
- `type_fidelity`: 3/5
- `auth_protocol`: 2/10 when scored with the browser-sniff spec
- `data_pipeline_integrity`: 7/10
- Live API verification remains unscored in the structural scorecard path

## Score Improvement Backlog

Completed in the 2026-05-22 score-improvement pass:

- Added curated MCP code-orchestration tools (`erank_search`, `erank_execute`) for seller workflows.
- Added archive freshness checks and stale-cache warnings for local insight/search commands.
- Recorded both customizations in `.printing-press-patches.json`.

Remaining backlog:

1. Stabilize auth protocol.
   Formalize the browser-session auth path, add explicit session freshness checks, document refresh steps, and make failure modes actionable. If eRank exposes a durable official auth mechanism later, move to that.

2. Split slow insight workflows.
   Break multi-endpoint insight commands into smaller commands or add per-endpoint concurrency and deadline controls. The current live sample probe loses points because broad insight commands exceed the 10 second probe timeout.

3. Improve MCP tool design.
   Replace broad endpoint-shaped tools with intent-shaped seller workflows. Keep low-level endpoint commands available in the CLI, but make MCP tools answer practical tasks like listing underperforming tags, finding keyword gaps, or checking rank movement.

4. Add deterministic fixture tests for live-shaped responses.
   Preserve live dogfood coverage, but add recorded or synthetic fixtures for timeout-prone command paths so regressions are caught without requiring live eRank availability.

5. Tighten documentation around supported and unsupported endpoint behavior.
   The browser-sniff artifacts identify endpoint surfaces, but the repo needs a maintainer-facing map of stable workflows, flaky endpoints, and auth requirements.

6. Add release packaging.
   The repo has GoReleaser config. Finish local dry-run release verification and document how to build signed or checksummed artifacts.

## Safety Notes

- Sanitized HAR and research artifacts are under `docs/printing-press-run/20260522-130049`.
- Local browser-session config and proof files are intentionally outside this repo.
- Re-run an exact-value secret scan before pushing public or shared history.
