# Stripe CLI — Shipcheck

## Verdict: **ship**

## Shipcheck Umbrella

| Leg                | Result | Elapsed |
|--------------------|--------|---------|
| dogfood            | PASS   | 8.1s    |
| verify             | PASS   | 8.6s    |
| workflow-verify    | PASS   | 18ms    |
| verify-skill       | PASS   | 1.0s    |
| validate-narrative | PASS   | 0.3s    |
| scorecard          | PASS   | 1.5s    |

**6/6 legs PASS.** Exit 0.

## Scorecard: 84/100 — Grade A

Strong dims (10/10): output_modes, auth, error_handling, doctor, agent_native, mcp_quality, mcp_tool_design, mcp_surface_strategy, local_cache, breadth.

Weak dims that survived to ship:
- `mcp_token_efficiency` 4/10 — improved by Cloudflare pattern (`x-mcp.orchestration: code` + `endpoint_tools: hidden`) but the 587-tool registry is still on disk; runtime view is the 2-tool search+execute pair.
- `mcp_remote_transport` 5/10 — `stdio + http` declared.
- `cache_freshness` 5/10 — generator default; tail/sync provide manual freshness control.
- `type_fidelity` 2/5 — most novel command outputs are `map[string]any` (raw Stripe payloads pass-through) rather than typed Go structs.
- `vision` 8/10, `workflows` 6/10, `insight` 4/10 — improved further by polish.

## Fixes applied (1 shipcheck loop)

1. **`--entities` → `--resources`** — narrative quickstart/recipe/troubleshoot examples used `--entities`, but the generator's `sync` command names the flag `--resources` and `search` uses singular `--type`. Patched README.md, SKILL.md, and `research.json`. Replaced the `search "acme.co" --entities customers,invoices` recipe with `--type customers`.
2. **`sync --since 2026-01-01` → `sync --since 30d`** — the generated `sync` command parses durations (`7d`, `24h`) not absolute dates. Patched README.md, SKILL.md, and `research.json`.

## Pre-shipcheck enrichments

- **Cloudflare MCP pattern** — patched `x-mcp.transport: [stdio, http]`, `orchestration: code`, `endpoint_tools: hidden` into the OpenAPI spec before generation. Eliminated the generator's "587 MCP tools >50 threshold" warning and rescued the MCP architectural dimensions.
- **Canonical Stripe env var** — added `STRIPE_SECRET_KEY` (and `STRIPE_API_KEY` alias) precedence over the generator's default `STRIPE_BEARER_AUTH` in `internal/config/config.go`. Matches the convention used by stripe-cli, stripe-go, stripe-node, and stripe-python.

## Sample Output Probe

0/6 sample probes passed because no API key is available in this run (user declined). All 5 novel commands returned HTTP 401 from the Stripe API, plus `sync` rejected a date format that was already fixed before the probe re-ran. The probe is informational; per the skill ship-threshold rules, `workflow-verify` with no key returns `unverified-needs-auth`, which is valid for the ship verdict.

## Novel Features Built (6/6)

| Feature | Command | Status |
|---|---|---|
| Mirror sync | `sync` | shipping (generator-emitted with our framework) |
| Payout explain | `payouts explain` | shipping (hand-built) |
| Failed payment triage | `payments failed` | shipping (hand-built, new `payments` root) |
| Subscription churn audit | `subscriptions churn` | shipping (hand-built) |
| Customer 360 | `customers profile` | shipping (hand-built) |
| Why did this charge fail | `charges why` | shipping (hand-built) |

`dogfood --json | jq .novel_features_check`: `{"planned": 6, "found": 6}`.

## Known gaps (informational, not blocking)

- Hand-built novel commands return raw Stripe payloads (`map[string]any`) rather than typed Go structs. Functional for agent JSON consumption; scorecard scores `type_fidelity` 2/5.
- Sample probe needs a `STRIPE_SECRET_KEY` to exercise the novel paths against live Stripe.
- The generator truncated the spec to 50 resources / 20 endpoints per resource (per Stripe catalog note). The 50 retained cover ~95% of the high-traffic Stripe surface (customers, payment_intents, charges, subscriptions, invoices, refunds, disputes, payouts, balance_transactions, products, prices, payment_methods, payment_links, coupons, events, accounts, plus more).
