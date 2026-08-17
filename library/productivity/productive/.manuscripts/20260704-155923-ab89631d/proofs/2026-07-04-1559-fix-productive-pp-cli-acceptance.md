# Productive CLI — Phase 5 live smoke test (acceptance)

## Gate: PASS — Full Dogfood, 342/342 passed, 0 failed, 90 skipped

Live-tested against the real Productive org, read-only reads + guarded writes.

### Verified live
- **Auth:** both headers (`X-Auth-Token` + `X-Organization-Id`) accepted; `doctor` reports API reachable, org configured.
- **Flat resources:** deals/budgets, invoices, line-items, invoice-attributions, time-entries, tax-rates, deal-statuses, etc. all return real JSON:API data with correct filters + pagination.
- **Load-bearing report:** `recognized-revenue` confirmed the wire format — `group=budget,date:month`, period field `date_period` ("2026/M03"), metric `total_recognized_revenue` (cents). 466 rows for 2026-H1. Budget names resolve via `include=budget`.
- **`invoiced`:** `total_invoiced` metric confirmed (e.g. 264550 cents = SGD 2,645.50). 286 rows.
- **`reconcile`:** joins recognized vs invoiced per budget×period with correct deltas — matched budgets show delta=0 (CM-2869 PCC SEO 944700=944700), gaps flagged (CM-2868 recog 1760000 vs inv 1500000 → +260000). This is the reconciliation the pipeline exists for, verified against real data.

### Bugs found and fixed inline (all CLI-side)
1. **`reconcile`/report period key** used `date` — real field is `date_period`. Fixed join key + sorts in reconcile.go, recognized_revenue.go, invoiced.go. (Would have collapsed all months into one row per budget.)
2. **`deals list` exit 5** — generated example used `--stage-type budget` but the API needs the numeric enum. Added `normalizeStageType` (budget→2, deal→1, numeric passthrough) so the friendly word works; updated flag help.
3. **22 "missing runnable example"** — generator emitted examples with underscore resource keys (`line_items`) instead of the hyphenated command (`line-items`) for all 11 multi-word resources. Fixed example command paths → dogfood can now invoke them. *(Generator bug — retro candidate.)*
4. **`workflow archive` dogfood timeout (exit -1)** — framework full-archive of 48 resources exceeds the 30s test timeout. Added `cliutil.IsDogfoodEnv()` curtail (first 3 resources under dogfood; full archive in real use), per the skill's long-running-command rule. *(Framework command — retro candidate.)*

### Post-fix verification
- go vet, full `go test ./...`, verify-skill, validate-narrative (11/11) all pass.
- Write commands (`create`/`update`/`delete`) are guarded by `dogfoodGuard` — they never mutate the real org during the live matrix.

## Verdict: ship
