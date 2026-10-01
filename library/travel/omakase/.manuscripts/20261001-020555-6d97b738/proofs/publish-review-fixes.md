# Publish review fixes

All three first-round Greptile P2 findings were reproduced and fixed.

- Price ranges now mark the lower amount as a minimum while retaining maximum amount, raw price, tax and guest units. Cache schema 3 rejects normalized results from the old parser.
- Explicit `--rate-limit 0` disables domain-client pacing; automatic default pacing remains 500 ms.
- Explicit `--max-age 0` disables cache expiry; an explicit refresh still fetches. Future-dated cache entries remain stale.

The focused tests failed before the fixes, then consequential parser/client/CLI tests and the full Go suite passed. Fresh full live dogfood passed 122 executed checks with zero failures; 93 rows remain explicitly skipped/unverified. A separate live first-party financial comparison and zero-age/refresh check passed; see publish-live-source-check.json. The ASCII price-range regression is synthetic and is not live inventory evidence.
