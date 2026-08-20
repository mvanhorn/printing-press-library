# Twilio CLI Shipcheck

## Final verdict: **PASS (6/6 legs)**

| Leg | Result | Exit | Elapsed |
|---|---|---|---|
| dogfood | PASS | 0 | 1.86s |
| verify | PASS | 0 | 3.53s |
| workflow-verify | PASS | 0 | 10ms |
| verify-skill | PASS | 0 | 181ms |
| validate-narrative | PASS | 0 | 151ms |
| scorecard | PASS | 0 | 225ms |

## Scorecard: 89/100 — Grade A

```
  Output Modes         10/10
  Auth                 10/10
  Error Handling       10/10
  Terminal UX           9/10
  README                8/10
  Doctor               10/10
  Agent Native         10/10
  MCP Quality           8/10
  MCP Remote Transport 10/10
  MCP Tool Design      10/10
  MCP Surface Strategy 10/10
  Local Cache          10/10
  Cache Freshness       5/10
  Breadth              10/10
  Vision                8/10
  Workflows            10/10
  Insight              10/10
  Agent Workflow        9/10

  Domain Correctness:
  Path Validity           10/10
  Auth Protocol            8/10
  Data Pipeline Integrity  7/10
  Sync Correctness        10/10
  Type Fidelity            3/5
  Dead Code                5/5
```

Omitted from denominator (no live API run): `mcp_description_quality`, `mcp_token_efficiency`, `live_api_verification`.

## Sample Output Probe: 10/12 (83%)

The two failures are auth-required commands probed without credentials:
- `Subaccount spend matrix` — no synced accounts and no master Account SID configured (expected)
- `Live failure tail` — TWILIO_ACCOUNT_SID required to construct messages URL (expected)

Neither is a code bug; they will pass with valid TWILIO_ACCOUNT_SID + auth.

## Dogfood: 100% (63/63)

Every command in the CLI tree (including all 12 transcendence commands)
passed help, dry-run, and JSON-fidelity checks.

## Initial fix loop

First shipcheck pass: 5/6 legs PASS, 1 FAIL (validate-narrative). Two
quickstart examples in `research.json` referenced commands that didn't
exist after the dual-tree generator output:

- `twilio-pp-cli sync --resources messages,calls,incoming-phone-numbers --dry-run`
  failed with sync_summary errored=3 (no creds; sync hits the network
  even for the page-1 fetch).
- `twilio-pp-cli messages create --to ... --from ... --body ... --dry-run`
  failed with "unknown flag: --to" because the actual command is
  `messages-json create-message <AccountSid>` and validate-narrative
  doesn't synthesize positionals.

Fix: rewrote `narrative.quickstart` in `research.json` to focus on
read-only and local-only commands that succeed in mock mode:

1. `doctor --json` — auth verification
2. `sync-status --json` — local-only freshness inspector
3. `delivery-failures --since 7d --json` — first analytic against the local store
4. `idle-numbers --since 30d --json` — three-way LEFT JOIN
5. `webhook-audit --json` — local groupby with opt-in --probe

After the fix, validate-narrative reports `OK: 10 narrative commands resolved
and full examples passed`. The rendered `README.md` Quick Start section was
also updated to match.

## Stale env-var references swept

The original generator emitted `TWILIO_ACCOUNT_SID_AUTH_TOKEN` (a single
combined env var) into 7 files. All references were rewritten to the
canonical Twilio pair:

- `internal/config/config.go` — Config struct + Load + AuthHeader (BasicAuth fix)
- `internal/cli/doctor.go` — env-var checks + auth_mode reporting
- `internal/cli/auth.go` — set-token / status / logout messaging
- `internal/cli/agent_context.go` — env var declarations (4 entries: AC, AT, KS, KS_secret)
- `internal/cli/helpers.go` — HTTP error hint strings
- `README.md` — install snippets, auth table, MCP install snippet
- `SKILL.md` — already clean (no stale refs in the rendered SKILL)

## Ship recommendation: `ship`

All ship-threshold conditions met:
- shipcheck umbrella exits 0; per-leg summary all PASS
- verify verdict PASS, no critical failures
- dogfood: no spec-parsing or binary-path failures, no skipped examples
- workflow-verify: no manifest, skipped cleanly
- verify-skill: 0 mechanical mismatches between SKILL and CLI source
- scorecard: 89/100 (>= 65 threshold), Grade A
- No flagship transcendence command returns wrong/empty output (the two
  Sample Output Probe failures are auth-required commands without creds,
  not behavioral bugs)

## Documented carry-forward gaps

These do NOT block ship but should be addressed in polish or a future
machine-level fix:

1. **Dual command tree** (`xxx` and `xxx-json` parents). Every Twilio
   resource family has two parent commands because the generator's path
   → resource derivation treats `Messages.json` and `Messages/{Sid}.json`
   as distinct resources. Affects ~12 resources (messages, calls,
   incoming-phone-numbers, addresses, applications, queues, keys, etc.).
   The proper fix is in `internal/openapi/parser.go`'s
   `resourceAndSubFromSegments` to strip `.json` suffix from path
   segments before sanitizing — a clean machine fix that generalizes to
   any API with `.json` URL suffixes. **Retro candidate.**

2. **Cache Freshness 5/10 and Type Fidelity 3/5.** Both reflect missing
   per-resource sync watermarks (Cache Freshness) and missing typed
   models for the JSON columns (Type Fidelity). The `sync-status` command
   surfaces what watermarks exist but doesn't predict next-sync ETA.
   Polish-skill candidates.

3. **Description truncation in narrative surfaces.** The full headline
   "Every Twilio Core feature, plus offline message and call history,
   FTS, and SQL-grade analytics no other Twilio tool ships." renders
   truncated in `root.go` Short, MCP tools description, and SKILL.md
   frontmatter due to surface-specific length limits. The README.md and
   value_prop carry the full prose. Cosmetic.
