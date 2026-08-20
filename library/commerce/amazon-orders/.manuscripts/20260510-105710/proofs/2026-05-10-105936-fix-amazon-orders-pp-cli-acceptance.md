# Acceptance Report — amazon-orders-pp-cli

## Verdict: PASS — Full Dogfood, 79/79 tests passed against live amazon.com

## Setup
- `pipx install pycookiecheat` succeeded; system Python additionally got `pycookiecheat 0.8.0` via `--break-system-packages` so the CLI's `python3 -c "import pycookiecheat"` detector finds it.
- `auth login --chrome` auto-detected Chrome profile "Person 1 (Default)", read 21 cookies for `.amazon.com`, validated the session against `/gp/your-account/order-history`, and wrote the proof to `~/.config/amazon-orders-pp-cli/config.toml`.

## Live matrix results

| Test class | Count | Result |
|---|---|---|
| Help walk | 26 | 26 PASS |
| Happy path | 26 | 26 PASS |
| JSON fidelity | 23 | 23 PASS |
| Error path | 4 | 4 PASS |
| Skipped (no positional argument applicable) | 42 | n/a |
| **Total** | **79** | **79 PASS / 0 FAIL** |

The 3 `error_path` failures from the first live run (orders get / orders invoice / track all returning exit 0 on bogus order IDs) were resolved by adding `isValidOrderID(...)` validation in front of the live fetch. Now `orders get foo` correctly exits 1 with `invalid order ID "foo": expected canonical Amazon shape XXX-XXXXXXX-XXXXXXX`.

## Smoke (PII-redacted)

The user-visible novel commands all parsed real Amazon HTML:

- `orders list --json` → 10 orders in the recent window, each with `placedDate`, `total`, `status`, `shipTo`, `etaDate`/`deliveredOn`, `asins[]`, `itemTitles[]`, detail/track/invoice URLs.
- `where-is-my-stuff --json` → `[]` (empty list, correct shape — every recent order is delivered).
- `arriving-soon --days 30 --json` → `[]` (correct, none in the window).
- `late --json` → `[]` (correct, none).
- `spend --by month --year 2026 --json` → 5 month buckets (2026-01..2026-05) with `orderCount` per bucket.
- `top-items --by count --limit 5 --json` → 5 ASIN-grouped rows with order counts (e.g., the user's most-frequently-ordered ASIN appeared 5 times).
- `find '<query>'` → returns matching orders (FTS-style live filter; FTS5 store path is documented for v2).
- `auth status` → `Authenticated, Source: browser, Domain: .amazon.com`.
- `auth export | auth import` roundtrip → preserves the 2693-char cookie blob via `amazon-orders-session/v1` JSON shape; cookies survive disk-roundtrip and the CLI re-validates.

## Fixes applied during Phase 5

1. **Order-ID validation (3 commands)**. `orders get`, `orders invoice`, `track` now validate the canonical shape (`XXX-XXXXXXX-XXXXXXX` or `D01-…`) before issuing an HTTP request. Amazon silently redirects unknown order IDs to a default page, which the parser would have happily parsed; validation surfaces the user mistake at exit code 1.
2. **`null` → `[]` for empty novel-command results**. `inflightOrders`, `arrivingByDay`, `lateOrders` now allocate empty slices so JSON output renders as `[]`, never `null`.
3. **`auth export` cookie source**. The generator's `SaveTokens(...)` writes cookies into `c.AccessToken`, not `c.AmazonCookies`. `auth export` now reads from whichever is populated.

## Printing Press issues for retro

- **HTML-response generator should not emit `ScriptSelector: "script#__NEXT_DATA__"` by default**. For sites without Next.js (Amazon, most non-Vercel sites), the generic `extractHTMLResponse` returns garbage. The skill's Phase 3 build expects custom parsers in this case, but the generator could emit a `// TODO(parser)` comment instead of the misleading `__NEXT_DATA__` selector.
- **`SaveTokens(clientID, clientSecret, accessToken, refreshToken, expiry)` is OAuth-shaped but used to store cookie blobs.** The cookie ends up in the `access_token` TOML field even though the spec declared `auth.type: cookie`. A `SaveCookies(domain, cookies string)` method that targets `c.AmazonCookies` would be cleaner.
- **`mcp:read-only` annotations on hand-written commands work**, but the generator could pre-fill `Annotations: map[string]string{"mcp:read-only": "true"}` when the spec's `endpoints.<name>.method` is `GET`. Today this is hand-managed.

## Auth context

```json
{
  "schema_version": 1,
  "api_name": "amazon-orders",
  "run_id": "20260510-105710",
  "status": "pass",
  "level": "full",
  "matrix_size": 79,
  "tests_passed": 79,
  "tests_skipped": 42,
  "auth_context": { "type": "cookie" }
}
```

Gate: PASS — proceed to Phase 5.5 (polish), then promote and archive.
