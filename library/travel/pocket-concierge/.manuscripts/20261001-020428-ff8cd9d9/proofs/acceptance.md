# Live acceptance

Full Press matrix: 35/35 mandatory checks pass, zero failures. Nine error-path probes skipped because this CLI uses named flags rather than positional arguments; the offline schema dry-run contract probe skipped because it emits its schema directly without IO. These ten skips are not claimed as passed tests. Dedicated live E2E: 20/20 source correctness/query relevance rows pass, including wrong course, missing ID, invalid date, impossible party, and deliberately empty search. Consequential synthetic tests cover additional parsing, HTTP/GraphQL failures, 429/timeouts, cache ownership, nulls/party bounds and projection. No fixtures substituted for live results.

Marker evidence/phase5-acceptance.json was written by the Press runner, not authored manually, and is bound to the final Go source fingerprint. Source reads only: no reservation, payment or account action.
