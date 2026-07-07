# Phase 1.9 Reachability Gate — PASS

- Probe: GET https://api.scrapecreators.com/v1/account/credit-balance (no key)
- Result: HTTP 500, body {"success":false,"message":"apiKey is required"}
- Decision: PASS — API is reachable and returns a structured auth-required response.
  The 500 is the vendor's (non-standard) "auth required" signal; equivalent to the
  matrix's "401 (no key provided) → PASS (expected when API needs auth and user
  declined key)". Not a bot-block, not a transport failure.
- API key: none in environment. Phase 5 live dogfood limited to no-key paths unless
  the user supplies SCRAPECREATORS_API_KEY.
