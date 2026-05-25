# Live smoke test — jinko-pp-cli v0.1

Ran against `https://mcp-bff.dev.gojinko.com` (dev environment) on 2026-05-25 with a freshly-issued API key.

Date used: `+30 days` from now.

| # | Command | Endpoint | Result | Bytes returned |
|---|---------|----------|--------|----------------|
| 1 | `auth status --api-key jnk_...` | local | `authenticated=true source=flag method=api_key` | n/a |
| 2 | `find-flight --from PAR --to NYC --date ...` | `POST /api/v1/flights/search` | ✓ 200, real itineraries with `offer_token`s | 6852 |
| 3 | `find-destination --from PAR --date ...` | `POST /api/v1/flights/destination-search` | ✓ 200, ranked destinations | 21943 |
| 4 | `flight-search --from PAR --to NYC --date ... --return ... --passengers 1` | `POST /api/v1/flights/shop` | ✓ 200, live fares with `trip_item_token`s | 73959 |
| 5 | `hotel-search --query Paris --checkin ... --checkout ... --adults 2 --max-results 3` | `POST /api/v1/hotels/shop` | ✓ 200, bookable rates with `offer_id`s, prices, cancellation schedules | 62566 |

Truncated sample payloads are in `smoke-*.json` (3KB each). Full responses contain the agent-friendly token chains documented in `README.md` — `offer_token`, `trip_item_token`, `offer_id` — exactly as the upstream `@gojinko/cli` returns them.

Not exercised in this smoke (require user-side payment for end-to-end proof):
- `trip` (multi-product cart add)
- `book` (Stripe checkout URL)
- `trip-status` (lifecycle polling)

These three commands hit the same BFF endpoints used by the Node CLI's own staging smoke (`jinko-dev-tools/scripts/smoke-test.sh`) and share the same request envelopes, so a green search-tier smoke is a strong indicator that the cart-tier commands will work end-to-end. They are intentionally deferred to manual review-time to avoid creating noisy test trips in the dev tenant.

## How to reproduce

```bash
export JINKO_API_BASE=https://mcp-bff.dev.gojinko.com
export JINKO_API_KEY=jnk_...    # get one at https://app.gojinko.com/devplatform
DATE=$(date -v+30d +%Y-%m-%d)   # macOS; on Linux: date -d '+30 days' +%Y-%m-%d
RET=$(date -v+37d +%Y-%m-%d)

jinko-pp-cli find-flight --from PAR --to NYC --date "$DATE" --limit 2
jinko-pp-cli find-destination --from PAR --date "$DATE" --limit 3
jinko-pp-cli flight-search --from PAR --to NYC --date "$DATE" --return "$RET" --passengers 1
jinko-pp-cli hotel-search --query Paris --checkin "$DATE" --checkout "$RET" --adults 2 --max-results 3
```
