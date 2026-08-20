# eRank CLI Live Smoke

- Source: authenticated browser session approved for browser-sniff.
- Validation endpoint: GET /api/keyword-tool/stats?keyword=dad+mug&country=USA&marketplace=etsy
- Result: HTTP 200 using captured browser-session cookies plus XSRF header.
- Local generated doctor: browser_session_proof=valid, auth_source=config.
- Full live dogfood: PASS, 129 passed, 72 skipped.
- Acceptance marker: /Users/smacdonald/printing-press/.runstate/grumpykinco-7e542738/runs/20260522-130049/proofs/phase5-acceptance.json, status pass.

Secrets are intentionally omitted from this proof.
