# Stripe CLI — Build Log

## Generation

- Spec: `https://raw.githubusercontent.com/stripe/openapi/master/openapi/spec3.json` (~7.7 MB, ~500 endpoints, ~200 resources)
- Catalog entry: `stripe` (tier=official, verified 2026-03-23)
- Generator: printing-press v4.6.1 (upgraded from v4.5.2 at preflight)

### First pass (raw spec)

- Warning: 587 MCP endpoint tools exceeds >50 threshold; default endpoint-mirror surface scores poorly on MCP architectural dimensions.
- Built successfully (go mod tidy, govulncheck, go vet, go build, `--help`, version, doctor all PASS).

### Second pass (with `x-mcp` Cloudflare pattern)

Patched the downloaded spec at `$API_RUN_DIR/stripe-spec.json` with:

```json
"x-mcp": {
  "transport": ["stdio", "http"],
  "orchestration": "code",
  "endpoint_tools": "hidden"
}
```

Re-ran generate. Warning eliminated. Same quality gates PASS. The 587-tool registry is on disk; runtime exposes the thin `stripe_search` + `stripe_execute` orchestration pair (~1k tokens vs ~50k tokens at startup).

## Hand-built novel features

Generator emits Priority 0 (data layer / `sync` / FTS5 `search`) and Priority 1 (absorbed API endpoints) automatically. The following Priority 2 transcendence commands were hand-built:

| File | Parent | Command | LoC |
|---|---|---|---|
| `internal/cli/customers_profile.go` | `customers` | `customers profile <id_or_email>` | ~135 |
| `internal/cli/payouts_explain.go` | `payouts` | `payouts explain <po_id>` | ~180 |
| `internal/cli/charges_why.go` | `charges` | `charges why <ch_id>` | ~110 |
| `internal/cli/subscriptions_churn.go` | `subscriptions` | `subscriptions churn` | ~170 |
| `internal/cli/payments.go` | root | `payments` (parent) | ~15 |
| `internal/cli/payments_failed.go` | `payments` | `payments failed` | ~140 |

### Pattern used

- `Use: "<leaf> [arg]"` with square brackets (optional positional) where the command takes an ID — keeps verify-skill happy and Cobra falls through to help on no-arg invocations.
- `RunE` always opens with `if len(args) == 0 { return cmd.Help() }` (where applicable) and `if dryRunOK(flags) { return nil }` for verify-friendliness.
- `Annotations: map[string]string{"mcp:read-only": "true"}` on every novel command (none mutates external state).
- Output flows through `flags.printJSON(cmd, out)` — `--select`, `--compact`, `--csv`, `--quiet` all inherit for free via the generator's `printOutputWithFlags` plumbing.

### Helpers added

- `unmarshalList(raw)` — parses Stripe list envelopes `{data: […], has_more}` and returns `[]map[string]any`.
- `parseSinceArg(s)` — accepts Unix ts, RFC3339, `YYYY-MM-DD`, and relative durations (`7d`, `24h`, `30m`).
- `extractCancelReason`, `effectiveGroupBy`, `computeSubscriptionMRR` for `subscriptions churn`.
- `bucketKey`, `effectivePaymentsGroupBy` for `payments failed`.
- `lookupChargeCustomer`, `lookupCustomerName` (best-effort follow-on API calls) for `payouts explain` attribution.

## Auth env var fix

`internal/config/config.go` was edited to make `STRIPE_SECRET_KEY` (canonical, used by stripe-cli/stripe-go/stripe-node/stripe-python) win over the generator's default `STRIPE_BEARER_AUTH`. `STRIPE_API_KEY` is recognized as an alias. `STRIPE_BEARER_AUTH` is preserved as a fallback so existing setups don't break.

## Intentionally deferred

- **Customer timeline** (`customers timeline`) — high-value (score 8) but cut for v1 time budget; data is in the local store after `sync`, command is straightforward to add later.
- **Top-customers attribution in `payouts explain`** is implemented but gated behind `--top-customers N` (default 0) — extra API call per charge. Off by default to keep the command fast and not blow rate limits.
- **Typed Go structs for novel output** — currently `map[string]any` pass-through of raw Stripe payloads. Scorecard `type_fidelity` 2/5 reflects this; planned for a polish or v1.1 pass.

## Generator limitations encountered

- Auth env var name defaults to `<API_NAME>_BEARER_AUTH` rather than the API's canonical name (`STRIPE_SECRET_KEY`). The pre-generation enrichment in SKILL.md says to add `x-auth-env-vars` to the spec; that was missed in this run and patched post-gen in `config.go` instead. A retro candidate.
- Spec truncation (50 resources, 20 endpoints/resource) is silent — the resources that get cut are not surfaced in the build output. Acceptable for Stripe (the surface stayed wide) but worth a retro note about visibility.
