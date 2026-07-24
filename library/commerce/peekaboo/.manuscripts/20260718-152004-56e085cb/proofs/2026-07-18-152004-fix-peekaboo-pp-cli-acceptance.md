# Peekaboo CLI — Phase 5 Acceptance Report

Level: Full Dogfood (live)
Tests: 106/106 passed
Gate: PASS
Auth context: bearer_token (public guest token, auto-bootstrapped; no user credential needed)

## Fixes applied during dogfood (fix-before-ship)
1. **Absorbed body-param flag mismatch (CLI + spec).** The dogfood matrix synthesizes
   happy-path flags from endpoint wire keys (camelCase `entityId`/`targetEntityId`), but
   the generator kebab-cases flags (`--entity-id`). Added a per-command flag
   normalization func (`normalizeWireFlags`) on amenities/deals/places-detail so both
   spellings resolve — a genuine UX win. (Underlying matrix behavior filed for retro.)
2. **Zero-config auth for absorbed commands (CLI).** Only the novel commands called
   ensureGuestToken; the generated endpoint commands 401'd on a fresh install until a
   novel command bootstrapped the token first. Added a root pre-run
   (`maybeEnsureGuestToken`) that bootstraps the public guest token before any
   authenticated command, skipping local/public/metadata commands. Verified: `amenities`
   in a fresh HOME now auto-bootstraps and returns data.
3. **Framework learn commands lacked happy-args (generator gap).** teach / teach-pattern /
   teach-playbook / playbook-amend had no `pp:happy-args`, so the matrix parsed their
   multi-line Example fields and passed garbage (`\`, placeholders). Added valid
   `pp:happy-args` to each. Filed for retro (generator should emit these).
4. **teach --json was silent (framework).** `--json` was gated behind the quiet-default;
   an explicit `--json`/`--agent` now emits JSON. Filed for retro.

## Behavioral spot-checks (live)
- directions 13 --city lahore -> 4 branches + Google Maps directions URLs
- nearest 13 --near lahore -> closest branch + distance
- wallet hbl --city lahore --category 1 -> merchants honoring HBL cards
- top-deals --city lahore --category 1 -> deals ranked (biggest 50%)
- expiring --city lahore --category 1 --within 60d -> deals with days-left
- open-now --city lahore --category 1 -> merchants open now
- deals / amenities / places detail / branches / cards / categories / locations / brands -> all return correct data

## Printing Press issues (retro)
- Dogfood live matrix synthesizes happy-path flags from endpoint wire keys, not the
  generated (kebab-cased) flag names, and does not apply pp:happy-args for
  promoted/endpoint-derived paths -> false failures for any camelCase body param.
- Framework learn commands (teach/teach-pattern/teach-playbook/playbook amend) ship
  without pp:happy-args, so their dogfood happy-path/json-fidelity fail on Example-parsed args.
- teach suppresses --json output behind the quiet default.
- Generator drops defaulted bool body params (only sent when flag Changed) -> deals
  associatedDeals had to be modeled as a string.
- Hand-added top-level AddCommand + root PersistentPreRunE bootstrap are dropped by
  regen's lost-registration merge (custom, not in research.json).
