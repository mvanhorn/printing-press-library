# Acceptance Report — sea-airport-parking

Level: Full Dogfood (binary-owned live matrix)
Tests: 85/85 passed
Failures: none (3 earlier failures fixed inline — see shipcheck report)
Gate: PASS

Live-verified behaviors:
- quote 3-night stay → $141 (3×$47), correct nightly breakdown, available=true
- quote short/invalid range → invalid=true with site validation message
- quote ×2 → drift snapshots=2, currently_available=true (persist→read loop)
- sweep flexible window → cheapest available returned, scan-capped
- history on empty range → [] ; on populated range → snapshot list
- parking → live page product info (title/description/links) via cookie-jar fix
- book → guest-checkout handoff, never submits payment

Auth context: type none (no key/login required; guest checkout).
