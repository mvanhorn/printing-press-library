# SolarEdge CLI Shipcheck Report

## Command

```
cli-printing-press shipcheck --dir .../working/solaredge-pp-cli --spec .../research/solaredge-spec.yaml --research-dir .../runs/20260630-095604-6b32e05e
```

## Per-leg results

| Leg | Result | Notes |
|---|---|---|
| verify | PASS | 100% (18/18), 0 critical. Mock mode. |
| validate-narrative | PASS | 9/9 narrative commands resolved and full examples passed |
| dogfood | PASS | WARN-level only: 1 dead helper function (`writeNoop`, generator-emitted, out of scope — internal/cliutil/cobratree convention, not hand-written code) |
| workflow-verify | PASS | `workflow-pass` (no workflow manifest authored; not required for this API shape) |
| apify-audit | PASS | n/a, no Apify references |
| verify-skill | PASS | All checks passed: flag-names, flag-commands, positional-args, shell-var-quotes, unknown-command, canonical-sections |
| scorecard | PASS | 90/100, Grade A |

**Shipcheck umbrella verdict: PASS (7/7 legs).**

## Blockers found and fixed (this loop)

1. **`version` resource name collision.** The spec used `version` as a resource key, which collides with the framework's built-in `<cli> version` command. Renamed to `api-version`. (1 generation retry.)
2. **camelCase flag references in `example`/`happy_args`.** Several spec fields and one `research.json` recipe referenced `--startDate`/`--endDate`/`--startTime`/`--endTime`, but the generator emits kebab-case flags (`--start-date`, `--end-time`, etc.). Caught by `validate-narrative --full-examples` (`unknown flag: --startTime`). Fixed all instances in the spec and research.json, regenerated, re-verified clean.
3. **Description/rationale drift for 3 of 5 novel features.** During implementation, `site changes` was scoped down from "diffs energy, equipment status, and battery cycles" to "energy delta + a current (non-delta) equipment snapshot" because the API has no equipment-history endpoint to diff against, and `site health`/`site underperformance` descriptions referenced "locally synced" data when the actual implementation calls the live API directly (no per-site sync path exists — see Build Priorities below). Updated the absorb manifest's "Why Only We Can Do This" column and every affected `research.json` field (`description`, `rationale`, narrative `value_prop`, `quickstart`) to match the as-built behavior before this shipcheck run.

## Before/after

- Verify pass rate: 100% throughout (no failures introduced or fixed at the verify layer this loop — fixes were narrative/spec-level).
- Scorecard: 90/100 Grade A (single run; no regression loop needed).

## Scorecard gaps (non-blocking, documented for the record)

- `Type Fidelity 2/5`, `Insight 4/10` — structural scoring against deeply nested response types (SolarEdge wraps most responses in single-key envelopes with further-nested meter/telemetry arrays); the spec intentionally leaves these as `object`/raw-JSON rather than fully expanding every nested field, since the API's own response shapes are highly variable per endpoint (see spec.yaml `types:` section). Does not affect correctness — full JSON is preserved and returned to the caller either way.
- `Cache Freshness 5/10` — cache.enabled was deliberately left off (see Pre-Generation Cache Enrichment decision in the brief): SolarEdge's per-site resources require a siteId and aren't sync-eligible, and the API's hard 300/day rate limit means an auto-refresh-before-serve pattern risks silently burning the user's daily budget — exactly the failure mode `budget status` exists to prevent.
- `MCP Token Efficiency 7/10`, `MCP Tool Design 7/10` — default endpoint-mirror MCP surface for a ~44-tool CLI; user explicitly chose stdio-only / no Cloudflare-pattern orchestration during Pre-Generation MCP Enrichment (CLI is well under the 50-tool auto-orchestration threshold).

## Phase 4.8 / 4.9 review findings

- **Phase 4.8 (Agentic SKILL Review):** 1 warning — trigger phrase `"SolarEdge battery status"` didn't map to any real command (the battery-health novel feature was cut at the Phase 1.5 approval gate). Fixed by replacing it with `"check my SolarEdge equipment for faults"` in `research.json`, regenerated, re-verified. All other checks (verified-set alignment, description accuracy, stub disclosure, auth narrative, recipe claims, marketing-copy smell) passed clean.
- **Phase 4.9 (README/SKILL/AGENTS audit):** No error-severity findings. Medium/Low findings are all generator-template-level, not fixable without editing DO-NOT-EDIT generated files — logged here as retro candidates rather than patched:
  - "Read-only by default" wording in README/SKILL implies a toggleable mutating mode that doesn't exist for this 100%-read-only API (template boilerplate, not research.json-driven).
  - Generic mutating-state caution boilerplate in AGENTS.md doesn't apply to this all-GET API (template language).
  - `internal/cli/root.go` and `internal/cli/auth.go` render the brand as "Solaredge" (lowercase e) instead of "SolarEdge" — the generator's slug-capitalization fallback doesn't consult `narrative.display_name` for this particular text path (also independently noticed during Phase 2's description-surface check). Confirmed: README.md/SKILL.md/AGENTS.md prose all use the correct "SolarEdge" casing; only this one generated-code surface is affected.
  - Confirmed clean: no leaked references to the 3 cut novel features anywhere in README/SKILL/AGENTS; all command paths, exit codes, and auth description verified accurate.

## Live-check sample probe (no API key available)

`scorecard --live-check` sampled all 5 novel features against the real API. 1/5 passed (`budget status`, the only purely local command). The other 4 (`site health`, `site underperformance`, `site changes`, `equipment faults`) each failed with a clean `exit 4` + `HTTP 401 "API key missing in the request"` — correctly classified auth errors, not silent or wrong output. This is expected: the user explicitly declined to provide a `SOLAREDGE_API_KEY` during the Phase 0.5 API Key Gate. No flagship feature returned wrong/empty output; all returned the correct, actionable auth-error response for the credential-less state.

## Final ship recommendation: **ship**

All ship-threshold conditions met. No known functional bugs in shipping-scope features. Live behavioral correctness will be confirmed in Phase 5 if/when an API key becomes available; until then, Phase 5 will record a documented `auth_required_no_credential` skip per the skill's rules (no fabricated pass marker).
