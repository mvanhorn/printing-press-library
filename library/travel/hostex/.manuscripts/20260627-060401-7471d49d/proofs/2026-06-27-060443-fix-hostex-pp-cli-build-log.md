Manifest transcendence rows: 7 planned, 7 built. Phase 3 complete.

Built (hand-code):
- ops-gaps (local), inbox-sla (local), revenue-rollup (local), stay-brief (local), automation-preview (local)
- price-parity (live), oversell-watch (live)

Client fix: added Hostex error_code envelope handling (HTTP 200 + error_code) in internal/client/client.go + internal/client/hostex_envelope.go so non-zero error_code becomes typed errors and 429/5xx drive retry.
