# Shipcheck — sea-airport-parking-pp-cli

Verdict: **ship**

## Legs (final)
| Leg | Result |
|-----|--------|
| verify | PASS |
| validate-narrative | PASS |
| dogfood | PASS |
| workflow-verify | workflow-pass |
| apify-audit | pass |
| verify-skill | PASS (exit 0) |
| scorecard | 84/100 Grade A |

Sample Output Probe: 5/5 (100%).
Live dogfood matrix: 85/85 PASS (full level).

## Blockers found & fixed in-session
1. `original_price` mis-parse (grabbed wrong originalPrice occurrence) → read price+originalPrice from the same first quote object; only surface a genuine higher list price. **Fixed.**
2. `history`/`drift` "readonly database" — ListQuotes ran CREATE TABLE on a read-only connection → ListQuotes no longer writes; missing table returns empty. **Fixed.**
3. `sweep`/`calendar` timeouts — warmed the session on every quote → warm once per client, reuse session. **Fixed** (sweep 11-day: 10s→2.8s).
4. `parking` generated endpoint redirect-loop — root cause: generic HTTP client built with a **nil cookie jar**, so it couldn't hold the session cookie across the AeroParker redirect chain → gave the generic client a real cookie jar (patch: session-cookie-jar-on-http-client). **Fixed** — parking now fetches the live page.
5. `watch --json` emitted a non-JSON status line on the curtailed/unavailable path → machine mode emits only valid JSON. **Fixed.**

## Before/after
- Scorecard: 84/100 Grade A (Path Validity 10/10, Dead Code 5/5, Insight 10/10, Local Cache 10/10, Workflows 10/10).
- Live dogfood: 85/85.

Ship recommendation: **ship**.
