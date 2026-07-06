# Agilix Dawn CLI — Acceptance Report

Level: Full Dogfood (live, against drivered.agilixdawn.com)
Tests: 73/73 passed
Gate: PASS

## Coverage
Binary-owned live matrix across every leaf subcommand: help, happy-path, JSON parse
validation, output-mode fidelity, and error paths. All commands read-only.

## Spot-verified behaviors (values redacted for PII)
- doctor: auth valid, API reachable, credentials valid.
- concept list / get: returns the tenant's real catalog (2 courses).
- course stats (big course): 34 sections / 392 instructions / 1223 interactions / 617 pts / 47.0h.
- course tree / outline: full nested structure rendered and exported (md + csv).
- roster export: returns the real user roster as CSV/JSON (user PII present in output by design; not persisted here).
- purchase reconcile: joins purchases to users; unmatched buyers fall back to id with buyer_matched=false.
- catalog diff: baseline recorded on first run; change detection on subsequent runs.
- config: tenant config (name, root org).

## Failures
None.

## Fixes applied during shipcheck (pre-dogfood)
- config invocation corrected from `config get` to `config` across narrative + README.
- course outline md header includes concept id + status.

## Printing Press issues for retro
- Generator promoted a single-endpoint `config` resource to a top-level `config` command but the
  narrative/README used the `<resource> <endpoint>` form (`config get`), which verify-skill then
  flagged. Consider auto-rewriting promoted-command examples to the promoted path.
