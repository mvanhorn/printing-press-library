# The Points Guy CLI — Shipcheck Report

## Verdict: ship

## Shipcheck umbrella: PASS (7/7 legs)
- verify PASS, validate-narrative PASS, dogfood PASS, workflow-verify PASS,
  apify-audit PASS, verify-skill PASS, scorecard PASS.

## Scorecard: 81/100 — Grade A
- Strong: Output Modes 10, Terminal UX 10, README 10, Doctor 10, Agent Native 10,
  Local Cache 10, Workflows 10, MCP Remote Transport 10, MCP Desc 10, Path Validity 10.
- Weaker (accepted): Cache Freshness 3 (read-through, no pre-read refresh path);
  Dead Code 2 (3 unused GENERATED helpers in helpers.go — template-shape, retro candidate,
  not patched per Phase 4.95 escape hatch); Breadth 7, Insight 7.
- Sample Output Probe: 6/6 (100%) after fixing portfolio positional args + since window field.

## MCP surface
- Runtime cobratree mirror exposes 23 tools (all commands). Static tools-manifest.json
  lists 2 (the spec endpoints) — cosmetic; runtime surface is complete. Readiness: full.

## Reviews (performed inline)
- Doc audit (4.8/4.9): fixed a real bug — `cards compare` example/recipe used invalid slugs
  (amex-platinum) that would 404; corrected to chase-sapphire-preferred-card/chase-sapphire-reserve
  and re-synced README+SKILL. Anti-triggers present; read-only disclosed; no placeholder leaks
  in executable examples; brand name "The Points Guy" used throughout.
- Output review (4.85): fixed portfolio (accept positional balances), since (echo window),
  glossary (honest title+URL, suppress generic tagline), read (JSON-LD body + filtered
  paragraph fallback, boilerplate stripped).
- Code review (4.95): parameterized store access; adaptive rate limiting + typed RateLimitError;
  boundCtx timeout on all sibling-client calls; safe index-keyed goroutine fan-out in cards compare;
  correct typed exit codes (2 usage / 3 not-found / 0 ok, verified). No secrets in source — the
  public Algolia app id + search key are discovered at runtime from the site bundle, never baked in.

## Behavioral verification (live)
- valuations (34 programs), worth, redeem-check, portfolio, cards get/list/best/compare,
  search (+--select), suggest, latest, since, read, browse, glossary, sync, valuations drift
  all exercised live against thepointsguy.com and return correct, structured output.

## Ship recommendation: ship
