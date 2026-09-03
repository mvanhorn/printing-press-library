# MakerWorld CLI — Phase 5 Live Dogfood Acceptance

## Level: Full Dogfood
## Gate: PASS — 75/75 tests passed, 0 failed (42 skipped: error_path for no-input / no-error-path-probe commands)

## Matrix coverage (live against api.bambulab.com, no auth)
- Every leaf command: help, happy-path, JSON-fidelity, error-path where applicable.
- Synced mirror in-subprocess: designs 101, categories 15, designs-youlike 1.

## Failures found and fixed inline (all fixture/probe issues, no CLI bugs)
1. `designs ratings --design-id <UUID>` → HTTP 400 (comment-service ParseInt). MakerWorld IDs are numeric; dogfood synthesized a UUID. Fix: `pp:happy-args=--design-id=2865269` + numeric Example. (CLI fix — fixture.)
2. `designs search __invalid__` error_path → search is fuzzy and returns results for any keyword, so it cannot error on bad input. Fix: `pp:no-error-path-probe`. (CLI fix — annotation.)
3. `discover __invalid__` error_path → a non-matching keyword correctly returns empty (exit 0), not an error. Fix: `pp:no-error-path-probe`. (CLI fix — annotation.)
4. Misleading UUID examples in designs get/related/remixes replaced with a real numeric ID (2865269); numeric happy-args added so happy-path exercises real data.

## Token-gated commands (no MAKERWORLD_TOKEN available)
- download, favorites: graceful exit 0 with a clear "set MAKERWORLD_TOKEN" hint + empty result. Verified.

## Printing Press issues for retro
- Generated endpoint commands invented UUID example IDs / fixtures for a numeric-ID API. The generator could derive realistic fixtures from spec param type/format or research, or default positional/ID fixtures to numeric when the API uses integer IDs.
