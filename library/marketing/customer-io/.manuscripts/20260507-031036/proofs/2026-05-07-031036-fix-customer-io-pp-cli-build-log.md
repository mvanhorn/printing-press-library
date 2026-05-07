# Phase 3 build log — customer-io-pp-cli

## Spec

- Internal YAML at `$RESEARCH_DIR/customer-io-spec.yaml`
- 14 resources, ~49 endpoint operations
- Auth: `bearer_token` with `CUSTOMERIO_TOKEN` env var (the JWT — minted from the SA token by `auth login`)
- MCP enrichment applied: `transport: [stdio, http]`, `orchestration: code`, `endpoint_tools: hidden` (Cloudflare pattern, mandatory at this surface size)

## What was built (Priority 0 + 1 + 2)

**Priority 0 — Foundation (generated)**
- Data layer: 14 resource tables in SQLite (`customers`, `segments`, `campaigns`, `broadcasts`, `deliveries`, `suppressions`, `transactional`, `webhooks`, `cdp_sources`, `cdp_reverse_etl`, etc.)
- `sync` (per-resource and full)
- `sql` and `search` (offline FTS)
- `doctor` (auth + reachability check)

**Priority 1 — Absorbed surface (generated, ~49 typed commands)**
- `customers {search,get,list-segments,list-messages,list-activities}`
- `campaigns {list,get,metrics,journey-metrics,list-messages}`
- `broadcasts {list,get,metrics,trigger}`
- `segments {list,get,members,customer-count}`
- `transactional {send-email,send-sms,send-push,list-templates,get-template,template-metrics}`
- `deliveries {list,get}`
- `exports {list,get,download,start-segment,start-deliveries}`
- `suppressions {list,add,remove}`
- `webhooks {list,get,delete}`
- `cdp-sources {list,get}`
- `cdp-destinations` (promoted), `in-app` (promoted), `workspaces` (promoted)
- `cdp-reverse-etl {list,get,trigger}`
- Universal flags: `--json`, `--select`, `--csv`, `--compact`, `--dry-run`, `--data-source`, `--rate-limit`, `--timeout`, `--no-cache`, `--quiet`, `--plain`, `--no-input`, `--idempotent`, `--profile`, `--deliver`
- Generic passthrough `customer-io-pp-cli api <method> <path>` (covers any of the 879 endpoints not in the typed surface)
- Bundled MCP server with stdio + HTTP transports, code-orchestration mode, endpoint tools hidden
- MCPB bundle generated at `build/customer-io-pp-mcp-darwin-arm64.mcpb`

**Priority 2 — Hand-written transcendence (8 commands)**
- `auth login --sa-token` — exchanges `sa_live_*` for JWT via `POST /v1/service_accounts/oauth/token`, caches JWT + region in config (mirrors official `customerio/cli` flow byte-for-byte)
- `campaigns funnel <id> [--segment <id>] [--since 7d]` — live API journey_metrics, plus optional per-segment cross-cut from local `deliveries`
- `segments overlap <id-a> <id-b> ...` — live segment-membership fetch + in-memory bitset Venn-region computation
- `customers timeline <email|id> [--since 30d]` — local SQL join across `deliveries` × `suppressions` for one customer
- `broadcasts preflight <id> --segment <id>` — segment size + suppression overlap (live API) + last-sent recency (local cache); emits green/yellow/red verdict with structured reasons
- `suppressions audit [--since 30d] [--reason ...]` — local SQL: each suppression attributed to triggering bounce/complaint/dropped delivery (or "manual")
- `cdp-reverse-etl health [--watch]` — named verb over Reverse-ETL run history; --watch polls every 60s
- `suppressions bulk {add,remove} --from-csv|--from-jsonl|stdin` — fan-out with adaptive throttle (`cliutil.AdaptiveLimiter`) + per-day JSONL audit log at `~/.config/customer-io-pp-cli/audit/suppressions-YYYYMMDD.jsonl`
- `deliveries triage --bundle <dir>` — writes self-contained `summary.md + deliveries.jsonl + recipients.txt` for incident handoff

## What was deferred / out of scope (documented in the brief)

- Track API (`track.customer.io`) — different host + different auth (Site ID + API Key, Basic). v1 commits to single SA-token model. Reachable via the catch-all `api` passthrough only with manual auth header override.
- CDP write/ingest (per-source write keys) — separate credential family.
- `segments overlap --show-ids` exposes one extra slice per Venn region — feature works, just costs more memory at large segment sizes.
- Funnel × segment cross-cut requires `sync --resources deliveries` first; without sync data, the funnel still prints the live `journey_metrics` (no per-segment slice).

## Generator limitations encountered

- The generator emits `cdp-reverse-etl` as a single hyphenated top-level command (not a nested `cdp` parent + `reverse-etl` child). The narrative was originally "cdp reverse-etl health"; corrected to `cdp-reverse-etl health` and re-validated.
- The 879-operation live OpenAPI spec was not used directly — too large to digest as typed surface. Instead a focused 49-endpoint internal YAML covers the high-leverage workflows; everything else routes through the `api` passthrough.

## Build status

- `go fmt ./...` clean
- `go vet ./...` clean
- `go build` produces 18 MB binary in ~15 s
- All 7 generator quality gates pass (go mod tidy, go vet, go build, runnable binary, --help, version, doctor)
- `printing-press validate-narrative --strict --full-examples` — 10 of 10 narrative commands resolve against the CLI tree
